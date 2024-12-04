package flowlocalrunner

import (
	"context"
	"fmt"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/idwrap"
	"time"
)

type FlowLocalRunner struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	FlowNodeMap map[idwrap.IDWrap]node.FlowNode
	StartNodeID idwrap.IDWrap
	Timeout     time.Duration
}

func CreateFlowRunner(id, flowID, StartNodeID idwrap.IDWrap, FlowNodeMap map[idwrap.IDWrap]node.FlowNode, timeout time.Duration) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:          id,
		FlowID:      flowID,
		StartNodeID: StartNodeID,
		FlowNodeMap: FlowNodeMap,
		Timeout:     timeout,
	}
}

func (r FlowLocalRunner) Run(ctx context.Context, status chan runner.FlowStatus) error {
	nextNodeID := &r.StartNodeID
	var err error
	variableMap := make(map[string]interface{})
	for nextNodeID != nil {
		currentNode, ok := r.FlowNodeMap[*nextNodeID]
		if !ok {
			return runner.ErrNodeNotFound
		}
		if r.Timeout == 0 {
			nextNodeID, err = RunNodeSync(ctx, currentNode, variableMap)
		} else {
			nextNodeID, err = RunNodeAsync(ctx, currentNode, variableMap, r.Timeout)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RunNodeSync(ctx context.Context, currentNode node.FlowNode, variableMap map[string]interface{}) (*idwrap.IDWrap, error) {
	res := currentNode.RunSync(ctx, variableMap)
	return res.NextNodeID, res.Err
}

func RunNodeAsync(ctx context.Context, currentNode node.FlowNode, variableMap map[string]interface{}, timeout time.Duration) (*idwrap.IDWrap, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resultChan := make(chan node.FlowNodeResult, 1)
	go currentNode.RunAsync(ctx, variableMap, resultChan)
	select {
	case <-ctx.Done():
		fmt.Println("timeout")
		return nil, ctx.Err()
	case result := <-resultChan:
		return result.NextNodeID, result.Err
	}
}
