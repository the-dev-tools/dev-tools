package scollection_test

import (
	"context"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/testutil"
	"time"
)

func TestCollectionMovableRepository_UpdatePosition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name         string
		setup        func(*testing.T, context.Context, *scollection.CollectionService, idwrap.IDWrap) []idwrap.IDWrap
		itemIndex    int // which collection to move (index in setup result)
		newPosition  int
		wantOrder    []int // expected order after move (indices from original setup)
		expectError  bool
	}{
        {
            name: "move_first_to_last",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
            itemIndex:   0, // move A
            newPosition: 4, // to last position (account for base collection at position 0)
			wantOrder:   []int{1, 2, 3, 0}, // B, C, D, A
			expectError: false,
		},
        {
            name: "move_last_to_first",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
            itemIndex:   3, // move D
            newPosition: 1, // to first position among created (base occupies 0)
			wantOrder:   []int{3, 0, 1, 2}, // D, A, B, C
			expectError: false,
		},
        {
            name: "move_middle_to_middle",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D", "E"})
			},
            itemIndex:   1, // move B
            newPosition: 4, // to middle position (account for base at 0)
			wantOrder:   []int{0, 2, 3, 1, 4}, // A, C, D, B, E
			expectError: false,
		},
        {
            name: "move_to_same_position",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C"})
			},
            itemIndex:   1, // move B
            newPosition: 2, // same relative position (base at 0)
			wantOrder:   []int{0, 1, 2}, // A, B, C (no change)
			expectError: false,
		},
        {
            name: "move_single_item",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A"})
			},
            itemIndex:   0, // move A
            newPosition: 1, // keep after base (only valid without changing position)
			wantOrder:   []int{0}, // A (no change)
			expectError: false,
		},
		{
			name: "invalid_position_negative",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C"})
			},
			itemIndex:   1, // move B
			newPosition: -1, // invalid position
			expectError: true,
		},
		{
			name: "invalid_position_too_large",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C"})
			},
			itemIndex:   1, // move B
			newPosition: 5, // invalid position (> length-1)
			expectError: true,
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
			
			// Create repository directly to test its methods
			repo := scollection.NewCollectionMovableRepository(base.Queries)

			// Execute the move operation
			err := repo.UpdatePosition(ctx, nil, collectionIDs[tt.itemIndex], movable.CollectionListTypeCollections, tt.newPosition)

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

func TestCollectionMovableRepository_UpdatePositions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name        string
		setup       func(*testing.T, context.Context, *scollection.CollectionService, idwrap.IDWrap) []idwrap.IDWrap
		updates     func([]idwrap.IDWrap) []movable.PositionUpdate // generate updates based on collection IDs
		wantOrder   []int // expected order after updates (indices from original setup)
		expectError bool
	}{
		{
			name: "batch_reorder_complete",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			updates: func(ids []idwrap.IDWrap) []movable.PositionUpdate {
				// Reverse the order: A->3, B->2, C->1, D->0
				return []movable.PositionUpdate{
					{ItemID: ids[0], ListType: movable.CollectionListTypeCollections, Position: 3}, // A to position 3
					{ItemID: ids[1], ListType: movable.CollectionListTypeCollections, Position: 2}, // B to position 2
					{ItemID: ids[2], ListType: movable.CollectionListTypeCollections, Position: 1}, // C to position 1
					{ItemID: ids[3], ListType: movable.CollectionListTypeCollections, Position: 0}, // D to position 0
				}
			},
			wantOrder:   []int{3, 2, 1, 0}, // D, C, B, A
			expectError: false,
		},
		{
			name: "batch_partial_reorder",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D", "E"})
			},
			updates: func(ids []idwrap.IDWrap) []movable.PositionUpdate {
				// Only move B and D. Repository UpdatePositions expects a full-batch reorder;
				// partial updates are not supported, so this case should error.
				return []movable.PositionUpdate{
					{ItemID: ids[1], ListType: movable.CollectionListTypeCollections, Position: 3}, // B to position 3
					{ItemID: ids[3], ListType: movable.CollectionListTypeCollections, Position: 1}, // D to position 1
				}
			},
			wantOrder:   nil,
			expectError: true,
		},
		{
			name: "empty_updates",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C"})
			},
			updates: func(ids []idwrap.IDWrap) []movable.PositionUpdate {
				return []movable.PositionUpdate{}
			},
			wantOrder:   []int{0, 1, 2}, // A, B, C (no change)
			expectError: false,
		},
		{
			name: "invalid_position_in_batch",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C"})
			},
			updates: func(ids []idwrap.IDWrap) []movable.PositionUpdate {
				return []movable.PositionUpdate{
					{ItemID: ids[0], ListType: movable.CollectionListTypeCollections, Position: 1}, // Valid
					{ItemID: ids[1], ListType: movable.CollectionListTypeCollections, Position: 5}, // Invalid
				}
			},
			expectError: true,
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
			
			// Create repository directly to test its methods
			repo := scollection.NewCollectionMovableRepository(base.Queries)

			// Generate updates
			updates := tt.updates(collectionIDs)

			// Execute the batch update operation
			err := repo.UpdatePositions(ctx, nil, updates)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the order after updates
			if tt.wantOrder != nil {
				verifyCollectionOrder(t, ctx, &cs, wsID, collectionIDs, tt.wantOrder)
			}
		})
	}
}

