package testing

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

func TestStatusValidator_ValidExecutionSequence(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create a valid execution sequence: RUNNING -> SUCCESS
	statuses := []runner.FlowNodeStatus{
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_RUNNING,
			RunDuration: 100 * time.Millisecond,
		},
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_SUCCESS,
			RunDuration: 200 * time.Millisecond,
		},
	}

	for _, status := range statuses {
		collector.Capture(status)
	}

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err != nil {
		t.Fatalf("Expected no validation errors, got: %v", err)
	}
}

func TestStatusValidator_UnterminatedRunning(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create an unterminated execution: RUNNING without final state
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_RUNNING,
		RunDuration: 100 * time.Millisecond,
	}

	collector.Capture(status)

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for unterminated RUNNING, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == "unterminated_running" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected 'unterminated_running' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_MultipleFinalStates(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create an execution with multiple final states
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

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for multiple final states, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == "multiple_final" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected 'multiple_final' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_InvalidTransition(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create an invalid transition: SUCCESS -> RUNNING (should be allowed for retry)
	// Let's test a truly invalid transition: SUCCESS -> FAILURE
	statuses := []runner.FlowNodeStatus{
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

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for invalid transition, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == "invalid_transition" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected 'invalid_transition' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_ExecutionIDReuse(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID1 := idwrap.NewNow()
	nodeID2 := idwrap.NewNow()
	executionID := idwrap.NewNow() // Same execution ID for different nodes

	// Create statuses with same execution ID but different nodes
	statuses := []runner.FlowNodeStatus{
		{
			ExecutionID: executionID,
			NodeID:      nodeID1,
			Name:        "node-1",
			State:       mnnode.NODE_STATE_SUCCESS,
		},
		{
			ExecutionID: executionID,
			NodeID:      nodeID2,
			Name:        "node-2",
			State:       mnnode.NODE_STATE_SUCCESS,
		},
	}

	for _, status := range statuses {
		collector.Capture(status)
	}

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for execution ID reuse, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == "execution_id_reuse" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected 'execution_id_reuse' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_MissingIterationContext(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create an iteration event without context
	status := runner.FlowNodeStatus{
		ExecutionID:      executionID,
		NodeID:           nodeID,
		Name:             "test-node",
		State:            mnnode.NODE_STATE_SUCCESS,
		IterationEvent:   true,
		IterationContext: nil, // Missing context
	}

	collector.Capture(status)

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for missing iteration context, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == "missing_iteration_context" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected 'missing_iteration_context' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_LoopWithoutContext(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	loopNodeID := idwrap.NewNow()

	// Create a status with loop node ID but no iteration context
	status := runner.FlowNodeStatus{
		ExecutionID:      executionID,
		NodeID:           nodeID,
		Name:             "test-node",
		State:            mnnode.NODE_STATE_SUCCESS,
		LoopNodeID:       loopNodeID,
		IterationContext: nil, // Missing context
	}

	collector.Capture(status)

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for loop without context, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == "loop_without_context" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected 'loop_without_context' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_InvalidIterationContext(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	loopNodeID := idwrap.NewNow()

	// Create an iteration context with inconsistent data
	iterationContext := &runner.IterationContext{
		IterationPath:  []int{0, 1},                 // 2 elements
		ExecutionIndex: -1,                          // Invalid negative index
		ParentNodes:    []idwrap.IDWrap{loopNodeID}, // 1 element - mismatch!
	}

	status := runner.FlowNodeStatus{
		ExecutionID:      executionID,
		NodeID:           nodeID,
		Name:             "test-node",
		State:            mnnode.NODE_STATE_SUCCESS,
		LoopNodeID:       loopNodeID,
		IterationContext: iterationContext,
	}

	collector.Capture(status)

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for invalid iteration context, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		foundInvalidIndex := false
		foundInconsistentData := false

		for _, validationErr := range validationErrors {
			switch validationErr.Type {
			case "invalid_execution_index":
				foundInvalidIndex = true
			case "inconsistent_iteration_data":
				foundInconsistentData = true
			}
		}

		if !foundInvalidIndex {
			t.Errorf("Expected 'invalid_execution_index' error, got: %v", err)
		}
		if !foundInconsistentData {
			t.Errorf("Expected 'inconsistent_iteration_data' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_NegativeDuration(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create a status with negative duration
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_SUCCESS,
		RunDuration: -100 * time.Millisecond, // Negative!
	}

	collector.Capture(status)

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected validation error for negative duration, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == "negative_duration" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected 'negative_duration' error, got: %v", err)
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_ValidIterationContext(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	loopNodeID := idwrap.NewNow()

	// Create a valid iteration context
	iterationContext := &runner.IterationContext{
		IterationPath:  []int{0, 1},
		ExecutionIndex: 1,
		ParentNodes:    []idwrap.IDWrap{loopNodeID, idwrap.NewNow()},
		Labels: []runner.IterationLabel{
			{
				NodeID:    loopNodeID,
				Name:      "loop-1",
				Iteration: 0,
			},
		},
	}

	status := runner.FlowNodeStatus{
		ExecutionID:      executionID,
		NodeID:           nodeID,
		Name:             "test-node",
		State:            mnnode.NODE_STATE_SUCCESS,
		LoopNodeID:       loopNodeID,
		IterationContext: iterationContext,
		IterationEvent:   true,
		IterationIndex:   1,
	}

	collector.Capture(status)

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err != nil {
		t.Fatalf("Expected no validation errors for valid iteration context, got: %v", err)
	}
}

func TestStatusValidator_MultipleErrors(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create multiple validation errors
	statuses := []runner.FlowNodeStatus{
		{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_RUNNING,
			RunDuration: -100 * time.Millisecond, // Negative duration
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
			State:       mnnode.NODE_STATE_FAILURE, // Multiple final states
		},
	}

	for _, status := range statuses {
		collector.Capture(status)
	}

	validator := NewStatusValidator(collector)
	err := validator.ValidateAll()
	if err == nil {
		t.Fatal("Expected multiple validation errors, got nil")
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		if len(validationErrors) < 2 {
			t.Fatalf("Expected at least 2 validation errors, got %d: %v", len(validationErrors), err)
		}

		// Check for expected error types
		errorTypes := make(map[string]bool)
		for _, validationErr := range validationErrors {
			errorTypes[validationErr.Type] = true
		}

		if !errorTypes["negative_duration"] {
			t.Errorf("Expected 'negative_duration' error in multiple errors")
		}
		if !errorTypes["multiple_final"] {
			t.Errorf("Expected 'multiple_final' error in multiple errors")
		}
	} else {
		t.Fatalf("Expected ValidationErrors, got: %T", err)
	}
}

func TestStatusValidator_IndividualValidationMethods(t *testing.T) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Test individual validation methods
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_SUCCESS, // Use SUCCESS to avoid unterminated error
		RunDuration: -100 * time.Millisecond,   // Negative duration
	}

	collector.Capture(status)

	validator := NewStatusValidator(collector)

	// Test execution sequence validation (should pass - just SUCCESS)
	err := validator.ValidateExecutionSequences()
	if err != nil {
		t.Errorf("Expected no execution sequence errors, got: %v", err)
	}

	// Test status transition validation (should pass - single status)
	err = validator.ValidateStatusTransitions()
	if err != nil {
		t.Errorf("Expected no status transition errors, got: %v", err)
	}

	// Test timing consistency validation (should fail - negative duration)
	err = validator.ValidateTimingConsistency()
	if err == nil {
		t.Error("Expected timing consistency error for negative duration, got nil")
	}

	// Test execution ID uniqueness (should pass)
	err = validator.ValidateExecutionIDUniqueness()
	if err != nil {
		t.Errorf("Expected no execution ID uniqueness errors, got: %v", err)
	}

	// Test iteration context (should pass - no iteration context)
	err = validator.ValidateIterationContext()
	if err != nil {
		t.Errorf("Expected no iteration context errors, got: %v", err)
	}

	// Test loop node consistency (should pass - no loop node)
	err = validator.ValidateLoopNodeConsistency()
	if err != nil {
		t.Errorf("Expected no loop node consistency errors, got: %v", err)
	}
}
