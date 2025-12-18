//nolint:revive // exported
package nnoop

import (
	"context"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

const NodeOutputKey = "noop"

type NodeNoop struct {
	FlowNodeID idwrap.IDWrap
	Name       string
}

func New(id idwrap.IDWrap, name string) *NodeNoop {
	return &NodeNoop{
		FlowNodeID: id,
		Name:       name,
	}
}

func (n NodeNoop) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeNoop) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n NodeNoop) GetName() string {
	return n.Name
}

func (n NodeNoop) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	var result node.FlowNodeResult
	result.NextNodeID = nextID
	return result
}

func (n NodeNoop) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	trueID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	var result node.FlowNodeResult
	result.NextNodeID = trueID
	resultChan <- result
}
