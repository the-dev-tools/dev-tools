//nolint:revive // exported
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

// ReorderAfter moves an item to be positioned after the target item using pure functions
func (m *DefaultLinkedListManager) ReorderAfter(ctx context.Context, tx *sql.Tx, itemID, targetID idwrap.IDWrap, listType ListType) (*MoveResult, error) {
	// First, determine the parent context by getting the target item
	targetItems, err := m.repo.GetItemsByParent(ctx, targetID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get target item context: %w", err),
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

	// Get all items in the same list to convert to pure functions format
	allItems, err := m.repo.GetItemsByParent(ctx, *targetItem.ParentID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get items in list: %w", err),
		}, nil
	}

	// Find target position for pure function call
	targetPosition := -1
	for _, item := range allItems {
		if item.ID == targetID {
			targetPosition = item.Position
			break
		}
	}
	
	if targetPosition == -1 {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item not found in list"),
		}, nil
	}

	// Calculate position after target (for pure function call)
	newPosition := targetPosition + 1
	if newPosition >= len(allItems) {
		newPosition = len(allItems) - 1 // Move to end
	}

	// Use the repository's UpdatePosition which leverages pure functions
	repoWithTx := m.repo
	if tx != nil {
		if txRepo, ok := m.repo.(interface{ TX(tx *sql.Tx) MovableRepository }); ok {
			repoWithTx = txRepo.TX(tx)
		}
	}
	
	err = repoWithTx.UpdatePosition(ctx, tx, itemID, listType, newPosition)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to update position using pure functions: %w", err),
		}, nil
	}

	// Get affected items by re-querying (pure function repositories handle affected items internally)
	updatedItems, err := m.repo.GetItemsByParent(ctx, *targetItem.ParentID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get updated items: %w", err),
		}, nil
	}

	// Find the actual new position and collect affected IDs
	actualNewPosition := -1
	var affectedIDs []idwrap.IDWrap
	for _, item := range updatedItems {
		if item.ID == itemID {
			actualNewPosition = item.Position
		}
		affectedIDs = append(affectedIDs, item.ID)
	}

	return &MoveResult{
		Success:     true,
		NewPosition: actualNewPosition,
		AffectedIDs: affectedIDs,
	}, nil
}

// ReorderBefore moves an item to be positioned before the target item using pure functions
func (m *DefaultLinkedListManager) ReorderBefore(ctx context.Context, tx *sql.Tx, itemID, targetID idwrap.IDWrap, listType ListType) (*MoveResult, error) {
	// First, determine the parent context by getting the target item
	targetItems, err := m.repo.GetItemsByParent(ctx, targetID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get target item context: %w", err),
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

	// Get all items in the same list to convert to pure functions format
	allItems, err := m.repo.GetItemsByParent(ctx, *targetItem.ParentID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get items in list: %w", err),
		}, nil
	}

	// Find target position for pure function call
	targetPosition := -1
	for _, item := range allItems {
		if item.ID == targetID {
			targetPosition = item.Position
			break
		}
	}
	
	if targetPosition == -1 {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("target item not found in list"),
		}, nil
	}

	// Calculate position before target (for pure function call)
	newPosition := targetPosition
	if newPosition > 0 {
		newPosition = targetPosition
	} else {
		newPosition = 0 // Move to beginning
	}

	// Use the repository's UpdatePosition which leverages pure functions
	repoWithTx := m.repo
	if tx != nil {
		if txRepo, ok := m.repo.(interface{ TX(tx *sql.Tx) MovableRepository }); ok {
			repoWithTx = txRepo.TX(tx)
		}
	}
	
	err = repoWithTx.UpdatePosition(ctx, tx, itemID, listType, newPosition)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to update position using pure functions: %w", err),
		}, nil
	}

	// Get affected items by re-querying (pure function repositories handle affected items internally)
	updatedItems, err := m.repo.GetItemsByParent(ctx, *targetItem.ParentID, listType)
	if err != nil {
		return &MoveResult{
			Success: false,
			Error:   fmt.Errorf("failed to get updated items: %w", err),
		}, nil
	}

	// Find the actual new position and collect affected IDs
	actualNewPosition := -1
	var affectedIDs []idwrap.IDWrap
	for _, item := range updatedItems {
		if item.ID == itemID {
			actualNewPosition = item.Position
		}
		affectedIDs = append(affectedIDs, item.ID)
	}

	return &MoveResult{
		Success:     true,
		NewPosition: actualNewPosition,
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
// The modern repositories using pure functions handle this automatically, but this provides explicit compaction
func (m *DefaultLinkedListManager) CompactPositions(ctx context.Context, tx *sql.Tx, parentID idwrap.IDWrap, listType ListType) error {
	// With pure function repositories, we can trigger rebalancing
	// Check if the repository supports rebalancing (newer repositories do)
	if rebalancer, ok := m.repo.(interface {
		RebalanceCollections(ctx context.Context, tx *sql.Tx, parentID idwrap.IDWrap) error
	}); ok {
		return rebalancer.RebalanceCollections(ctx, tx, parentID)
	}

	// Fallback to manual position updates for legacy compatibility
	items, err := m.repo.GetItemsByParent(ctx, parentID, listType)
	if err != nil {
		return fmt.Errorf("failed to get items for compacting: %w", err)
	}

	// Use sequential position updates through the repository
	// The repository's UpdatePosition method uses pure functions internally
	repoWithTx := m.repo
	if tx != nil {
		if txRepo, ok := m.repo.(interface{ TX(tx *sql.Tx) MovableRepository }); ok {
			repoWithTx = txRepo.TX(tx)
		}
	}

	for i, item := range items {
		err := repoWithTx.UpdatePosition(ctx, tx, item.ID, listType, i)
		if err != nil {
			return fmt.Errorf("failed to compact position for item %s: %w", item.ID.String(), err)
		}
	}

	return nil
}

// SafeDelete orchestrates a safe delete by first unlinking the item from its neighbors
// via the repository's Remove, then invoking the provided delete function to remove
// the row from storage. When tx is non-nil, implementations should operate within it.
func (m *DefaultLinkedListManager) SafeDelete(
    ctx context.Context,
    tx *sql.Tx,
    itemID idwrap.IDWrap,
    deleteFn func(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error,
) error {
    if (idwrap.IDWrap{}) == itemID {
        return fmt.Errorf("SafeDelete: itemID cannot be empty")
    }
    if deleteFn == nil {
        return fmt.Errorf("SafeDelete: deleteFn cannot be nil")
    }

    // Unlink the node from its list neighbors first
    if err := m.repo.Remove(ctx, tx, itemID); err != nil {
        return fmt.Errorf("SafeDelete: unlink failed: %w", err)
    }
    // Perform the actual delete
    if err := deleteFn(ctx, tx, itemID); err != nil {
        return fmt.Errorf("SafeDelete: delete failed: %w", err)
    }
    return nil
}
