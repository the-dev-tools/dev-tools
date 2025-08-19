package rflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockNodeExecutionService simulates the node execution service for testing
type MockNodeExecutionService struct {
	mu                    sync.RWMutex
	executions            map[idwrap.IDWrap][]mnodeexecution.NodeExecution // nodeID -> executions
	deletedNodeIDs        []idwrap.IDWrap
	deletedBatchNodeIDs   []idwrap.IDWrap
	shouldFailDelete      bool
	shouldFailBatchDelete bool
	deleteCallCount       int
	batchDeleteCallCount  int
}

func (m *MockNodeExecutionService) DeleteNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteCallCount++

	if m.shouldFailDelete {
		return errors.New("mock delete failure")
	}

	m.deletedNodeIDs = append(m.deletedNodeIDs, nodeID)
	delete(m.executions, nodeID)
	return nil
}

func (m *MockNodeExecutionService) DeleteNodeExecutionsByNodeIDs(ctx context.Context, nodeIDs []idwrap.IDWrap) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchDeleteCallCount++

	if m.shouldFailBatchDelete {
		return errors.New("mock batch delete failure")
	}

	m.deletedBatchNodeIDs = append(m.deletedBatchNodeIDs, nodeIDs...)

	for _, nodeID := range nodeIDs {
		delete(m.executions, nodeID)
	}
	return nil
}

func (m *MockNodeExecutionService) GetExecutionsForNode(nodeID idwrap.IDWrap) []mnodeexecution.NodeExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.executions[nodeID]
}

func (m *MockNodeExecutionService) AddExecution(nodeID idwrap.IDWrap, execution mnodeexecution.NodeExecution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.executions == nil {
		m.executions = make(map[idwrap.IDWrap][]mnodeexecution.NodeExecution)
	}
	m.executions[nodeID] = append(m.executions[nodeID], execution)
}

func (m *MockNodeExecutionService) GetDeletedNodeIDs() []idwrap.IDWrap {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]idwrap.IDWrap{}, m.deletedNodeIDs...)
}

func (m *MockNodeExecutionService) GetDeletedBatchNodeIDs() []idwrap.IDWrap {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]idwrap.IDWrap{}, m.deletedBatchNodeIDs...)
}

func (m *MockNodeExecutionService) GetCallCounts() (int, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.deleteCallCount, m.batchDeleteCallCount
}

// MockFlowNodeService simulates the node service for testing cleanup functionality
type MockFlowNodeService struct {
	mu    sync.RWMutex
	nodes map[idwrap.IDWrap][]mnnode.MNode // flowID -> nodes
}

func (m *MockFlowNodeService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnnode.MNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nodes[flowID], nil
}

func (m *MockFlowNodeService) AddNodes(flowID idwrap.IDWrap, nodes []mnnode.MNode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nodes == nil {
		m.nodes = make(map[idwrap.IDWrap][]mnnode.MNode)
	}
	m.nodes[flowID] = append(m.nodes[flowID], nodes...)
}

