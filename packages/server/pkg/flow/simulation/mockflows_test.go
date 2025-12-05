package simulation

import (
	"context"
	"errors"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/flow/runner"
	flowlocalrunner "the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"github.com/stretchr/testify/require"
)

func TestCreateMockFlow_BasicStructure(t *testing.T) {
	params := MockFlowParams{
		RequestCount: 3,
		ForLoopCount: 2,
		Delay:        10 * time.Millisecond,
	}

	result := CreateMockFlow(params)

	// Verify total nodes count (1 start + 3 request + 2 for loop = 6)
	expectedNodeCount := 1 + params.RequestCount + params.ForLoopCount
	if len(result.Nodes) != expectedNodeCount {
		t.Errorf("Expected %d nodes, got %d", expectedNodeCount, len(result.Nodes))
	}

	// Verify edges count (should be nodes - 1 for linear flow)
	expectedEdgeCount := expectedNodeCount - 1
	if len(result.Edges) != expectedEdgeCount {
		t.Errorf("Expected %d edges, got %d", expectedEdgeCount, len(result.Edges))
	}

	// Verify edges map is not nil
	if result.EdgesMap == nil {
		t.Error("EdgesMap should not be nil")
	}

	// Verify start node ID is set (check if it's empty by comparing to zero value)
	var zeroID idwrap.IDWrap
	if result.StartNodeID == zeroID {
		t.Error("StartNodeID should not be zero")
	}

	// Verify start node exists
	startNode, exists := result.Nodes[result.StartNodeID]
	if !exists {
		t.Error("Start node should exist in nodes map")
	}
	if startNode.GetName() != "mock" {
		t.Errorf("Expected start node name 'mock', got '%s'", startNode.GetName())
	}
}

func TestCreateMockFlow_LinearFlow(t *testing.T) {
	params := MockFlowParams{
		RequestCount: 2,
		ForLoopCount: 1,
		Delay:        10 * time.Millisecond, // Use non-zero delay to distinguish nodes
	}

	result := CreateMockFlow(params)

	// Verify total node count
	expectedTotalNodes := 1 + params.RequestCount + params.ForLoopCount
	if len(result.Nodes) != expectedTotalNodes {
		t.Errorf("Expected %d total nodes, got %d", expectedTotalNodes, len(result.Nodes))
	}

	// Verify that exactly one node (the last one) has no next nodes
	nodesWithNoNext := 0
	for _, node := range result.Nodes {
		mockNode, ok := node.(*mocknode.MockNode)
		if !ok {
			continue
		}
		if len(mockNode.Next) == 0 {
			nodesWithNoNext++
		}
	}

	if nodesWithNoNext != 1 {
		t.Errorf("Expected exactly 1 node with no next nodes (the last node), got %d", nodesWithNoNext)
	}

	// Verify that all other nodes have exactly one next node
	nodesWithOneNext := 0
	for _, node := range result.Nodes {
		mockNode, ok := node.(*mocknode.MockNode)
		if !ok {
			continue
		}
		if len(mockNode.Next) == 1 {
			nodesWithOneNext++
		}
	}

	expectedNodesWithOneNext := len(result.Nodes) - 1
	require.Equal(t, expectedNodesWithOneNext, nodesWithOneNext, "Expected %d nodes with one next node, got %d", expectedNodesWithOneNext, nodesWithOneNext)

	// Verify that nodes with delays have the correct delay
	nodesWithCorrectDelay := 0
	for _, node := range result.Nodes {
		mockNode, ok := node.(*mocknode.MockNode)
		if !ok {
			continue
		}
		if mockNode.Delay == params.Delay {
			nodesWithCorrectDelay++
		}
	}

	expectedNodesWithDelay := params.RequestCount + params.ForLoopCount
	require.Equal(t, expectedNodesWithDelay, nodesWithCorrectDelay, "Expected %d nodes with delay %v, got %d", expectedNodesWithDelay, params.Delay, nodesWithCorrectDelay)
}

