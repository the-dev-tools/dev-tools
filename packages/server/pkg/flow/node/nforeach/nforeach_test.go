package nforeach_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"
)

// TODO: refactor this tests

func TestForEachNode_RunSyncArray(t *testing.T) {
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
	// iterCount := int64(3)

	timeOut := time.Duration(time.Second)

	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "var.test == 'test'",
		},
	}

	nodeForEach := nforeach.New(id, "test", "var.array", timeOut, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	varMap := map[string]any{
		"var": map[string]any{
			"array": []string{"a", "b", "c"},
			"test":  "test",
		},
	}

	req := &node.FlowNodeRequest{
		VarMap:        varMap,
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeOut,
		LogPushFunc:   logMockFunc,
	}

	resault := nodeForEach.RunSync(ctx, req)
	if resault.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", resault.Err)
	}
	// TODO: fix this test
	//if runCounter.Load() != 9 {
	//	t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	//}
}

func TestForEachNode_RunAsyncArray(t *testing.T) {
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

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	// iterCount := int64(3)
	timeOut := time.Duration(time.Second)

	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "var.test == 'test'",
		},
	}

	nodeForEach := nforeach.New(id, "test", "var.array", timeOut, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	ctx := context.Background()

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	varMap := map[string]any{
		"var": map[string]any{
			"array": []string{"a", "b", "c"},
			"test":  "test",
		},
	}

	req := &node.FlowNodeRequest{
		VarMap:        varMap,
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		LogPushFunc:   logMockFunc,
		Timeout:       timeOut,
	}

	wg.Add(9) // Expect 9 runs

	resultChan := make(chan node.FlowNodeResult, 1)
	go nodeForEach.RunAsync(ctx, req, resultChan)

	// Wait for the initial result from RunAsync (indicates loop setup is done or immediate error)
	var result node.FlowNodeResult
	select {
	case result = <-resultChan:
		// Got the result from RunAsync
		if result.Err != nil {
			// Use Fatalf to stop the test immediately on error
			t.Fatalf("RunAsync returned an immediate error: %v", result.Err)
		}
	case <-time.After(1 * time.Second): // Short timeout for RunAsync to send its result
		t.Fatalf("Timed out waiting for RunAsync result channel")
	}

	// Now, wait for all async operations triggered by the loop to complete
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	// Wait with a timeout to prevent hanging tests
	select {
	case <-waitChan:
		// wg.Wait() completed successfully
	case <-time.After(5 * time.Second): // Adjust timeout as needed for async tasks
		t.Fatalf("Timed out waiting for WaitGroup (runCounter=%d)", runCounter.Load())
	}

	// Check the final count *after* waiting for the WaitGroup
	if runCounter.Load() != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	}
}

func TestForEachNode_RunSync_Map(t *testing.T) {
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
	timeOut := time.Duration(0)

	// Use a map for iteration by setting IterPath to "var.hash"
	nodeForEach := nforeach.New(id, "test", "var.hash", timeOut, mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"var": map[string]interface{}{
				"hash": map[string]string{
					"a": "valueA",
					"b": "valueB",
					"c": "valueC",
				},
			},
		},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeOut,
		LogPushFunc:   logMockFunc,
	}

	result := nodeForEach.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	if runCounter.Load() != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	}
}

func TestForEachNode_RunAsync_Map(t *testing.T) {
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

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	timeOut := time.Duration(0)
	// Use a map for iteration by setting IterPath to "var.hash"
	nodeForEach := nforeach.New(id, "test", "var.hash", timeOut, mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	ctx := context.Background()

	logMockFunc := func(runner.FlowNodeStatus) {
	}

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"var": map[string]interface{}{
				"hash": map[string]string{
					"a": "valueA",
					"b": "valueB",
					"c": "valueC",
				},
			},
		},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		LogPushFunc:   logMockFunc,
		Timeout:       time.Second,
	}

	wg.Add(9) // Expect 3 iterations since the hashmap has three keys

	resultChan := make(chan node.FlowNodeResult, 1)
	go func() {
		nodeForEach.RunAsync(ctx, req, resultChan)
	}()

	result := <-resultChan
	wg.Wait() // Ensure all iterations complete
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	if runCounter.Load() != 9 {
		t.Errorf("Expected runCounter to be 3, but got %d", runCounter.Load())
	}
}

func TestForEachNode_SetID(t *testing.T) {
	id := idwrap.NewNow()
	timeOut := time.Duration(0)

	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "test == 'test'",
		},
	}
	nodeForEach := nforeach.New(id, "test", "test", timeOut, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	nodeForEach.SetID(id)
	if nodeForEach.GetID() != id {
		t.Errorf("Expected nodeFor.GetID() to be %v, but got %v", id, nodeForEach.GetID())
	}
}

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

// TestForEachNode_ErrorHandling_Ignore tests that errors are ignored when ErrorHandling is set to IGNORE
func TestForEachNode_ErrorHandling_Ignore(t *testing.T) {
	// Setup
	forEachNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

	// Create a foreach node with IGNORE error handling
	forEachNode := nforeach.New(
		forEachNodeID,
		"TestForEachIgnore",
		"[1, 2, 3]", // Iterate over array
		time.Second*5,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
	)

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
	edgeMap[forEachNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleLoop: {errorNodeID},
		edge.HandleThen: {nextNodeID},
	}

	// Setup node map
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	nodeMap[forEachNodeID] = forEachNode
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
	result := forEachNode.RunSync(ctx, req)

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

// TestForEachNode_ErrorHandling_Break tests that loop stops on error when ErrorHandling is set to BREAK
func TestForEachNode_ErrorHandling_Break(t *testing.T) {
	// Setup
	forEachNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

	// Create a foreach node with BREAK error handling
	forEachNode := nforeach.New(
		forEachNodeID,
		"TestForEachBreak",
		"[1, 2, 3, 4, 5]", // Iterate over array
		time.Second*5,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_BREAK,
	)

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
	edgeMap[forEachNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleLoop: {errorNodeID},
		edge.HandleThen: {nextNodeID},
	}

	// Setup node map
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	nodeMap[forEachNodeID] = forEachNode
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
	result := forEachNode.RunSync(ctx, req)

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

// TestForEachNode_ErrorHandling_Unspecified tests default error behavior
func TestForEachNode_ErrorHandling_Unspecified(t *testing.T) {
	// Setup
	forEachNodeID := idwrap.NewNow()
	errorNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

	// Create a foreach node with UNSPECIFIED error handling (default fail behavior)
	forEachNode := nforeach.New(
		forEachNodeID,
		"TestForEachUnspecified",
		"[1, 2, 3, 4, 5]", // Iterate over array
		time.Second*5,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)

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
	edgeMap[forEachNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
		edge.HandleLoop: {errorNodeID},
		edge.HandleThen: {nextNodeID},
	}

	// Setup node map
	nodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	nodeMap[forEachNodeID] = forEachNode
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
	result := forEachNode.RunSync(ctx, req)

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
