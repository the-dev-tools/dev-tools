package svar

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
)

// VariableMovableRepository implements movable.MovableRepository for Variables
// It adapts position-based operations to linked list operations using prev/next pointers
type VariableMovableRepository struct {
	queries *gen.Queries
}

// NewVariableMovableRepository creates a new VariableMovableRepository
func NewVariableMovableRepository(queries *gen.Queries) *VariableMovableRepository {
	return &VariableMovableRepository{
		queries: queries,
	}
}

// TX returns a new repository instance with transaction support
func (r *VariableMovableRepository) TX(tx *sql.Tx) *VariableMovableRepository {
	return &VariableMovableRepository{
		queries: r.queries.WithTx(tx),
	}
}

// UpdatePosition updates the position of a variable in the linked list
// For variables, parentID is the env_id and listType is ignored
func (r *VariableMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get variable to find env_id
	variable, err := repo.queries.GetVariable(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get variable: %w", err)
	}

	// Get ordered list of variables in environment
	orderedVariables, err := repo.queries.GetVariablesByEnvironmentIDOrdered(ctx, gen.GetVariablesByEnvironmentIDOrderedParams{
		EnvID:   variable.EnvID,
		EnvID_2: variable.EnvID,
	})
	if err != nil {
		return fmt.Errorf("failed to get variables in order: %w", err)
	}

	// Find current position and validate new position
	currentIdx := -1
	for i, v := range orderedVariables {
		if idwrap.NewFromBytesMust(v.ID).Compare(itemID) == 0 {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("variable not found in environment")
	}

	if position < 0 || position >= len(orderedVariables) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(orderedVariables)-1)
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
		if len(orderedVariables) > 1 {
			nextID := idwrap.NewFromBytesMust(orderedVariables[1].ID)
			newNext = &nextID
		}
	} else if position == len(orderedVariables)-1 {
		// Moving to tail position  
		prevID := idwrap.NewFromBytesMust(orderedVariables[len(orderedVariables)-2].ID)
		newPrev = &prevID
		newNext = nil
	} else {
		// Moving to middle position
		prevID := idwrap.NewFromBytesMust(orderedVariables[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedVariables[position+1].ID)
		newPrev = &prevID
		newNext = &nextID
	}

	// Update the variable's position
	return repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
		Prev:  newPrev,
		Next:  newNext,
		ID:    itemID,
		EnvID: variable.EnvID,
	})
}

// UpdatePositions updates positions for multiple variables in batch
func (r *VariableMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the environment ID from the first variable to validate all are in same environment
	firstVariable, err := repo.queries.GetVariable(ctx, updates[0].ItemID)
	if err != nil {
		return fmt.Errorf("failed to get first variable: %w", err)
	}
	envID := firstVariable.EnvID
	
	// Validate all variables are in the same environment and create ID position map
	positionMap := make(map[idwrap.IDWrap]int)
	for _, update := range updates {
		variable, err := repo.queries.GetVariable(ctx, update.ItemID)
		if err != nil {
			return fmt.Errorf("failed to get variable %s: %w", update.ItemID.String(), err)
		}
		if variable.EnvID.Compare(envID) != 0 {
			return fmt.Errorf("all variables must be in the same environment")
		}
		positionMap[update.ItemID] = update.Position
	}
	
	// Build the complete ordered list with all variables at their new positions
	orderedIDs := make([]idwrap.IDWrap, len(updates))
	for _, update := range updates {
		if update.Position < 0 || update.Position >= len(updates) {
			return fmt.Errorf("invalid position %d for variable %s (valid range: 0-%d)", 
				update.Position, update.ItemID.String(), len(updates)-1)
		}
		orderedIDs[update.Position] = update.ItemID
	}
	
	// Calculate prev/next pointers for each variable in the new order
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
		if err := repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
			Prev:  update.prev,
			Next:  update.next,
			ID:    update.id,
			EnvID: envID,
		}); err != nil {
			return fmt.Errorf("failed to update variable %s order: %w", update.id.String(), err)
		}
	}

	return nil
}

// GetMaxPosition returns the maximum position value for variables in an environment
// For linked lists, this is the count of variables minus 1
func (r *VariableMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	// For variables, parentID is the env_id
	orderedVariables, err := r.queries.GetVariablesByEnvironmentIDOrdered(ctx, gen.GetVariablesByEnvironmentIDOrderedParams{
		EnvID:   parentID,
		EnvID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No variables means no max position
		}
		return 0, fmt.Errorf("failed to get variables in order: %w", err)
	}

	if len(orderedVariables) == 0 {
		return -1, nil
	}

	return len(orderedVariables) - 1, nil
}

