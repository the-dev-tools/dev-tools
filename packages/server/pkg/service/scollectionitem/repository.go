package scollectionitem

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
)

// CollectionItemsMovableRepository implements movable.MovableRepository for CollectionItems
// It handles mixed folder/endpoint ordering using the unified collection_items table with prev_id/next_id linked list
type CollectionItemsMovableRepository struct {
	queries *gen.Queries
}

// NewCollectionItemsMovableRepository creates a new CollectionItemsMovableRepository
func NewCollectionItemsMovableRepository(queries *gen.Queries) *CollectionItemsMovableRepository {
	return &CollectionItemsMovableRepository{
		queries: queries,
	}
}

// TX returns a new repository instance with transaction support
func (r *CollectionItemsMovableRepository) TX(tx *sql.Tx) *CollectionItemsMovableRepository {
    return &CollectionItemsMovableRepository{
        queries: r.queries.WithTx(tx),
    }
}

// Remove unlinks an item from its collection/folder chain
func (r *CollectionItemsMovableRepository) Remove(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
    // Unlink the item by stitching its neighbors together using only direct neighbor reads
    repo := r
    if tx != nil {
        repo = r.TX(tx)
    }

    // Load the item to get its neighbors
    item, err := repo.queries.GetCollectionItem(ctx, itemID)
    if err != nil {
        return fmt.Errorf("failed to get collection item: %w", err)
    }

    var prevID, nextID *idwrap.IDWrap
    if item.PrevID != nil { prevID = item.PrevID }
    if item.NextID != nil { nextID = item.NextID }

    // Update prev.next to point to next, preserving prev.prev
    if prevID != nil {
        prevRow, err := repo.queries.GetCollectionItem(ctx, *prevID)
        if err != nil {
            return fmt.Errorf("failed to get prev item: %w", err)
        }
        var prevPrev *idwrap.IDWrap
        if prevRow.PrevID != nil { prevPrev = prevRow.PrevID }
        if err := repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
            PrevID: prevPrev,
            NextID: nextID,
            ID:     *prevID,
        }); err != nil {
            return fmt.Errorf("failed to update prev item: %w", err)
        }
    }

    // Update next.prev to point to prev, preserving next.next
    if nextID != nil {
        nextRow, err := repo.queries.GetCollectionItem(ctx, *nextID)
        if err != nil {
            return fmt.Errorf("failed to get next item: %w", err)
        }
        var nextNext *idwrap.IDWrap
        if nextRow.NextID != nil { nextNext = nextRow.NextID }
        if err := repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
            PrevID: prevID,
            NextID: nextNext,
            ID:     *nextID,
        }); err != nil {
            return fmt.Errorf("failed to update next item: %w", err)
        }
    }

    // Isolate the item (optional before delete)
    if err := repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
        PrevID: nil,
        NextID: nil,
        ID:     itemID,
    }); err != nil {
        return fmt.Errorf("failed to isolate removed item: %w", err)
    }
    return nil
}

// UpdatePosition updates the position of a collection item in the linked list
// For collection items, parentID can be either collection_id (for root items) or parent_folder_id
func (r *CollectionItemsMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the collection item to understand its context
	item, err := repo.queries.GetCollectionItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get collection item: %w", err)
	}

	// Get ordered list of items in the same parent context
	orderedItems, err := repo.getOrderedItemsInSameParent(ctx, item)
	if err != nil {
		return fmt.Errorf("failed to get items in order: %w", err)
	}

	// Find current position and validate new position
	currentIdx := -1
	for i, orderItem := range orderedItems {
		if idwrap.NewFromBytesMust(orderItem.ID).Compare(itemID) == 0 {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("collection item not found in parent context")
	}

	if position < 0 || position >= len(orderedItems) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(orderedItems)-1)
	}

	if currentIdx == position {
		// No change needed
		return nil
	}

	// Calculate new prev and next pointers
	var newPrev, newNext *idwrap.IDWrap

	if position == 0 {
		// Moving to head position
		newPrev = nil
		if len(orderedItems) > 1 {
			nextID := idwrap.NewFromBytesMust(orderedItems[1].ID)
			newNext = &nextID
		}
	} else if position == len(orderedItems)-1 {
		// Moving to tail position
		prevID := idwrap.NewFromBytesMust(orderedItems[len(orderedItems)-2].ID)
		newPrev = &prevID
		newNext = nil
	} else {
		// Moving to middle position
		prevID := idwrap.NewFromBytesMust(orderedItems[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedItems[position+1].ID)
		newPrev = &prevID
		newNext = &nextID
	}

	// Update the collection item's position using SQLC generated query
	return repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
		PrevID: newPrev,
		NextID: newNext,
		ID:     itemID,
	})
}

