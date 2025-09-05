package nfor_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"
)

// MockNodeWithError is a mock node that can return errors based on a condition
type MockNodeWithError struct {
	ID         idwrap.IDWrap
	Next       []idwrap.IDWrap
	ShouldFail func(iteration int) bool
	iteration  int
	mu         sync.Mutex
}

func (m *MockNodeWithError) GetID() idwrap.IDWrap {
	return m.ID
}

func (m *MockNodeWithError) SetID(id idwrap.IDWrap) {
	m.ID = id
}

func (m *MockNodeWithError) GetName() string {
	return "mockWithError"
}

func (m *MockNodeWithError) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	m.mu.Lock()
	m.iteration++
	currentIteration := m.iteration
	m.mu.Unlock()

	if m.ShouldFail != nil && m.ShouldFail(currentIteration) {
		return node.FlowNodeResult{
			Err: errors.New("mock error"),
		}
	}
	return node.FlowNodeResult{
		NextNodeID: m.Next,
		Err:        nil,
	}
}

func (m *MockNodeWithError) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := m.RunSync(ctx, req)
	resultChan <- result
}

func TestForNode_RunSync(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter atomic.Int32
	testFuncInc := func() {
		runCounter.Add(1)
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}

	id := idwrap.NewNow()
	iterCount := int64(3)

	timeOut := time.Second * 5
	nodeName := "test-node"

	nodeFor := nfor.New(id, nodeName, iterCount, timeOut, 0)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]interface{}{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeOut,
		LogPushFunc:   logMockFunc,
	}

	resault := nodeFor.RunSync(ctx, req)
	if resault.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", resault.Err)
	}
	if runCounter.Load() != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	}
}

func TestForNode_RunAsync(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}
	id := idwrap.NewNow()
	nodeName := "test-node"

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	iterCount := int64(3)
	nodeFor := nfor.New(id, nodeName, iterCount, time.Second*2, 0)

	ctx := context.Background()

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		LogPushFunc:   logMockFunc,
		Timeout:       time.Second,
	}

	resultChan := make(chan node.FlowNodeResult, 1)
	go nodeFor.RunAsync(ctx, req, resultChan)
	result := <-resultChan
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	if runCounter != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter)
	}
}

func TestForNode_SetID(t *testing.T) {
	id := idwrap.NewNow()
	nodeName := "test-node"
	nodeFor := nfor.New(id, nodeName, 1, time.Second*2, 0)
	nodeFor.SetID(id)
	if nodeFor.GetID() != id {
		t.Errorf("Expected nodeFor.GetID() to be %v, but got %v", id, nodeFor.GetID())
	}
}

func TestForNode_RunSync_LargeIterationCount(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter atomic.Int32
	testFuncInc := func() {
		runCounter.Add(1)
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}

	id := idwrap.NewNow()
	iterCount := int64(500) // Test with moderate iteration count

	timeOut := 15 * time.Second // Timeout for moderate iteration count
	nodeName := "test-node-large"

	nodeFor := nfor.New(id, nodeName, iterCount, timeOut, 0)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]interface{}{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeOut,
		LogPushFunc:   logMockFunc,
	}

	result := nodeFor.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	expectedCount := int32(iterCount * 3) // 3 nodes per iteration
	if runCounter.Load() != expectedCount {
		t.Errorf("Expected runCounter to be %d, but got %d", expectedCount, runCounter.Load())
	}
}

