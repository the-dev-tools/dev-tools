package movable

import (
	"fmt"
	"reflect"
	"testing"
)

// =============================================================================
// TEST HELPERS AND FIXTURES
// =============================================================================

// createPureFunctionTestItems creates a slice of test items with sequential positions
func createPureFunctionTestItems(count int) []PureOrderable {
	items := make([]PureOrderable, count)
	for i := 0; i < count; i++ {
		// Generate IDs that work for large counts
		var id string
		if i < 26 {
			id = string(rune('A' + i))
		} else {
			// For counts > 26, use pattern like A1, B1, C1, etc.
			id = string(rune('A' + (i % 26))) + fmt.Sprintf("%d", i/26)
		}
		
		items[i] = PureOrderable{
			ID:       PureID(id),
			Position: PurePosition(i),
			ParentID: PureID("parent1"),
		}
	}
	return UpdatePrevNextPointers(items)
}

// createPureFunctionTestItemsWithGaps creates items with position gaps for testing
// Note: We don't call UpdatePrevNextPointers here to preserve the gaps
func createPureFunctionTestItemsWithGaps() []PureOrderable {
	items := []PureOrderable{
		{ID: PureID("A"), Position: 0, ParentID: PureID("parent1")},
		{ID: PureID("B"), Position: 2, ParentID: PureID("parent1")}, // Gap at position 1
		{ID: PureID("C"), Position: 5, ParentID: PureID("parent1")}, // Gap at positions 3,4
		{ID: PureID("D"), Position: 6, ParentID: PureID("parent1")},
		{ID: PureID("E"), Position: 10, ParentID: PureID("parent1")}, // Gap at positions 7,8,9
	}
	
	// Manually set up prev/next pointers to maintain the gaps
	// A -> B
	items[0].Next = &items[1].ID
	items[1].Prev = &items[0].ID
	
	// B -> C
	items[1].Next = &items[2].ID
	items[2].Prev = &items[1].ID
	
	// C -> D
	items[2].Next = &items[3].ID
	items[3].Prev = &items[2].ID
	
	// D -> E
	items[3].Next = &items[4].ID
	items[4].Prev = &items[3].ID
	
	return items
}

// createBrokenLinkedList creates a list with broken links for validation testing
func createBrokenLinkedList() []PureOrderable {
	itemA := PureOrderable{ID: PureID("A"), Position: 0, ParentID: PureID("parent1")}
	itemB := PureOrderable{ID: PureID("B"), Position: 1, ParentID: PureID("parent1")}
	itemC := PureOrderable{ID: PureID("C"), Position: 2, ParentID: PureID("parent1")}
	
	// Create broken links - A points to B, but B doesn't point back to A
	itemA.Next = &itemB.ID
	itemB.Prev = &itemC.ID // Wrong! Should be &itemA.ID
	itemB.Next = &itemC.ID
	itemC.Prev = &itemB.ID
	
	return []PureOrderable{itemA, itemB, itemC}
}

// assertItemsEqual compares two slices of PureOrderable items
func assertItemsEqual(t *testing.T, expected, actual []PureOrderable) {
	t.Helper()
	
	if len(expected) != len(actual) {
		t.Fatalf("Length mismatch: expected %d, got %d", len(expected), len(actual))
	}
	
	for i, exp := range expected {
		act := actual[i]
		if exp.ID != act.ID {
			t.Errorf("Item %d ID mismatch: expected %s, got %s", i, exp.ID, act.ID)
		}
		if exp.Position != act.Position {
			t.Errorf("Item %d Position mismatch: expected %d, got %d", i, exp.Position, act.Position)
		}
	}
}

// =============================================================================
// ENHANCED POSITION CALCULATION TESTS - COMPREHENSIVE COVERAGE
// =============================================================================

func TestCalculatePositions(t *testing.T) {
	tests := []struct {
		name     string
		items    []PureOrderable
		expected map[string]*PurePosition
	}{
		{
			name:     "empty list",
			items:    []PureOrderable{},
			expected: map[string]*PurePosition{},
		},
		{
			name:  "single item",
			items: createPureFunctionTestItems(1),
			expected: map[string]*PurePosition{
				"A": func() *PurePosition { p := PurePosition(0); return &p }(),
			},
		},
		{
			name:  "sequential items",
			items: createPureFunctionTestItems(3),
			expected: map[string]*PurePosition{
				"A": func() *PurePosition { p := PurePosition(0); return &p }(),
				"B": func() *PurePosition { p := PurePosition(1); return &p }(),
				"C": func() *PurePosition { p := PurePosition(2); return &p }(),
			},
		},
		{
			name: "broken head (no item with prev=nil)",
			items: []PureOrderable{
				{ID: PureID("A"), Position: 0, Prev: func() *PureID { p := PureID("C"); return &p }()},
				{ID: PureID("B"), Position: 1, Prev: func() *PureID { p := PureID("A"); return &p }()},
			},
			expected: map[string]*PurePosition{
				"A": func() *PurePosition { p := PurePosition(0); return &p }(),
			},
		},
		{
			name: "medium list (10 items)",
			items: createPureFunctionTestItems(10),
			expected: func() map[string]*PurePosition {
				result := make(map[string]*PurePosition)
				for i := 0; i < 10; i++ {
					id := string(rune('A' + i))
					p := PurePosition(i)
					result[id] = &p
				}
				return result
			}(),
		},
		{
			name: "circular reference detection",
			items: []PureOrderable{
				{ID: PureID("A"), Position: 0, Prev: nil, Next: func() *PureID { p := PureID("B"); return &p }()},
				{ID: PureID("B"), Position: 1, Prev: func() *PureID { p := PureID("A"); return &p }(), Next: func() *PureID { p := PureID("A"); return &p }()}, // Circular!
			},
			expected: map[string]*PurePosition{
				"A": func() *PurePosition { p := PurePosition(0); return &p }(),
				"B": func() *PurePosition { p := PurePosition(1); return &p }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculatePositions(tt.items)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Result length mismatch: expected %d, got %d", len(tt.expected), len(result))
				return
			}
			
			for key, expectedPos := range tt.expected {
				actualPos, exists := result[key]
				if !exists {
					t.Errorf("Missing key %s in result", key)
					continue
				}
				if expectedPos == nil && actualPos != nil {
					t.Errorf("Expected nil for key %s, got %v", key, *actualPos)
					continue
				}
				if expectedPos != nil && actualPos == nil {
					t.Errorf("Expected %v for key %s, got nil", *expectedPos, key)
					continue
				}
				if expectedPos != nil && actualPos != nil && *expectedPos != *actualPos {
					t.Errorf("Position mismatch for key %s: expected %d, got %d", key, *expectedPos, *actualPos)
				}
			}
		})
	}
}

