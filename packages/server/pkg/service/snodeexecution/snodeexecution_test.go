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
			ID:               idwrap.NewNow(),
			NodeID:           idwrap.NewNow(),
			FlowRunID:        idwrap.NewNow(),
			State:            2, // Success state
			Data:             []byte(`{"result": "test"}`),
			DataCompressType: 0,
		}

		dbExec := snodeexecution.ConvertNodeExecutionToDB(nodeExec)
		
		assert.Equal(t, nodeExec.ID, dbExec.ID)
		assert.Equal(t, nodeExec.NodeID, dbExec.NodeID)
		assert.Equal(t, nodeExec.FlowRunID, dbExec.FlowRunID)
		assert.Equal(t, nodeExec.State, dbExec.State)
		assert.Equal(t, nodeExec.Data, dbExec.Data)
		assert.Equal(t, nodeExec.DataCompressType, dbExec.DataCompressType)
		assert.False(t, dbExec.Error.Valid) // No error in this case
	})

	t.Run("ConvertNodeExecutionToDB_WithError", func(t *testing.T) {
		errorMsg := "Node execution failed"
		nodeExec := mnodeexecution.NodeExecution{
			ID:               idwrap.NewNow(),
			NodeID:           idwrap.NewNow(),
			FlowRunID:        idwrap.NewNow(),
			State:            3, // Failure state
			Data:             []byte(`{"error": "test"}`),
			DataCompressType: 0,
			Error:            &errorMsg,
		}

		dbExec := snodeexecution.ConvertNodeExecutionToDB(nodeExec)
		
		assert.Equal(t, nodeExec.ID, dbExec.ID)
		assert.Equal(t, nodeExec.NodeID, dbExec.NodeID)
		assert.Equal(t, nodeExec.FlowRunID, dbExec.FlowRunID)
		assert.Equal(t, nodeExec.State, dbExec.State)
		assert.Equal(t, nodeExec.Data, dbExec.Data)
		assert.Equal(t, nodeExec.DataCompressType, dbExec.DataCompressType)
		assert.True(t, dbExec.Error.Valid)
		assert.Equal(t, errorMsg, dbExec.Error.String)
	})

	t.Run("ConvertNodeExecutionToModel", func(t *testing.T) {
		dbExec := gen.NodeExecution{
			ID:               idwrap.NewNow(),
			NodeID:           idwrap.NewNow(),
			FlowRunID:        idwrap.NewNow(),
			State:            3, // Failure state
			Data:             []byte(`{"error": "test error"}`),
			DataCompressType: 0,
		}

		modelExec := snodeexecution.ConvertNodeExecutionToModel(dbExec)
		
		assert.Equal(t, dbExec.ID, modelExec.ID)
		assert.Equal(t, dbExec.NodeID, modelExec.NodeID)
		assert.Equal(t, dbExec.FlowRunID, modelExec.FlowRunID)
		assert.Equal(t, dbExec.State, modelExec.State)
		assert.Equal(t, dbExec.Data, modelExec.Data)
		assert.Equal(t, dbExec.DataCompressType, modelExec.DataCompressType)
		assert.Nil(t, modelExec.Error) // No error in this case
	})

	t.Run("ConvertNodeExecutionToModel_WithError", func(t *testing.T) {
		errorMsg := "Database connection failed"
		dbExec := gen.NodeExecution{
			ID:               idwrap.NewNow(),
			NodeID:           idwrap.NewNow(),
			FlowRunID:        idwrap.NewNow(),
			State:            3, // Failure state
			Data:             []byte(`{"error": "test error"}`),
			DataCompressType: 0,
			Error:            sql.NullString{String: errorMsg, Valid: true},
		}

		modelExec := snodeexecution.ConvertNodeExecutionToModel(dbExec)
		
		assert.Equal(t, dbExec.ID, modelExec.ID)
		assert.Equal(t, dbExec.NodeID, modelExec.NodeID)
		assert.Equal(t, dbExec.FlowRunID, modelExec.FlowRunID)
		assert.Equal(t, dbExec.State, modelExec.State)
		assert.Equal(t, dbExec.Data, modelExec.Data)
		assert.Equal(t, dbExec.DataCompressType, modelExec.DataCompressType)
		require.NotNil(t, modelExec.Error)
		assert.Equal(t, errorMsg, *modelExec.Error)
	})
}

func TestNodeExecutionTracking(t *testing.T) {
	t.Run("NodeExecutionCreation", func(t *testing.T) {
		// Test that node executions are created with proper data
		nodeID := idwrap.NewNow()
		flowRunID := idwrap.NewNow()
		
		executions := []mnodeexecution.NodeExecution{
			{
				ID:               idwrap.NewNow(),
				NodeID:           nodeID,
				FlowRunID:        flowRunID,
				State:            1, // Running
				Data:             []byte(`{}`),
				DataCompressType: 0,
			},
			{
				ID:               idwrap.NewNow(),
				NodeID:           nodeID,
				FlowRunID:        flowRunID,
				State:            2, // Success
				Data:             []byte(`{"output": "test result"}`),
				DataCompressType: 0,
			},
			{
				ID:               idwrap.NewNow(),
				NodeID:           nodeID,
				FlowRunID:        flowRunID,
				State:            3, // Failure
				Data:             []byte(`{"error": "test error"}`),
				DataCompressType: 0,
				Error:            &[]string{"Connection timeout"}[0],
			},
		}
		
		// Verify executions have required fields
		for _, exec := range executions {
			require.NotEqual(t, idwrap.IDWrap{}, exec.ID)
			require.Equal(t, nodeID, exec.NodeID)
			require.Equal(t, flowRunID, exec.FlowRunID)
			require.NotNil(t, exec.Data)
		}
	})
}