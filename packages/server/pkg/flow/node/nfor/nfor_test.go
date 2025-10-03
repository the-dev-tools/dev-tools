package nfor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

type failingNode struct {
	id   idwrap.IDWrap
	name string
	err  error
	ran  *bool
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
			State:       mnnode.NODE_STATE_FAILURE,
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
			State:       mnnode.NODE_STATE_FAILURE,
			Error:       n.err,
		})
	}
	resultChan <- node.FlowNodeResult{Err: n.err}
}

func TestNodeForDefaultErrorDoesNotLogLoopFailure(t *testing.T) {
	loopID := idwrap.NewNow()
	childID := idwrap.NewNow()
	childErr := errors.New("child execution failed")

	loop := New(loopID, "LoopNode", 1, 0, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	childRan := false
	child := failingNode{id: childID, name: "Child", err: childErr, ran: &childRan}

	edgeMap := edge.EdgesMap{
		loopID: {
			edge.HandleLoop: []idwrap.IDWrap{childID},
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
		if st.NodeID == childID && st.State == mnnode.NODE_STATE_FAILURE {
			childLogged = true
		}
		if st.NodeID == loopID && st.State == mnnode.NODE_STATE_FAILURE {
			loopFailureLogged = true
			loopFailure = st
		}
		if st.NodeID == loopID && st.State == mnnode.NODE_STATE_CANCELED {
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
	loop := New(loopID, "LoopNode", 2, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	edgeMap := edge.EdgesMap{
		loopID: {
			edge.HandleLoop: []idwrap.IDWrap{},
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
		require.True(t, st.State == mnnode.NODE_STATE_RUNNING || st.State == mnnode.NODE_STATE_SUCCESS)
	}
	require.NotNil(t, finalStatus, "expected loop terminal status")
	require.False(t, finalStatus.IterationEvent)
	require.Equal(t, loopID, finalStatus.NodeID)
}
