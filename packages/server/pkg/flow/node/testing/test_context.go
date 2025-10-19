package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// MockDependencies provides mock implementations for common node dependencies.
type MockDependencies struct {
	Logger          *MockLogger
	VariableTracker *MockVariableTracker
}

// MockLogger provides a simple mock logger for testing.
type MockLogger struct {
	mu    sync.Mutex
	logs  []string
	debug bool
}

// NewMockLogger creates a new MockLogger.
func NewMockLogger(debug bool) *MockLogger {
	return &MockLogger{
		logs:  make([]string, 0),
		debug: debug,
	}
}

// Debug logs a debug message.
func (m *MockLogger) Debug(msg string, args ...any) {
	m.log("DEBUG", msg, args...)
}

// Info logs an info message.
func (m *MockLogger) Info(msg string, args ...any) {
	m.log("INFO", msg, args...)
}

// Warn logs a warning message.
func (m *MockLogger) Warn(msg string, args ...any) {
	m.log("WARN", msg, args...)
}

// Error logs an error message.
func (m *MockLogger) Error(msg string, args ...any) {
	m.log("ERROR", msg, args...)
}

// log adds a log entry.
func (m *MockLogger) log(level, msg string, args ...any) {
	if !m.debug && level == "DEBUG" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Simple formatting for testing
	entry := level + ": " + msg
	if len(args) > 0 {
		entry += " " + fmt.Sprintf("%v", args...)
	}
	m.logs = append(m.logs, entry)
}

// GetLogs returns all captured log entries.
func (m *MockLogger) GetLogs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]string, len(m.logs))
	copy(result, m.logs)
	return result
}

// Clear clears all log entries.
func (m *MockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = m.logs[:0]
}

// MockVariableTracker provides a mock variable tracker for testing.
type MockVariableTracker struct {
	mu       sync.RWMutex
	writes   map[string]any
	reads    map[string]any
	readKeys []string
}

// NewMockVariableTracker creates a new MockVariableTracker.
func NewMockVariableTracker() *MockVariableTracker {
	return &MockVariableTracker{
		writes:   make(map[string]any),
		reads:    make(map[string]any),
		readKeys: make([]string, 0),
	}
}

// TrackWrite records a variable write.
func (m *MockVariableTracker) TrackWrite(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes[key] = value
}

// TrackRead records a variable read.
func (m *MockVariableTracker) TrackRead(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads[key] = value
	m.readKeys = append(m.readKeys, key)
}

// GetWrites returns all recorded writes.
func (m *MockVariableTracker) GetWrites() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]any, len(m.writes))
	for k, v := range m.writes {
		result[k] = v
	}
	return result
}

// GetReads returns all recorded reads.
func (m *MockVariableTracker) GetReads() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]any, len(m.reads))
	for k, v := range m.reads {
		result[k] = v
	}
	return result
}

// GetReadKeys returns the ordered list of read keys.
func (m *MockVariableTracker) GetReadKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, len(m.readKeys))
	copy(result, m.readKeys)
	return result
}

// Clear clears all tracking data.
func (m *MockVariableTracker) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.writes = make(map[string]any)
	m.reads = make(map[string]any)
	m.readKeys = make([]string, 0)
}

// TestContext provides test isolation and cleanup utilities for flow node testing.
type TestContext struct {
	t          *testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	collector  *StatusCollector
	validator  *StatusValidator
	mockDeps   *MockDependencies
	cleanupFns []func()
	mu         sync.Mutex
}

// TestContextOptions configures a TestContext.
type TestContextOptions struct {
	Timeout         time.Duration
	EnableDebugLogs bool
	AutoValidate    bool // Automatically validate on cleanup
}

// DefaultTestContextOptions returns default options for TestContext.
func DefaultTestContextOptions() TestContextOptions {
	return TestContextOptions{
		Timeout:         30 * time.Second,
		EnableDebugLogs: false,
		AutoValidate:    true,
	}
}

