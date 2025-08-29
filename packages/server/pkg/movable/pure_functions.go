package movable

import (
	"errors"
	"fmt"
	"sort"
)

// =============================================================================
// CORE DATA STRUCTURES FROM DESIGN DOCUMENT  
// =============================================================================

// PureID represents a unique identifier for pure functions
type PureID string

// PurePosition represents an item's position in a list
type PurePosition int

// PureParentScope defines the context for ordering operations
type PureParentScope struct {
	ID   PureID
	Type string
}

// PureOrderable represents any item that can be ordered
type PureOrderable struct {
	ID       PureID
	Prev     *PureID
	Next     *PureID
	ParentID PureID
	Position PurePosition
}

// PureGap represents missing positions in a sequence
type PureGap struct {
	StartPosition PurePosition
	EndPosition   PurePosition
	Size          int
}

// PurePositionMap maps item IDs to their calculated positions
type PurePositionMap map[PureID]PurePosition

// PureMoveResult contains the outcome of a move operation
type PureMoveResult struct {
	UpdatedItems []PureOrderable
	AffectedIDs  []PureID
	NewPositions PurePositionMap
}

// PureValidationError contains validation failure details
type PureValidationError struct {
	ItemID  PureID
	Message string
	Code    string
}

// PureRebalanceResult contains position rebalancing outcome
type PureRebalanceResult struct {
	UpdatedItems    []PureOrderable
	PositionChanges map[PureID]PurePosition
	GapsEliminated  int
}

// PureMoveOperation represents a batch move operation
type PureMoveOperation struct {
	ItemID      PureID
	NewPosition PurePosition
}

// PureGapMetrics provides statistics about position distribution
type PureGapMetrics struct {
	TotalGaps    int
	LargestGap   *PureGap
	AverageSize  float64
	TotalMissing int
}

// =============================================================================
// 1. POSITION CALCULATION FUNCTIONS
// =============================================================================

// CalculatePositions computes positions for all items in a parent scope
// Pure function with O(n) performance, single pass traversal
func CalculatePositions(items []PureOrderable) map[string]*PurePosition {
	positions := make(map[string]*PurePosition)
	
	if len(items) == 0 {
		return positions
	}
	
	// Find head item (prev == nil)
	var head *PureOrderable
	itemMap := make(map[PureID]*PureOrderable)
	
	for i := range items {
		item := &items[i]
		itemMap[item.ID] = item
		if item.Prev == nil {
			head = item
		}
	}
	
	if head == nil {
		// No clear head found, use first item
		if len(items) > 0 {
			head = &items[0]
		}
	}
	
	// Traverse linked list from head and assign positions
	current := head
	position := PurePosition(0)
	visited := make(map[PureID]bool)
	
	for current != nil {
		// Prevent infinite loops
		if visited[current.ID] {
			break
		}
		visited[current.ID] = true
		
		pos := position
		positions[string(current.ID)] = &pos
		
		// Move to next item
		if current.Next == nil {
			break
		}
		
		next, exists := itemMap[*current.Next]
		if !exists {
			break
		}
		
		current = next
		position++
	}
	
	return positions
}

// =============================================================================
// 2. ITEM INSERTION FUNCTIONS
// =============================================================================

// InsertItem creates a new ordered list with item inserted at specified position
// Pure function with O(n) time, O(n) space for immutable operation
func InsertItem(items []PureOrderable, newItem PureOrderable, afterID string) []PureOrderable {
	if len(items) == 0 {
		// First item in list
		newItem.Prev = nil
		newItem.Next = nil
		newItem.Position = 0
		return []PureOrderable{newItem}
	}
	
	result := make([]PureOrderable, 0, len(items)+1)
	inserted := false
	
	for i, item := range items {
		result = append(result, item)
		
		// Insert after this item if it matches afterID
		if !inserted && string(item.ID) == afterID {
			// Set up new item pointers
			newItem.Prev = &item.ID
			newItem.Next = item.Next
			newItem.Position = PurePosition(i + 1)
			
			// Insert new item
			result = append(result, newItem)
			inserted = true
			
			// Update positions of subsequent items
			for j := i + 1; j < len(items); j++ {
				items[j].Position++
			}
		}
	}
	
	// If not inserted yet, append to end
	if !inserted {
		if len(items) > 0 {
			lastItem := &items[len(items)-1]
			newItem.Prev = &lastItem.ID
			newItem.Next = nil
			newItem.Position = PurePosition(len(items))
		} else {
			newItem.Prev = nil
			newItem.Next = nil
			newItem.Position = 0
		}
		result = append(result, newItem)
	}
	
	// Update prev/next pointers for affected items
	return UpdatePrevNextPointers(result)
}

