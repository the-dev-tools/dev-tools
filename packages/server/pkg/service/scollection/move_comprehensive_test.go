package scollection

import (
	"context"
	"fmt"
	"testing"
	
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	
	"log/slog"
	"math/rand"
	"os"
	"time"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectionMoveComprehensive creates comprehensive tests to catch all edge cases
func TestCollectionMoveComprehensive(t *testing.T) {
	tests := []struct {
		name          string
		collectionCount int
		testFunc      func(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection)
	}{
		{
			name:          "SingleCollection",
			collectionCount: 1,
			testFunc:      testSingleCollectionEdgeCases,
		},
		{
			name:          "TwoCollections",
			collectionCount: 2,
			testFunc:      testTwoCollectionsEdgeCases,
		},
		{
			name:          "ThreeCollections",
			collectionCount: 3,
			testFunc:      testThreeCollectionsEdgeCases,
		},
		{
			name:          "ManyCollections",
			collectionCount: 10,
			testFunc:      testManyCollectionsEdgeCases,
		},
		{
			name:          "StressTest",
			collectionCount: 20,
			testFunc:      testStressEdgeCases,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, ctx, workspaceID, collections := setupTestCollections(t, tt.collectionCount)
			tt.testFunc(t, service, ctx, workspaceID, collections)
		})
	}
}

func setupTestCollections(t *testing.T, count int) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection) {
	// Create in-memory database  
	db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
	require.NoError(t, err)
	t.Cleanup(cleanup)
	
	// Initialize database
	queries, err := gen.Prepare(context.Background(), db)
	require.NoError(t, err)
	
	// Create service
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	service := New(queries, logger)
	
	ctx := context.Background()
	workspaceID := idwrap.NewNow()
	
	// Create test collections
	collections := make([]*mcollection.Collection, count)
	for i := 0; i < count; i++ {
		collections[i] = &mcollection.Collection{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        fmt.Sprintf("Collection %c", 'A'+i),
		}
		
		err := service.CreateCollection(ctx, collections[i])
		require.NoError(t, err, "Failed to create collection %d", i)
		
		// Small delay to ensure different timestamps if needed
		time.Sleep(1 * time.Millisecond)
	}
	
	// Verify initial order
	verifyLinkedListIntegrity(t, service, ctx, workspaceID, collections)
	
	return service, ctx, workspaceID, collections
}

func testSingleCollectionEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 1)
	
	// Test: Try to move single collection to itself (should fail)
	err := service.MoveCollectionAfter(ctx, collections[0].ID, collections[0].ID)
	assert.Error(t, err, "Moving single collection to itself should fail")
	assert.Contains(t, err.Error(), "cannot move collection relative to itself")
	
	// Verify collection still exists and order is intact
	orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, orderedCollections, 1, "Collection should not have disappeared")
	assert.Equal(t, collections[0].ID, orderedCollections[0].ID)
}

func testTwoCollectionsEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 2)
	colA, colB := collections[0], collections[1]
	
	// Test: Move A after B (A should move to end)
	t.Run("Move A after B", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, colA.ID, colB.ID)
		require.NoError(t, err, "MoveCollectionAfter should succeed")
		
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 2, "Should have 2 collections")
		
		// Order should be: B, A
		assert.Equal(t, colB.ID, orderedCollections[0].ID, "B should be first")
		assert.Equal(t, colA.ID, orderedCollections[1].ID, "A should be second")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, []*mcollection.Collection{colB, colA})
	})
	
	// Test: Move A before B (A should move to beginning)
	t.Run("Move A before B", func(t *testing.T) {
		err := service.MoveCollectionBefore(ctx, colA.ID, colB.ID)
		require.NoError(t, err, "MoveCollectionBefore should succeed")
		
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 2, "Should have 2 collections")
		
		// Order should be back to: A, B
		assert.Equal(t, colA.ID, orderedCollections[0].ID, "A should be first")
		assert.Equal(t, colB.ID, orderedCollections[1].ID, "B should be second")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, []*mcollection.Collection{colA, colB})
	})
	
	// Test: Try to move to itself
	t.Run("Move to self", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, colA.ID, colA.ID)
		assert.Error(t, err, "Moving to self should fail")
		
		// Verify no collections disappeared
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, 2, "Both collections should still exist")
	})
}

func testThreeCollectionsEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 3)
	colA, colB, colC := collections[0], collections[1], collections[2]
	
	// Initial order: A, B, C
	
	// Test: Move first to last
	t.Run("Move A after C", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, colA.ID, colC.ID)
		require.NoError(t, err, "MoveCollectionAfter should succeed")
		
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 3, "Should have 3 collections")
		
		// Order should be: B, C, A
		assert.Equal(t, colB.ID, orderedCollections[0].ID, "B should be first")
		assert.Equal(t, colC.ID, orderedCollections[1].ID, "C should be second")  
		assert.Equal(t, colA.ID, orderedCollections[2].ID, "A should be third")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, []*mcollection.Collection{colB, colC, colA})
	})
	
	// Test: Move last to first
	t.Run("Move A before B", func(t *testing.T) {
		err := service.MoveCollectionBefore(ctx, colA.ID, colB.ID)
		require.NoError(t, err, "MoveCollectionBefore should succeed")
		
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		require.Len(t, orderedCollections, 3, "Should have 3 collections")
		
		// Order should be: A, B, C (back to original)
		assert.Equal(t, colA.ID, orderedCollections[0].ID, "A should be first")
		assert.Equal(t, colB.ID, orderedCollections[1].ID, "B should be second")
		assert.Equal(t, colC.ID, orderedCollections[2].ID, "C should be third")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, []*mcollection.Collection{colA, colB, colC})
	})
	
	// Test: Move middle to ends
	t.Run("Move middle to first and last", func(t *testing.T) {
		// Move B to after C (B becomes last)
		err := service.MoveCollectionAfter(ctx, colB.ID, colC.ID)
		require.NoError(t, err)
		
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, 3, "Should have 3 collections")
		// Order: A, C, B
		
		// Move B to before A (B becomes first)
		err = service.MoveCollectionBefore(ctx, colB.ID, colA.ID)
		require.NoError(t, err)
		
		orderedCollections, err = service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, 3, "Should have 3 collections")
		// Order: B, A, C
		assert.Equal(t, colB.ID, orderedCollections[0].ID, "B should be first")
		assert.Equal(t, colA.ID, orderedCollections[1].ID, "A should be second")  
		assert.Equal(t, colC.ID, orderedCollections[2].ID, "C should be third")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, []*mcollection.Collection{colB, colA, colC})
	})
}

func testManyCollectionsEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 10)
	
	// Test: Move first to various positions
	t.Run("Move first collection to different positions", func(t *testing.T) {
		firstCol := collections[0]
		
		// Move to position 5 (after collection[4])
		err := service.MoveCollectionAfter(ctx, firstCol.ID, collections[4].ID)
		require.NoError(t, err)
		
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, 10, "Should have 10 collections")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, nil)
		
		// Move to last position
		err = service.MoveCollectionAfter(ctx, firstCol.ID, collections[9].ID)
		require.NoError(t, err)
		
		orderedCollections, err = service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, 10, "Should have 10 collections")
		assert.Equal(t, firstCol.ID, orderedCollections[9].ID, "First collection should now be last")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, nil)
	})
	
	// Test: Move last to various positions
	t.Run("Move last collection to different positions", func(t *testing.T) {
		lastCol := collections[9]
		
		// Move to first position
		err := service.MoveCollectionBefore(ctx, lastCol.ID, collections[1].ID)
		require.NoError(t, err)
		
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, 10, "Should have 10 collections")
		
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, nil)
	})
	
	// Test: Sequential moves without losing collections
	t.Run("Sequential moves", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			// Move random collection after random target
			sourceIdx := rand.Intn(10)
			targetIdx := rand.Intn(10)
			if sourceIdx == targetIdx {
				continue // Skip moving to self
			}
			
			err := service.MoveCollectionAfter(ctx, collections[sourceIdx].ID, collections[targetIdx].ID)
			require.NoError(t, err, "Move %d should succeed", i)
			
			// Verify no collections disappeared
			orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
			require.NoError(t, err)
			assert.Len(t, orderedCollections, 10, "Should always have 10 collections after move %d", i)
			
			verifyLinkedListIntegrity(t, service, ctx, workspaceID, nil)
		}
	})
}

func testStressEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 20)
	
	// Test: Random moves with integrity checking
	t.Run("Random stress test", func(t *testing.T) {
		rand.Seed(time.Now().UnixNano())
		
		for i := 0; i < 50; i++ {
			sourceIdx := rand.Intn(20)
			targetIdx := rand.Intn(20)
			if sourceIdx == targetIdx {
				continue
			}
			
			if rand.Intn(2) == 0 {
				err := service.MoveCollectionAfter(ctx, collections[sourceIdx].ID, collections[targetIdx].ID)
				require.NoError(t, err, "MoveAfter %d should succeed", i)
			} else {
				err := service.MoveCollectionBefore(ctx, collections[sourceIdx].ID, collections[targetIdx].ID)
				require.NoError(t, err, "MoveBefore %d should succeed", i)
			}
			
			// Every 5 moves, verify integrity
			if i%5 == 0 {
				orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
				require.NoError(t, err)
				assert.Len(t, orderedCollections, 20, "Should always have 20 collections at iteration %d", i)
				
				verifyLinkedListIntegrity(t, service, ctx, workspaceID, nil)
			}
		}
		
		// Final integrity check
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, 20, "Should have 20 collections at end")
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, nil)
	})
}

// verifyLinkedListIntegrity checks that the linked list structure is consistent
func verifyLinkedListIntegrity(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, expectedOrder []*mcollection.Collection) {
	t.Helper()
	
	// Get ordered collections
	orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err)
	
	if len(orderedCollections) == 0 {
		return // Empty list is valid
	}
	
	// If expected order is provided, verify it matches
	if expectedOrder != nil {
		require.Len(t, orderedCollections, len(expectedOrder), "Order length mismatch")
		for i, expected := range expectedOrder {
			assert.Equal(t, expected.ID, orderedCollections[i].ID, "Order mismatch at position %d", i)
		}
	}
	
	// Verify linked list pointers by getting database objects
	for i, col := range orderedCollections {
		dbCol, err := service.queries.GetCollection(ctx, col.ID)
		require.NoError(t, err, "Failed to get database collection %d", i)
		
		if i == 0 {
			// First collection should have nil prev
			assert.Nil(t, dbCol.Prev, "First collection should have nil prev")
		} else {
			// Other collections should point to previous
			require.NotNil(t, dbCol.Prev, "Collection %d should have prev pointer", i)
			assert.Equal(t, orderedCollections[i-1].ID, *dbCol.Prev, "Collection %d prev pointer mismatch", i)
		}
		
		if i == len(orderedCollections)-1 {
			// Last collection should have nil next
			assert.Nil(t, dbCol.Next, "Last collection should have nil next")
		} else {
			// Other collections should point to next
			require.NotNil(t, dbCol.Next, "Collection %d should have next pointer", i)
			assert.Equal(t, orderedCollections[i+1].ID, *dbCol.Next, "Collection %d next pointer mismatch", i)
		}
	}
}

// TestCollectionCreationOrder verifies that collections are created in proper linked list order
func TestCollectionCreationOrder(t *testing.T) {
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
	
	// Create collections one by one
	collections := make([]*mcollection.Collection, 5)
	for i := 0; i < 5; i++ {
		collections[i] = &mcollection.Collection{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        fmt.Sprintf("Collection %d", i+1),
		}
		
		err := service.CreateCollection(ctx, collections[i])
		require.NoError(t, err, "Failed to create collection %d", i)
		
		// Verify order after each creation
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		assert.Len(t, orderedCollections, i+1, "Should have %d collections after creating %d", i+1, i+1)
		
		// Verify the newly created collection is at the end
		assert.Equal(t, collections[i].ID, orderedCollections[i].ID, "Collection %d should be at position %d", i+1, i)
		
		// Verify linked list integrity
		verifyLinkedListIntegrity(t, service, ctx, workspaceID, collections[:i+1])
		
		time.Sleep(1 * time.Millisecond) // Small delay for timestamp differences
	}
}