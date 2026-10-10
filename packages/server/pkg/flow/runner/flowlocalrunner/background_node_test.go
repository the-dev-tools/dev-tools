package flowlocalrunner_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// backgroundNode keeps the context it ran with, like a WebSocket connection that keeps reading.
type backgroundNode struct {
	stubNode
	ctx context.Context
}

func (b *backgroundNode) RunsInBackground() bool { return true }

func (b *backgroundNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	b.ctx = ctx
	return b.stubNode.RunSync(ctx, req)
}

func (b *backgroundNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- b.RunSync(ctx, req)
}

// laterNode records whether the background node's context was still alive when it ran.
type laterNode struct {
	stubNode
	bg    *backgroundNode
	alive bool
}

func (l *laterNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	l.alive = l.bg.ctx != nil && l.bg.ctx.Err() == nil
	return l.stubNode.RunSync(ctx, req)
}

func (l *laterNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- l.RunSync(ctx, req)
}

// A per-node timeout must not end a background node's work when the node returns: later nodes
// (ws_send) still use what it started (the connection).
func TestBackgroundNodeOutlivesPerNodeTimeout(t *testing.T) {
	for _, mode := range []flowlocalrunner.ExecutionMode{flowlocalrunner.ExecutionModeSingle, flowlocalrunner.ExecutionModeMulti} {
		bgID, laterID := idwrap.NewNow(), idwrap.NewNow()
		bg := &backgroundNode{stubNode: stubNode{id: bgID, name: "conn", next: []idwrap.IDWrap{laterID}}}
		later := &laterNode{stubNode: stubNode{id: laterID, name: "send"}, bg: bg}
		nodeMap := map[idwrap.IDWrap]node.FlowNode{bgID: bg, laterID: later}
		edges := mflow.EdgesMap{
			bgID:    {mflow.HandleUnspecified: []idwrap.IDWrap{laterID}},
			laterID: {},
		}

		r := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), []idwrap.IDWrap{bgID}, nodeMap, edges, time.Minute, nil)
		r.SetExecutionMode(mode)
		statuses := make(chan runner.FlowNodeStatus, 16)
		flowStatuses := make(chan runner.FlowStatus, 4)
		require.NoError(t, r.Run(context.Background(), statuses, flowStatuses, nil))
		drainStates(statuses)
		drainFlowStatus(flowStatuses)

		require.True(t, later.alive, "mode %v: the background node's context ended when it returned", mode)
	}
}
