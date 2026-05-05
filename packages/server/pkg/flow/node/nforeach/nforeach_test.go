package nforeach

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
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

func TestNodeForEachDefaultErrorDoesNotLogLoopFailure(t *testing.T) {
	loopID := idwrap.NewNow()
	childID := idwrap.NewNow()
	childErr := errors.New("child execution failed")

	loop := New(loopID, "ForEachNode", "items", 0, mcondition.Condition{}, mflow.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	childRan := false
	child := failingNode{id: childID, name: "Child", err: childErr, ran: &childRan}

	edgeMap := mflow.EdgesMap{
		loopID: {
			mflow.HandleLoop: []idwrap.IDWrap{childID},
		},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), []idwrap.IDWrap{loopID}, map[idwrap.IDWrap]node.FlowNode{
		loopID:  loop,
		childID: child,
	}, edgeMap, 0, nil)

	statusCh := make(chan runner.FlowNodeStatus, 16)
	flowCh := make(chan runner.FlowStatus, 4)

	err := flowRunner.Run(context.Background(), statusCh, flowCh, map[string]any{"items": []any{"v"}})

	var statuses []runner.FlowNodeStatus
	for st := range statusCh {
		statuses = append(statuses, st)
	}
	for range flowCh {
	}
	for _, st := range statuses {
		t.Logf("status node=%s state=%v err=%v name=%s", st.NodeID.String(), st.State, st.Error, st.Name)
	}

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
	require.True(t, loopFailureLogged, "expected foreach iteration failure to be logged")
	if loopFailureLogged {
		require.True(t, loopFailure.IterationEvent, "foreach failure should be iteration event")
		require.NotNil(t, loopFailure.OutputData)
		if data, ok := loopFailure.OutputData.(map[string]any); ok {
			require.Contains(t, data, "item")
			require.Contains(t, data, "key")
		} else {
			t.Fatalf("foreach failure output not map: %#v", loopFailure.OutputData)
		}
	}
	require.True(t, loopCancelled, "expected foreach node to emit canceled status")
	require.True(t, childRan, "child node did not execute")
}

func TestNodeForEachSetsIterationEventFlag(t *testing.T) {
	loopID := idwrap.NewNow()
	loop := New(loopID, "ForEachNode", "items", 0, mcondition.Condition{}, mflow.ErrorHandling_ERROR_HANDLING_IGNORE)

	edgeMap := mflow.EdgesMap{
		loopID: {
			mflow.HandleLoop: []idwrap.IDWrap{},
		},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), []idwrap.IDWrap{loopID}, map[idwrap.IDWrap]node.FlowNode{
		loopID: loop,
	}, edgeMap, 0, nil)

	statusCh := make(chan runner.FlowNodeStatus, 16)
	flowCh := make(chan runner.FlowStatus, 4)

	vars := map[string]any{"items": []any{"a", "b"}}
	require.NoError(t, flowRunner.Run(context.Background(), statusCh, flowCh, vars))

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
	require.NotNil(t, finalStatus, "expected foreach terminal status")
	require.False(t, finalStatus.IterationEvent)
	require.Equal(t, loopID, finalStatus.NodeID)
}

func TestNodeForEachSkipsDuplicateLoopEntryTargets(t *testing.T) {
	loopID := idwrap.NewNow()
	nodeAID := idwrap.NewNow()
	nodeBID := idwrap.NewNow()
	nodeCID := idwrap.NewNow()

	loop := New(loopID, "ForEachNode", "items", 0, mcondition.Condition{}, mflow.ErrorHandling_ERROR_HANDLING_IGNORE)

	var nodeARuns, nodeBRuns, nodeCRuns int

	nodeA := recordingNode{id: nodeAID, name: "A", runCount: &nodeARuns}
	nodeB := recordingNode{id: nodeBID, name: "B", runCount: &nodeBRuns}
	nodeC := recordingNode{id: nodeCID, name: "C", runCount: &nodeCRuns}

	edges := mflow.NewEdgesMap(mflow.NewEdges(
		mflow.NewEdge(idwrap.NewNow(), loopID, nodeAID, mflow.HandleLoop),
		mflow.NewEdge(idwrap.NewNow(), loopID, nodeCID, mflow.HandleLoop),
		mflow.NewEdge(idwrap.NewNow(), nodeAID, nodeBID, mflow.HandleThen),
		mflow.NewEdge(idwrap.NewNow(), nodeBID, nodeCID, mflow.HandleThen),
	))

	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		[]idwrap.IDWrap{loopID},
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

	vars := map[string]any{"items": []any{"value"}}
	require.NoError(t, flowRunner.Run(context.Background(), statusCh, flowCh, vars))

	for range statusCh {
	}
	for range flowCh {
	}

	require.Equal(t, 1, nodeARuns, "node A should execute exactly once")
	require.Equal(t, 1, nodeBRuns, "node B should execute exactly once")
	require.Equal(t, 1, nodeCRuns, "node C should execute exactly once")
}