func TestCreateMockFlow_EdgesConnectivity(t *testing.T) {
	params := MockFlowParams{
		RequestCount: 2,
		ForLoopCount: 1,
		Delay:        0,
	}

	result := CreateMockFlow(params)

	// Verify that edges form a connected linear path
	// Start with the start node and follow the edges
	currentID := result.StartNodeID
	visitedNodes := make(map[idwrap.IDWrap]bool)
	visitedNodes[currentID] = true

	for i := 0; i < len(result.Nodes)-1; i++ {
		// Get next nodes from edges map
		nextNodes := edge.GetNextNodeID(result.EdgesMap, currentID, edge.HandleThen)
		if len(nextNodes) != 1 {
			t.Errorf("Expected exactly 1 next node from %v, got %d", currentID, len(nextNodes))
			break
		}

		nextID := nextNodes[0]
		if visitedNodes[nextID] {
			t.Errorf("Cycle detected: node %v visited twice", nextID)
			break
		}

		// Verify the next node exists
		if _, exists := result.Nodes[nextID]; !exists {
			t.Errorf("Next node %v does not exist in nodes map", nextID)
			break
		}

		visitedNodes[nextID] = true
		currentID = nextID
	}

	// Verify all nodes were visited
	if len(visitedNodes) != len(result.Nodes) {
		t.Errorf("Not all nodes were visited. Expected %d, got %d", len(result.Nodes), len(visitedNodes))
	}
}

func TestCreateMockFlow_ZeroNodes(t *testing.T) {
	params := MockFlowParams{
		RequestCount: 0,
		ForLoopCount: 0,
		Delay:        0,
	}

	result := CreateMockFlow(params)

	// Should have only the start node
	if len(result.Nodes) != 1 {
		t.Errorf("Expected 1 node (start only), got %d", len(result.Nodes))
	}

	if len(result.Edges) != 0 {
		t.Errorf("Expected 0 edges, got %d", len(result.Edges))
	}
}

func TestCreateMockFlow_OnlyRequestNodes(t *testing.T) {
	params := MockFlowParams{
		RequestCount: 3,
		ForLoopCount: 0,
		Delay:        5 * time.Millisecond,
	}

	result := CreateMockFlow(params)

	// Should have start + 3 request nodes = 4 total
	expectedCount := 1 + params.RequestCount
	if len(result.Nodes) != expectedCount {
		t.Errorf("Expected %d nodes, got %d", expectedCount, len(result.Nodes))
	}

	// Verify all nodes except start have the expected delay
	for nodeID, node := range result.Nodes {
		if nodeID == result.StartNodeID {
			continue // Skip start node
		}

		mockNode, ok := node.(*mocknode.MockNode)
		if !ok {
			t.Errorf("Expected MockNode, got %T", node)
			continue
		}

		if mockNode.Delay != params.Delay {
			t.Errorf("Expected delay %v, got %v", params.Delay, mockNode.Delay)
		}
	}
}

func TestCreateMockFlow_OnlyForLoopNodes(t *testing.T) {
	params := MockFlowParams{
		RequestCount: 0,
		ForLoopCount: 2,
		Delay:        15 * time.Millisecond,
	}

	result := CreateMockFlow(params)

	// Should have start + 2 for loop nodes = 3 total
	expectedCount := 1 + params.ForLoopCount
	if len(result.Nodes) != expectedCount {
		t.Errorf("Expected %d nodes, got %d", expectedCount, len(result.Nodes))
	}
}

func TestCreateMockFlow_IntegrationWithFlowLocalRunner(t *testing.T) {
	params := MockFlowParams{
		RequestCount: 2,
		ForLoopCount: 1,
		Delay:        10 * time.Millisecond,
	}

	result := CreateMockFlow(params)

	// Create a FlowLocalRunner with the mock flow
	flowID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		flowID,
		result.StartNodeID,
		result.Nodes,
		result.EdgesMap,
		5*time.Second, // 5 second timeout
		nil,           // logger
	)

	// Run the flow
	ctx := context.Background()
	nodeStatusChan := make(chan runner.FlowNodeStatus, 20)
	flowStatusChan := make(chan runner.FlowStatus, 5)

	// Run the flow synchronously
	if err := flowRunner.Run(ctx, nodeStatusChan, flowStatusChan, nil); err != nil {
		t.Errorf("Flow execution failed: %v", err)
		return
	}

	// Collect all node statuses (wait for channel to close)
	nodeStatuses := make([]runner.FlowNodeStatus, 0)
	for status := range nodeStatusChan {
		nodeStatuses = append(nodeStatuses, status)
	}

	// Collect all flow statuses (wait for channel to close)
	flowStatuses := make([]runner.FlowStatus, 0)
	for status := range flowStatusChan {
		flowStatuses = append(flowStatuses, status)
	}

	// Count successful nodes (each node should have a SUCCESS state)
	successCount := 0
	nodeSuccessMap := make(map[idwrap.IDWrap]bool)

	for _, status := range nodeStatuses {
		if status.State == mnnode.NODE_STATE_SUCCESS {
			successCount++
			nodeSuccessMap[status.NodeID] = true
		}
	}

	expectedNodeCount := len(result.Nodes)
	require.Equal(t, expectedNodeCount, successCount, "Expected %d successful node statuses, got %d", expectedNodeCount, successCount)

	// Verify all nodes completed successfully
	for nodeID := range result.Nodes {
		if !nodeSuccessMap[nodeID] {
			t.Errorf("Node %v did not complete successfully", nodeID)
		}
	}

	// Verify flow completed successfully
	if len(flowStatuses) == 0 {
		t.Error("Expected at least one flow status")
	} else {
		// Look for success status in the flow statuses
		foundSuccess := false
		for _, status := range flowStatuses {
			if status == runner.FlowStatusSuccess {
				foundSuccess = true
				break
			}
		}
		if !foundSuccess {
			t.Errorf("Expected flow success status in %v", flowStatuses)
		}
	}
}

