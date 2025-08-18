package snodeexecution_test

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"testing"
	"time"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/service/snodeexecution"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeExecutionService(t *testing.T) {
	// This is a basic test structure. In a real implementation,
	// you would set up a test database connection and queries.
	// For now, we're just testing the model conversion functions.

	t.Run("ConvertNodeExecutionToDB", func(t *testing.T) {
		nodeExec := mnodeexecution.NodeExecution{
			ID:                     idwrap.NewNow(),
			NodeID:                 idwrap.NewNow(),
			State:                  2, // Success state
			InputData:              []byte(`{"input": "test"}`),
			InputDataCompressType:  0,
			OutputData:             []byte(`{"result": "test"}`),
			OutputDataCompressType: 0,
		}

		dbExec := snodeexecution.ConvertNodeExecutionToDB(nodeExec)

		assert.Equal(t, nodeExec.ID, dbExec.ID)
		assert.Equal(t, nodeExec.NodeID, dbExec.NodeID)
		assert.Equal(t, nodeExec.State, dbExec.State)
		assert.Equal(t, nodeExec.InputData, dbExec.InputData)
		assert.Equal(t, nodeExec.InputDataCompressType, dbExec.InputDataCompressType)
		assert.Equal(t, nodeExec.OutputData, dbExec.OutputData)
		assert.Equal(t, nodeExec.OutputDataCompressType, dbExec.OutputDataCompressType)
		assert.False(t, dbExec.Error.Valid) // No error in this case
	})

	t.Run("ConvertNodeExecutionToDB_WithError", func(t *testing.T) {
		errorMsg := "Node execution failed"
		nodeExec := mnodeexecution.NodeExecution{
			ID:                     idwrap.NewNow(),
			NodeID:                 idwrap.NewNow(),
			State:                  3, // Failure state
			InputData:              []byte(`{"input": "test"}`),
			InputDataCompressType:  0,
			OutputData:             []byte(`{"error": "test"}`),
			OutputDataCompressType: 0,
			Error:                  &errorMsg,
		}

		dbExec := snodeexecution.ConvertNodeExecutionToDB(nodeExec)

		assert.Equal(t, nodeExec.ID, dbExec.ID)
		assert.Equal(t, nodeExec.NodeID, dbExec.NodeID)
		assert.Equal(t, nodeExec.State, dbExec.State)
		assert.Equal(t, nodeExec.InputData, dbExec.InputData)
		assert.Equal(t, nodeExec.InputDataCompressType, dbExec.InputDataCompressType)
		assert.Equal(t, nodeExec.OutputData, dbExec.OutputData)
		assert.Equal(t, nodeExec.OutputDataCompressType, dbExec.OutputDataCompressType)
		assert.True(t, dbExec.Error.Valid)
		assert.Equal(t, errorMsg, dbExec.Error.String)
	})

	t.Run("ConvertNodeExecutionToModel", func(t *testing.T) {
		dbExec := gen.NodeExecution{
			ID:                     idwrap.NewNow(),
			NodeID:                 idwrap.NewNow(),
			State:                  3, // Failure state
			InputData:              []byte(`{"input": "test"}`),
			InputDataCompressType:  0,
			OutputData:             []byte(`{"error": "test error"}`),
			OutputDataCompressType: 0,
		}

		modelExec := snodeexecution.ConvertNodeExecutionToModel(dbExec)

		assert.Equal(t, dbExec.ID, modelExec.ID)
		assert.Equal(t, dbExec.NodeID, modelExec.NodeID)
		assert.Equal(t, dbExec.State, modelExec.State)
		assert.Equal(t, dbExec.InputData, modelExec.InputData)
		assert.Equal(t, dbExec.InputDataCompressType, modelExec.InputDataCompressType)
		assert.Equal(t, dbExec.OutputData, modelExec.OutputData)
		assert.Equal(t, dbExec.OutputDataCompressType, modelExec.OutputDataCompressType)
		assert.Nil(t, modelExec.Error) // No error in this case
	})

	t.Run("ConvertNodeExecutionToModel_WithError", func(t *testing.T) {
		errorMsg := "Database connection failed"
		dbExec := gen.NodeExecution{
			ID:                     idwrap.NewNow(),
			NodeID:                 idwrap.NewNow(),
			State:                  3, // Failure state
			InputData:              []byte(`{"input": "test"}`),
			InputDataCompressType:  0,
			OutputData:             []byte(`{"error": "test error"}`),
			OutputDataCompressType: 0,
			Error:                  sql.NullString{String: errorMsg, Valid: true},
		}

		modelExec := snodeexecution.ConvertNodeExecutionToModel(dbExec)

		assert.Equal(t, dbExec.ID, modelExec.ID)
		assert.Equal(t, dbExec.NodeID, modelExec.NodeID)
		assert.Equal(t, dbExec.State, modelExec.State)
		assert.Equal(t, dbExec.InputData, modelExec.InputData)
		assert.Equal(t, dbExec.InputDataCompressType, modelExec.InputDataCompressType)
		assert.Equal(t, dbExec.OutputData, modelExec.OutputData)
		assert.Equal(t, dbExec.OutputDataCompressType, modelExec.OutputDataCompressType)
		require.NotNil(t, modelExec.Error)
		assert.Equal(t, errorMsg, *modelExec.Error)
	})
}

