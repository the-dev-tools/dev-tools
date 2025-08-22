package sflowvariable

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
)

// FlowVariableMovableRepository implements movable.MovableRepository for FlowVariables
// It adapts position-based operations to linked list operations using prev/next pointers
type FlowVariableMovableRepository struct {
	queries *gen.Queries
}

// NewFlowVariableMovableRepository creates a new FlowVariableMovableRepository
func NewFlowVariableMovableRepository(queries *gen.Queries) *FlowVariableMovableRepository {
	return &FlowVariableMovableRepository{
		queries: queries,
	}
}

// TX returns a new repository instance with transaction support
func (r *FlowVariableMovableRepository) TX(tx *sql.Tx) *FlowVariableMovableRepository {
	return &FlowVariableMovableRepository{
		queries: r.queries.WithTx(tx),
	}
}

// UpdatePosition updates the position of a flow variable in the linked list
// For flow variables, parentID is the flow_id and listType is ignored
func (r *FlowVariableMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get flow variable to find flow_id
	flowVariable, err := repo.queries.GetFlowVariable(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get flow variable: %w", err)
	}

	// Get ordered list of flow variables in flow
	orderedFlowVariables, err := repo.queries.GetFlowVariablesByFlowIDOrdered(ctx, gen.GetFlowVariablesByFlowIDOrderedParams{
		FlowID:   flowVariable.FlowID,
		FlowID_2: flowVariable.FlowID,
	})
	if err != nil {
		return fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	// Find current position and validate new position
	currentIdx := -1
	for i, fv := range orderedFlowVariables {
		if idwrap.NewFromBytesMust(fv.ID).Compare(itemID) == 0 {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("flow variable not found in flow")
	}

	if position < 0 || position >= len(orderedFlowVariables) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(orderedFlowVariables)-1)
	}

	if currentIdx == position {
		// No change needed
		return nil
	}

	// FIXED: Use proper remove/insert pattern to maintain linked list integrity
	// Step 1: Remove the flow variable from its current position
	if err := repo.removeFromPosition(ctx, tx, itemID); err != nil {
		return fmt.Errorf("failed to remove flow variable from current position: %w", err)
	}

	// Step 2: Calculate the target position after removal
	// When we remove an item, positions of items after it shift down by 1
	targetPosition := position
	if currentIdx < position {
		targetPosition = position - 1
	}
	
	// Special case: if we're moving to the last position in the original list,
	// we want to append to the end of the reduced list
	if position == len(orderedFlowVariables)-1 {
		targetPosition = len(orderedFlowVariables) // This will trigger append to end in insertAtPosition
	}

	// Step 3: Insert the flow variable at the new position
	if err := repo.insertAtPosition(ctx, tx, itemID, flowVariable.FlowID, targetPosition); err != nil {
		return fmt.Errorf("failed to insert flow variable at new position: %w", err)
	}

	return nil
}

// UpdatePositions updates positions for multiple flow variables in batch
func (r *FlowVariableMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the flow ID from the first flow variable to validate all are in same flow
	firstFlowVariable, err := repo.queries.GetFlowVariable(ctx, updates[0].ItemID)
	if err != nil {
		return fmt.Errorf("failed to get first flow variable: %w", err)
	}
	flowID := firstFlowVariable.FlowID
	
	// Validate all flow variables are in the same flow and create ID position map
	positionMap := make(map[idwrap.IDWrap]int)
	for _, update := range updates {
		flowVariable, err := repo.queries.GetFlowVariable(ctx, update.ItemID)
		if err != nil {
			return fmt.Errorf("failed to get flow variable %s: %w", update.ItemID.String(), err)
		}
		if flowVariable.FlowID.Compare(flowID) != 0 {
			return fmt.Errorf("all flow variables must be in the same flow")
		}
		positionMap[update.ItemID] = update.Position
	}
	
	// Build the complete ordered list with all flow variables at their new positions
	orderedIDs := make([]idwrap.IDWrap, len(updates))
	for _, update := range updates {
		if update.Position < 0 || update.Position >= len(updates) {
			return fmt.Errorf("invalid position %d for flow variable %s (valid range: 0-%d)", 
				update.Position, update.ItemID.String(), len(updates)-1)
		}
		orderedIDs[update.Position] = update.ItemID
	}
	
	// Calculate prev/next pointers for each flow variable in the new order
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
		if err := repo.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
			Prev:   update.prev,
			Next:   update.next,
			ID:     update.id,
			FlowID: flowID,
		}); err != nil {
			return fmt.Errorf("failed to update flow variable %s order: %w", update.id.String(), err)
		}
	}

	return nil
}