// TestForeach_RunSync_TracksIterPath verifies that the FOREACH node tracks
// variables accessed in the iteration path expression (pure expr-lang).
func TestForeach_RunSync_TracksIterPath(t *testing.T) {
	loopID := idwrap.NewNow()

	// Create a simple FOREACH node
	loop := New(loopID, "testLoop", "httpNode.response.items", 0, mcondition.Condition{}, mflow.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Create tracker
	tracker := tracking.NewVariableTracker()

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"httpNode": map[string]any{
				"response": map[string]any{
					"items": []any{"a", "b", "c"},
				},
			},
		},
		ReadWriteLock:   &sync.RWMutex{},
		NodeMap:         map[idwrap.IDWrap]node.FlowNode{loopID: loop},
		EdgeSourceMap:   mflow.NewEdgesMap(nil),
		VariableTracker: tracker,
	}

	result := loop.RunSync(context.Background(), req)
	require.NoError(t, result.Err)

	// Verify that the iteration path variable was tracked
	readVars := tracker.GetReadVars()
	require.NotEmpty(t, readVars, "Expected variables to be tracked")
	require.Contains(t, readVars, "httpNode.response.items",
		"Expected 'httpNode.response.items' to be tracked for FOREACH iter path")
}

// valueWritingNode is a child node for tests: each call writes
// `<n.name>.value = <runs>` to the parent VarMap (mimicking how a real
// HTTP/JS node makes its output visible to a loop's break expression).
type valueWritingNode struct {
	id   idwrap.IDWrap
	name string
	runs *int
}

func (n valueWritingNode) GetID() idwrap.IDWrap { return n.id }
func (n valueWritingNode) GetName() string      { return n.name }

func (n valueWritingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if n.runs != nil {
		*n.runs++
	}
	_ = node.WriteNodeVar(req, n.name, "value", *n.runs)
	next := mflow.GetNextNodeID(req.EdgeSourceMap, n.id, mflow.HandleThen)
	return node.FlowNodeResult{NextNodeID: next}
}

func (n valueWritingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

// TestNodeForEachBreakConditionSeesIterationOutputs verifies that the
// break expression is evaluated AFTER each iteration's children run, so it
// can reference outputs the children just produced. Also verifies that the
// "true means break" semantics matches NodeFor (was previously inverted).
func TestNodeForEachBreakConditionSeesIterationOutputs(t *testing.T) {
	loopID := idwrap.NewNow()
	childID := idwrap.NewNow()

	cond := mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "child.value >= 3"}}
	loop := New(loopID, "ForEachNode", "items", 0, cond, mflow.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	runs := 0
	child := valueWritingNode{id: childID, name: "child", runs: &runs}

	edgeMap := mflow.EdgesMap{
		loopID: {mflow.HandleLoop: []idwrap.IDWrap{childID}},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), []idwrap.IDWrap{loopID},
		map[idwrap.IDWrap]node.FlowNode{loopID: loop, childID: child}, edgeMap, 0, nil)

	statusCh := make(chan runner.FlowNodeStatus, 64)
	flowCh := make(chan runner.FlowStatus, 4)

	require.NoError(t, flowRunner.Run(context.Background(), statusCh, flowCh, map[string]any{
		"items": []any{"a", "b", "c", "d", "e", "f"},
	}))
	for range statusCh {
	}
	for range flowCh {
	}

	require.Equal(t, 3, runs, "child should run 3 times: iter0 writes 1, iter1 writes 2, iter2 writes 3 → break")
}

// TestNodeForEachBreakConditionToleratesUndefinedIdentifier verifies that an
// expression referencing a not-yet-written variable doesn't crash the flow.
func TestNodeForEachBreakConditionToleratesUndefinedIdentifier(t *testing.T) {
	loopID := idwrap.NewNow()
	childID := idwrap.NewNow()

	cond := mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "nonexistent.value > 0"}}
	loop := New(loopID, "ForEachNode", "items", 0, cond, mflow.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	runs := 0
	child := valueWritingNode{id: childID, name: "child", runs: &runs}

	edgeMap := mflow.EdgesMap{
		loopID: {mflow.HandleLoop: []idwrap.IDWrap{childID}},
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), []idwrap.IDWrap{loopID},
		map[idwrap.IDWrap]node.FlowNode{loopID: loop, childID: child}, edgeMap, 0, nil)

	statusCh := make(chan runner.FlowNodeStatus, 64)
	flowCh := make(chan runner.FlowStatus, 4)

	require.NoError(t, flowRunner.Run(context.Background(), statusCh, flowCh, map[string]any{
		"items": []any{"a", "b", "c", "d", "e"},
	}))
	for range statusCh {
	}
	for range flowCh {
	}

	require.Equal(t, 5, runs, "loop should iterate over all 5 items when break expression references an undefined identifier")
}