// =============================================================================
// ITEM INSERTION TESTS
// =============================================================================

func TestInsertItem(t *testing.T) {
	tests := []struct {
		name          string
		items         []PureOrderable
		newItem       PureOrderable
		afterID       string
		expectedCount int
		expectedOrder []string
	}{
		{
			name:    "insert into empty list",
			items:   []PureOrderable{},
			newItem: PureOrderable{ID: PureID("A"), ParentID: PureID("parent1")},
			afterID: "",
			expectedCount: 1,
			expectedOrder: []string{"A"},
		},
		{
			name:    "insert after first item",
			items:   createPureFunctionTestItems(3),
			newItem: PureOrderable{ID: PureID("X"), ParentID: PureID("parent1")},
			afterID: "A",
			expectedCount: 4,
			expectedOrder: []string{"A", "X", "B", "C"},
		},
		{
			name:    "insert at end (after last item)",
			items:   createPureFunctionTestItems(3),
			newItem: PureOrderable{ID: PureID("X"), ParentID: PureID("parent1")},
			afterID: "C",
			expectedCount: 4,
			expectedOrder: []string{"A", "B", "C", "X"},
		},
		{
			name:    "insert with non-existent afterID (should append)",
			items:   createPureFunctionTestItems(3),
			newItem: PureOrderable{ID: PureID("X"), ParentID: PureID("parent1")},
			afterID: "Z",
			expectedCount: 4,
			expectedOrder: []string{"A", "B", "C", "X"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InsertItem(tt.items, tt.newItem, tt.afterID)
			
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d items, got %d", tt.expectedCount, len(result))
				return
			}
			
			// Verify order
			for i, expectedID := range tt.expectedOrder {
				if i >= len(result) {
					t.Errorf("Missing item at position %d", i)
					continue
				}
				if string(result[i].ID) != expectedID {
					t.Errorf("Position %d: expected ID %s, got %s", i, expectedID, result[i].ID)
				}
			}
			
			// Verify prev/next pointers are consistent
			if err := ValidateOrdering(result); err != nil {
				t.Errorf("Ordering validation failed: %v", err)
			}
		})
	}
}

// =============================================================================
// ITEM MOVEMENT TESTS
// =============================================================================

func TestMoveItem(t *testing.T) {
	tests := []struct {
		name          string
		items         []PureOrderable
		itemID        string
		newPosition   *PurePosition
		expectedOrder []string
		expectError   bool
	}{
		{
			name:        "move to beginning",
			items:       createPureFunctionTestItems(3),
			itemID:      "C",
			newPosition: func() *PurePosition { p := PurePosition(0); return &p }(),
			expectedOrder: []string{"C", "A", "B"},
		},
		{
			name:        "move to middle",
			items:       createPureFunctionTestItems(4),
			itemID:      "D",
			newPosition: func() *PurePosition { p := PurePosition(1); return &p }(),
			expectedOrder: []string{"A", "D", "B", "C"},
		},
		{
			name:        "move to end",
			items:       createPureFunctionTestItems(3),
			itemID:      "A",
			newPosition: func() *PurePosition { p := PurePosition(2); return &p }(),
			expectedOrder: []string{"B", "C", "A"},
		},
		{
			name:        "move non-existent item",
			items:       createPureFunctionTestItems(3),
			itemID:      "Z",
			newPosition: func() *PurePosition { p := PurePosition(0); return &p }(),
			expectError: true,
		},
		{
			name:        "move in empty list",
			items:       []PureOrderable{},
			itemID:      "A",
			newPosition: func() *PurePosition { p := PurePosition(0); return &p }(),
			expectError: true,
		},
		{
			name:        "move with nil position (should default to 0)",
			items:       createPureFunctionTestItems(3),
			itemID:      "C",
			newPosition: nil,
			expectedOrder: []string{"C", "A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MoveItem(tt.items, tt.itemID, tt.newPosition)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if len(result) != len(tt.expectedOrder) {
				t.Errorf("Expected %d items, got %d", len(tt.expectedOrder), len(result))
				return
			}
			
			// Verify order
			for i, expectedID := range tt.expectedOrder {
				if string(result[i].ID) != expectedID {
					t.Errorf("Position %d: expected ID %s, got %s", i, expectedID, result[i].ID)
				}
			}
			
			// Verify ordering is valid
			if err := ValidateOrdering(result); err != nil {
				t.Errorf("Ordering validation failed: %v", err)
			}
		})
	}
}

