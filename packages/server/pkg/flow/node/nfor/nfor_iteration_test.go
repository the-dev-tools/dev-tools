package nfor_test

import (
	"context"
	"fmt"
	"sync"
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

func TestForNodeIterationLogging(t *testing.T) {
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
	
	// Create FOR node with specific iteration count
	forNodeID := idwrap.NewNow()
	iterCount := int64(5)
	timeout := time.Second * 5
	nodeName := "test-for-iteration"
	
	forNode := nfor.New(forNodeID, nodeName, iterCount, timeout, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Setup edges
	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), forNodeID, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
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
	
	// Create request with LogPushFunc
	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]interface{}{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeout,
		LogPushFunc:   logPushFunc,
	}
	
	// Execute the FOR node
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)
	
	// Verify execution succeeded
	if result.Err != nil {
		t.Errorf("Expected no error, but got %v", result.Err)
	}
	
	// Verify the correct number of iteration statuses were logged
	statusMutex.Lock()
	defer statusMutex.Unlock()
	
	var iterationStatuses []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.NodeID == forNodeID && status.State == mnnode.NODE_STATE_RUNNING {
			iterationStatuses = append(iterationStatuses, status)
		}
	}
	
	expectedIterations := int(iterCount)
	if len(iterationStatuses) != expectedIterations {
		t.Errorf("Expected %d iteration statuses, but got %d", expectedIterations, len(iterationStatuses))
	}
	
	// Verify each iteration status contains expected data
	for i, status := range iterationStatuses {
		if status.NodeID != forNodeID {
			t.Errorf("Iteration %d: Expected NodeID %v, but got %v", i, forNodeID, status.NodeID)
		}
		
		expectedName := fmt.Sprintf("%s iteration %d", nodeName, i+1)
		if status.Name != expectedName {
			t.Errorf("Iteration %d: Expected Name %s, but got %s", i, expectedName, status.Name)
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
		
		iterationValue, exists := outputMap["index"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'index' key in OutputData", i)
			continue
		}
		
		expectedIteration := int64(i)
		if iterationValue != expectedIteration {
			t.Errorf("Iteration %d: Expected iteration value %d, but got %v", i, expectedIteration, iterationValue)
		}
	}
	
	// Verify node variables were set correctly for final state
	totalIterationsValue, err := node.ReadNodeVar(req, nodeName, "totalIterations")
	if err != nil {
		t.Errorf("Expected to read totalIterations variable, but got error: %v", err)
	} else if totalIterationsValue != iterCount {
		t.Errorf("Expected totalIterations to be %d, but got %v", iterCount, totalIterationsValue)
	}
}

func TestForNodeIterationLoggingWithZeroIterations(t *testing.T) {
	// Test edge case with zero iterations
	forNodeID := idwrap.NewNow()
	iterCount := int64(0)
	timeout := time.Second * 5
	nodeName := "test-for-zero"
	
	forNode := nfor.New(forNodeID, nodeName, iterCount, timeout, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Capture logged statuses
	var loggedStatuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	
	logPushFunc := func(status runner.FlowNodeStatus) {
		statusMutex.Lock()
		defer statusMutex.Unlock()
		loggedStatuses = append(loggedStatuses, status)
	}
	
	// Create request with minimal setup since no iterations will occur
	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]interface{}{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{},
		EdgeSourceMap: make(edge.EdgesMap),
		Timeout:       timeout,
		LogPushFunc:   logPushFunc,
	}
	
	// Execute the FOR node
	ctx := context.Background()
	result := forNode.RunSync(ctx, req)
	
	// Verify execution succeeded
	if result.Err != nil {
		t.Errorf("Expected no error, but got %v", result.Err)
	}
	
	// Verify no iteration statuses were logged
	statusMutex.Lock()
	defer statusMutex.Unlock()
	
	var iterationStatuses []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.NodeID == forNodeID && status.State == mnnode.NODE_STATE_RUNNING {
			iterationStatuses = append(iterationStatuses, status)
		}
	}
	
	if len(iterationStatuses) != 0 {
		t.Errorf("Expected 0 iteration statuses for zero iterations, but got %d", len(iterationStatuses))
	}
	
	// Verify totalIterations variable was still set
	totalIterationsValue, err := node.ReadNodeVar(req, nodeName, "totalIterations")
	if err != nil {
		t.Errorf("Expected to read totalIterations variable, but got error: %v", err)
	} else if totalIterationsValue != iterCount {
		t.Errorf("Expected totalIterations to be %d, but got %v", iterCount, totalIterationsValue)
	}
}

func TestForNodeIterationLoggingAsync(t *testing.T) {
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
	
	// Create FOR node
	forNodeID := idwrap.NewNow()
	iterCount := int64(3)
	timeout := time.Second * 5
	nodeName := "test-for-async"
	
	forNode := nfor.New(forNodeID, nodeName, iterCount, timeout, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Setup edges
	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), forNodeID, mockNode1ID, edge.HandleLoop, edge.EdgeKindUnspecified)
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
	
	// Create request
	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]interface{}{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeout,
		LogPushFunc:   logPushFunc,
	}
	
	// Execute the FOR node async
	ctx := context.Background()
	resultChan := make(chan node.FlowNodeResult, 1)
	go forNode.RunAsync(ctx, req, resultChan)
	
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
		if status.NodeID == forNodeID && status.State == mnnode.NODE_STATE_RUNNING {
			iterationStatuses = append(iterationStatuses, status)
		}
	}
	
	expectedIterations := int(iterCount)
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
		
		iterationValue, exists := outputMap["index"]
		if !exists {
			t.Errorf("Iteration %d: Expected 'index' key in OutputData", i)
			continue
		}
		
		expectedIteration := int64(i)
		if iterationValue != expectedIteration {
			t.Errorf("Iteration %d: Expected iteration value %d, but got %v", i, expectedIteration, iterationValue)
		}
	}
}