func TestCreateMockFlow_Performance_BasicLoad(t *testing.T) {
	// Create a mock flow with enough nodes to make parallel execution meaningful
	params := MockFlowParams{
		RequestCount: 20,
		ForLoopCount: 5,
		Delay:        2 * time.Millisecond, // Small delay for measurable but fast execution
	}

	result := CreateMockFlow(params)
	totalNodes := len(result.Nodes)

	// Calculate theoretical sequential execution time
	sequentialTimeEstimate := time.Duration(totalNodes-1) * params.Delay // -1 because start node has no delay

	// Test parallel execution
	flowID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		flowID,
		result.StartNodeID,
		result.Nodes,
		result.EdgesMap,
		10*time.Second, // Generous timeout
		nil,            // logger
	)

	// Force parallel execution mode
	flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeMulti)

	ctx := context.Background()
	nodeStatusChan := make(chan runner.FlowNodeStatus, totalNodes*2)
	flowStatusChan := make(chan runner.FlowStatus, 5)

	// Measure parallel execution time
	start := time.Now()
	if err := flowRunner.Run(ctx, nodeStatusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("Parallel flow execution failed: %v", err)
	}
	parallelTime := time.Since(start)

	// Collect statuses to verify execution completed
	nodeStatuses := make([]runner.FlowNodeStatus, 0)
	for status := range nodeStatusChan {
		nodeStatuses = append(nodeStatuses, status)
	}

	flowStatuses := make([]runner.FlowStatus, 0)
	for status := range flowStatusChan {
		flowStatuses = append(flowStatuses, status)
	}

	// Verify all nodes completed successfully
	successCount := 0
	for _, status := range nodeStatuses {
		if status.State == mnnode.NODE_STATE_SUCCESS {
			successCount++
		}
	}

	require.Equal(t, totalNodes, successCount, "Expected %d successful nodes, got %d", totalNodes, successCount)

	// Verify flow completed successfully
	foundSuccess := false
	for _, status := range flowStatuses {
		if status == runner.FlowStatusSuccess {
			foundSuccess = true
			break
		}
	}
	if !foundSuccess {
		t.Error("Flow did not complete successfully")
	}

	// Performance assertion: parallel should be significantly faster than sequential
	// For a linear flow, parallel execution won't be much faster, but it should still complete
	// in reasonable time. We use a more lenient check since this is still a linear flow.
	maxAcceptableTime := sequentialTimeEstimate + 50*time.Millisecond // Allow some overhead

	if parallelTime > maxAcceptableTime {
		t.Errorf("Parallel execution took %v, which is slower than expected (max acceptable: %v, sequential estimate: %v)",
			parallelTime, maxAcceptableTime, sequentialTimeEstimate)
	}

	// Additional sanity check: execution should complete in reasonable time
	if parallelTime > 5*time.Second {
		t.Errorf("Execution took too long: %v (expected < 5s)", parallelTime)
	}

	t.Logf("Performance test completed: %d nodes in %v (sequential estimate: %v)",
		totalNodes, parallelTime, sequentialTimeEstimate)
}

