package sitemapiexample

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/movable"
)

// ExampleMovableRepository implements movable.MovableRepository for Examples
// It adapts position-based operations to linked list operations using prev/next pointers
// Examples are scoped by endpoint_id (item_api_id)
type ExampleMovableRepository struct {
	queries *gen.Queries
}

// NewExampleMovableRepository creates a new ExampleMovableRepository
func NewExampleMovableRepository(queries *gen.Queries) *ExampleMovableRepository {
	return &ExampleMovableRepository{
		queries: queries,
	}
}

// TX returns a new repository instance with transaction support
func (r *ExampleMovableRepository) TX(tx *sql.Tx) *ExampleMovableRepository {
    return &ExampleMovableRepository{
        queries: r.queries.WithTx(tx),
    }
}

// UpdatePosition updates the position of an example in the linked list
// For examples, parentID is the endpoint_id (item_api_id) and listType should be CollectionListTypeExamples
// This method performs an atomic move operation that prevents examples from becoming isolated
func (r *ExampleMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get example to find endpoint_id
	example, err := repo.queries.GetItemApiExample(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get example: %w", err)
	}

	// Get ordered list of examples for the endpoint
	orderedExamples, err := repo.queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   example.ItemApiID,
		ItemApiID_2: example.ItemApiID,
	})
	if err != nil {
		return fmt.Errorf("failed to get examples in order: %w", err)
	}

	// Find current position and validate new position
	currentIdx := -1
	for i, ex := range orderedExamples {
		if idwrap.NewFromBytesMust(ex.ID).Compare(itemID) == 0 {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("example not found in endpoint")
	}

	if position < 0 || position >= len(orderedExamples) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(orderedExamples)-1)
	}

	if currentIdx == position {
		// No change needed
		return nil
	}

	// Perform atomic move operation to prevent isolation
	if err := repo.atomicMove(ctx, tx, itemID, example.ItemApiID, currentIdx, position, orderedExamples); err != nil {
		return fmt.Errorf("failed to perform atomic move: %w", err)
	}

	return nil
}

// UpdatePositions updates positions for multiple examples in batch
func (r *ExampleMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the endpoint ID from the first example to validate all are in same endpoint
	firstExample, err := repo.queries.GetItemApiExample(ctx, updates[0].ItemID)
	if err != nil {
		return fmt.Errorf("failed to get first example: %w", err)
	}
	endpointID := firstExample.ItemApiID

	// Validate all examples are in the same endpoint and create ID position map
	positionMap := make(map[idwrap.IDWrap]int)
	for _, update := range updates {
		example, err := repo.queries.GetItemApiExample(ctx, update.ItemID)
		if err != nil {
			return fmt.Errorf("failed to get example %s: %w", update.ItemID.String(), err)
		}
		if example.ItemApiID.Compare(endpointID) != 0 {
			return fmt.Errorf("all examples must be in the same endpoint")
		}
		positionMap[update.ItemID] = update.Position
	}

	// Build the complete ordered list with all examples at their new positions
	orderedIDs := make([]idwrap.IDWrap, len(updates))
	for _, update := range updates {
		if update.Position < 0 || update.Position >= len(updates) {
			return fmt.Errorf("invalid position %d for example %s (valid range: 0-%d)",
				update.Position, update.ItemID.String(), len(updates)-1)
		}
		orderedIDs[update.Position] = update.ItemID
	}

	// Calculate prev/next pointers for each example in the new order
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
		if err := repo.queries.UpdateExampleOrder(ctx, gen.UpdateExampleOrderParams{
			Prev:      update.prev,
			Next:      update.next,
			ID:        update.id,
			ItemApiID: endpointID,
		}); err != nil {
			return fmt.Errorf("failed to update example %s order: %w", update.id.String(), err)
		}
	}

	return nil
}

// GetMaxPosition returns the maximum position value for examples in an endpoint
// For linked lists, this is the count of examples minus 1
func (r *ExampleMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	// For examples, parentID is the endpoint_id (item_api_id)
	orderedExamples, err := r.queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   parentID,
		ItemApiID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No examples means no max position
		}
		return 0, fmt.Errorf("failed to get examples in order: %w", err)
	}

	if len(orderedExamples) == 0 {
		return -1, nil
	}

	return len(orderedExamples) - 1, nil
}

