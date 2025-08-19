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

// TestCollectionMoveEdgeCases provides exhaustive testing of all edge cases for collection moves
func TestCollectionMoveEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection)
		testFunc  func(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection)
	}{
		{
			name:      "EmptyWorkspace",
			setupFunc: setupEmptyWorkspace,
			testFunc:  testEmptyWorkspaceEdgeCases,
		},
		{
			name:      "SingleCollection",
			setupFunc: setupSingleCollection,
			testFunc:  testSingleCollectionEdgeCases,
		},
		{
			name:      "TwoCollections",
			setupFunc: setupTwoCollections,
			testFunc:  testTwoCollectionsEdgeCases,
		},
		{
			name:      "ThreeCollections", 
			setupFunc: setupThreeCollections,
			testFunc:  testThreeCollectionsEdgeCases,
		},
		{
			name:      "FiveCollections",
			setupFunc: setupFiveCollections,
			testFunc:  testFiveCollectionsEdgeCases,
		},
		{
			name:      "TenCollections",
			setupFunc: setupTenCollections,
			testFunc:  testTenCollectionsEdgeCases,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, ctx, workspaceID, collections := tt.setupFunc(t)
			tt.testFunc(t, service, ctx, workspaceID, collections)
		})
	}
}

// Setup functions

func setupEmptyWorkspace(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection) {
	service, ctx, workspaceID := setupService(t)
	return service, ctx, workspaceID, []*mcollection.Collection{}
}

func setupSingleCollection(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection) {
	service, ctx, workspaceID := setupService(t)
	collections := createCollections(t, service, ctx, workspaceID, 1)
	return service, ctx, workspaceID, collections
}

func setupTwoCollections(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection) {
	service, ctx, workspaceID := setupService(t)
	collections := createCollections(t, service, ctx, workspaceID, 2)
	return service, ctx, workspaceID, collections
}

func setupThreeCollections(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection) {
	service, ctx, workspaceID := setupService(t)
	collections := createCollections(t, service, ctx, workspaceID, 3)
	return service, ctx, workspaceID, collections
}

func setupFiveCollections(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection) {
	service, ctx, workspaceID := setupService(t)
	collections := createCollections(t, service, ctx, workspaceID, 5)
	return service, ctx, workspaceID, collections
}

func setupTenCollections(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap, []*mcollection.Collection) {
	service, ctx, workspaceID := setupService(t)
	collections := createCollections(t, service, ctx, workspaceID, 10)
	return service, ctx, workspaceID, collections
}

func setupService(t *testing.T) (CollectionService, context.Context, idwrap.IDWrap) {
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
	
	return service, ctx, workspaceID
}

func createCollections(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, count int) []*mcollection.Collection {
	collections := make([]*mcollection.Collection, count)
	for i := 0; i < count; i++ {
		collections[i] = &mcollection.Collection{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        getCollectionName(i),
		}
		
		err := service.CreateCollection(ctx, collections[i])
		require.NoError(t, err, "Failed to create collection %d", i)
	}
	
	// Verify initial order
	verifyCollectionIntegrity(t, service, ctx, workspaceID, collections)
	
	return collections
}

func getCollectionName(index int) string {
	return string(rune('A' + index))
}

// Test functions

func testEmptyWorkspaceEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	// Test: Get collections from empty workspace
	orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
	assert.NoError(t, err)
	assert.Len(t, orderedCollections, 0, "Empty workspace should have no collections")
	
	// Test: Try to move non-existent collection
	fakeID := idwrap.NewNow()
	err = service.MoveCollectionAfter(ctx, fakeID, fakeID)
	assert.Error(t, err, "Moving non-existent collection should fail")
}

func testSingleCollectionEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 1)
	col := collections[0]
	
	// Test: Try to move single collection to itself
	err := service.MoveCollectionAfter(ctx, col.ID, col.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot move collection relative to itself")
	
	err = service.MoveCollectionBefore(ctx, col.ID, col.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot move collection relative to itself")
	
	// Verify collection still exists
	verifyCollectionIntegrity(t, service, ctx, workspaceID, collections)
}

func testTwoCollectionsEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 2)
	colA, colB := collections[0], collections[1]
	
	// Initial order: A, B
	t.Run("MoveFirstAfterLast_A_after_B", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, colA.ID, colB.ID)
		require.NoError(t, err)
		
		// Expected order: B, A
		expectedOrder := []*mcollection.Collection{colB, colA}
		verifyCollectionIntegrity(t, service, ctx, workspaceID, expectedOrder)
	})
	
	// Current order: B, A
	t.Run("MoveLastBeforeFirst_A_before_B", func(t *testing.T) {
		err := service.MoveCollectionBefore(ctx, colA.ID, colB.ID)
		require.NoError(t, err)
		
		// Expected order: A, B (back to original)
		expectedOrder := []*mcollection.Collection{colA, colB}
		verifyCollectionIntegrity(t, service, ctx, workspaceID, expectedOrder)
	})
	
	// Current order: A, B  
	t.Run("MoveLastAfterFirst_B_after_A", func(t *testing.T) {
		err := service.MoveCollectionAfter(ctx, colB.ID, colA.ID)
		require.NoError(t, err)
		
		// Expected order: A, B (no change, since B is already after A)
		expectedOrder := []*mcollection.Collection{colA, colB}
		verifyCollectionIntegrity(t, service, ctx, workspaceID, expectedOrder)
	})
	
	// Test error cases
	t.Run("ErrorCases", func(t *testing.T) {
		// Move to self
		err := service.MoveCollectionAfter(ctx, colA.ID, colA.ID)
		assert.Error(t, err)
		
		// Move with non-existent target
		fakeID := idwrap.NewNow()
		err = service.MoveCollectionAfter(ctx, colA.ID, fakeID)
		assert.Error(t, err)
		
		// Move non-existent source
		err = service.MoveCollectionAfter(ctx, fakeID, colB.ID)
		assert.Error(t, err)
		
		// Verify no collections were lost
		verifyCollectionIntegrity(t, service, ctx, workspaceID, []*mcollection.Collection{colA, colB})
	})
}

func testThreeCollectionsEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 3)
	colA, colB, colC := collections[0], collections[1], collections[2]
	
	// Initial order: A, B, C
	testCases := []struct {
		name        string
		sourceID    idwrap.IDWrap
		targetID    idwrap.IDWrap
		operation   string // "after" or "before"
		expectedOrder []*mcollection.Collection
	}{
		{
			name: "MoveFirstToMiddle_A_after_B",
			sourceID: colA.ID, targetID: colB.ID, operation: "after",
			expectedOrder: []*mcollection.Collection{colB, colA, colC},
		},
		{
			name: "MoveMiddleToFirst_A_before_B", 
			sourceID: colA.ID, targetID: colB.ID, operation: "before",
			expectedOrder: []*mcollection.Collection{colA, colB, colC}, // back to original
		},
		{
			name: "MoveFirstToLast_A_after_C",
			sourceID: colA.ID, targetID: colC.ID, operation: "after",
			expectedOrder: []*mcollection.Collection{colB, colC, colA},
		},
		{
			name: "MoveLastToFirst_A_before_B",
			sourceID: colA.ID, targetID: colB.ID, operation: "before",
			expectedOrder: []*mcollection.Collection{colA, colB, colC}, // back to original
		},
		{
			name: "MoveLastToMiddle_C_before_B",
			sourceID: colC.ID, targetID: colB.ID, operation: "before",
			expectedOrder: []*mcollection.Collection{colA, colC, colB},
		},
		{
			name: "MoveMiddleToLast_C_after_B",
			sourceID: colC.ID, targetID: colB.ID, operation: "after",
			expectedOrder: []*mcollection.Collection{colA, colB, colC}, // back to original
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.operation == "after" {
				err = service.MoveCollectionAfter(ctx, tc.sourceID, tc.targetID)
			} else {
				err = service.MoveCollectionBefore(ctx, tc.sourceID, tc.targetID)
			}
			require.NoError(t, err, "Move operation should succeed")
			verifyCollectionIntegrity(t, service, ctx, workspaceID, tc.expectedOrder)
		})
	}
}

func testFiveCollectionsEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 5)
	
	// Initial order: A, B, C, D, E
	testCases := []struct {
		name        string
		description string
		moves       []moveOperation
	}{
		{
			name: "MoveFirstToEveryPosition",
			description: "Move A to every possible position",
			moves: []moveOperation{
				{"A", "after", "B", []string{"B", "A", "C", "D", "E"}}, // A after B
				{"A", "after", "C", []string{"B", "C", "A", "D", "E"}}, // A after C
				{"A", "after", "D", []string{"B", "C", "D", "A", "E"}}, // A after D  
				{"A", "after", "E", []string{"B", "C", "D", "E", "A"}}, // A after E (last)
				{"A", "before", "B", []string{"A", "B", "C", "D", "E"}}, // A back to first
			},
		},
		{
			name: "MoveLastToEveryPosition", 
			description: "Move E to every possible position",
			moves: []moveOperation{
				{"E", "before", "D", []string{"A", "B", "C", "E", "D"}}, // E before D
				{"E", "before", "C", []string{"A", "B", "E", "C", "D"}}, // E before C
				{"E", "before", "B", []string{"A", "E", "B", "C", "D"}}, // E before B
				{"E", "before", "A", []string{"E", "A", "B", "C", "D"}}, // E before A (first)
				{"E", "after", "D", []string{"A", "B", "C", "D", "E"}}, // E back to last
			},
		},
		{
			name: "MoveMiddleAroundComplex",
			description: "Complex moves of middle elements",
			moves: []moveOperation{
				{"C", "after", "A", []string{"A", "C", "B", "D", "E"}}, // C after A
				{"D", "before", "B", []string{"A", "C", "D", "B", "E"}}, // D before B
				{"B", "after", "E", []string{"A", "C", "D", "E", "B"}}, // B after E
				{"C", "before", "A", []string{"C", "A", "D", "E", "B"}}, // C before A
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Test: %s", tc.description)
			
			for i, move := range tc.moves {
				t.Logf("Step %d: Move %s %s %s", i+1, move.source, move.operation, move.target)
				
				sourceCol := findCollectionByName(collections, move.source)
				targetCol := findCollectionByName(collections, move.target)
				require.NotNil(t, sourceCol, "Source collection %s not found", move.source)
				require.NotNil(t, targetCol, "Target collection %s not found", move.target)
				
				var err error
				if move.operation == "after" {
					err = service.MoveCollectionAfter(ctx, sourceCol.ID, targetCol.ID)
				} else {
					err = service.MoveCollectionBefore(ctx, sourceCol.ID, targetCol.ID)
				}
				require.NoError(t, err, "Move %d should succeed", i+1)
				
				// Verify expected order
				expectedCollections := make([]*mcollection.Collection, len(move.expectedOrder))
				for j, name := range move.expectedOrder {
					expectedCollections[j] = findCollectionByName(collections, name)
					require.NotNil(t, expectedCollections[j], "Expected collection %s not found", name)
				}
				
				verifyCollectionIntegrity(t, service, ctx, workspaceID, expectedCollections)
			}
		})
	}
}

func testTenCollectionsEdgeCases(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, collections []*mcollection.Collection) {
	require.Len(t, collections, 10)
	
	// Test moving elements to boundary positions
	t.Run("MoveToBoundaries", func(t *testing.T) {
		colA, colJ := collections[0], collections[9]
		
		// Move first to last
		err := service.MoveCollectionAfter(ctx, colA.ID, colJ.ID)
		require.NoError(t, err)
		verifyCollectionCount(t, service, ctx, workspaceID, 10)
		
		// Move last to first  
		orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
		require.NoError(t, err)
		newLast := orderedCollections[9]
		newFirst := orderedCollections[0]
		
		err = service.MoveCollectionBefore(ctx, newLast.ID, newFirst.ID)
		require.NoError(t, err)
		verifyCollectionCount(t, service, ctx, workspaceID, 10)
	})
	
	// Test moving to middle positions
	t.Run("MoveToMiddlePositions", func(t *testing.T) {
		for i := 1; i < 9; i++ {
			// Get current order
			orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
			require.NoError(t, err)
			
			source := orderedCollections[0] // Always move first element
			target := orderedCollections[i]
			
			err = service.MoveCollectionAfter(ctx, source.ID, target.ID)
			require.NoError(t, err, "Move to position %d should succeed", i)
			
			verifyCollectionCount(t, service, ctx, workspaceID, 10)
			verifyLinkedListIntegrity(t, service, ctx, workspaceID)
		}
	})
}

// Helper types and functions

type moveOperation struct {
	source        string
	operation     string // "after" or "before"  
	target        string
	expectedOrder []string
}

func findCollectionByName(collections []*mcollection.Collection, name string) *mcollection.Collection {
	for _, col := range collections {
		if col.Name == name {
			return col
		}
	}
	return nil
}

func verifyCollectionIntegrity(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, expectedOrder []*mcollection.Collection) {
	t.Helper()
	
	// Get actual order
	actualOrder, err := service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err, "Failed to get ordered collections")
	
	// Verify count
	assert.Len(t, actualOrder, len(expectedOrder), "Collection count mismatch")
	
	if len(expectedOrder) == 0 {
		return // Empty list is valid
	}
	
	// Verify order
	for i, expected := range expectedOrder {
		if i < len(actualOrder) {
			assert.Equal(t, expected.ID, actualOrder[i].ID, 
				"Order mismatch at position %d: expected %s, got %s", 
				i, expected.Name, actualOrder[i].Name)
		}
	}
	
	// Verify linked list pointers
	verifyLinkedListIntegrity(t, service, ctx, workspaceID)
}

func verifyCollectionCount(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap, expectedCount int) {
	t.Helper()
	
	collections, err := service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, collections, expectedCount, "Collection count should be %d", expectedCount)
}

func verifyLinkedListIntegrity(t *testing.T, service CollectionService, ctx context.Context, workspaceID idwrap.IDWrap) {
	t.Helper()
	
	// Get ordered collections
	orderedCollections, err := service.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err)
	
	if len(orderedCollections) == 0 {
		return // Empty list is valid
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