// =============================================================================
// 3. ITEM MOVEMENT FUNCTIONS
// =============================================================================

// MoveItem creates new ordered list with item moved to new position
// Pure function with O(n) time complexity
func MoveItem(items []PureOrderable, itemID string, newPosition *PurePosition) ([]PureOrderable, error) {
	if len(items) == 0 {
		return nil, errors.New("empty items list")
	}
	
	// Find item to move
	itemIndex := -1
	var movingItem PureOrderable
	
	for i, item := range items {
		if string(item.ID) == itemID {
			itemIndex = i
			movingItem = item
			break
		}
	}
	
	if itemIndex == -1 {
		return nil, fmt.Errorf("item %s not found", itemID)
	}
	
	targetPos := 0
	if newPosition != nil {
		targetPos = int(*newPosition)
	}
	
	if targetPos < 0 || targetPos >= len(items) {
		targetPos = len(items) - 1
	}
	
	// Create new slice without the moving item
	result := make([]PureOrderable, 0, len(items))
	for i, item := range items {
		if i != itemIndex {
			result = append(result, item)
		}
	}
	
	// Insert at new position
	if targetPos >= len(result) {
		result = append(result, movingItem)
	} else {
		result = append(result[:targetPos+1], result[targetPos:]...)
		result[targetPos] = movingItem
	}
	
	// Recalculate positions and pointers
	return UpdatePrevNextPointers(result), nil
}

// MoveItemAfter creates new ordered list with item moved after target
func MoveItemAfter(items []PureOrderable, itemID PureID, targetID PureID) ([]PureOrderable, error) {
	// Find target position
	targetPos := -1
	for i, item := range items {
		if item.ID == targetID {
			targetPos = i
			break
		}
	}
	
	if targetPos == -1 {
		return nil, fmt.Errorf("target %s not found", targetID)
	}
	
	newPos := PurePosition(targetPos + 1)
	return MoveItem(items, string(itemID), &newPos)
}

// MoveItemBefore creates new ordered list with item moved before target
func MoveItemBefore(items []PureOrderable, itemID PureID, targetID PureID) ([]PureOrderable, error) {
	// Find target position
	targetPos := -1
	for i, item := range items {
		if item.ID == targetID {
			targetPos = i
			break
		}
	}
	
	if targetPos == -1 {
		return nil, fmt.Errorf("target %s not found", targetID)
	}
	
	newPos := PurePosition(targetPos)
	return MoveItem(items, string(itemID), &newPos)
}

// =============================================================================
// 4. VALIDATION FUNCTIONS
// =============================================================================

// ValidateOrdering checks linked-list integrity and position consistency
// Returns slice of validation errors (empty if valid)
func ValidateOrdering(items []PureOrderable) error {
	if len(items) == 0 {
		return nil
	}
	
	// Check for exactly one head and one tail
	headCount := 0
	tailCount := 0
	itemMap := make(map[PureID]*PureOrderable)
	
	for i := range items {
		item := &items[i]
		itemMap[item.ID] = item
		
		if item.Prev == nil {
			headCount++
		}
		if item.Next == nil {
			tailCount++
		}
	}
	
	if headCount != 1 {
		return fmt.Errorf("expected exactly 1 head item, found %d", headCount)
	}
	if tailCount != 1 {
		return fmt.Errorf("expected exactly 1 tail item, found %d", tailCount)
	}
	
	// Validate forward and backward links
	for _, item := range items {
		// Check next link consistency
		if item.Next != nil {
			nextItem, exists := itemMap[*item.Next]
			if !exists {
				return fmt.Errorf("item %s points to non-existent next item %s", item.ID, *item.Next)
			}
			if nextItem.Prev == nil || *nextItem.Prev != item.ID {
				return fmt.Errorf("broken backward link: %s->next=%s but %s->prev=%v", 
					item.ID, *item.Next, *item.Next, nextItem.Prev)
			}
		}
		
		// Check prev link consistency
		if item.Prev != nil {
			prevItem, exists := itemMap[*item.Prev]
			if !exists {
				return fmt.Errorf("item %s points to non-existent prev item %s", item.ID, *item.Prev)
			}
			if prevItem.Next == nil || *prevItem.Next != item.ID {
				return fmt.Errorf("broken forward link: %s->prev=%s but %s->next=%v", 
					item.ID, *item.Prev, *item.Prev, prevItem.Next)
			}
		}
	}
	
	// Check for circular references by traversing from head
	visited := make(map[PureID]bool)
	var head *PureOrderable
	
	for i := range items {
		if items[i].Prev == nil {
			head = &items[i]
			break
		}
	}
	
	current := head
	for current != nil {
		if visited[current.ID] {
			return fmt.Errorf("circular reference detected at item %s", current.ID)
		}
		visited[current.ID] = true
		
		if current.Next == nil {
			break
		}
		current = itemMap[*current.Next]
	}
	
	return nil
}