func TestMoveItemAfter(t *testing.T) {
	tests := []struct {
		name          string
		items         []PureOrderable
		itemID        PureID
		targetID      PureID
		expectedOrder []string
		expectError   bool
	}{
		{
			name:          "move A after B",
			items:         createPureFunctionTestItems(3),
			itemID:        PureID("A"),
			targetID:      PureID("B"),
			expectedOrder: []string{"B", "A", "C"},
		},
		{
			name:          "move C after A",
			items:         createPureFunctionTestItems(3),
			itemID:        PureID("C"),
			targetID:      PureID("A"),
			expectedOrder: []string{"A", "C", "B"},
		},
		{
			name:        "move after non-existent target",
			items:       createPureFunctionTestItems(3),
			itemID:      PureID("A"),
			targetID:    PureID("Z"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MoveItemAfter(tt.items, tt.itemID, tt.targetID)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Verify order
			for i, expectedID := range tt.expectedOrder {
				if string(result[i].ID) != expectedID {
					t.Errorf("Position %d: expected ID %s, got %s", i, expectedID, result[i].ID)
				}
			}
		})
	}
}

func TestMoveItemBefore(t *testing.T) {
	tests := []struct {
		name          string
		items         []PureOrderable
		itemID        PureID
		targetID      PureID
		expectedOrder []string
		expectError   bool
	}{
		{
			name:          "move C before B",
			items:         createPureFunctionTestItems(3),
			itemID:        PureID("C"),
			targetID:      PureID("B"),
			expectedOrder: []string{"A", "C", "B"},
		},
		{
			name:          "move A before C",
			items:         createPureFunctionTestItems(3),
			itemID:        PureID("A"),
			targetID:      PureID("C"),
			expectedOrder: []string{"B", "A", "C"},
		},
		{
			name:        "move before non-existent target",
			items:       createPureFunctionTestItems(3),
			itemID:      PureID("A"),
			targetID:    PureID("Z"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MoveItemBefore(tt.items, tt.itemID, tt.targetID)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Verify order
			for i, expectedID := range tt.expectedOrder {
				if string(result[i].ID) != expectedID {
					t.Errorf("Position %d: expected ID %s, got %s", i, expectedID, result[i].ID)
				}
			}
		})
	}
}

// =============================================================================
// VALIDATION TESTS
// =============================================================================

func TestValidateOrdering(t *testing.T) {
	tests := []struct {
		name        string
		items       []PureOrderable
		expectError bool
		errorContains string
	}{
		{
			name:        "empty list is valid",
			items:       []PureOrderable{},
			expectError: false,
		},
		{
			name:        "single item is valid",
			items:       createPureFunctionTestItems(1),
			expectError: false,
		},
		{
			name:        "valid sequential items",
			items:       createPureFunctionTestItems(3),
			expectError: false,
		},
		{
			name: "no head item (all have prev)",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: func() *PureID { p := PureID("B"); return &p }()},
				{ID: PureID("B"), Prev: func() *PureID { p := PureID("C"); return &p }()},
			},
			expectError: true,
			errorContains: "expected exactly 1 head item",
		},
		{
			name: "multiple head items",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: nil, Next: func() *PureID { p := PureID("B"); return &p }()},
				{ID: PureID("B"), Prev: nil, Next: nil}, // Second head
			},
			expectError: true,
			errorContains: "expected exactly 1 head item",
		},
		{
			name: "broken forward link",
			items: createBrokenLinkedList(),
			expectError: true,
			errorContains: "broken backward link",
		},
		{
			name: "non-existent next reference",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: nil, Next: func() *PureID { p := PureID("Z"); return &p }()},
			},
			expectError: true,
			errorContains: "non-existent next item",
		},
		{
			name: "circular reference",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: nil, Next: func() *PureID { p := PureID("A"); return &p }()},
			},
			expectError: true,
			errorContains: "circular reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOrdering(tt.items)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr || 
		      containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// GAP DETECTION TESTS
// =============================================================================