// GetMaxPosition returns the maximum position value for flow variables in a flow
// For linked lists, this is the count of flow variables minus 1
func (r *FlowVariableMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	// For flow variables, parentID is the flow_id
	orderedFlowVariables, err := r.queries.GetFlowVariablesByFlowIDOrdered(ctx, gen.GetFlowVariablesByFlowIDOrderedParams{
		FlowID:   parentID,
		FlowID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No flow variables means no max position
		}
		return 0, fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	if len(orderedFlowVariables) == 0 {
		return -1, nil
	}

	return len(orderedFlowVariables) - 1, nil
}

// GetItemsByParent returns all flow variables under a flow, ordered by position
func (r *FlowVariableMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	// For flow variables, parentID is the flow_id
	orderedFlowVariables, err := r.queries.GetFlowVariablesByFlowIDOrdered(ctx, gen.GetFlowVariablesByFlowIDOrderedParams{
		FlowID:   parentID,
		FlowID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []movable.MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	// Convert to MovableItem structs
	items := make([]movable.MovableItem, len(orderedFlowVariables))
	for i, fv := range orderedFlowVariables {
		items[i] = movable.MovableItem{
			ID:       idwrap.NewFromBytesMust(fv.ID),
			ParentID: &parentID, // flow_id as parent
			Position: int(fv.Position),
			ListType: listType,
		}
	}

	return items, nil
}

// insertAtPosition inserts a flow variable at a specific position in the linked list
// This is a helper method for operations that need to insert new flow variables
func (r *FlowVariableMovableRepository) insertAtPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, flowID idwrap.IDWrap, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get ordered list of flow variables in flow
	orderedFlowVariables, err := repo.queries.GetFlowVariablesByFlowIDOrdered(ctx, gen.GetFlowVariablesByFlowIDOrderedParams{
		FlowID:   flowID,
		FlowID_2: flowID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get flow variables in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedFlowVariables) == 0 {
		// First flow variable in flow
		newPrev = nil
		newNext = nil
	} else if position == 0 {
		// Insert at head
		newPrev = nil
		if len(orderedFlowVariables) > 0 {
			currentHeadID := idwrap.NewFromBytesMust(orderedFlowVariables[0].ID)
			newNext = &currentHeadID
			
			// Get the current head's next pointer to preserve it
			currentHead, err := repo.queries.GetFlowVariable(ctx, currentHeadID)
			if err != nil {
				return fmt.Errorf("failed to get current head: %w", err)
			}
			
			// Update the current head to point back to new item
			err = repo.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
				Prev:   &itemID,        // Current head's prev now points to new item
				Next:   currentHead.Next, // Preserve current head's next pointer
				ID:     currentHeadID,
				FlowID: flowID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedFlowVariables) {
		// Insert at tail
		if len(orderedFlowVariables) > 0 {
			currentTailID := idwrap.NewFromBytesMust(orderedFlowVariables[len(orderedFlowVariables)-1].ID)
			newPrev = &currentTailID
			
			// Get the current tail's prev pointer to preserve it
			currentTail, err := repo.queries.GetFlowVariable(ctx, currentTailID)
			if err != nil {
				return fmt.Errorf("failed to get current tail: %w", err)
			}
			
			// Update the current tail to point forward to new item
			err = repo.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
				Prev:   currentTail.Prev, // Preserve current tail's prev pointer
				Next:   &itemID,         // Current tail's next now points to new item
				ID:     currentTailID,
				FlowID: flowID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current tail: %w", err)
			}
		}
		newNext = nil
	} else {
		// Insert in middle
		prevID := idwrap.NewFromBytesMust(orderedFlowVariables[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedFlowVariables[position].ID)
		newPrev = &prevID
		newNext = &nextID
		
		// Get existing prev and next items to preserve their other pointers
		prevItem, err := repo.queries.GetFlowVariable(ctx, prevID)
		if err != nil {
			return fmt.Errorf("failed to get prev item: %w", err)
		}
		
		nextItem, err := repo.queries.GetFlowVariable(ctx, nextID)
		if err != nil {
			return fmt.Errorf("failed to get next item: %w", err)
		}
		
		// Update prev item to point to new item
		err = repo.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
			Prev:   prevItem.Prev, // Preserve prev item's own prev pointer
			Next:   &itemID,       // Point to new item
			ID:     prevID,
			FlowID: flowID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}
		
		// Update next item to point back to new item
		err = repo.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
			Prev:   &itemID,       // Point to new item
			Next:   nextItem.Next, // Preserve next item's own next pointer
			ID:     nextID,
			FlowID: flowID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next item: %w", err)
		}
	}

	// Set the new item's position
	return repo.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
		Prev:   newPrev,
		Next:   newNext,
		ID:     itemID,
		FlowID: flowID,
	})
}

// removeFromPosition removes a flow variable from its current position in the linked list
// This is a helper method for operations that need to remove flow variables
func (r *FlowVariableMovableRepository) removeFromPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the flow variable to remove and its current prev/next pointers
	flowVariable, err := repo.queries.GetFlowVariable(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get flow variable: %w", err)
	}

	// Update prev flow variable's next pointer to skip the removed flow variable
	if flowVariable.Prev != nil {
		err = repo.queries.UpdateFlowVariableNext(ctx, gen.UpdateFlowVariableNextParams{
			Next:   flowVariable.Next, // Point to the removed flow variable's next
			ID:     *flowVariable.Prev,
			FlowID: flowVariable.FlowID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous flow variable's next pointer: %w", err)
		}
	}

	// Update next flow variable's prev pointer to skip the removed flow variable
	if flowVariable.Next != nil {
		err = repo.queries.UpdateFlowVariablePrev(ctx, gen.UpdateFlowVariablePrevParams{
			Prev:   flowVariable.Prev, // Point to the removed flow variable's prev
			ID:     *flowVariable.Next,
			FlowID: flowVariable.FlowID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next flow variable's prev pointer: %w", err)
		}
	}

	// Clear the removed flow variable's pointers
	err = repo.queries.UpdateFlowVariableOrder(ctx, gen.UpdateFlowVariableOrderParams{
		Prev:   nil,
		Next:   nil,
		ID:     itemID,
		FlowID: flowVariable.FlowID,
	})
	if err != nil {
		return fmt.Errorf("failed to clear removed flow variable's pointers: %w", err)
	}

	return nil
}