package movable

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/server/pkg/idwrap"
)

// DefaultLinkedListManager provides a default implementation of LinkedListManager
type DefaultLinkedListManager struct {
	repo MovableRepository
}

// NewDefaultLinkedListManager creates a new DefaultLinkedListManager
func NewDefaultLinkedListManager(repo MovableRepository) *DefaultLinkedListManager {
	return &DefaultLinkedListManager{
		repo: repo,
	}
}

// Move performs a move operation on an item
func (m *DefaultLinkedListManager) Move(ctx context.Context, tx *sql.Tx, operation MoveOperation) (*MoveResult, error) {
	// Validate the move operation first
	if err := m.ValidateMove(ctx, operation); err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("move validation failed: %w", err),
		}, nil
	}

	switch operation.Position {
	case MovePositionAfter:
		if operation.TargetID == nil {
			return &MoveResult{
				Success: false,
				Error:   fmt.Errorf("target ID is required for AFTER position"),
			}, nil
		}
		return m.ReorderAfter(ctx, tx, operation.ItemID, *operation.TargetID, operation.ListType)
	
	case MovePositionBefore:
		if operation.TargetID == nil {
			return &MoveResult{
				Success: false,
				Error:   fmt.Errorf("target ID is required for BEFORE position"),
			}, nil
		}
		return m.ReorderBefore(ctx, tx, operation.ItemID, *operation.TargetID, operation.ListType)
	
	default:
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("unsupported move position: %v", operation.Position),
		}, nil
	}
}

// ReorderAfter moves an item to be positioned after the target item
func (m *DefaultLinkedListManager) ReorderAfter(ctx context.Context, tx *sql.Tx, itemID, targetID idwrap.IDWrap, listType ListType) (*MoveResult, error) {
	// Get target item's parent to determine the list context
	targetItems, err := m.repo.GetItemsByParent(ctx, targetID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get target item: %w", err),
		}, nil
	}
	
	if len(targetItems) == 0 {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item not found"),
		}, nil
	}
	
	targetItem := targetItems[0]
	if targetItem.ParentID == nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item has no parent"),
		}, nil
	}

	// Get all items in the same list
	allItems, err := m.repo.GetItemsByParent(ctx, *targetItem.ParentID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get items in list: %w", err),
		}, nil
	}

	// Find target position and calculate new positions
	var targetPosition int = -1
	var itemCurrentPosition int = -1
	
	for _, item := range allItems {
		if item.ID == targetID {
			targetPosition = item.Position
		}
		if item.ID == itemID {
			itemCurrentPosition = item.Position
		}
	}

	if targetPosition == -1 {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item not found in list"),
		}, nil
	}

	// Calculate new position (after target)
	newPosition := targetPosition + 1

	// Prepare position updates
	var updates []PositionUpdate
	affectedIDs := []idwrap.IDWrap{itemID}

	// If moving item down in the list, shift items between old and new position
	if itemCurrentPosition != -1 && itemCurrentPosition < newPosition {
		for _, item := range allItems {
			if item.Position > itemCurrentPosition && item.Position <= targetPosition {
				updates = append(updates, PositionUpdate{
					ItemID:   item.ID,
					ListType: listType,
					Position: item.Position - 1,
				})
				affectedIDs = append(affectedIDs, item.ID)
			}
		}
		newPosition = targetPosition
	} else if itemCurrentPosition != -1 && itemCurrentPosition > newPosition {
		// Moving item up in the list, shift items between new and old position
		for _, item := range allItems {
			if item.Position >= newPosition && item.Position < itemCurrentPosition {
				updates = append(updates, PositionUpdate{
					ItemID:   item.ID,
					ListType: listType,
					Position: item.Position + 1,
				})
				affectedIDs = append(affectedIDs, item.ID)
			}
		}
	}

	// Add the main item's position update
	updates = append(updates, PositionUpdate{
		ItemID:   itemID,
		ListType: listType,
		Position: newPosition,
	})

	// Execute all updates
	if err := m.repo.UpdatePositions(ctx, tx, updates); err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to update positions: %w", err),
		}, nil
	}

	return &MoveResult{
		Success:     true,
		NewPosition: newPosition,
		AffectedIDs: affectedIDs,
	}, nil
}

