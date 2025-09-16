package flowlocalrunner

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/tracking"
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
	predecessorMap := buildPredecessorMap(r.EdgesMap)

	flowStatusChan <- runner.FlowStatusStarting
	if r.Timeout == 0 {
		err = RunNodeSync(ctx, *nextNodeID, req, logWorkaround, predecessorMap)
	} else {
		err = RunNodeASync(ctx, *nextNodeID, req, logWorkaround, predecessorMap)
	}

	if err != nil {
		flowStatusChan <- runner.FlowStatusFailed
	} else {
		flowStatusChan <- runner.FlowStatusSuccess
	}
	return err
}

type processResult struct {
	originalID      idwrap.IDWrap
	executionID     idwrap.IDWrap
	nextNodes       []idwrap.IDWrap
	err             error
	inputData       map[string]any
	outputData      map[string]any // NEW: From tracker.GetWrittenVars()
	skipFinalStatus bool           // From FlowNodeResult.SkipFinalStatus
}

func processNode(ctx context.Context, n node.FlowNode, req *node.FlowNodeRequest,
) node.FlowNodeResult {
	return n.RunSync(ctx, req)
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

func buildPredecessorMap(edgesMap edge.EdgesMap) map[idwrap.IDWrap][]idwrap.IDWrap {
	predecessors := make(map[idwrap.IDWrap][]idwrap.IDWrap, len(edgesMap))
	for sourceID, edges := range edgesMap {
		for _, targets := range edges {
			for _, targetID := range targets {
				predecessors[targetID] = append(predecessors[targetID], sourceID)
			}
		}
	}
	return predecessors
}

func RunNodeSync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
	predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	var status runner.FlowNodeStatus
	var processCount int
	// Mutex to protect PendingAtmoicMap from concurrent access
	var pendingMapMutex sync.Mutex
	// Track nodes that have been sent RUNNING status but haven't completed
	// Map from executionID to the full status for proper state transitions
	runningNodes := make(map[idwrap.IDWrap]runner.FlowNodeStatus)
	runningNodesMutex := sync.Mutex{}
	// Track start times for duration calculation
	nodeStartTimes := make(map[idwrap.IDWrap]time.Time)

	// Cleanup function to send CANCELED status for all running/queued nodes
	sendCanceledStatuses := func(cancelErr error) {
		// Send CANCELED status for any nodes still in RUNNING state
		runningNodesMutex.Lock()
		for execID, runningStatus := range runningNodes {
			// Calculate actual duration if we have a start time
			duration := time.Duration(0)
			if startTime, ok := nodeStartTimes[execID]; ok {
				duration = time.Since(startTime)
			}

			canceledStatus := runner.FlowNodeStatus{
				ExecutionID:      execID,
				NodeID:           runningStatus.NodeID,
				Name:             runningStatus.Name,
				State:            mnnode.NODE_STATE_CANCELED,
				Error:            cancelErr,
				IterationContext: runningStatus.IterationContext,
				RunDuration:      duration,
			}
			statusLogFunc(canceledStatus)
		}
		// Clear the maps after sending all canceled statuses
		runningNodes = make(map[idwrap.IDWrap]runner.FlowNodeStatus)
		nodeStartTimes = make(map[idwrap.IDWrap]time.Time)
		runningNodesMutex.Unlock()

		// Send CANCELED status for any nodes still in the queue
		for _, nodeID := range queue {
			if node, ok := req.NodeMap[nodeID]; ok {
				canceledStatus := runner.FlowNodeStatus{
					ExecutionID:      idwrap.NewNow(),
					NodeID:           nodeID,
					Name:             node.GetName(),
					State:            mnnode.NODE_STATE_CANCELED,
					Error:            cancelErr,
					IterationContext: req.IterationContext,
				}
				statusLogFunc(canceledStatus)
			}
		}
	}

	// Ensure we send canceled statuses on any return path
	defer func() {
		if ctx.Err() != nil {
			sendCanceledStatuses(ctx.Err())
		}
	}()

	for queueLen := len(queue); queueLen != 0; queueLen = len(queue) {
		// Check if context was cancelled before processing next batch
		if ctx.Err() != nil {
			return ctx.Err()
		}

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
			currentNode, ok := req.NodeMap[flowNodeId]
			if !ok {
				return fmt.Errorf("node not found: %v", currentNode)
			}
			nodeStateMap[flowNodeId] = FlowNodeStatusLocal{StartTime: time.Now()}
			go func(nodeID idwrap.IDWrap) {
				defer wg.Done()

				// Wait for all predecessors to complete before reading their output
				// This prevents race conditions where we read before predecessors finish writing
				predecessors := predecessorMap[nodeID]

				// For each predecessor, wait until its variable is available
				inputData := make(map[string]any)
				for _, predID := range predecessors {
					if predNode, ok := req.NodeMap[predID]; ok {
						predName := predNode.GetName()

						// Retry reading with backoff to handle race conditions
						var predData interface{}
						var err error
						maxRetries := 10 // Max ~1ms total wait
						for retry := 0; retry < maxRetries; retry++ {
							predData, err = node.ReadVarRaw(req, predName)
							if err == nil {
								break
							}
							// Very short wait before retry to allow predecessor to complete
							time.Sleep(100 * time.Microsecond)
						}

						// Only add to inputData if we successfully read the predecessor data
						if err == nil {
							inputData[predName] = predData
						}
					}
				}

				// Generate execution ID right before processing
				executionID := idwrap.NewNow()

				// Log RUNNING status with execution ID
				runningStatus := runner.FlowNodeStatus{
					ExecutionID:      executionID,
					NodeID:           nodeID,
					Name:             currentNode.GetName(),
					State:            mnnode.NODE_STATE_RUNNING,
					Error:            nil,
					IterationContext: req.IterationContext,
				}
				statusLogFunc(runningStatus)

				// Track this node as running with its start time
				runningNodesMutex.Lock()
				runningNodes[executionID] = runningStatus
				nodeStartTimes[executionID] = time.Now()
				runningNodesMutex.Unlock()

				// Create a copy of the request for this execution to avoid race conditions
				// This ensures each goroutine has its own tracker and execution ID
				nodeReq := *req // Shallow copy of the request struct

				// Initialize tracker for this node execution
				tracker := tracking.NewVariableTracker()
				nodeReq.VariableTracker = tracker

				// Set the execution ID in the copied request
				nodeReq.ExecutionID = executionID

				result := processNode(FlowNodeCancelCtx, currentNode, &nodeReq)

				// Capture tracked data as tree structures
				outputData := tracker.GetWrittenVarsAsTree()

				// Merge tracked variable reads as tree structure into inputData
				trackedReads := tracker.GetReadVarsAsTree()
				if len(trackedReads) > 0 {
					// Merge the tracked reads into inputData without wrapping
					inputData = tracking.MergeTreesPreferFirst(inputData, trackedReads)
				}

				resultChan <- processResult{
					originalID:      currentNode.GetID(),
					executionID:     executionID,
					nextNodes:       result.NextNodeID,
					err:             result.Err,
					inputData:       inputData,
					outputData:      outputData,
					skipFinalStatus: result.SkipFinalStatus,
				}
			}(flowNodeId)
		}

		wg.Wait()

		close(resultChan)

		var lastNodeError error
		for result := range resultChan {
			status.NodeID = result.originalID
			status.ExecutionID = result.executionID
			currentNode := req.NodeMap[result.originalID]
			status.Name = currentNode.GetName()
			status.IterationContext = req.IterationContext
			nodeState := nodeStateMap[status.NodeID]
			status.RunDuration = time.Since(nodeState.StartTime)

			// Remove from running nodes since we're processing its completion
			runningNodesMutex.Lock()
			delete(runningNodes, result.executionID)
			delete(nodeStartTimes, result.executionID)
			runningNodesMutex.Unlock()

			// Prefer node-specific result error over global cancellation status.
			// If the node returned an error, report it first; only mark as canceled
			// due to global context cancellation when there is no node error.
			if result.err != nil {
				if runner.IsCancellationError(result.err) {
					status.State = mnnode.NODE_STATE_CANCELED
				} else {
					status.State = mnnode.NODE_STATE_FAILURE
				}
				status.Error = result.err
				statusLogFunc(status)
				lastNodeError = result.err
				// Trigger cancellation for remaining nodes after reporting this failure
				FlowNodeCancelCtxCancel()
				continue
			}

			if FlowNodeCancelCtx.Err() != nil {
				status.State = mnnode.NODE_STATE_CANCELED
				status.Error = FlowNodeCancelCtx.Err()
				// Capture tracked input/output data even for canceled nodes
				// This ensures we show what data was read/written before cancellation
				status.InputData = node.DeepCopyValue(result.inputData)
				status.OutputData = node.DeepCopyValue(result.outputData)
				statusLogFunc(status)
				// Remove from running nodes since we've sent the CANCELED status
				runningNodesMutex.Lock()
				delete(runningNodes, result.executionID)
				delete(nodeStartTimes, result.executionID)
				runningNodesMutex.Unlock()
				continue
			}

			// All nodes should report SUCCESS when they complete successfully
			// Loop nodes handle their own iteration tracking internally
			// FOR/FOREACH nodes set skipFinalStatus to avoid creating empty main execution
			if !result.skipFinalStatus {
				status.State = mnnode.NODE_STATE_SUCCESS
				status.Error = nil
				// Use the tracked output data which has the proper tree structure
				status.OutputData = node.DeepCopyValue(result.outputData)
				// Deep copy input data as well
				status.InputData = node.DeepCopyValue(result.inputData)
				statusLogFunc(status)
			}

			for _, id := range result.nextNodes {
				pendingMapMutex.Lock()
				i, ok := req.PendingAtmoicMap[id]
				if !ok || i == 1 {
					pendingMapMutex.Unlock()
					queue = append(queue, id)
				} else {
					req.PendingAtmoicMap[id] = i - 1
					pendingMapMutex.Unlock()
				}
			}
		}

		if lastNodeError != nil {
			return lastNodeError
		}

		// Check if flow was canceled - the defer will handle sending CANCELED statuses
		if FlowNodeCancelCtx.Err() != nil {
			return FlowNodeCancelCtx.Err()
		}

		// remove from queue
		queue = queue[processCount:]
	}

	return nil
}

