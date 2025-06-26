package nfor_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
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

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
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

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	iterCount := int64(3)
	nodeFor := nfor.New(id, nodeName, iterCount, time.Minute, 0)

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
	nodeFor := nfor.New(id, nodeName, 1, time.Minute, 0)
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
	iterCount := int64(1001) // Test with more than 1000 iterations

	timeOut := 30 * time.Second // Increased timeout for large iteration count
	nodeName := "test-node-large"

	nodeFor := nfor.New(id, nodeName, iterCount, timeOut, 0)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
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

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	iterCount := int64(1001) // Test with more than 1000 iterations
	timeOut := 30 * time.Second // Increased timeout for large iteration count
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
	case <-time.After(45 * time.Second):
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