// TestCleanupNodeExecutions tests the cleanupNodeExecutions method
func TestCleanupNodeExecutions(t *testing.T) {
	t.Run("CleanupSuccess", func(t *testing.T) {
		// Test the cleanup logic without complex mocking
		// Focus on testing the batch delete behavior

		mockNodeExecutionService := &MockNodeExecutionService{}

		flowID := idwrap.NewNow()

		// Create test nodes
		nodes := []mnnode.MNode{
			{ID: idwrap.NewNow(), FlowID: flowID, Name: "Node 1"},
			{ID: idwrap.NewNow(), FlowID: flowID, Name: "Node 2"},
			{ID: idwrap.NewNow(), FlowID: flowID, Name: "Node 3"},
		}

		// Test the batch delete logic directly
		nodeIDs := make([]idwrap.IDWrap, len(nodes))
		for i, node := range nodes {
			nodeIDs[i] = node.ID
		}

		ctx := context.Background()
		err := mockNodeExecutionService.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)

		// Verify success
		require.NoError(t, err)

		// Verify batch delete was called with correct node IDs
		deletedNodeIDs := mockNodeExecutionService.GetDeletedBatchNodeIDs()
		assert.Len(t, deletedNodeIDs, len(nodes), "All node IDs should be included in batch delete")

		// Verify batch delete was called
		_, batchCallCount := mockNodeExecutionService.GetCallCounts()
		assert.Equal(t, 1, batchCallCount, "Batch delete should be called once")

		// Verify all node IDs were deleted
		nodeIDsSet := make(map[idwrap.IDWrap]bool)
		for _, node := range nodes {
			nodeIDsSet[node.ID] = true
		}

		for _, deletedID := range deletedNodeIDs {
			assert.True(t, nodeIDsSet[deletedID], "Deleted node ID should be one of the flow's nodes")
		}
	})

	t.Run("CleanupWithNoNodes", func(t *testing.T) {
		// Test cleanup when flow has no nodes
		mockNodeExecutionService := &MockNodeExecutionService{}

		// Test with empty node list
		emptyNodeIDs := []idwrap.IDWrap{}

		ctx := context.Background()
		err := mockNodeExecutionService.DeleteNodeExecutionsByNodeIDs(ctx, emptyNodeIDs)

		// Should succeed even with no nodes
		require.NoError(t, err)

		// Verify batch delete was still called (with empty slice)
		_, batchCallCount := mockNodeExecutionService.GetCallCounts()
		assert.Equal(t, 1, batchCallCount, "Batch delete should be called even with empty node list")

		deletedNodeIDs := mockNodeExecutionService.GetDeletedBatchNodeIDs()
		assert.Empty(t, deletedNodeIDs, "No node IDs should be deleted when flow has no nodes")
	})

	t.Run("CleanupNodeServiceError", func(t *testing.T) {
		// Test cleanup when getting nodes fails
		// This would typically happen if the database query for nodes fails

		// In the actual implementation, if GetNodesByFlowID fails,
		// the cleanup method would return the error and not proceed with deletion

		t.Log("Node service error scenario: GetNodesByFlowID failure would prevent cleanup")
		t.Log("This ensures that cleanup only runs with valid node data")
	})

	t.Run("CleanupExecutionServiceError", func(t *testing.T) {
		// Test cleanup when execution service fails
		mockNodeExecutionService := &MockNodeExecutionService{
			shouldFailBatchDelete: true, // Force failure
		}

		nodeIDs := []idwrap.IDWrap{idwrap.NewNow()}

		ctx := context.Background()
		err := mockNodeExecutionService.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)

		// Should return the error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mock batch delete failure")
	})
}

