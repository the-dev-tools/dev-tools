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
		
		// Should have exactly 4 records (4 iterations with RUNNING only, no SUCCESS for successful loops)
		assert.Len(t, capturedStatuses, 4, "Should have 4 records (RUNNING only for successful loops)")
		
		// Verify each record is RUNNING and follows proper naming format
		for i, status := range capturedStatuses {
			assert.Equal(t, mnnode.NODE_STATE_RUNNING, status.State, 
				fmt.Sprintf("Record %d should be RUNNING (successful loops don't create SUCCESS records)", i))
			assert.Equal(t, fmt.Sprintf("Iteration %d", i), status.Name, "Should follow Iteration N format")
			
			// Verify output data
			if outputData, ok := status.OutputData.(map[string]any); ok {
				assert.Equal(t, i, outputData["index"], "Index should match iteration")
				assert.Equal(t, i+1, outputData["value"], "Value should match array item")
			}
		}
		
		t.Logf("✅ All %d iterations recorded as RUNNING (conditional summary: no SUCCESS for successful loops)", len(capturedStatuses))
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
		
		// Should have exactly 3 records (3 iterations with RUNNING only, no SUCCESS for successful loops)
		assert.Len(t, capturedStatuses, 3, "Should have 3 records (RUNNING only for successful loops)")
		
		// Verify each record is RUNNING and follows proper naming format
		for i, status := range capturedStatuses {
			assert.Equal(t, mnnode.NODE_STATE_RUNNING, status.State, 
				fmt.Sprintf("Record %d should be RUNNING (successful loops don't create SUCCESS records)", i))
			assert.Equal(t, fmt.Sprintf("Iteration %d", i), status.Name, "Should follow Iteration N format")
			
			// Verify output data structure for map iteration
			if outputData, ok := status.OutputData.(map[string]any); ok {
				assert.Contains(t, outputData, "key", "Should contain key field")
				assert.Contains(t, outputData, "value", "Should contain value field")
			}
		}
		
		t.Logf("✅ All %d map iterations recorded as RUNNING (conditional summary: no SUCCESS for successful loops)", len(capturedStatuses))
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
		
		// Filter out MockNode records - only look at ForEach iteration and error records
		forEachRecords := []runner.FlowNodeStatus{}
		for _, status := range capturedStatuses {
			if status.NodeID == nodeID {
				forEachRecords = append(forEachRecords, status)
			}
		}
		
		// Should have RUNNING record + Error Summary (no FAILURE updates to iterations)
		assert.Len(t, forEachRecords, 2, "Should have 1 RUNNING + 1 Error Summary (conditional summary behavior)")
		
		// First record should be RUNNING for first iteration
		runningStatus := forEachRecords[0]
		assert.Equal(t, mnnode.NODE_STATE_RUNNING, runningStatus.State, "First record should be RUNNING")
		assert.Equal(t, "Iteration 0", runningStatus.Name, "Should follow Iteration N format")
		
		// Second record should be Error Summary
		summaryStatus := forEachRecords[1]
		assert.Equal(t, mnnode.NODE_STATE_FAILURE, summaryStatus.State, "Second record should be Error Summary")
		assert.Equal(t, "Error Summary", summaryStatus.Name, "Should have Error Summary name")
		assert.NotNil(t, summaryStatus.Error, "Error Summary should have error")
		
		// Verify error summary output data
		if summaryData, ok := summaryStatus.OutputData.(map[string]interface{}); ok {
			assert.Equal(t, 0, summaryData["failedAtIndex"], "Should show failure at index 0")
			assert.Equal(t, 1, summaryData["totalItems"], "Should show 1 item processed before failure")
		}
		
		t.Logf("✅ Failure correctly handled with conditional summary (RUNNING + Error Summary only)")
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
		
		// Should have exactly 8 records (8 iterations with RUNNING only, no SUCCESS for successful loops)
		assert.Equal(t, 8, totalStatuses, "Should have exactly 8 status records (RUNNING only)")
		
		// Verify all records are RUNNING (no SUCCESS records for successful loops)
		runningCount := 0
		for _, status := range statusesCopy {
			assert.Equal(t, mnnode.NODE_STATE_RUNNING, status.State, "All records should be RUNNING for successful loops")
			runningCount++
		}
		
		assert.Equal(t, 8, runningCount, "All 8 iterations should be RUNNING only")
		t.Logf("✅ Async execution completed without race conditions - all %d iterations recorded as RUNNING (conditional summary)", runningCount)
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