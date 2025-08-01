package nfor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForNodeConditionalSummary(t *testing.T) {
	t.Run("SuccessfulLoop_NoSummaryRecord", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		forNode := New(nodeID, "TestForNode", 3, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		req := &node.FlowNodeRequest{
			VarMap:       make(map[string]interface{}),
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "Successful loop should not return error")
		
		// Should have exactly 6 iteration records (3 RUNNING + 3 SUCCESS, no final summary)
		assert.Len(t, capturedStatuses, 6, "Should have exactly 6 iteration records for successful loop (RUNNING + SUCCESS for each)")
		
		// Verify iteration records - should have pairs of RUNNING then SUCCESS
		for i := 0; i < 3; i++ {
			runningIndex := i * 2
			successIndex := i * 2 + 1
			
			// Verify RUNNING record
			runningStatus := capturedStatuses[runningIndex]
			assert.Equal(t, nodeID, runningStatus.NodeID, "NodeID should match")
			assert.Equal(t, mnnode.NODE_STATE_RUNNING, runningStatus.State, "First record should be RUNNING")
			expectedName := fmt.Sprintf("TestForNode iteration %d", i+1)
			assert.Equal(t, expectedName, runningStatus.Name, "Should have improved iteration naming")
			
			runningOutputData, ok := runningStatus.OutputData.(map[string]interface{})
			require.True(t, ok, "OutputData should be a map")
			assert.Equal(t, int64(i), runningOutputData["index"], "Index should match iteration")
			
			// Verify SUCCESS record
			successStatus := capturedStatuses[successIndex]
			assert.Equal(t, nodeID, successStatus.NodeID, "NodeID should match")
			assert.Equal(t, mnnode.NODE_STATE_SUCCESS, successStatus.State, "Second record should be SUCCESS")
			assert.Equal(t, expectedName, successStatus.Name, "Should have same iteration naming")
			
			successOutputData, ok := successStatus.OutputData.(map[string]interface{})
			require.True(t, ok, "OutputData should be a map")
			assert.Equal(t, int64(i), successOutputData["index"], "Index should match iteration")
			assert.Equal(t, true, successOutputData["completed"], "Should have completion flag")
			
			// Verify same ExecutionID for both records (update logic)
			assert.Equal(t, runningStatus.ExecutionID, successStatus.ExecutionID, "RUNNING and SUCCESS records should have same ExecutionID")
		}
	})

	t.Run("FailedLoop_CreatesSummaryRecord", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		forNode := New(nodeID, "TestForNode", 5, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create edge map that points to a child node (will simulate failure)
		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]interface{}),
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forNode.RunSync(context.Background(), req)

		// Assert
		// The test will fail at iteration 0 because flowlocalrunner.RunNodeSync 
		// will return an error for the non-existent child node
		require.Error(t, result.Err, "Failed loop should return error")
		
		// Should have 1 iteration record + 1 error summary record
		assert.Len(t, capturedStatuses, 2, "Should have 1 iteration record + 1 error summary")
		
		// First record should be iteration record
		iterationStatus := capturedStatuses[0]
		assert.Equal(t, nodeID, iterationStatus.NodeID)
		assert.Equal(t, mnnode.NODE_STATE_RUNNING, iterationStatus.State)
		expectedIterationName := "TestForNode iteration 1"
		assert.Equal(t, expectedIterationName, iterationStatus.Name)
		
		// Second record should be error summary
		summaryStatus := capturedStatuses[1]
		assert.Equal(t, nodeID, summaryStatus.NodeID)
		assert.Equal(t, mnnode.NODE_STATE_FAILURE, summaryStatus.State)
		expectedSummaryName := "Error Summary"
		assert.Equal(t, expectedSummaryName, summaryStatus.Name)
		
		// Verify summary output data
		summaryData, ok := summaryStatus.OutputData.(map[string]interface{})
		require.True(t, ok, "Summary OutputData should be a map")
		assert.Equal(t, int64(0), summaryData["failedAtIteration"], "Should show failure at iteration 0")
		assert.Equal(t, int64(5), summaryData["totalIterations"], "Should show total iterations planned")
	})

	t.Run("IgnoreErrorHandling_ContinuesAfterFailure", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		forNode := New(nodeID, "TestForNode", 3, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
		
		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create edge map that points to a child node (will simulate failure)
		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]interface{}),
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "IGNORE error handling should not propagate errors")
		
		// Should have exactly 3 iteration records (errors ignored, no summary)
		assert.Len(t, capturedStatuses, 3, "Should have exactly 3 iteration records when ignoring errors")
		
		// All records should be iteration records (no error summary)
		for i, status := range capturedStatuses {
			assert.Equal(t, mnnode.NODE_STATE_RUNNING, status.State, "All records should be iteration records")
			expectedName := fmt.Sprintf("TestForNode iteration %d", i+1)
			assert.Equal(t, expectedName, status.Name)
		}
	})

	t.Run("BreakErrorHandling_StopsEarly", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		forNode := New(nodeID, "TestForNode", 5, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_BREAK)
		
		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create edge map that points to a child node (will simulate failure)
		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]interface{}),
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "BREAK error handling should not propagate errors")
		
		// Should have exactly 1 iteration record (breaks early, no summary)
		assert.Len(t, capturedStatuses, 1, "Should have exactly 1 iteration record when breaking on error")
		
		// Verify the single record
		status := capturedStatuses[0]
		assert.Equal(t, mnnode.NODE_STATE_RUNNING, status.State, "Should be iteration record")
		expectedName := "TestForNode iteration 1"
		assert.Equal(t, expectedName, status.Name)
	})
}

func TestForNodeExecutionNaming(t *testing.T) {
	t.Run("IterationNamingFormat", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		forNode := New(nodeID, "TestForNode", 2, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		req := &node.FlowNodeRequest{
			VarMap:       make(map[string]interface{}),
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err)
		require.Len(t, capturedStatuses, 2)
		
		// Verify naming format
		expectedNames := []string{
			"TestForNode iteration 1",
			"TestForNode iteration 2",
		}
		
		for i, status := range capturedStatuses {
			assert.Equal(t, expectedNames[i], status.Name, "Should follow Iteration N format")
		}
	})

	t.Run("ErrorSummaryNamingFormat", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		forNode := New(nodeID, "TestForNode", 2, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
		
		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        make(map[string]interface{}),
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forNode.RunSync(context.Background(), req)

		// Assert
		require.Error(t, result.Err)
		require.Len(t, capturedStatuses, 2) // 1 iteration + 1 error summary
		
		// Verify error summary naming
		summaryStatus := capturedStatuses[1]
		expectedSummaryName := "Error Summary"
		assert.Equal(t, expectedSummaryName, summaryStatus.Name, "Should follow Error Summary format")
	})
}