func TestCollectionMovableRepository_GetMaxPosition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name           string
		setup          func(*testing.T, context.Context, *scollection.CollectionService, idwrap.IDWrap) []idwrap.IDWrap
		expectedMaxPos int
		expectError    bool
	}{
		{
			name: "empty_workspace",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return []idwrap.IDWrap{}
			},
			expectedMaxPos: 0, // Base collection from setup means max position 0
			expectError:    false,
		},
		{
			name: "single_collection",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A"})
			},
			expectedMaxPos: 1, // Base collection + 1 additional = max position 1
			expectError:    false,
		},
		{
			name: "multiple_collections",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D", "E"})
			},
			expectedMaxPos: 5, // Base collection + 5 additional = max position 5
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
			_ = tt.setup(t, ctx, &cs, wsID)
			
			// Create repository directly to test its methods
			repo := scollection.NewCollectionMovableRepository(base.Queries)

			// Get max position
			maxPos, err := repo.GetMaxPosition(ctx, wsID, movable.CollectionListTypeCollections)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if maxPos != tt.expectedMaxPos {
				t.Errorf("expected max position %d, got %d", tt.expectedMaxPos, maxPos)
			}
		})
	}
}

func TestCollectionMovableRepository_GetItemsByParent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name          string
		setup         func(*testing.T, context.Context, *scollection.CollectionService, idwrap.IDWrap) []idwrap.IDWrap
		expectedCount int
		expectError   bool
	}{
		{
			name: "empty_workspace",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return []idwrap.IDWrap{}
			},
			expectedCount: 1, // Base collection from setup
			expectError:   false,
		},
		{
			name: "single_collection",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A"})
			},
			expectedCount: 2, // Base collection + 1 additional
			expectError:   false,
		},
		{
			name: "multiple_collections",
			setup: func(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
				return createTestCollections(t, ctx, cs, wsID, []string{"A", "B", "C", "D"})
			},
			expectedCount: 5, // Base collection + 4 additional
			expectError:   false,
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
			
			// Create repository directly to test its methods
			repo := scollection.NewCollectionMovableRepository(base.Queries)

			// Get items by parent
			items, err := repo.GetItemsByParent(ctx, wsID, movable.CollectionListTypeCollections)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(items) != tt.expectedCount {
				t.Errorf("expected %d items, got %d", tt.expectedCount, len(items))
			}

			// Verify that items are in correct order and have correct properties
			for i, item := range items {
				// Position should match the index (sequential positions)
				if item.Position != i {
					t.Errorf("expected item at index %d to have position %d, got %d", i, i, item.Position)
				}

				if item.ParentID == nil {
					t.Errorf("expected item at index %d to have parent ID, got nil", i)
				} else if item.ParentID.Compare(wsID) != 0 {
					t.Errorf("expected item at index %d to have parent ID %s, got %s", i, wsID.String(), item.ParentID.String())
				}

				if item.ListType != movable.CollectionListTypeCollections {
					t.Errorf("expected item at index %d to have list type Collections, got %s", i, item.ListType.String())
				}
			}

			// For non-empty test cases, verify that at least some of our created collections are present
			if len(collectionIDs) > 0 {
				foundCreatedCollections := 0
				itemIDs := make(map[string]bool)
				for _, item := range items {
					itemIDs[item.ID.String()] = true
				}
				
				for _, id := range collectionIDs {
					if itemIDs[id.String()] {
						foundCreatedCollections++
					}
				}
				
				if foundCreatedCollections != len(collectionIDs) {
					t.Errorf("expected to find %d created collections, but found %d", len(collectionIDs), foundCreatedCollections)
				}
			}
		})
	}
}

