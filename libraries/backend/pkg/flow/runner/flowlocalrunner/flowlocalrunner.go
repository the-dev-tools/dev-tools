package flowlocalrunner

import (
	"context"
	"fmt"
	"sync"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnnode"
	"time"
)

type FlowLocalRunner struct {
	ID               idwrap.IDWrap
	FlowID           idwrap.IDWrap
	FlowNodeMap      map[idwrap.IDWrap]node.FlowNode
	PendingAtmoicMap map[idwrap.IDWrap]uint32

	EdgesMap    edge.EdgesMap
	StartNodeID idwrap.IDWrap
	Timeout     time.Duration
}

func CreateFlowRunner(id, flowID, StartNodeID idwrap.IDWrap, FlowNodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap edge.EdgesMap, timeout time.Duration) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:               id,
		FlowID:           flowID,
		StartNodeID:      StartNodeID,
		FlowNodeMap:      FlowNodeMap,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
		EdgesMap:         edgesMap,
		Timeout:          timeout,
	}
}

func (r FlowLocalRunner) Run(ctx context.Context, flowNodeStatusChan chan runner.FlowNodeStatus, flowStatusChan chan runner.FlowStatus) error {
	defer close(flowNodeStatusChan)
	defer close(flowStatusChan)
	nextNodeID := &r.StartNodeID
	var err error

	logWorkaround := func(status runner.FlowNodeStatus) {
		flowNodeStatusChan <- status
	}

	flowEdgeDepCounter := make(map[idwrap.IDWrap]uint32)
	for _, v := range r.EdgesMap {
		for _, targetIDs := range v {
			for _, targetID := range targetIDs {
				v, ok := flowEdgeDepCounter[targetID]
				if !ok {
					flowEdgeDepCounter[targetID] = 0
				}
				flowEdgeDepCounter[targetID] = v + 1
			}
		}
	}

	pendingAtmoicMap := make(map[idwrap.IDWrap]uint32)
	for k, v := range flowEdgeDepCounter {
		if v > 1 {
			pendingAtmoicMap[k] = v
		}
	}

	req := &node.FlowNodeRequest{
		VarMap:           map[string]any{},
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          r.FlowNodeMap,
		EdgeSourceMap:    r.EdgesMap,
		LogPushFunc:      node.LogPushFunc(logWorkaround),
		Timeout:          r.Timeout,
		PendingAtmoicMap: pendingAtmoicMap,
	}
	flowStatusChan <- runner.FlowStatusStarting
	if r.Timeout == 0 {
		err = RunNodeSync(ctx, *nextNodeID, req, logWorkaround)
	} else {
		err = RunNodeASync(ctx, *nextNodeID, req, logWorkaround)
	}

	if err != nil {
		flowStatusChan <- runner.FlowStatusFailed
		return err
	}
	flowStatusChan <- runner.FlowStatusSuccess
	return nil
}

type processResult struct {
	originalID idwrap.IDWrap
	nextNodes  []idwrap.IDWrap
	err        error
}

func processNode(ctx context.Context, n node.FlowNode, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) ([]idwrap.IDWrap, error) {
	id := n.GetID()
	status := runner.FlowNodeStatus{
		NodeID: id,
		State:  mnnode.NODE_STATE_RUNNING,
	}
	statusLogFunc(status)

	res := n.RunSync(ctx, req)

	if res.Err != nil {
		status.State = mnnode.NODE_STATE_FAILURE
		statusLogFunc(status)
		return nil, res.Err
	}

	status.State = mnnode.NODE_STATE_SUCCESS
	statusLogFunc(status)
	return res.NextNodeID, nil
}

func RunNodeSync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	var processCount int
	for queueLen := len(queue); queueLen != 0; queueLen = len(queue) {
		processCount = min(goroutineCount, queueLen)

		var wg sync.WaitGroup
		resultChan := make(chan processResult, processCount)
		wg.Add(processCount)
		for i := range processCount {
			currentNode, ok := req.NodeMap[queue[i]]
			if !ok {
				return fmt.Errorf("node not found: %v", currentNode)
			}

			go func() {
				defer wg.Done()
				ids, localErr := processNode(ctx, currentNode, req, statusLogFunc)
				resultChan <- processResult{
					originalID: currentNode.GetID(),
					nextNodes:  ids,
					err:        localErr,
				}
			}()
		}

		wg.Wait()

		close(resultChan)
		queue = queue[processCount:]

		for result := range resultChan {
			if result.err != nil {
				return result.err
			}

			for _, id := range result.nextNodes {
				i, ok := req.PendingAtmoicMap[id]
				if !ok || i == 1 {
					queue = append(queue, id)
				} else {
					req.PendingAtmoicMap[id] = i - 1
				}
			}
		}
	}

	return nil
}

const goroutineCount = 10

// RunNodeASync runs nodes with timeout handling
func RunNodeASync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	var status runner.FlowNodeStatus
	var processCount int
	for queueLen := len(queue); queueLen != 0; queueLen = len(queue) {
		processCount = min(goroutineCount, queueLen)

		ctxTimed, cancelFn := context.WithDeadline(ctx, time.Now().Add(req.Timeout))
		defer cancelFn()

		var wg sync.WaitGroup
		resultChan := make(chan processResult, processCount)
		wg.Add(processCount)
		for i := range processCount {
			id := queue[i]

			currentNode, ok := req.NodeMap[id]
			if !ok {
				return fmt.Errorf("node not found: %v", currentNode)
			}

			status.NodeID = id
			status.State = mnnode.NODE_STATE_RUNNING
			statusLogFunc(status)

			go func() {
				defer wg.Done()
				ids, localErr := processNode(ctxTimed, currentNode, req, statusLogFunc)
				if ctxTimed.Err() != nil {
					return
				}
				resultChan <- processResult{
					originalID: currentNode.GetID(),
					nextNodes:  ids,
					err:        localErr,
				}
			}()
		}

		waitCh := make(chan struct{}, 1)
		go func() {
			wg.Wait()
			close(waitCh)
		}()

		select {
		case <-ctxTimed.Done():
			<-waitCh
			return ctxTimed.Err()
		case <-waitCh:
			cancelFn()
		}

		close(resultChan)
		queue = queue[processCount:]

		for result := range resultChan {
			status.NodeID = result.originalID
			if result.err != nil {
				status.State = mnnode.NODE_STATE_FAILURE
				statusLogFunc(status)
				return result.err
			}
			status.State = mnnode.NODE_STATE_SUCCESS
			outputData, ok := req.VarMap[req.NodeMap[status.NodeID].GetName()]
			if ok {
				status.OutputData = outputData
			}
			statusLogFunc(status)

			for _, id := range result.nextNodes {
				i, ok := req.PendingAtmoicMap[id]
				if !ok || i == 1 {
					queue = append(queue, id)
				} else {
					req.PendingAtmoicMap[id] = i - 1
				}
			}
		}
	}

	return nil
}
