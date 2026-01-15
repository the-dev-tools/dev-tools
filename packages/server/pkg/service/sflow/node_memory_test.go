package sflow

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func setupNodeMemoryTest(t *testing.T) (context.Context, *sql.DB, *gen.Queries, idwrap.IDWrap) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)

	queries := gen.New(db)

	// Create workspace
	workspaceID := idwrap.NewNow()
	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	// Create flow
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (MEMORY kind)
	nodeID := idwrap.NewNow()
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Memory Node",
		NodeKind:  int32(mflow.NODE_KIND_AI_MEMORY),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })

	return ctx, db, queries, nodeID
}

func TestNodeMemoryMapper_RoundTrip(t *testing.T) {
	nodeID := idwrap.NewNow()

	mn := mflow.NodeMemory{
		FlowNodeID: nodeID,
		MemoryType: mflow.AiMemoryTypeWindowBuffer,
		WindowSize: 10,
	}

	dbn := ConvertNodeMemoryToDB(mn)
	assert.Equal(t, nodeID.Bytes(), dbn.FlowNodeID)
	assert.Equal(t, int8(mflow.AiMemoryTypeWindowBuffer), dbn.MemoryType)
	assert.Equal(t, int32(10), dbn.WindowSize)

	mn2 := ConvertDBToNodeMemory(dbn)
	assert.Equal(t, mn.FlowNodeID, mn2.FlowNodeID)
	assert.Equal(t, mn.MemoryType, mn2.MemoryType)
	assert.Equal(t, mn.WindowSize, mn2.WindowSize)
}

func TestNodeMemoryService_CRUD(t *testing.T) {
	ctx, db, queries, nodeID := setupNodeMemoryTest(t)

	service := NewNodeMemoryService(queries)

	// Create
	memory := mflow.NodeMemory{
		FlowNodeID: nodeID,
		MemoryType: mflow.AiMemoryTypeWindowBuffer,
		WindowSize: 20,
	}

	// Use TX for write operations
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	writer := service.TX(tx)

	err = writer.CreateNodeMemory(ctx, memory)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Read
	retrieved, err := service.GetNodeMemory(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, nodeID, retrieved.FlowNodeID)
	assert.Equal(t, mflow.AiMemoryTypeWindowBuffer, retrieved.MemoryType)
	assert.Equal(t, int32(20), retrieved.WindowSize)

	// Update
	memory.WindowSize = 50

	tx2, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	writer2 := service.TX(tx2)

	err = writer2.UpdateNodeMemory(ctx, memory)
	require.NoError(t, err)

	err = tx2.Commit()
	require.NoError(t, err)

	// Verify update
	updated, err := service.GetNodeMemory(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, int32(50), updated.WindowSize)

	// Delete
	tx3, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	writer3 := service.TX(tx3)

	err = writer3.DeleteNodeMemory(ctx, nodeID)
	require.NoError(t, err)

	err = tx3.Commit()
	require.NoError(t, err)

	// Verify deletion
	_, err = service.GetNodeMemory(ctx, nodeID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestNodeMemoryService_GetNonExistent(t *testing.T) {
	ctx, _, queries, _ := setupNodeMemoryTest(t)

	service := NewNodeMemoryService(queries)

	nonExistentID := idwrap.NewNow()
	_, err := service.GetNodeMemory(ctx, nonExistentID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestNodeMemoryService_VariousWindowSizes(t *testing.T) {
	ctx, db, queries, _ := setupNodeMemoryTest(t)

	service := NewNodeMemoryService(queries)

	tests := []struct {
		name       string
		windowSize int32
	}{
		{"Small window", 5},
		{"Medium window", 50},
		{"Large window", 1000},
		{"Zero window", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new node for each test
			flowID := idwrap.NewNow()
			err := queries.CreateFlow(ctx, gen.CreateFlowParams{
				ID:          flowID,
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Flow " + tt.name,
			})
			require.NoError(t, err)

			nodeID := idwrap.NewNow()
			err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
				ID:        nodeID,
				FlowID:    flowID,
				Name:      "Memory " + tt.name,
				NodeKind:  int32(mflow.NODE_KIND_AI_MEMORY),
				PositionX: 0,
				PositionY: 0,
			})
			require.NoError(t, err)

			memory := mflow.NodeMemory{
				FlowNodeID: nodeID,
				MemoryType: mflow.AiMemoryTypeWindowBuffer,
				WindowSize: tt.windowSize,
			}

			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			writer := service.TX(tx)

			err = writer.CreateNodeMemory(ctx, memory)
			require.NoError(t, err)

			err = tx.Commit()
			require.NoError(t, err)

			retrieved, err := service.GetNodeMemory(ctx, nodeID)
			require.NoError(t, err)
			assert.Equal(t, tt.windowSize, retrieved.WindowSize)
		})
	}
}
