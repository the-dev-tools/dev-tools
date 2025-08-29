package movable

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/server/pkg/idwrap"
)

// LinkedListPointers represents the pointer structure of a linked list item
type LinkedListPointers struct {
	ItemID   idwrap.IDWrap
	Prev     *idwrap.IDWrap
	Next     *idwrap.IDWrap
	ParentID idwrap.IDWrap
	Position int
}

// MoveContext contains all necessary context information for a move operation
type MoveContext struct {
	ItemID       idwrap.IDWrap
	TargetID     idwrap.IDWrap
	ParentID     idwrap.IDWrap
	ListType     ListType
	Position     MovePosition
	AllItems     []LinkedListPointers
	ItemIndex    int
	TargetIndex  int
}

// PointerCalculation represents a calculated pointer update
type PointerCalculation struct {
	ItemID   idwrap.IDWrap
	NewPrev  *idwrap.IDWrap
	NewNext  *idwrap.IDWrap
	NewPos   int
}

// LinkedListUtils provides generic utility functions for linked list operations
type LinkedListUtils struct{}

// NewLinkedListUtils creates a new LinkedListUtils instance
func NewLinkedListUtils() *LinkedListUtils {
	return &LinkedListUtils{}
}

// ValidateMoveOperation performs comprehensive validation for move operations
func (u *LinkedListUtils) ValidateMoveOperation(ctx context.Context, operation MoveOperation) error {
	// Basic validation
	if err := u.validateBasicOperation(operation); err != nil {
		return err
	}

	// Self-reference validation
	if operation.TargetID != nil && operation.ItemID == *operation.TargetID {
		return ErrSelfReference
	}

	return nil
}

// validateBasicOperation performs basic validation of operation parameters
func (u *LinkedListUtils) validateBasicOperation(operation MoveOperation) error {
	if operation.ItemID == (idwrap.IDWrap{}) {
		return ErrEmptyItemID
	}

	if operation.ListType == nil {
		return ErrInvalidListType
	}

	switch operation.Position {
	case MovePositionAfter, MovePositionBefore:
		if operation.TargetID == nil {
			return ErrEmptyTargetID
		}
		if *operation.TargetID == (idwrap.IDWrap{}) {
			return ErrEmptyTargetID
		}
	case MovePositionUnspecified:
		return ErrInvalidPosition
	default:
		return fmt.Errorf("%w: %v", ErrInvalidPosition, operation.Position)
	}

	return nil
}

// BuildMoveContext creates a MoveContext by analyzing the current list state
func (u *LinkedListUtils) BuildMoveContext(
	ctx context.Context,
	operation MoveOperation,
	allItems []LinkedListPointers,
) (*MoveContext, error) {
	
	if operation.TargetID == nil {
		return nil, ErrEmptyTargetID
	}

	moveCtx := &MoveContext{
		ItemID:      operation.ItemID,
		TargetID:    *operation.TargetID,
		ListType:    operation.ListType,
		Position:    operation.Position,
		AllItems:    allItems,
		ItemIndex:   -1,
		TargetIndex: -1,
	}

	// Find indices of item and target
	for i, item := range allItems {
		if item.ItemID == operation.ItemID {
			moveCtx.ItemIndex = i
			moveCtx.ParentID = item.ParentID
		}
		if item.ItemID == *operation.TargetID {
			moveCtx.TargetIndex = i
		}
	}

	if moveCtx.ItemIndex == -1 {
		return nil, ErrItemNotFound
	}
	if moveCtx.TargetIndex == -1 {
		return nil, ErrTargetNotFound
	}

	return moveCtx, nil
}

// CalculatePointerUpdatesAfter calculates pointer updates for moving an item after a target
func (u *LinkedListUtils) CalculatePointerUpdatesAfter(moveCtx *MoveContext) ([]PointerCalculation, error) {
	if moveCtx.ItemIndex == moveCtx.TargetIndex {
		return nil, ErrSelfReference
	}

	var calculations []PointerCalculation
	items := moveCtx.AllItems
	itemIdx := moveCtx.ItemIndex
	targetIdx := moveCtx.TargetIndex

	// Remove item from its current position logically
	newItems := make([]LinkedListPointers, 0, len(items))
	for i, item := range items {
		if i != itemIdx {
			newItems = append(newItems, item)
		}
	}

	// Adjust target index if item was before target in original list
	newTargetIdx := targetIdx
	if itemIdx < targetIdx {
		newTargetIdx = targetIdx - 1
	}

	// Insert item after target in new list
	insertPos := newTargetIdx + 1
	finalItems := make([]LinkedListPointers, 0, len(items))
	finalItems = append(finalItems, newItems[:insertPos]...)
	finalItems = append(finalItems, items[itemIdx])
	finalItems = append(finalItems, newItems[insertPos:]...)

	// Calculate pointer updates based on final positions
	calculations = u.calculatePointersFromOrder(finalItems)

	return calculations, nil
}

