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

	logWorkaround := func(a node.NodeStatus, id idwrap.IDWrap) {
		status <- runner.NewFlowStatus(runner.FlowStatusRunning, node.NodeStatusRunning, &id)
	}

	var readerWriterLock sync.RWMutex
	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		NodeMap:       r.FlowNodeMap,
		EdgeSourceMap: r.EdgesMap,
		LogPushFunc:   node.LogPushFunc(logWorkaround),
		Timeout:       r.Timeout,
		ReadWriteLock: &readerWriterLock,
	}
	fmt.Println("FlowLocalRunner.Run")
	status <- runner.NewFlowStatus(runner.FlowStatusStarting, node.NodeNone, nil)
	currentNode, ok := r.FlowNodeMap[*nextNodeID]
	if !ok {
		return fmt.Errorf("node not found: %v", nextNodeID)
	}
	if r.Timeout == 0 {
		_, err = RunNodeSync(ctx, currentNode, req, logWorkaround)
	} else {
		_, err = RunNodeASync(ctx, currentNode, req, logWorkaround)
	}

	if err != nil {
		fmt.Println(err)

		status <- runner.NewFlowStatus(runner.FlowStatusFailed, node.NodeStatusFailed, nextNodeID)
		return err
	}
	status <- runner.NewFlowStatus(runner.FlowStatusSuccess, node.NodeStatusSuccess, nil)
	return nil
}

func RunNodeSync(ctx context.Context, currentNode node.FlowNode, req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc) ([]idwrap.IDWrap, error) {
	id := currentNode.GetID()
	statusLogFunc(node.NodeStatusRunning, id)
	res := currentNode.RunSync(ctx, req)

	nextNodeLen := len(res.NextNodeID)
	if nextNodeLen == 0 {
		return nil, res.Err
	}
	if nextNodeLen == 1 {
		// TODO: check for hashmap before getting the next node
		return RunNodeSync(ctx, req.NodeMap[res.NextNodeID[0]], req, statusLogFunc)
	}
	var wg sync.WaitGroup

	parallel := make(chan node.FlowNodeResult, nextNodeLen)
	for _, v := range res.NextNodeID {
		currentNode, ok := req.NodeMap[v]
		if !ok {
			return nil, fmt.Errorf("node not found: %v", v)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := RunNodeSync(ctx, currentNode, req, statusLogFunc)
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

func RunNodeASync(ctx context.Context, currentNode node.FlowNode, req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc) ([]idwrap.IDWrap, error) {
	id := currentNode.GetID()
	statusLogFunc(node.NodeStatusRunning, id)

	resChan := make(chan node.FlowNodeResult, 1)
	go func() {
		res := currentNode.RunSync(ctx, req)
		resChan <- res
	}()

	var res *node.FlowNodeResult
	timedCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context done")
	case resLocal := <-resChan:
		res = &resLocal
	case <-timedCtx.Done():
		return nil, fmt.Errorf("timeout")
	}

	nextNodeLen := len(res.NextNodeID)
	if nextNodeLen == 0 {
		return nil, res.Err
	}
	if nextNodeLen == 1 {
		return RunNodeASync(ctx, req.NodeMap[res.NextNodeID[0]], req, statusLogFunc)
	}

	var wg sync.WaitGroup
	parallel := make(chan node.FlowNodeResult, nextNodeLen)
	for _, v := range res.NextNodeID {
		currentNode, ok := req.NodeMap[v]
		if !ok {
			return nil, fmt.Errorf("node not found: %v", v)
		}
		wg.Add(1)
		go func(currentNode node.FlowNode) {
			defer wg.Done()
			_, err := RunNodeASync(ctx, currentNode, req, statusLogFunc)
			if err != nil {
				parallel <- node.FlowNodeResult{
					NextNodeID: nil,
					Err:        err,
				}
				return
			}
		}(currentNode)
	}

	wg.Wait()
	close(parallel)

	var nextNodeID []idwrap.IDWrap
	for a := range parallel {
		if a.Err != nil {
			return nil, a.Err
		}
		nextNodeID = append(nextNodeID, a.NextNodeID...)
	}

	return nextNodeID, res.Err
}