func TestFindGaps(t *testing.T) {
	tests := []struct {
		name         string
		items        []PureOrderable
		expectedGaps []*PureGap
	}{
		{
			name:         "no gaps in sequential items",
			items:        createPureFunctionTestItems(3),
			expectedGaps: nil,
		},
		{
			name:         "empty list has no gaps",
			items:        []PureOrderable{},
			expectedGaps: nil,
		},
		{
			name:         "single item has no gaps",
			items:        createPureFunctionTestItems(1),
			expectedGaps: nil,
		},
		{
			name:  "items with gaps",
			items: createPureFunctionTestItemsWithGaps(),
			expectedGaps: []*PureGap{
				{StartPosition: 1, EndPosition: 1, Size: 1},
				{StartPosition: 3, EndPosition: 4, Size: 2},
				{StartPosition: 7, EndPosition: 9, Size: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindGaps(tt.items)
			
			if len(result) != len(tt.expectedGaps) {
				t.Errorf("Expected %d gaps, got %d", len(tt.expectedGaps), len(result))
				return
			}
			
			for i, expected := range tt.expectedGaps {
				if i >= len(result) {
					t.Errorf("Missing gap at index %d", i)
					continue
				}
				actual := result[i]
				if actual.StartPosition != expected.StartPosition {
					t.Errorf("Gap %d StartPosition: expected %d, got %d", i, expected.StartPosition, actual.StartPosition)
				}
				if actual.EndPosition != expected.EndPosition {
					t.Errorf("Gap %d EndPosition: expected %d, got %d", i, expected.EndPosition, actual.EndPosition)
				}
				if actual.Size != expected.Size {
					t.Errorf("Gap %d Size: expected %d, got %d", i, expected.Size, actual.Size)
				}
			}
		})
	}
}

func TestFindLargestGap(t *testing.T) {
	tests := []struct {
		name        string
		items       []PureOrderable
		expectedGap *PureGap
		expectError bool
	}{
		{
			name:        "no gaps returns error",
			items:       createPureFunctionTestItems(3),
			expectedGap: nil,
			expectError: true,
		},
		{
			name:  "find largest gap",
			items: createPureFunctionTestItemsWithGaps(),
			expectedGap: &PureGap{
				StartPosition: 7,
				EndPosition:   9,
				Size:          3,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindLargestGap(tt.items)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if !reflect.DeepEqual(result, tt.expectedGap) {
				t.Errorf("Expected gap %+v, got %+v", tt.expectedGap, result)
			}
		})
	}
}

func TestCalculateGapMetrics(t *testing.T) {
	tests := []struct {
		name            string
		items           []PureOrderable
		expectedMetrics PureGapMetrics
	}{
		{
			name:  "no gaps",
			items: createPureFunctionTestItems(3),
			expectedMetrics: PureGapMetrics{
				TotalGaps:    0,
				LargestGap:   nil,
				AverageSize:  0,
				TotalMissing: 0,
			},
		},
		{
			name:  "items with gaps",
			items: createPureFunctionTestItemsWithGaps(),
			expectedMetrics: PureGapMetrics{
				TotalGaps:    3,
				LargestGap:   &PureGap{StartPosition: 7, EndPosition: 9, Size: 3},
				AverageSize:  2.0, // (1 + 2 + 3) / 3 = 2.0
				TotalMissing: 6,   // 1 + 2 + 3 = 6
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateGapMetrics(tt.items)
			
			if result.TotalGaps != tt.expectedMetrics.TotalGaps {
				t.Errorf("TotalGaps: expected %d, got %d", tt.expectedMetrics.TotalGaps, result.TotalGaps)
			}
			
			if result.AverageSize != tt.expectedMetrics.AverageSize {
				t.Errorf("AverageSize: expected %.2f, got %.2f", tt.expectedMetrics.AverageSize, result.AverageSize)
			}
			
			if result.TotalMissing != tt.expectedMetrics.TotalMissing {
				t.Errorf("TotalMissing: expected %d, got %d", tt.expectedMetrics.TotalMissing, result.TotalMissing)
			}
			
			if tt.expectedMetrics.LargestGap == nil && result.LargestGap != nil {
				t.Errorf("Expected LargestGap to be nil, got %+v", result.LargestGap)
			} else if tt.expectedMetrics.LargestGap != nil && result.LargestGap == nil {
				t.Errorf("Expected LargestGap %+v, got nil", tt.expectedMetrics.LargestGap)
			} else if tt.expectedMetrics.LargestGap != nil && result.LargestGap != nil {
				if !reflect.DeepEqual(result.LargestGap, tt.expectedMetrics.LargestGap) {
					t.Errorf("LargestGap: expected %+v, got %+v", tt.expectedMetrics.LargestGap, result.LargestGap)
				}
			}
		})
	}
}

// =============================================================================
// REBALANCING TESTS
// =============================================================================

func TestRebalancePositions(t *testing.T) {
	tests := []struct {
		name               string
		items              []PureOrderable
		expectedPositions  []PurePosition
		expectedOrder      []string
	}{
		{
			name:              "empty list",
			items:             []PureOrderable{},
			expectedPositions: []PurePosition{},
			expectedOrder:     []string{},
		},
		{
			name:              "already balanced",
			items:             createPureFunctionTestItems(3),
			expectedPositions: []PurePosition{0, 1, 2},
			expectedOrder:     []string{"A", "B", "C"},
		},
		{
			name:              "rebalance items with gaps",
			items:             createPureFunctionTestItemsWithGaps(),
			expectedPositions: []PurePosition{0, 1, 2, 3, 4},
			expectedOrder:     []string{"A", "B", "C", "D", "E"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RebalancePositions(tt.items)
			
			if len(result) != len(tt.expectedPositions) {
				t.Errorf("Expected %d items, got %d", len(tt.expectedPositions), len(result))
				return
			}
			
			// Check positions and order
			for i, expected := range tt.expectedPositions {
				if result[i].Position != expected {
					t.Errorf("Position %d: expected %d, got %d", i, expected, result[i].Position)
				}
				if len(tt.expectedOrder) > i && string(result[i].ID) != tt.expectedOrder[i] {
					t.Errorf("Order %d: expected ID %s, got %s", i, tt.expectedOrder[i], result[i].ID)
				}
			}
			
			// Verify ordering is valid
			if err := ValidateOrdering(result); err != nil {
				t.Errorf("Ordering validation failed: %v", err)
			}
		})
	}
}

func TestRebalanceWithSpacing(t *testing.T) {
	tests := []struct {
		name              string
		items             []PureOrderable
		spacing           int
		expectedPositions []PurePosition
	}{
		{
			name:              "spacing of 2",
			items:             createPureFunctionTestItems(3),
			spacing:           2,
			expectedPositions: []PurePosition{0, 2, 4},
		},
		{
			name:              "spacing of 10",
			items:             createPureFunctionTestItems(3),
			spacing:           10,
			expectedPositions: []PurePosition{0, 10, 20},
		},
		{
			name:              "invalid spacing (< 1) defaults to rebalance",
			items:             createPureFunctionTestItemsWithGaps(),
			spacing:           0,
			expectedPositions: []PurePosition{0, 1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RebalanceWithSpacing(tt.items, tt.spacing)
			
			if len(result) != len(tt.expectedPositions) {
				t.Errorf("Expected %d items, got %d", len(tt.expectedPositions), len(result))
				return
			}
			
			for i, expected := range tt.expectedPositions {
				if result[i].Position != expected {
					t.Errorf("Position %d: expected %d, got %d", i, expected, result[i].Position)
				}
			}
		})
	}
}

func TestSelectiveRebalance(t *testing.T) {
	tests := []struct {
		name                string
		items               []PureOrderable
		gapThreshold        int
		expectedRebalanced  bool
		expectedGapsEliminated int
	}{
		{
			name:                "no large gaps, no rebalancing",
			items:               createPureFunctionTestItems(3),
			gapThreshold:        5,
			expectedRebalanced:  false,
			expectedGapsEliminated: 0,
		},
		{
			name:                "large gaps trigger rebalancing",
			items:               createPureFunctionTestItemsWithGaps(),
			gapThreshold:        2,
			expectedRebalanced:  true,
			expectedGapsEliminated: 2, // Gaps of size 2 and 3 (size 1 gap doesn't meet threshold)
		},
		{
			name:                "high threshold prevents rebalancing",
			items:               createPureFunctionTestItemsWithGaps(),
			gapThreshold:        10,
			expectedRebalanced:  false,
			expectedGapsEliminated: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SelectiveRebalance(tt.items, tt.gapThreshold)
			
			if result.GapsEliminated != tt.expectedGapsEliminated {
				t.Errorf("GapsEliminated: expected %d, got %d", tt.expectedGapsEliminated, result.GapsEliminated)
			}
			
			rebalanced := len(result.PositionChanges) > 0
			if rebalanced != tt.expectedRebalanced {
				t.Errorf("Expected rebalanced=%v, got %v", tt.expectedRebalanced, rebalanced)
			}
			
			// Verify ordering is valid
			if err := ValidateOrdering(result.UpdatedItems); err != nil {
				t.Errorf("Ordering validation failed: %v", err)
			}
		})
	}
}

