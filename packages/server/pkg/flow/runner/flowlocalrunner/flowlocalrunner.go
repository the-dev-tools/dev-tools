package flowlocalrunner

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
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

func (r FlowLocalRunner) Run(ctx context.Context, flowNodeStatusChan chan runner.FlowNodeStatus, flowStatusChan chan runner.FlowStatus, baseVars map[string]any) error {
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

	if baseVars == nil {
		baseVars = make(map[string]any)
	}

	req := &node.FlowNodeRequest{
		VarMap:           baseVars,
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
	} else {
		flowStatusChan <- runner.FlowStatusSuccess
	}
	return err
}

type processResult struct {
	originalID idwrap.IDWrap
	nextNodes  []idwrap.IDWrap
	err        error
}

func processNode(ctx context.Context, n node.FlowNode, req *node.FlowNodeRequest,
) ([]idwrap.IDWrap, error) {
	res := n.RunSync(ctx, req)
	return res.NextNodeID, res.Err
}

type FlowNodeStatusLocal struct {
	StartTime time.Time
}

func MaxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}

var goroutineCount int = MaxParallelism()

func RunNodeSync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	var status runner.FlowNodeStatus
	var processCount int
	for queueLen := len(queue); queueLen != 0; queueLen = len(queue) {
		processCount = min(goroutineCount, queueLen)

		var wg sync.WaitGroup
		resultChan := make(chan processResult, processCount)

		// TODO: can be done better
		nodeStateMap := make(map[idwrap.IDWrap]FlowNodeStatusLocal, processCount)

		subqueue := queue[:processCount]

		wg.Add(processCount)
		FlowNodeCancelCtx, FlowNodeCancelCtxCancel := context.WithCancel(ctx)
		defer FlowNodeCancelCtxCancel()
		for _, flowNodeId := range subqueue {

			status.NodeID = flowNodeId
			status.Name = req.NodeMap[flowNodeId].GetName()
			status.State = mnnode.NODE_STATE_RUNNING
			status.Error = nil
			statusLogFunc(status)
			currentNode, ok := req.NodeMap[flowNodeId]
			if !ok {
				return fmt.Errorf("node not found: %v", currentNode)
			}
			nodeStateMap[flowNodeId] = FlowNodeStatusLocal{StartTime: time.Now()}
			go func() {
				defer wg.Done()
				ids, localErr := processNode(FlowNodeCancelCtx, currentNode, req)
				resultChan <- processResult{
					originalID: currentNode.GetID(),
					nextNodes:  ids,
					err:        localErr,
				}
			}()
		}

		wg.Wait()

		close(resultChan)

		var lastNodeError error
		for result := range resultChan {
			status.NodeID = result.originalID
			currentNode := req.NodeMap[result.originalID]
			status.Name = currentNode.GetName()
			nodeState := nodeStateMap[status.NodeID]
			status.RunDuration = time.Since(nodeState.StartTime)
			if FlowNodeCancelCtx.Err() != nil {
				status.State = mnnode.NODE_STATE_CANCELED
				status.Error = FlowNodeCancelCtx.Err()
				statusLogFunc(status)
				continue
			}

			if result.err != nil {
				status.State = mnnode.NODE_STATE_FAILURE
				status.Error = result.err
				statusLogFunc(status)
				lastNodeError = result.err
				FlowNodeCancelCtxCancel()
				continue
			}

			status.State = mnnode.NODE_STATE_SUCCESS
			status.Error = nil
			outputData, err := node.ReadVarRaw(req, status.Name)
			if err == nil {
				status.OutputData = outputData
			} else {
				status.OutputData = nil
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

		if lastNodeError != nil {
			return lastNodeError
		}

		// remove from queue
		queue = queue[processCount:]
	}

	return nil
}

// RunNodeASync runs nodes with timeout handling
func RunNodeASync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	var status runner.FlowNodeStatus
	var processCount int
	for queueLen := len(queue); queueLen != 0; queueLen = len(queue) {
		processCount = min(goroutineCount, queueLen)

		ctxTimed, cancelTimeFn := context.WithDeadline(ctx, time.Now().Add(req.Timeout))
		defer cancelTimeFn()

		var wg sync.WaitGroup
		resultChan := make(chan processResult, processCount)

		// TODO: can be done better
		timeStart := make(map[idwrap.IDWrap]time.Time, processCount)

		wg.Add(processCount)
		FlowNodeCancelCtx, FlowNodeCancelCtxCancelFn := context.WithCancel(ctxTimed)
		defer FlowNodeCancelCtxCancelFn()
		for i := range processCount {
			id := queue[i]

			currentNode, ok := req.NodeMap[id]
			if !ok {
				return fmt.Errorf("node not found: %v", currentNode)
			}

			status.NodeID = id
			status.Name = req.NodeMap[id].GetName()
			status.State = mnnode.NODE_STATE_RUNNING
			status.Error = nil
			statusLogFunc(status)
			timeStart[id] = time.Now()

			go func() {
				defer wg.Done()
				ids, localErr := processNode(FlowNodeCancelCtx, currentNode, req)
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
		}

		close(resultChan)
		queue = queue[processCount:]

		var lastNodeError error
		for result := range resultChan {
			status.NodeID = result.originalID
			currentNode := req.NodeMap[result.originalID]
			status.Name = currentNode.GetName()
			status.RunDuration = time.Since(timeStart[status.NodeID])
			if FlowNodeCancelCtx.Err() != nil {
				status.State = mnnode.NODE_STATE_CANCELED
				status.Error = FlowNodeCancelCtx.Err()
				statusLogFunc(status)
				continue
			}
			if result.err != nil {
				status.State = mnnode.NODE_STATE_FAILURE
				status.Error = result.err
				statusLogFunc(status)
				lastNodeError = result.err
				FlowNodeCancelCtxCancelFn()
				continue
			}
			status.State = mnnode.NODE_STATE_SUCCESS
			status.Error = nil
			outputData, err := node.ReadVarRaw(req, status.Name)
			if err == nil {
				status.OutputData = outputData
			} else {
				status.OutputData = nil
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

		if lastNodeError != nil {
			return lastNodeError
		}
	}

	return nil
}
