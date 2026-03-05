//nolint:revive // exported
package nwait

import (
	"context"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWait struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	DurationMs int64
}

func New(id idwrap.IDWrap, name string, durationMs int64) *NodeWait {
	return &NodeWait{
		FlowNodeID: id,
		Name:       name,
		DurationMs: durationMs,
	}
}

func (n *NodeWait) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeWait) GetName() string {
	return n.Name
}

func (n *NodeWait) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	timer := time.NewTimer(time.Duration(n.DurationMs) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return node.FlowNodeResult{Err: ctx.Err()}
	case <-timer.C:
	}

	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	return node.FlowNodeResult{NextNodeID: nextID}
}

func (n *NodeWait) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}
