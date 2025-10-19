package testing

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

func TestTestContext_BasicUsage(t *testing.T) {
	tc := NewTestContext(t)

	// Test basic properties
	if tc.Context() == nil {
		t.Error("Expected non-nil context")
	}

	if tc.Collector() == nil {
		t.Error("Expected non-nil collector")
	}

	if tc.Validator() == nil {
		t.Error("Expected non-nil validator")
	}

	if tc.MockDeps() == nil {
		t.Error("Expected non-nil mock dependencies")
	}

	// Test that collector and validator are linked
	if tc.Validator().collector != tc.Collector() {
		t.Error("Expected validator to use the same collector")
	}
}

func TestTestContext_CustomOptions(t *testing.T) {
	customTimeout := 5 * time.Second
	opts := TestContextOptions{
		Timeout:         customTimeout,
		EnableDebugLogs: true,
		AutoValidate:    false,
	}

	tc := NewTestContext(t, opts)

	// Test that mock logger has debug enabled
	if !tc.MockDeps().Logger.debug {
		t.Error("Expected debug logs to be enabled")
	}

	// Test context timeout (can't easily test the actual timeout without waiting)
	// but we can verify the context is not nil
	if tc.Context() == nil {
		t.Error("Expected non-nil context with custom options")
	}
}

func TestTestContext_CreateNodeRequest(t *testing.T) {
	tc := NewTestContext(t)

	nodeID := idwrap.NewNow()
	nodeName := "test-node"

	req := tc.CreateNodeRequest(nodeID, nodeName)

	if req == nil {
		t.Fatal("Expected non-nil FlowNodeRequest")
	}

	if req.ExecutionID == (idwrap.IDWrap{}) {
		t.Error("Expected auto-generated execution ID")
	}

	if req.LogPushFunc == nil {
		t.Error("Expected LogPushFunc to be set")
	}

	if req.ReadWriteLock == nil {
		t.Error("Expected ReadWriteLock to be set")
	}

	// Test that LogPushFunc actually captures statuses
	status := runner.FlowNodeStatus{
		ExecutionID: req.ExecutionID,
		NodeID:      nodeID,
		Name:        nodeName,
		State:       mnnode.NODE_STATE_RUNNING,
	}

	req.LogPushFunc(status)

	if tc.Collector().Count() != 1 {
		t.Errorf("Expected 1 captured status, got %d", tc.Collector().Count())
	}
}

func TestTestContext_CreateNodeRequestWithOptions(t *testing.T) {
	tc := NewTestContext(t)

	nodeID := idwrap.NewNow()
	nodeName := "test-node"
	customExecID := idwrap.NewNow()

	opts := NodeRequestOptions{
		VarMap:      map[string]any{"key": "value"},
		Timeout:     20 * time.Second,
		ExecutionID: customExecID,
	}

	req := tc.CreateNodeRequest(nodeID, nodeName, opts)

	if req.ExecutionID != customExecID {
		t.Errorf("Expected execution ID %s, got %s", customExecID.String(), req.ExecutionID.String())
	}

	if req.Timeout != 20*time.Second {
		t.Errorf("Expected timeout 20s, got %v", req.Timeout)
	}

	if req.VarMap["key"] != "value" {
		t.Errorf("Expected VarMap to contain custom data")
	}
}

func TestTestContext_CreateIterationContext(t *testing.T) {
	tc := NewTestContext(t)

	path := []int{0, 1, 2}
	parentNodes := []idwrap.IDWrap{idwrap.NewNow(), idwrap.NewNow()}
	labels := []runner.IterationLabel{
		{
			NodeID:    parentNodes[0],
			Name:      "loop-1",
			Iteration: 0,
		},
		{
			NodeID:    parentNodes[1],
			Name:      "loop-2",
			Iteration: 1,
		},
	}

	ctx := tc.CreateIterationContext(path, parentNodes, labels)

	if ctx == nil {
		t.Fatal("Expected non-nil IterationContext")
	}

	if len(ctx.IterationPath) != 3 {
		t.Errorf("Expected iteration path length 3, got %d", len(ctx.IterationPath))
	}

	if len(ctx.ParentNodes) != 2 {
		t.Errorf("Expected parent nodes length 2, got %d", len(ctx.ParentNodes))
	}

	if len(ctx.Labels) != 2 {
		t.Errorf("Expected labels length 2, got %d", len(ctx.Labels))
	}

	if ctx.ExecutionIndex != 0 {
		t.Errorf("Expected execution index 0, got %d", ctx.ExecutionIndex)
	}
}

