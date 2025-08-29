package scollection

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
)

// CollectionMovableRepository implements movable.MovableRepository for Collections
// It adapts position-based operations to linked list operations using prev/next pointers
type CollectionMovableRepository struct {
	queries *gen.Queries
}

// NewCollectionMovableRepository creates a new CollectionMovableRepository
func NewCollectionMovableRepository(queries *gen.Queries) *CollectionMovableRepository {
	return &CollectionMovableRepository{
		queries: queries,
	}
}

// TX returns a new repository instance with transaction support
func (r *CollectionMovableRepository) TX(tx *sql.Tx) *CollectionMovableRepository {
	return &CollectionMovableRepository{
		queries: r.queries.WithTx(tx),
	}
}

// UpdatePosition updates the position of a collection in the linked list
// For collections, parentID is the workspace_id and listType is ignored
func (r *CollectionMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get collection to find workspace_id
	collection, err := repo.queries.GetCollection(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get collection: %w", err)
	}

	// Get ordered list of collections in workspace
	orderedCollections, err := repo.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   collection.WorkspaceID,
		WorkspaceID_2: collection.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get collections in order: %w", err)
	}

	// Find current position and validate new position
	currentIdx := -1
	for i, col := range orderedCollections {
		if idwrap.NewFromBytesMust(col.ID).Compare(itemID) == 0 {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("collection not found in workspace")
	}

	if position < 0 || position >= len(orderedCollections) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(orderedCollections)-1)
	}

	if currentIdx == position {
		// No change needed
		return nil
	}

	// Build the new order by simulating the move operation
	newOrder := make([]idwrap.IDWrap, len(orderedCollections))
	for i, col := range orderedCollections {
		newOrder[i] = idwrap.NewFromBytesMust(col.ID)
	}
	
	// Remove the item from its current position
	movedItem := newOrder[currentIdx]
	newOrder = append(newOrder[:currentIdx], newOrder[currentIdx+1:]...)
	
	// Insert the item at the target position
	if position == len(orderedCollections) - 1 {
		// Moving to last position - append to end
		newOrder = append(newOrder, movedItem)
	} else if position <= currentIdx {
		// Target position is before or at the removed item's original position
		// Insert at the target position directly
		newOrder = append(newOrder[:position], append([]idwrap.IDWrap{movedItem}, newOrder[position:]...)...)
	} else {
		// Target position is after the removed item's original position
		// Insert at the original target position (no adjustment needed)
		if position >= len(newOrder) {
			// Append to end if target position is beyond the shortened array
			newOrder = append(newOrder, movedItem)
		} else {
			newOrder = append(newOrder[:position], append([]idwrap.IDWrap{movedItem}, newOrder[position:]...)...)
		}
	}

	// Update all items with their new prev/next pointers
	for i, id := range newOrder {
		var prev, next *idwrap.IDWrap
		if i > 0 {
			prev = &newOrder[i-1]
		}
		if i < len(newOrder)-1 {
			next = &newOrder[i+1]
		}
		
		err := repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
			Prev:        prev,
			Next:        next,
			ID:          id,
			WorkspaceID: collection.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to update collection %s: %w", id.String(), err)
		}
	}

	return nil
}

// UpdatePositions updates positions for multiple collections in batch
func (r *CollectionMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the workspace ID from the first collection to validate all are in same workspace
	firstCollection, err := repo.queries.GetCollection(ctx, updates[0].ItemID)
	if err != nil {
		return fmt.Errorf("failed to get first collection: %w", err)
	}
	workspaceID := firstCollection.WorkspaceID
	
	// Validate all collections are in the same workspace and create ID position map
	positionMap := make(map[idwrap.IDWrap]int)
	for _, update := range updates {
		collection, err := repo.queries.GetCollection(ctx, update.ItemID)
		if err != nil {
			return fmt.Errorf("failed to get collection %s: %w", update.ItemID.String(), err)
		}
		if collection.WorkspaceID.Compare(workspaceID) != 0 {
			return fmt.Errorf("all collections must be in the same workspace")
		}
		positionMap[update.ItemID] = update.Position
	}
	
	// Build the complete ordered list with all collections at their new positions
	orderedIDs := make([]idwrap.IDWrap, len(updates))
	for _, update := range updates {
		if update.Position < 0 || update.Position >= len(updates) {
			return fmt.Errorf("invalid position %d for collection %s (valid range: 0-%d)", 
				update.Position, update.ItemID.String(), len(updates)-1)
		}
		orderedIDs[update.Position] = update.ItemID
	}
	
	// Calculate prev/next pointers for each collection in the new order
	type ptrUpdate struct {
		id   idwrap.IDWrap
		prev *idwrap.IDWrap
		next *idwrap.IDWrap
	}
	
	ptrUpdates := make([]ptrUpdate, len(orderedIDs))
	for i, id := range orderedIDs {
		var prev, next *idwrap.IDWrap
		
		if i > 0 {
			prev = &orderedIDs[i-1]
		}
		if i < len(orderedIDs)-1 {
			next = &orderedIDs[i+1]
		}
		
		ptrUpdates[i] = ptrUpdate{
			id:   id,
			prev: prev,
			next: next,
		}
	}
	
	// Apply all updates atomically
	for _, update := range ptrUpdates {
		if err := repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
			Prev:        update.prev,
			Next:        update.next,
			ID:          update.id,
			WorkspaceID: workspaceID,
		}); err != nil {
			return fmt.Errorf("failed to update collection %s order: %w", update.id.String(), err)
		}
	}

	return nil
}

