package runner

import (
	"sync"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeExecution identifies a single node execution for status tracking.
type NodeExecution struct {
	ExecutionID idwrap.IDWrap
	NodeID      idwrap.IDWrap
	Name        string
	StartTime   time.Time
	IterCtx     *IterationContext
}

// StatusEmitter tracks running nodes and emits status events.
// It delegates the actual event delivery to an emit function, making it
// usable with both channel-based delivery (RunWithEvents) and callback-based
// delivery (RunNodeSync from loop nodes).
//
// For a remote runner, the same StatusEmitter works — only the emitFn changes.
type StatusEmitter struct {
	emitFn func(FlowNodeStatus)

	mu      sync.Mutex
	running map[idwrap.IDWrap]runningEntry
}

type runningEntry struct {
	nodeID    idwrap.IDWrap
	name      string
	startTime time.Time
	iterCtx   *IterationContext
}

// NewStatusEmitter creates an emitter that delegates to the given function.
func NewStatusEmitter(emitFn func(FlowNodeStatus)) *StatusEmitter {
	return &StatusEmitter{
		emitFn:  emitFn,
		running: make(map[idwrap.IDWrap]runningEntry),
	}
}

// Emit sends a status event. This is used as the LogPushFunc callback for nodes.
func (se *StatusEmitter) Emit(status FlowNodeStatus) {
	se.emitFn(status)
}

// EmitRunning atomically registers a node as running and emits RUNNING status.
// This eliminates the race where cancellation could miss a node between
// status emission and registration.
func (se *StatusEmitter) EmitRunning(exec NodeExecution) {
	se.mu.Lock()
	se.running[exec.ExecutionID] = runningEntry{
		nodeID:    exec.NodeID,
		name:      exec.Name,
		startTime: exec.StartTime,
		iterCtx:   exec.IterCtx,
	}
	se.mu.Unlock()

	se.emitFn(FlowNodeStatus{
		ExecutionID:      exec.ExecutionID,
		NodeID:           exec.NodeID,
		Name:             exec.Name,
		State:            mflow.NODE_STATE_RUNNING,
		IterationContext: exec.IterCtx,
	})
}

// EmitTerminal deregisters the node and emits a pre-built terminal status.
// The wasRunning guard prevents double emission: if CancelAllRunning already
// processed this node (cleared from running map), emission is skipped.
func (se *StatusEmitter) EmitTerminal(executionID idwrap.IDWrap, status FlowNodeStatus, skip bool) {
	se.mu.Lock()
	_, wasRunning := se.running[executionID]
	delete(se.running, executionID)
	se.mu.Unlock()

	if skip || !wasRunning {
		return
	}
	se.emitFn(status)
}

// Deregister removes a node from running tracking without emitting a status.
// Used when the caller handles status emission itself (e.g., with custom state
// logic for errors, timeouts, or loop coordinators).
func (se *StatusEmitter) Deregister(executionID idwrap.IDWrap) {
	se.mu.Lock()
	delete(se.running, executionID)
	se.mu.Unlock()
}

// CancelAllRunning emits CANCELED for every node currently tracked as RUNNING.
// Called during context cancellation cleanup.
func (se *StatusEmitter) CancelAllRunning(cancelErr error) {
	se.mu.Lock()
	entries := make(map[idwrap.IDWrap]runningEntry, len(se.running))
	for k, v := range se.running {
		entries[k] = v
	}
	se.running = make(map[idwrap.IDWrap]runningEntry)
	se.mu.Unlock()

	for execID, entry := range entries {
		se.emitFn(FlowNodeStatus{
			ExecutionID:      execID,
			NodeID:           entry.nodeID,
			Name:             entry.name,
			State:            mflow.NODE_STATE_CANCELED,
			Error:            cancelErr,
			RunDuration:      time.Since(entry.startTime),
			IterationContext: entry.iterCtx,
		})
	}
}

// NewChannelEmitFunc creates an emit function that routes status events to
// FlowEventChannels. RUNNING events go to NodeStates only; terminal events
// go to both NodeStates and NodeLogs.
// Safe to call after channels are closed (recovers from send-on-closed-channel).
func NewChannelEmitFunc(channels FlowEventChannels) func(FlowNodeStatus) {
	return func(status FlowNodeStatus) {
		// Background goroutines (e.g., WebSocket message readers) may try to emit
		// after RunWithEvents closes channels. Recover rather than crashing.
		defer func() { recover() }() //nolint:errcheck // intentional panic recovery

		targets := FlowNodeEventTargetState
		if status.State != mflow.NODE_STATE_RUNNING {
			targets |= FlowNodeEventTargetLog
		}
		event := FlowNodeEvent{
			Status:  status,
			Targets: targets,
		}
		if event.ShouldSend(FlowNodeEventTargetState) && channels.NodeStates != nil {
			channels.NodeStates <- event.Status
		}
		if event.ShouldSend(FlowNodeEventTargetLog) && channels.NodeLogs != nil {
			channels.NodeLogs <- FlowNodeLogPayload{
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
		}
	}
}