func TestTestContext_CreateTestStatus(t *testing.T) {
	tc := NewTestContext(t)

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	name := "test-node"
	state := mnnode.NODE_STATE_SUCCESS

	status := tc.CreateTestStatus(nodeID, executionID, name, state)

	if status.NodeID != nodeID {
		t.Errorf("Expected node ID %s, got %s", nodeID.String(), status.NodeID.String())
	}

	if status.ExecutionID != executionID {
		t.Errorf("Expected execution ID %s, got %s", executionID.String(), status.ExecutionID.String())
	}

	if status.Name != name {
		t.Errorf("Expected name %s, got %s", name, status.Name)
	}

	if status.State != state {
		t.Errorf("Expected state %s, got %s", mnnode.StringNodeState(state), mnnode.StringNodeState(status.State))
	}

	// Test default values
	if status.RunDuration != 100*time.Millisecond {
		t.Errorf("Expected default duration 100ms, got %v", status.RunDuration)
	}

	if status.IterationEvent {
		t.Error("Expected default iteration event to be false")
	}
}

func TestTestContext_CreateTestStatusWithOptions(t *testing.T) {
	tc := NewTestContext(t)

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	name := "test-node"
	state := mnnode.NODE_STATE_SUCCESS

	loopNodeID := idwrap.NewNow()
	iterationContext := tc.CreateIterationContext([]int{0}, []idwrap.IDWrap{loopNodeID}, nil)

	opts := TestStatusOptions{
		OutputData:       "test-output",
		InputData:        "test-input",
		RunDuration:      500 * time.Millisecond,
		Error:            nil,
		IterationContext: iterationContext,
		IterationEvent:   true,
		IterationIndex:   2,
		LoopNodeID:       loopNodeID,
	}

	status := tc.CreateTestStatus(nodeID, executionID, name, state, opts)

	if status.OutputData != "test-output" {
		t.Errorf("Expected output data 'test-output', got %v", status.OutputData)
	}

	if status.InputData != "test-input" {
		t.Errorf("Expected input data 'test-input', got %v", status.InputData)
	}

	if status.RunDuration != 500*time.Millisecond {
		t.Errorf("Expected duration 500ms, got %v", status.RunDuration)
	}

	if !status.IterationEvent {
		t.Error("Expected iteration event to be true")
	}

	if status.IterationIndex != 2 {
		t.Errorf("Expected iteration index 2, got %d", status.IterationIndex)
	}

	if status.LoopNodeID != loopNodeID {
		t.Errorf("Expected loop node ID %s, got %s", loopNodeID.String(), status.LoopNodeID.String())
	}

	if status.IterationContext != iterationContext {
		t.Error("Expected iteration context to be set")
	}
}

func TestTestContext_Assertions(t *testing.T) {
	tc := NewTestContext(t)

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Test AssertStatusCount with no statuses
	tc.AssertStatusCount(0)

	// Add some statuses
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
		tc.Collector().Capture(status)
	}

	// Test AssertStatusCount
	tc.AssertStatusCount(3)

	// Test AssertStateCount
	tc.AssertStateCount(mnnode.NODE_STATE_RUNNING, 1)
	tc.AssertStateCount(mnnode.NODE_STATE_SUCCESS, 1)
	tc.AssertStateCount(mnnode.NODE_STATE_FAILURE, 1)
	tc.AssertStateCount(mnnode.NODE_STATE_CANCELED, 0)
}

func TestTestContext_ValidateAndAssert(t *testing.T) {
	tc := NewTestContext(t)

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create a valid execution sequence
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
	}

	for _, status := range statuses {
		tc.Collector().Capture(status)
	}

	// Should not panic
	tc.ValidateAndAssert()
}

func TestTestContext_ExpectValidationError(t *testing.T) {
	tc := NewTestContext(t)

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create an invalid execution (unterminated RUNNING)
	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_RUNNING,
	}

	tc.Collector().Capture(status)

	// Should expect the validation error
	tc.ExpectValidationError("unterminated_running")
}

