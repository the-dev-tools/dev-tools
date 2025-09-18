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

// ExecutionMode controls how FlowLocalRunner schedules nodes.
type ExecutionMode int

const (
	ExecutionModeAuto ExecutionMode = iota
	ExecutionModeSingle
	ExecutionModeMulti
)

type FlowLocalRunner struct {
	ID               idwrap.IDWrap
	FlowID           idwrap.IDWrap
	FlowNodeMap      map[idwrap.IDWrap]node.FlowNode
	PendingAtmoicMap map[idwrap.IDWrap]uint32

	EdgesMap    edge.EdgesMap
	StartNodeID idwrap.IDWrap
	Timeout     time.Duration

	mode               ExecutionMode
	selectedMode       ExecutionMode
	enableDataTracking bool
}

func CreateFlowRunner(id, flowID, StartNodeID idwrap.IDWrap, FlowNodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap edge.EdgesMap, timeout time.Duration) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:                 id,
		FlowID:             flowID,
		StartNodeID:        StartNodeID,
		FlowNodeMap:        FlowNodeMap,
		PendingAtmoicMap:   make(map[idwrap.IDWrap]uint32),
		EdgesMap:           edgesMap,
		Timeout:            timeout,
		mode:               ExecutionModeAuto,
		selectedMode:       ExecutionModeMulti,
		enableDataTracking: true,
	}
}

// SetExecutionMode overrides the default Auto mode for the next run.
func (r *FlowLocalRunner) SetExecutionMode(mode ExecutionMode) {
	if mode < ExecutionModeAuto || mode > ExecutionModeMulti {
		mode = ExecutionModeAuto
	}
	r.mode = mode
}

// SelectedMode reports the effective mode used during the last Run invocation.
func (r *FlowLocalRunner) SelectedMode() ExecutionMode {
	return r.selectedMode
}

// SetDataTrackingEnabled toggles variable tracking during execution.
func (r *FlowLocalRunner) SetDataTrackingEnabled(enabled bool) {
	r.enableDataTracking = enabled
}

func selectExecutionMode(nodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap edge.EdgesMap) ExecutionMode {
	nodeCount := len(nodeMap)
	if nodeCount == 0 {
		return ExecutionModeSingle
	}

	const smallFlowThreshold = 6

	simpleStructure := true
	incomingNonLoop := make(map[idwrap.IDWrap]int)

	for sourceID, handles := range edgesMap {
		nonLoopTargets := 0
		for handle, targetIDs := range handles {
			if len(targetIDs) == 0 {
				continue
			}
			if handle == edge.HandleLoop {
				if len(targetIDs) > 1 {
					simpleStructure = false
				}
				continue
			}

			nonLoopTargets += len(targetIDs)
			if len(targetIDs) > 1 {
				simpleStructure = false
			}
			for _, targetID := range targetIDs {
				incomingNonLoop[targetID]++
			}
		}
		if nonLoopTargets > 1 {
			simpleStructure = false
		}
		if _, ok := handles[edge.HandleLoop]; ok && nonLoopTargets > 0 {
			// Loop node with additional branch work beyond the loop/then path
			if nonLoopTargets > 1 {
				simpleStructure = false
			}
		}

		if _, exists := nodeMap[sourceID]; !exists {
			// Node present in edges but missing from map; treat as complex and bail out
			simpleStructure = false
		}
	}

	for targetID, count := range incomingNonLoop {
		if count > 1 {
			simpleStructure = false
			break
		}
		if _, exists := nodeMap[targetID]; !exists {
			simpleStructure = false
		}
	}

	if nodeCount <= smallFlowThreshold && simpleStructure {
		return ExecutionModeSingle
	}

	return ExecutionModeMulti
}

func runNodes(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	mode ExecutionMode, timeout time.Duration, trackData bool,
) error {
	switch mode {
	case ExecutionModeSingle:
		return runNodesSingle(ctx, startNodeID, req, statusLogFunc, predecessorMap, timeout, trackData)
	case ExecutionModeMulti:
		if timeout == 0 {
			return runNodesMultiNoTimeout(ctx, startNodeID, req, statusLogFunc, predecessorMap, trackData)
		}
		return runNodesMultiWithTimeout(ctx, startNodeID, req, statusLogFunc, predecessorMap, timeout, trackData)
	default:
		if timeout == 0 {
			return runNodesMultiNoTimeout(ctx, startNodeID, req, statusLogFunc, predecessorMap, trackData)
		}
		return runNodesMultiWithTimeout(ctx, startNodeID, req, statusLogFunc, predecessorMap, timeout, trackData)
	}
}

