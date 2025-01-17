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

	for i := int64(0); i < nr.IterCount; i++ {
		for j, nextNodeID := range loopID {
			currentNode, ok := req.NodeMap[nextNodeID]
			if !ok {
				return node.FlowNodeResult{
					NextNodeID: nil,
					Err:        node.ErrNodeNotFound,
				}
			}
			_, err := flowlocalrunner.RunNodeSync(ctx, currentNode, req, req.LogPushFunc)
			// TODO: add run for subflow
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
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

	for i := int64(0); i < nr.IterCount; i++ {
		for j, nextNodeID := range loopID {
			currentNode, ok := req.NodeMap[nextNodeID]
			if !ok {
				resultChan <- node.FlowNodeResult{
					NextNodeID: nil,
					Err:        node.ErrNodeNotFound,
				}
				return
			}
			_, err := flowlocalrunner.RunNodeSync(ctx, currentNode, req, req.LogPushFunc)
			// TODO: add run for subflow
			if err != nil {
				resultChan <- node.FlowNodeResult{
					Err: err,
				}
				return
			}
		}
	}

	resultChan <- node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}
