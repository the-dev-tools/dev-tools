package nforeach

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForEachNodeConditionalSummary(t *testing.T) {
	t.Run("SuccessfulArrayLoop_NoSummaryRecord", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testArray", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create test array in variables
		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": []interface{}{"item1", "item2", "item3"},
			},
		}

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "Successful loop should not return error")

		// Filter to get only SUCCESS records (we now create both RUNNING and SUCCESS)
		var successStatuses []runner.FlowNodeStatus
		for _, status := range capturedStatuses {
			if status.State == mnnode.NODE_STATE_SUCCESS {
				successStatuses = append(successStatuses, status)
			}
		}

		// Should have exactly 3 SUCCESS records (no final summary)
		assert.Len(t, successStatuses, 3, "Should have exactly 3 SUCCESS iteration records for successful array loop")

		// Verify each iteration record
		expectedItems := []interface{}{"item1", "item2", "item3"}
		for i, status := range successStatuses {
			assert.Equal(t, nodeID, status.NodeID, "NodeID should match")
			assert.Equal(t, mnnode.NODE_STATE_SUCCESS, status.State, "All iteration records should be SUCCESS")
			expectedName := fmt.Sprintf("Iteration %d", i)
			assert.Equal(t, expectedName, status.Name, "Should have improved iteration naming")

			// Verify output data
			outputData, ok := status.OutputData.(map[string]any)
			require.True(t, ok, "OutputData should be a map")
			assert.Equal(t, i, outputData["key"], "Index should match iteration")
			assert.Equal(t, expectedItems[i], outputData["item"], "Value should match array item")
		}
	})

	t.Run("SuccessfulMapLoop_NoSummaryRecord", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testMap", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create test map in variables
		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testMap": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
		}

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "Successful map loop should not return error")

		// Filter for SUCCESS states only (we now get both RUNNING and SUCCESS for each iteration)
		successStatuses := make([]runner.FlowNodeStatus, 0)
		for _, status := range capturedStatuses {
			if status.State == mnnode.NODE_STATE_SUCCESS {
				successStatuses = append(successStatuses, status)
			}
		}

		// Should have exactly 2 SUCCESS iteration records (no final summary)
		assert.Len(t, successStatuses, 2, "Should have exactly 2 SUCCESS iteration records for successful map loop")

		// Verify iteration records (order might vary for maps)
		for _, status := range successStatuses {
			assert.Equal(t, nodeID, status.NodeID, "NodeID should match")
			assert.Equal(t, mnnode.NODE_STATE_SUCCESS, status.State, "All iteration records should be SUCCESS")

			// Check naming format for map iterations (should use Iteration format like arrays)
			assert.Contains(t, status.Name, "Iteration", "Should have iteration-based naming")

			// Verify output data structure
			outputData, ok := status.OutputData.(map[string]any)
			require.True(t, ok, "OutputData should be a map")
			assert.Contains(t, outputData, "key", "Should contain key field")
			assert.Contains(t, outputData, "item", "Should contain item field")
		}
	})

	t.Run("FailedArrayLoop_CreatesSummaryRecord", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testArray", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create test array in variables
		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": []interface{}{"item1", "item2", "item3"},
			},
		}

		// Create edge map that points to a child node (will simulate failure)
		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.Error(t, result.Err, "Failed loop should return error")

		// Should have 1 iteration record + 1 error summary record
		assert.Len(t, capturedStatuses, 2, "Should have 1 iteration record + 1 error summary")

		// First record should be iteration record
		iterationStatus := capturedStatuses[0]
		assert.Equal(t, nodeID, iterationStatus.NodeID)
		assert.Equal(t, mnnode.NODE_STATE_RUNNING, iterationStatus.State)
		expectedIterationName := "Iteration 0"
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
		assert.Equal(t, 0, summaryData["failedAtIndex"], "Should show failure at index 0")
		assert.Equal(t, 1, summaryData["totalItems"], "Should show total items processed before failure")
	})

	t.Run("FailedMapLoop_CreatesSummaryRecord", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testMap", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create test map in variables
		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testMap": map[string]interface{}{
					"key1": "value1",
				},
			},
		}

		// Create edge map that points to a child node (will simulate failure)
		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.Error(t, result.Err, "Failed map loop should return error")

		// Should have 1 iteration record + 1 error summary record
		assert.Len(t, capturedStatuses, 2, "Should have 1 iteration record + 1 error summary")

		// First record should be iteration record
		iterationStatus := capturedStatuses[0]
		assert.Equal(t, nodeID, iterationStatus.NodeID)
		assert.Equal(t, mnnode.NODE_STATE_RUNNING, iterationStatus.State)
		assert.Contains(t, iterationStatus.Name, "Iteration")

		// Second record should be error summary
		summaryStatus := capturedStatuses[1]
		assert.Equal(t, nodeID, summaryStatus.NodeID)
		assert.Equal(t, mnnode.NODE_STATE_FAILURE, summaryStatus.State)
		expectedSummaryName := "Error Summary"
		assert.Equal(t, expectedSummaryName, summaryStatus.Name)

		// Verify summary output data for map
		summaryData, ok := summaryStatus.OutputData.(map[string]interface{})
		require.True(t, ok, "Summary OutputData should be a map")
		assert.Equal(t, "key1", summaryData["failedAtKey"], "Should show failure at key1")
		assert.Equal(t, 1, summaryData["totalItems"], "Should show total items processed before failure")
	})

	t.Run("IgnoreErrorHandling_ContinuesAfterFailure", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testArray", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create test array in variables
		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": []interface{}{"item1", "item2"},
			},
		}

		// Create edge map that points to a child node (will simulate failure)
		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "IGNORE error handling should not propagate errors")

		// Should have exactly 2 iteration records (errors ignored, no summary)
		assert.Len(t, capturedStatuses, 2, "Should have exactly 2 iteration records when ignoring errors")

		// All records should be iteration records (no error summary)
		for i, status := range capturedStatuses {
			assert.Equal(t, mnnode.NODE_STATE_RUNNING, status.State, "All records should be iteration records")
			expectedName := fmt.Sprintf("Iteration %d", i)
			assert.Equal(t, expectedName, status.Name)
		}
	})

	t.Run("BreakErrorHandling_StopsEarly", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testArray", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_BREAK)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		// Create test array in variables
		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": []interface{}{"item1", "item2", "item3"},
			},
		}

		// Create edge map that points to a child node (will simulate failure)
		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err, "BREAK error handling should not propagate errors")

		// Should have exactly 1 iteration record (breaks early, no summary)
		assert.Len(t, capturedStatuses, 1, "Should have exactly 1 iteration record when breaking on error")

		// Verify the single record
		status := capturedStatuses[0]
		assert.Equal(t, mnnode.NODE_STATE_RUNNING, status.State, "Should be iteration record")
		expectedName := "Iteration 0"
		assert.Equal(t, expectedName, status.Name)
	})
}

