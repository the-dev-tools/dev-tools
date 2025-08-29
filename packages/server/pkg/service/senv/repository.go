package senv

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
)

// EnvironmentMovableRepository implements movable.MovableRepository for Environments
// It adapts position-based operations to linked list operations using prev/next pointers
type EnvironmentMovableRepository struct {
	queries *gen.Queries
}

// NewEnvironmentMovableRepository creates a new EnvironmentMovableRepository
func NewEnvironmentMovableRepository(queries *gen.Queries) *EnvironmentMovableRepository {
	return &EnvironmentMovableRepository{
		queries: queries,
	}
}

// TX returns a new repository instance with transaction support
func (r *EnvironmentMovableRepository) TX(tx *sql.Tx) *EnvironmentMovableRepository {
	return &EnvironmentMovableRepository{
		queries: r.queries.WithTx(tx),
	}
}

// UpdatePosition updates the position of an environment in the linked list
// For environments, parentID is the workspace_id and listType is ignored
func (r *EnvironmentMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get environment to find workspace_id
	environment, err := repo.queries.GetEnvironment(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Get ordered list of environments in workspace
	orderedEnvironments, err := repo.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, gen.GetEnvironmentsByWorkspaceIDOrderedParams{
		WorkspaceID:   environment.WorkspaceID,
		WorkspaceID_2: environment.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get environments in order: %w", err)
	}

	// Find current position and validate new position
	currentIdx := -1
	for i, env := range orderedEnvironments {
		if idwrap.NewFromBytesMust(env.ID).Compare(itemID) == 0 {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("environment not found in workspace")
	}

	if position < 0 || position >= len(orderedEnvironments) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(orderedEnvironments)-1)
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
		if len(orderedEnvironments) > 1 {
			nextID := idwrap.NewFromBytesMust(orderedEnvironments[1].ID)
			newNext = &nextID
		}
	} else if position == len(orderedEnvironments)-1 {
		// Moving to tail position  
		prevID := idwrap.NewFromBytesMust(orderedEnvironments[len(orderedEnvironments)-2].ID)
		newPrev = &prevID
		newNext = nil
	} else {
		// Moving to middle position
		prevID := idwrap.NewFromBytesMust(orderedEnvironments[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedEnvironments[position+1].ID)
		newPrev = &prevID
		newNext = &nextID
	}

	// Update the environment's position
	return repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
		Prev:        newPrev,
		Next:        newNext,
		ID:          itemID,
		WorkspaceID: environment.WorkspaceID,
	})
}

// UpdatePositions updates positions for multiple environments in batch
func (r *EnvironmentMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the workspace ID from the first environment to validate all are in same workspace
	firstEnvironment, err := repo.queries.GetEnvironment(ctx, updates[0].ItemID)
	if err != nil {
		return fmt.Errorf("failed to get first environment: %w", err)
	}
	workspaceID := firstEnvironment.WorkspaceID
	
	// Validate all environments are in the same workspace and create ID position map
	positionMap := make(map[idwrap.IDWrap]int)
	for _, update := range updates {
		environment, err := repo.queries.GetEnvironment(ctx, update.ItemID)
		if err != nil {
			return fmt.Errorf("failed to get environment %s: %w", update.ItemID.String(), err)
		}
		if environment.WorkspaceID.Compare(workspaceID) != 0 {
			return fmt.Errorf("all environments must be in the same workspace")
		}
		positionMap[update.ItemID] = update.Position
	}
	
	// Build the complete ordered list with all environments at their new positions
	orderedIDs := make([]idwrap.IDWrap, len(updates))
	for _, update := range updates {
		if update.Position < 0 || update.Position >= len(updates) {
			return fmt.Errorf("invalid position %d for environment %s (valid range: 0-%d)", 
				update.Position, update.ItemID.String(), len(updates)-1)
		}
		orderedIDs[update.Position] = update.ItemID
	}
	
	// Calculate prev/next pointers for each environment in the new order
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
		if err := repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
			Prev:        update.prev,
			Next:        update.next,
			ID:          update.id,
			WorkspaceID: workspaceID,
		}); err != nil {
			return fmt.Errorf("failed to update environment %s order: %w", update.id.String(), err)
		}
	}

	return nil
}

// GetMaxPosition returns the maximum position value for environments in a workspace
// For linked lists, this is the count of environments minus 1
func (r *EnvironmentMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	// For environments, parentID is the workspace_id
	orderedEnvironments, err := r.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, gen.GetEnvironmentsByWorkspaceIDOrderedParams{
		WorkspaceID:   parentID,
		WorkspaceID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No environments means no max position
		}
		return 0, fmt.Errorf("failed to get environments in order: %w", err)
	}

	if len(orderedEnvironments) == 0 {
		return -1, nil
	}

	return len(orderedEnvironments) - 1, nil
}