func TestCreateMockFlow_ExecutionModeSelection(t *testing.T) {
	tests := []struct {
		name         string
		requestCount int
		forLoopCount int
		expectedMode flowlocalrunner.ExecutionMode
	}{
		{
			name:         "small flow should select single mode",
			requestCount: 2,
			forLoopCount: 1,
			expectedMode: flowlocalrunner.ExecutionModeSingle,
		},
		{
			name:         "medium flow should select single mode",
			requestCount: 4,
			forLoopCount: 1,
			expectedMode: flowlocalrunner.ExecutionModeSingle,
		},
		{
			name:         "large flow should select multi mode",
			requestCount: 10,
			forLoopCount: 5,
			expectedMode: flowlocalrunner.ExecutionModeMulti,
		},
		{
			name:         "very large flow should select multi mode",
			requestCount: 15,
			forLoopCount: 5,
			expectedMode: flowlocalrunner.ExecutionModeMulti,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := MockFlowParams{
				RequestCount: tt.requestCount,
				ForLoopCount: tt.forLoopCount,
				Delay:        1 * time.Millisecond,
			}

			result := CreateMockFlow(params)

			// Create FlowLocalRunner with auto execution mode
			flowID := idwrap.NewNow()
			flowRunner := flowlocalrunner.CreateFlowRunner(
				idwrap.NewNow(),
				flowID,
				result.StartNodeID,
				result.Nodes,
				result.EdgesMap,
				5*time.Second,
				nil,
			)

			// Set execution mode to auto to trigger automatic selection
			flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeAuto)

			// Run the flow to trigger mode selection
			ctx := context.Background()
			nodeStatusChan := make(chan runner.FlowNodeStatus, len(result.Nodes)*2)
			flowStatusChan := make(chan runner.FlowStatus, 5)

			if err := flowRunner.Run(ctx, nodeStatusChan, flowStatusChan, nil); err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}

			// Check what mode was actually selected
			selectedMode := flowRunner.SelectedMode()
			if selectedMode != tt.expectedMode {
				t.Errorf("Expected execution mode %v, got %v (flow has %d nodes)",
					tt.expectedMode, selectedMode, len(result.Nodes))
			}

			// Verify flow completed successfully
			flowStatuses := make([]runner.FlowStatus, 0)
			for status := range flowStatusChan {
				flowStatuses = append(flowStatuses, status)
			}

			foundSuccess := false
			for _, status := range flowStatuses {
				if status == runner.FlowStatusSuccess {
					foundSuccess = true
					break
				}
			}
			if !foundSuccess {
				t.Error("Flow did not complete successfully")
			}

			t.Logf("Flow with %d nodes selected mode: %v (expected: %v)",
				len(result.Nodes), selectedMode, tt.expectedMode)
		})
	}
}

func TestCreateMockFlow_TimeoutBehavior(t *testing.T) {
	// Create a mock flow with several nodes that have delays longer than the timeout
	params := MockFlowParams{
		RequestCount: 3,
		ForLoopCount: 2,
		Delay:        200 * time.Millisecond, // Each node takes 200ms
	}

	result := CreateMockFlow(params)
	totalNodes := len(result.Nodes)

	// Create FlowLocalRunner with a short timeout
	flowID := idwrap.NewNow()
	timeout := 100 * time.Millisecond // Shorter than any individual node delay
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		flowID,
		result.StartNodeID,
		result.Nodes,
		result.EdgesMap,
		timeout,
		nil,
	)

	// Force multi-mode execution to ensure timeout handling is tested
	flowRunner.SetExecutionMode(flowlocalrunner.ExecutionModeMulti)

	// Run the flow
	ctx := context.Background()
	nodeStatusChan := make(chan runner.FlowNodeStatus, totalNodes*2)
	flowStatusChan := make(chan runner.FlowStatus, 5)

	// Measure execution time to verify timeout occurs
	start := time.Now()
	err := flowRunner.Run(ctx, nodeStatusChan, flowStatusChan, nil)
	executionTime := time.Since(start)

	// Verify that timeout error is returned
	if err == nil {
		t.Error("Expected timeout error, but execution completed successfully")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
	}

	// Verify execution time is reasonable (should be close to timeout, not sum of all delays)
	if executionTime > 2*timeout {
		t.Errorf("Execution took %v, which is much longer than timeout %v", executionTime, timeout)
	}

	// Collect all node statuses
	nodeStatuses := make([]runner.FlowNodeStatus, 0)
	for status := range nodeStatusChan {
		nodeStatuses = append(nodeStatuses, status)
	}

	// Collect all flow statuses
	flowStatuses := make([]runner.FlowStatus, 0)
	for status := range flowStatusChan {
		flowStatuses = append(flowStatuses, status)
	}

	// Count nodes by state
	successCount := 0
	canceledCount := 0
	failureCount := 0

	for _, status := range nodeStatuses {
		switch status.State {
		case mnnode.NODE_STATE_SUCCESS:
			successCount++
		case mnnode.NODE_STATE_CANCELED:
			canceledCount++
		case mnnode.NODE_STATE_FAILURE:
			failureCount++
		}
	}

	// Verify that not all nodes succeeded (timeout should prevent completion)
	if successCount == totalNodes {
		t.Error("All nodes succeeded, but timeout should have prevented completion")
	}

	// Verify that we have some node status updates
	if len(nodeStatuses) == 0 {
		t.Error("No node status updates received")
	}

	// Verify flow status indicates failure
	if len(flowStatuses) == 0 {
		t.Error("Expected at least one flow status")
	} else {
		// Look for failure or timeout status in the flow statuses
		foundFailure := false
		for _, status := range flowStatuses {
			if status == runner.FlowStatusFailed || status == runner.FlowStatusTimeout {
				foundFailure = true
				break
			}
		}
		if !foundFailure {
			// Check if the last status is not success (which would indicate failure)
			lastStatus := flowStatuses[len(flowStatuses)-1]
			if lastStatus != runner.FlowStatusSuccess {
				foundFailure = true
			}
		}
		if !foundFailure {
			t.Errorf("Expected flow failure or timeout status due to timeout, got: %v", flowStatuses)
		}
	}

	t.Logf("Timeout test completed: %d total nodes, %d succeeded, %d canceled, %d failed, execution time: %v",
		totalNodes, successCount, canceledCount, failureCount, executionTime)
}