func TestForEachNodeExecutionNaming(t *testing.T) {
	t.Run("ArrayIterationNamingFormat", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testArray", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": []interface{}{"a", "b"},
			},
		}

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err)

		// Filter for SUCCESS states only (we now get both RUNNING and SUCCESS for each iteration)
		successStatuses := make([]runner.FlowNodeStatus, 0)
		for _, status := range capturedStatuses {
			if status.State == mnnode.NODE_STATE_SUCCESS {
				successStatuses = append(successStatuses, status)
			}
		}
		require.Len(t, successStatuses, 2)

		// Verify naming format for array iterations
		expectedNames := []string{
			"Iteration 0",
			"Iteration 1",
		}

		for i, status := range successStatuses {
			assert.Equal(t, expectedNames[i], status.Name, "Should follow Iteration N format")
		}
	})

	t.Run("MapIterationNamingFormat", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testMap", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testMap": map[string]interface{}{
					"testKey": "testValue",
				},
			},
		}

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: make(edge.EdgesMap),
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.NoError(t, result.Err)

		// Filter for SUCCESS states only (we now get both RUNNING and SUCCESS for each iteration)
		successStatuses := make([]runner.FlowNodeStatus, 0)
		for _, status := range capturedStatuses {
			if status.State == mnnode.NODE_STATE_SUCCESS {
				successStatuses = append(successStatuses, status)
			}
		}
		require.Len(t, successStatuses, 1)

		// Verify naming format for map iteration (should use Iteration format)
		status := successStatuses[0]
		expectedName := "Iteration 0"
		assert.Equal(t, expectedName, status.Name, "Should follow Iteration N format")
	})

	t.Run("ErrorSummaryNamingFormat", func(t *testing.T) {
		// Arrange
		nodeID := idwrap.NewNow()
		childNodeID := idwrap.NewNow()
		condition := mcondition.Condition{} // Empty condition
		forEachNode := New(nodeID, "TestForEachNode", "var.testArray", 5*time.Second, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

		var capturedStatuses []runner.FlowNodeStatus
		logPushFunc := func(status runner.FlowNodeStatus) {
			capturedStatuses = append(capturedStatuses, status)
		}

		varMap := map[string]interface{}{
			"var": map[string]interface{}{
				"testArray": []interface{}{"item1"},
			},
		}

		edges := []edge.Edge{
			edge.NewEdge(idwrap.NewNow(), nodeID, childNodeID, edge.HandleLoop, edge.EdgeKindNoOp),
		}
		edgeSourceMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        varMap,
			EdgeSourceMap: edgeSourceMap,
			LogPushFunc:   logPushFunc,
			ReadWriteLock: &sync.RWMutex{},
		}

		// Act
		result := forEachNode.RunSync(context.Background(), req)

		// Assert
		require.Error(t, result.Err)
		require.Len(t, capturedStatuses, 2) // 1 iteration + 1 error summary

		// Verify error summary naming
		summaryStatus := capturedStatuses[1]
		expectedSummaryName := "Error Summary"
		assert.Equal(t, expectedSummaryName, summaryStatus.Name, "Should follow Error Summary format")
	})
}
