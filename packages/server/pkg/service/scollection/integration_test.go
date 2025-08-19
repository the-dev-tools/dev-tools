package scollection_test

import (
	"context"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/testutil"
)

func TestCollectionMove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name            string
		setup           func(*testing.T, context.Context, *scollection.CollectionService, idwrap.IDWrap) []idwrap.IDWrap
		moveItemIndex   int // which collection to move (index in setup result)
		targetItemIndex int // which collection to move after/before
		moveAfter       bool // true for MoveAfter, false for MoveBefore
		wantOrder       []int // expected order after move (indices from original setup)
		expectError     bool
	}{
		{
			name: "move_after_basic",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			moveItemIndex:   0, // Move A
			targetItemIndex: 2, // After C
			moveAfter:       true,
			wantOrder:       []int{1, 2, 0, 3}, // B, C, A, D
			expectError:     false,
		},
		{
			name: "move_before_basic",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			moveItemIndex:   3, // Move D
			targetItemIndex: 1, // Before B
			moveAfter:       false,
			wantOrder:       []int{0, 3, 1, 2}, // A, D, B, C
			expectError:     false,
		},
		{
			name: "move_after_to_end",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			moveItemIndex:   1, // Move B
			targetItemIndex: 3, // After D (last item)
			moveAfter:       true,
			wantOrder:       []int{0, 2, 3, 1}, // A, C, D, B
			expectError:     false,
		},
		{
			name: "move_before_to_beginning",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			moveItemIndex:   2, // Move C
			targetItemIndex: 0, // Before A (first item)
			moveAfter:       false,
			wantOrder:       []int{2, 0, 1, 3}, // C, A, B, D
			expectError:     false,
		},
		{
			name: "move_adjacent_after",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			moveItemIndex:   1, // Move B
			targetItemIndex: 0, // After A (adjacent)
			moveAfter:       true,
			wantOrder:       []int{0, 1, 2, 3}, // A, B, C, D (no change)
			expectError:     false,
		},
		{
			name: "move_adjacent_before",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			moveItemIndex:   1, // Move B
			targetItemIndex: 2, // Before C (adjacent)
			moveAfter:       false,
			wantOrder:       []int{0, 1, 2, 3}, // A, B, C, D (no change)
			expectError:     false,
		},
		{
			name: "move_single_collection_workspace",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A"})
			},
			moveItemIndex:   0, // Move A
			targetItemIndex: 0, // After itself (only option)
			moveAfter:       true,
			wantOrder:       []int{0}, // A (no change)
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			base := testutil.CreateBaseDB(ctx, t)
			t.Cleanup(func() { base.Close() })

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Create workspace and user
			wsID, _ := setupWorkspaceAndUser(t, ctx, base)

			// Setup test collections
			collectionIDs := tt.setup(t, ctx, &cs, wsID)

			// Execute the move operation
			var err error
			if tt.moveAfter {
				err = cs.MoveCollectionAfter(ctx, collectionIDs[tt.moveItemIndex], collectionIDs[tt.targetItemIndex])
			} else {
				err = cs.MoveCollectionBefore(ctx, collectionIDs[tt.moveItemIndex], collectionIDs[tt.targetItemIndex])
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the order after move
			if tt.wantOrder != nil {
				verifyCollectionOrder(t, ctx, &cs, wsID, collectionIDs, tt.wantOrder)
			}
		})
	}
}

func TestCollectionReorder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name          string
		setup         func(*testing.T, context.Context, *scollection.CollectionService, idwrap.IDWrap) []idwrap.IDWrap
		reorderIndices []int // new order as indices from original setup
		expectError   bool
	}{
		{
			name: "reorder_complete_reverse",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			reorderIndices: []int{3, 2, 1, 0}, // D, C, B, A
			expectError:    false,
		},
		{
			name: "reorder_partial_shuffle",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D", "E"})
			},
			reorderIndices: []int{1, 4, 0, 2, 3}, // B, E, A, C, D
			expectError:    false,
		},
		{
			name: "reorder_same_order",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C"})
			},
			reorderIndices: []int{0, 1, 2}, // A, B, C (no change)
			expectError:    false,
		},
		{
			name: "reorder_single_collection",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A"})
			},
			reorderIndices: []int{0}, // A
			expectError:    false,
		},
		{
			name: "reorder_empty_list",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return []idwrap.IDWrap{}
			},
			reorderIndices: []int{}, // empty
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			base := testutil.CreateBaseDB(ctx, t)
			t.Cleanup(func() { base.Close() })

			mockLogger := mocklogger.NewMockLogger()
			cs := scollection.New(base.Queries, mockLogger)
			
			// Create workspace and user
			wsID, _ := setupWorkspaceAndUser(t, ctx, base)

			// Setup test collections
			collectionIDs := tt.setup(t, ctx, &cs, wsID)

			// Create the reordered list
			reorderedIDs := make([]idwrap.IDWrap, len(tt.reorderIndices))
			for i, idx := range tt.reorderIndices {
				reorderedIDs[i] = collectionIDs[idx]
			}

			// Execute the reorder operation
			err := cs.ReorderCollections(ctx, wsID, reorderedIDs)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the order after reorder
			if len(tt.reorderIndices) > 0 {
				verifyCollectionOrder(t, ctx, &cs, wsID, collectionIDs, tt.reorderIndices)
			}
		})
	}
}

func TestCircularReferenceDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("no_circular_reference_normal_operation", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D"})

		// Perform multiple moves that should maintain list integrity
		err := cs.MoveCollectionAfter(ctx, collectionIDs[0], collectionIDs[2]) // A after C
		if err != nil {
			t.Fatalf("unexpected error in first move: %v", err)
		}

		err = cs.MoveCollectionBefore(ctx, collectionIDs[3], collectionIDs[1]) // D before B
		if err != nil {
			t.Fatalf("unexpected error in second move: %v", err)
		}

		// Verify final order is valid and contains all items
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get ordered collections: %v", err)
		}

		if len(orderedCollections) != 4 {
			t.Errorf("expected 4 collections, got %d", len(orderedCollections))
		}

		// Verify all original IDs are present
		foundIDs := make(map[string]bool)
		for _, collection := range orderedCollections {
			foundIDs[collection.ID.String()] = true
		}

		for _, id := range collectionIDs {
			if !foundIDs[id.String()] {
				t.Errorf("collection %s missing from ordered list", id.String())
			}
		}
	})

	t.Run("data_integrity_after_multiple_operations", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create a larger set of collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D", "E", "F", "G", "H"})

		// Perform a series of complex moves
		operations := []struct {
			moveID   int
			targetID int
			after    bool
		}{
			{0, 3, true},  // A after D
			{7, 1, false}, // H before B
			{2, 5, true},  // C after F
			{4, 6, false}, // E before G
			{1, 0, true},  // B after A
		}

		for i, op := range operations {
			var err error
			if op.after {
				err = cs.MoveCollectionAfter(ctx, collectionIDs[op.moveID], collectionIDs[op.targetID])
			} else {
				err = cs.MoveCollectionBefore(ctx, collectionIDs[op.moveID], collectionIDs[op.targetID])
			}

			if err != nil {
				t.Fatalf("unexpected error in operation %d: %v", i, err)
			}

			// Verify integrity after each operation
			orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
			if err != nil {
				t.Fatalf("failed to get ordered collections after operation %d: %v", i, err)
			}

			if len(orderedCollections) != len(collectionIDs) {
				t.Errorf("operation %d: expected %d collections, got %d", i, len(collectionIDs), len(orderedCollections))
			}
		}
	})
}

func TestEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("empty_workspace_operations", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user but no collections
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Test getting ordered collections from empty workspace
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("unexpected error getting collections from empty workspace: %v", err)
		}

		if len(orderedCollections) != 0 {
			t.Errorf("expected 0 collections in empty workspace, got %d", len(orderedCollections))
		}

		// Test reordering empty list
		err = cs.ReorderCollections(ctx, wsID, []idwrap.IDWrap{})
		if err != nil {
			t.Errorf("unexpected error reordering empty workspace: %v", err)
		}

		// Test compacting empty workspace
		err = cs.CompactCollectionPositions(ctx, wsID)
		if err != nil {
			t.Errorf("unexpected error compacting empty workspace: %v", err)
		}
	})

	t.Run("single_item_operations", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create single collection
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"Lonely"})

		// Test moving after itself
		err := cs.MoveCollectionAfter(ctx, collectionIDs[0], collectionIDs[0])
		if err != nil {
			t.Errorf("unexpected error moving single collection after itself: %v", err)
		}

		// Test moving before itself
		err = cs.MoveCollectionBefore(ctx, collectionIDs[0], collectionIDs[0])
		if err != nil {
			t.Errorf("unexpected error moving single collection before itself: %v", err)
		}

		// Test reordering single item
		err = cs.ReorderCollections(ctx, wsID, collectionIDs)
		if err != nil {
			t.Errorf("unexpected error reordering single collection: %v", err)
		}

		// Test compacting single item
		err = cs.CompactCollectionPositions(ctx, wsID)
		if err != nil {
			t.Errorf("unexpected error compacting single collection: %v", err)
		}

		// Verify the collection is still there and correct
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections: %v", err)
		}

		if len(orderedCollections) != 1 {
			t.Errorf("expected 1 collection, got %d", len(orderedCollections))
		} else if orderedCollections[0].ID.Compare(collectionIDs[0]) != 0 {
			t.Errorf("collection ID mismatch after operations")
		}
	})

	t.Run("invalid_collection_references", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C"})

		// Create a non-existent collection ID
		nonExistentID := idwrap.NewNow()

		// Test moving non-existent collection
		err := cs.MoveCollectionAfter(ctx, nonExistentID, collectionIDs[0])
		if err == nil {
			t.Errorf("expected error when moving non-existent collection, got none")
		}

		// Test moving collection after non-existent target
		err = cs.MoveCollectionAfter(ctx, collectionIDs[0], nonExistentID)
		if err == nil {
			t.Errorf("expected error when moving collection after non-existent target, got none")
		}

		// Test reordering with non-existent collection
		invalidReorderList := []idwrap.IDWrap{collectionIDs[0], nonExistentID, collectionIDs[1]}
		err = cs.ReorderCollections(ctx, wsID, invalidReorderList)
		if err == nil {
			t.Errorf("expected error when reordering with non-existent collection, got none")
		}

		// Verify original collections are unchanged
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections after invalid operations: %v", err)
		}

		if len(orderedCollections) != 3 {
			t.Errorf("expected 3 collections after invalid operations, got %d", len(orderedCollections))
		}
	})
}

func TestTransactionalMoves(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("successful_transaction_commit", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D"})

		// Start transaction
		tx, err := base.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to start transaction: %v", err)
		}
		defer tx.Rollback() // Safety rollback

		// Perform multiple moves within transaction
		err = cs.MoveCollectionAfterTX(ctx, tx, collectionIDs[0], collectionIDs[2]) // A after C
		if err != nil {
			t.Fatalf("failed to move collection in transaction: %v", err)
		}

		err = cs.MoveCollectionBeforeTX(ctx, tx, collectionIDs[3], collectionIDs[1]) // D before B
		if err != nil {
			t.Fatalf("failed to move collection in transaction: %v", err)
		}

		// Commit transaction
		err = tx.Commit()
		if err != nil {
			t.Fatalf("failed to commit transaction: %v", err)
		}

		// Verify changes are persisted
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections after transaction: %v", err)
		}

		if len(orderedCollections) != 4 {
			t.Errorf("expected 4 collections after transaction, got %d", len(orderedCollections))
		}

		// Verify specific order based on the moves made
		// Original: A, B, C, D
		// After A after C: B, C, A, D
		// After D before B: D, B, C, A
		expectedNames := []string{"D", "B", "C", "A"}
		for i, expectedName := range expectedNames {
			if orderedCollections[i].Name != expectedName {
				t.Errorf("at position %d: expected %s, got %s", i, expectedName, orderedCollections[i].Name)
			}
		}
	})

	t.Run("transaction_rollback", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D"})

		// Store original order
		originalOrder, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get original order: %v", err)
		}

		// Start transaction
		tx, err := base.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to start transaction: %v", err)
		}

		// Perform moves within transaction
		err = cs.MoveCollectionAfterTX(ctx, tx, collectionIDs[0], collectionIDs[3]) // A after D
		if err != nil {
			t.Fatalf("failed to move collection in transaction: %v", err)
		}

		err = cs.ReorderCollectionsTX(ctx, tx, wsID, []idwrap.IDWrap{collectionIDs[3], collectionIDs[2], collectionIDs[1], collectionIDs[0]}) // Reverse order
		if err != nil {
			t.Fatalf("failed to reorder collections in transaction: %v", err)
		}

		// Rollback transaction
		err = tx.Rollback()
		if err != nil {
			t.Fatalf("failed to rollback transaction: %v", err)
		}

		// Verify original order is restored
		currentOrder, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections after rollback: %v", err)
		}

		if len(currentOrder) != len(originalOrder) {
			t.Errorf("collection count changed after rollback: expected %d, got %d", len(originalOrder), len(currentOrder))
		}

		for i, originalCollection := range originalOrder {
			if i >= len(currentOrder) || currentOrder[i].ID.Compare(originalCollection.ID) != 0 {
				t.Errorf("order changed after rollback at position %d", i)
			}
		}
	})

	t.Run("transaction_with_compact_positions", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D", "E"})

		// Start transaction
		tx, err := base.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to start transaction: %v", err)
		}
		defer tx.Rollback()

		// Perform some moves to potentially create gaps
		err = cs.MoveCollectionAfterTX(ctx, tx, collectionIDs[0], collectionIDs[4]) // A after E
		if err != nil {
			t.Fatalf("failed to move collection: %v", err)
		}

		err = cs.MoveCollectionBeforeTX(ctx, tx, collectionIDs[2], collectionIDs[1]) // C before B
		if err != nil {
			t.Fatalf("failed to move collection: %v", err)
		}

		// Compact positions within transaction
		err = cs.CompactCollectionPositionsTX(ctx, tx, wsID)
		if err != nil {
			t.Fatalf("failed to compact positions: %v", err)
		}

		// Commit transaction
		err = tx.Commit()
		if err != nil {
			t.Fatalf("failed to commit transaction: %v", err)
		}

		// Verify all collections are still present and in consistent order
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections after compact: %v", err)
		}

		if len(orderedCollections) != 5 {
			t.Errorf("expected 5 collections after compact, got %d", len(orderedCollections))
		}

		// Verify no gaps in logical ordering (all collections present)
		foundIDs := make(map[string]bool)
		for _, collection := range orderedCollections {
			foundIDs[collection.ID.String()] = true
		}

		for _, id := range collectionIDs {
			if !foundIDs[id.String()] {
				t.Errorf("collection %s missing after compact", id.String())
			}
		}
	})
}