// GetItemsByParent returns all examples under an endpoint, ordered by position
func (r *ExampleMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	// For examples, parentID is the endpoint_id (item_api_id)
	orderedExamples, err := r.queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   parentID,
		ItemApiID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []movable.MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get examples in order: %w", err)
	}

	// Convert to MovableItem structs
	items := make([]movable.MovableItem, len(orderedExamples))
	for i, ex := range orderedExamples {
		items[i] = movable.MovableItem{
			ID:       idwrap.NewFromBytesMust(ex.ID),
			ParentID: &parentID, // endpoint_id as parent
			Position: i,         // Use index as position (0, 1, 2, etc.)
			ListType: listType,
		}
	}

	return items, nil
}

// ExampleListType creates a ListType for examples scoped to an endpoint
func ExampleListType(endpointID idwrap.IDWrap) movable.ListType {
	// Return the examples list type from the movable package
	return movable.CollectionListTypeExamples
}

// Create creates a new example with proper linked list management
func (r *ExampleMovableRepository) Create(ctx context.Context, tx *sql.Tx, example mitemapiexample.ItemApiExample) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Special handling for default examples - they exist outside the user chain
	if example.IsDefault {
		// Default examples don't participate in the linked list
		// They have prev=NULL and next=NULL and stay isolated
		arg := ConvertToDBItem(example)
		err := repo.queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
			ID:              arg.ID,
			ItemApiID:       arg.ItemApiID,
			CollectionID:    arg.CollectionID,
			IsDefault:       arg.IsDefault,
			BodyType:        arg.BodyType,
			Name:            arg.Name,
			VersionParentID: arg.VersionParentID,
			Prev:            nil, // Default examples are isolated
			Next:            nil, // Default examples are isolated
		})
		if err != nil {
			return fmt.Errorf("failed to create default example: %w", err)
		}
		return nil
	}

	// Plan a safe append using movable planner (preflight and tail detection)
	plan, err := movable.BuildAppendPlanFromRepo(ctx, repo, example.ItemApiID, movable.CollectionListTypeExamples, example.ID)
	if err != nil {
		return fmt.Errorf("append plan failed: %w", err)
	}

	// Create the example with proper linking
	arg := ConvertToDBItem(example)
	err = repo.queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:              arg.ID,
		ItemApiID:       arg.ItemApiID,
		CollectionID:    arg.CollectionID,
		IsDefault:       arg.IsDefault,
		BodyType:        arg.BodyType,
		Name:            arg.Name,
		VersionParentID: arg.VersionParentID,
		Prev:            plan.PrevID, // Link to planner-detected tail or NULL if first user example
		Next:            nil,         // New tail has no next
	})
	if err != nil {
		return fmt.Errorf("failed to create example: %w", err)
	}

    // If there was a previous tail, link it to the new example.
    // Be resilient to benign tail advances between planning and linking by re-linking to the current tail.
    if plan.PrevID != nil {
        // Read what we thought was the tail when we planned
        prevRow, gerr := repo.queries.GetItemApiExample(ctx, *plan.PrevID)
        if gerr != nil {
            return fmt.Errorf("failed to read tail: %w", gerr)
        }

        if prevRow.Next == nil {
            // Tail did not advance; perform the normal link
            if err := repo.queries.UpdateExampleNext(ctx, gen.UpdateExampleNextParams{
                Next:      &example.ID,
                ID:        *plan.PrevID,
                ItemApiID: example.ItemApiID,
            }); err != nil {
                return fmt.Errorf("failed to update previous tail: %w", err)
            }
        } else {
            // Tail advanced after planning; re-link to the actual current tail.
            orderedExamples, gerr := repo.queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
                ItemApiID:   example.ItemApiID,
                ItemApiID_2: example.ItemApiID,
            })
            if gerr != nil {
                return fmt.Errorf("failed to fetch ordered examples for tail recovery: %w", gerr)
            }
            if len(orderedExamples) > 0 {
                currentTailID := idwrap.NewFromBytesMust(orderedExamples[len(orderedExamples)-1].ID)
                // If the new example is already the tail, nothing to do
                if currentTailID.Compare(example.ID) != 0 {
                    // Update new example's prev to the current tail, then point current tail to new example
                    if err := repo.queries.UpdateExamplePrev(ctx, gen.UpdateExamplePrevParams{
                        Prev:      &currentTailID,
                        ID:        example.ID,
                        ItemApiID: example.ItemApiID,
                    }); err != nil {
                        return fmt.Errorf("failed to set new example prev during tail recovery: %w", err)
                    }
                    if err := repo.queries.UpdateExampleNext(ctx, gen.UpdateExampleNextParams{
                        Next:      &example.ID,
                        ID:        currentTailID,
                        ItemApiID: example.ItemApiID,
                    }); err != nil {
                        return fmt.Errorf("failed to set current tail next during tail recovery: %w", err)
                    }
                }
            }
            // If no ordered examples were returned, we leave the inserted row as-is; next append will correct chain.
        }
    }

	return nil
}

