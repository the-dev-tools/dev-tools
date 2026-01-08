package sflow

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeWriter_UpdateNodeState_ValidStates(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	writer := NewNodeWriter(db)

	// Create a test flow and node
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	nodeID := idwrap.NewNow()
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  int32(mflow.NODE_KIND_REQUEST),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	tests := []struct {
		name  string
		state mflow.NodeState
	}{
		{
			name:  "UNSPECIFIED state",
			state: mflow.NODE_STATE_UNSPECIFIED,
		},
		{
			name:  "RUNNING state",
			state: mflow.NODE_STATE_RUNNING,
		},
		{
			name:  "SUCCESS state",
			state: mflow.NODE_STATE_SUCCESS,
		},
		{
			name:  "FAILURE state",
			state: mflow.NODE_STATE_FAILURE,
		},
		{
			name:  "CANCELED state",
			state: mflow.NODE_STATE_CANCELED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writer.UpdateNodeState(ctx, nodeID, tt.state)
			require.NoError(t, err)

			// Verify state was updated
			node, err := queries.GetFlowNode(ctx, nodeID)
			require.NoError(t, err)
			assert.Equal(t, int8(tt.state), node.State)
		})
	}
}

func TestNodeWriter_UpdateNodeState_InvalidStates(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	writer := NewNodeWriter(db)

	// Create a test flow and node
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	nodeID := idwrap.NewNow()
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  int32(mflow.NODE_KIND_REQUEST),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		state       mflow.NodeState
		errContains string
	}{
		{
			name:        "Invalid negative state -1",
			state:       mflow.NodeState(-1),
			errContains: "invalid node state",
		},
		{
			name:        "Invalid negative state -100",
			state:       mflow.NodeState(-100),
			errContains: "invalid node state",
		},
		{
			name:        "Invalid state 5 (above max)",
			state:       mflow.NodeState(5),
			errContains: "invalid node state",
		},
		{
			name:        "Invalid state 127",
			state:       mflow.NodeState(127),
			errContains: "invalid node state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writer.UpdateNodeState(ctx, nodeID, tt.state)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.errContains)

			// Verify state was NOT updated
			node, err := queries.GetFlowNode(ctx, nodeID)
			require.NoError(t, err)
			assert.Equal(t, int8(0), node.State) // Should still be UNSPECIFIED (default)
		})
	}
}

func TestNodeWriter_UpdateNodeState_BoundaryValues(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	writer := NewNodeWriter(db)

	// Create a test flow and node
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	nodeID := idwrap.NewNow()
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  int32(mflow.NODE_KIND_REQUEST),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	t.Run("Lower boundary (0 - UNSPECIFIED)", func(t *testing.T) {
		err := writer.UpdateNodeState(ctx, nodeID, mflow.NODE_STATE_UNSPECIFIED)
		assert.NoError(t, err)
	})

	t.Run("Upper boundary (4 - CANCELED)", func(t *testing.T) {
		err := writer.UpdateNodeState(ctx, nodeID, mflow.NODE_STATE_CANCELED)
		assert.NoError(t, err)
	})

	t.Run("Just below lower boundary (-1)", func(t *testing.T) {
		err := writer.UpdateNodeState(ctx, nodeID, mflow.NodeState(-1))
		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid node state")
	})

	t.Run("Just above upper boundary (5)", func(t *testing.T) {
		err := writer.UpdateNodeState(ctx, nodeID, mflow.NodeState(5))
		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid node state")
	})
}

func TestNodeWriter_UpdateNodeState_NonExistentNode(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	writer := NewNodeWriter(db)

	nonExistentID := idwrap.NewNow()
	err = writer.UpdateNodeState(ctx, nonExistentID, mflow.NODE_STATE_SUCCESS)
	// SQL UPDATE returns 0 rows affected but no error in Go's sql package
	// The behavior depends on the database driver - SQLite may not return error for non-existent updates
	// So we just verify the function doesn't panic
	assert.NoError(t, err)
}