// UpdatePositions updates positions for multiple collection items in batch
func (r *CollectionItemsMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the first item to establish parent context and validate all items are in same context
	firstItem, err := repo.queries.GetCollectionItem(ctx, updates[0].ItemID)
	if err != nil {
		return fmt.Errorf("failed to get first collection item: %w", err)
	}

	// Create ID position map and validate all items are in same parent context
	positionMap := make(map[idwrap.IDWrap]int)
	for _, update := range updates {
		item, err := repo.queries.GetCollectionItem(ctx, update.ItemID)
		if err != nil {
			return fmt.Errorf("failed to get collection item %s: %w", update.ItemID.String(), err)
		}

		// Validate items are in same parent context
		if !areInSameParentContext(firstItem, item) {
			return fmt.Errorf("all collection items must be in the same parent context")
		}

		positionMap[update.ItemID] = update.Position
	}

	// Build the complete ordered list with all items at their new positions
	orderedIDs := make([]idwrap.IDWrap, len(updates))
	for _, update := range updates {
		if update.Position < 0 || update.Position >= len(updates) {
			return fmt.Errorf("invalid position %d for collection item %s (valid range: 0-%d)",
				update.Position, update.ItemID.String(), len(updates)-1)
		}
		orderedIDs[update.Position] = update.ItemID
	}

	// Calculate prev/next pointers for each item in the new order
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
		if err := repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
			PrevID: update.prev,
			NextID: update.next,
			ID:     update.id,
		}); err != nil {
			return fmt.Errorf("failed to update collection item %s order: %w", update.id.String(), err)
		}
	}

	return nil
}

// GetMaxPosition returns the maximum position value for collection items in a parent context
func (r *CollectionItemsMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	// Determine the appropriate parent context based on listType
	var orderedItems []gen.GetCollectionItemsInOrderRow
	var err error

	switch listType {
	case movable.CollectionListTypeItems:
		// For mixed items, parentID could be collection_id or parent_folder_id
		// We need to get items where the parent matches the parentID
		orderedItems, err = r.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
			CollectionID:   parentID,
			ParentFolderID: nil, // Root level items
			Column3:        nil, // Same value for null check in SQL
			CollectionID_2: parentID,
		})
		if err != nil && err != sql.ErrNoRows {
			// Try as folder parent if collection root fails
			orderedItems, err = r.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
				CollectionID:   parentID, // This needs to be collection_id, but we need it from context
				ParentFolderID: &parentID,
				Column3:        &parentID, // Same value for null check in SQL
				CollectionID_2: parentID,
			})
		}
	default:
		return 0, fmt.Errorf("unsupported list type for collection items: %s", listType.String())
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No items means no max position
		}
		return 0, fmt.Errorf("failed to get collection items in order: %w", err)
	}

	if len(orderedItems) == 0 {
		return -1, nil
	}

	return len(orderedItems) - 1, nil
}

// GetItemsByParent returns all collection items under a parent, ordered by position
func (r *CollectionItemsMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	var orderedItems []gen.GetCollectionItemsInOrderRow
	var err error

	switch listType {
	case movable.CollectionListTypeItems:
		// For mixed items, try as root level first
		orderedItems, err = r.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
			CollectionID:   parentID,
			ParentFolderID: nil,
			Column3:        nil, // Same value for null check in SQL
			CollectionID_2: parentID,
		})
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get collection items in order: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported list type for collection items: %s", listType.String())
	}

	if err == sql.ErrNoRows || len(orderedItems) == 0 {
		return []movable.MovableItem{}, nil
	}

	// Convert to MovableItem structs
	items := make([]movable.MovableItem, len(orderedItems))
	for i, item := range orderedItems {
		items[i] = movable.MovableItem{
			ID:       idwrap.NewFromBytesMust(item.ID),
			ParentID: &parentID,
			Position: int(item.Position),
			ListType: listType,
		}
	}

	return items, nil
}

// Helper function to get ordered items in the same parent context as the given item
func (r *CollectionItemsMovableRepository) getOrderedItemsInSameParent(ctx context.Context, item gen.CollectionItem) ([]gen.GetCollectionItemsInOrderRow, error) {
	return r.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
		CollectionID:   item.CollectionID,
		ParentFolderID: item.ParentFolderID,
		Column3:        item.ParentFolderID, // Same value for null check in SQL
		CollectionID_2: item.CollectionID,
	})
}

