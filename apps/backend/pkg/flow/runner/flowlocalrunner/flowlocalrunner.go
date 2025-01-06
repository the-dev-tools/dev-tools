package flowlocalrunner

import (
	"context"
	"fmt"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/idwrap"
	"time"
)

type FlowLocalRunner struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	FlowNodeMap map[idwrap.IDWrap]node.FlowNode
	EdgesMap    edge.EdgesMap
	StartNodeID idwrap.IDWrap
	Timeout     time.Duration
}

func CreateFlowRunner(id, flowID, StartNodeID idwrap.IDWrap, FlowNodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap edge.EdgesMap, timeout time.Duration) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:          id,
		FlowID:      flowID,
		StartNodeID: StartNodeID,
		FlowNodeMap: FlowNodeMap,
		EdgesMap:    edgesMap,
		Timeout:     timeout,
	}
}

func (r FlowLocalRunner) Run(ctx context.Context, status chan runner.FlowStatusResp) error {
	nextNodeID := &r.StartNodeID
	var err error
	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		NodeMap:       r.FlowNodeMap,
		EdgeSourceMap: r.EdgesMap,
	}
	status <- runner.NewFlowStatus(runner.FlowStatusStarting, node.NodeNone, nil)
	for nextNodeID != nil {
		status <- runner.NewFlowStatus(runner.FlowStatusRunning, node.NodeStatusRunning, nextNodeID)
		currentNode, ok := r.FlowNodeMap[*nextNodeID]
		if !ok {
			return fmt.Errorf("node not found: %v", nextNodeID)
		}
		if r.Timeout == 0 {
			nextNodeID, err = RunNodeSync(ctx, currentNode, req)
		} else {
			nextNodeID, err = RunNodeAsync(ctx, currentNode, req, r.Timeout)
		}
		if err != nil {
			if err == context.DeadlineExceeded {
				status <- runner.NewFlowStatus(runner.FlowStatusTimeout, node.NodeStatusFailed, nextNodeID)
			}
			status <- runner.NewFlowStatus(runner.FlowStatusFailed, node.NodeStatusFailed, nextNodeID)
			return err
		}
	}
	status <- runner.NewFlowStatus(runner.FlowStatusSuccess, node.NodeStatusSuccess, nil)
	return nil
}

func RunNodeSync(ctx context.Context, currentNode node.FlowNode, req *node.FlowNodeRequest) (*idwrap.IDWrap, error) {
	res := currentNode.RunSync(ctx, req)
	return res.NextNodeID, res.Err
}

func RunNodeAsync(ctx context.Context, currentNode node.FlowNode, req *node.FlowNodeRequest, timeout time.Duration) (*idwrap.IDWrap, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resultChan := make(chan node.FlowNodeResult, 1)
	go currentNode.RunAsync(ctx, req, resultChan)
	select {
	case <-ctx.Done():
		fmt.Println("timeout")
		return nil, ctx.Err()
	case result := <-resultChan:
		return result.NextNodeID, result.Err
	}
}
