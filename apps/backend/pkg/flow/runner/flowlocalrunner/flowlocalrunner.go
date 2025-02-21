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
		status <- runner.NewFlowStatus(runner.FlowStatusRunning, a, &id)
	}

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       r.FlowNodeMap,
		EdgeSourceMap: r.EdgesMap,
		LogPushFunc:   node.LogPushFunc(logWorkaround),
		Timeout:       r.Timeout,
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

func RunNodeSync(ctx context.Context, startNode node.FlowNode, req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc) ([]idwrap.IDWrap, error) {
	var nextNodeID []idwrap.IDWrap
	queue := []node.FlowNode{startNode}
	processedNodes := make(map[idwrap.IDWrap]bool)

	for len(queue) > 0 {
		currentNode := queue[0]
		queue = queue[1:]

		id := currentNode.GetID()
		if processedNodes[id] {
			continue
		}
		processedNodes[id] = true

		statusLogFunc(node.NodeStatusRunning, id)
		res := currentNode.RunSync(ctx, req)

		nextNodeLen := len(res.NextNodeID)
		if nextNodeLen == 0 {
			if res.Err != nil {
				statusLogFunc(node.NodeStatusFailed, id)
				return nil, res.Err
			}
			statusLogFunc(node.NodeStatusSuccess, id)
			continue
		}
		statusLogFunc(node.NodeStatusSuccess, id)

		if nextNodeLen == 1 {
			nextNode, ok := req.NodeMap[res.NextNodeID[0]]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", res.NextNodeID[0])
			}
			queue = append(queue, nextNode)
			continue
		}

		var wg sync.WaitGroup
		parallel := make(chan node.FlowNodeResult, nextNodeLen)
		for _, v := range res.NextNodeID {
			nextNode, ok := req.NodeMap[v]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", v)
			}

			wg.Add(1)
			go func(nextNode node.FlowNode) {
				defer wg.Done()

				statusLogFunc(node.NodeStatusRunning, v)

				res := nextNode.RunSync(ctx, req)
				if res.Err != nil {
					statusLogFunc(node.NodeStatusFailed, v)
				} else {
					statusLogFunc(node.NodeStatusSuccess, v)
				}

				parallel <- node.FlowNodeResult{
					NextNodeID: res.NextNodeID,
					Err:        res.Err,
				}
			}(nextNode)
		}

		wg.Wait()
		close(parallel)

		mergedNextNodeIDs := make(map[idwrap.IDWrap]bool)
		for a := range parallel {

			if a.Err != nil {
				return nil, a.Err
			}
			for _, nextID := range a.NextNodeID {
				mergedNextNodeIDs[nextID] = true
			}
		}

		for nextID := range mergedNextNodeIDs {
			nextNode, ok := req.NodeMap[nextID]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", nextID)
			}
			queue = append(queue, nextNode)
		}
	}

	return nextNodeID, nil
}

func RunNodeASync(ctx context.Context, startNode node.FlowNode, req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc) ([]idwrap.IDWrap, error) {
	var nextNodeID []idwrap.IDWrap
	queue := []node.FlowNode{startNode}
	processedNodes := make(map[idwrap.IDWrap]bool)

	for len(queue) > 0 {
		currentNode := queue[0]
		queue = queue[1:]

		id := currentNode.GetID()
		if processedNodes[id] {
			continue
		}
		processedNodes[id] = true

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
		statusLogFunc(node.NodeStatusSuccess, id)

		nextNodeLen := len(res.NextNodeID)
		if nextNodeLen == 0 {
			if res.Err != nil {

				statusLogFunc(node.NodeStatusFailed, id)
				return nil, res.Err
			}
			continue
		}

		if nextNodeLen == 1 {
			nextNode, ok := req.NodeMap[res.NextNodeID[0]]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", res.NextNodeID[0])
			}
			queue = append(queue, nextNode)
			continue
		}

		var wg sync.WaitGroup
		parallel := make(chan node.FlowNodeResult, nextNodeLen)
		for _, v := range res.NextNodeID {
			nextNode, ok := req.NodeMap[v]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", v)
			}
			wg.Add(1)
			go func(nextNode node.FlowNode) {
				defer wg.Done()

				statusLogFunc(node.NodeStatusRunning, v)
				res := nextNode.RunSync(ctx, req)
				if res.Err != nil {
					statusLogFunc(node.NodeStatusFailed, v)
				} else {
					statusLogFunc(node.NodeStatusSuccess, v)
				}
				parallel <- node.FlowNodeResult{
					NextNodeID: res.NextNodeID,
					Err:        res.Err,
				}
			}(nextNode)
		}

		wg.Wait()
		close(parallel)

		mergedNextNodeIDs := make(map[idwrap.IDWrap]bool)
		for a := range parallel {
			if a.Err != nil {
				return nil, a.Err
			}
			for _, nextID := range a.NextNodeID {
				mergedNextNodeIDs[nextID] = true
			}
		}

		for nextID := range mergedNextNodeIDs {
			nextNode, ok := req.NodeMap[nextID]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", nextID)
			}
			queue = append(queue, nextNode)
		}
	}

	return nextNodeID, nil
}