// =============================================================================
// BATCH OPERATION TESTS
// =============================================================================

func TestBatchInsert(t *testing.T) {
	tests := []struct {
		name          string
		items         []PureOrderable
		newItems      []PureOrderable
		positions     []PurePosition
		expectedCount int
		expectError   bool
	}{
		{
			name:  "insert multiple items",
			items: createPureFunctionTestItems(2),
			newItems: []PureOrderable{
				{ID: PureID("X"), ParentID: PureID("parent1")},
				{ID: PureID("Y"), ParentID: PureID("parent1")},
			},
			positions:     []PurePosition{1, 3},
			expectedCount: 4,
			expectError:   false,
		},
		{
			name:  "mismatched slices length",
			items: createPureFunctionTestItems(2),
			newItems: []PureOrderable{
				{ID: PureID("X"), ParentID: PureID("parent1")},
			},
			positions:   []PurePosition{1, 2}, // Different length
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BatchInsert(tt.items, tt.newItems, tt.positions)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d items, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

func TestBatchMove(t *testing.T) {
	tests := []struct {
		name        string
		items       []PureOrderable
		moves       []PureMoveOperation
		expectError bool
	}{
		{
			name:  "batch move operations",
			items: createPureFunctionTestItems(4),
			moves: []PureMoveOperation{
				{ItemID: PureID("A"), NewPosition: PurePosition(2)},
				{ItemID: PureID("C"), NewPosition: PurePosition(0)},
			},
			expectError: false,
		},
		{
			name:  "move non-existent item",
			items: createPureFunctionTestItems(3),
			moves: []PureMoveOperation{
				{ItemID: PureID("Z"), NewPosition: PurePosition(0)},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BatchMove(tt.items, tt.moves)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if len(result) != len(tt.items) {
				t.Errorf("Expected %d items, got %d", len(tt.items), len(result))
			}
		})
	}
}

func TestBatchReorder(t *testing.T) {
	tests := []struct {
		name          string
		items         []PureOrderable
		newOrder      []PureID
		expectedOrder []string
		expectError   bool
	}{
		{
			name:          "reorder all items",
			items:         createPureFunctionTestItems(3),
			newOrder:      []PureID{PureID("C"), PureID("A"), PureID("B")},
			expectedOrder: []string{"C", "A", "B"},
			expectError:   false,
		},
		{
			name:        "missing item in new order",
			items:       createPureFunctionTestItems(3),
			newOrder:    []PureID{PureID("A"), PureID("B")}, // Missing C
			expectError: true,
		},
		{
			name:        "non-existent item in new order",
			items:       createPureFunctionTestItems(3),
			newOrder:    []PureID{PureID("A"), PureID("B"), PureID("Z")},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BatchReorder(tt.items, tt.newOrder)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Verify order
			for i, expectedID := range tt.expectedOrder {
				if string(result[i].ID) != expectedID {
					t.Errorf("Position %d: expected ID %s, got %s", i, expectedID, result[i].ID)
				}
			}
			
			// Verify positions are sequential
			for i, item := range result {
				if item.Position != PurePosition(i) {
					t.Errorf("Position %d: expected position %d, got %d", i, i, item.Position)
				}
			}
		})
	}
}

// =============================================================================
// UTILITY FUNCTION TESTS
// =============================================================================

func TestFindItemByID(t *testing.T) {
	items := createPureFunctionTestItems(3)
	
	tests := []struct {
		name          string
		id            PureID
		expectedFound bool
		expectedIndex int
	}{
		{
			name:          "find existing item",
			id:            PureID("B"),
			expectedFound: true,
			expectedIndex: 1,
		},
		{
			name:          "find non-existent item",
			id:            PureID("Z"),
			expectedFound: false,
			expectedIndex: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, index, found := FindItemByID(items, tt.id)
			
			if found != tt.expectedFound {
				t.Errorf("Expected found=%v, got %v", tt.expectedFound, found)
			}
			
			if index != tt.expectedIndex {
				t.Errorf("Expected index=%d, got %d", tt.expectedIndex, index)
			}
			
			if tt.expectedFound {
				if item == nil {
					t.Error("Expected non-nil item")
				} else if item.ID != tt.id {
					t.Errorf("Expected ID %s, got %s", tt.id, item.ID)
				}
			} else if item != nil {
				t.Error("Expected nil item for not found")
			}
		})
	}
}

func TestGetOrderedItemIDs(t *testing.T) {
	tests := []struct {
		name        string
		items       []PureOrderable
		expectedIDs []PureID
	}{
		{
			name:        "empty list",
			items:       []PureOrderable{},
			expectedIDs: []PureID{},
		},
		{
			name:        "sequential items",
			items:       createPureFunctionTestItems(3),
			expectedIDs: []PureID{PureID("A"), PureID("B"), PureID("C")},
		},
		{
			name: "items with gaps (should sort by position)",
			items: []PureOrderable{
				{ID: PureID("C"), Position: 10, ParentID: PureID("parent1")},
				{ID: PureID("A"), Position: 0, ParentID: PureID("parent1")},
				{ID: PureID("B"), Position: 5, ParentID: PureID("parent1")},
			},
			expectedIDs: []PureID{PureID("A"), PureID("B"), PureID("C")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetOrderedItemIDs(tt.items)
			
			if len(result) != len(tt.expectedIDs) {
				t.Errorf("Expected %d IDs, got %d", len(tt.expectedIDs), len(result))
				return
			}
			
			for i, expected := range tt.expectedIDs {
				if result[i] != expected {
					t.Errorf("Position %d: expected ID %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestOrderableFromLinkedList(t *testing.T) {
	tests := []struct {
		name          string
		items         []PureOrderable
		expectError   bool
		errorContains string
	}{
		{
			name:        "empty list",
			items:       []PureOrderable{},
			expectError: false,
		},
		{
			name:        "valid linked list",
			items:       createPureFunctionTestItems(3),
			expectError: false,
		},
		{
			name: "no head item",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: func() *PureID { p := PureID("B"); return &p }()},
			},
			expectError:   true,
			errorContains: "no head item found",
		},
		{
			name: "circular reference",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: nil, Next: func() *PureID { p := PureID("A"); return &p }()},
			},
			expectError:   true,
			errorContains: "circular reference",
		},
		{
			name: "broken next reference",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: nil, Next: func() *PureID { p := PureID("Z"); return &p }()},
			},
			expectError:   true,
			errorContains: "non-existent next item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := OrderableFromLinkedList(tt.items)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if len(tt.items) == 0 && result != nil {
				t.Error("Expected nil result for empty input")
				return
			}
			
			if len(tt.items) > 0 {
				// Verify positions are sequential
				for i, item := range result {
					if item.Position != PurePosition(i) {
						t.Errorf("Position %d: expected %d, got %d", i, i, item.Position)
					}
				}
			}
		})
	}
}

