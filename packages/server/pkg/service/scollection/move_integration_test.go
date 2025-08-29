package scollection

import (
	"context"
	"testing"
	
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	
	"log/slog"
	"os"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionMove_Integration(t *testing.T) {
	// Create in-memory database
	db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
	require.NoError(t, err)
	defer cleanup()
	
	// Initialize database
	queries, err := gen.Prepare(context.Background(), db)
	require.NoError(t, err)
	
	// Create service
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := New(queries, logger)
	
	ctx := context.Background()
	workspaceID := idwrap.NewNow()
	
	// Create test collections
	collections := []*mcollection.Collection{
		{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        "Collection A",
		},
		{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        "Collection B",
		},
		{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        "Collection C",
		},
	}
	
	// Create collections in database
	for _, col := range collections {
		err := service.CreateCollection(ctx, col)
		require.NoError(t, err)
	}
	
	t.Log("Created collections:")
	for i, col := range collections {
		t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
	}
	
	// Test 1: Move Collection A after Collection B
	t.Run("Move A after B", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, collections[0].ID, collections[1].ID)
		assert.NoError(t, err, "MoveCollectionAfter should succeed")
		
		// Verify order
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 3)
		
		t.Log("Order after moving A after B:")
		for i, col := range orderedCollections {
			t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
		}
	})
	
	// Test 2: Move Collection C before Collection A
	t.Run("Move C before A", func(t *testing.T) {
		err := service.MoveCollectionBefore(ctx, collections[2].ID, collections[0].ID)
		assert.NoError(t, err, "MoveCollectionBefore should succeed")
		
		// Verify order
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 3)
		
		t.Log("Order after moving C before A:")
		for i, col := range orderedCollections {
			t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
		}
	})
	
	// Test 3: Error cases
	t.Run("Error cases", func(t *testing.T) {
		// Try to move collection to itself
		err := service.MoveCollectionAfter(ctx, collections[0].ID, collections[0].ID)
		assert.Error(t, err, "Moving collection to itself should fail")
		assert.Contains(t, err.Error(), "cannot move collection relative to itself")
		
		// Try to move non-existent collection
		nonExistentID := idwrap.NewNow()
		err = service.MoveCollectionAfter(ctx, nonExistentID, collections[0].ID)
		assert.Error(t, err, "Moving non-existent collection should fail")
		assert.Contains(t, err.Error(), "source collection not found")
		
		// Try to move to non-existent target
		err = service.MoveCollectionAfter(ctx, collections[0].ID, nonExistentID)
		assert.Error(t, err, "Moving to non-existent target should fail")
		assert.Contains(t, err.Error(), "target collection not found")
	})
}

func TestCollectionMove_DifferentWorkspaces(t *testing.T) {
	// Create in-memory database
	db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
	require.NoError(t, err)
	defer cleanup()
	
	// Initialize database
	queries, err := gen.Prepare(context.Background(), db)
	require.NoError(t, err)
	
	// Create service
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := New(queries, logger)
	
	ctx := context.Background()
	workspace1ID := idwrap.NewNow()
	workspace2ID := idwrap.NewNow()
	
	// Create collections in different workspaces
	collection1 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspace1ID,
		Name:        "Collection in Workspace 1",
	}
	collection2 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspace2ID,
		Name:        "Collection in Workspace 2",
	}
	
	err = service.CreateCollection(ctx, collection1)
	require.NoError(t, err)
	err = service.CreateCollection(ctx, collection2)
	require.NoError(t, err)
	
	// Try to move collection from workspace1 after collection in workspace2
	err = service.MoveCollectionAfter(ctx, collection1.ID, collection2.ID)
	assert.Error(t, err, "Moving collections between different workspaces should fail")
	assert.Contains(t, err.Error(), "collections must be in the same workspace")
}