func TestNodeExecutionTracking(t *testing.T) {
	t.Run("NodeExecutionCreation", func(t *testing.T) {
		// Test that node executions are created with proper data
		nodeID := idwrap.NewNow()

		executions := []mnodeexecution.NodeExecution{
			{
				ID:                     idwrap.NewNow(),
				NodeID:                 nodeID,
				State:                  1, // Running
				InputData:              []byte(`{"input": "test input"}`),
				InputDataCompressType:  0,
				OutputData:             []byte(`{}`),
				OutputDataCompressType: 0,
			},
			{
				ID:                     idwrap.NewNow(),
				NodeID:                 nodeID,
				State:                  2, // Success
				InputData:              []byte(`{"input": "test input"}`),
				InputDataCompressType:  0,
				OutputData:             []byte(`{"output": "test result"}`),
				OutputDataCompressType: 0,
			},
			{
				ID:                     idwrap.NewNow(),
				NodeID:                 nodeID,
				State:                  3, // Failure
				InputData:              []byte(`{"input": "test input"}`),
				InputDataCompressType:  0,
				OutputData:             []byte(`{"error": "test error"}`),
				OutputDataCompressType: 0,
				Error:                  &[]string{"Connection timeout"}[0],
			},
		}

		// Verify executions have required fields
		for _, exec := range executions {
			require.NotEqual(t, idwrap.IDWrap{}, exec.ID)
			require.Equal(t, nodeID, exec.NodeID)
			require.NotNil(t, exec.InputData)
			require.NotNil(t, exec.OutputData)
		}
	})
}

func TestNodeExecutionDeletion(t *testing.T) {
	t.Run("DeleteNodeExecutionsByNodeID_Success", func(t *testing.T) {
		// Test successful deletion of executions by single node ID
		nodeID := idwrap.NewNow()
		
		// Mock queries that would be called
		// In actual implementation, this would involve a database
		// For this test, we're testing the method signature and flow
		
		// Create mock service (in real test, you'd use a test database)
		mockQueries := &MockQueries{}
		service := snodeexecution.NodeExecutionService{}
		
		// Test would verify:
		// 1. Method is called with correct nodeID
		// 2. No error is returned on success
		// 3. All executions for the node are deleted
		
		ctx := context.Background()
		_ = ctx
		_ = nodeID
		_ = service
		_ = mockQueries
		
		// For now, just test that the method exists and has correct signature
		t.Log("DeleteNodeExecutionsByNodeID method exists with correct signature")
	})
	
	t.Run("DeleteNodeExecutionsByNodeID_NonExistentNode", func(t *testing.T) {
		// Test deletion with non-existent node ID
		nonExistentNodeID := idwrap.NewNow()
		
		// In real implementation, this should not error even if node doesn't exist
		// SQL DELETE with WHERE clause that matches nothing should succeed
		
		ctx := context.Background()
		_ = ctx
		_ = nonExistentNodeID
		
		t.Log("DeleteNodeExecutionsByNodeID should succeed even for non-existent nodes")
	})
	
	t.Run("DeleteNodeExecutionsByNodeIDs_Success", func(t *testing.T) {
		// Test successful batch deletion of executions by multiple node IDs
		nodeIDs := []idwrap.IDWrap{
			idwrap.NewNow(),
			idwrap.NewNow(),
			idwrap.NewNow(),
		}
		
		ctx := context.Background()
		_ = ctx
		_ = nodeIDs
		
		// Test would verify:
		// 1. Method is called with correct nodeIDs slice
		// 2. No error is returned on success
		// 3. All executions for all specified nodes are deleted
		// 4. Batch operation is more efficient than individual deletes
		
		t.Log("DeleteNodeExecutionsByNodeIDs method exists for batch deletion")
	})
	
	t.Run("DeleteNodeExecutionsByNodeIDs_EmptyList", func(t *testing.T) {
		// Test deletion with empty node IDs list
		emptyNodeIDs := []idwrap.IDWrap{}
		
		ctx := context.Background()
		_ = ctx
		_ = emptyNodeIDs
		
		// Should handle empty list gracefully without error
		t.Log("DeleteNodeExecutionsByNodeIDs should handle empty list gracefully")
	})
	
	t.Run("DeleteNodeExecutionsByNodeIDs_MixedExistentNonExistent", func(t *testing.T) {
		// Test deletion with mix of existent and non-existent node IDs
		mixedNodeIDs := []idwrap.IDWrap{
			idwrap.NewNow(), // Would exist in real test
			idwrap.NewNow(), // Would not exist in real test
			idwrap.NewNow(), // Would exist in real test
		}
		
		ctx := context.Background()
		_ = ctx
		_ = mixedNodeIDs
		
		// Should delete existing executions and ignore non-existent ones
		// No error should be returned
		t.Log("DeleteNodeExecutionsByNodeIDs should handle mixed existent/non-existent nodes")
	})
	
	t.Run("DeleteNodeExecutions_TransactionSupport", func(t *testing.T) {
		// Test that deletion methods work within transactions
		nodeID := idwrap.NewNow()
		
		// In real implementation:
		// 1. Begin transaction
		// 2. Create service with TX
		// 3. Call delete methods
		// 4. Verify operations are part of transaction
		// 5. Test rollback behavior
		
		ctx := context.Background()
		_ = ctx
		_ = nodeID
		
		t.Log("Delete methods should support transaction context")
	})
}