func TestConcurrentMoves(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("concurrent_move_operations", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create more collections for better concurrency testing
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D", "E", "F", "G", "H"})

		const numGoroutines = 8
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		// Launch concurrent move operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				// Each goroutine performs a different move operation
				sourceIdx := routineID % len(collectionIDs)
				targetIdx := (routineID + 1) % len(collectionIDs)

				if sourceIdx == targetIdx {
					targetIdx = (targetIdx + 1) % len(collectionIDs)
				}

				var err error
				if routineID%2 == 0 {
					err = cs.MoveCollectionAfter(ctx, collectionIDs[sourceIdx], collectionIDs[targetIdx])
				} else {
					err = cs.MoveCollectionBefore(ctx, collectionIDs[sourceIdx], collectionIDs[targetIdx])
				}

				if err != nil {
					errors <- err
				}
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(errors)

		// Check for errors
		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		// We allow some concurrent operations to fail due to race conditions,
		// but the final state should be consistent
		if len(errs) > 0 {
			t.Logf("Concurrent operations had %d errors (expected in concurrent scenario): %v", len(errs), errs)
		}

		// Verify data integrity after concurrent operations
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections after concurrent operations: %v", err)
		}

		// Verify all collections are still present
		if len(orderedCollections) != len(collectionIDs) {
			t.Errorf("expected %d collections after concurrent operations, got %d", len(collectionIDs), len(orderedCollections))
		}

		// Verify no duplicates
		foundIDs := make(map[string]bool)
		for _, collection := range orderedCollections {
			idStr := collection.ID.String()
			if foundIDs[idStr] {
				t.Errorf("duplicate collection found: %s", idStr)
			}
			foundIDs[idStr] = true
		}

		// Verify all original IDs are present
		for _, id := range collectionIDs {
			if !foundIDs[id.String()] {
				t.Errorf("collection %s missing after concurrent operations", id.String())
			}
		}
	})

	t.Run("concurrent_reorder_operations", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D", "E"})

		const numGoroutines = 5
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		// Launch concurrent reorder operations with different orders
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				// Create different reorder patterns
				reorderedIDs := make([]idwrap.IDWrap, len(collectionIDs))
				switch routineID {
				case 0:
					// Reverse order
					for i, id := range collectionIDs {
						reorderedIDs[len(collectionIDs)-1-i] = id
					}
				case 1:
					// Rotate left
					copy(reorderedIDs, collectionIDs[1:])
					reorderedIDs[len(reorderedIDs)-1] = collectionIDs[0]
				case 2:
					// Rotate right
					reorderedIDs[0] = collectionIDs[len(collectionIDs)-1]
					copy(reorderedIDs[1:], collectionIDs[:len(collectionIDs)-1])
				case 3:
					// Swap first and last
					copy(reorderedIDs, collectionIDs)
					reorderedIDs[0], reorderedIDs[len(reorderedIDs)-1] = reorderedIDs[len(reorderedIDs)-1], reorderedIDs[0]
				case 4:
					// Original order
					copy(reorderedIDs, collectionIDs)
				}

				err := cs.ReorderCollections(ctx, wsID, reorderedIDs)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(errors)

		// Check for errors
		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		// Some concurrent reorder operations may fail, which is acceptable
		if len(errs) > 0 {
			t.Logf("Concurrent reorder operations had %d errors (expected): %v", len(errs), errs)
		}

		// Verify final state integrity
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections after concurrent reorders: %v", err)
		}

		// Verify all collections are still present
		if len(orderedCollections) != len(collectionIDs) {
			t.Errorf("expected %d collections, got %d", len(collectionIDs), len(orderedCollections))
		}

		// Verify no duplicates and all IDs present
		foundIDs := make(map[string]bool)
		for _, collection := range orderedCollections {
			idStr := collection.ID.String()
			if foundIDs[idStr] {
				t.Errorf("duplicate collection found: %s", idStr)
			}
			foundIDs[idStr] = true
		}

		for _, id := range collectionIDs {
			if !foundIDs[id.String()] {
				t.Errorf("collection %s missing after concurrent reorders", id.String())
			}
		}
	})

	t.Run("race_condition_stress_test", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C", "D", "E", "F"})

		const numOperations = 20
		var wg sync.WaitGroup
		errors := make(chan error, numOperations)

		// Mix of different operations running concurrently
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(opID int) {
				defer wg.Done()

				switch opID % 4 {
				case 0:
					// Move after
					sourceIdx := opID % len(collectionIDs)
					targetIdx := (opID + 2) % len(collectionIDs)
					if sourceIdx == targetIdx {
						targetIdx = (targetIdx + 1) % len(collectionIDs)
					}
					err := cs.MoveCollectionAfter(ctx, collectionIDs[sourceIdx], collectionIDs[targetIdx])
					if err != nil {
						errors <- err
					}
				case 1:
					// Move before
					sourceIdx := opID % len(collectionIDs)
					targetIdx := (opID + 3) % len(collectionIDs)
					if sourceIdx == targetIdx {
						targetIdx = (targetIdx + 1) % len(collectionIDs)
					}
					err := cs.MoveCollectionBefore(ctx, collectionIDs[sourceIdx], collectionIDs[targetIdx])
					if err != nil {
						errors <- err
					}
				case 2:
					// Reorder (partial)
					reorderLen := (opID%3) + 2 // 2-4 items
					if reorderLen > len(collectionIDs) {
						reorderLen = len(collectionIDs)
					}
					reorderedIDs := make([]idwrap.IDWrap, reorderLen)
					for j := 0; j < reorderLen; j++ {
						reorderedIDs[j] = collectionIDs[(opID+j)%len(collectionIDs)]
					}
					err := cs.ReorderCollections(ctx, wsID, reorderedIDs)
					if err != nil {
						errors <- err
					}
				case 3:
					// Compact positions
					err := cs.CompactCollectionPositions(ctx, wsID)
					if err != nil {
						errors <- err
					}
				}
			}(i)
		}

		// Wait for all operations to complete
		wg.Wait()
		close(errors)

		// Collect errors
		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		// Log errors but don't fail test - some concurrent operations failing is expected
		if len(errs) > 0 {
			t.Logf("Stress test had %d errors out of %d operations (%.1f%% error rate, expected in high-concurrency scenario): %v", 
				len(errs), numOperations, float64(len(errs))/float64(numOperations)*100, errs)
		}

		// Verify final state integrity is maintained
		orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
		if err != nil {
			t.Fatalf("failed to get collections after stress test: %v", err)
		}

		// Critical integrity checks
		if len(orderedCollections) != len(collectionIDs) {
			t.Errorf("data integrity failure: expected %d collections, got %d", len(collectionIDs), len(orderedCollections))
		}

		foundIDs := make(map[string]bool)
		for _, collection := range orderedCollections {
			idStr := collection.ID.String()
			if foundIDs[idStr] {
				t.Errorf("data integrity failure: duplicate collection %s", idStr)
			}
			foundIDs[idStr] = true
		}

		for _, id := range collectionIDs {
			if !foundIDs[id.String()] {
				t.Errorf("data integrity failure: missing collection %s", id.String())
			}
		}
	})
}