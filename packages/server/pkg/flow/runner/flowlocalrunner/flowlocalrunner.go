//nolint:revive // exported
package flowlocalrunner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
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

	EdgesMap     mflow.EdgesMap
	StartNodeIDs []idwrap.IDWrap
	Timeout      time.Duration

	mode               ExecutionMode
	selectedMode       ExecutionMode
	enableDataTracking bool
	logger             *slog.Logger
}

var _ runner.FlowRunner = (*FlowLocalRunner)(nil)

func CreateFlowRunner(id, flowID idwrap.IDWrap, startNodeIDs []idwrap.IDWrap, flowNodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap mflow.EdgesMap, timeout time.Duration, logger *slog.Logger) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:                 id,
		FlowID:             flowID,
		StartNodeIDs:       startNodeIDs,
		FlowNodeMap:        flowNodeMap,
		PendingAtmoicMap:   make(map[idwrap.IDWrap]uint32),
		EdgesMap:           edgesMap,
		Timeout:            timeout,
		mode:               ExecutionModeAuto,
		selectedMode:       ExecutionModeMulti,
		enableDataTracking: true,
		logger:             logger,
	}
}

type nodeStatusEmitter struct {
	channels runner.FlowEventChannels
}

func newNodeStatusEmitter(channels runner.FlowEventChannels) *nodeStatusEmitter {
	return &nodeStatusEmitter{channels: channels}
}

func (e *nodeStatusEmitter) emit(status runner.FlowNodeStatus) {
	targets := runner.FlowNodeEventTargetState
	if status.State != mflow.NODE_STATE_RUNNING {
		targets |= runner.FlowNodeEventTargetLog
	}
	e.emitWithTargets(status, targets)
}

