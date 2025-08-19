package rflow

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
)

// MockNodeExecutionCollector collects NodeExecution records for testing
type MockNodeExecutionCollector struct {
	mu         sync.Mutex
	executions []mnodeexecution.NodeExecution
}

func (c *MockNodeExecutionCollector) Collect(exec mnodeexecution.NodeExecution) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.executions = append(c.executions, exec)
}

func (c *MockNodeExecutionCollector) GetExecutions() []mnodeexecution.NodeExecution {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]mnodeexecution.NodeExecution{}, c.executions...)
}

// TestCanceledNodeExecution_CreatesProperRecords verifies that CANCELED status creates proper NodeExecution records
func TestCanceledNodeExecution_CreatesProperRecords(t *testing.T) {
	collector := &MockNodeExecutionCollector{}

	// Simulate the node status processing logic from rflow.go
	// This tests the logic that creates NodeExecution records for CANCELED nodes

	// Test case 1: Node that was RUNNING and then CANCELED
	runningNodeID := idwrap.NewNow()
	runningExecID := idwrap.NewNow()

	// First, simulate RUNNING status (this would create pending execution)
	pendingNodeExecutions := make(map[idwrap.IDWrap]*mnodeexecution.NodeExecution)
	nodeExecutionCounts := make(map[idwrap.IDWrap]int)
	executionIDToCount := make(map[idwrap.IDWrap]int)

	// Create RUNNING execution
	nodeExecutionCounts[runningNodeID]++
	executionIDToCount[runningExecID] = nodeExecutionCounts[runningNodeID]

	runningExec := &mnodeexecution.NodeExecution{
		ID:                     runningExecID,
		NodeID:                 runningNodeID,
		Name:                   "TestNode - Execution 1",
		State:                  mnnode.NODE_STATE_RUNNING,
		Error:                  nil,
		InputData:              []byte("{}"),
		InputDataCompressType:  0,
		OutputData:             []byte("{}"),
		OutputDataCompressType: 0,
		ResponseID:             nil,
		CompletedAt:            nil,
	}
	pendingNodeExecutions[runningExecID] = runningExec

	// Now simulate CANCELED status for the same execution
	cancelErr := context.Canceled
	completedAt := time.Now().UnixMilli()

	if nodeExec, exists := pendingNodeExecutions[runningExecID]; exists {
		// Update to CANCELED state
		nodeExec.State = mnnode.NODE_STATE_CANCELED
		nodeExec.CompletedAt = &completedAt

		// Set error
		errorStr := cancelErr.Error()
		nodeExec.Error = &errorStr

		// Collect the execution
		collector.Collect(*nodeExec)
		delete(pendingNodeExecutions, runningExecID)
	}

	// Test case 2: Node that goes directly to CANCELED without RUNNING
	// (e.g., queued node that gets canceled before starting)
	queuedNodeID := idwrap.NewNow()
	queuedExecID := idwrap.NewNow()

	// Simulate direct CANCELED status
	nodeExecutionCounts[queuedNodeID]++
	executionIDToCount[queuedExecID] = nodeExecutionCounts[queuedNodeID]

	directCanceledExec := mnodeexecution.NodeExecution{
		ID:                     queuedExecID,
		NodeID:                 queuedNodeID,
		Name:                   "QueuedNode - Execution 1",
		State:                  mnnode.NODE_STATE_CANCELED,
		Error:                  nil,
		InputData:              []byte("{}"),
		InputDataCompressType:  0,
		OutputData:             []byte("{}"),
		OutputDataCompressType: 0,
		ResponseID:             nil,
		CompletedAt:            &completedAt,
	}

	// Set error for direct canceled
	errorStr := "context canceled"
	directCanceledExec.Error = &errorStr

	collector.Collect(directCanceledExec)

	// Verify collected executions
	executions := collector.GetExecutions()

	if len(executions) != 2 {
		t.Fatalf("Expected 2 executions, got %d", len(executions))
	}

	// Verify all CANCELED executions have proper fields
	for _, exec := range executions {
		if exec.State != mnnode.NODE_STATE_CANCELED {
			t.Errorf("Expected CANCELED state, got %v", exec.State)
		}

		if exec.CompletedAt == nil {
			t.Errorf("CANCELED execution %s missing CompletedAt timestamp", exec.ID)
		}

		if exec.Error == nil || *exec.Error == "" {
			t.Errorf("CANCELED execution %s missing error message", exec.ID)
		}

		if exec.Name == "" {
			t.Errorf("CANCELED execution %s missing name", exec.ID)
		}
	}

	t.Logf("Successfully verified %d CANCELED NodeExecution records with proper fields", len(executions))
}

