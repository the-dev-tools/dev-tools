package movable

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
)

// CollectionRepository implements MovableRepository for Collections using the new
// idiomatic Go patterns with composition instead of inheritance
type CollectionRepository struct {
	// Composition: use configuration instead of inheriting from BaseLinkedListRepository
	queries *gen.Queries
	config  MoveConfigV2
}

// NewCollectionRepository creates a new CollectionRepository using composition pattern
func NewCollectionRepository(db *sql.DB, queries *gen.Queries) *CollectionRepository {
	// Build collection-specific configuration using V2 config system
	config, err := NewConfigBuilderV2().
		ForDirectFK("collections", "workspace_id").
		WithValidation(DefaultValidationRulesV2()).
		WithPerformance(DefaultPerformanceConfigV2()).
		Build()
	
	if err != nil {
		// In production, this would be handled differently
		panic(fmt.Sprintf("Failed to build collection configuration: %v", err))
	}
	
	return &CollectionRepository{
		queries: queries,
		config:  config,
	}
}

// TX returns a new repository instance with transaction support
// Implements the required TX method for transaction handling
func (r *CollectionRepository) TX(tx *sql.Tx) MovableRepository {
	return &CollectionRepository{
		queries: r.queries.WithTx(tx),
		config:  r.config,
	}
}

// UpdatePosition updates the position of a collection in the linked list
// Uses pure functions and configuration for collection-specific operations
func (r *CollectionRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType ListType, position int) error {
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*CollectionRepository)
	}
	
	// Get collection to determine workspace context (parent scope)
	collection, err := repo.queries.GetCollection(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get collection: %w", err)
	}
	
	workspaceID := collection.WorkspaceID
	
	// Get ordered collections in this workspace
	collectionRows, err := repo.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get ordered collections: %w", err)
	}
	
	// Convert to pure function types for calculation
	pureItems := make([]PureOrderable, len(collectionRows))
	for i, col := range collectionRows {
		colID := idwrap.NewFromBytesMust(col.ID)
		var prev, next *PureID
		if col.Prev != nil {
			prevID := PureID(idwrap.NewFromBytesMust(col.Prev).String())
			prev = &prevID
		}
		if col.Next != nil {
			nextID := PureID(idwrap.NewFromBytesMust(col.Next).String())
			next = &nextID
		}
		
		pureItems[i] = PureOrderable{
			ID:       PureID(colID.String()),
			Prev:     prev,
			Next:     next,
			ParentID: PureID(workspaceID.String()),
			Position: PurePosition(col.Position),
		}
	}
	
	// Validate position bounds
	if position < 0 || position >= len(pureItems) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(pureItems)-1)
	}
	
	// Use pure function to calculate move operation
	pos := PurePosition(position)
	moveResult, err := MoveItem(pureItems, itemID.String(), &pos)
	if err != nil {
		return fmt.Errorf("move calculation failed: %w", err)
	}
	
	// Validate the move result
	if err := ValidateOrdering(moveResult); err != nil {
		return fmt.Errorf("move validation failed: %w", err)
	}
	
	// Apply updates using collection-specific persistence
	return repo.persistCollectionUpdates(ctx, moveResult, workspaceID)
}

// UpdatePositions updates positions for multiple collections in batch
func (r *CollectionRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*CollectionRepository)
	}
	
	// For simplicity, process updates sequentially
	// In production, would group by workspace for atomic processing
	for _, update := range updates {
		err := repo.UpdatePosition(ctx, tx, update.ItemID, update.ListType, update.Position)
		if err != nil {
			return fmt.Errorf("failed to update collection %s: %w", update.ItemID.String(), err)
		}
	}
	
	return nil
}

// GetMaxPosition returns the maximum position value for collections in a workspace
func (r *CollectionRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType ListType) (int, error) {
	// For collections, parentID is the workspace_id
	collectionRows, err := r.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   parentID,
		WorkspaceID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No collections means no max position
		}
		return 0, fmt.Errorf("failed to get collections: %w", err)
	}
	
	if len(collectionRows) == 0 {
		return -1, nil
	}
	
	return len(collectionRows) - 1, nil
}

// GetItemsByParent returns all collections under a workspace, ordered by position
func (r *CollectionRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]MovableItem, error) {
	// For collections, parentID is the workspace_id
	collectionRows, err := r.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   parentID,
		WorkspaceID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get collections: %w", err)
	}
	
	// Convert to MovableItem structs
	items := make([]MovableItem, len(collectionRows))
	for i, col := range collectionRows {
		colID := idwrap.NewFromBytesMust(col.ID)
		items[i] = MovableItem{
			ID:       colID,
			ParentID: &parentID, // workspace_id as parent
			Position: int(col.Position),
			ListType: listType,
		}
	}
	
	return items, nil
}