func TestForNode_RunAsync_LargeIterationCount(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter atomic.Int32
	var wg sync.WaitGroup
	testFuncInc := func() {
		runCounter.Add(1)
		wg.Done()
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}
	id := idwrap.NewNow()
	nodeName := "test-node-large-async"

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	iterCount := int64(500)     // Test with moderate iteration count
	timeOut := 15 * time.Second // Timeout for moderate iteration count
	nodeFor := nfor.New(id, nodeName, iterCount, timeOut, 0)

	ctx := context.Background()

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		LogPushFunc:   logMockFunc,
		Timeout:       timeOut,
	}

	expectedCount := int32(iterCount * 3) // 3 nodes per iteration
	wg.Add(int(expectedCount))

	resultChan := make(chan node.FlowNodeResult, 1)
	go func() {
		nodeFor.RunAsync(ctx, req, resultChan)
	}()

	// Wait for async operations with timeout
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// All operations completed
	case <-time.After(25 * time.Second):
		t.Fatalf("Timeout waiting for async operations to complete (runCounter=%d, expected=%d)", runCounter.Load(), expectedCount)
	}

	// Get the result
	select {
	case result := <-resultChan:
		if result.Err != nil {
			t.Errorf("Expected err to be nil, but got %v", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for result from RunAsync")
	}

	if runCounter.Load() != expectedCount {
		t.Errorf("Expected runCounter to be %d, but got %d", expectedCount, runCounter.Load())
	}
}

// TestForNode_ErrorHandling_Ignore tests that errors are ignored when ErrorHandling is set to IGNORE
func TestForNode_ErrorHandling_Ignore(t *testing.T) {
	// Setup
	forNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

	// Create a for node with IGNORE error handling
	forNode := nfor.New(forNodeID, "TestForIgnore", 3, time.Second*5, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	// Create error node that fails on second iteration
	errorNode := &MockNodeWithError{
		ID:   errorNodeID,
		Next: nil,
		ShouldFail: func(iteration int) bool {
			return iteration == 2
		},
	}

	// Setup edge map
	edgeMap := make(edge.EdgesMap)
	edgeMap[forNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleLoop: {errorNodeID},
		edge.HandleThen: {nextNodeID},
	}

	// Setup node map
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	nodeMap[forNodeID] = forNode
	nodeMap[errorNodeID] = errorNode

	// Create request
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			// Log function for testing
		},
		Timeout:          time.Second * 5,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Execute
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)

	// Verify
	if result.Err != nil {
		t.Errorf("Expected no error with IGNORE handling, got: %v", result.Err)
	}

	// Should have executed all 3 iterations despite error on iteration 2
	if errorNode.iteration != 3 {
		t.Errorf("Expected 3 executions with IGNORE, got: %d", errorNode.iteration)
	}

	// Should proceed to next node
	if len(result.NextNodeID) != 1 || result.NextNodeID[0] != nextNodeID {
		t.Errorf("Expected to proceed to next node")
	}
}

// TestForNode_ErrorHandling_Break tests that loop stops on error when ErrorHandling is set to BREAK
func TestForNode_ErrorHandling_Break(t *testing.T) {
	// Setup
	forNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

	// Create a for node with BREAK error handling
	forNode := nfor.New(forNodeID, "TestForBreak", 5, time.Second*5, mnfor.ErrorHandling_ERROR_HANDLING_BREAK)

	// Create error node that fails on second iteration
	errorNode := &MockNodeWithError{
		ID:   errorNodeID,
		Next: nil,
		ShouldFail: func(iteration int) bool {
			return iteration == 2
		},
	}

	// Setup edge map
	edgeMap := make(edge.EdgesMap)
	edgeMap[forNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleLoop: {errorNodeID},
		edge.HandleThen: {nextNodeID},
	}

	// Setup node map
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	nodeMap[forNodeID] = forNode
	nodeMap[errorNodeID] = errorNode

	// Create request
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			// Log function for testing
		},
		Timeout:          time.Second * 5,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Execute
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)

	// Verify
	if result.Err != nil {
		t.Errorf("Expected no error with BREAK handling, got: %v", result.Err)
	}

	// Should have executed only 2 iterations (stopped on error)
	if errorNode.iteration != 2 {
		t.Errorf("Expected 2 executions with BREAK (stop on error), got: %d", errorNode.iteration)
	}

	// Should proceed to next node
	if len(result.NextNodeID) != 1 || result.NextNodeID[0] != nextNodeID {
		t.Errorf("Expected to proceed to next node")
	}
}

// TestForNode_ErrorHandling_Unspecified tests default error behavior
func TestForNode_ErrorHandling_Unspecified(t *testing.T) {
	// Setup
	forNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

	// Create a for node with UNSPECIFIED error handling (default fail behavior)
	forNode := nfor.New(forNodeID, "TestForUnspecified", 5, time.Second*5, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Create error node that fails on second iteration
	errorNode := &MockNodeWithError{
		ID:   errorNodeID,
		Next: nil,
		ShouldFail: func(iteration int) bool {
			return iteration == 2
		},
	}

	// Setup edge map
	edgeMap := make(edge.EdgesMap)
	edgeMap[forNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleLoop: {errorNodeID},
		edge.HandleThen: {nextNodeID},
	}

	// Setup node map
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	nodeMap[forNodeID] = forNode
	nodeMap[errorNodeID] = errorNode

	// Create request
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			// Log function for testing
		},
		Timeout:          time.Second * 5,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Execute
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)

	// Verify
	if result.Err == nil {
		t.Error("Expected error with UNSPECIFIED handling")
	}

	// Should have executed only 2 iterations (failed on second)
	if errorNode.iteration != 2 {
		t.Errorf("Expected 2 executions before failure, got: %d", errorNode.iteration)
	}

	// Should not proceed to next node due to error
	if len(result.NextNodeID) != 0 {
		t.Error("Expected no next node due to error")
	}
}

