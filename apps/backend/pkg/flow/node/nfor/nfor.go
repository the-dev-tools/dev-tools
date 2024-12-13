package nfor

import (
	"context"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/backend/pkg/idwrap"
	"time"
)

type NodeFor struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	IterCount  int64
	Timeout    time.Duration
}

func New(id idwrap.IDWrap, name string, iterCount int64, timeout time.Duration) *NodeFor {
	return &NodeFor{
		FlowNodeID: id,
		Name:       name,
		IterCount:  iterCount,
		Timeout:    timeout,
	}
}

func (nr *NodeFor) GetID() idwrap.IDWrap {
	return nr.FlowNodeID
}

func (nr *NodeFor) SetID(id idwrap.IDWrap) {
	nr.FlowNodeID = id
}

func (nr *NodeFor) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleUnspecified)

	if loopID != nil {
		for i := int64(0); i < nr.IterCount; i++ {
			for nextNodeID := loopID; nextNodeID != nil; {
				currentNode, ok := req.NodeMap[*nextNodeID]
				if !ok {
					return node.FlowNodeResult{
						NextNodeID: nil,
						Err:        node.ErrNodeNotFound,
					}
				}
				res := currentNode.RunSync(ctx, req)
				nextNodeID = res.NextNodeID
				if res.Err != nil {
					return res
				}
			}
		}
	}

	return node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}

func (nr *NodeFor) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleUnspecified)

	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
	if loopID != nil {
		for i := int64(0); i < nr.IterCount; i++ {
			for nextNodeID := loopID; nextNodeID != nil; {
				currentNode, ok := req.NodeMap[*nextNodeID]
				if !ok {
					result.Err = node.ErrNodeNotFound
					resultChan <- result
				}
				id, err := flowlocalrunner.RunNodeAsync(ctx, currentNode, req, nr.Timeout)
				if err != nil {
					result.Err = node.ErrNodeNotFound
					resultChan <- result
				}
				nextNodeID = id
			}
		}
	}

	resultChan <- result
}
