package nfor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type failingNode struct {
	id   idwrap.IDWrap
	name string
	err  error
	ran  *bool
}

type recordingNode struct {
	id       idwrap.IDWrap
	name     string
	runCount *int
}

func (n recordingNode) GetID() idwrap.IDWrap { return n.id }

func (n recordingNode) GetName() string { return n.name }

func (n recordingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if n.runCount != nil {
		*n.runCount++
	}
	next := mflow.GetNextNodeID(req.EdgeSourceMap, n.id, mflow.HandleThen)
	return node.FlowNodeResult{NextNodeID: next}
}

func (n recordingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	if n.runCount != nil {
		*n.runCount++
	}
	next := mflow.GetNextNodeID(req.EdgeSourceMap, n.id, mflow.HandleThen)
	resultChan <- node.FlowNodeResult{NextNodeID: next}
}

func (n failingNode) GetID() idwrap.IDWrap { return n.id }

func (n failingNode) GetName() string { return n.name }

func (n failingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if n.ran != nil {
		*n.ran = true
	}
	if req.LogPushFunc != nil {
		req.LogPushFunc(runner.FlowNodeStatus{
			ExecutionID: req.ExecutionID,
			NodeID:      n.id,
			Name:        n.name,
			State:       mflow.NODE_STATE_FAILURE,
			Error:       n.err,
		})
	}
	return node.FlowNodeResult{Err: n.err}
}

func (n failingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	if n.ran != nil {
		*n.ran = true
	}
	if req.LogPushFunc != nil {
		req.LogPushFunc(runner.FlowNodeStatus{
			ExecutionID: req.ExecutionID,
			NodeID:      n.id,
			Name:        n.name,
			State:       mflow.NODE_STATE_FAILURE,
			Error:       n.err,
		})
	}
	resultChan <- node.FlowNodeResult{Err: n.err}
}

func TestNodeForDefaultErrorDoesNotLogLoopFailure(t *testing.T) {
	loopID := idwrap.NewNow()
	childID := idwrap.NewNow()
	childErr := errors.New("child execution failed")

	loop := New(loopID, "LoopNode", 1, 0, mflow.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	childRan := false
	child := failingNode{id: childID, name: "Child", err: childErr, ran: &childRan}

	edgeMap := mflow.EdgesMap{
		loopID: {
			mflow.HandleLoop: []idwrap.IDWrap{childID},
		},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), loopID, map[idwrap.IDWrap]node.FlowNode{
		loopID:  loop,
		childID: child,
	}, edgeMap, 0, nil)

	statusCh := make(chan runner.FlowNodeStatus, 16)
	flowCh := make(chan runner.FlowStatus, 4)

	err := flowRunner.Run(context.Background(), statusCh, flowCh, map[string]any{})

	var statuses []runner.FlowNodeStatus
	for st := range statusCh {
		statuses = append(statuses, st)
	}
	// Drain flow status channel
	for range flowCh {
	}
	for _, st := range statuses {
		t.Logf("status node=%s state=%v err=%v name=%s", st.NodeID.String(), st.State, st.Error, st.Name)
	}
	t.Logf("childRan=%v", childRan)

	require.ErrorIsf(t, err, childErr, "statuses=%v", statuses)
	require.ErrorIs(t, err, runner.ErrFlowCanceledByThrow)

	childLogged := false
	loopFailureLogged := false
	loopCancelled := false
	var loopFailure runner.FlowNodeStatus
	for _, st := range statuses {
		if st.NodeID == childID && st.State == mflow.NODE_STATE_FAILURE {
			childLogged = true
		}
		if st.NodeID == loopID && st.State == mflow.NODE_STATE_FAILURE {
			loopFailureLogged = true
			loopFailure = st
		}
		if st.NodeID == loopID && st.State == mflow.NODE_STATE_CANCELED {
			loopCancelled = true
		}
		require.NotEqual(t, "Error Summary", st.Name)
	}
	require.True(t, childLogged, "expected child node failure to be logged")
	require.True(t, loopFailureLogged, "expected loop iteration failure to be logged")
	if loopFailureLogged {
		require.True(t, loopFailure.IterationEvent, "loop failure should be iteration event")
		require.NotNil(t, loopFailure.OutputData)
		if data, ok := loopFailure.OutputData.(map[string]any); ok {
			require.Contains(t, data, "index")
		} else {
			t.Fatalf("loop failure output not map: %#v", loopFailure.OutputData)
		}
	}
	require.True(t, loopCancelled, "expected loop node to emit canceled status")
	require.True(t, childRan, "child node did not execute")
}

