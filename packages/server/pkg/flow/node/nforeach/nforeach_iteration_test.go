package nforeach_test

import (
	"context"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"
)

func TestForEachNodeArrayIterationLogging(t *testing.T) {
	// Setup mock nodes
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	
	testFunc := func() {
		// Simple mock function
	}
	
	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFunc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFunc)
	
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}
	
	// Create FOR_EACH node for array iteration
	forEachNodeID := idwrap.NewNow()
	timeout := time.Second * 5
	nodeName := "test-foreach-array"
	
	// Create node to iterate over an array
	forEachNode := nforeach.New(
		forEachNodeID,
		nodeName,
		"var.testArray", // Path to array
		timeout,
		mcondition.Condition{}, // No break condition
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	// Setup edges
	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), forEachNodeID, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)
	
	// Capture logged statuses
	var loggedStatuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	
	logPushFunc := func(status runner.FlowNodeStatus) {
		statusMutex.Lock()
		defer statusMutex.Unlock()
		loggedStatuses = append(loggedStatuses, status)
	}
	
	// Test array with specific values
	testArray := []string{"apple", "banana", "cherry"}
	
	// Create request with LogPushFunc and test data
	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap: map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": testArray,
			},
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeout,
		LogPushFunc:   logPushFunc,
	}
	
	// Execute the FOR_EACH node
	ctx := context.Background()
	result := forEachNode.RunSync(ctx, req)
	
	// Verify execution succeeded
	if result.Err != nil {
		t.Errorf("Expected no error, but got %v", result.Err)
	}
	
	// Verify the correct number of iteration statuses were logged
	statusMutex.Lock()
	defer statusMutex.Unlock()
	
	var iterationStatuses []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.NodeID == forEachNodeID && status.State == mnnode.NODE_STATE_RUNNING {
			iterationStatuses = append(iterationStatuses, status)
		}
	}
	
	expectedIterations := len(testArray)
	if len(iterationStatuses) != expectedIterations {
		t.Errorf("Expected %d iteration statuses, but got %d", expectedIterations, len(iterationStatuses))
	}
	
	// Verify each iteration status contains expected data
	for i, status := range iterationStatuses {
		if status.NodeID != forEachNodeID {
			t.Errorf("Iteration %d: Expected NodeID %v, but got %v", i, forEachNodeID, status.NodeID)
		}
		
		if status.Name != nodeName {
			t.Errorf("Iteration %d: Expected Name %s, but got %s", i, nodeName, status.Name)
		}
		
		if status.State != mnnode.NODE_STATE_RUNNING {
			t.Errorf("Iteration %d: Expected State %v, but got %v", i, mnnode.NODE_STATE_RUNNING, status.State)
		}
		
		// Verify OutputData contains iteration information
		if status.OutputData == nil {
			t.Errorf("Iteration %d: Expected OutputData to be non-nil", i)
			continue
		}
		
		outputMap, ok := status.OutputData.(map[string]interface{})
		if !ok {
			t.Errorf("Iteration %d: Expected OutputData to be map[string]interface{}, but got %T", i, status.OutputData)
			continue
		}
		
		// Check index
		indexValue, exists := outputMap["index"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'index' key in OutputData", i)
			continue
		}
		
		expectedIndex := i
		if indexValue != expectedIndex {
			t.Errorf("Iteration %d: Expected index value %d, but got %v", i, expectedIndex, indexValue)
		}
		
		// Check value
		valueValue, exists := outputMap["value"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'value' key in OutputData", i)
			continue
		}
		
		expectedValue := testArray[i]
		if valueValue != expectedValue {
			t.Errorf("Iteration %d: Expected value %s, but got %v", i, expectedValue, valueValue)
		}
	}
	
	// Verify node variables were set correctly for final state
	totalItemsValue, err := node.ReadNodeVar(req, nodeName, "totalItems")
	if err != nil {
		t.Errorf("Expected to read totalItems variable, but got error: %v", err)
	} else if totalItemsValue != len(testArray) {
		t.Errorf("Expected totalItems to be %d, but got %v", len(testArray), totalItemsValue)
	}
}