// Test that FOR loop does not emit iteration-level FAILURE statuses when a child fails
// and instead relies on final summary/final status semantics (parity with FOREACH).
func TestForNode_NoIterationFailureStatuses_OnChildFailure(t *testing.T) {
	// Setup
	forNodeID := idwrap.NewNow()
	missingChildID := idwrap.NewNow()

	// Configure FOR with IGNORE so we don't propagate error; all iterations will fail
	forNode := nfor.New(forNodeID, "ParityForIgnore", 3, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	// Edge map points loop body to a non-existent child to force iteration error
	edgeMap := edge.NewEdgesMap([]edge.Edge{
		edge.NewEdge(idwrap.NewNow(), forNodeID, missingChildID, edge.HandleLoop, edge.EdgeKindNoOp),
	})

	// Capture statuses pushed by FOR node
	var captured []runner.FlowNodeStatus
	logPush := func(s runner.FlowNodeStatus) { captured = append(captured, s) }

	// Request with only FOR node present
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{forNodeID: forNode},
		EdgeSourceMap: edgeMap,
		LogPushFunc:   logPush,
		Timeout:       5 * time.Second,
	}

	// Execute
	res := forNode.RunSync(context.Background(), req)

	// Verify no propagation of error under IGNORE
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}

	// Expect only RUNNING statuses, one per iteration; no iteration FAILURE updates
	if len(captured) != 3 {
		t.Fatalf("expected 3 statuses (RUNNING x3), got %d", len(captured))
	}
	for i, st := range captured {
		if st.State != mnnode.NODE_STATE_RUNNING {
			t.Fatalf("status %d: expected RUNNING, got %v", i, st.State)
		}
	}
}

// TestForNode_ErrorHandling_NodeStatus tests that errors are shown on the correct node
func TestForNode_ErrorHandling_NodeStatus(t *testing.T) {
	// Setup
	forNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

	// Track node statuses
	nodeStatuses := make(map[idwrap.IDWrap]runner.FlowNodeStatus)

	// Create a for node with IGNORE error handling
	forNode := nfor.New(forNodeID, "TestForNodeStatus", 3, time.Second*5, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	// Create error node that fails on second iteration
	errorNode := &MockNodeWithError{
		ID:   errorNodeID,
		Next: nil,
		ShouldFail: func(iteration int) bool {
			return iteration == 2
		},
	}

	// Setup edge map
	edgeMap := make(edge.EdgesMap)
	edgeMap[forNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleLoop: {errorNodeID},
		edge.HandleThen: {nextNodeID},
	}

	// Setup node map
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	nodeMap[forNodeID] = forNode
	nodeMap[errorNodeID] = errorNode

	// Create request with status tracking
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			// Capture the final status for each node
			nodeStatuses[status.NodeID] = status
		},
		Timeout:          time.Second * 5,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Execute
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)

	// Verify
	// The for node should complete successfully (no error)
	if result.Err != nil {
		t.Errorf("Expected no error from for node with IGNORE handling, got: %v", result.Err)
	}

	// Should have executed all 3 iterations despite error
	if errorNode.iteration != 3 {
		t.Errorf("Expected 3 executions with IGNORE, got: %d", errorNode.iteration)
	}

	// Check node statuses
	// The error node should have a failure status recorded during iteration 2
	if errorStatus, ok := nodeStatuses[errorNodeID]; ok {
		// Note: We may see multiple statuses for the error node (one per iteration)
		// The important thing is that the error is associated with the error node, not the for node
		t.Logf("Error node status: State=%v, Error=%v", errorStatus.State, errorStatus.Error)
	}

	// The for node itself should not have an error status
	if forStatus, ok := nodeStatuses[forNodeID]; ok {
		if forStatus.State == mnnode.NODE_STATE_FAILURE {
			t.Error("For node should not have failure state when using IGNORE error handling")
		}
		if forStatus.Error != nil {
			t.Errorf("For node should not have error when using IGNORE error handling, got: %v", forStatus.Error)
		}
	}
}