// =============================================================================
// 5. GAP DETECTION FUNCTIONS
// =============================================================================

// FindGaps identifies missing positions in sequence
// Time complexity: O(n log n) due to sorting requirement
func FindGaps(items []PureOrderable) []*PureGap {
	if len(items) <= 1 {
		return nil
	}
	
	// Collect and sort positions
	positions := make([]int, len(items))
	for i, item := range items {
		positions[i] = int(item.Position)
	}
	sort.Ints(positions)
	
	gaps := make([]*PureGap, 0)
	
	for i := 1; i < len(positions); i++ {
		current := positions[i]
		previous := positions[i-1]
		
		if current > previous+1 {
			gaps = append(gaps, &PureGap{
				StartPosition: PurePosition(previous + 1),
				EndPosition:   PurePosition(current - 1),
				Size:          current - previous - 1,
			})
		}
	}
	
	return gaps
}

// FindLargestGap returns the largest gap for insertion optimization
func FindLargestGap(items []PureOrderable) (*PureGap, error) {
	gaps := FindGaps(items)
	if len(gaps) == 0 {
		return nil, errors.New("no gaps found")
	}
	
	largestGap := gaps[0]
	for _, gap := range gaps[1:] {
		if gap.Size > largestGap.Size {
			largestGap = gap
		}
	}
	
	return largestGap, nil
}

// CalculateGapMetrics provides statistics about position distribution
func CalculateGapMetrics(items []PureOrderable) PureGapMetrics {
	gaps := FindGaps(items)
	
	metrics := PureGapMetrics{
		TotalGaps: len(gaps),
	}
	
	if len(gaps) == 0 {
		return metrics
	}
	
	totalSize := 0
	largestGap := gaps[0]
	
	for _, gap := range gaps {
		totalSize += gap.Size
		if gap.Size > largestGap.Size {
			largestGap = gap
		}
	}
	
	metrics.LargestGap = largestGap
	metrics.AverageSize = float64(totalSize) / float64(len(gaps))
	metrics.TotalMissing = totalSize
	
	return metrics
}

// =============================================================================
// 6. REBALANCING FUNCTIONS
// =============================================================================

// RebalancePositions redistributes positions to eliminate gaps
// Linear time O(n) rebalancing
func RebalancePositions(items []PureOrderable) []PureOrderable {
	if len(items) == 0 {
		return items
	}
	
	// Create copy to avoid mutating input
	result := make([]PureOrderable, len(items))
	copy(result, items)
	
	// Sort by current position to maintain order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Position < result[j].Position
	})
	
	// Reassign sequential positions
	for i := range result {
		result[i].Position = PurePosition(i)
	}
	
	// Update prev/next pointers
	return UpdatePrevNextPointers(result)
}

// RebalanceWithSpacing redistributes with specified spacing between items
func RebalanceWithSpacing(items []PureOrderable, spacing int) []PureOrderable {
	if len(items) == 0 || spacing < 1 {
		return RebalancePositions(items)
	}
	
	// Create copy and sort by position
	result := make([]PureOrderable, len(items))
	copy(result, items)
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].Position < result[j].Position
	})
	
	// Reassign positions with spacing
	for i := range result {
		result[i].Position = PurePosition(i * spacing)
	}
	
	return UpdatePrevNextPointers(result)
}

// SelectiveRebalance rebalances only items with gaps larger than threshold
func SelectiveRebalance(items []PureOrderable, gapThreshold int) PureRebalanceResult {
	result := PureRebalanceResult{
		PositionChanges: make(map[PureID]PurePosition),
		GapsEliminated:  0,
	}
	
	gaps := FindGaps(items)
	largeGaps := 0
	
	// Count gaps that exceed threshold
	for _, gap := range gaps {
		if gap.Size >= gapThreshold {
			largeGaps++
		}
	}
	
	// Only rebalance if there are significant gaps
	if largeGaps > 0 {
		oldPositions := make(map[PureID]PurePosition)
		for _, item := range items {
			oldPositions[item.ID] = item.Position
		}
		
		rebalanced := RebalancePositions(items)
		
		// Track position changes
		for _, item := range rebalanced {
			if oldPos, exists := oldPositions[item.ID]; exists {
				if oldPos != item.Position {
					result.PositionChanges[item.ID] = item.Position
				}
			}
		}
		
		result.UpdatedItems = rebalanced
		result.GapsEliminated = largeGaps
	} else {
		// No rebalancing needed
		result.UpdatedItems = make([]PureOrderable, len(items))
		copy(result.UpdatedItems, items)
	}
	
	return result
}

