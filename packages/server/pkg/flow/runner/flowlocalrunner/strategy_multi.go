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
	cfg RunConfig, executor *LocalExecutor, tracker *runner.ConvergenceTracker,
) error {
	nodeSignals := &sync.Map{}
	seenNodes := &sync.Map{}

	// Semaphore for bounded processing concurrency
	sem := make(chan struct{}, cfg.MaxConcurrency)
	resultChan := make(chan processResult, cfg.MaxConcurrency)

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
			if predecessors := cfg.PredecessorMap[nodeID]; len(predecessors) > 0 {
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
			cfg.Emitter.EmitRunning(runner.NodeExecution{
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
			if cfg.Timeout > 0 {
				if _, isLoop := currentNode.(node.LoopCoordinator); !isLoop {
					nodeCtx, cancelNode = context.WithTimeout(flowCtx, cfg.Timeout)
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
			cfg.Emitter.CancelAllRunning(ctx.Err())
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

			// Build base status (shared across all terminal paths)
			base := runner.FlowNodeStatus{
				NodeID:           result.originalID,
				ExecutionID:      result.executionID,
				Name:             nodeName,
				IterationContext: req.IterationContext,
				RunDuration:      time.Since(result.startTime),
				AuxiliaryID:      result.AuxiliaryID,
			}

			// ERROR PATH
			if result.err != nil {
				status := buildTerminalStatus(base, result.err, result.timedOut, result.inputData, result.outputData, req, cfg.TrackData)
				cfg.Emitter.EmitTerminal(result.executionID, status, false)

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
					status := buildTerminalStatus(base, flowCtx.Err(), false, result.inputData, result.outputData, req, cfg.TrackData)
					cfg.Emitter.EmitTerminal(result.executionID, status, false)
					continue
				}
				// LoopCoordinator exemption: fall through to success path.
				// EmitTerminal handles deregistration — calling Deregister here
				// would clear wasRunning and suppress the success emission.
			}

			// SUCCESS PATH
			status := buildTerminalStatus(base, nil, false, result.inputData, result.outputData, req, cfg.TrackData)
			cfg.Emitter.EmitTerminal(result.executionID, status, result.skipFinalStatus)

			// Dispatch ready successors immediately
			if firstErr == nil {
				for _, nextID := range result.nextNodes {
					if !tracker.Arrive(nextID) {
						continue
					}
					launchNode(nextID)
				}
			}

		case <-ctx.Done():
			flowCancel()
			// Send CANCELED status for all currently running nodes
			// BEFORE draining, since drain removes goroutines from tracking.
			cfg.Emitter.CancelAllRunning(ctx.Err())
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
