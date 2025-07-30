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
	_ "modernc.org/sqlite"
)

func TestNodeExecutionOrderingIntegration(t *testing.T) {
	// Create in-memory SQLite database for testing
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("Failed to close database: %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Create the schema
	createTableSQL := `
	CREATE TABLE node_execution (
		id BLOB NOT NULL PRIMARY KEY,
		node_id BLOB NOT NULL,
		name TEXT NOT NULL,
		state INT8 NOT NULL,
		error TEXT,
		input_data BLOB,
		input_data_compress_type INT8 NOT NULL DEFAULT 0,
		output_data BLOB,
		output_data_compress_type INT8 NOT NULL DEFAULT 0,
		response_id BLOB,
		completed_at BIGINT
	);`

	_, err = db.ExecContext(ctx, createTableSQL)
	require.NoError(t, err)

	// Create queries instance
	queries := gen.New(db)
	service := snodeexecution.New(queries)

	t.Run("DatabaseOrderingByID", func(t *testing.T) {
		nodeID := idwrap.NewNow()
		var createdExecutions []mnodeexecution.NodeExecution

		// Create multiple node executions with time delays
		for i := 0; i < 5; i++ {
			execution := mnodeexecution.NodeExecution{
				ID:                     idwrap.NewNow(),
				NodeID:                 nodeID,
				Name:                   fmt.Sprintf("Test Execution %d", i+1),
				State:                  1, // Running
				InputData:              []byte(fmt.Sprintf(`{"iteration": %d}`, i)),
				InputDataCompressType:  0,
				OutputData:             []byte(fmt.Sprintf(`{"index": %d}`, i)),
				OutputDataCompressType: 0,
			}

			// Insert into database
			err := service.CreateNodeExecution(ctx, execution)
			require.NoError(t, err)

			createdExecutions = append(createdExecutions, execution)
			time.Sleep(2 * time.Millisecond) // Ensure different ULID timestamps
		}

		// Retrieve executions using the service method that uses the SQL query
		retrievedExecutions, err := service.GetNodeExecutionsByNodeID(ctx, nodeID)
		require.NoError(t, err)
		require.Len(t, retrievedExecutions, 5)

		// Verify they are returned in ID order (DESC - latest first)
		for i := 1; i < len(retrievedExecutions); i++ {
			prevID := retrievedExecutions[i-1].ID
			currID := retrievedExecutions[i].ID

			prevBytes := prevID.Bytes()
			currBytes := currID.Bytes()

			// Compare ULID timestamps (first 6 bytes)
			prevTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, prevBytes[:6]...))
			currTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, currBytes[:6]...))

			assert.GreaterOrEqual(t, prevTimestamp, currTimestamp,
				"Database should return executions ordered by ID (ULID timestamp) DESC")
		}

		// Verify the order matches creation order (reversed for DESC)
		for i, retrieved := range retrievedExecutions {
			expectedIndex := len(createdExecutions) - 1 - i
			assert.Equal(t, createdExecutions[expectedIndex].ID, retrieved.ID,
				"Retrieved execution order should match creation order (DESC)")
			assert.Equal(t, fmt.Sprintf(`{"index": %d}`, expectedIndex), string(retrieved.OutputData),
				"Output data should match expected iteration sequence (reversed)")
		}
	})

	t.Run("DatabaseOrderingWithCompletedAt", func(t *testing.T) {
		nodeID := idwrap.NewNow()
		var createdExecutions []mnodeexecution.NodeExecution

		// Create executions with some completed, some running
		states := []int8{1, 2, 1, 2, 1} // Running, Success, Running, Success, Running
		for i := 0; i < 5; i++ {
			var completedAt *int64
			if states[i] == 2 { // Success state
				timestamp := time.Now().UnixMilli()
				completedAt = &timestamp
			}

			execution := mnodeexecution.NodeExecution{
				ID:                     idwrap.NewNow(),
				NodeID:                 nodeID,
				Name:                   fmt.Sprintf("Mixed State Execution %d", i+1),
				State:                  states[i],
				InputData:              []byte(fmt.Sprintf(`{"iteration": %d}`, i)),
				InputDataCompressType:  0,
				OutputData:             []byte(fmt.Sprintf(`{"index": %d}`, i)),
				OutputDataCompressType: 0,
				CompletedAt:            completedAt,
			}

			err := service.CreateNodeExecution(ctx, execution)
			require.NoError(t, err)

			createdExecutions = append(createdExecutions, execution)
			time.Sleep(2 * time.Millisecond)
		}

		// Retrieve and verify ordering is by ID DESC, not by completed_at
		retrievedExecutions, err := service.GetNodeExecutionsByNodeID(ctx, nodeID)
		require.NoError(t, err)
		require.Len(t, retrievedExecutions, 5)

		// Verify ordering is by ID DESC despite some having completed_at and others not
		for i := 1; i < len(retrievedExecutions); i++ {
			prevID := retrievedExecutions[i-1].ID
			currID := retrievedExecutions[i].ID

			prevBytes := prevID.Bytes()
			currBytes := currID.Bytes()

			prevTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, prevBytes[:6]...))
			currTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, currBytes[:6]...))

			assert.GreaterOrEqual(t, prevTimestamp, currTimestamp,
				"Executions should be ordered by ID DESC regardless of completion status")
		}

		// Verify creation order is preserved (reversed for DESC)
		for i, retrieved := range retrievedExecutions {
			expectedIndex := len(createdExecutions) - 1 - i
			assert.Equal(t, createdExecutions[expectedIndex].ID, retrieved.ID,
				"Creation order should be preserved in retrieval (DESC)")
		}
	})

	t.Run("DatabaseOrderingSimulatesIterationTracking", func(t *testing.T) {
		// Simulate the exact scenario from iteration tracking
		nodeID := idwrap.NewNow()

		// Create iteration records like those produced by FOR/FOR_EACH nodes
		iterationCount := 10
		for i := 0; i < iterationCount; i++ {
			execution := mnodeexecution.NodeExecution{
				ID:                     idwrap.NewNow(),
				NodeID:                 nodeID,
				Name:                   "FOR Node Iteration",
				State:                  1, // Running (typical for iteration tracking)
				InputData:              []byte(`{}`),
				InputDataCompressType:  0,
				OutputData:             []byte(fmt.Sprintf(`{"index": %d}`, i)), // FOR node format
				OutputDataCompressType: 0,
			}

			err := service.CreateNodeExecution(ctx, execution)
			require.NoError(t, err)

			time.Sleep(1 * time.Millisecond) // Small delay like in real iteration tracking
		}

		// Retrieve all iteration records
		retrievedExecutions, err := service.GetNodeExecutionsByNodeID(ctx, nodeID)
		require.NoError(t, err)
		require.Len(t, retrievedExecutions, iterationCount)

		// Verify they appear in reverse execution order (index 9, 8, 7, ..., 0) due to DESC
		for i, execution := range retrievedExecutions {
			expectedIndex := iterationCount - 1 - i // Reverse order
			expectedOutput := fmt.Sprintf(`{"index": %d}`, expectedIndex)
			assert.Equal(t, expectedOutput, string(execution.OutputData),
				"Iteration records should appear in reverse sequential order (DESC)")
			assert.Equal(t, "FOR Node Iteration", execution.Name)
			assert.Equal(t, int8(1), execution.State) // Running state
		}

		// Verify ULID ordering is maintained (DESC)
		for i := 1; i < len(retrievedExecutions); i++ {
			prevID := retrievedExecutions[i-1].ID
			currID := retrievedExecutions[i].ID

			prevBytes := prevID.Bytes()
			currBytes := currID.Bytes()

			prevTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, prevBytes[:6]...))
			currTimestamp := binary.BigEndian.Uint64(append([]byte{0, 0}, currBytes[:6]...))

			assert.GreaterOrEqual(t, prevTimestamp, currTimestamp,
				"ULID timestamps should maintain DESC order")
		}
	})
}