// GetItemsByParent returns all environments under a workspace, ordered by position
func (r *EnvironmentMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	// For environments, parentID is the workspace_id
	orderedEnvironments, err := r.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, gen.GetEnvironmentsByWorkspaceIDOrderedParams{
		WorkspaceID:   parentID,
		WorkspaceID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []movable.MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get environments in order: %w", err)
	}

	// Convert to MovableItem structs
	items := make([]movable.MovableItem, len(orderedEnvironments))
	for i, env := range orderedEnvironments {
		items[i] = movable.MovableItem{
			ID:       idwrap.NewFromBytesMust(env.ID),
			ParentID: &parentID, // workspace_id as parent
			Position: int(env.Position),
			ListType: listType,
		}
	}

	return items, nil
}

// insertAtPosition inserts an environment at a specific position in the linked list
// This is a helper method for operations that need to insert new environments
func (r *EnvironmentMovableRepository) insertAtPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, workspaceID idwrap.IDWrap, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get ordered list of environments in workspace
	orderedEnvironments, err := repo.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, gen.GetEnvironmentsByWorkspaceIDOrderedParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get environments in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedEnvironments) == 0 {
		// First environment in workspace
		newPrev = nil
		newNext = nil
	} else if position == 0 {
		// Insert at head
		newPrev = nil
		if len(orderedEnvironments) > 0 {
			nextID := idwrap.NewFromBytesMust(orderedEnvironments[0].ID)
			newNext = &nextID
			
			// Update the current head to point back to new item
			err = repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
				Prev:        &itemID,
				Next:        newNext,
				ID:          nextID,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedEnvironments) {
		// Insert at tail
		if len(orderedEnvironments) > 0 {
			prevID := idwrap.NewFromBytesMust(orderedEnvironments[len(orderedEnvironments)-1].ID)
			newPrev = &prevID
			
			// Update the current tail to point forward to new item
			err = repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
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
		prevID := idwrap.NewFromBytesMust(orderedEnvironments[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedEnvironments[position].ID)
		newPrev = &prevID
		newNext = &nextID
		
		// Update prev item to point to new item
		err = repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
			Prev:        newPrev,
			Next:        &itemID,
			ID:          prevID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}
		
		// Update next item to point back to new item
		err = repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
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
	return repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
		Prev:        newPrev,
		Next:        newNext,
		ID:          itemID,
		WorkspaceID: workspaceID,
	})
}

// removeFromPosition removes an environment from its current position in the linked list
// This is a helper method for operations that need to remove environments
func (r *EnvironmentMovableRepository) removeFromPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the environment to remove
	environment, err := repo.queries.GetEnvironment(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Get ordered environments to find prev and next
	orderedEnvironments, err := repo.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, gen.GetEnvironmentsByWorkspaceIDOrderedParams{
		WorkspaceID:   environment.WorkspaceID,
		WorkspaceID_2: environment.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get environments in order: %w", err)
	}

	// Find the item and its neighbors
	var prevID, nextID *idwrap.IDWrap
	for i, env := range orderedEnvironments {
		if idwrap.NewFromBytesMust(env.ID).Compare(itemID) == 0 {
			if i > 0 {
				prev := idwrap.NewFromBytesMust(orderedEnvironments[i-1].ID)
				prevID = &prev
			}
			if i < len(orderedEnvironments)-1 {
				next := idwrap.NewFromBytesMust(orderedEnvironments[i+1].ID)
				nextID = &next
			}
			break
		}
	}

	// Update prev item's next pointer to skip the removed item
	if prevID != nil {
		// Get the prev item's current prev pointer to preserve it
		for _, env := range orderedEnvironments {
			if idwrap.NewFromBytesMust(env.ID).Compare(*prevID) == 0 {
				var currentPrev *idwrap.IDWrap
				if env.Prev != nil {
					prev := idwrap.NewFromBytesMust(env.Prev)
					currentPrev = &prev
				}
				
				err = repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
					Prev:        currentPrev, // Preserve the prev item's own prev pointer
					Next:        nextID,      // Point to the item after the removed one  
					ID:          *prevID,
					WorkspaceID: environment.WorkspaceID,
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
		for _, env := range orderedEnvironments {
			if idwrap.NewFromBytesMust(env.ID).Compare(*nextID) == 0 {
				var currentNext *idwrap.IDWrap
				if env.Next != nil {
					next := idwrap.NewFromBytesMust(env.Next)
					currentNext = &next
				}
				
				err = repo.queries.UpdateEnvironmentOrder(ctx, gen.UpdateEnvironmentOrderParams{
					Prev:        prevID,      // Point to the item before the removed one
					Next:        currentNext, // Preserve the next item's own next pointer
					ID:          *nextID,
					WorkspaceID: environment.WorkspaceID,
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