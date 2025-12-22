package sflow

import (
	"context"
	"testing"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEdgeWriter_UpdateEdgeState_ValidStates(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	writer := NewEdgeWriter(db)

	// Create a test flow and edge
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	sourceID := idwrap.NewNow()
	targetID := idwrap.NewNow()

	// Create source and target nodes
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        sourceID,
		FlowID:    flowID,
		Name:      "Source Node",
		NodeKind:  int32(mflow.NODE_KIND_MANUAL_START),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        targetID,
		FlowID:    flowID,
		Name:      "Target Node",
		NodeKind:  int32(mflow.NODE_KIND_REQUEST),
		PositionX: 300,
		PositionY: 400,
	})
	require.NoError(t, err)

	edgeID := idwrap.NewNow()
	err = queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      sourceID,
		TargetID:      targetID,
		SourceHandle:  0,
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		state    mflow.NodeState
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
			err := writer.UpdateEdgeState(ctx, edgeID, tt.state)
			require.NoError(t, err)

			// Verify state was updated
			edge, err := queries.GetFlowEdge(ctx, edgeID)
			require.NoError(t, err)
			assert.Equal(t, int8(tt.state), edge.State)
		})
	}
}

func TestEdgeWriter_UpdateEdgeState_InvalidStates(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	writer := NewEdgeWriter(db)

	// Create a test flow and edge
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	sourceID := idwrap.NewNow()
	targetID := idwrap.NewNow()

	// Create source and target nodes
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        sourceID,
		FlowID:    flowID,
		Name:      "Source Node",
		NodeKind:  int32(mflow.NODE_KIND_MANUAL_START),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        targetID,
		FlowID:    flowID,
		Name:      "Target Node",
		NodeKind:  int32(mflow.NODE_KIND_REQUEST),
		PositionX: 300,
		PositionY: 400,
	})
	require.NoError(t, err)

	edgeID := idwrap.NewNow()
	err = queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      sourceID,
		TargetID:      targetID,
		SourceHandle:  0,
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
			errContains: "invalid edge state",
		},
		{
			name:        "Invalid negative state -100",
			state:       mflow.NodeState(-100),
			errContains: "invalid edge state",
		},
		{
			name:        "Invalid state 5 (above max)",
			state:       mflow.NodeState(5),
			errContains: "invalid edge state",
		},
		{
			name:        "Invalid state 127",
			state:       mflow.NodeState(127),
			errContains: "invalid edge state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writer.UpdateEdgeState(ctx, edgeID, tt.state)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.errContains)

			// Verify state was NOT updated
			edge, err := queries.GetFlowEdge(ctx, edgeID)
			require.NoError(t, err)
			assert.Equal(t, int8(0), edge.State) // Should still be UNSPECIFIED (default)
		})
	}
}

func TestEdgeWriter_UpdateEdgeState_BoundaryValues(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)
	writer := NewEdgeWriter(db)

	// Create a test flow and edge
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	sourceID := idwrap.NewNow()
	targetID := idwrap.NewNow()

	// Create source and target nodes
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        sourceID,
		FlowID:    flowID,
		Name:      "Source Node",
		NodeKind:  int32(mflow.NODE_KIND_MANUAL_START),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        targetID,
		FlowID:    flowID,
		Name:      "Target Node",
		NodeKind:  int32(mflow.NODE_KIND_REQUEST),
		PositionX: 300,
		PositionY: 400,
	})
	require.NoError(t, err)

	edgeID := idwrap.NewNow()
	err = queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      sourceID,
		TargetID:      targetID,
		SourceHandle:  0,
	})
	require.NoError(t, err)

	t.Run("Lower boundary (0 - UNSPECIFIED)", func(t *testing.T) {
		err := writer.UpdateEdgeState(ctx, edgeID, mflow.NODE_STATE_UNSPECIFIED)
		assert.NoError(t, err)
	})

	t.Run("Upper boundary (4 - CANCELED)", func(t *testing.T) {
		err := writer.UpdateEdgeState(ctx, edgeID, mflow.NODE_STATE_CANCELED)
		assert.NoError(t, err)
	})

	t.Run("Just below lower boundary (-1)", func(t *testing.T) {
		err := writer.UpdateEdgeState(ctx, edgeID, mflow.NodeState(-1))
		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid edge state")
	})

	t.Run("Just above upper boundary (5)", func(t *testing.T) {
		err := writer.UpdateEdgeState(ctx, edgeID, mflow.NodeState(5))
		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid edge state")
	})
}

func TestEdgeWriter_UpdateEdgeState_NonExistentEdge(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	writer := NewEdgeWriter(db)

	nonExistentID := idwrap.NewNow()
	err = writer.UpdateEdgeState(ctx, nonExistentID, mflow.NODE_STATE_SUCCESS)
	// SQL UPDATE returns 0 rows affected but no error in Go's sql package
	// The behavior depends on the database driver - SQLite may not return error for non-existent updates
	// So we just verify the function doesn't panic
	assert.NoError(t, err)
}
