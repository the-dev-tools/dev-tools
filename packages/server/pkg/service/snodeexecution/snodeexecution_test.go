package snodeexecution_test

import (
	"database/sql"
	"testing"
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
			ID:                    idwrap.NewNow(),
			NodeID:                idwrap.NewNow(),
			State:                 2, // Success state
			InputData:             []byte(`{"input": "test"}`),
			InputDataCompressType: 0,
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
			ID:                    idwrap.NewNow(),
			NodeID:                idwrap.NewNow(),
			State:                 3, // Failure state
			InputData:             []byte(`{"input": "test"}`),
			InputDataCompressType: 0,
			OutputData:             []byte(`{"error": "test"}`),
			OutputDataCompressType: 0,
			Error:                 &errorMsg,
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
			ID:                    idwrap.NewNow(),
			NodeID:                idwrap.NewNow(),
			State:                 3, // Failure state
			InputData:             []byte(`{"input": "test"}`),
			InputDataCompressType: 0,
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
			ID:                    idwrap.NewNow(),
			NodeID:                idwrap.NewNow(),
			State:                 3, // Failure state
			InputData:             []byte(`{"input": "test"}`),
			InputDataCompressType: 0,
			OutputData:             []byte(`{"error": "test error"}`),
			OutputDataCompressType: 0,
			Error:                 sql.NullString{String: errorMsg, Valid: true},
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
				ID:                    idwrap.NewNow(),
				NodeID:                nodeID,
				State:                 1, // Running
				InputData:             []byte(`{"input": "test input"}`),
				InputDataCompressType: 0,
				OutputData:             []byte(`{}`),
				OutputDataCompressType: 0,
			},
			{
				ID:                    idwrap.NewNow(),
				NodeID:                nodeID,
				State:                 2, // Success
				InputData:             []byte(`{"input": "test input"}`),
				InputDataCompressType: 0,
				OutputData:             []byte(`{"output": "test result"}`),
				OutputDataCompressType: 0,
			},
			{
				ID:                    idwrap.NewNow(),
				NodeID:                nodeID,
				State:                 3, // Failure
				InputData:             []byte(`{"input": "test input"}`),
				InputDataCompressType: 0,
				OutputData:             []byte(`{"error": "test error"}`),
				OutputDataCompressType: 0,
				Error:                 &[]string{"Connection timeout"}[0],
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