// CalculatePointerUpdatesBefore calculates pointer updates for moving an item before a target
func (u *LinkedListUtils) CalculatePointerUpdatesBefore(moveCtx *MoveContext) ([]PointerCalculation, error) {
	if moveCtx.ItemIndex == moveCtx.TargetIndex {
		return nil, ErrSelfReference
	}

	var calculations []PointerCalculation
	items := moveCtx.AllItems
	itemIdx := moveCtx.ItemIndex
	targetIdx := moveCtx.TargetIndex

	// Remove item from its current position logically
	newItems := make([]LinkedListPointers, 0, len(items))
	for i, item := range items {
		if i != itemIdx {
			newItems = append(newItems, item)
		}
	}

	// Adjust target index if item was before target in original list
	newTargetIdx := targetIdx
	if itemIdx < targetIdx {
		newTargetIdx = targetIdx - 1
	}

	// Insert item before target in new list
	insertPos := newTargetIdx
	finalItems := make([]LinkedListPointers, 0, len(items))
	finalItems = append(finalItems, newItems[:insertPos]...)
	finalItems = append(finalItems, items[itemIdx])
	finalItems = append(finalItems, newItems[insertPos:]...)

	// Calculate pointer updates based on final positions
	calculations = u.calculatePointersFromOrder(finalItems)

	return calculations, nil
}

// calculatePointersFromOrder calculates prev/next pointers from ordered item list
func (u *LinkedListUtils) calculatePointersFromOrder(orderedItems []LinkedListPointers) []PointerCalculation {
	calculations := make([]PointerCalculation, len(orderedItems))

	for i, item := range orderedItems {
		var newPrev, newNext *idwrap.IDWrap

		if i > 0 {
			newPrev = &orderedItems[i-1].ItemID
		}
		if i < len(orderedItems)-1 {
			newNext = &orderedItems[i+1].ItemID
		}

		calculations[i] = PointerCalculation{
			ItemID:  item.ItemID,
			NewPrev: newPrev,
			NewNext: newNext,
			NewPos:  i,
		}
	}

	return calculations
}

// OptimizePointerUpdates removes redundant pointer updates (items that don't need changes)
func (u *LinkedListUtils) OptimizePointerUpdates(
	calculations []PointerCalculation,
	originalItems []LinkedListPointers,
) []PointerCalculation {
	
	// Create a map for quick lookup of original pointers
	originalMap := make(map[idwrap.IDWrap]LinkedListPointers)
	for _, item := range originalItems {
		originalMap[item.ItemID] = item
	}

	var optimized []PointerCalculation
	for _, calc := range calculations {
		original, exists := originalMap[calc.ItemID]
		if !exists {
			// Item doesn't exist in original - definitely needs update
			optimized = append(optimized, calc)
			continue
		}

		// Check if pointers actually changed
		prevChanged := !u.pointersEqual(original.Prev, calc.NewPrev)
		nextChanged := !u.pointersEqual(original.Next, calc.NewNext)
		posChanged := original.Position != calc.NewPos

		if prevChanged || nextChanged || posChanged {
			optimized = append(optimized, calc)
		}
	}

	return optimized
}

// pointersEqual compares two pointers for equality
func (u *LinkedListUtils) pointersEqual(a, b *idwrap.IDWrap) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Compare(*b) == 0
}

// ValidateListIntegrity performs comprehensive linked list integrity validation
func (u *LinkedListUtils) ValidateListIntegrity(ctx context.Context, items []LinkedListPointers) error {
	if len(items) == 0 {
		return nil // Empty list is valid
	}

	// Check for exactly one head (prev == nil)
	headCount := 0
	tailCount := 0
	itemMap := make(map[idwrap.IDWrap]LinkedListPointers)

	for _, item := range items {
		itemMap[item.ItemID] = item
		
		if item.Prev == nil {
			headCount++
		}
		if item.Next == nil {
			tailCount++
		}
	}

	if headCount != 1 {
		return fmt.Errorf("linked list integrity violation: expected 1 head, found %d", headCount)
	}
	if tailCount != 1 {
		return fmt.Errorf("linked list integrity violation: expected 1 tail, found %d", tailCount)
	}

	// Validate forward and backward links
	for _, item := range items {
		// Validate forward link
		if item.Next != nil {
			nextItem, exists := itemMap[*item.Next]
			if !exists {
				return fmt.Errorf("linked list integrity violation: item %s points to non-existent next item %s", 
					item.ItemID.String(), item.Next.String())
			}
			
			// Check backward link consistency
			if nextItem.Prev == nil || nextItem.Prev.Compare(item.ItemID) != 0 {
				return fmt.Errorf("linked list integrity violation: forward/backward link mismatch between %s and %s",
					item.ItemID.String(), nextItem.ItemID.String())
			}
		}

		// Validate backward link
		if item.Prev != nil {
			prevItem, exists := itemMap[*item.Prev]
			if !exists {
				return fmt.Errorf("linked list integrity violation: item %s points to non-existent prev item %s", 
					item.ItemID.String(), item.Prev.String())
			}
			
			// Check forward link consistency
			if prevItem.Next == nil || prevItem.Next.Compare(item.ItemID) != 0 {
				return fmt.Errorf("linked list integrity violation: forward/backward link mismatch between %s and %s",
					prevItem.ItemID.String(), item.ItemID.String())
			}
		}
	}

	// Validate that we can traverse the entire list from head to tail
	var head *LinkedListPointers
	for i := range items {
		if items[i].Prev == nil {
			head = &items[i]
			break
		}
	}

	if head == nil {
		return fmt.Errorf("linked list integrity violation: no head found")
	}

	// Traverse from head and count items
	visited := make(map[idwrap.IDWrap]bool)
	current := head
	count := 0

	for current != nil {
		if visited[current.ItemID] {
			return fmt.Errorf("linked list integrity violation: circular reference detected at %s", 
				current.ItemID.String())
		}
		
		visited[current.ItemID] = true
		count++

		if current.Next == nil {
			break
		}

		nextItem, exists := itemMap[*current.Next]
		if !exists {
			return fmt.Errorf("linked list integrity violation: traversal failed at %s", 
				current.ItemID.String())
		}
		current = &nextItem
	}

	if count != len(items) {
		return fmt.Errorf("linked list integrity violation: traversal found %d items, expected %d", 
			count, len(items))
	}

	return nil
}