// =============================================================================
// 7. BATCH OPERATION FUNCTIONS
// =============================================================================

// BatchInsert efficiently inserts multiple items
func BatchInsert(items []PureOrderable, newItems []PureOrderable, positions []PurePosition) ([]PureOrderable, error) {
	if len(newItems) != len(positions) {
		return nil, errors.New("newItems and positions slices must have same length")
	}
	
	result := make([]PureOrderable, len(items))
	copy(result, items)
	
	// Sort insertions by position (descending to avoid index shifts)
	type insertion struct {
		item PureOrderable
		pos  PurePosition
	}
	
	insertions := make([]insertion, len(newItems))
	for i, item := range newItems {
		insertions[i] = insertion{item: item, pos: positions[i]}
	}
	
	sort.Slice(insertions, func(i, j int) bool {
		return insertions[i].pos > insertions[j].pos
	})
	
	// Insert items one by one
	for _, ins := range insertions {
		pos := int(ins.pos)
		if pos > len(result) {
			pos = len(result)
		}
		
		// Insert at position
		ins.item.Position = PurePosition(pos)
		result = append(result[:pos], append([]PureOrderable{ins.item}, result[pos:]...)...)
	}
	
	return UpdatePrevNextPointers(result), nil
}

// BatchMove efficiently moves multiple items to new positions
func BatchMove(items []PureOrderable, moves []PureMoveOperation) ([]PureOrderable, error) {
	result := make([]PureOrderable, len(items))
	copy(result, items)
	
	// Sort moves by target position to minimize conflicts
	sort.Slice(moves, func(i, j int) bool {
		return moves[i].NewPosition < moves[j].NewPosition
	})
	
	// Apply moves sequentially
	for _, move := range moves {
		var err error
		result, err = MoveItem(result, string(move.ItemID), &move.NewPosition)
		if err != nil {
			return nil, fmt.Errorf("failed to move item %s: %w", move.ItemID, err)
		}
	}
	
	return result, nil
}

// BatchReorder reorders items according to new ID sequence
func BatchReorder(items []PureOrderable, newOrder []PureID) ([]PureOrderable, error) {
	if len(newOrder) != len(items) {
		return nil, errors.New("newOrder must contain all item IDs")
	}
	
	// Create lookup map for items
	itemMap := make(map[PureID]*PureOrderable)
	for i := range items {
		itemMap[items[i].ID] = &items[i]
	}
	
	// Verify all IDs exist
	for _, id := range newOrder {
		if _, exists := itemMap[id]; !exists {
			return nil, fmt.Errorf("item %s not found in original list", id)
		}
	}
	
	// Build new ordered list
	result := make([]PureOrderable, len(newOrder))
	for i, id := range newOrder {
		item := *itemMap[id] // Copy item
		item.Position = PurePosition(i)
		result[i] = item
	}
	
	return UpdatePrevNextPointers(result), nil
}

// =============================================================================
// HELPER UTILITY FUNCTIONS
// =============================================================================

// UpdatePrevNextPointers recalculates prev/next for slice-ordered items
func UpdatePrevNextPointers(orderedItems []PureOrderable) []PureOrderable {
	result := make([]PureOrderable, len(orderedItems))
	
	for i, item := range orderedItems {
		result[i] = item
		result[i].Position = PurePosition(i)
		
		// Set previous pointer
		if i > 0 {
			prevID := orderedItems[i-1].ID
			result[i].Prev = &prevID
		} else {
			result[i].Prev = nil
		}
		
		// Set next pointer
		if i < len(orderedItems)-1 {
			nextID := orderedItems[i+1].ID
			result[i].Next = &nextID
		} else {
			result[i].Next = nil
		}
	}
	
	return result
}

// FindItemByID locates item in slice by ID
func FindItemByID(items []PureOrderable, id PureID) (*PureOrderable, int, bool) {
	for i, item := range items {
		if item.ID == id {
			return &item, i, true
		}
	}
	return nil, -1, false
}