func TestNodeForSetsIterationEventFlag(t *testing.T) {
	loopID := idwrap.NewNow()
	loop := New(loopID, "LoopNode", 2, 0, mflow.ErrorHandling_ERROR_HANDLING_IGNORE)

	edgeMap := mflow.EdgesMap{
		loopID: {
			mflow.HandleLoop: []idwrap.IDWrap{},
		},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), loopID, map[idwrap.IDWrap]node.FlowNode{
		loopID: loop,
	}, edgeMap, 0, nil)

	statusCh := make(chan runner.FlowNodeStatus, 16)
	flowCh := make(chan runner.FlowStatus, 4)

	require.NoError(t, flowRunner.Run(context.Background(), statusCh, flowCh, map[string]any{}))

	var iterationEvents []runner.FlowNodeStatus
	var finalStatus *runner.FlowNodeStatus
	for st := range statusCh {
		if st.NodeID != loopID {
			continue
		}
		if st.IterationEvent {
			iterationEvents = append(iterationEvents, st)
		} else {
			copy := st
			finalStatus = &copy
		}
	}
	for range flowCh {
	}

	require.Len(t, iterationEvents, 4, "expected two iterations with RUNNING/SUCCESS updates")
	for _, st := range iterationEvents {
		require.Equal(t, loopID, st.LoopNodeID)
		require.True(t, st.IterationIndex == 0 || st.IterationIndex == 1)
		require.True(t, st.State == mflow.NODE_STATE_RUNNING || st.State == mflow.NODE_STATE_SUCCESS)
	}
	require.NotNil(t, finalStatus, "expected loop terminal status")
	require.False(t, finalStatus.IterationEvent)
	require.Equal(t, loopID, finalStatus.NodeID)
}

func TestNodeForSkipsDuplicateLoopEntryTargets(t *testing.T) {
	loopID := idwrap.NewNow()
	nodeAID := idwrap.NewNow()
	nodeBID := idwrap.NewNow()
	nodeCID := idwrap.NewNow()

	loop := New(loopID, "LoopNode", 1, 0, mflow.ErrorHandling_ERROR_HANDLING_IGNORE)

	var nodeARuns, nodeBRuns, nodeCRuns int

	nodeA := recordingNode{id: nodeAID, name: "A", runCount: &nodeARuns}
	nodeB := recordingNode{id: nodeBID, name: "B", runCount: &nodeBRuns}
	nodeC := recordingNode{id: nodeCID, name: "C", runCount: &nodeCRuns}

	edges := mflow.NewEdgesMap(mflow.NewEdges(
		mflow.NewEdge(idwrap.NewNow(), loopID, nodeAID, mflow.HandleLoop, 0),
		mflow.NewEdge(idwrap.NewNow(), loopID, nodeCID, mflow.HandleLoop, 0),
		mflow.NewEdge(idwrap.NewNow(), nodeAID, nodeBID, mflow.HandleThen, 0),
		mflow.NewEdge(idwrap.NewNow(), nodeBID, nodeCID, mflow.HandleThen, 0),
	))

	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		loopID,
		map[idwrap.IDWrap]node.FlowNode{
			loopID:  loop,
			nodeAID: nodeA,
			nodeBID: nodeB,
			nodeCID: nodeC,
		},
		edges,
		0,
		nil,
	)

	statusCh := make(chan runner.FlowNodeStatus, 16)
	flowCh := make(chan runner.FlowStatus, 4)

	require.NoError(t, flowRunner.Run(context.Background(), statusCh, flowCh, map[string]any{}))

	for range statusCh {
	}
	for range flowCh {
	}

	require.Equal(t, 1, nodeARuns, "node A should execute exactly once")
	require.Equal(t, 1, nodeBRuns, "node B should execute exactly once")
	require.Equal(t, 1, nodeCRuns, "node C should execute exactly once")
}