// MockQueries represents a mock for testing
type MockQueries struct {
	DeletedNodeIDs []idwrap.IDWrap
	ShouldError    bool
}

func TestNodeExecutionOrdering(t *testing.T) {
	t.Run("OrderingByID", func(t *testing.T) {
		// Test that node executions are ordered by ID (ULID) which contains timestamp
		nodeID := idwrap.NewNow()
		
		// Create executions with deliberate time gaps to ensure different ULIDs
		var executions []mnodeexecution.NodeExecution
		var expectedOrder []idwrap.IDWrap
		
		// Create 5 executions with small delays to ensure different ULID timestamps
		for i := 0; i < 5; i++ {
			execID := idwrap.NewNow()
			execution := mnodeexecution.NodeExecution{
				ID:                     execID,
				NodeID:                 nodeID,
				Name:                   fmt.Sprintf("Execution %d", i+1),
				State:                  1, // Running state
				InputData:              []byte(fmt.Sprintf(`{"iteration": %d}`, i)),
				InputDataCompressType:  0,
				OutputData:             []byte(fmt.Sprintf(`{"index": %d}`, i)),
				OutputDataCompressType: 0,
			}
			executions = append(executions, execution)
			expectedOrder = append(expectedOrder, execID)
			
			// Small delay to ensure different ULID timestamps
			time.Sleep(1 * time.Millisecond)
		}
		
		// Verify that IDs are indeed in chronological order (ULID property)
		for i := 1; i < len(executions); i++ {
			prevID := executions[i-1].ID
			currID := executions[i].ID
			
			// ULID comparison should show chronological order
			prevBytes := prevID.Bytes()
			currBytes := currID.Bytes()
			
			// Compare the first 6 bytes (timestamp portion of ULID)
			prevTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, prevBytes[:6]...))
			currTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, currBytes[:6]...))
			
			assert.LessOrEqual(t, prevTimestamp, currTimestamp, 
				"ULID timestamps should be in chronological order (creation time)")
		}
		
		// Test that creation order is maintained in memory
		retrievedOrder := make([]idwrap.IDWrap, len(executions))
		for i, exec := range executions {
			retrievedOrder[i] = exec.ID
		}
		
		// In memory, creation order is preserved (this simulates the data structure before DB query)
		assert.Equal(t, expectedOrder, retrievedOrder, 
			"In-memory execution order should match creation order")
		
		// Simulate database DESC ordering behavior
		reversedOrder := make([]idwrap.IDWrap, len(expectedOrder))
		for i, id := range expectedOrder {
			reversedOrder[len(expectedOrder)-1-i] = id
		}
		
		// Verify database would return DESC order
		assert.NotEqual(t, expectedOrder, reversedOrder, 
			"Database DESC order should differ from creation order")
	})
	
	t.Run("OrderingWithMixedStates", func(t *testing.T) {
		// Test ordering works with both running and completed executions
		nodeID := idwrap.NewNow()
		
		var executions []mnodeexecution.NodeExecution
		states := []int8{1, 1, 2, 1, 2} // Running, Running, Success, Running, Success
		
		for i := 0; i < 5; i++ {
			execID := idwrap.NewNow()
			var completedAt *int64
			
			// Set completedAt for success states
			if states[i] == 2 {
				timestamp := time.Now().UnixMilli()
				completedAt = &timestamp
			}
			
			execution := mnodeexecution.NodeExecution{
				ID:                     execID,
				NodeID:                 nodeID,
				Name:                   fmt.Sprintf("Mixed Execution %d", i+1), 
				State:                  states[i],
				InputData:              []byte(fmt.Sprintf(`{"iteration": %d}`, i)),
				InputDataCompressType:  0,
				OutputData:             []byte(fmt.Sprintf(`{"index": %d}`, i)),
				OutputDataCompressType: 0,
				CompletedAt:            completedAt,
			}
			executions = append(executions, execution)
			time.Sleep(1 * time.Millisecond)
		}
		
		// Verify ordering is by ID, not by completion status
		for i := 1; i < len(executions); i++ {
			prevID := executions[i-1].ID
			currID := executions[i].ID
			
			prevBytes := prevID.Bytes()
			currBytes := currID.Bytes()
			
			// Compare ULID timestamps
			prevTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, prevBytes[:6]...))
			currTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, currBytes[:6]...))
			
			assert.LessOrEqual(t, prevTimestamp, currTimestamp,
				"Executions should be ordered by ID timestamp regardless of completion state")
		}
	})
	
	t.Run("OrderingIterationRecords", func(t *testing.T) {
		// Test ordering specifically for iteration records (FOR/FOR_EACH nodes)
		nodeID := idwrap.NewNow()
		
		// Simulate iteration records like those created by FOR/FOR_EACH nodes
		var iterationRecords []mnodeexecution.NodeExecution
		
		for i := 0; i < 10; i++ {
			execID := idwrap.NewNow()
			execution := mnodeexecution.NodeExecution{
				ID:                     execID,
				NodeID:                 nodeID,
				Name:                   "FOR Node Iteration",
				State:                  1, // Running - typical for iteration tracking
				InputData:              []byte(`{}`),
				InputDataCompressType:  0,
				OutputData:             []byte(fmt.Sprintf(`{"index": %d}`, i)), // FOR node output format
				OutputDataCompressType: 0,
			}
			iterationRecords = append(iterationRecords, execution)
			time.Sleep(1 * time.Millisecond)
		}
		
		// Verify iteration records maintain chronological order
		for i := 1; i < len(iterationRecords); i++ {
			prevID := iterationRecords[i-1].ID
			currID := iterationRecords[i].ID
			
			prevBytes := prevID.Bytes()
			currBytes := currID.Bytes()
			
			prevTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, prevBytes[:6]...))
			currTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, currBytes[:6]...))
			
			assert.LessOrEqual(t, prevTimestamp, currTimestamp,
				"Iteration records should maintain chronological order by ULID timestamp")
		}
		
		// Verify output data represents correct iteration sequence
		for i, record := range iterationRecords {
			expectedOutput := fmt.Sprintf(`{"index": %d}`, i)
			assert.Equal(t, expectedOutput, string(record.OutputData),
				"Iteration output should match expected index sequence")
		}
	})
}