func TestForEachNodeMapIterationLogging(t *testing.T) {
	// Setup mock nodes
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	
	testFunc := func() {
		// Simple mock function
	}
	
	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFunc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFunc)
	
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}
	
	// Create FOR_EACH node for map iteration
	forEachNodeID := idwrap.NewNow()
	timeout := time.Second * 5
	nodeName := "test-foreach-map"
	
	// Create node to iterate over a map
	forEachNode := nforeach.New(
		forEachNodeID,
		nodeName,
		"var.testMap", // Path to map
		timeout,
		mcondition.Condition{}, // No break condition
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	// Setup edges
	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), forEachNodeID, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)
	
	// Capture logged statuses
	var loggedStatuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	
	logPushFunc := func(status runner.FlowNodeStatus) {
		statusMutex.Lock()
		defer statusMutex.Unlock()
		loggedStatuses = append(loggedStatuses, status)
	}
	
	// Test map with specific key-value pairs
	testMap := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	
	// Create request with LogPushFunc and test data
	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap: map[string]interface{}{
			"var": map[string]interface{}{
				"testMap": testMap,
			},
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeout,
		LogPushFunc:   logPushFunc,
	}
	
	// Execute the FOR_EACH node
	ctx := context.Background()
	result := forEachNode.RunSync(ctx, req)
	
	// Verify execution succeeded
	if result.Err != nil {
		t.Errorf("Expected no error, but got %v", result.Err)
	}
	
	// Verify the correct number of iteration statuses were logged
	statusMutex.Lock()
	defer statusMutex.Unlock()
	
	var iterationStatuses []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.NodeID == forEachNodeID && status.State == mnnode.NODE_STATE_RUNNING {
			iterationStatuses = append(iterationStatuses, status)
		}
	}
	
	expectedIterations := len(testMap)
	if len(iterationStatuses) != expectedIterations {
		t.Errorf("Expected %d iteration statuses, but got %d", expectedIterations, len(iterationStatuses))
	}
	
	// Collect all logged keys and values to verify they match the test map
	loggedPairs := make(map[string]string)
	
	// Verify each iteration status contains expected data
	for i, status := range iterationStatuses {
		if status.NodeID != forEachNodeID {
			t.Errorf("Iteration %d: Expected NodeID %v, but got %v", i, forEachNodeID, status.NodeID)
		}
		
		if status.Name != nodeName {
			t.Errorf("Iteration %d: Expected Name %s, but got %s", i, nodeName, status.Name)
		}
		
		if status.State != mnnode.NODE_STATE_RUNNING {
			t.Errorf("Iteration %d: Expected State %v, but got %v", i, mnnode.NODE_STATE_RUNNING, status.State)
		}
		
		// Verify OutputData contains iteration information
		if status.OutputData == nil {
			t.Errorf("Iteration %d: Expected OutputData to be non-nil", i)
			continue
		}
		
		outputMap, ok := status.OutputData.(map[string]interface{})
		if !ok {
			t.Errorf("Iteration %d: Expected OutputData to be map[string]interface{}, but got %T", i, status.OutputData)
			continue
		}
		
		// Check key
		keyValue, exists := outputMap["key"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'key' key in OutputData", i)
			continue
		}
		
		keyStr, ok := keyValue.(string)
		if !ok {
			t.Errorf("Iteration %d: Expected key to be string, but got %T", i, keyValue)
			continue
		}
		
		// Check value
		valueValue, exists := outputMap["value"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'value' key in OutputData", i)
			continue
		}
		
		valueStr, ok := valueValue.(string)
		if !ok {
			t.Errorf("Iteration %d: Expected value to be string, but got %T", i, valueValue)
			continue
		}
		
		// Store the logged pair
		loggedPairs[keyStr] = valueStr
		
		// Verify this key exists in our test map
		expectedValue, exists := testMap[keyStr]
		if !exists {
			t.Errorf("Iteration %d: Unexpected key %s in logged data", i, keyStr)
			continue
		}
		
		if valueStr != expectedValue {
			t.Errorf("Iteration %d: Expected value %s for key %s, but got %s", i, expectedValue, keyStr, valueStr)
		}
	}
	
	// Verify all test map entries were logged
	if len(loggedPairs) != len(testMap) {
		t.Errorf("Expected %d logged pairs, but got %d", len(testMap), len(loggedPairs))
	}
	
	for expectedKey, expectedValue := range testMap {
		if loggedValue, exists := loggedPairs[expectedKey]; !exists {
			t.Errorf("Expected key %s was not logged", expectedKey)
		} else if loggedValue != expectedValue {
			t.Errorf("Expected value %s for key %s, but logged %s", expectedValue, expectedKey, loggedValue)
		}
	}
	
	// Verify node variables were set correctly for final state
	totalItemsValue, err := node.ReadNodeVar(req, nodeName, "totalItems")
	if err != nil {
		t.Errorf("Expected to read totalItems variable, but got error: %v", err)
	} else if totalItemsValue != len(testMap) {
		t.Errorf("Expected totalItems to be %d, but got %v", len(testMap), totalItemsValue)
	}
}

