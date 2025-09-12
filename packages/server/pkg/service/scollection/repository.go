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

// Remove unlinks the collection from its neighbors in the workspace-scoped chain
func (r *CollectionMovableRepository) Remove(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
    return r.removeFromPosition(ctx, tx, itemID)
}

// UpdatePosition updates the position of a collection in the linked list
// For collections, parentID is the workspace_id and listType is ignored
func (r *CollectionMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
    // Get repository with transaction support
    repo := r
    if tx != nil {
        repo = r.TX(tx)
    }

    // Get collection to find workspace_id and current order
    collection, err := repo.queries.GetCollection(ctx, itemID)
    if err != nil {
        return fmt.Errorf("failed to get collection: %w", err)
    }
    ordered, err := repo.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
        WorkspaceID:   collection.WorkspaceID,
        WorkspaceID_2: collection.WorkspaceID,
    })
    if err != nil {
        return fmt.Errorf("failed to get collections in order: %w", err)
    }
    if len(ordered) == 0 {
        return nil
    }
    if position < 0 || position >= len(ordered) {
        return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(ordered)-1)
    }

    // Build final order by moving item to requested position
    cur := make([]idwrap.IDWrap, 0, len(ordered))
    idx := -1
    for i, row := range ordered {
        id := idwrap.NewFromBytesMust(row.ID)
        if id.Compare(itemID) == 0 { idx = i; continue }
        cur = append(cur, id)
    }
    if idx == -1 { return fmt.Errorf("collection not found in workspace") }
    if position > len(cur) { position = len(cur) }
    newOrder := append(cur[:position], append([]idwrap.IDWrap{itemID}, cur[position:]...)...)

    // Two-phase relink to avoid uniqueness conflicts on (prev,next,workspace)
    for _, id := range newOrder {
        if err := repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{Prev: nil, Next: nil, ID: id, WorkspaceID: collection.WorkspaceID}); err != nil {
            return fmt.Errorf("detach failed for %s: %w", id.String(), err)
        }
    }
    for i, id := range newOrder {
        var prev, next *idwrap.IDWrap
        if i > 0 { prev = &newOrder[i-1] }
        if i < len(newOrder)-1 { next = &newOrder[i+1] }
        if err := repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{Prev: prev, Next: next, ID: id, WorkspaceID: collection.WorkspaceID}); err != nil {
            return fmt.Errorf("relink failed for %s: %w", id.String(), err)
        }
    }
    return nil
}

// UpdatePositions updates positions for multiple collections in batch
func (r *CollectionMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
    if len(updates) == 0 { return nil }

    // Get repository with transaction support
    repo := r
    if tx != nil { repo = r.TX(tx) }

    // Determine workspace from the first update and load current order
    firstCollection, err := repo.queries.GetCollection(ctx, updates[0].ItemID)
    if err != nil { return fmt.Errorf("failed to get first collection: %w", err) }
    workspaceID := firstCollection.WorkspaceID
    current, err := repo.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{WorkspaceID: workspaceID, WorkspaceID_2: workspaceID})
    if err != nil { return fmt.Errorf("failed to get collections in order: %w", err) }
    if len(current) == 0 { return nil }

    // Validate and build map id->desired position
    desired := make(map[idwrap.IDWrap]int, len(updates))
    for _, u := range updates {
        col, err := repo.queries.GetCollection(ctx, u.ItemID)
        if err != nil { return fmt.Errorf("failed to get collection %s: %w", u.ItemID.String(), err) }
        if col.WorkspaceID.Compare(workspaceID) != 0 { return fmt.Errorf("all collections must be in the same workspace") }
        if u.Position < 0 || u.Position >= len(current) { return fmt.Errorf("invalid position %d for collection %s (valid range: 0-%d)", u.Position, u.ItemID.String(), len(current)-1) }
        if _, ok := desired[u.ItemID]; ok { return fmt.Errorf("duplicate update for %s", u.ItemID.String()) }
        desired[u.ItemID] = u.Position
    }

    // Build final order
    final := make([]idwrap.IDWrap, len(current))
    occupied := make([]bool, len(current))
    for id, pos := range desired {
        if occupied[pos] { return fmt.Errorf("conflicting updates for position %d", pos) }
        final[pos] = id
        occupied[pos] = true
    }
    // Fill remaining positions preserving current relative order
    idx := 0
    for _, row := range current {
        id := idwrap.NewFromBytesMust(row.ID)
        if _, isUpdated := desired[id]; isUpdated { continue }
        for idx < len(final) && occupied[idx] { idx++ }
        if idx >= len(final) { break }
        final[idx] = id
        occupied[idx] = true
        idx++
    }

    // Two-phase relink for the workspace
    for _, id := range final {
        if err := repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{Prev: nil, Next: nil, ID: id, WorkspaceID: workspaceID}); err != nil {
            return fmt.Errorf("detach failed for %s: %w", id.String(), err)
        }
    }
    for i, id := range final {
        var prev, next *idwrap.IDWrap
        if i > 0 { prev = &final[i-1] }
        if i < len(final)-1 { next = &final[i+1] }
        if err := repo.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{Prev: prev, Next: next, ID: id, WorkspaceID: workspaceID}); err != nil {
            return fmt.Errorf("relink failed for %s: %w", id.String(), err)
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