// Helper function to check if two collection items are in the same parent context
func areInSameParentContext(item1, item2 gen.CollectionItem) bool {
	// Items are in same context if they have same collection_id and parent_folder_id
	if item1.CollectionID.Compare(item2.CollectionID) != 0 {
		return false
	}

	// Both have nil parent_folder_id (root level)
	if item1.ParentFolderID == nil && item2.ParentFolderID == nil {
		return true
	}

	// Both have same non-nil parent_folder_id
	if item1.ParentFolderID != nil && item2.ParentFolderID != nil {
		return item1.ParentFolderID.Compare(*item2.ParentFolderID) == 0
	}

	// One has nil, other doesn't
	return false
}

// CalculateInsertPosition calculates the correct prev/next IDs for inserting a new item at a position
// This method should be used BEFORE calling InsertCollectionItem to avoid linked list corruption
func (r *CollectionItemsMovableRepository) CalculateInsertPosition(ctx context.Context, collectionID idwrap.IDWrap, parentFolderID *idwrap.IDWrap, position int) (*idwrap.IDWrap, *idwrap.IDWrap, error) {
	// Get ordered list of items in the same parent context
	orderedItems, err := r.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
		CollectionID:   collectionID,
		ParentFolderID: parentFolderID,
		Column3:        parentFolderID, // Same value for null check in SQL
		CollectionID_2: collectionID,
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("failed to get collection items in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedItems) == 0 {
		// First item in this parent context
		newPrev = nil
		newNext = nil
	} else if position <= 0 {
		// Insert at head
		newPrev = nil
		if len(orderedItems) > 0 {
			nextID := idwrap.NewFromBytesMust(orderedItems[0].ID)
			newNext = &nextID
		}
	} else if position >= len(orderedItems) {
		// Insert at tail
		if len(orderedItems) > 0 {
			prevID := idwrap.NewFromBytesMust(orderedItems[len(orderedItems)-1].ID)
			newPrev = &prevID
		}
		newNext = nil
	} else {
		// Insert in middle
		prevID := idwrap.NewFromBytesMust(orderedItems[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedItems[position].ID)
		newPrev = &prevID
		newNext = &nextID
	}

	return newPrev, newNext, nil
}

// InsertNewItemAtPosition inserts a new collection item directly at the specified position
// This is the correct method to use for new items (replaces InsertCollectionItem + InsertAtPosition pattern)
func (r *CollectionItemsMovableRepository) InsertNewItemAtPosition(ctx context.Context, tx *sql.Tx, params gen.InsertCollectionItemParams, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Calculate correct prev/next positions
	newPrev, newNext, err := repo.CalculateInsertPosition(ctx, params.CollectionID, params.ParentFolderID, position)
	if err != nil {
		return fmt.Errorf("failed to calculate insert position: %w", err)
	}

	// Set the calculated prev/next values
	params.PrevID = newPrev
	params.NextID = newNext

	// Insert the item with correct positioning
	err = repo.queries.InsertCollectionItem(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert collection item: %w", err)
	}

	// Now update existing items to link to the new item
	if newPrev != nil {
		// Get the previous item to preserve its PrevID
		prevItem, err := repo.queries.GetCollectionItem(ctx, *newPrev)
		if err != nil {
			return fmt.Errorf("failed to get previous item: %w", err)
		}
		
		// Update the previous item to point to the new item
		err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
			PrevID: prevItem.PrevID, // Keep existing PrevID
			NextID: &params.ID,      // Point to new item
			ID:     *newPrev,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous item: %w", err)
		}
	}

	if newNext != nil {
		// Get the next item to preserve its NextID
		nextItem, err := repo.queries.GetCollectionItem(ctx, *newNext)
		if err != nil {
			return fmt.Errorf("failed to get next item: %w", err)
		}
		
		// Update the next item to point back to the new item
		err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
			PrevID: &params.ID,      // Point back to new item
			NextID: nextItem.NextID, // Keep existing NextID
			ID:     *newNext,
		})
		if err != nil {
			return fmt.Errorf("failed to update next item: %w", err)
		}
	}

	return nil
}