// AtomicMoveOperation performs an atomic move operation with transaction support
type AtomicMoveOperation struct {
	utils *LinkedListUtils
}

// NewAtomicMoveOperation creates a new AtomicMoveOperation
func NewAtomicMoveOperation() *AtomicMoveOperation {
	return &AtomicMoveOperation{
		utils: NewLinkedListUtils(),
	}
}

// ExecuteMove performs the complete move operation atomically
func (a *AtomicMoveOperation) ExecuteMove(
	ctx context.Context,
	tx *sql.Tx,
	operation MoveOperation,
	repository MovableRepository,
) (*MoveResult, error) {
	
	// Validate operation
	if err := a.utils.ValidateMoveOperation(ctx, operation); err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("validation failed: %w", err),
		}, nil
	}

	// Get all items in the list
	parentID := operation.NewParentID
	if parentID == nil {
		// Try to determine parent from the target item
		// This would need to be implemented based on specific repository logic
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("parent ID determination not implemented"),
		}, nil
	}

	allItems, err := repository.GetItemsByParent(ctx, *parentID, operation.ListType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get items: %w", err),
		}, nil
	}

	// Convert to LinkedListPointers
	linkedItems := make([]LinkedListPointers, len(allItems))
	for i, item := range allItems {
		linkedItems[i] = LinkedListPointers{
			ItemID:   item.ID,
			ParentID: *item.ParentID,
			Position: item.Position,
			// Note: Prev/Next would need to be populated from repository
		}
	}

	// Build move context
	moveCtx, err := a.utils.BuildMoveContext(ctx, operation, linkedItems)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to build move context: %w", err),
		}, nil
	}

	// Calculate pointer updates
	var calculations []PointerCalculation
	switch operation.Position {
	case MovePositionAfter:
		calculations, err = a.utils.CalculatePointerUpdatesAfter(moveCtx)
	case MovePositionBefore:
		calculations, err = a.utils.CalculatePointerUpdatesBefore(moveCtx)
	default:
		return &MoveResult{
			Success: false,
			Error:   ErrInvalidPosition,
		}, nil
	}

	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to calculate updates: %w", err),
		}, nil
	}

	// Optimize updates
	calculations = a.utils.OptimizePointerUpdates(calculations, linkedItems)

	// Convert to position updates
	var positionUpdates []PositionUpdate
	var affectedIDs []idwrap.IDWrap
	
	for _, calc := range calculations {
		positionUpdates = append(positionUpdates, PositionUpdate{
			ItemID:   calc.ItemID,
			ListType: operation.ListType,
			Position: calc.NewPos,
		})
		affectedIDs = append(affectedIDs, calc.ItemID)
	}

	// Execute updates
	if err := repository.UpdatePositions(ctx, tx, positionUpdates); err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to update positions: %w", err),
		}, nil
	}

	// Determine new position of moved item
	var newPosition int
	for _, calc := range calculations {
		if calc.ItemID == operation.ItemID {
			newPosition = calc.NewPos
			break
		}
	}

	return &MoveResult{
		Success:     true,
		NewPosition: newPosition,
		AffectedIDs: affectedIDs,
	}, nil
}

// CompactPositions recalculates and compacts position values to eliminate gaps
func (a *AtomicMoveOperation) CompactPositions(
	ctx context.Context,
	tx *sql.Tx,
	parentID idwrap.IDWrap,
	listType ListType,
	repository MovableRepository,
) error {
	
	items, err := repository.GetItemsByParent(ctx, parentID, listType)
	if err != nil {
		return fmt.Errorf("failed to get items for compacting: %w", err)
	}

	// Prepare updates with sequential positions
	var updates []PositionUpdate
	for i, item := range items {
		updates = append(updates, PositionUpdate{
			ItemID:   item.ID,
			ListType: listType,
			Position: i,
		})
	}

	// Execute the updates
	if err := repository.UpdatePositions(ctx, tx, updates); err != nil {
		return fmt.Errorf("failed to compact positions: %w", err)
	}

	return nil
}