// insertAtPosition inserts an example at a specific position in the linked list
// This is a helper method for operations that need to insert examples
func (r *ExampleMovableRepository) insertAtPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, endpointID idwrap.IDWrap, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get ordered list of examples for the endpoint
	orderedExamples, err := repo.queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get examples in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedExamples) == 0 {
		// First example for endpoint
		newPrev = nil
		newNext = nil
	} else if position == 0 {
		// Insert at head
		newPrev = nil
		if len(orderedExamples) > 0 {
			currentHeadID := idwrap.NewFromBytesMust(orderedExamples[0].ID)
			newNext = &currentHeadID

			// Update the current head to point back to new item
			err = repo.queries.UpdateExamplePrev(ctx, gen.UpdateExamplePrevParams{
				Prev:      &itemID, // Current head's prev now points to new item
				ID:        currentHeadID,
				ItemApiID: endpointID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedExamples) {
		// Insert at tail
		if len(orderedExamples) > 0 {
			currentTailID := idwrap.NewFromBytesMust(orderedExamples[len(orderedExamples)-1].ID)
			newPrev = &currentTailID

			// Update the current tail to point forward to new item
			err = repo.queries.UpdateExampleNext(ctx, gen.UpdateExampleNextParams{
				Next:      &itemID, // Current tail's next now points to new item
				ID:        currentTailID,
				ItemApiID: endpointID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current tail: %w", err)
			}
		}
		newNext = nil
	} else {
		// Insert in middle
		prevID := idwrap.NewFromBytesMust(orderedExamples[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedExamples[position].ID)
		newPrev = &prevID
		newNext = &nextID

		// Update prev item to point to new item
		err = repo.queries.UpdateExampleNext(ctx, gen.UpdateExampleNextParams{
			Next:      &itemID, // Point to new item
			ID:        prevID,
			ItemApiID: endpointID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}

		// Update next item to point back to new item
		err = repo.queries.UpdateExamplePrev(ctx, gen.UpdateExamplePrevParams{
			Prev:      &itemID, // Point to new item
			ID:        nextID,
			ItemApiID: endpointID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next item: %w", err)
		}
	}

	// Set the new item's position
	return repo.queries.UpdateExampleOrder(ctx, gen.UpdateExampleOrderParams{
		Prev:      newPrev,
		Next:      newNext,
		ID:        itemID,
		ItemApiID: endpointID,
	})
}

// atomicMove performs an atomic move operation that prevents examples from becoming isolated
// This replaces the problematic remove/insert pattern with a single atomic operation
func (r *ExampleMovableRepository) atomicMove(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, endpointID idwrap.IDWrap, currentIdx, targetIdx int, orderedExamples []gen.GetExamplesByEndpointIDOrderedRow) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Calculate all required pointer updates for the atomic move
	updates, err := repo.calculateAtomicMoveUpdates(itemID, currentIdx, targetIdx, orderedExamples)
	if err != nil {
		return fmt.Errorf("failed to calculate move updates: %w", err)
	}

	// Apply all updates atomically - if any fails, the transaction will rollback
	for _, update := range updates {
		if err := repo.queries.UpdateExampleOrder(ctx, gen.UpdateExampleOrderParams{
			Prev:      update.prev,
			Next:      update.next,
			ID:        update.id,
			ItemApiID: endpointID,
		}); err != nil {
			return fmt.Errorf("failed to update example %s pointers: %w", update.id.String(), err)
		}
	}

	return nil
}

// atomicMoveUpdate represents a single pointer update in an atomic move
type atomicMoveUpdate struct {
	id   idwrap.IDWrap
	prev *idwrap.IDWrap
	next *idwrap.IDWrap
}

// calculateAtomicMoveUpdates calculates all pointer updates needed for atomic move
// This ensures no example ever becomes isolated during the move process
func (r *ExampleMovableRepository) calculateAtomicMoveUpdates(itemID idwrap.IDWrap, currentIdx, targetIdx int, orderedExamples []gen.GetExamplesByEndpointIDOrderedRow) ([]atomicMoveUpdate, error) {
	var updates []atomicMoveUpdate

	// Handle empty list case
	if len(orderedExamples) == 0 {
		return updates, nil
	}

	// Validate indices
	if currentIdx < 0 || currentIdx >= len(orderedExamples) {
		return nil, fmt.Errorf("invalid currentIdx: %d (valid range: 0-%d)", currentIdx, len(orderedExamples)-1)
	}
	if targetIdx < 0 || targetIdx >= len(orderedExamples) {
		return nil, fmt.Errorf("invalid targetIdx: %d (valid range: 0-%d)", targetIdx, len(orderedExamples)-1)
	}

	// Create a working copy of the example order with the item moved to its new position
	workingOrder := make([]idwrap.IDWrap, len(orderedExamples))
	for i, ex := range orderedExamples {
		workingOrder[i] = idwrap.NewFromBytesMust(ex.ID)
	}

	// Move item from currentIdx to targetIdx
	movedItem := workingOrder[currentIdx]

	// Remove the item from its current position
	workingOrder = append(workingOrder[:currentIdx], workingOrder[currentIdx+1:]...)

	// Calculate where to insert in the shortened array
	insertIndex := targetIdx
	if currentIdx < targetIdx {
		// When moving forward, we need to account for the removal
		// If targetIdx was the last position in the original array, append to end
		if targetIdx == len(orderedExamples)-1 {
			insertIndex = len(workingOrder) // Append to end
		} else {
			insertIndex = targetIdx - 1
		}
	}

	// Insert the item at the calculated position
	workingOrder = append(workingOrder[:insertIndex], append([]idwrap.IDWrap{movedItem}, workingOrder[insertIndex:]...)...)

	// Calculate new prev/next pointers for all affected examples
	for i, id := range workingOrder {
		var prev, next *idwrap.IDWrap

		if i > 0 {
			prev = &workingOrder[i-1]
		}
		if i < len(workingOrder)-1 {
			next = &workingOrder[i+1]
		}

		// Only include updates for examples that need pointer changes
		needsUpdate := false

		// Find current pointers for this example
		for _, ex := range orderedExamples {
			if idwrap.NewFromBytesMust(ex.ID).Compare(id) == 0 {
				// Get current prev/next as idwrap pointers
				var currentPrev, currentNext *idwrap.IDWrap
				if ex.Prev != nil {
					currentPrevID := idwrap.NewFromBytesMust(ex.Prev)
					currentPrev = &currentPrevID
				}
				if ex.Next != nil {
					currentNextID := idwrap.NewFromBytesMust(ex.Next)
					currentNext = &currentNextID
				}

				// Check if prev pointer changed
				if (prev == nil && currentPrev != nil) || (prev != nil && currentPrev == nil) {
					needsUpdate = true
				} else if prev != nil && currentPrev != nil && prev.Compare(*currentPrev) != 0 {
					needsUpdate = true
				}

				// Check if next pointer changed
				if (next == nil && currentNext != nil) || (next != nil && currentNext == nil) {
					needsUpdate = true
				} else if next != nil && currentNext != nil && next.Compare(*currentNext) != 0 {
					needsUpdate = true
				}
				break
			}
		}

		if needsUpdate {
			updates = append(updates, atomicMoveUpdate{
				id:   id,
				prev: prev,
				next: next,
			})
		}
	}

	return updates, nil
}

// removeFromPosition removes an example from its current position in the linked list
// DEPRECATED: This method creates isolated nodes and should not be used for moves
// Use atomicMove instead for move operations
func (r *ExampleMovableRepository) removeFromPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, endpointID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the example to remove and its current prev/next pointers
	example, err := repo.queries.GetItemApiExample(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get example: %w", err)
	}

	// Update prev example's next pointer to skip the removed example
	if example.Prev != nil {
		err = repo.queries.UpdateExampleNext(ctx, gen.UpdateExampleNextParams{
			Next:      example.Next, // Point to the removed example's next
			ID:        *example.Prev,
			ItemApiID: endpointID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous example's next pointer: %w", err)
		}
	}

	// Update next example's prev pointer to skip the removed example
	if example.Next != nil {
		err = repo.queries.UpdateExamplePrev(ctx, gen.UpdateExamplePrevParams{
			Prev:      example.Prev, // Point to the removed example's prev
			ID:        *example.Next,
			ItemApiID: endpointID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next example's prev pointer: %w", err)
		}
	}

	// Clear the removed example's pointers
	err = repo.queries.UpdateExampleOrder(ctx, gen.UpdateExampleOrderParams{
		Prev:      nil,
		Next:      nil,
		ID:        itemID,
		ItemApiID: endpointID,
	})
	if err != nil {
		return fmt.Errorf("failed to clear removed example's pointers: %w", err)
	}

	return nil
}

// Remove unlinks an example from its endpoint chain
func (r *ExampleMovableRepository) Remove(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
    // Resolve endpoint scope for this example
    ex, err := r.queries.GetItemApiExample(ctx, itemID)
    if err != nil {
        return fmt.Errorf("failed to get example: %w", err)
    }
    return r.removeFromPosition(ctx, tx, itemID, ex.ItemApiID)
}

// DetectIsolatedExamples finds examples that are isolated (prev=NULL, next=NULL) but are not the only example
// These examples would be invisible to the recursive CTE query but still exist in the database
func (r *ExampleMovableRepository) DetectIsolatedExamples(ctx context.Context, endpointID idwrap.IDWrap) ([]idwrap.IDWrap, error) {
	// Get all examples for this endpoint directly from database
	allExamples, err := r.queries.GetItemApiExamples(ctx, endpointID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all examples: %w", err)
	}

	// If there's only one example, it's allowed to have prev=NULL and next=NULL
	if len(allExamples) <= 1 {
		return []idwrap.IDWrap{}, nil
	}

	var isolated []idwrap.IDWrap
	for _, ex := range allExamples {
		// Skip default examples
		if ex.IsDefault {
			continue
		}

		// Skip examples with version parent (not base examples)
		if ex.VersionParentID != nil {
			continue
		}

		// If example has both prev=NULL and next=NULL, and there are other examples, it's isolated
		if ex.Prev == nil && ex.Next == nil {
			isolated = append(isolated, ex.ID)
		}
	}

	return isolated, nil
}

// RepairIsolatedExamples automatically links isolated examples back into the chain
// This method should be used for recovery from corrupted linked lists
func (r *ExampleMovableRepository) RepairIsolatedExamples(ctx context.Context, tx *sql.Tx, endpointID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Find isolated examples
	isolatedIDs, err := repo.DetectIsolatedExamples(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("failed to detect isolated examples: %w", err)
	}

	if len(isolatedIDs) == 0 {
		return nil // No isolated examples to repair
	}

	// Get the current valid chain using the ordered query
	orderedExamples, err := repo.queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	if err != nil {
		return fmt.Errorf("failed to get ordered examples: %w", err)
	}

	// For each isolated example, append it to the end of the valid chain
	for _, isolatedID := range isolatedIDs {
		var newPrev *idwrap.IDWrap

		// If there are examples in the valid chain, link to the last one
		if len(orderedExamples) > 0 {
			lastExampleID := idwrap.NewFromBytesMust(orderedExamples[len(orderedExamples)-1].ID)
			newPrev = &lastExampleID

			// Update the current tail to point to the isolated example
			err = repo.queries.UpdateExampleNext(ctx, gen.UpdateExampleNextParams{
				Next:      &isolatedID,
				ID:        lastExampleID,
				ItemApiID: endpointID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current tail's next pointer: %w", err)
			}
		}

		// Update the isolated example to link into the chain
		err = repo.queries.UpdateExampleOrder(ctx, gen.UpdateExampleOrderParams{
			Prev:      newPrev,
			Next:      nil, // It becomes the new tail
			ID:        isolatedID,
			ItemApiID: endpointID,
		})
		if err != nil {
			return fmt.Errorf("failed to link isolated example %s: %w", isolatedID.String(), err)
		}

		// Add this repaired example to our ordered list for next iteration
		orderedExamples = append(orderedExamples, gen.GetExamplesByEndpointIDOrderedRow{
			ID: isolatedID.Bytes(),
		})
	}

	return nil
}

// ValidateLinkedListIntegrity checks if the linked list structure is valid
// Returns an error if corruption is detected
func (r *ExampleMovableRepository) ValidateLinkedListIntegrity(ctx context.Context, endpointID idwrap.IDWrap) error {
	// Get all examples directly from database
	allExamples, err := r.queries.GetItemApiExamples(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("failed to get all examples: %w", err)
	}

	// Filter to non-default, base examples
	var baseExamples []gen.ItemApiExample
	for _, ex := range allExamples {
		if !ex.IsDefault && ex.VersionParentID == nil {
			baseExamples = append(baseExamples, ex)
		}
	}

	if len(baseExamples) == 0 {
		return nil // No examples to validate
	}

	// Get examples via ordered query (what the API sees)
	orderedExamples, err := r.queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	if err != nil {
		return fmt.Errorf("failed to get ordered examples: %w", err)
	}

	// Check 1: All base examples should be visible via ordered query
	if len(baseExamples) != len(orderedExamples) {
		isolatedIDs, _ := r.DetectIsolatedExamples(ctx, endpointID)
		return fmt.Errorf("linked list corruption detected: %d examples in database, %d visible via API (%d isolated)",
			len(baseExamples), len(orderedExamples), len(isolatedIDs))
	}

	// Check 2: Verify pointer consistency
	exampleMap := make(map[string]gen.ItemApiExample)
	for _, ex := range baseExamples {
		exampleMap[ex.ID.String()] = ex
	}

	for _, ex := range baseExamples {
		exID := ex.ID

		// If has prev, prev should point back to this example
		if ex.Prev != nil {
			prevID := *ex.Prev
			prevEx, exists := exampleMap[prevID.String()]
			if !exists {
				return fmt.Errorf("example %s points to non-existent prev %s", exID.String(), prevID.String())
			}
			if prevEx.Next == nil || prevEx.Next.Compare(exID) != 0 {
				return fmt.Errorf("bidirectional link broken: %s->prev->%s but %s->next does not point back",
					exID.String(), prevID.String(), prevID.String())
			}
		}

		// If has next, next should point back to this example
		if ex.Next != nil {
			nextID := *ex.Next
			nextEx, exists := exampleMap[nextID.String()]
			if !exists {
				return fmt.Errorf("example %s points to non-existent next %s", exID.String(), nextID.String())
			}
			if nextEx.Prev == nil || nextEx.Prev.Compare(exID) != 0 {
				return fmt.Errorf("bidirectional link broken: %s->next->%s but %s->prev does not point back",
					exID.String(), nextID.String(), nextID.String())
			}
		}
	}

	// Check 3: Should have exactly one head (prev=NULL) and one tail (next=NULL)
	heads := 0
	tails := 0
	for _, ex := range baseExamples {
		if ex.Prev == nil {
			heads++
		}
		if ex.Next == nil {
			tails++
		}
	}

	if heads != 1 {
		return fmt.Errorf("linked list has %d head nodes (should be 1)", heads)
	}
	if tails != 1 {
		return fmt.Errorf("linked list has %d tail nodes (should be 1)", tails)
	}

	return nil
}