// NewTestContext creates a new TestContext with the given testing.T and options.
func NewTestContext(t *testing.T, opts ...TestContextOptions) *TestContext {
	options := DefaultTestContextOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)

	collector := NewStatusCollector()
	validator := NewStatusValidator(collector)

	mockDeps := &MockDependencies{
		Logger:          NewMockLogger(options.EnableDebugLogs),
		VariableTracker: NewMockVariableTracker(),
	}

	tc := &TestContext{
		t:          t,
		ctx:        ctx,
		cancel:     cancel,
		collector:  collector,
		validator:  validator,
		mockDeps:   mockDeps,
		cleanupFns: make([]func(), 0),
	}

	// Register cleanup with the testing.T
	t.Cleanup(tc.Cleanup)

	return tc
}

// Context returns the test context.
func (tc *TestContext) Context() context.Context {
	return tc.ctx
}

// Collector returns the status collector.
func (tc *TestContext) Collector() *StatusCollector {
	return tc.collector
}

// Validator returns the status validator.
func (tc *TestContext) Validator() *StatusValidator {
	return tc.validator
}

// MockDeps returns the mock dependencies.
func (tc *TestContext) MockDeps() *MockDependencies {
	return tc.mockDeps
}

// NodeRequestOptions configures a FlowNodeRequest.
type NodeRequestOptions struct {
	VarMap           map[string]any
	NodeMap          map[idwrap.IDWrap]node.FlowNode
	EdgeSourceMap    edge.EdgesMap
	Timeout          time.Duration
	PendingAtomicMap map[idwrap.IDWrap]uint32
	IterationContext *runner.IterationContext
	ExecutionID      idwrap.IDWrap
}

// DefaultNodeRequestOptions returns default options for FlowNodeRequest.
func DefaultNodeRequestOptions() NodeRequestOptions {
	return NodeRequestOptions{
		VarMap:           make(map[string]any),
		NodeMap:          make(map[idwrap.IDWrap]node.FlowNode),
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          10 * time.Second,
		PendingAtomicMap: make(map[idwrap.IDWrap]uint32),
	}
}

// CreateNodeRequest creates a FlowNodeRequest with test context setup.
func (tc *TestContext) CreateNodeRequest(nodeID idwrap.IDWrap, nodeName string, opts ...NodeRequestOptions) *node.FlowNodeRequest {
	options := DefaultNodeRequestOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	req := &node.FlowNodeRequest{
		VarMap:           options.VarMap,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          options.NodeMap,
		EdgeSourceMap:    options.EdgeSourceMap,
		Timeout:          options.Timeout,
		LogPushFunc:      tc.collector.CaptureFromFunc(),
		PendingAtmoicMap: options.PendingAtomicMap,
		VariableTracker:  nil, // Use nil for testing to avoid type conflicts
		IterationContext: options.IterationContext,
		ExecutionID:      options.ExecutionID,
		Logger:           nil, // Use nil for testing to avoid type conflicts
	}

	// Set default execution ID if not provided
	if req.ExecutionID == (idwrap.IDWrap{}) {
		req.ExecutionID = idwrap.NewNow()
	}

	return req
}

// CreateIterationContext creates an IterationContext for testing.
func (tc *TestContext) CreateIterationContext(path []int, parentNodes []idwrap.IDWrap, labels []runner.IterationLabel) *runner.IterationContext {
	return &runner.IterationContext{
		IterationPath:  path,
		ExecutionIndex: 0,
		ParentNodes:    parentNodes,
		Labels:         labels,
	}
}

// CreateTestStatus creates a FlowNodeStatus for testing.
func (tc *TestContext) CreateTestStatus(nodeID, executionID idwrap.IDWrap, name string, state mnnode.NodeState, opts ...TestStatusOptions) runner.FlowNodeStatus {
	options := DefaultTestStatusOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	return runner.FlowNodeStatus{
		ExecutionID:      executionID,
		NodeID:           nodeID,
		Name:             name,
		State:            state,
		OutputData:       options.OutputData,
		InputData:        options.InputData,
		RunDuration:      options.RunDuration,
		Error:            options.Error,
		IterationContext: options.IterationContext,
		IterationEvent:   options.IterationEvent,
		IterationIndex:   options.IterationIndex,
		LoopNodeID:       options.LoopNodeID,
	}
}

