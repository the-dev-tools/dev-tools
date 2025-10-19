package testing

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

func TestStatusCollector_BasicCapture(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_RUNNING,
	}

	// Capture a status
	collector.Capture(status)

	// Verify it was captured
	allStatuses := collector.GetAll()
	if len(allStatuses) != 1 {
		t.Fatalf("Expected 1 status, got %d", len(allStatuses))
	}

	if allStatuses[0].Status.NodeID != nodeID {
		t.Errorf("Expected node ID %s, got %s", nodeID.String(), allStatuses[0].Status.NodeID.String())
	}

	if allStatuses[0].Status.State != mnnode.NODE_STATE_RUNNING {
		t.Errorf("Expected state RUNNING, got %s", mnnode.StringNodeState(allStatuses[0].Status.State))
	}
}

func TestStatusCollector_ConcurrentCapture(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Capture from multiple goroutines
	const numGoroutines = 10
	const numStatusesPerGoroutine = 5

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < numStatusesPerGoroutine; j++ {
				status := runner.FlowNodeStatus{
					ExecutionID: executionID,
					NodeID:      nodeID,
					Name:        "test-node",
					State:       mnnode.NODE_STATE_RUNNING,
				}
				collector.Capture(status)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all statuses were captured
	allStatuses := collector.GetAll()
	expectedCount := numGoroutines * numStatusesPerGoroutine
	if len(allStatuses) != expectedCount {
		t.Fatalf("Expected %d statuses, got %d", expectedCount, len(allStatuses))
	}
}

func TestStatusCollector_FilterByNodeID(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID1 := idwrap.NewNow()
	nodeID2 := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Capture statuses for different nodes
	status1 := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID1,
		Name:        "node-1",
		State:       mnnode.NODE_STATE_RUNNING,
	}
	status2 := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID2,
		Name:        "node-2",
		State:       mnnode.NODE_STATE_SUCCESS,
	}

	collector.Capture(status1)
	collector.Capture(status2)

	// Filter by node ID 1
	filtered := collector.GetByNodeID(nodeID1)
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 status for node ID 1, got %d", len(filtered))
	}
	if filtered[0].Status.NodeID != nodeID1 {
		t.Errorf("Expected node ID %s, got %s", nodeID1.String(), filtered[0].Status.NodeID.String())
	}

	// Filter by node ID 2
	filtered = collector.GetByNodeID(nodeID2)
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 status for node ID 2, got %d", len(filtered))
	}
	if filtered[0].Status.NodeID != nodeID2 {
		t.Errorf("Expected node ID %s, got %s", nodeID2.String(), filtered[0].Status.NodeID.String())
	}
}

func TestStatusCollector_FilterByState(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Capture statuses with different states
	statuses := []runner.FlowNodeStatus{
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_RUNNING,
		},
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_SUCCESS,
		},
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_FAILURE,
		},
	}

	for _, status := range statuses {
		collector.Capture(status)
	}

	// Test filtering by each state
	running := collector.GetByState(mnnode.NODE_STATE_RUNNING)
	if len(running) != 1 {
		t.Fatalf("Expected 1 RUNNING status, got %d", len(running))
	}

	success := collector.GetByState(mnnode.NODE_STATE_SUCCESS)
	if len(success) != 1 {
		t.Fatalf("Expected 1 SUCCESS status, got %d", len(success))
	}

	failure := collector.GetByState(mnnode.NODE_STATE_FAILURE)
	if len(failure) != 1 {
		t.Fatalf("Expected 1 FAILURE status, got %d", len(failure))
	}

	// Test count by state
	counts := collector.CountByState()
	if counts[mnnode.NODE_STATE_RUNNING] != 1 {
		t.Errorf("Expected 1 RUNNING count, got %d", counts[mnnode.NODE_STATE_RUNNING])
	}
	if counts[mnnode.NODE_STATE_SUCCESS] != 1 {
		t.Errorf("Expected 1 SUCCESS count, got %d", counts[mnnode.NODE_STATE_SUCCESS])
	}
	if counts[mnnode.NODE_STATE_FAILURE] != 1 {
		t.Errorf("Expected 1 FAILURE count, got %d", counts[mnnode.NODE_STATE_FAILURE])
	}
}

