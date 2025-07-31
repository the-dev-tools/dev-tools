package nforeach

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
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForEachNode_FinalStateVerification(t *testing.T) {
	t.Run("ArrayIteration_EndWithSuccessState", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		forEachNode := New(nodeID, "TestForEachNode", "[1, 2, 3, 4]", 5*time.Second, 
			mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		var statusMutex sync.Mutex
		
		logPushFunc := func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			capturedStatuses = append(capturedStatuses, status)
			
			// Log each status for debugging
			t.Logf("Status: ID=%s, NodeID=%s, Name=%s, State=%s, ExecutionID=%s", 
				status.ExecutionID.String()[:8], 
				status.NodeID.String()[:8], 
				status.Name, 
				mnnode.StringNodeState(status.State),
				status.ExecutionID.String()[:8])
		}

		req := &node.FlowNodeRequest{
			VarMap:        map[string]any{"testArray": []any{1, 2, 3, 4}},
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "ForEach loop should complete successfully")
		
		statusMutex.Lock()
		defer statusMutex.Unlock()
		
		// Should have exactly 8 records (4 iterations × 2 records each: RUNNING + SUCCESS)
		assert.Len(t, capturedStatuses, 8, "Should have 8 records (4 RUNNING + 4 SUCCESS)")
		
		// Group records by ExecutionID to verify RUNNING -> SUCCESS transitions
		executionGroups := make(map[string][]runner.FlowNodeStatus)
		for _, status := range capturedStatuses {
			execID := status.ExecutionID.String()
			executionGroups[execID] = append(executionGroups[execID], status)
		}
		
		// Should have exactly 4 execution groups (one per iteration)
		assert.Len(t, executionGroups, 4, "Should have 4 unique ExecutionIDs")
		
		// Verify each execution group has exactly 2 records: RUNNING -> SUCCESS
		iterationIndex := 0
		for execID, statuses := range executionGroups {
			assert.Len(t, statuses, 2, fmt.Sprintf("ExecutionID %s should have exactly 2 records", execID[:8]))
			
			if len(statuses) == 2 {
				// First record should be RUNNING
				runningRecord := statuses[0]
				assert.Equal(t, mnnode.NODE_STATE_RUNNING, runningRecord.State, 
					fmt.Sprintf("First record for ExecutionID %s should be RUNNING", execID[:8]))
				assert.Equal(t, fmt.Sprintf("Iteration %d", iterationIndex), runningRecord.Name)
				
				// Second record should be SUCCESS
				successRecord := statuses[1]
				assert.Equal(t, mnnode.NODE_STATE_SUCCESS, successRecord.State, 
					fmt.Sprintf("Second record for ExecutionID %s should be SUCCESS", execID[:8]))
				assert.Equal(t, fmt.Sprintf("Iteration %d", iterationIndex), successRecord.Name)
				
				// Both records should have same ExecutionID
				assert.Equal(t, runningRecord.ExecutionID, successRecord.ExecutionID, 
					"RUNNING and SUCCESS records should share the same ExecutionID")
				
				// SUCCESS record should have completed flag
				if successData, ok := successRecord.OutputData.(map[string]any); ok {
					assert.Equal(t, true, successData["completed"], "SUCCESS record should have completed=true")
					assert.Equal(t, int64(iterationIndex), successData["index"], "Index should match iteration")
				}
			}
			
			iterationIndex++
		}
		
		// Verify the FINAL state of each iteration is SUCCESS (most important for UI restart)
		finalStates := make(map[string]mnnode.NodeState)
		for _, status := range capturedStatuses {
			execID := status.ExecutionID.String()
			// Last status for this ExecutionID is the final state
			finalStates[execID] = status.State
		}
		
		for execID, finalState := range finalStates {
			assert.Equal(t, mnnode.NODE_STATE_SUCCESS, finalState, 
				fmt.Sprintf("Final state for ExecutionID %s should be SUCCESS", execID[:8]))
		}
		
		t.Logf("✅ All %d iterations ended with SUCCESS state", len(finalStates))
	})
	
	t.Run("MapIteration_EndWithSuccessState", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		forEachNode := New(nodeID, "TestForEachNode", `{"a": 1, "b": 2, "c": 3}`, 5*time.Second, 
			mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		var statusMutex sync.Mutex
		
		logPushFunc := func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			capturedStatuses = append(capturedStatuses, status)
		}

		req := &node.FlowNodeRequest{
			VarMap:        map[string]any{"testMap": map[string]any{"a": 1, "b": 2, "c": 3}},
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "ForEach loop should complete successfully")
		
		statusMutex.Lock()
		defer statusMutex.Unlock()
		
		// Should have exactly 6 records (3 iterations × 2 records each: RUNNING + SUCCESS)
		assert.Len(t, capturedStatuses, 6, "Should have 6 records (3 RUNNING + 3 SUCCESS)")
		
		// Group records by ExecutionID
		executionGroups := make(map[string][]runner.FlowNodeStatus)
		for _, status := range capturedStatuses {
			execID := status.ExecutionID.String()
			executionGroups[execID] = append(executionGroups[execID], status)
		}
		
		// Should have exactly 3 execution groups (one per map entry)
		assert.Len(t, executionGroups, 3, "Should have 3 unique ExecutionIDs")
		
		// Verify final states are all SUCCESS
		finalStates := make(map[string]mnnode.NodeState)
		for _, status := range capturedStatuses {
			execID := status.ExecutionID.String()
			finalStates[execID] = status.State
		}
		
		successCount := 0
		for execID, finalState := range finalStates {
			if finalState == mnnode.NODE_STATE_SUCCESS {
				successCount++
			}
			assert.Equal(t, mnnode.NODE_STATE_SUCCESS, finalState, 
				fmt.Sprintf("Final state for ExecutionID %s should be SUCCESS", execID[:8]))
		}
		
		assert.Equal(t, 3, successCount, "All 3 map iterations should end with SUCCESS")
		t.Logf("✅ All %d map iterations ended with SUCCESS state", successCount)
	})
	
	t.Run("WithFailure_EndWithFailureState", func(t *testing.T) {
		// Arrange - Create a mock that fails on second iteration
		nodeID := idwrap.NewNow()
		forEachNode := New(nodeID, "TestForEachNode", "[1, 2, 3]", 5*time.Second, 
			mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		var statusMutex sync.Mutex
		
		logPushFunc := func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create a mock that fails
		failingNodeID := idwrap.NewNow()
		mockFailingNode := &MockNode{
			id: failingNodeID,
			shouldFail: true,
			failMessage: "simulated foreach failure",
		}
		
		nodeMap := map[idwrap.IDWrap]node.FlowNode{
			failingNodeID: mockFailingNode,
		}
		
		edgesMap := edge.EdgesMap{
			nodeID: map[edge.EdgeHandle][]idwrap.IDWrap{
				edge.HandleLoop: {failingNodeID},
			},
		}

		req := &node.FlowNodeRequest{
			VarMap:        map[string]any{"testArray": []any{1, 2, 3}},
			EdgeSourceMap: edgesMap,
			NodeMap:       nodeMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.Error(t, result.Err, "ForEach loop should fail")
		
		statusMutex.Lock()
		defer statusMutex.Unlock()
		
		// Should have at least 2 records for the first iteration (RUNNING + FAILURE)
		assert.GreaterOrEqual(t, len(capturedStatuses), 2, "Should have at least 2 records")
		
		// Find the final states for each ExecutionID
		finalStates := make(map[string]runner.FlowNodeStatus)
		for _, status := range capturedStatuses {
			execID := status.ExecutionID.String()
			finalStates[execID] = status // Last status for this ExecutionID
		}
		
		// At least one execution should end with FAILURE
		hasFailure := false
		for execID, finalStatus := range finalStates {
			if finalStatus.State == mnnode.NODE_STATE_FAILURE {
				hasFailure = true
				assert.NotNil(t, finalStatus.Error, fmt.Sprintf("FAILURE record %s should have error", execID[:8]))
				t.Logf("✅ ExecutionID %s correctly ended with FAILURE state", execID[:8])
			}
		}
		
		assert.True(t, hasFailure, "At least one iteration should end with FAILURE state")
	})
	
	t.Run("AsyncExecution_NoRaceConditions", func(t *testing.T) {
		// Arrange - Test async execution with race condition detection
		nodeID := idwrap.NewNow()
		forEachNode := New(nodeID, "TestForEachNode", "[1, 2, 3, 4, 5, 6, 7, 8]", 10*time.Millisecond, 
			mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		var statusMutex sync.RWMutex
		
		logPushFunc := func(status runner.FlowNodeStatus) {
			statusMutex.Lock()
			defer statusMutex.Unlock()
			capturedStatuses = append(capturedStatuses, status)
			
			// Add small delay to increase chance of race conditions
			time.Sleep(100 * time.Microsecond)
		}

		req := &node.FlowNodeRequest{
			VarMap:        map[string]any{"testArray": []any{1, 2, 3, 4, 5, 6, 7, 8}},
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act - Use async execution
		resultChan := make(chan node.FlowNodeResult, 1)
		forEachNode.RunAsync(context.Background(), req, resultChan)
		
		// Wait for completion with timeout
		select {
		case result := <-resultChan:
			require.NoError(t, result.Err, "Async ForEach loop should complete successfully")
		case <-time.After(5 * time.Second):
			t.Fatal("Async execution timed out")
		}

		// Give some time for all status updates to complete
		time.Sleep(100 * time.Millisecond)

		// Assert
		statusMutex.RLock()
		totalStatuses := len(capturedStatuses)
		statusesCopy := make([]runner.FlowNodeStatus, len(capturedStatuses))
		copy(statusesCopy, capturedStatuses)
		statusMutex.RUnlock()
		
		// Should have exactly 16 records (8 iterations × 2 records each)
		assert.Equal(t, 16, totalStatuses, "Should have exactly 16 status records")
		
		// Group by ExecutionID
		executionGroups := make(map[string][]runner.FlowNodeStatus)
		for _, status := range statusesCopy {
			execID := status.ExecutionID.String()
			executionGroups[execID] = append(executionGroups[execID], status)
		}
		
		// Should have exactly 8 unique ExecutionIDs
		assert.Len(t, executionGroups, 8, "Should have 8 unique ExecutionIDs")
		
		// Verify no race conditions: each ExecutionID has proper state transitions
		successCount := 0
		for execID, statuses := range executionGroups {
			assert.Len(t, statuses, 2, fmt.Sprintf("ExecutionID %s should have exactly 2 records", execID[:8]))
			
			if len(statuses) >= 2 {
				// Find RUNNING and SUCCESS records
				var runningFound, successFound bool
				for _, status := range statuses {
					switch status.State {
					case mnnode.NODE_STATE_RUNNING:
						runningFound = true
					case mnnode.NODE_STATE_SUCCESS:
						successFound = true
						successCount++
					}
				}
				
				assert.True(t, runningFound, fmt.Sprintf("ExecutionID %s should have RUNNING record", execID[:8]))
				assert.True(t, successFound, fmt.Sprintf("ExecutionID %s should have SUCCESS record", execID[:8]))
			}
		}
		
		assert.Equal(t, 8, successCount, "All 8 iterations should end with SUCCESS")
		t.Logf("✅ Async execution completed without race conditions - all %d iterations properly transitioned", successCount)
	})
}

// MockNode for testing failures  
type MockNode struct {
	id          idwrap.IDWrap
	shouldFail  bool
	failMessage string
}

func (m *MockNode) GetID() idwrap.IDWrap {
	return m.id
}

func (m *MockNode) GetName() string {
	return "MockNode"
}

func (m *MockNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if m.shouldFail {
		return node.FlowNodeResult{
			Err: fmt.Errorf("%s", m.failMessage),
		}
	}
	return node.FlowNodeResult{}
}

func (m *MockNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	if m.shouldFail {
		resultChan <- node.FlowNodeResult{
			Err: fmt.Errorf("%s", m.failMessage),
		}
	} else {
		resultChan <- node.FlowNodeResult{}
	}
}