// GetMaxPosition returns the maximum position value for collections in a workspace
// For linked lists, this is the count of collections minus 1
func (r *CollectionMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	// For collections, parentID is the workspace_id
	orderedCollections, err := r.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   parentID,
		WorkspaceID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No collections means no max position
		}
		return 0, fmt.Errorf("failed to get collections in order: %w", err)
	}

	if len(orderedCollections) == 0 {
		return -1, nil
	}

	return len(orderedCollections) - 1, nil
}

// GetItemsByParent returns all collections under a workspace, ordered by position
func (r *CollectionMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	// For collections, parentID is the workspace_id
	orderedCollections, err := r.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   parentID,
		WorkspaceID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []movable.MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get collections in order: %w", err)
	}

	// Convert to MovableItem structs
	items := make([]movable.MovableItem, len(orderedCollections))
	for i, col := range orderedCollections {
		items[i] = movable.MovableItem{
			ID:       idwrap.NewFromBytesMust(col.ID),
			ParentID: &parentID, // workspace_id as parent
			Position: int(col.Position),
			ListType: listType,
		}
	}

	return items, nil
}

// insertAtPosition inserts a collection at a specific position in the linked list
// This is a helper method for operations that need to insert new collections
func (r *CollectionMovableRepository) insertAtPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, workspaceID idwrap.IDWrap, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get ordered list of collections in workspace
	orderedCollections, err := repo.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get collections in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedCollections) == 0 {
		// First collection in workspace
		newPrev = nil
		newNext = nil
	} else if position == 0 {
		// Insert at head
		newPrev = nil
		if len(orderedCollections) > 0 {
			nextID := idwrap.NewFromBytesMust(orderedCollections[0].ID)
			newNext = &nextID
			
			// Update the current head to point back to new item
			err = repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
				Prev:        &itemID,
				Next:        newNext,
				ID:          nextID,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedCollections) {
		// Insert at tail
		if len(orderedCollections) > 0 {
			prevID := idwrap.NewFromBytesMust(orderedCollections[len(orderedCollections)-1].ID)
			newPrev = &prevID
			
			// Update the current tail to point forward to new item
			err = repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
				Prev:        newPrev,
				Next:        &itemID,
				ID:          prevID,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current tail: %w", err)
			}
		}
		newNext = nil
	} else {
		// Insert in middle
		prevID := idwrap.NewFromBytesMust(orderedCollections[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedCollections[position].ID)
		newPrev = &prevID
		newNext = &nextID
		
		// Update prev item to point to new item
		err = repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
			Prev:        newPrev,
			Next:        &itemID,
			ID:          prevID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}
		
		// Update next item to point back to new item
		err = repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
			Prev:        &itemID,
			Next:        newNext,
			ID:          nextID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next item: %w", err)
		}
	}

	// Set the new item's position
	return repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
		Prev:        newPrev,
		Next:        newNext,
		ID:          itemID,
		WorkspaceID: workspaceID,
	})
}

// removeFromPosition removes a collection from its current position in the linked list
// This is a helper method for operations that need to remove collections
func (r *CollectionMovableRepository) removeFromPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the collection to remove
	collection, err := repo.queries.GetCollection(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get collection: %w", err)
	}

	// Get ordered collections to find prev and next
	orderedCollections, err := repo.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   collection.WorkspaceID,
		WorkspaceID_2: collection.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get collections in order: %w", err)
	}

	// Find the item and its neighbors
	var prevID, nextID *idwrap.IDWrap
	for i, col := range orderedCollections {
		if idwrap.NewFromBytesMust(col.ID).Compare(itemID) == 0 {
			if i > 0 {
				prev := idwrap.NewFromBytesMust(orderedCollections[i-1].ID)
				prevID = &prev
			}
			if i < len(orderedCollections)-1 {
				next := idwrap.NewFromBytesMust(orderedCollections[i+1].ID)
				nextID = &next
			}
			break
		}
	}

	// Update prev item's next pointer to skip the removed item
	if prevID != nil {
		// Get the prev item's current prev pointer to preserve it
		for _, col := range orderedCollections {
			if idwrap.NewFromBytesMust(col.ID).Compare(*prevID) == 0 {
				var currentPrev *idwrap.IDWrap
				if col.Prev != nil {
					prev := idwrap.NewFromBytesMust(col.Prev)
					currentPrev = &prev
				}
				
				err = repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
					Prev:        currentPrev, // Preserve the prev item's own prev pointer
					Next:        nextID,      // Point to the item after the removed one  
					ID:          *prevID,
					WorkspaceID: collection.WorkspaceID,
				})
				if err != nil {
					return fmt.Errorf("failed to update prev item: %w", err)
				}
				break
			}
		}
	}

	// Update next item's prev pointer to skip the removed item
	if nextID != nil {
		// Get the next item's current next pointer to preserve it
		for _, col := range orderedCollections {
			if idwrap.NewFromBytesMust(col.ID).Compare(*nextID) == 0 {
				var currentNext *idwrap.IDWrap
				if col.Next != nil {
					next := idwrap.NewFromBytesMust(col.Next)
					currentNext = &next
				}
				
				err = repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
					Prev:        prevID,      // Point to the item before the removed one
					Next:        currentNext, // Preserve the next item's own next pointer
					ID:          *nextID,
					WorkspaceID: collection.WorkspaceID,
				})
				if err != nil {
					return fmt.Errorf("failed to update next item: %w", err)
				}
				break
			}
		}
	}

	return nil
}