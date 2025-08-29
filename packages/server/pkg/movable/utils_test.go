package movable_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
)

// MockMovableRepository implements MovableRepository for testing
type MockMovableRepository struct {
	items           map[string][]movable.MovableItem
	maxPositions    map[string]int
	updatePositions func(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error
}

func NewMockMovableRepository() *MockMovableRepository {
	return &MockMovableRepository{
		items:        make(map[string][]movable.MovableItem),
		maxPositions: make(map[string]int),
	}
}

func (m *MockMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Simple mock implementation
	return nil
}

func (m *MockMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if m.updatePositions != nil {
		return m.updatePositions(ctx, tx, updates)
	}
	return nil
}

func (m *MockMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	key := parentID.String() + listType.String()
	if max, exists := m.maxPositions[key]; exists {
		return max, nil
	}
	return 0, nil
}

func (m *MockMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	key := parentID.String() + listType.String()
	if items, exists := m.items[key]; exists {
		return items, nil
	}
	return []movable.MovableItem{}, nil
}

// Helper function to create test IDs
func createTestID(suffix string) idwrap.IDWrap {
	// ULID must be exactly 26 characters
	base := "01HPQR2S3T4U5V6W7X8Y"
	if len(suffix) > 6 {
		suffix = suffix[:6]
	}
	// Pad with zeros if suffix is too short
	for len(base+suffix) < 26 {
		suffix += "0"
	}
	id := base + suffix
	if len(id) > 26 {
		id = id[:26]
	}
	return idwrap.NewTextMust(id)
}

func TestLinkedListUtils_ValidateMoveOperation(t *testing.T) {
	utils := movable.NewLinkedListUtils()
	ctx := context.Background()

	tests := []struct {
		name      string
		operation movable.MoveOperation
		wantError bool
		errorType error
	}{
		{
			name: "valid after operation",
			operation: movable.MoveOperation{
				ItemID:   createTestID("ABC"),
				TargetID: &[]idwrap.IDWrap{createTestID("DEF")}[0],
				Position: movable.MovePositionAfter,
				ListType: movable.CollectionListTypeItems,
			},
			wantError: false,
		},
		{
			name: "valid before operation",
			operation: movable.MoveOperation{
				ItemID:   createTestID("ABC"),
				TargetID: &[]idwrap.IDWrap{createTestID("DEF")}[0],
				Position: movable.MovePositionBefore,
				ListType: movable.CollectionListTypeItems,
			},
			wantError: false,
		},
		{
			name: "empty item ID",
			operation: movable.MoveOperation{
				ItemID:   idwrap.IDWrap{},
				TargetID: &[]idwrap.IDWrap{createTestID("DEF")}[0],
				Position: movable.MovePositionAfter,
				ListType: movable.CollectionListTypeItems,
			},
			wantError: true,
			errorType: movable.ErrEmptyItemID,
		},
		{
			name: "nil list type",
			operation: movable.MoveOperation{
				ItemID:   createTestID("ABC"),
				TargetID: &[]idwrap.IDWrap{createTestID("DEF")}[0],
				Position: movable.MovePositionAfter,
				ListType: nil,
			},
			wantError: true,
			errorType: movable.ErrInvalidListType,
		},
		{
			name: "unspecified position",
			operation: movable.MoveOperation{
				ItemID:   createTestID("ABC"),
				TargetID: &[]idwrap.IDWrap{createTestID("DEF")}[0],
				Position: movable.MovePositionUnspecified,
				ListType: movable.CollectionListTypeItems,
			},
			wantError: true,
			errorType: movable.ErrInvalidPosition,
		},
		{
			name: "missing target for after position",
			operation: movable.MoveOperation{
				ItemID:   createTestID("ABC"),
				TargetID: nil,
				Position: movable.MovePositionAfter,
				ListType: movable.CollectionListTypeItems,
			},
			wantError: true,
			errorType: movable.ErrEmptyTargetID,
		},
		{
			name: "self-referential move",
			operation: movable.MoveOperation{
				ItemID:   createTestID("ABC"),
				TargetID: &[]idwrap.IDWrap{createTestID("ABC")}[0],
				Position: movable.MovePositionAfter,
				ListType: movable.CollectionListTypeItems,
			},
			wantError: true,
			errorType: movable.ErrSelfReference,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := utils.ValidateMoveOperation(ctx, tt.operation)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("expected error %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLinkedListUtils_BuildMoveContext(t *testing.T) {
	utils := movable.NewLinkedListUtils()
	ctx := context.Background()

	item1ID := createTestID("ABC")
	item2ID := createTestID("DEF")
	item3ID := createTestID("GHI")
	parentID := createTestID("PAR")

	allItems := []movable.LinkedListPointers{
		{ItemID: item1ID, ParentID: parentID, Position: 0},
		{ItemID: item2ID, ParentID: parentID, Position: 1},
		{ItemID: item3ID, ParentID: parentID, Position: 2},
	}

	tests := []struct {
		name      string
		operation movable.MoveOperation
		allItems  []movable.LinkedListPointers
		wantError bool
		wantCtx   *movable.MoveContext
	}{
		{
			name: "valid move context",
			operation: movable.MoveOperation{
				ItemID:   item1ID,
				TargetID: &item2ID,
				Position: movable.MovePositionAfter,
				ListType: movable.CollectionListTypeItems,
			},
			allItems:  allItems,
			wantError: false,
			wantCtx: &movable.MoveContext{
				ItemID:      item1ID,
				TargetID:    item2ID,
				ParentID:    parentID,
				ItemIndex:   0,
				TargetIndex: 1,
			},
		},
		{
			name: "item not found",
			operation: movable.MoveOperation{
				ItemID:   createTestID("XXX"),
				TargetID: &item2ID,
				Position: movable.MovePositionAfter,
				ListType: movable.CollectionListTypeItems,
			},
			allItems:  allItems,
			wantError: true,
		},
		{
			name: "target not found",
			operation: movable.MoveOperation{
				ItemID:   item1ID,
				TargetID: &[]idwrap.IDWrap{createTestID("YYY")}[0],
				Position: movable.MovePositionAfter,
				ListType: movable.CollectionListTypeItems,
			},
			allItems:  allItems,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moveCtx, err := utils.BuildMoveContext(ctx, tt.operation, tt.allItems)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if moveCtx == nil {
					t.Errorf("expected move context but got nil")
					return
				}
				if moveCtx.ItemIndex != tt.wantCtx.ItemIndex {
					t.Errorf("expected ItemIndex %d, got %d", tt.wantCtx.ItemIndex, moveCtx.ItemIndex)
				}
				if moveCtx.TargetIndex != tt.wantCtx.TargetIndex {
					t.Errorf("expected TargetIndex %d, got %d", tt.wantCtx.TargetIndex, moveCtx.TargetIndex)
				}
			}
		})
	}
}

func TestLinkedListUtils_CalculatePointerUpdatesAfter(t *testing.T) {
	utils := movable.NewLinkedListUtils()

	item1ID := createTestID("001")
	item2ID := createTestID("002")
	item3ID := createTestID("003")
	item4ID := createTestID("004")
	parentID := createTestID("PAR")

	// Test scenario: Move item1 after item3
	// Original order: [item1, item2, item3, item4]
	// Expected order: [item2, item3, item1, item4]
	allItems := []movable.LinkedListPointers{
		{ItemID: item1ID, ParentID: parentID, Position: 0},
		{ItemID: item2ID, ParentID: parentID, Position: 1},
		{ItemID: item3ID, ParentID: parentID, Position: 2},
		{ItemID: item4ID, ParentID: parentID, Position: 3},
	}

	moveCtx := &movable.MoveContext{
		ItemID:      item1ID,
		TargetID:    item3ID,
		ParentID:    parentID,
		AllItems:    allItems,
		ItemIndex:   0,
		TargetIndex: 2,
	}

	calculations, err := utils.CalculatePointerUpdatesAfter(moveCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calculations) != 4 {
		t.Fatalf("expected 4 calculations, got %d", len(calculations))
	}

	// Verify the expected order: [item2, item3, item1, item4]
	expectedOrder := []idwrap.IDWrap{item2ID, item3ID, item1ID, item4ID}
	for i, expectedID := range expectedOrder {
		_ = i
		found := false
		for _, c := range calculations {
			if c.ItemID == expectedID && c.NewPos == i {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected item %s at position %d, but not found in calculations", expectedID.String(), i)
		}
	}
}

func TestLinkedListUtils_CalculatePointerUpdatesBefore(t *testing.T) {
	utils := movable.NewLinkedListUtils()

	item1ID := createTestID("001")
	item2ID := createTestID("002")
	item3ID := createTestID("003")
	item4ID := createTestID("004")
	parentID := createTestID("PAR")

	// Test scenario: Move item4 before item2
	// Original order: [item1, item2, item3, item4]
	// Expected order: [item1, item4, item2, item3]
	allItems := []movable.LinkedListPointers{
		{ItemID: item1ID, ParentID: parentID, Position: 0},
		{ItemID: item2ID, ParentID: parentID, Position: 1},
		{ItemID: item3ID, ParentID: parentID, Position: 2},
		{ItemID: item4ID, ParentID: parentID, Position: 3},
	}

	moveCtx := &movable.MoveContext{
		ItemID:      item4ID,
		TargetID:    item2ID,
		ParentID:    parentID,
		AllItems:    allItems,
		ItemIndex:   3,
		TargetIndex: 1,
	}

	calculations, err := utils.CalculatePointerUpdatesBefore(moveCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calculations) != 4 {
		t.Fatalf("expected 4 calculations, got %d", len(calculations))
	}

	// Verify the expected order: [item1, item4, item2, item3]
	expectedOrder := []idwrap.IDWrap{item1ID, item4ID, item2ID, item3ID}
	for i, expectedID := range expectedOrder {
		found := false
		for _, c := range calculations {
			if c.ItemID == expectedID && c.NewPos == i {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected item %s at position %d, but not found in calculations", expectedID.String(), i)
		}
	}
}

func TestLinkedListUtils_ValidateListIntegrity(t *testing.T) {
	utils := movable.NewLinkedListUtils()
	ctx := context.Background()

	item1ID := createTestID("001")
	item2ID := createTestID("002")
	item3ID := createTestID("003")
	parentID := createTestID("PAR")

	tests := []struct {
		name      string
		items     []movable.LinkedListPointers
		wantError bool
	}{
		{
			name:      "empty list",
			items:     []movable.LinkedListPointers{},
			wantError: false,
		},
		{
			name: "valid single item",
			items: []movable.LinkedListPointers{
				{ItemID: item1ID, ParentID: parentID, Position: 0, Prev: nil, Next: nil},
			},
			wantError: false,
		},
		{
			name: "valid three-item list",
			items: []movable.LinkedListPointers{
				{ItemID: item1ID, ParentID: parentID, Position: 0, Prev: nil, Next: &item2ID},
				{ItemID: item2ID, ParentID: parentID, Position: 1, Prev: &item1ID, Next: &item3ID},
				{ItemID: item3ID, ParentID: parentID, Position: 2, Prev: &item2ID, Next: nil},
			},
			wantError: false,
		},
		{
			name: "multiple heads",
			items: []movable.LinkedListPointers{
				{ItemID: item1ID, ParentID: parentID, Position: 0, Prev: nil, Next: &item2ID},
				{ItemID: item2ID, ParentID: parentID, Position: 1, Prev: nil, Next: &item3ID}, // Invalid: should have prev
				{ItemID: item3ID, ParentID: parentID, Position: 2, Prev: &item2ID, Next: nil},
			},
			wantError: true,
		},
		{
			name: "broken forward link",
			items: []movable.LinkedListPointers{
				{ItemID: item1ID, ParentID: parentID, Position: 0, Prev: nil, Next: &item3ID}, // Points to item3 instead of item2
				{ItemID: item2ID, ParentID: parentID, Position: 1, Prev: &item1ID, Next: &item3ID},
				{ItemID: item3ID, ParentID: parentID, Position: 2, Prev: &item2ID, Next: nil},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := utils.ValidateListIntegrity(ctx, tt.items)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLinkedListUtils_OptimizePointerUpdates(t *testing.T) {
	utils := movable.NewLinkedListUtils()

	item1ID := createTestID("001")
	item2ID := createTestID("002")
	item3ID := createTestID("003")
	parentID := createTestID("PAR")

	// Original items with their current pointers
	originalItems := []movable.LinkedListPointers{
		{ItemID: item1ID, ParentID: parentID, Position: 0, Prev: nil, Next: &item2ID},
		{ItemID: item2ID, ParentID: parentID, Position: 1, Prev: &item1ID, Next: &item3ID},
		{ItemID: item3ID, ParentID: parentID, Position: 2, Prev: &item2ID, Next: nil},
	}

	// Calculations where only item2 actually changes
	calculations := []movable.PointerCalculation{
		{ItemID: item1ID, NewPrev: nil, NewNext: &item2ID, NewPos: 0},        // No change
		{ItemID: item2ID, NewPrev: &item3ID, NewNext: nil, NewPos: 2},        // Changed
		{ItemID: item3ID, NewPrev: &item2ID, NewNext: nil, NewPos: 2},        // No change
	}

	optimized := utils.OptimizePointerUpdates(calculations, originalItems)

	// Should only include item2 since it's the only one that changed
	if len(optimized) != 1 {
		t.Errorf("expected 1 optimized update, got %d", len(optimized))
		return
	}

	if optimized[0].ItemID != item2ID {
		t.Errorf("expected optimized update for item2, got %s", optimized[0].ItemID.String())
	}
}

// Benchmark tests for performance validation
func BenchmarkLinkedListUtils_ValidateMoveOperation(b *testing.B) {
	utils := movable.NewLinkedListUtils()
	ctx := context.Background()

	operation := movable.MoveOperation{
		ItemID:   createTestID("ABC"),
		TargetID: &[]idwrap.IDWrap{createTestID("DEF")}[0],
		Position: movable.MovePositionAfter,
		ListType: movable.CollectionListTypeItems,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = utils.ValidateMoveOperation(ctx, operation)
	}
}

func BenchmarkLinkedListUtils_CalculatePointerUpdatesAfter(b *testing.B) {
	utils := movable.NewLinkedListUtils()

	// Create a list with 50 items for realistic performance testing
	allItems := make([]movable.LinkedListPointers, 50)
	parentID := createTestID("PAR")
	
	for i := 0; i < 50; i++ {
		allItems[i] = movable.LinkedListPointers{
			ItemID:   createTestID(fmt.Sprintf("%03d", i)),
			ParentID: parentID,
			Position: i,
		}
	}

	moveCtx := &movable.MoveContext{
		ItemID:      allItems[0].ItemID,
		TargetID:    allItems[25].ItemID,
		ParentID:    parentID,
		AllItems:    allItems,
		ItemIndex:   0,
		TargetIndex: 25,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = utils.CalculatePointerUpdatesAfter(moveCtx)
	}
}

func BenchmarkLinkedListUtils_ValidateListIntegrity(b *testing.B) {
	utils := movable.NewLinkedListUtils()
	ctx := context.Background()

	// Create a valid linked list with 100 items
	items := make([]movable.LinkedListPointers, 100)
	parentID := createTestID("PAR")
	
	for i := 0; i < 100; i++ {
		itemID := createTestID(fmt.Sprintf("%03d", i))
		var prev, next *idwrap.IDWrap
		
		if i > 0 {
			prevID := createTestID(fmt.Sprintf("%03d", i-1))
			prev = &prevID
		}
		if i < 99 {
			nextID := createTestID(fmt.Sprintf("%03d", i+1))
			next = &nextID
		}
		
		items[i] = movable.LinkedListPointers{
			ItemID:   itemID,
			ParentID: parentID,
			Position: i,
			Prev:     prev,
			Next:     next,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = utils.ValidateListIntegrity(ctx, items)
	}
}

// Helper function for fmt.Sprintf in benchmarks - already imported above