func TestForEachNodeEmptyArrayIterationLogging(t *testing.T) {
	// Test edge case with empty array
	forEachNodeID := idwrap.NewNow()
	timeout := time.Second * 5
	nodeName := "test-foreach-empty"
	
	forEachNode := nforeach.New(
		forEachNodeID,
		nodeName,
		"var.emptyArray",
		timeout,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	// Capture logged statuses
	var loggedStatuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	
	logPushFunc := func(status runner.FlowNodeStatus) {
		statusMutex.Lock()
		defer statusMutex.Unlock()
		loggedStatuses = append(loggedStatuses, status)
	}
	
	// Create request with empty array
	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap: map[string]interface{}{
			"var": map[string]interface{}{
				"emptyArray": []string{}, // Empty array
			},
		},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{},
		EdgeSourceMap: make(edge.EdgesMap),
		Timeout:       timeout,
		LogPushFunc:   logPushFunc,
	}
	
	// Execute the FOR_EACH node
	ctx := context.Background()
	result := forEachNode.RunSync(ctx, req)
	
	// Verify execution succeeded
	if result.Err != nil {
		t.Errorf("Expected no error, but got %v", result.Err)
	}
	
	// Verify no iteration statuses were logged
	statusMutex.Lock()
	defer statusMutex.Unlock()
	
	var iterationStatuses []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.NodeID == forEachNodeID && status.State == mnnode.NODE_STATE_RUNNING {
			iterationStatuses = append(iterationStatuses, status)
		}
	}
	
	if len(iterationStatuses) != 0 {
		t.Errorf("Expected 0 iteration statuses for empty array, but got %d", len(iterationStatuses))
	}
	
	// Verify totalItems variable was still set
	totalItemsValue, err := node.ReadNodeVar(req, nodeName, "totalItems")
	if err != nil {
		t.Errorf("Expected to read totalItems variable, but got error: %v", err)
	} else if totalItemsValue != 0 {
		t.Errorf("Expected totalItems to be 0, but got %v", totalItemsValue)
	}
}

func TestForEachNodeArrayIterationLoggingAsync(t *testing.T) {
	// Setup mock nodes
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	
	testFunc := func() {
		// Simple mock function
	}
	
	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFunc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFunc)
	
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}
	
	// Create FOR_EACH node
	forEachNodeID := idwrap.NewNow()
	timeout := time.Second * 5
	nodeName := "test-foreach-async"
	
	forEachNode := nforeach.New(
		forEachNodeID,
		nodeName,
		"var.testArray",
		timeout,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	// Setup edges
	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), forEachNodeID, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)
	
	// Capture logged statuses
	var loggedStatuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	
	logPushFunc := func(status runner.FlowNodeStatus) {
		statusMutex.Lock()
		defer statusMutex.Unlock()
		loggedStatuses = append(loggedStatuses, status)
	}
	
	testArray := []int{10, 20, 30}
	
	// Create request
	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap: map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": testArray,
			},
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeout,
		LogPushFunc:   logPushFunc,
	}
	
	// Execute the FOR_EACH node async
	ctx := context.Background()
	resultChan := make(chan node.FlowNodeResult, 1)
	go forEachNode.RunAsync(ctx, req, resultChan)
	
	// Wait for completion
	select {
	case result := <-resultChan:
		if result.Err != nil {
			t.Errorf("Expected no error, but got %v", result.Err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for async execution to complete")
	}
	
	// Give a moment for all statuses to be logged
	time.Sleep(100 * time.Millisecond)
	
	// Verify iteration statuses
	statusMutex.Lock()
	defer statusMutex.Unlock()
	
	var iterationStatuses []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.NodeID == forEachNodeID && status.State == mnnode.NODE_STATE_RUNNING {
			iterationStatuses = append(iterationStatuses, status)
		}
	}
	
	expectedIterations := len(testArray)
	if len(iterationStatuses) != expectedIterations {
		t.Errorf("Expected %d iteration statuses, but got %d", expectedIterations, len(iterationStatuses))
	}
	
	// Verify each iteration status contains expected data
	for i, status := range iterationStatuses {
		if status.OutputData == nil {
			t.Errorf("Iteration %d: Expected OutputData to be non-nil", i)
			continue
		}
		
		outputMap, ok := status.OutputData.(map[string]interface{})
		if !ok {
			t.Errorf("Iteration %d: Expected OutputData to be map[string]interface{}, but got %T", i, status.OutputData)
			continue
		}
		
		// Check index
		indexValue, exists := outputMap["index"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'index' key in OutputData", i)
			continue
		}
		
		expectedIndex := i
		if indexValue != expectedIndex {
			t.Errorf("Iteration %d: Expected index value %d, but got %v", i, expectedIndex, indexValue)
		}
		
		// Check value
		valueValue, exists := outputMap["value"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'value' key in OutputData", i)
			continue
		}
		
		expectedValue := testArray[i]
		if valueValue != expectedValue {
			t.Errorf("Iteration %d: Expected value %d, but got %v", i, expectedValue, valueValue)
		}
	}
}