// ReorderBefore moves an item to be positioned before the target item
func (m *DefaultLinkedListManager) ReorderBefore(ctx context.Context, tx *sql.Tx, itemID, targetID idwrap.IDWrap, listType ListType) (*MoveResult, error) {
	// Get target item's parent to determine the list context
	targetItems, err := m.repo.GetItemsByParent(ctx, targetID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get target item: %w", err),
		}, nil
	}
	
	if len(targetItems) == 0 {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item not found"),
		}, nil
	}
	
	targetItem := targetItems[0]
	if targetItem.ParentID == nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item has no parent"),
		}, nil
	}

	// Get all items in the same list
	allItems, err := m.repo.GetItemsByParent(ctx, *targetItem.ParentID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get items in list: %w", err),
		}, nil
	}

	// Find target position and calculate new positions
	var targetPosition int = -1
	var itemCurrentPosition int = -1
	
	for _, item := range allItems {
		if item.ID == targetID {
			targetPosition = item.Position
		}
		if item.ID == itemID {
			itemCurrentPosition = item.Position
		}
	}

	if targetPosition == -1 {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item not found in list"),
		}, nil
	}

	// Calculate new position (before target)
	newPosition := targetPosition

	// Prepare position updates
	var updates []PositionUpdate
	affectedIDs := []idwrap.IDWrap{itemID}

	// If moving item down in the list, shift items between old and new position
	if itemCurrentPosition != -1 && itemCurrentPosition < newPosition {
		for _, item := range allItems {
			if item.Position >= itemCurrentPosition+1 && item.Position < newPosition {
				updates = append(updates, PositionUpdate{
					ItemID:   item.ID,
					ListType: listType,
					Position: item.Position - 1,
				})
				affectedIDs = append(affectedIDs, item.ID)
			}
		}
		newPosition = targetPosition - 1
	} else if itemCurrentPosition != -1 && itemCurrentPosition > newPosition {
		// Moving item up in the list, shift items between new and old position
		for _, item := range allItems {
			if item.Position >= newPosition && item.Position < itemCurrentPosition {
				updates = append(updates, PositionUpdate{
					ItemID:   item.ID,
					ListType: listType,
					Position: item.Position + 1,
				})
				affectedIDs = append(affectedIDs, item.ID)
			}
		}
	}

	// Add the main item's position update
	updates = append(updates, PositionUpdate{
		ItemID:   itemID,
		ListType: listType,
		Position: newPosition,
	})

	// Execute all updates
	if err := m.repo.UpdatePositions(ctx, tx, updates); err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to update positions: %w", err),
		}, nil
	}

	return &MoveResult{
		Success:     true,
		NewPosition: newPosition,
		AffectedIDs: affectedIDs,
	}, nil
}

// GetItemsInList returns all items in the specified list, ordered by position
func (m *DefaultLinkedListManager) GetItemsInList(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]Movable, error) {
	items, err := m.repo.GetItemsByParent(ctx, parentID, listType)
	if err != nil {
		return nil, fmt.Errorf("failed to get items by parent: %w", err)
	}

	// Convert MovableItem to Movable interface
	// Note: This would need to be implemented by specific item types
	var movables []Movable
	for _, item := range items {
		// This is a placeholder - actual implementation would need to convert
		// specific item types to their Movable interface implementations
		_ = item
	}

	return movables, nil
}

// ValidateMove checks if a move operation is valid
func (m *DefaultLinkedListManager) ValidateMove(ctx context.Context, operation MoveOperation) error {
	// Basic validation
	if operation.ItemID == (idwrap.IDWrap{}) {
		return fmt.Errorf("item ID cannot be empty")
	}

	if operation.ListType == nil {
		return fmt.Errorf("list type cannot be nil")
	}

	// Validate position requirements
	switch operation.Position {
	case MovePositionAfter, MovePositionBefore:
		if operation.TargetID == nil {
			return fmt.Errorf("target ID is required for position %v", operation.Position)
		}
		
		// Prevent moving item to itself
		if operation.ItemID == *operation.TargetID {
			return fmt.Errorf("cannot move item to itself")
		}

	case MovePositionUnspecified:
		return fmt.Errorf("move position must be specified")

	default:
		return fmt.Errorf("unsupported move position: %v", operation.Position)
	}

	return nil
}

// CompactPositions recalculates and compacts position values to eliminate gaps
func (m *DefaultLinkedListManager) CompactPositions(ctx context.Context, tx *sql.Tx, parentID idwrap.IDWrap, listType ListType) error {
	items, err := m.repo.GetItemsByParent(ctx, parentID, listType)
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
	if err := m.repo.UpdatePositions(ctx, tx, updates); err != nil {
		return fmt.Errorf("failed to compact positions: %w", err)
	}

	return nil
}