func TestLinkedListFromOrderable(t *testing.T) {
	tests := []struct {
		name  string
		items []PureOrderable
	}{
		{
			name:  "empty list",
			items: []PureOrderable{},
		},
		{
			name:  "single item",
			items: createPureFunctionTestItems(1),
		},
		{
			name:  "multiple items",
			items: createPureFunctionTestItems(3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LinkedListFromOrderable(tt.items)
			
			if len(tt.items) == 0 {
				if result != nil {
					t.Error("Expected nil result for empty input")
				}
				return
			}
			
			// Verify ordering is valid
			if err := ValidateOrdering(result); err != nil {
				t.Errorf("Ordering validation failed: %v", err)
			}
		})
	}
}

func TestGroupByParent(t *testing.T) {
	items := []PureOrderable{
		{ID: PureID("A"), ParentID: PureID("parent1")},
		{ID: PureID("B"), ParentID: PureID("parent1")},
		{ID: PureID("C"), ParentID: PureID("parent2")},
		{ID: PureID("D"), ParentID: PureID("parent2")},
		{ID: PureID("E"), ParentID: PureID("parent1")},
	}
	
	result := GroupByParent(items)
	
	// Check that we have 2 groups
	if len(result) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(result))
		return
	}
	
	// Check parent1 group
	parent1Scope := PureParentScope{ID: PureID("parent1"), Type: "default"}
	parent1Items, exists := result[parent1Scope]
	if !exists {
		t.Error("Expected parent1 group to exist")
		return
	}
	if len(parent1Items) != 3 {
		t.Errorf("Expected 3 items in parent1 group, got %d", len(parent1Items))
	}
	
	// Check parent2 group
	parent2Scope := PureParentScope{ID: PureID("parent2"), Type: "default"}
	parent2Items, exists := result[parent2Scope]
	if !exists {
		t.Error("Expected parent2 group to exist")
		return
	}
	if len(parent2Items) != 2 {
		t.Errorf("Expected 2 items in parent2 group, got %d", len(parent2Items))
	}
}

