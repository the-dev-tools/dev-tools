// Package testing provides foundational testing infrastructure for flow node status validation.
// It enables comprehensive testing of node execution patterns, status transitions, and
// iteration behavior in the flow execution system.
package testing

import (
	"context"
	"sync"
	"time"

	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// TimestampedStatus wraps a FlowNodeStatus with capture timing information.
type TimestampedStatus struct {
	Status    runner.FlowNodeStatus
	Timestamp time.Time
}

// StatusFilter defines criteria for filtering collected statuses.
type StatusFilter struct {
	NodeID        *idwrap.IDWrap
	ExecutionID   *idwrap.IDWrap
	State         *mnnode.NodeState
	LoopNodeID    *idwrap.IDWrap
	MinTimestamp  *time.Time
	MaxTimestamp  *time.Time
	IterationOnly bool // Only include iteration events
}

// StatusCollector captures and provides access to FlowNodeStatus emissions
// with thread-safe concurrent access support.
type StatusCollector struct {
	mu       sync.RWMutex
	statuses []TimestampedStatus
	closed   bool
}

// NewStatusCollector creates a new StatusCollector ready to capture status updates.
func NewStatusCollector() *StatusCollector {
	return &StatusCollector{
		statuses: make([]TimestampedStatus, 0),
	}
}

// Capture records a FlowNodeStatus with the current timestamp.
// This method is safe for concurrent use.
func (sc *StatusCollector) Capture(status runner.FlowNodeStatus) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.closed {
		return
	}

	sc.statuses = append(sc.statuses, TimestampedStatus{
		Status:    status,
		Timestamp: time.Now().UTC(),
	})
}

// CaptureFromFunc creates a LogPushFunc that captures statuses when called.
// This is useful for integrating with existing node runners.
func (sc *StatusCollector) CaptureFromFunc() node.LogPushFunc {
	return func(status runner.FlowNodeStatus) {
		sc.Capture(status)
	}
}

// GetAll returns all captured statuses in chronological order.
// The returned slice is a defensive copy to prevent external modification.
func (sc *StatusCollector) GetAll() []TimestampedStatus {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make([]TimestampedStatus, len(sc.statuses))
	copy(result, sc.statuses)
	return result
}

// Filter returns statuses matching the provided filter criteria.
// Multiple criteria are combined with AND logic.
func (sc *StatusCollector) Filter(filter StatusFilter) []TimestampedStatus {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	var result []TimestampedStatus
	for _, ts := range sc.statuses {
		if !matchesFilter(ts, filter) {
			continue
		}
		result = append(result, ts)
	}

	return result
}

// GetByNodeID returns all statuses for the specified node ID.
func (sc *StatusCollector) GetByNodeID(nodeID idwrap.IDWrap) []TimestampedStatus {
	filter := StatusFilter{NodeID: &nodeID}
	return sc.Filter(filter)
}

// GetByExecutionID returns all statuses for the specified execution ID.
func (sc *StatusCollector) GetByExecutionID(executionID idwrap.IDWrap) []TimestampedStatus {
	filter := StatusFilter{ExecutionID: &executionID}
	return sc.Filter(filter)
}

// GetByState returns all statuses with the specified state.
func (sc *StatusCollector) GetByState(state mnnode.NodeState) []TimestampedStatus {
	filter := StatusFilter{State: &state}
	return sc.Filter(filter)
}

// GetIterationEvents returns all iteration-related statuses.
func (sc *StatusCollector) GetIterationEvents() []TimestampedStatus {
	filter := StatusFilter{IterationOnly: true}
	return sc.Filter(filter)
}

// Count returns the total number of captured statuses.
func (sc *StatusCollector) Count() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.statuses)
}

// CountByState returns the count of statuses for each state.
func (sc *StatusCollector) CountByState() map[mnnode.NodeState]int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	counts := make(map[mnnode.NodeState]int)
	for _, ts := range sc.statuses {
		counts[ts.Status.State]++
	}
	return counts
}

// GetLast returns the most recently captured status, or nil if none exist.
func (sc *StatusCollector) GetLast() *TimestampedStatus {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if len(sc.statuses) == 0 {
		return nil
	}

	// Return a copy to prevent external modification
	last := sc.statuses[len(sc.statuses)-1]
	return &last
}

// GetLastByNodeID returns the most recent status for the specified node ID.
func (sc *StatusCollector) GetLastByNodeID(nodeID idwrap.IDWrap) *TimestampedStatus {
	statuses := sc.GetByNodeID(nodeID)
	if len(statuses) == 0 {
		return nil
	}
	return &statuses[len(statuses)-1]
}

// WaitForStatus waits until a status matching the filter is captured or the context is cancelled.
// Returns the matching status or an error if the context is cancelled.
func (sc *StatusCollector) WaitForStatus(ctx context.Context, filter StatusFilter) (*TimestampedStatus, error) {
	// First check if we already have a matching status
	if existing := sc.Filter(filter); len(existing) > 0 {
		return &existing[len(existing)-1], nil
	}

	// Set up a channel to receive notifications
	notifyCh := make(chan struct{}, 1)

	// Start a goroutine to monitor for new statuses
	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				if matches := sc.Filter(filter); len(matches) > 0 {
					select {
					case notifyCh <- struct{}{}:
					default:
					}
					return
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-notifyCh:
		if matches := sc.Filter(filter); len(matches) > 0 {
			return &matches[len(matches)-1], nil
		}
		return nil, nil // Shouldn't happen, but handle gracefully
	}
}

// Clear removes all captured statuses and resets the collector.
func (sc *StatusCollector) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.statuses = sc.statuses[:0]
	sc.closed = false
}

// Close prevents further status capture and cleans up resources.
func (sc *StatusCollector) Close() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.closed = true
}

// IsClosed returns true if the collector has been closed.
func (sc *StatusCollector) IsClosed() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.closed
}

// matchesFilter checks if a timestamped status matches the provided filter criteria.
func matchesFilter(ts TimestampedStatus, filter StatusFilter) bool {
	status := ts.Status

	// Check node ID filter
	if filter.NodeID != nil && status.NodeID != *filter.NodeID {
		return false
	}

	// Check execution ID filter
	if filter.ExecutionID != nil && status.ExecutionID != *filter.ExecutionID {
		return false
	}

	// Check state filter
	if filter.State != nil && status.State != *filter.State {
		return false
	}

	// Check loop node ID filter
	if filter.LoopNodeID != nil && status.LoopNodeID != *filter.LoopNodeID {
		return false
	}

	// Check iteration event filter
	if filter.IterationOnly && !status.IterationEvent {
		return false
	}

	// Check timestamp range
	if filter.MinTimestamp != nil && ts.Timestamp.Before(*filter.MinTimestamp) {
		return false
	}
	if filter.MaxTimestamp != nil && ts.Timestamp.After(*filter.MaxTimestamp) {
		return false
	}

	return true
}