func gatherSingleModeInputData(req *node.FlowNodeRequest, predecessorIDs []idwrap.IDWrap) map[string]any {
	if len(predecessorIDs) == 0 {
		return nil
	}

	inputs := make(map[string]any, len(predecessorIDs))
	for _, predID := range predecessorIDs {
		predNode, ok := req.NodeMap[predID]
		if !ok {
			continue
		}
		predName := predNode.GetName()
		if predName == "" {
			continue
		}
		if data, err := node.ReadVarRaw(req, predName); err == nil {
			inputs[predName] = node.DeepCopyValue(data)
		}
	}

	if len(inputs) == 0 {
		return nil
	}
	return inputs
}

func collectSingleModeOutput(req *node.FlowNodeRequest, nodeName string) any {
	if nodeName == "" {
		return nil
	}
	if data, err := node.ReadVarRaw(req, nodeName); err == nil {
		return node.DeepCopyValue(data)
	}
	return nil
}

func sendQueuedCancellationStatuses(queue []idwrap.IDWrap, req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc, cancelErr error) {
	for _, nodeID := range queue {
		if nodeRef, ok := req.NodeMap[nodeID]; ok {
			statusLogFunc(runner.FlowNodeStatus{
				ExecutionID:      idwrap.NewNow(),
				NodeID:           nodeID,
				Name:             nodeRef.GetName(),
				State:            mnnode.NODE_STATE_CANCELED,
				Error:            cancelErr,
				IterationContext: req.IterationContext,
			})
		}
	}
}

func runNodesSingle(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	timeout time.Duration, trackData bool,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	for len(queue) > 0 {
		if ctx.Err() != nil {
			sendQueuedCancellationStatuses(queue, req, statusLogFunc, ctx.Err())
			return ctx.Err()
		}

		nodeID := queue[0]
		queue = queue[1:]

		currentNode, ok := req.NodeMap[nodeID]
		if !ok {
			return fmt.Errorf("node not found: %v", nodeID)
		}

		var inputData map[string]any
		if trackData {
			inputData = gatherSingleModeInputData(req, predecessorMap[nodeID])
		}

		executionID := idwrap.NewNow()
		runningStatus := runner.FlowNodeStatus{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             currentNode.GetName(),
			State:            mnnode.NODE_STATE_RUNNING,
			IterationContext: req.IterationContext,
		}
		statusLogFunc(runningStatus)

		nodeReq := *req
		nodeReq.ExecutionID = executionID
		var tracker *tracking.VariableTracker
		if trackData {
			tracker = trackerPool.Get().(*tracking.VariableTracker)
			tracker.Reset()
			nodeReq.VariableTracker = tracker
		} else {
			nodeReq.VariableTracker = nil
		}

		nodeCtx := ctx
		cancelNodeCtx := func() {}
		if timeout > 0 {
			nodeCtx, cancelNodeCtx = context.WithDeadline(ctx, time.Now().Add(timeout))
		}
		startTime := time.Now()

		result := processNode(nodeCtx, currentNode, &nodeReq)

		var (
			trackedOutput map[string]any
			trackedInput  map[string]any
		)
		if tracker != nil {
			trackedOutput = tracker.GetWrittenVarsAsTree()
			reads := tracker.GetReadVarsAsTree()
			if len(reads) > 0 {
				trackedInput = reads
			}
			tracker.Reset()
			trackerPool.Put(tracker)
		}
		nodeCtxErr := nodeCtx.Err()
		cancelNodeCtx()

		status := runner.FlowNodeStatus{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             currentNode.GetName(),
			IterationContext: req.IterationContext,
			RunDuration:      time.Since(startTime),
		}

		if trackData {
			if len(trackedInput) > 0 {
				status.InputData = node.DeepCopyValue(trackedInput)
			} else if len(inputData) > 0 {
				status.InputData = node.DeepCopyValue(inputData)
			}
		}

		if result.Err != nil {
			if runner.IsCancellationError(result.Err) {
				status.State = mnnode.NODE_STATE_CANCELED
			} else {
				status.State = mnnode.NODE_STATE_FAILURE
			}
			status.Error = result.Err
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			statusLogFunc(status)
			return result.Err
		}

		if nodeCtxErr != nil {
			status.State = mnnode.NODE_STATE_CANCELED
			status.Error = nodeCtxErr
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			statusLogFunc(status)
			return nodeCtxErr
		}

		if !result.SkipFinalStatus {
			status.State = mnnode.NODE_STATE_SUCCESS
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			statusLogFunc(status)
		}

		for _, nextID := range result.NextNodeID {
			if remaining, ok := req.PendingAtmoicMap[nextID]; ok && remaining > 1 {
				req.PendingAtmoicMap[nextID] = remaining - 1
				continue
			}
			queue = append(queue, nextID)
		}
	}

	return nil
}