func TestCalculateSequentialPositions(t *testing.T) {
	items := createPureFunctionTestItems(3)
	result := CalculateSequentialPositions(items)
	
	expectedPositions := map[PureID]PurePosition{
		PureID("A"): PurePosition(0),
		PureID("B"): PurePosition(1),
		PureID("C"): PurePosition(2),
	}
	
	if len(result) != len(expectedPositions) {
		t.Errorf("Expected %d positions, got %d", len(expectedPositions), len(result))
		return
	}
	
	for id, expected := range expectedPositions {
		actual, exists := result[id]
		if !exists {
			t.Errorf("Missing position for ID %s", id)
			continue
		}
		if actual != expected {
			t.Errorf("ID %s: expected position %d, got %d", id, expected, actual)
		}
	}
}

func TestCalculateSpacedPositions(t *testing.T) {
	items := createPureFunctionTestItems(3)
	spacing := 5
	result := CalculateSpacedPositions(items, spacing)
	
	expectedPositions := map[PureID]PurePosition{
		PureID("A"): PurePosition(0),
		PureID("B"): PurePosition(5),
		PureID("C"): PurePosition(10),
	}
	
	for id, expected := range expectedPositions {
		actual, exists := result[id]
		if !exists {
			t.Errorf("Missing position for ID %s", id)
			continue
		}
		if actual != expected {
			t.Errorf("ID %s: expected position %d, got %d", id, expected, actual)
		}
	}
}

// =============================================================================
// STRESS TESTS - LARGE DATASETS
// =============================================================================

func TestCalculatePositionsStress(t *testing.T) {
	// Test with 1000 items
	count := 1000
	items := createPureFunctionTestItems(count)
	
	result := CalculatePositions(items)
	
	if len(result) != count {
		t.Errorf("Expected %d positions, got %d", count, len(result))
		return
	}
	
	// Verify all positions are correct
	for i := 0; i < count; i++ {
		var expectedID string
		if i < 26 {
			expectedID = string(rune('A' + i))
		} else {
			expectedID = string(rune('A' + (i % 26))) + fmt.Sprintf("%d", i/26)
		}
		
		pos, exists := result[expectedID]
		if !exists {
			t.Errorf("Missing position for item %s", expectedID)
			continue
		}
		
		if pos == nil || *pos != PurePosition(i) {
			t.Errorf("Item %s: expected position %d, got %v", expectedID, i, pos)
		}
	}
}

func TestMoveItemStress(t *testing.T) {
	// Test moving multiple items in a large list
	count := 500
	items := createPureFunctionTestItems(count)
	
	// Move first item to last position
	firstID := string(items[0].ID)
	lastPos := PurePosition(count - 1)
	
	result, err := MoveItem(items, firstID, &lastPos)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	
	if len(result) != count {
		t.Errorf("Expected %d items, got %d", count, len(result))
		return
	}
	
	// Verify the first item is now at the end
	if string(result[count-1].ID) != firstID {
		t.Errorf("Expected moved item at position %d, got %s", count-1, result[count-1].ID)
	}
	
	// Verify ordering is still valid
	if err := ValidateOrdering(result); err != nil {
		t.Errorf("Ordering validation failed after stress move: %v", err)
	}
}