// Benchmark functions for performance CI

// BenchmarkCreateMockFlow_Small benchmarks flow creation with small parameters
func BenchmarkCreateMockFlow_Small(b *testing.B) {
	params := MockFlowParams{
		RequestCount: 2,
		ForLoopCount: 1,
		Delay:        0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateMockFlow(params)
	}
}

// BenchmarkCreateMockFlow_Medium benchmarks flow creation with medium parameters
func BenchmarkCreateMockFlow_Medium(b *testing.B) {
	params := MockFlowParams{
		RequestCount: 5,
		ForLoopCount: 3,
		Delay:        0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateMockFlow(params)
	}
}

// BenchmarkCreateMockFlow_Large benchmarks flow creation with large parameters
func BenchmarkCreateMockFlow_Large(b *testing.B) {
	params := MockFlowParams{
		RequestCount: 15,
		ForLoopCount: 8,
		Delay:        0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateMockFlow(params)
	}
}

// BenchmarkFlowExecution_Small benchmarks flow execution with small flows
func BenchmarkFlowExecution_Small(b *testing.B) {
	params := MockFlowParams{
		RequestCount: 2,
		ForLoopCount: 1,
		Delay:        1 * time.Millisecond,
	}

	result := CreateMockFlow(params)
	flowID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		flowID,
		result.StartNodeID,
		result.Nodes,
		result.EdgesMap,
		5*time.Second,
		nil,
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodeStatusChan := make(chan runner.FlowNodeStatus, len(result.Nodes)*2)
		flowStatusChan := make(chan runner.FlowStatus, 5)

		_ = flowRunner.Run(ctx, nodeStatusChan, flowStatusChan, nil)

		// Drain channels to avoid goroutine leaks
		for range nodeStatusChan {
		}
		for range flowStatusChan {
		}
	}
}

// BenchmarkFlowExecution_Medium benchmarks flow execution with medium flows
func BenchmarkFlowExecution_Medium(b *testing.B) {
	params := MockFlowParams{
		RequestCount: 5,
		ForLoopCount: 3,
		Delay:        1 * time.Millisecond,
	}

	result := CreateMockFlow(params)
	flowID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		flowID,
		result.StartNodeID,
		result.Nodes,
		result.EdgesMap,
		10*time.Second,
		nil,
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodeStatusChan := make(chan runner.FlowNodeStatus, len(result.Nodes)*2)
		flowStatusChan := make(chan runner.FlowStatus, 5)

		_ = flowRunner.Run(ctx, nodeStatusChan, flowStatusChan, nil)

		// Drain channels to avoid goroutine leaks
		for range nodeStatusChan {
		}
		for range flowStatusChan {
		}
	}
}

// BenchmarkFlowExecution_Large benchmarks flow execution with large flows
func BenchmarkFlowExecution_Large(b *testing.B) {
	params := MockFlowParams{
		RequestCount: 15,
		ForLoopCount: 8,
		Delay:        1 * time.Millisecond,
	}

	result := CreateMockFlow(params)
	flowID := idwrap.NewNow()
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		flowID,
		result.StartNodeID,
		result.Nodes,
		result.EdgesMap,
		30*time.Second,
		nil,
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodeStatusChan := make(chan runner.FlowNodeStatus, len(result.Nodes)*2)
		flowStatusChan := make(chan runner.FlowStatus, 5)

		_ = flowRunner.Run(ctx, nodeStatusChan, flowStatusChan, nil)

		// Drain channels to avoid goroutine leaks
		for range nodeStatusChan {
		}
		for range flowStatusChan {
		}
	}
}