// RunNodeSync retains the legacy behaviour for packages that directly invoke the runner.
func RunNodeSync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
) error {
	return runNodes(ctx, startNodeID, req, statusLogFunc, predecessorMap, ExecutionModeMulti, 0, true)
}

// RunNodeASync retains the legacy behaviour for packages that directly invoke the runner with timeouts.
func RunNodeASync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
) error {
	return runNodes(ctx, startNodeID, req, statusLogFunc, predecessorMap, ExecutionModeMulti, req.Timeout, true)
}

func (r *FlowLocalRunner) Run(ctx context.Context, flowNodeStatusChan chan runner.FlowNodeStatus, flowStatusChan chan runner.FlowStatus, baseVars map[string]any) error {
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
	predecessorMap := BuildPredecessorMap(r.EdgesMap)

	mode := r.mode
	if mode == ExecutionModeAuto {
		mode = selectExecutionMode(r.FlowNodeMap, r.EdgesMap)
	}
	r.selectedMode = mode

	flowStatusChan <- runner.FlowStatusStarting
	err = runNodes(ctx, *nextNodeID, req, logWorkaround, predecessorMap, mode, r.Timeout, r.enableDataTracking)

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

type nodeSignal struct {
	once sync.Once
	ch   chan struct{}
}

func acquireNodeSignal(signals *sync.Map, id idwrap.IDWrap) *nodeSignal {
	val, _ := signals.LoadOrStore(id, &nodeSignal{ch: make(chan struct{})})
	return val.(*nodeSignal)
}

func waitForPredecessors(ctx context.Context, signals, seen *sync.Map, predecessors []idwrap.IDWrap) error {
	for _, predID := range predecessors {
		if seen != nil {
			if _, ok := seen.Load(predID); !ok {
				continue
			}
		}
		sig := acquireNodeSignal(signals, predID)
		select {
		case <-sig.ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func signalNodeComplete(signals *sync.Map, id idwrap.IDWrap) {
	sig := acquireNodeSignal(signals, id)
	sig.once.Do(func() {
		close(sig.ch)
	})
}

func MaxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}

var (
	goroutineCount int = MaxParallelism()
	trackerPool        = sync.Pool{New: func() any { return tracking.NewVariableTracker() }}
)

func BuildPredecessorMap(edgesMap edge.EdgesMap) map[idwrap.IDWrap][]idwrap.IDWrap {
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

func runNodesMultiNoTimeout(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
	predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	trackData bool,
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

	nodeSignals := &sync.Map{}
	seenNodes := &sync.Map{}
	seenNodes.Store(startNodeID, struct{}{})

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
		for _, flowNodeID := range subqueue {
			currentNode, ok := req.NodeMap[flowNodeID]
			if !ok {
				return fmt.Errorf("node not found: %v", currentNode)
			}
			nodeStateMap[flowNodeID] = FlowNodeStatusLocal{StartTime: time.Now()}
			seenNodes.Store(flowNodeID, struct{}{})
			go func(nodeID idwrap.IDWrap) {
				defer wg.Done()

				if predecessors := predecessorMap[nodeID]; len(predecessors) > 0 {
					if err := waitForPredecessors(FlowNodeCancelCtx, nodeSignals, seenNodes, predecessors); err != nil {
						resultChan <- processResult{
							originalID:  currentNode.GetID(),
							executionID: idwrap.IDWrap{},
							err:         err,
						}
						return
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

				var tracker *tracking.VariableTracker
				if trackData {
					tracker = trackerPool.Get().(*tracking.VariableTracker)
					tracker.Reset()
					nodeReq.VariableTracker = tracker
				}

				// Set the execution ID in the copied request
				nodeReq.ExecutionID = executionID

				result := processNode(FlowNodeCancelCtx, currentNode, &nodeReq)

				// Capture tracked data as tree structures when enabled
				var (
					outputData map[string]any
					inputData  map[string]any
				)
				if tracker != nil {
					outputData = tracker.GetWrittenVarsAsTree()
					trackedReads := tracker.GetReadVarsAsTree()
					if len(trackedReads) > 0 {
						inputData = trackedReads
					}
					tracker.Reset()
					trackerPool.Put(tracker)
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
			}(flowNodeID)
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
			status.InputData = nil
			status.OutputData = nil
			signalNodeComplete(nodeSignals, result.originalID)

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
				if trackData {
					// Capture tracked input/output data even for canceled nodes
					// This ensures we show what data was read/written before cancellation
					if result.inputData != nil {
						status.InputData = node.DeepCopyValue(result.inputData)
					}
					if result.outputData != nil {
						status.OutputData = node.DeepCopyValue(result.outputData)
					}
				}
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
				if trackData {
					// Use the tracked output data which has the proper tree structure
					if result.outputData != nil {
						status.OutputData = node.DeepCopyValue(result.outputData)
					}
					// Deep copy input data as well
					if result.inputData != nil {
						status.InputData = node.DeepCopyValue(result.inputData)
					}
				}
				statusLogFunc(status)
			}

			for _, id := range result.nextNodes {
				pendingMapMutex.Lock()
				i, ok := req.PendingAtmoicMap[id]
				if !ok || i == 1 {
					pendingMapMutex.Unlock()
					queue = append(queue, id)
					seenNodes.Store(id, struct{}{})
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
func runNodesMultiWithTimeout(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
	predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	timeout time.Duration,
	trackData bool,
) error {
	if timeout <= 0 {
		return runNodesMultiNoTimeout(ctx, startNodeID, req, statusLogFunc, predecessorMap, trackData)
	}

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

	nodeSignals := &sync.Map{}
	seenNodes := &sync.Map{}
	seenNodes.Store(startNodeID, struct{}{})

	for queueLen := len(queue); queueLen != 0; queueLen = len(queue) {
		// Check if context was cancelled before processing next batch
		if ctx.Err() != nil {
			return ctx.Err()
		}

		processCount = min(goroutineCount, queueLen)

		ctxTimed, cancelTimeFn := context.WithDeadline(ctx, time.Now().Add(timeout))
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
			seenNodes.Store(id, struct{}{})

			go func(nodeID idwrap.IDWrap) {
				defer wg.Done()

				if predecessors := predecessorMap[nodeID]; len(predecessors) > 0 {
					if err := waitForPredecessors(FlowNodeCancelCtx, nodeSignals, seenNodes, predecessors); err != nil {
						resultChan <- processResult{
							originalID:  currentNode.GetID(),
							executionID: idwrap.IDWrap{},
							err:         err,
						}
						return
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

				var tracker *tracking.VariableTracker
				if trackData {
					tracker = trackerPool.Get().(*tracking.VariableTracker)
					tracker.Reset()
					nodeReq.VariableTracker = tracker
				}

				// Set the execution ID in the copied request
				nodeReq.ExecutionID = executionID

				result := processNode(FlowNodeCancelCtx, currentNode, &nodeReq)

				// Always capture tracked data and send result, even if context timed out
				// This ensures nodes don't get stuck in RUNNING state
				var (
					outputData map[string]any
					inputData  map[string]any
				)
				if tracker != nil {
					outputData = tracker.GetWrittenVarsAsTree()
					trackedReads := tracker.GetReadVarsAsTree()
					if len(trackedReads) > 0 {
						inputData = trackedReads
					}
					tracker.Reset()
					trackerPool.Put(tracker)
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
			status.InputData = nil
			status.OutputData = nil
			signalNodeComplete(nodeSignals, result.originalID)

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
				if trackData {
					if result.inputData != nil {
						status.InputData = node.DeepCopyValue(result.inputData)
					}
					if result.outputData != nil {
						status.OutputData = node.DeepCopyValue(result.outputData)
					}
				}
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
				if trackData {
					if result.outputData != nil {
						status.OutputData = node.DeepCopyValue(result.outputData)
					}
					if result.inputData != nil {
						status.InputData = node.DeepCopyValue(result.inputData)
					}
				}
				statusLogFunc(status)
			}

			for _, id := range result.nextNodes {
				pendingMapMutex.Lock()
				i, ok := req.PendingAtmoicMap[id]
				if !ok || i == 1 {
					pendingMapMutex.Unlock()
					queue = append(queue, id)
					seenNodes.Store(id, struct{}{})
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