func TestStatusCollector_ComplexFilter(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	loopNodeID := idwrap.NewNow()

	// Create iteration context
	iterationContext := &runner.IterationContext{
		IterationPath:  []int{0, 1},
		ExecutionIndex: 1,
		ParentNodes:    []idwrap.IDWrap{loopNodeID},
	}

	// Capture various statuses
	baseTime := time.Now().UTC()
	statuses := []runner.FlowNodeStatus{
		{
			ExecutionID:    executionID,
			NodeID:         nodeID,
			Name:           "test-node",
			State:          mnnode.NODE_STATE_RUNNING,
			IterationEvent: false,
			LoopNodeID:     idwrap.IDWrap{},
		},
		{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             "test-node",
			State:            mnnode.NODE_STATE_SUCCESS,
			IterationEvent:   true,
			IterationContext: iterationContext,
			LoopNodeID:       loopNodeID,
		},
	}

	// Add small delays to ensure different timestamps
	for i, status := range statuses {
		if i > 0 {
			time.Sleep(1 * time.Millisecond)
		}
		collector.Capture(status)
	}

	// Test iteration events filter
	iterationEvents := collector.GetIterationEvents()
	if len(iterationEvents) != 1 {
		t.Fatalf("Expected 1 iteration event, got %d", len(iterationEvents))
	}
	if !iterationEvents[0].Status.IterationEvent {
		t.Error("Expected iteration event to be true")
	}

	// Test complex filter with multiple criteria
	minTime := baseTime.Add(500 * time.Microsecond) // After first status
	filter := StatusFilter{
		NodeID:       &nodeID,
		MinTimestamp: &minTime,
	}

	filtered := collector.Filter(filter)
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 status with complex filter, got %d", len(filtered))
	}
	if filtered[0].Status.State != mnnode.NODE_STATE_SUCCESS {
		t.Errorf("Expected SUCCESS status, got %s", mnnode.StringNodeState(filtered[0].Status.State))
	}
}

func TestStatusCollector_WaitForStatus(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Test waiting for a status that doesn't exist yet
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	filter := StatusFilter{
		NodeID: &nodeID,
		State:  &[]mnnode.NodeState{mnnode.NODE_STATE_SUCCESS}[0],
	}

	// Should timeout
	_, err := collector.WaitForStatus(ctx, filter)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}

	// Add the status
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_SUCCESS,
	}
	collector.Capture(status)

	// Now it should find the status immediately
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	result, err := collector.WaitForStatus(ctx2, filter)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Status.State != mnnode.NODE_STATE_SUCCESS {
		t.Errorf("Expected SUCCESS state, got %s", mnnode.StringNodeState(result.Status.State))
	}
}

func TestStatusCollector_GetLast(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Test empty collector
	last := collector.GetLast()
	if last != nil {
		t.Error("Expected nil for empty collector")
	}

	// Add statuses
	status1 := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_RUNNING,
	}
	status2 := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_SUCCESS,
	}

	collector.Capture(status1)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	collector.Capture(status2)

	// Get last status
	last = collector.GetLast()
	if last == nil {
		t.Fatal("Expected last status, got nil")
	}
	if last.Status.State != mnnode.NODE_STATE_SUCCESS {
		t.Errorf("Expected SUCCESS state, got %s", mnnode.StringNodeState(last.Status.State))
	}

	// Test GetLastByNodeID
	lastByNode := collector.GetLastByNodeID(nodeID)
	if lastByNode == nil {
		t.Fatal("Expected last status by node ID, got nil")
	}
	if lastByNode.Status.State != mnnode.NODE_STATE_SUCCESS {
		t.Errorf("Expected SUCCESS state, got %s", mnnode.StringNodeState(lastByNode.Status.State))
	}
}

func TestStatusCollector_ClearAndClose(t *testing.T) {
	collector := NewStatusCollector()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Add a status
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_RUNNING,
	}
	collector.Capture(status)

	// Verify it was added
	if collector.Count() != 1 {
		t.Errorf("Expected count 1, got %d", collector.Count())
	}

	// Clear
	collector.Clear()
	if collector.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", collector.Count())
	}
	if collector.IsClosed() {
		t.Error("Expected collector to be open after clear")
	}

	// Add another status
	collector.Capture(status)
	if collector.Count() != 1 {
		t.Errorf("Expected count 1 after adding to cleared collector, got %d", collector.Count())
	}

	// Close
	collector.Close()
	if !collector.IsClosed() {
		t.Error("Expected collector to be closed")
	}

	// Try to add status after close (should be ignored)
	collector.Capture(status)
	if collector.Count() != 1 {
		t.Errorf("Expected count to remain 1 after close, got %d", collector.Count())
	}
}

func TestStatusCollector_CaptureFromFunc(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Get the capture function
	captureFunc := collector.CaptureFromFunc()

	// Use it as a LogPushFunc
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_RUNNING,
	}

	captureFunc(status)

	// Verify it was captured
	if collector.Count() != 1 {
		t.Errorf("Expected count 1, got %d", collector.Count())
	}

	allStatuses := collector.GetAll()
	if allStatuses[0].Status.NodeID != nodeID {
		t.Errorf("Expected node ID %s, got %s", nodeID.String(), allStatuses[0].Status.NodeID.String())
	}
}
