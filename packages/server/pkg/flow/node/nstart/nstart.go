//nolint:revive // exported
package nstart

import (
	"context"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeStart struct {
	FlowNodeID idwrap.IDWrap
	Name       string
}

func New(id idwrap.IDWrap, name string) *NodeStart {
	return &NodeStart{
		FlowNodeID: id,
		Name:       name,
	}
}

func (nr *NodeStart) GetID() idwrap.IDWrap {
	return nr.FlowNodeID
}

func (nr *NodeStart) SetID(id idwrap.IDWrap) {
	nr.FlowNodeID = id
}

func (nr *NodeStart) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, mflow.HandleUnspecified)

	return node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}

func (nr *NodeStart) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, mflow.HandleUnspecified)

	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
	resultChan <- result
}