func TestTestContext_WaitForStatus(t *testing.T) {
	tc := NewTestContext(t, TestContextOptions{Timeout: 1 * time.Second})

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Test waiting for a status that will be added asynchronously
	go func() {
		time.Sleep(100 * time.Millisecond)
		status := runner.FlowNodeStatus{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_SUCCESS,
		}
		tc.Collector().Capture(status)
	}()

	// Wait for the status
	filter := StatusFilter{
		NodeID: &nodeID,
		State:  &[]mnnode.NodeState{mnnode.NODE_STATE_SUCCESS}[0],
	}

	result, err := tc.WaitForStatus(filter)
	if err != nil {
		t.Fatalf("Expected no error waiting for status, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected status result, got nil")
	}

	if result.Status.State != mnnode.NODE_STATE_SUCCESS {
		t.Errorf("Expected SUCCESS state, got %s", mnnode.StringNodeState(result.Status.State))
	}
}

func TestTestContext_WaitForNodeState(t *testing.T) {
	tc := NewTestContext(t, TestContextOptions{Timeout: 1 * time.Second})

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Test waiting for a specific node state
	go func() {
		time.Sleep(100 * time.Millisecond)
		status := runner.FlowNodeStatus{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NODE_STATE_FAILURE,
		}
		tc.Collector().Capture(status)
	}()

	result, err := tc.WaitForNodeState(nodeID, mnnode.NODE_STATE_FAILURE)
	if err != nil {
		t.Fatalf("Expected no error waiting for node state, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected status result, got nil")
	}

	if result.Status.State != mnnode.NODE_STATE_FAILURE {
		t.Errorf("Expected FAILURE state, got %s", mnnode.StringNodeState(result.Status.State))
	}
}

func TestTestContext_MockDependencies(t *testing.T) {
	tc := NewTestContext(t, TestContextOptions{EnableDebugLogs: true})

	logger := tc.MockDeps().Logger
	tracker := tc.MockDeps().VariableTracker

	// Test mock logger
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	logs := tc.GetMockLogs()
	if len(logs) != 4 {
		t.Errorf("Expected 4 log entries, got %d", len(logs))
	}

	// Test mock variable tracker
	tracker.TrackWrite("key1", "value1")
	tracker.TrackWrite("key2", "value2")
	tracker.TrackRead("key1", "value1")
	tracker.TrackRead("key3", "value3")

	writes := tc.GetMockWrites()
	if len(writes) != 2 {
		t.Errorf("Expected 2 writes, got %d", len(writes))
	}

	reads := tc.GetMockReads()
	if len(reads) != 2 {
		t.Errorf("Expected 2 reads, got %d", len(reads))
	}

	readKeys := tracker.GetReadKeys()
	if len(readKeys) != 2 || readKeys[0] != "key1" || readKeys[1] != "key3" {
		t.Errorf("Expected read keys [key1, key3], got %v", readKeys)
	}
}

func TestTestContext_AddCleanup(t *testing.T) {
	tc := NewTestContext(t)

	cleanupCalled := false

	// Add a cleanup function
	tc.AddCleanup(func() {
		cleanupCalled = true
	})

	// Trigger cleanup manually (normally done by t.Cleanup)
	tc.Cleanup()

	if !cleanupCalled {
		t.Error("Expected cleanup function to be called")
	}

	// Test that collector is closed
	if !tc.Collector().IsClosed() {
		t.Error("Expected collector to be closed after cleanup")
	}
}

func TestTestContext_CleanupOrder(t *testing.T) {
	tc := NewTestContext(t)

	var callOrder []int

	// Add multiple cleanup functions
	tc.AddCleanup(func() { callOrder = append(callOrder, 1) })
	tc.AddCleanup(func() { callOrder = append(callOrder, 2) })
	tc.AddCleanup(func() { callOrder = append(callOrder, 3) })

	// Trigger cleanup
	tc.Cleanup()

	// Should be called in reverse order (like defer)
	expected := []int{3, 2, 1}
	if len(callOrder) != 3 {
		t.Fatalf("Expected 3 cleanup calls, got %d", len(callOrder))
	}

	for i, expectedVal := range expected {
		if callOrder[i] != expectedVal {
			t.Errorf("Expected cleanup order %v, got %v", expected, callOrder)
			break
		}
	}
}