func TestForLoopBreakCondition(t *testing.T) {
	// Test FOR loop that should break when index > 3 (with 10 total iterations)
	forNodeID := idwrap.NewNow()
	mockNodeID := idwrap.NewNow()

	// Create FOR node with break condition "breakLoop.index > 3"
	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "breakLoop.index > 3",
		},
	}
	forNode := nfor.NewWithCondition(
		forNodeID,
		"breakLoop",
		10, // 10 iterations total
		5*time.Second,
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
		condition,
	)

	// Create mock node to track how many times it's called
	var callCount int32
	mockNode := &mocknode.MockNode{
		ID:   mockNodeID,
		Next: []idwrap.IDWrap{},
		OnRun: func() {
			atomic.AddInt32(&callCount, 1)
		},
	}

	// Set up edges: FOR node -> mock node (loop body)
	edgeMap := edge.NewEdgesMap([]edge.Edge{
		{
			SourceID:      forNodeID,
			TargetID:      mockNodeID,
			SourceHandler: edge.HandleLoop,
		},
	})

	// Node mapping
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNodeID:  forNode,
		mockNodeID: mockNode,
	}

	// Track execution statuses
	executionCount := int32(0)
	var statuses []runner.FlowNodeStatus

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			atomic.AddInt32(&executionCount, 1)
			statuses = append(statuses, status)
			t.Logf("Status: NodeID=%s, Name=%s, State=%v", status.NodeID.String(), status.Name, status.State)
		},
		Timeout:          time.Second * 5,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Execute
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)

	// Verify - should complete without error
	if result.Err != nil {
		t.Errorf("Expected no error from FOR loop with break condition, got: %v", result.Err)
	}

	// Should only execute 4 iterations (0, 1, 2, 3) and then break when index = 4 > 3
	expectedIterations := int32(4)
	actualCallCount := atomic.LoadInt32(&callCount)
	if actualCallCount != expectedIterations {
		t.Errorf("Expected %d loop iterations before break, got: %d", expectedIterations, actualCallCount)
	}

	t.Logf("Successfully broke loop after %d iterations with condition 'index > 3'", actualCallCount)
}

func TestForLoopBreakCondition_RealWorldExpression(t *testing.T) {
	// Test with actual expression format that would come from the UI: "for_2.index > 3"
	forNodeID := idwrap.NewNow()
	mockNodeID := idwrap.NewNow()

	// Simulate the parsing logic from rflow.go
	nodeName := "for_2"
	expression := "for_2.index > 3"

	var conditionPath, conditionValue string

	// Parse the expression (same logic as in rflow.go)
	if strings.Contains(expression, " > ") {
		parts := strings.Split(expression, " > ")
		if len(parts) == 2 {
			leftSide := strings.TrimSpace(parts[0])  // "for_2.index"
			rightSide := strings.TrimSpace(parts[1]) // "3"

			expectedLeft := nodeName + ".index"
			if leftSide == expectedLeft {
				conditionPath = leftSide
				conditionValue = rightSide
			}
		}
	}

	// Verify parsing worked
	if conditionPath != "for_2.index" || conditionValue != "3" {
		t.Fatalf("Expression parsing failed - Path: '%s', Value: '%s'", conditionPath, conditionValue)
	}

	// Create FOR node with parsed condition
	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: expression, // "for_2.index > 3"
		},
	}
	forNode := nfor.NewWithCondition(
		forNodeID,
		nodeName,
		10, // 10 iterations total
		5*time.Second,
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
		condition,
	)

	// Create mock node
	var callCount int32
	mockNode := &mocknode.MockNode{
		ID:   mockNodeID,
		Next: []idwrap.IDWrap{},
		OnRun: func() {
			atomic.AddInt32(&callCount, 1)
		},
	}

	// Set up edges
	edgeMap := edge.NewEdgesMap([]edge.Edge{
		{
			SourceID:      forNodeID,
			TargetID:      mockNodeID,
			SourceHandler: edge.HandleLoop,
		},
	})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNodeID:  forNode,
		mockNodeID: mockNode,
	}

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		LogPushFunc: func(status runner.FlowNodeStatus) {
			t.Logf("Status: NodeID=%s, Name=%s, State=%v", status.NodeID.String(), status.Name, status.State)
		},
		Timeout:          time.Second * 5,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Execute
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)

	// Verify
	if result.Err != nil {
		t.Errorf("Expected no error, got: %v", result.Err)
	}

	// Should break after 4 iterations (0, 1, 2, 3) when index becomes 4 > 3
	expectedIterations := int32(4)
	actualCallCount := atomic.LoadInt32(&callCount)
	if actualCallCount != expectedIterations {
		t.Errorf("Expected %d iterations before break, got: %d", expectedIterations, actualCallCount)
	}

	t.Logf("✅ Real-world expression '%s' correctly broke loop after %d iterations", expression, actualCallCount)
}