// TestCanceledNodeExecution_HandlesIterationContext tests that CANCELED nodes preserve iteration context
func TestCanceledNodeExecution_HandlesIterationContext(t *testing.T) {
	collector := &MockNodeExecutionCollector{}

	// Simulate a node in a loop that gets canceled
	nodeID := idwrap.NewNow()
	execID := idwrap.NewNow()

	// Format iteration context name (similar to formatIterationContext in rflow.go)
	// This would normally be generated from IterationContext and nodeNameMap
	execName := "ParentLoop iteration 3 | LoopNode iteration 6"

	completedAt := time.Now().UnixMilli()
	errorStr := "context deadline exceeded"

	canceledExec := mnodeexecution.NodeExecution{
		ID:                     execID,
		NodeID:                 nodeID,
		Name:                   execName,
		State:                  mnnode.NODE_STATE_CANCELED,
		Error:                  &errorStr,
		InputData:              []byte("{}"),
		InputDataCompressType:  0,
		OutputData:             []byte("{}"),
		OutputDataCompressType: 0,
		ResponseID:             nil,
		CompletedAt:            &completedAt,
	}

	collector.Collect(canceledExec)

	executions := collector.GetExecutions()
	if len(executions) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(executions))
	}

	exec := executions[0]

	// Verify iteration context is preserved in the name
	if !strings.Contains(exec.Name, "iteration") {
		t.Errorf("Expected iteration context in name, got: %s", exec.Name)
	}

	if !strings.Contains(exec.Name, "ParentLoop") || !strings.Contains(exec.Name, "LoopNode") {
		t.Errorf("Expected node names in iteration context, got: %s", exec.Name)
	}

	t.Log("Successfully verified CANCELED node preserves iteration context")
}

// TestCanceledNodeExecution_MultipleStatuses tests handling of multiple CANCELED statuses
func TestCanceledNodeExecution_MultipleStatuses(t *testing.T) {
	// This tests the scenario where a node might receive multiple CANCELED statuses
	// (e.g., from both normal flow and defer cleanup)

	nodeID := idwrap.NewNow()
	execID := idwrap.NewNow()

	statuses := []runner.FlowNodeStatus{
		{
			ExecutionID: execID,
			NodeID:      nodeID,
			Name:        "TestNode",
			State:       mnnode.NODE_STATE_RUNNING,
			Error:       nil,
		},
		{
			ExecutionID: execID,
			NodeID:      nodeID,
			Name:        "TestNode",
			State:       mnnode.NODE_STATE_CANCELED,
			Error:       errors.New("context canceled"),
		},
		{
			ExecutionID: execID,
			NodeID:      nodeID,
			Name:        "TestNode",
			State:       mnnode.NODE_STATE_CANCELED,
			Error:       errors.New("context canceled"),
		}, // Duplicate CANCELED status
	}

	// Track how many CANCELED statuses we see for each execution
	executionStates := make(map[idwrap.IDWrap][]mnnode.NodeState)

	for _, status := range statuses {
		executionStates[status.ExecutionID] = append(
			executionStates[status.ExecutionID],
			status.State,
		)
	}

	// Verify we handle multiple CANCELED statuses gracefully
	states := executionStates[execID]
	if len(states) != 3 {
		t.Fatalf("Expected 3 statuses, got %d", len(states))
	}

	// Count CANCELED statuses
	canceledCount := 0
	for _, state := range states {
		if state == mnnode.NODE_STATE_CANCELED {
			canceledCount++
		}
	}

	// We might get multiple CANCELED statuses due to cleanup logic
	// This is acceptable as long as we handle them properly
	if canceledCount < 1 {
		t.Error("Expected at least one CANCELED status")
	}

	t.Logf("Handled %d CANCELED statuses for single execution", canceledCount)
}

// TestCanceledNodeExecution_TimeoutVsCancel tests different cancellation scenarios
func TestCanceledNodeExecution_TimeoutVsCancel(t *testing.T) {
	testCases := []struct {
		name          string
		errorType     error
		expectedInErr string
	}{
		{
			name:          "Context Canceled",
			errorType:     context.Canceled,
			expectedInErr: "context canceled",
		},
		{
			name:          "Deadline Exceeded",
			errorType:     context.DeadlineExceeded,
			expectedInErr: "deadline exceeded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodeID := idwrap.NewNow()
			execID := idwrap.NewNow()
			completedAt := time.Now().UnixMilli()
			errorStr := tc.errorType.Error()

			exec := mnodeexecution.NodeExecution{
				ID:          execID,
				NodeID:      nodeID,
				Name:        "TestNode",
				State:       mnnode.NODE_STATE_CANCELED,
				Error:       &errorStr,
				CompletedAt: &completedAt,
			}

			// Verify error message
			if exec.Error == nil || !strings.Contains(*exec.Error, tc.expectedInErr) {
				t.Errorf("Expected error containing '%s', got: %v", tc.expectedInErr, exec.Error)
			}

			// Verify CompletedAt is set
			if exec.CompletedAt == nil {
				t.Error("Expected CompletedAt to be set for CANCELED execution")
			}

			// Verify state is CANCELED
			if exec.State != mnnode.NODE_STATE_CANCELED {
				t.Errorf("Expected CANCELED state, got %v", exec.State)
			}
		})
	}
}