// TestStatusOptions configures a test FlowNodeStatus.
type TestStatusOptions struct {
	OutputData       any
	InputData        any
	RunDuration      time.Duration
	Error            error
	IterationContext *runner.IterationContext
	IterationEvent   bool
	IterationIndex   int
	LoopNodeID       idwrap.IDWrap
}

// DefaultTestStatusOptions returns default options for test FlowNodeStatus.
func DefaultTestStatusOptions() TestStatusOptions {
	return TestStatusOptions{
		RunDuration:    100 * time.Millisecond,
		IterationEvent: false,
		IterationIndex: 0,
		LoopNodeID:     idwrap.IDWrap{},
	}
}

// AddCleanup adds a cleanup function to be called during test cleanup.
func (tc *TestContext) AddCleanup(fn func()) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cleanupFns = append(tc.cleanupFns, fn)
}

// Cleanup performs all cleanup operations.
func (tc *TestContext) Cleanup() {
	// Cancel the context
	tc.cancel()

	// Close the collector
	tc.collector.Close()

	// Run custom cleanup functions
	tc.mu.Lock()
	cleanupFns := make([]func(), len(tc.cleanupFns))
	copy(cleanupFns, tc.cleanupFns)
	tc.mu.Unlock()

	// Run cleanup functions in reverse order (like defer)
	for i := len(cleanupFns) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					tc.t.Logf("Cleanup function panicked: %v", r)
				}
			}()
			cleanupFns[i]()
		}()
	}
}

// ValidateAndAssert runs validation and asserts there are no errors.
func (tc *TestContext) ValidateAndAssert() {
	err := tc.validator.ValidateAll()
	if err != nil {
		tc.t.Fatalf("Status validation failed: %v", err)
	}
}

// ExpectValidationError expects a validation error of the specified type.
func (tc *TestContext) ExpectValidationError(expectedType string) {
	err := tc.validator.ValidateAll()
	if err == nil {
		tc.t.Fatalf("Expected validation error of type '%s', but got none", expectedType)
	}

	if validationErrors, ok := err.(ValidationErrors); ok {
		found := false
		for _, validationErr := range validationErrors {
			if validationErr.Type == expectedType {
				found = true
				break
			}
		}
		if !found {
			tc.t.Fatalf("Expected validation error of type '%s', but got: %v", expectedType, err)
		}
	} else {
		tc.t.Fatalf("Expected ValidationErrors, but got: %T", err)
	}
}

// WaitForStatus waits for a status matching the filter criteria.
func (tc *TestContext) WaitForStatus(filter StatusFilter) (*TimestampedStatus, error) {
	return tc.collector.WaitForStatus(tc.ctx, filter)
}

// WaitForNodeState waits for a specific node to reach a specific state.
func (tc *TestContext) WaitForNodeState(nodeID idwrap.IDWrap, state mnnode.NodeState) (*TimestampedStatus, error) {
	filter := StatusFilter{
		NodeID: &nodeID,
		State:  &state,
	}
	return tc.WaitForStatus(filter)
}

// AssertStatusCount asserts the expected number of total statuses.
func (tc *TestContext) AssertStatusCount(expected int) {
	actual := tc.collector.Count()
	if actual != expected {
		tc.t.Fatalf("Expected %d statuses, got %d", expected, actual)
	}
}

// AssertStateCount asserts the expected number of statuses for a specific state.
func (tc *TestContext) AssertStateCount(state mnnode.NodeState, expected int) {
	counts := tc.collector.CountByState()
	actual := counts[state]
	if actual != expected {
		tc.t.Fatalf("Expected %d statuses with state %s, got %d",
			expected, mnnode.StringNodeState(state), actual)
	}
}

// GetMockLogs returns all logs from the mock logger.
func (tc *TestContext) GetMockLogs() []string {
	return tc.mockDeps.Logger.GetLogs()
}

// GetMockWrites returns all variable writes from the mock tracker.
func (tc *TestContext) GetMockWrites() map[string]any {
	return tc.mockDeps.VariableTracker.GetWrites()
}

// GetMockReads returns all variable reads from the mock tracker.
func (tc *TestContext) GetMockReads() map[string]any {
	return tc.mockDeps.VariableTracker.GetReads()
}
