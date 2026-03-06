package flowlocalrunner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

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

// runNodesMultiEventDriven executes nodes concurrently using an event-driven model.
// Unlike the previous batch model that waited for all nodes in a batch to complete,
// this dispatches successors immediately when a node finishes. Only converge points
// (nodes with multiple incoming edges) wait for all predecessors.
func runNodesMultiEventDriven(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
	predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	timeout time.Duration,
	trackData bool,
	emitter *runner.StatusEmitter,
	executor *LocalExecutor,
	maxConcurrency int,
) error {
	// Use shared mutex from request if available (multi-entry), otherwise local
	pendingMapMu := req.PendingMapMu
	if pendingMapMu == nil {
		pendingMapMu = &sync.Mutex{}
	}

	nodeSignals := &sync.Map{}
	seenNodes := &sync.Map{}

	// Semaphore for bounded processing concurrency
	sem := make(chan struct{}, maxConcurrency)
	resultChan := make(chan processResult, maxConcurrency)

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

			// Atomically register + emit RUNNING (fixes race with cancellation)
			emitter.EmitRunning(runner.NodeExecution{
				ExecutionID: executionID,
				NodeID:      nodeID,
				Name:        currentNode.GetName(),
				StartTime:   startTime,
				IterCtx:     req.IterationContext,
			})

			// Create per-node request copy
			nodeReq := *req
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

			// Execute the node with variable tracking
			outcome := executor.Execute(nodeCtx, currentNode, &nodeReq)
			inputData := outcome.TrackedInput
			outputData := outcome.TrackedOutput
			result := outcome.Result

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

	defer func() {
		if ctx.Err() != nil {
			emitter.CancelAllRunning(ctx.Err())
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
			emitter.Deregister(result.executionID)

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
			// Send CANCELED status for all currently running nodes
			// BEFORE draining, since drain removes goroutines from tracking.
			emitter.CancelAllRunning(ctx.Err())
			// Drain remaining results so goroutines can exit
			for atomic.LoadInt64(&outstanding) > 0 {
				result := <-resultChan
				atomic.AddInt64(&outstanding, -1)
				signalNodeComplete(nodeSignals, result.originalID)
			}
			if firstErr != nil {
				return firstErr
			}
			return ctx.Err()
		}
	}

	return firstErr
}