// GetItemsByParent returns all variables under an environment, ordered by position
func (r *VariableMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	// For variables, parentID is the env_id
	orderedVariables, err := r.queries.GetVariablesByEnvironmentIDOrdered(ctx, gen.GetVariablesByEnvironmentIDOrderedParams{
		EnvID:   parentID,
		EnvID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []movable.MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get variables in order: %w", err)
	}

	// Convert to MovableItem structs
	items := make([]movable.MovableItem, len(orderedVariables))
	for i, v := range orderedVariables {
		items[i] = movable.MovableItem{
			ID:       idwrap.NewFromBytesMust(v.ID),
			ParentID: &parentID, // env_id as parent
			Position: int(v.Position),
			ListType: listType,
		}
	}

	return items, nil
}

// insertAtPosition inserts a variable at a specific position in the linked list
// This is a helper method for operations that need to insert new variables
func (r *VariableMovableRepository) insertAtPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, envID idwrap.IDWrap, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get ordered list of variables in environment
	orderedVariables, err := repo.queries.GetVariablesByEnvironmentIDOrdered(ctx, gen.GetVariablesByEnvironmentIDOrderedParams{
		EnvID:   envID,
		EnvID_2: envID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get variables in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedVariables) == 0 {
		// First variable in environment
		newPrev = nil
		newNext = nil
	} else if position == 0 {
		// Insert at head
		newPrev = nil
		if len(orderedVariables) > 0 {
			nextID := idwrap.NewFromBytesMust(orderedVariables[0].ID)
			newNext = &nextID
			
			// Update the current head to point back to new item
			err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
				Prev:  &itemID,
				Next:  newNext,
				ID:    nextID,
				EnvID: envID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedVariables) {
		// Insert at tail
		if len(orderedVariables) > 0 {
			prevID := idwrap.NewFromBytesMust(orderedVariables[len(orderedVariables)-1].ID)
			newPrev = &prevID
			
			// Update the current tail to point forward to new item
			err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
				Prev:  newPrev,
				Next:  &itemID,
				ID:    prevID,
				EnvID: envID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current tail: %w", err)
			}
		}
		newNext = nil
	} else {
		// Insert in middle
		prevID := idwrap.NewFromBytesMust(orderedVariables[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedVariables[position].ID)
		newPrev = &prevID
		newNext = &nextID
		
		// Update prev item to point to new item
		err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
			Prev:  newPrev,
			Next:  &itemID,
			ID:    prevID,
			EnvID: envID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}
		
		// Update next item to point back to new item
		err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
			Prev:  &itemID,
			Next:  newNext,
			ID:    nextID,
			EnvID: envID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next item: %w", err)
		}
	}

	// Set the new item's position
	return repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
		Prev:  newPrev,
		Next:  newNext,
		ID:    itemID,
		EnvID: envID,
	})
}

// removeFromPosition removes a variable from its current position in the linked list
// This is a helper method for operations that need to remove variables
func (r *VariableMovableRepository) removeFromPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the variable to remove
	variable, err := repo.queries.GetVariable(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get variable: %w", err)
	}

	// Get ordered variables to find prev and next
	orderedVariables, err := repo.queries.GetVariablesByEnvironmentIDOrdered(ctx, gen.GetVariablesByEnvironmentIDOrderedParams{
		EnvID:   variable.EnvID,
		EnvID_2: variable.EnvID,
	})
	if err != nil {
		return fmt.Errorf("failed to get variables in order: %w", err)
	}

	// Find the item and its neighbors
	var prevID, nextID *idwrap.IDWrap
	for i, v := range orderedVariables {
		if idwrap.NewFromBytesMust(v.ID).Compare(itemID) == 0 {
			if i > 0 {
				prev := idwrap.NewFromBytesMust(orderedVariables[i-1].ID)
				prevID = &prev
			}
			if i < len(orderedVariables)-1 {
				next := idwrap.NewFromBytesMust(orderedVariables[i+1].ID)
				nextID = &next
			}
			break
		}
	}

	// Update prev item's next pointer to skip the removed item
	if prevID != nil {
		// Get the prev item's current prev pointer to preserve it
		for _, v := range orderedVariables {
			if idwrap.NewFromBytesMust(v.ID).Compare(*prevID) == 0 {
				var currentPrev *idwrap.IDWrap
				if v.Prev != nil {
					prev := idwrap.NewFromBytesMust(v.Prev)
					currentPrev = &prev
				}
				
				err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
					Prev:  currentPrev, // Preserve the prev item's own prev pointer
					Next:  nextID,      // Point to the item after the removed one  
					ID:    *prevID,
					EnvID: variable.EnvID,
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
		for _, v := range orderedVariables {
			if idwrap.NewFromBytesMust(v.ID).Compare(*nextID) == 0 {
				var currentNext *idwrap.IDWrap
				if v.Next != nil {
					next := idwrap.NewFromBytesMust(v.Next)
					currentNext = &next
				}
				
				err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
					Prev:  prevID,      // Point to the item before the removed one
					Next:  currentNext, // Preserve the next item's own next pointer
					ID:    *nextID,
					EnvID: variable.EnvID,
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