func TestCollectionMovableRepository_TransactionSupport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("update_position_with_transaction", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C"})

		// Start transaction
		tx, err := base.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to start transaction: %v", err)
		}

		// Create repository with transaction
		repo := scollection.NewCollectionMovableRepository(base.Queries)

        // Move collection B to last position within transaction (account for base at 0)
        err = repo.UpdatePosition(ctx, tx, collectionIDs[1], movable.CollectionListTypeCollections, 3)
		if err != nil {
			tx.Rollback()
			t.Fatalf("failed to update position: %v", err)
		}

		// Commit transaction
		err = tx.Commit()
		if err != nil {
			t.Fatalf("failed to commit transaction: %v", err)
		}

		// Verify the order after transaction commit
		verifyCollectionOrder(t, ctx, &cs, wsID, collectionIDs, []int{0, 2, 1}) // A, C, B
	})

	t.Run("rollback_position_update", func(t *testing.T) {
		ctx := context.Background()
		base := testutil.CreateBaseDB(ctx, t)
		t.Cleanup(func() { base.Close() })

		mockLogger := mocklogger.NewMockLogger()
		cs := scollection.New(base.Queries, mockLogger)
		
		// Create workspace and user
		wsID, _ := setupWorkspaceAndUser(t, ctx, base)

		// Create test collections
		collectionIDs := createTestCollections(t, ctx, &cs, wsID, []string{"A", "B", "C"})

		// Store original order
		originalOrder := getCollectionOrder(t, ctx, &cs, wsID)

		// Start transaction
		tx, err := base.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("failed to start transaction: %v", err)
		}

		// Create repository with transaction
		repo := scollection.NewCollectionMovableRepository(base.Queries)

        // Move collection B to last position within transaction (account for base at 0)
        err = repo.UpdatePosition(ctx, tx, collectionIDs[1], movable.CollectionListTypeCollections, 3)
		if err != nil {
			tx.Rollback()
			t.Fatalf("failed to update position: %v", err)
		}

		// Rollback transaction
		err = tx.Rollback()
		if err != nil {
			t.Fatalf("failed to rollback transaction: %v", err)
		}

		// Verify that the order is unchanged after rollback
		currentOrder := getCollectionOrder(t, ctx, &cs, wsID)
		if len(currentOrder) != len(originalOrder) {
			t.Errorf("expected order length %d, got %d after rollback", len(originalOrder), len(currentOrder))
		}

		for i, id := range originalOrder {
			if i >= len(currentOrder) || currentOrder[i].Compare(id) != 0 {
				t.Errorf("order changed after rollback at index %d", i)
			}
		}
	})
}

// Helper functions for testing

func setupWorkspaceAndUser(t *testing.T, ctx context.Context, base *testutil.BaseDBQueries) (wsID, userID idwrap.IDWrap) {
	t.Helper()
	
	wsID = idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID = idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsuserID, userID, baseCollectionID)
	return wsID, userID
}

func createTestCollections(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap, names []string) []idwrap.IDWrap {
	t.Helper()
	
	ids := make([]idwrap.IDWrap, len(names))
	for i, name := range names {
		id := idwrap.NewNow()
		collection := &mcollection.Collection{
			ID:          id,
			WorkspaceID: wsID,
			Name:        name,
			Updated:     time.Now(),
		}
		
		err := cs.CreateCollection(ctx, collection)
		if err != nil {
			t.Fatalf("failed to create collection %s: %v", name, err)
		}
		
		ids[i] = id
	}
	
	return ids
}

func verifyCollectionOrder(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap, originalIDs []idwrap.IDWrap, expectedOrder []int) {
	t.Helper()
	
	orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
	if err != nil {
		t.Fatalf("failed to get ordered collections: %v", err)
	}
	
	// Create a map of original IDs for quick lookup
	originalIDMap := make(map[string]int)
	for i, id := range originalIDs {
		originalIDMap[id.String()] = i
	}
	
	// Filter out collections that are from our original set
	var relevantCollections []mcollection.Collection
	var relevantIndices []int
	
	for _, collection := range orderedCollections {
		if idx, exists := originalIDMap[collection.ID.String()]; exists {
			relevantCollections = append(relevantCollections, collection)
			relevantIndices = append(relevantIndices, idx)
		}
	}
	
	// Verify the order of our relevant collections matches the expected order
	if len(relevantCollections) != len(expectedOrder) {
		t.Errorf("expected %d relevant collections, got %d", len(expectedOrder), len(relevantCollections))
		return
	}
	
	for i, expectedIdx := range expectedOrder {
		if i >= len(relevantIndices) {
			t.Errorf("missing collection at position %d", i)
			continue
		}
		
		actualIdx := relevantIndices[i]
		if actualIdx != expectedIdx {
			t.Errorf("at position %d: expected collection with original index %d, got %d", i, expectedIdx, actualIdx)
		}
	}
}

func getCollectionOrder(t *testing.T, ctx context.Context, cs *scollection.CollectionService, wsID idwrap.IDWrap) []idwrap.IDWrap {
	t.Helper()
	
	orderedCollections, err := cs.GetCollectionsOrdered(ctx, wsID)
	if err != nil {
		t.Fatalf("failed to get ordered collections: %v", err)
	}
	
	ids := make([]idwrap.IDWrap, len(orderedCollections))
	for i, collection := range orderedCollections {
		ids[i] = collection.ID
	}
	
	return ids
}