func TestBatchOperationsStress(t *testing.T) {
	items := createPureFunctionTestItems(100)
	
	// Create many batch move operations
	moves := make([]PureMoveOperation, 20)
	for i := 0; i < 20; i++ {
		itemIndex := i * 5 // Every 5th item
		if itemIndex < len(items) {
			moves[i] = PureMoveOperation{
				ItemID:      items[itemIndex].ID,
				NewPosition: PurePosition((i * 3) % len(items)), // Scattered positions
			}
		}
	}
	
	result, err := BatchMove(items, moves[:10]) // First 10 moves
	if err != nil {
		t.Errorf("Batch move failed: %v", err)
		return
	}
	
	if len(result) != len(items) {
		t.Errorf("Expected %d items after batch move, got %d", len(items), len(result))
	}
}

func TestRebalanceStress(t *testing.T) {
	// Create items with many gaps
	items := make([]PureOrderable, 50)
	for i := 0; i < 50; i++ {
		items[i] = PureOrderable{
			ID:       PureID(fmt.Sprintf("item_%d", i)),
			Position: PurePosition(i * 10), // Gaps of 9 between each item
			ParentID: PureID("parent1"),
		}
	}
	
	// Rebalance the positions
	rebalanced := RebalancePositions(items)
	
	if len(rebalanced) != 50 {
		t.Errorf("Expected 50 items after rebalance, got %d", len(rebalanced))
		return
	}
	
	// Verify positions are now sequential
	for i, item := range rebalanced {
		if item.Position != PurePosition(i) {
			t.Errorf("Item %d: expected position %d, got %d", i, i, item.Position)
		}
	}
	
	// Verify no gaps remain
	gaps := FindGaps(rebalanced)
	if len(gaps) != 0 {
		t.Errorf("Expected no gaps after rebalancing, found %d gaps", len(gaps))
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestEdgeCases(t *testing.T) {
	t.Run("move item to itself", func(t *testing.T) {
		items := createPureFunctionTestItems(3)
		currentPos := items[1].Position
		
		result, err := MoveItem(items, "B", &currentPos)
		if err != nil {
			t.Errorf("Moving item to its current position should succeed: %v", err)
			return
		}
		
		// Result should be identical to original
		if len(result) != len(items) {
			t.Errorf("Expected same number of items, got %d vs %d", len(result), len(items))
		}
	})
	
	t.Run("position overflow handling", func(t *testing.T) {
		items := createPureFunctionTestItems(3)
		overflowPos := PurePosition(999)
		
		result, err := MoveItem(items, "A", &overflowPos)
		if err != nil {
			t.Errorf("Move with overflow position failed: %v", err)
			return
		}
		
		// Should clamp to last position
		if string(result[len(result)-1].ID) != "A" {
			t.Error("Item should be moved to last position when overflow occurs")
		}
	})
	
	t.Run("negative position handling", func(t *testing.T) {
		items := createPureFunctionTestItems(3)
		negativePos := PurePosition(-5)
		
		result, err := MoveItem(items, "C", &negativePos)
		if err != nil {
			t.Errorf("Move with negative position failed: %v", err)
			return
		}
		
		// Should clamp to first position
		if string(result[0].ID) != "C" {
			t.Error("Item should be moved to first position when negative position given")
		}
	})
	
	t.Run("empty item ID validation", func(t *testing.T) {
		items := createPureFunctionTestItems(3)
		pos := PurePosition(1)
		
		_, err := MoveItem(items, "", &pos)
		if err == nil {
			t.Error("Expected error for empty item ID")
		}
	})
	
	t.Run("disconnected items handling", func(t *testing.T) {
		// Create items that aren't properly linked
		items := []PureOrderable{
			{ID: PureID("A"), Position: 0, Prev: nil, Next: nil}, // Disconnected
			{ID: PureID("B"), Position: 1, Prev: nil, Next: nil}, // Disconnected
		}
		
		err := ValidateOrdering(items)
		if err == nil {
			t.Error("Expected validation error for disconnected items")
		}
	})
}

func TestCalculatePositionFromPointers(t *testing.T) {
	tests := []struct {
		name             string
		items            []PureOrderable
		targetID         PureID
		expectedPosition PurePosition
		expectError      bool
		errorContains    string
	}{
		{
			name:             "find first item",
			items:            createPureFunctionTestItems(3),
			targetID:         PureID("A"),
			expectedPosition: PurePosition(0),
			expectError:      false,
		},
		{
			name:             "find middle item",
			items:            createPureFunctionTestItems(3),
			targetID:         PureID("B"),
			expectedPosition: PurePosition(1),
			expectError:      false,
		},
		{
			name:             "find last item",
			items:            createPureFunctionTestItems(3),
			targetID:         PureID("C"),
			expectedPosition: PurePosition(2),
			expectError:      false,
		},
		{
			name:          "item not found",
			items:         createPureFunctionTestItems(3),
			targetID:      PureID("Z"),
			expectError:   true,
			errorContains: "not found in linked list",
		},
		{
			name:          "empty list",
			items:         []PureOrderable{},
			targetID:      PureID("A"),
			expectError:   true,
			errorContains: "empty items list",
		},
		{
			name: "no head item",
			items: []PureOrderable{
				{ID: PureID("A"), Prev: func() *PureID { p := PureID("B"); return &p }()},
			},
			targetID:      PureID("A"),
			expectError:   true,
			errorContains: "no head item found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculatePositionFromPointers(tt.items, tt.targetID)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result != tt.expectedPosition {
				t.Errorf("Expected position %d, got %d", tt.expectedPosition, result)
			}
		})
	}
}