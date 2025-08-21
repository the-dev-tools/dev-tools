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

	// FIXED: Use proper remove/insert pattern to maintain linked list integrity
	// Step 1: Remove the variable from its current position
	if err := repo.removeFromPosition(ctx, tx, itemID); err != nil {
		return fmt.Errorf("failed to remove variable from current position: %w", err)
	}

	// Step 2: Calculate the target position after removal
	// When we remove an item, positions of items after it shift down by 1
	targetPosition := position
	if currentIdx < position {
		targetPosition = position - 1
	}
	
	// Special case: if we're moving to the last position in the original list,
	// we want to append to the end of the reduced list
	if position == len(orderedVariables)-1 {
		targetPosition = len(orderedVariables) // This will trigger append to end in insertAtPosition
	}

	// Step 3: Insert the variable at the new position
	if err := repo.insertAtPosition(ctx, tx, itemID, variable.EnvID, targetPosition); err != nil {
		return fmt.Errorf("failed to insert variable at new position: %w", err)
	}

	return nil
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
			currentHeadID := idwrap.NewFromBytesMust(orderedVariables[0].ID)
			newNext = &currentHeadID
			
			// Get the current head's next pointer to preserve it
			currentHead, err := repo.queries.GetVariable(ctx, currentHeadID)
			if err != nil {
				return fmt.Errorf("failed to get current head: %w", err)
			}
			
			// Update the current head to point back to new item
			err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
				Prev:  &itemID,        // Current head's prev now points to new item
				Next:  currentHead.Next, // Preserve current head's next pointer
				ID:    currentHeadID,
				EnvID: envID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedVariables) {
		// Insert at tail
		if len(orderedVariables) > 0 {
			currentTailID := idwrap.NewFromBytesMust(orderedVariables[len(orderedVariables)-1].ID)
			newPrev = &currentTailID
			
			// Get the current tail's prev pointer to preserve it
			currentTail, err := repo.queries.GetVariable(ctx, currentTailID)
			if err != nil {
				return fmt.Errorf("failed to get current tail: %w", err)
			}
			
			// Update the current tail to point forward to new item
			err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
				Prev:  currentTail.Prev, // Preserve current tail's prev pointer
				Next:  &itemID,         // Current tail's next now points to new item
				ID:    currentTailID,
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
		
		// Get existing prev and next items to preserve their other pointers
		prevItem, err := repo.queries.GetVariable(ctx, prevID)
		if err != nil {
			return fmt.Errorf("failed to get prev item: %w", err)
		}
		
		nextItem, err := repo.queries.GetVariable(ctx, nextID)
		if err != nil {
			return fmt.Errorf("failed to get next item: %w", err)
		}
		
		// Update prev item to point to new item
		err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
			Prev:  prevItem.Prev, // Preserve prev item's own prev pointer
			Next:  &itemID,       // Point to new item
			ID:    prevID,
			EnvID: envID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}
		
		// Update next item to point back to new item
		err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
			Prev:  &itemID,       // Point to new item
			Next:  nextItem.Next, // Preserve next item's own next pointer
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

	// Get the variable to remove and its current prev/next pointers
	variable, err := repo.queries.GetVariable(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get variable: %w", err)
	}

	// Update prev variable's next pointer to skip the removed variable
	if variable.Prev != nil {
		err = repo.queries.UpdateVariableNext(ctx, gen.UpdateVariableNextParams{
			Next:  variable.Next, // Point to the removed variable's next
			ID:    *variable.Prev,
			EnvID: variable.EnvID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous variable's next pointer: %w", err)
		}
	}

	// Update next variable's prev pointer to skip the removed variable
	if variable.Next != nil {
		err = repo.queries.UpdateVariablePrev(ctx, gen.UpdateVariablePrevParams{
			Prev:  variable.Prev, // Point to the removed variable's prev
			ID:    *variable.Next,
			EnvID: variable.EnvID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next variable's prev pointer: %w", err)
		}
	}

	// Clear the removed variable's pointers
	err = repo.queries.UpdateVariableOrder(ctx, gen.UpdateVariableOrderParams{
		Prev:  nil,
		Next:  nil,
		ID:    itemID,
		EnvID: variable.EnvID,
	})
	if err != nil {
		return fmt.Errorf("failed to clear removed variable's pointers: %w", err)
	}

	return nil
}