// RunNodeASync runs nodes with timeout handling
func RunNodeASync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
	predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	var status runner.FlowNodeStatus
	var processCount int
	// Mutex to protect PendingAtmoicMap from concurrent access
	var pendingMapMutex sync.Mutex
	// Track nodes that have been sent RUNNING status but haven't completed
	// Map from executionID to the full status for proper state transitions
	runningNodes := make(map[idwrap.IDWrap]runner.FlowNodeStatus)
	runningNodesMutex := sync.Mutex{}
	// Track start times for duration calculation
	nodeStartTimes := make(map[idwrap.IDWrap]time.Time)

	// Cleanup function to send CANCELED status for all running/queued nodes
	sendCanceledStatuses := func(cancelErr error) {
		// Send CANCELED status for any nodes still in RUNNING state
		runningNodesMutex.Lock()
		for execID, runningStatus := range runningNodes {
			// Calculate actual duration if we have a start time
			duration := time.Duration(0)
			if startTime, ok := nodeStartTimes[execID]; ok {
				duration = time.Since(startTime)
			}

			canceledStatus := runner.FlowNodeStatus{
				ExecutionID:      execID,
				NodeID:           runningStatus.NodeID,
				Name:             runningStatus.Name,
				State:            mnnode.NODE_STATE_CANCELED,
				Error:            cancelErr,
				IterationContext: runningStatus.IterationContext,
				RunDuration:      duration,
			}
			statusLogFunc(canceledStatus)
		}
		// Clear the maps after sending all canceled statuses
		runningNodes = make(map[idwrap.IDWrap]runner.FlowNodeStatus)
		nodeStartTimes = make(map[idwrap.IDWrap]time.Time)
		runningNodesMutex.Unlock()

		// Send CANCELED status for any nodes still in the queue
		for _, nodeID := range queue {
			if node, ok := req.NodeMap[nodeID]; ok {
				canceledStatus := runner.FlowNodeStatus{
					ExecutionID:      idwrap.NewNow(),
					NodeID:           nodeID,
					Name:             node.GetName(),
					State:            mnnode.NODE_STATE_CANCELED,
					Error:            cancelErr,
					IterationContext: req.IterationContext,
				}
				statusLogFunc(canceledStatus)
			}
		}
	}

	// Ensure we send canceled statuses on any return path
	defer func() {
		if ctx.Err() != nil {
			sendCanceledStatuses(ctx.Err())
		}
	}()

	for queueLen := len(queue); queueLen != 0; queueLen = len(queue) {
		// Check if context was cancelled before processing next batch
		if ctx.Err() != nil {
			return ctx.Err()
		}

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

			timeStart[id] = time.Now()

			go func(nodeID idwrap.IDWrap) {
				defer wg.Done()

				// Wait for all predecessors to complete before reading their output
				// This prevents race conditions where we read before predecessors finish writing
				predecessors := predecessorMap[nodeID]

				// For each predecessor, wait until its variable is available
				inputData := make(map[string]any)
				for _, predID := range predecessors {
					if predNode, ok := req.NodeMap[predID]; ok {
						predName := predNode.GetName()

						// Retry reading with backoff to handle race conditions
						var predData interface{}
						var err error
						maxRetries := 10 // Max ~1ms total wait
						for retry := 0; retry < maxRetries; retry++ {
							predData, err = node.ReadVarRaw(req, predName)
							if err == nil {
								break
							}
							// Very short wait before retry to allow predecessor to complete
							time.Sleep(100 * time.Microsecond)
						}

						// Only add to inputData if we successfully read the predecessor data
						if err == nil {
							inputData[predName] = predData
						}
					}
				}

				// Generate execution ID right before processing
				executionID := idwrap.NewNow()

				// Log RUNNING status with execution ID
				runningStatus := runner.FlowNodeStatus{
					ExecutionID:      executionID,
					NodeID:           nodeID,
					Name:             currentNode.GetName(),
					State:            mnnode.NODE_STATE_RUNNING,
					Error:            nil,
					IterationContext: req.IterationContext,
				}
				statusLogFunc(runningStatus)

				// Track this node as running with its start time
				runningNodesMutex.Lock()
				runningNodes[executionID] = runningStatus
				nodeStartTimes[executionID] = time.Now()
				runningNodesMutex.Unlock()

				// Create a copy of the request for this execution to avoid race conditions
				// This ensures each goroutine has its own tracker and execution ID
				nodeReq := *req // Shallow copy of the request struct

				// Initialize tracker for this node execution
				tracker := tracking.NewVariableTracker()
				nodeReq.VariableTracker = tracker

				// Set the execution ID in the copied request
				nodeReq.ExecutionID = executionID

				result := processNode(FlowNodeCancelCtx, currentNode, &nodeReq)

				// Always capture tracked data and send result, even if context timed out
				// This ensures nodes don't get stuck in RUNNING state
				outputData := tracker.GetWrittenVarsAsTree()

				// Merge tracked variable reads as tree structure into inputData
				trackedReads := tracker.GetReadVarsAsTree()
				if len(trackedReads) > 0 {
					// Merge the tracked reads into inputData without wrapping
					inputData = tracking.MergeTreesPreferFirst(inputData, trackedReads)
				}

				// If context timed out after node execution, mark it as an error
				if ctxTimed.Err() != nil && result.Err == nil {
					result.Err = ctxTimed.Err()
				}

				resultChan <- processResult{
					originalID:      currentNode.GetID(),
					executionID:     executionID,
					nextNodes:       result.NextNodeID,
					err:             result.Err,
					inputData:       inputData,
					outputData:      outputData,
					skipFinalStatus: result.SkipFinalStatus,
				}
			}(id)
		}

		waitCh := make(chan struct{}, 1)
		go func() {
			wg.Wait()
			close(waitCh)
		}()

		// Wait for all goroutines to complete or timeout
		timedOut := false
		select {
		case <-ctxTimed.Done():
			timedOut = true
			<-waitCh // Wait for goroutines to finish sending their results
		case <-waitCh:
		}

		close(resultChan)
		queue = queue[processCount:]

		var lastNodeError error
		for result := range resultChan {
			status.NodeID = result.originalID
			status.ExecutionID = result.executionID
			currentNode := req.NodeMap[result.originalID]
			status.Name = currentNode.GetName()
			status.IterationContext = req.IterationContext
			status.RunDuration = time.Since(timeStart[status.NodeID])

			// Remove from running nodes since we're processing its completion
			runningNodesMutex.Lock()
			delete(runningNodes, result.executionID)
			delete(nodeStartTimes, result.executionID)
			runningNodesMutex.Unlock()

			// Prefer node-specific result error over global cancellation status.
			// If the node returned an error, report it first; only mark as canceled
			// due to global context cancellation when there is no node error.
			if result.err != nil {
				if runner.IsCancellationError(result.err) {
					status.State = mnnode.NODE_STATE_CANCELED
				} else {
					status.State = mnnode.NODE_STATE_FAILURE
				}
				status.Error = result.err
				statusLogFunc(status)
				lastNodeError = result.err
				// Trigger cancellation for remaining nodes after reporting this failure
				FlowNodeCancelCtxCancelFn()
				continue
			}

			if FlowNodeCancelCtx.Err() != nil {
				status.State = mnnode.NODE_STATE_CANCELED
				status.Error = FlowNodeCancelCtx.Err()
				// Capture tracked input/output data even for canceled nodes
				// This ensures we show what data was read/written before cancellation
				status.InputData = node.DeepCopyValue(result.inputData)
				status.OutputData = node.DeepCopyValue(result.outputData)
				statusLogFunc(status)
				// Remove from running nodes since we've sent the CANCELED status
				runningNodesMutex.Lock()
				delete(runningNodes, result.executionID)
				delete(nodeStartTimes, result.executionID)
				runningNodesMutex.Unlock()
				continue
			}
			// All nodes should report SUCCESS when they complete successfully
			// Loop nodes handle their own iteration tracking internally
			// FOR/FOREACH nodes set skipFinalStatus to avoid creating empty main execution
			if !result.skipFinalStatus {
				status.State = mnnode.NODE_STATE_SUCCESS
				status.Error = nil
				// Use the tracked output data which has the proper tree structure
				status.OutputData = node.DeepCopyValue(result.outputData)
				// Deep copy input data as well
				status.InputData = node.DeepCopyValue(result.inputData)
				statusLogFunc(status)
			}

			for _, id := range result.nextNodes {
				pendingMapMutex.Lock()
				i, ok := req.PendingAtmoicMap[id]
				if !ok || i == 1 {
					pendingMapMutex.Unlock()
					queue = append(queue, id)
				} else {
					req.PendingAtmoicMap[id] = i - 1
					pendingMapMutex.Unlock()
				}
			}
		}

		if lastNodeError != nil {
			return lastNodeError
		}

		// Check if flow was canceled - the defer will handle sending CANCELED statuses
		if FlowNodeCancelCtx.Err() != nil {
			return FlowNodeCancelCtx.Err()
		}

		// If we timed out but no specific node error, return the timeout error
		// The defer will handle sending CANCELED statuses
		if timedOut {
			return ctxTimed.Err()
		}
	}

	return nil
}
