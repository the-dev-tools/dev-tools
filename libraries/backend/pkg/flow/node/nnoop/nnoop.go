package nnoop

import (
	"context"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"
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
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleUnspecified)
	var result node.FlowNodeResult
	result.NextNodeID = nextID
	return result
}

func (n NodeNoop) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	trueID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleUnspecified)
	var result node.FlowNodeResult
	result.NextNodeID = trueID
	resultChan <- result
}