// GetOrderedItemIDs returns slice of IDs in position order
func GetOrderedItemIDs(items []PureOrderable) []PureID {
	// Sort by position
	sorted := make([]PureOrderable, len(items))
	copy(sorted, items)
	
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})
	
	ids := make([]PureID, len(sorted))
	for i, item := range sorted {
		ids[i] = item.ID
	}
	
	return ids
}

// OrderableFromLinkedList converts linked-list structure to position-based slice
func OrderableFromLinkedList(items []PureOrderable) ([]PureOrderable, error) {
	if len(items) == 0 {
		return nil, nil
	}
	
	// Find head
	var head *PureOrderable
	itemMap := make(map[PureID]*PureOrderable)
	
	for i := range items {
		item := &items[i]
		itemMap[item.ID] = item
		if item.Prev == nil {
			head = item
		}
	}
	
	if head == nil {
		return nil, errors.New("no head item found (item with prev == nil)")
	}
	
	// Traverse from head to build ordered list
	result := make([]PureOrderable, 0, len(items))
	current := head
	visited := make(map[PureID]bool)
	position := PurePosition(0)
	
	for current != nil {
		if visited[current.ID] {
			return nil, fmt.Errorf("circular reference detected at item %s", current.ID)
		}
		visited[current.ID] = true
		
		// Copy item with updated position
		orderedItem := *current
		orderedItem.Position = position
		result = append(result, orderedItem)
		
		// Move to next
		if current.Next == nil {
			break
		}
		
		next, exists := itemMap[*current.Next]
		if !exists {
			return nil, fmt.Errorf("item %s references non-existent next item %s", current.ID, *current.Next)
		}
		
		current = next
		position++
	}
	
	if len(result) != len(items) {
		return nil, fmt.Errorf("traversal found %d items but expected %d (possible disconnected items)", len(result), len(items))
	}
	
	return result, nil
}

// LinkedListFromOrderable converts position-based slice to linked-list structure
func LinkedListFromOrderable(items []PureOrderable) []PureOrderable {
	if len(items) == 0 {
		return nil
	}
	
	// Sort by position first
	sorted := make([]PureOrderable, len(items))
	copy(sorted, items)
	
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})
	
	// Update prev/next pointers
	return UpdatePrevNextPointers(sorted)
}

// GroupByParent groups items by their parent scope
func GroupByParent(items []PureOrderable) map[PureParentScope][]PureOrderable {
	groups := make(map[PureParentScope][]PureOrderable)
	
	for _, item := range items {
		scope := PureParentScope{ID: item.ParentID, Type: "default"}
		groups[scope] = append(groups[scope], item)
	}
	
	return groups
}

// CalculateSequentialPositions assigns positions 0, 1, 2, ... n-1
func CalculateSequentialPositions(items []PureOrderable) PurePositionMap {
	positions := make(PurePositionMap)
	
	for i, item := range items {
		positions[item.ID] = PurePosition(i)
	}
	
	return positions
}

// CalculateSpacedPositions assigns positions with specified spacing
func CalculateSpacedPositions(items []PureOrderable, spacing int) PurePositionMap {
	positions := make(PurePositionMap)
	
	for i, item := range items {
		positions[item.ID] = PurePosition(i * spacing)
	}
	
	return positions
}

// CalculatePositionFromPointers traverses prev/next to determine position
func CalculatePositionFromPointers(items []PureOrderable, targetID PureID) (PurePosition, error) {
	if len(items) == 0 {
		return 0, errors.New("empty items list")
	}
	
	// Build item map
	itemMap := make(map[PureID]*PureOrderable)
	var head *PureOrderable
	
	for i := range items {
		item := &items[i]
		itemMap[item.ID] = item
		if item.Prev == nil {
			head = item
		}
	}
	
	if head == nil {
		return 0, errors.New("no head item found")
	}
	
	// Traverse from head until we find target
	current := head
	position := PurePosition(0)
	visited := make(map[PureID]bool)
	
	for current != nil {
		if visited[current.ID] {
			return 0, fmt.Errorf("circular reference detected")
		}
		visited[current.ID] = true
		
		if current.ID == targetID {
			return position, nil
		}
		
		if current.Next == nil {
			break
		}
		
		next, exists := itemMap[*current.Next]
		if !exists {
			return 0, fmt.Errorf("broken link: item %s references non-existent next item %s", current.ID, *current.Next)
		}
		
		current = next
		position++
	}
	
	return 0, fmt.Errorf("item %s not found in linked list", targetID)
}