// DEPRECATED: InsertAtPosition should not be used for new items - use InsertNewItemAtPosition instead
// This method is kept for backward compatibility but should be avoided for new item insertion
// InsertAtPosition inserts a collection item at a specific position in the linked list
// This is a helper method for operations that need to insert new items
func (r *CollectionItemsMovableRepository) InsertAtPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, collectionID idwrap.IDWrap, parentFolderID *idwrap.IDWrap, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get ordered list of items in the same parent context
	orderedItems, err := repo.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
		CollectionID:   collectionID,
		ParentFolderID: parentFolderID,
		Column3:        parentFolderID, // Same value for null check in SQL
		CollectionID_2: collectionID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get collection items in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedItems) == 0 {
		// First item in this parent context
		newPrev = nil
		newNext = nil
	} else if position == 0 {
		// Insert at head
		newPrev = nil
		if len(orderedItems) > 0 {
			nextID := idwrap.NewFromBytesMust(orderedItems[0].ID)
			newNext = &nextID

			// Update the current head to point back to new item
			// Keep the current head's existing NextID
			currentHeadNextID := convertBytesToIDWrap(orderedItems[0].NextID)
			err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
				PrevID: &itemID,
				NextID: currentHeadNextID,
				ID:     nextID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedItems) {
		// Insert at tail
		if len(orderedItems) > 0 {
			prevID := idwrap.NewFromBytesMust(orderedItems[len(orderedItems)-1].ID)
			newPrev = &prevID

			// Update the current tail to point forward to new item
			// Keep the current tail's existing PrevID
			currentTailPrevID := convertBytesToIDWrap(orderedItems[len(orderedItems)-1].PrevID)
			err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
				PrevID: currentTailPrevID,
				NextID: &itemID,
				ID:     prevID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current tail: %w", err)
			}
		}
		newNext = nil
	} else {
		// Insert in middle
		prevID := idwrap.NewFromBytesMust(orderedItems[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedItems[position].ID)
		newPrev = &prevID
		newNext = &nextID

		// Update prev item to point to new item
		// Keep the prev item's existing PrevID
		prevItemPrevID := convertBytesToIDWrap(orderedItems[position-1].PrevID)
		err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
			PrevID: prevItemPrevID,
			NextID: &itemID,
			ID:     prevID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}

		// Update next item to point back to new item
		// Keep the next item's existing NextID
		nextItemNextID := convertBytesToIDWrap(orderedItems[position].NextID)
		err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
			PrevID: &itemID,
			NextID: nextItemNextID,
			ID:     nextID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next item: %w", err)
		}
	}

	// Set the new item's position (this assumes the item was already inserted via InsertCollectionItem)
	return repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
		PrevID: newPrev,
		NextID: newNext,
		ID:     itemID,
	})
}

// RemoveFromPosition removes a collection item from its current position in the linked list
// This is a helper method for operations that need to remove items
func (r *CollectionItemsMovableRepository) RemoveFromPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the collection item to remove
	item, err := repo.queries.GetCollectionItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get collection item: %w", err)
	}

	// Get ordered items to find prev and next
	orderedItems, err := repo.getOrderedItemsInSameParent(ctx, item)
	if err != nil {
		return fmt.Errorf("failed to get collection items in order: %w", err)
	}

	// Find the item and its neighbors
	var prevID, nextID *idwrap.IDWrap
	for i, orderItem := range orderedItems {
		if idwrap.NewFromBytesMust(orderItem.ID).Compare(itemID) == 0 {
			if i > 0 {
				prev := idwrap.NewFromBytesMust(orderedItems[i-1].ID)
				prevID = &prev
			}
			if i < len(orderedItems)-1 {
				next := idwrap.NewFromBytesMust(orderedItems[i+1].ID)
				nextID = &next
			}
			break
		}
	}

	// Update prev item's next pointer to skip the removed item
	if prevID != nil {
		// Get the prev item's current prev pointer to preserve it
		for _, orderItem := range orderedItems {
			if idwrap.NewFromBytesMust(orderItem.ID).Compare(*prevID) == 0 {
				var currentPrev *idwrap.IDWrap
				if len(orderItem.PrevID) > 0 {
					prev := idwrap.NewFromBytesMust(orderItem.PrevID)
					currentPrev = &prev
				}

				err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
					PrevID: currentPrev, // Preserve the prev item's own prev pointer
					NextID: nextID,      // Point to the item after the removed one
					ID:     *prevID,
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
		for _, orderItem := range orderedItems {
			if idwrap.NewFromBytesMust(orderItem.ID).Compare(*nextID) == 0 {
				var currentNext *idwrap.IDWrap
				if len(orderItem.NextID) > 0 {
					next := idwrap.NewFromBytesMust(orderItem.NextID)
					currentNext = &next
				}

				err = repo.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
					PrevID: prevID,      // Point to the item before the removed one
					NextID: currentNext, // Preserve the next item's own next pointer
					ID:     *nextID,
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
