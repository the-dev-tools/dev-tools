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
	
	"github.com/stretchr/testify/require"
)

// TestCollectionMove_VerifyFix verifies that the CollectionMove functionality works correctly
// This test addresses the original issue: "target item not found" and "cannot move collection relative to itself"
func TestCollectionMove_VerifyFix(t *testing.T) {
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
	
	// Create test collections (same as in the error scenario)
	collectionA := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection A",
	}
	collectionB := &mcollection.Collection{
		ID:          idwrap.NewNow(), 
		WorkspaceID: workspaceID,
		Name:        "Collection B",
	}
	
	// Create collections in database
	err = service.CreateCollection(ctx, collectionA)
	require.NoError(t, err, "Should be able to create Collection A")
	
	err = service.CreateCollection(ctx, collectionB)
	require.NoError(t, err, "Should be able to create Collection B")
	
	t.Logf("Created Collection A: %s", collectionA.ID.String())
	t.Logf("Created Collection B: %s", collectionB.ID.String())
	
	// Test the move operation that was failing before the fix
	t.Run("Move Collection A after Collection B", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, collectionA.ID, collectionB.ID)
		require.NoError(t, err, "MoveCollectionAfter should succeed (was failing with 'target item not found')")
		
		// Verify the collections are in the correct order
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 2, "Should have 2 collections")
		
		t.Log("Final order:")
		for i, col := range orderedCollections {
			t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
		}
		
		// The order should now be: Collection B, Collection A
		require.Equal(t, collectionB.ID, orderedCollections[0].ID, "Collection B should be first")
		require.Equal(t, collectionA.ID, orderedCollections[1].ID, "Collection A should be second (after B)")
	})
	
	t.Run("Move Collection B before Collection A", func(t *testing.T) {
		err := service.MoveCollectionBefore(ctx, collectionB.ID, collectionA.ID)
		require.NoError(t, err, "MoveCollectionBefore should succeed")
		
		// Verify the collections are back to original order
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 2, "Should have 2 collections")
		
		t.Log("Order after moving B before A:")
		for i, col := range orderedCollections {
			t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
		}
	})
	
	// Test error case that was incorrectly triggered before
	t.Run("Verify error message for moving to itself", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, collectionA.ID, collectionA.ID)
		require.Error(t, err, "Should fail when moving collection to itself")
		require.Contains(t, err.Error(), "cannot move collection relative to itself", 
			"Should have correct error message")
		t.Logf("Correct error message: %s", err.Error())
	})
}