// persistCollectionUpdates applies collection updates using pure functions
func (r *CollectionRepository) persistCollectionUpdates(ctx context.Context, items []PureOrderable, workspaceID idwrap.IDWrap) error {
	// Ensure pointers are consistent using pure function
	consistentItems := UpdatePrevNextPointers(items)
	
	// Apply each update atomically
	for _, item := range consistentItems {
		collectionID, err := idwrap.NewText(string(item.ID))
		if err != nil {
			return fmt.Errorf("failed to parse collection ID %s: %w", item.ID, err)
		}
		
		var prev, next *idwrap.IDWrap
		if item.Prev != nil {
			prevID, err := idwrap.NewText(string(*item.Prev))
			if err != nil {
				return fmt.Errorf("failed to parse prev ID %s: %w", *item.Prev, err)
			}
			prev = &prevID
		}
		if item.Next != nil {
			nextID, err := idwrap.NewText(string(*item.Next))
			if err != nil {
				return fmt.Errorf("failed to parse next ID %s: %w", *item.Next, err)
			}
			next = &nextID
		}
		
		err = r.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
			ID:          collectionID,
			Prev:        prev,
			Next:        next,
			WorkspaceID: workspaceID,
		})
		
		if err != nil {
			return fmt.Errorf("failed to update collection %s: %w", collectionID.String(), err)
		}
	}
	
	return nil
}

// =============================================================================
// ADDITIONAL METHODS DEMONSTRATING PURE FUNCTION INTEGRATION
// =============================================================================

// RebalanceCollections demonstrates using pure functions for rebalancing
func (r *CollectionRepository) RebalanceCollections(ctx context.Context, tx *sql.Tx, workspaceID idwrap.IDWrap) error {
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*CollectionRepository)
	}
	
	// Get current collections
	collectionRows, err := repo.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get collections for rebalancing: %w", err)
	}
	
	if len(collectionRows) == 0 {
		return nil // Nothing to rebalance
	}
	
	// Convert to pure function types
	pureItems := make([]PureOrderable, len(collectionRows))
	for i, col := range collectionRows {
		colID := idwrap.NewFromBytesMust(col.ID)
		var prev, next *PureID
		if col.Prev != nil {
			prevID := PureID(idwrap.NewFromBytesMust(col.Prev).String())
			prev = &prevID
		}
		if col.Next != nil {
			nextID := PureID(idwrap.NewFromBytesMust(col.Next).String())
			next = &nextID
		}
		
		pureItems[i] = PureOrderable{
			ID:       PureID(colID.String()),
			Prev:     prev,
			Next:     next,
			ParentID: PureID(workspaceID.String()),
			Position: PurePosition(col.Position),
		}
	}
	
	// Use pure function for rebalancing
	rebalancedItems := RebalancePositions(pureItems)
	
	// Validate the result
	if err := ValidateOrdering(rebalancedItems); err != nil {
		return fmt.Errorf("rebalancing validation failed: %w", err)
	}
	
	// Persist rebalanced positions
	return repo.persistCollectionUpdates(ctx, rebalancedItems, workspaceID)
}

// ValidateCollectionOrdering demonstrates using pure functions for validation
func (r *CollectionRepository) ValidateCollectionOrdering(ctx context.Context, workspaceID idwrap.IDWrap) error {
	// Get current collections
	collectionRows, err := r.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get collections for validation: %w", err)
	}
	
	if len(collectionRows) == 0 {
		return nil // Empty list is valid
	}
	
	// Convert to pure function types
	pureItems := make([]PureOrderable, len(collectionRows))
	for i, col := range collectionRows {
		colID := idwrap.NewFromBytesMust(col.ID)
		var prev, next *PureID
		if col.Prev != nil {
			prevID := PureID(idwrap.NewFromBytesMust(col.Prev).String())
			prev = &prevID
		}
		if col.Next != nil {
			nextID := PureID(idwrap.NewFromBytesMust(col.Next).String())
			next = &nextID
		}
		
		pureItems[i] = PureOrderable{
			ID:       PureID(colID.String()),
			Prev:     prev,
			Next:     next,
			ParentID: PureID(workspaceID.String()),
			Position: PurePosition(col.Position),
		}
	}
	
	// Use pure function for validation
	return ValidateOrdering(pureItems)
}