func (e *nodeStatusEmitter) emitWithTargets(status runner.FlowNodeStatus, targets runner.FlowNodeEventTarget) {
	if e == nil {
		return
	}
	// Background goroutines (e.g., WebSocket message readers) may try to emit
	// after RunWithEvents closes channels. Recover rather than crashing.
	defer func() { recover() }() //nolint:errcheck // intentional panic recovery
	event := runner.FlowNodeEvent{
		Status:  status,
		Targets: targets,
	}
	if event.ShouldSend(runner.FlowNodeEventTargetState) && e.channels.NodeStates != nil {
		e.channels.NodeStates <- event.Status
	}
	if event.ShouldSend(runner.FlowNodeEventTargetLog) && e.channels.NodeLogs != nil {
		payload := runner.FlowNodeLogPayload{
			ExecutionID:      status.ExecutionID,
			NodeID:           status.NodeID,
			Name:             status.Name,
			State:            status.State,
			Error:            status.Error,
			OutputData:       status.OutputData,
			RunDuration:      status.RunDuration,
			IterationContext: status.IterationContext,
			IterationEvent:   status.IterationEvent,
			IterationIndex:   status.IterationIndex,
			LoopNodeID:       status.LoopNodeID,
		}
		e.channels.NodeLogs <- payload
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

func selectExecutionMode(nodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap mflow.EdgesMap) ExecutionMode {
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
			if handle == mflow.HandleLoop {
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
		if _, ok := handles[mflow.HandleLoop]; ok && nonLoopTargets > 0 {
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
	default:
		return runNodesMultiEventDriven(ctx, startNodeID, req, statusLogFunc, predecessorMap, timeout, trackData)
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

func flattenNodeOutput(nodeName string, output any) any {
	if nodeName == "" || output == nil {
		return output
	}
	m, ok := output.(map[string]any)
	if !ok {
		return output
	}
	nested, ok := m[nodeName]
	if !ok {
		return output
	}
	nestedMap, ok := nested.(map[string]any)
	if !ok {
		return output
	}
	delete(m, nodeName)
	for k, v := range nestedMap {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}

func sendQueuedCancellationStatuses(queue []idwrap.IDWrap, req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc, cancelErr error) {
	for _, nodeID := range queue {
		if nodeRef, ok := req.NodeMap[nodeID]; ok {
			statusLogFunc(runner.FlowNodeStatus{
				ExecutionID:      idwrap.NewMonotonic(),
				NodeID:           nodeID,
				Name:             nodeRef.GetName(),
				State:            mflow.NODE_STATE_CANCELED,
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

		executionID := idwrap.NewMonotonic()
		runningStatus := runner.FlowNodeStatus{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             currentNode.GetName(),
			State:            mflow.NODE_STATE_RUNNING,
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
			nodeCtx, cancelNodeCtx = context.WithTimeout(ctx, timeout)
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
			AuxiliaryID:      result.AuxiliaryID,
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
				status.State = mflow.NODE_STATE_CANCELED
			} else {
				status.State = mflow.NODE_STATE_FAILURE
			}
			status.Error = result.Err
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			status.OutputData = flattenNodeOutput(status.Name, status.OutputData)
			statusLogFunc(status)
			return result.Err
		}

		if nodeCtxErr != nil {
			status.State = mflow.NODE_STATE_CANCELED
			status.Error = nodeCtxErr
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			status.OutputData = flattenNodeOutput(status.Name, status.OutputData)
			statusLogFunc(status)
			return nodeCtxErr
		}

		if !result.SkipFinalStatus {
			status.State = mflow.NODE_STATE_SUCCESS
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			status.OutputData = flattenNodeOutput(status.Name, status.OutputData)
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
	return r.RunWithEvents(ctx, runner.LegacyFlowEventChannels(flowNodeStatusChan, flowStatusChan), baseVars)
}

func (r *FlowLocalRunner) RunWithEvents(ctx context.Context, channels runner.FlowEventChannels, baseVars map[string]any) error {
	// Cancel context before closing channels (LIFO order) so background
	// goroutines (e.g., WebSocket readers) get the stop signal first.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if channels.NodeStates != nil {
		defer close(channels.NodeStates)
	}
	if channels.NodeLogs != nil {
		defer close(channels.NodeLogs)
	}
	if channels.FlowStatus != nil {
		defer close(channels.FlowStatus)
	}

	flowEdgeDepCounter := make(map[idwrap.IDWrap]uint32)
	for _, v := range r.EdgesMap {
		for _, targetIDs := range v {
			for _, targetID := range targetIDs {
				current, ok := flowEdgeDepCounter[targetID]
				if !ok {
					flowEdgeDepCounter[targetID] = 0
				}
				flowEdgeDepCounter[targetID] = current + 1
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

	statusEmitter := newNodeStatusEmitter(channels)
	statusFunc := node.LogPushFunc(func(runner.FlowNodeStatus) {})
	if channels.NodeStates != nil || channels.NodeLogs != nil {
		statusFunc = node.LogPushFunc(statusEmitter.emit)
	}

	// Shared mutex for PendingAtmoicMap across concurrent entry chains
	pendingMu := &sync.Mutex{}

	req := &node.FlowNodeRequest{
		VarMap:           baseVars,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          r.FlowNodeMap,
		EdgeSourceMap:    r.EdgesMap,
		LogPushFunc:      statusFunc,
		Timeout:          r.Timeout,
		PendingAtmoicMap: pendingAtmoicMap,
		PendingMapMu:     pendingMu,
		Logger:           r.logger,
	}
	predecessorMap := BuildPredecessorMap(r.EdgesMap)

	mode := r.mode
	if mode == ExecutionModeAuto {
		mode = selectExecutionMode(r.FlowNodeMap, r.EdgesMap)
	}
	r.selectedMode = mode

	if channels.FlowStatus != nil {
		channels.FlowStatus <- runner.FlowStatusStarting
	}

	var err error
	if len(r.StartNodeIDs) == 1 {
		// Single entry — fast path, no errgroup overhead
		err = runNodes(ctx, r.StartNodeIDs[0], req, statusFunc, predecessorMap, mode, r.Timeout, r.enableDataTracking)
	} else {
		// Multiple entries — run each chain concurrently
		eg, egCtx := errgroup.WithContext(ctx)
		for _, startID := range r.StartNodeIDs {
			eg.Go(func() error {
				return runNodes(egCtx, startID, req, statusFunc, predecessorMap, mode, r.Timeout, r.enableDataTracking)
			})
		}
		err = eg.Wait()
	}

	if channels.FlowStatus != nil {
		if err != nil {
			channels.FlowStatus <- runner.FlowStatusFailed
		} else {
			channels.FlowStatus <- runner.FlowStatusSuccess
		}
	}
	return err
}

type processResult struct {
	originalID      idwrap.IDWrap
	executionID     idwrap.IDWrap
	nextNodes       []idwrap.IDWrap
	err             error
	inputData       map[string]any
	outputData      map[string]any
	skipFinalStatus bool // From FlowNodeResult.SkipFinalStatus
	AuxiliaryID     *idwrap.IDWrap
	startTime       time.Time // When the node started executing
	timedOut        bool      // Whether the node hit a per-node deadline
}

func processNode(ctx context.Context, n node.FlowNode, req *node.FlowNodeRequest,
) node.FlowNodeResult {
	return n.RunSync(ctx, req)
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

// SetGoroutineCountForTesting overrides the goroutine count for testing.
// Returns a cleanup function that restores the original value.
func SetGoroutineCountForTesting(n int) func() {
	old := goroutineCount
	goroutineCount = n
	return func() { goroutineCount = old }
}

func BuildPredecessorMap(edgesMap mflow.EdgesMap) map[idwrap.IDWrap][]idwrap.IDWrap {
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

// runningEntry tracks a node that has been sent RUNNING status.
type runningEntry struct {
	status    runner.FlowNodeStatus
	startTime time.Time
}

// runNodesMultiEventDriven executes nodes concurrently using an event-driven model.
// Unlike the previous batch model that waited for all nodes in a batch to complete,
// this dispatches successors immediately when a node finishes. Only converge points
// (nodes with multiple incoming edges) wait for all predecessors.
func runNodesMultiEventDriven(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
	predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	timeout time.Duration,
	trackData bool,
) error {
	// Use shared mutex from request if available (multi-entry), otherwise local
	pendingMapMu := req.PendingMapMu
	if pendingMapMu == nil {
		pendingMapMu = &sync.Mutex{}
	}

	nodeSignals := &sync.Map{}
	seenNodes := &sync.Map{}

	// Running node tracking for cancellation cleanup
	runningNodes := make(map[idwrap.IDWrap]runningEntry)
	var runningMu sync.Mutex

	// Semaphore for bounded processing concurrency
	sem := make(chan struct{}, goroutineCount)
	resultChan := make(chan processResult, goroutineCount)

	// Cancellation context for the entire execution
	flowCtx, flowCancel := context.WithCancel(ctx)
	defer flowCancel()

	var outstanding int64

	// launchNode spawns a goroutine to execute a node. It never blocks the caller.
	launchNode := func(nodeID idwrap.IDWrap) {
		currentNode, ok := req.NodeMap[nodeID]
		if !ok {
			atomic.AddInt64(&outstanding, 1)
			go func() {
				resultChan <- processResult{
					originalID: nodeID,
					err:        fmt.Errorf("node not found: %v", nodeID),
				}
			}()
			return
		}

		seenNodes.Store(nodeID, struct{}{})
		atomic.AddInt64(&outstanding, 1)

		go func() {
			// Phase 1: Wait for predecessors OUTSIDE the semaphore.
			// This prevents deadlocks where goroutines hold semaphore slots
			// while waiting for predecessors that need slots to finish.
			if predecessors := predecessorMap[nodeID]; len(predecessors) > 0 {
				if err := waitForPredecessors(flowCtx, nodeSignals, seenNodes, predecessors); err != nil {
					resultChan <- processResult{
						originalID: currentNode.GetID(),
						err:        err,
					}
					return
				}
			}

			// Phase 2: Acquire semaphore to bound active processing concurrency.
			select {
			case sem <- struct{}{}:
			case <-flowCtx.Done():
				resultChan <- processResult{
					originalID: currentNode.GetID(),
					err:        flowCtx.Err(),
				}
				return
			}
			defer func() { <-sem }()

			// Generate execution ID right before processing
			executionID := idwrap.NewMonotonic()
			startTime := time.Now()

			// Emit RUNNING status
			runningStatus := runner.FlowNodeStatus{
				ExecutionID:      executionID,
				NodeID:           nodeID,
				Name:             currentNode.GetName(),
				State:            mflow.NODE_STATE_RUNNING,
				IterationContext: req.IterationContext,
			}
			statusLogFunc(runningStatus)

			runningMu.Lock()
			runningNodes[executionID] = runningEntry{status: runningStatus, startTime: startTime}
			runningMu.Unlock()

			// Create per-node request copy with tracker
			nodeReq := *req
			var tracker *tracking.VariableTracker
			if trackData {
				tracker = trackerPool.Get().(*tracking.VariableTracker)
				tracker.Reset()
				nodeReq.VariableTracker = tracker
			}
			nodeReq.ExecutionID = executionID

			// Per-node timeout (skip for LoopCoordinator nodes)
			nodeCtx := flowCtx
			var cancelNode context.CancelFunc
			if timeout > 0 {
				if _, isLoop := currentNode.(node.LoopCoordinator); !isLoop {
					nodeCtx, cancelNode = context.WithTimeout(flowCtx, timeout)
				}
			}
			if cancelNode != nil {
				defer cancelNode()
			}

			// Execute the node
			result := processNode(nodeCtx, currentNode, &nodeReq)

			// Capture tracked data
			var outputData, inputData map[string]any
			if tracker != nil {
				outputData = tracker.GetWrittenVarsAsTree()
				trackedReads := tracker.GetReadVarsAsTree()
				if len(trackedReads) > 0 {
					inputData = trackedReads
				}
				tracker.Reset()
				trackerPool.Put(tracker)
			}

			// Check for node-level timeout
			nodeTimedOut := false
			if result.Err == nil && nodeCtx.Err() != nil && errors.Is(nodeCtx.Err(), context.DeadlineExceeded) {
				result.Err = nodeCtx.Err()
				nodeTimedOut = true
			}

			resultChan <- processResult{
				originalID:      currentNode.GetID(),
				executionID:     executionID,
				nextNodes:       result.NextNodeID,
				err:             result.Err,
				inputData:       inputData,
				outputData:      outputData,
				skipFinalStatus: result.SkipFinalStatus,
				AuxiliaryID:     result.AuxiliaryID,
				startTime:       startTime,
				timedOut:        nodeTimedOut,
			}
		}()
	}

	// Cancellation cleanup: send CANCELED status for running nodes
	sendCanceledStatuses := func(cancelErr error) {
		runningMu.Lock()
		for execID, entry := range runningNodes {
			statusLogFunc(runner.FlowNodeStatus{
				ExecutionID:      execID,
				NodeID:           entry.status.NodeID,
				Name:             entry.status.Name,
				State:            mflow.NODE_STATE_CANCELED,
				Error:            cancelErr,
				IterationContext: entry.status.IterationContext,
				RunDuration:      time.Since(entry.startTime),
			})
		}
		runningNodes = make(map[idwrap.IDWrap]runningEntry)
		runningMu.Unlock()
	}

	defer func() {
		if ctx.Err() != nil {
			sendCanceledStatuses(ctx.Err())
		}
	}()

	// Launch start node
	launchNode(startNodeID)

	// Main event loop: process results as they arrive
	var firstErr error

	for atomic.LoadInt64(&outstanding) > 0 {
		select {
		case result := <-resultChan:
			atomic.AddInt64(&outstanding, -1)

			currentNode := req.NodeMap[result.originalID]
			nodeName := ""
			if currentNode != nil {
				nodeName = currentNode.GetName()
			}

			// Signal that this node completed (unblocks nodes waiting for it)
			signalNodeComplete(nodeSignals, result.originalID)

			// Remove from running tracking
			runningMu.Lock()
			delete(runningNodes, result.executionID)
			runningMu.Unlock()

			// Build status
			status := runner.FlowNodeStatus{
				NodeID:           result.originalID,
				ExecutionID:      result.executionID,
				Name:             nodeName,
				IterationContext: req.IterationContext,
				RunDuration:      time.Since(result.startTime),
				AuxiliaryID:      result.AuxiliaryID,
			}

			// ERROR PATH
			if result.err != nil {
				switch {
				case result.timedOut || errors.Is(result.err, context.DeadlineExceeded):
					status.State = mflow.NODE_STATE_FAILURE
				case runner.IsCancellationError(result.err):
					status.State = mflow.NODE_STATE_CANCELED
				default:
					status.State = mflow.NODE_STATE_FAILURE
				}
				status.Error = result.err

				if trackData {
					if result.outputData != nil {
						status.OutputData = node.DeepCopyValue(result.outputData)
					} else {
						status.OutputData = collectSingleModeOutput(req, nodeName)
					}
					if result.inputData != nil {
						status.InputData = node.DeepCopyValue(result.inputData)
					}
				}
				status.OutputData = flattenNodeOutput(nodeName, status.OutputData)
				statusLogFunc(status)

				if firstErr == nil {
					firstErr = result.err
				}
				// Cancel all other in-flight work
				flowCancel()
				continue
			}

			// Check if flow was already canceled (by another node's failure)
			if flowCtx.Err() != nil {
				isLoop := false
				if currentNode != nil {
					_, isLoop = currentNode.(node.LoopCoordinator)
				}
				if !isLoop {
					status.State = mflow.NODE_STATE_CANCELED
					status.Error = flowCtx.Err()
					if trackData {
						if result.inputData != nil {
							status.InputData = node.DeepCopyValue(result.inputData)
						}
						if result.outputData != nil {
							status.OutputData = node.DeepCopyValue(result.outputData)
						}
					}
					status.OutputData = flattenNodeOutput(nodeName, status.OutputData)
					statusLogFunc(status)
					continue
				}
			}

			// SUCCESS PATH
			if !result.skipFinalStatus {
				status.State = mflow.NODE_STATE_SUCCESS
				status.Error = nil
				if trackData {
					if result.outputData != nil {
						status.OutputData = node.DeepCopyValue(result.outputData)
					}
					if result.inputData != nil {
						status.InputData = node.DeepCopyValue(result.inputData)
					}
				}
				status.OutputData = flattenNodeOutput(nodeName, status.OutputData)
				statusLogFunc(status)
			}

			// Dispatch ready successors immediately
			if firstErr == nil {
				for _, nextID := range result.nextNodes {
					pendingMapMu.Lock()
					remaining, hasPending := req.PendingAtmoicMap[nextID]
					if !hasPending || remaining <= 1 {
						if hasPending {
							delete(req.PendingAtmoicMap, nextID)
						}
						pendingMapMu.Unlock()
						launchNode(nextID)
					} else {
						req.PendingAtmoicMap[nextID] = remaining - 1
						pendingMapMu.Unlock()
					}
				}
			}

		case <-ctx.Done():
			flowCancel()
			// Drain remaining results so goroutines can exit
			for atomic.LoadInt64(&outstanding) > 0 {
				result := <-resultChan
				atomic.AddInt64(&outstanding, -1)
				signalNodeComplete(nodeSignals, result.originalID)
				runningMu.Lock()
				delete(runningNodes, result.executionID)
				runningMu.Unlock()
			}
			if firstErr != nil {
				return firstErr
			}
			return ctx.Err()
		}
	}

	return firstErr
}
