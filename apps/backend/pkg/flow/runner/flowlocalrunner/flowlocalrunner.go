package flowlocalrunner

import (
	"context"
	"fmt"
	"sync"
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
	fmt.Println("FlowLocalRunner.Run")
	status <- runner.NewFlowStatus(runner.FlowStatusStarting, node.NodeNone, nil)
	currentNode, ok := r.FlowNodeMap[*nextNodeID]
	if !ok {
		return fmt.Errorf("node not found: %v", nextNodeID)
	}
	_, err = r.RunNodeSync(ctx, currentNode, req, status)
	if err != nil {
		status <- runner.NewFlowStatus(runner.FlowStatusFailed, node.NodeStatusFailed, nextNodeID)
		return err
	}
	status <- runner.NewFlowStatus(runner.FlowStatusSuccess, node.NodeStatusSuccess, nil)
	return nil
}

func (r FlowLocalRunner) RunNodeSync(ctx context.Context, currentNode node.FlowNode, req *node.FlowNodeRequest, status chan runner.FlowStatusResp) ([]idwrap.IDWrap, error) {
	var wg sync.WaitGroup

	id := currentNode.GetID()
	status <- runner.NewFlowStatus(runner.FlowStatusRunning, node.NodeStatusRunning, &id)
	res := currentNode.RunSync(ctx, req)

	nextNodeLen := len(res.NextNodeID)
	if nextNodeLen == 0 {
		return nil, res.Err
	}
	if nextNodeLen == 1 {
		return res.NextNodeID, res.Err
	}

	parallel := make(chan node.FlowNodeResult, nextNodeLen)
	for _, v := range res.NextNodeID {
		currentNode, ok := r.FlowNodeMap[v]
		if !ok {
			return nil, fmt.Errorf("node not found: %v", v)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := r.RunNodeSync(ctx, currentNode, req, status)
			parallel <- node.FlowNodeResult{
				NextNodeID: res,
				Err:        err,
			}
		}()
	}

	wg.Wait()

	var nextNodeID []idwrap.IDWrap
	for i := 0; i < nextNodeLen; i++ {
		a := <-parallel
		if a.Err != nil {
			return nil, a.Err
		}
		nextNodeID = append(nextNodeID, a.NextNodeID...)
	}

	return nextNodeID, res.Err
}
