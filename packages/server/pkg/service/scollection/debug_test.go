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

func TestCollectionMoveDebug(t *testing.T) {
	// Create in-memory database  
	db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
	require.NoError(t, err)
	defer cleanup()
	
	// Initialize database
	queries, err := gen.Prepare(context.Background(), db)
	require.NoError(t, err)
	
	// Create service
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	service := New(queries, logger)
	
	ctx := context.Background()
	workspaceID := idwrap.NewNow()
	
	// Create exactly 2 collections
	colA := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection A",
	}
	colB := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection B",
	}
	
	t.Logf("Creating Collection A: %s", colA.ID.String())
	err = service.CreateCollection(ctx, colA)
	require.NoError(t, err)
	
	// Check after first creation
	collections, err := service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err)
	t.Logf("After creating A: %d collections", len(collections))
	for i, col := range collections {
		t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
	}
	
	t.Logf("Creating Collection B: %s", colB.ID.String())
	err = service.CreateCollection(ctx, colB)
	require.NoError(t, err)
	
	// Check after second creation
	collections, err = service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err)
	t.Logf("After creating B: %d collections", len(collections))
	for i, col := range collections {
		t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
	}
	
	// Now try the move that was failing: A after B
	t.Logf("Moving A after B")
	err = service.MoveCollectionAfter(ctx, colA.ID, colB.ID)
	if err != nil {
		t.Logf("Move failed with error: %s", err.Error())
		t.FailNow()
	}
	
	// Check after move
	collections, err = service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err)
	t.Logf("After moving A after B: %d collections", len(collections))
	for i, col := range collections {
		t.Logf("  %d: %s (%s)", i, col.Name, col.ID.String())
		
		// Check database state
		dbCol, err := service.queries.GetCollection(ctx, col.ID)
		require.NoError(t, err)
		var prevStr, nextStr string
		if dbCol.Prev != nil {
			prevStr = dbCol.Prev.String()
		} else {
			prevStr = "nil"
		}
		if dbCol.Next != nil {
			nextStr = dbCol.Next.String()
		} else {
			nextStr = "nil"
		}
		t.Logf("    DB: prev=%s, next=%s", prevStr, nextStr)
	}
	
	require.Len(t, collections, 2, "Should have 2 collections after move")
}