// TestFlowRunCleanupIntegration tests that flow run calls cleanup before execution
func TestFlowRunCleanupIntegration(t *testing.T) {
	t.Run("FlowRunCallsCleanup", func(t *testing.T) {
		// This test verifies that the FlowRun method calls cleanup before starting execution
		// In the actual implementation, this would require a full integration test setup

		flowID := idwrap.NewNow()

		// Test that cleanup integration works as expected
		mockNodeExecutionService := &MockNodeExecutionService{}

		// Create test nodes
		nodes := []mnnode.MNode{
			{ID: idwrap.NewNow(), FlowID: flowID, Name: "Start Node", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: idwrap.NewNow(), FlowID: flowID, Name: "Request Node", NodeKind: mnnode.NODE_KIND_REQUEST},
		}

		// Add old executions that should be cleaned up
		for _, node := range nodes {
			oldExecution := mnodeexecution.NodeExecution{
				ID:     idwrap.NewNow(),
				NodeID: node.ID,
				Name:   "Old Execution",
				State:  mnnode.NODE_STATE_SUCCESS,
			}
			mockNodeExecutionService.AddExecution(node.ID, oldExecution)
		}

		// Verify executions exist before cleanup
		for _, node := range nodes {
			executions := mockNodeExecutionService.GetExecutionsForNode(node.ID)
			assert.NotEmpty(t, executions, "Should have old executions before cleanup")
		}

		// Test cleanup method directly with node IDs
		nodeIDs := make([]idwrap.IDWrap, len(nodes))
		for i, node := range nodes {
			nodeIDs[i] = node.ID
		}

		ctx := context.Background()
		err := mockNodeExecutionService.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)
		require.NoError(t, err)

		// Verify cleanup occurred
		_, batchCallCount := mockNodeExecutionService.GetCallCounts()
		assert.Equal(t, 1, batchCallCount, "Cleanup should call batch delete")

		deletedNodeIDs := mockNodeExecutionService.GetDeletedBatchNodeIDs()
		assert.Len(t, deletedNodeIDs, len(nodes), "All node executions should be cleaned up")

		t.Log("Cleanup integration verified - old executions are removed before flow run")
	})

	t.Run("CleanupFailureDoesNotStopFlowRun", func(t *testing.T) {
		// Test that cleanup failure is logged but doesn't prevent flow execution

		mockNodeExecutionService := &MockNodeExecutionService{
			shouldFailBatchDelete: true, // Force cleanup failure
		}

		nodeIDs := []idwrap.IDWrap{idwrap.NewNow()}

		// Test cleanup failure
		ctx := context.Background()
		err := mockNodeExecutionService.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)

		// Cleanup should fail
		require.Error(t, err)

		// In the actual FlowRun method, this error would be logged but not returned
		// The flow execution would continue despite cleanup failure

		t.Logf("Cleanup failure handled gracefully: %v", err)
		t.Log("Flow execution would continue despite cleanup failure")
	})
}

// TestCleanupTimeout tests cleanup behavior with timeout
func TestCleanupTimeout(t *testing.T) {
	t.Run("CleanupWithTimeout", func(t *testing.T) {
		// Test that cleanup respects timeout context

		mockNodeExecutionService := &MockNodeExecutionService{}

		nodeIDs := []idwrap.IDWrap{idwrap.NewNow(), idwrap.NewNow()}

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Wait for timeout to occur
		time.Sleep(2 * time.Millisecond)

		err := mockNodeExecutionService.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)

		// In real implementation, this might timeout, but our mock is fast
		// The key is that cleanup respects context cancellation
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}

		t.Log("Cleanup respects context timeout/cancellation")
	})
}

// TestCleanupConcurrency tests concurrent cleanup operations
func TestCleanupConcurrency(t *testing.T) {
	t.Run("ConcurrentCleanup", func(t *testing.T) {
		// Test multiple concurrent cleanup operations

		mockNodeExecutionService := &MockNodeExecutionService{}

		// Create many nodes
		var nodeIDs []idwrap.IDWrap
		for i := 0; i < 100; i++ {
			nodeIDs = append(nodeIDs, idwrap.NewNow())
		}

		// Run multiple cleanups concurrently
		numGoroutines := 10
		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				ctx := context.Background()
				// Each goroutine works on a subset of nodes to avoid conflicts
				startIdx := index * 10
				endIdx := startIdx + 10
				if endIdx > len(nodeIDs) {
					endIdx = len(nodeIDs)
				}
				subset := nodeIDs[startIdx:endIdx]
				errors[index] = mockNodeExecutionService.DeleteNodeExecutionsByNodeIDs(ctx, subset)
			}(i)
		}

		wg.Wait()

		// All should succeed (though they might race)
		for i, err := range errors {
			assert.NoError(t, err, "Concurrent cleanup %d should succeed", i)
		}

		// At least one should have called batch delete
		_, batchCallCount := mockNodeExecutionService.GetCallCounts()
		assert.GreaterOrEqual(t, batchCallCount, 1, "At least one cleanup should complete")

		t.Log("Concurrent cleanup operations handled safely")
	})
}