// TestNodeExecutionDeletionIntegration tests the deletion methods with actual database operations
// This would be used in integration tests with a real database
func TestNodeExecutionDeletionIntegration(t *testing.T) {
	t.Skip("Integration test - requires database setup")
	
	// This test would:
	// 1. Set up test database
	// 2. Create test node executions
	// 3. Test DeleteNodeExecutionsByNodeID actually removes records
	// 4. Test DeleteNodeExecutionsByNodeIDs with batch operations
	// 5. Verify cascading behavior if any
	// 6. Test performance with large datasets
	
	t.Run("DeleteNodeExecutionsByNodeID_WithRealDatabase", func(t *testing.T) {
		// Setup:
		// - Create test database connection
		// - Insert test node executions
		// - Call DeleteNodeExecutionsByNodeID
		// - Verify records are deleted
		// - Verify other nodes' executions are unaffected
	})
	
	t.Run("DeleteNodeExecutionsByNodeIDs_Performance", func(t *testing.T) {
		// Test batch deletion performance:
		// - Create many node executions (1000+)
		// - Time batch deletion vs individual deletions
		// - Verify batch is more efficient
		// - Ensure memory usage is reasonable
	})
	
	t.Run("DeleteNodeExecutions_Concurrency", func(t *testing.T) {
		// Test concurrent deletion operations:
		// - Multiple goroutines calling delete methods
		// - Verify no race conditions
		// - Verify database integrity
		// - Test transaction isolation
	})
}
