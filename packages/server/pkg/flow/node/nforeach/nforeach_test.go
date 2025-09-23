package nforeach

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
	"the-dev-tools/server/pkg/model/mcondition"
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

func TestNodeForEachDefaultErrorDoesNotLogLoopFailure(t *testing.T) {
	loopID := idwrap.NewNow()
	childID := idwrap.NewNow()
	childErr := errors.New("child execution failed")

	loop := New(loopID, "ForEachNode", "items", 0, mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
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
	}, edgeMap, 0)

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
	loopCancelled := false
	for _, st := range statuses {
		if st.NodeID == childID && st.State == mnnode.NODE_STATE_FAILURE {
			childLogged = true
		}
		if st.NodeID == loopID && st.State == mnnode.NODE_STATE_CANCELED {
			loopCancelled = true
		}
		require.False(t, st.NodeID == loopID && st.State == mnnode.NODE_STATE_FAILURE,
			"unexpected foreach failure status: %#v", st)
		require.NotEqual(t, "Error Summary", st.Name)
	}
	require.True(t, childLogged, "expected child node failure to be logged")
	require.True(t, loopCancelled, "expected foreach node to emit canceled status")
	require.True(t, childRan, "child node did not execute")
}
