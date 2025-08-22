package sworkspace

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
)

// WorkspaceMovableRepository implements movable.MovableRepository for Workspaces
// It adapts position-based operations to linked list operations using prev/next pointers
// For workspaces, the key difference is that UserID acts as the parent scope instead of a direct FK
type WorkspaceMovableRepository struct {
	queries *gen.Queries
}

// NewWorkspaceMovableRepository creates a new WorkspaceMovableRepository
func NewWorkspaceMovableRepository(queries *gen.Queries) *WorkspaceMovableRepository {
	return &WorkspaceMovableRepository{
		queries: queries,
	}
}

// TX returns a new repository instance with transaction support
func (r *WorkspaceMovableRepository) TX(tx *sql.Tx) *WorkspaceMovableRepository {
	return &WorkspaceMovableRepository{
		queries: r.queries.WithTx(tx),
	}
}

// UpdatePosition updates the position of a workspace in the linked list
// For workspaces, parentID is the user_id and listType should be WorkspaceListTypeWorkspaces
func (r *WorkspaceMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// For workspaces, we need to get the user that owns this workspace
	// Since workspaces don't directly store userID, we need to find it via workspace_users table
	workspaceUsers, err := repo.queries.GetWorkspaceUserByWorkspaceID(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get workspace users: %w", err)
	}
	if len(workspaceUsers) == 0 {
		return fmt.Errorf("workspace has no users")
	}

	// For simplicity, use the first user (workspace owner/admin)
	// In a real system, you'd want to pass the userID as a parameter
	userID := workspaceUsers[0].UserID

	// Get ordered list of workspaces for user
	orderedWorkspaces, err := repo.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to get workspaces in order: %w", err)
	}

	// Find current position and validate new position
	currentIdx := -1
	for i, ws := range orderedWorkspaces {
		if idwrap.NewFromBytesMust(ws.ID).Compare(itemID) == 0 {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("workspace not found in user's workspace list")
	}

	if position < 0 || position >= len(orderedWorkspaces) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(orderedWorkspaces)-1)
	}

	if currentIdx == position {
		// No change needed
		return nil
	}

	// Use proper remove/insert pattern to maintain linked list integrity
	// Step 1: Remove the workspace from its current position
	if err := repo.removeFromPosition(ctx, tx, itemID, userID); err != nil {
		return fmt.Errorf("failed to remove workspace from current position: %w", err)
	}

	// Step 2: Calculate the target position after removal
	// When we remove an item, positions of items after it shift down by 1
	targetPosition := position
	if currentIdx < position {
		targetPosition = position - 1
	}
	
	// Special case: if we're moving to the last position in the original list,
	// we want to append to the end of the reduced list
	if position == len(orderedWorkspaces)-1 {
		targetPosition = len(orderedWorkspaces) // This will trigger append to end in insertAtPosition
	}

	// Step 3: Insert the workspace at the new position
	if err := repo.insertAtPosition(ctx, tx, itemID, userID, targetPosition); err != nil {
		return fmt.Errorf("failed to insert workspace at new position: %w", err)
	}

	return nil
}

// UpdatePositions updates positions for multiple workspaces in batch
func (r *WorkspaceMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get user ID from the first workspace to validate all are in same user's list
	firstWorkspaceUsers, err := repo.queries.GetWorkspaceUserByWorkspaceID(ctx, updates[0].ItemID)
	if err != nil {
		return fmt.Errorf("failed to get first workspace users: %w", err)
	}
	if len(firstWorkspaceUsers) == 0 {
		return fmt.Errorf("first workspace has no users")
	}
	userID := firstWorkspaceUsers[0].UserID
	
	// Validate all workspaces are accessible by the same user and create ID position map
	positionMap := make(map[idwrap.IDWrap]int)
	for _, update := range updates {
		workspaceUsers, err := repo.queries.GetWorkspaceUserByWorkspaceID(ctx, update.ItemID)
		if err != nil {
			return fmt.Errorf("failed to get workspace users for %s: %w", update.ItemID.String(), err)
		}
		
		// Check if user has access to this workspace
		hasAccess := false
		for _, wu := range workspaceUsers {
			if wu.UserID.Compare(userID) == 0 {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			return fmt.Errorf("user does not have access to workspace %s", update.ItemID.String())
		}
		
		positionMap[update.ItemID] = update.Position
	}
	
	// Build the complete ordered list with all workspaces at their new positions
	orderedIDs := make([]idwrap.IDWrap, len(updates))
	for _, update := range updates {
		if update.Position < 0 || update.Position >= len(updates) {
			return fmt.Errorf("invalid position %d for workspace %s (valid range: 0-%d)", 
				update.Position, update.ItemID.String(), len(updates)-1)
		}
		orderedIDs[update.Position] = update.ItemID
	}
	
	// Calculate prev/next pointers for each workspace in the new order
	type ptrUpdate struct {
		id   idwrap.IDWrap
		prev *idwrap.IDWrap
		next *idwrap.IDWrap
	}
	
	ptrUpdates := make([]ptrUpdate, len(orderedIDs))
	for i, id := range orderedIDs {
		var prev, next *idwrap.IDWrap
		
		if i > 0 {
			prevID := orderedIDs[i-1]
			prev = &prevID
		}
		if i < len(orderedIDs)-1 {
			nextID := orderedIDs[i+1]
			next = &nextID
		}
		
		ptrUpdates[i] = ptrUpdate{
			id:   id,
			prev: prev,
			next: next,
		}
	}
	
	// Apply all updates atomically
	for _, update := range ptrUpdates {
		if err := repo.queries.UpdateWorkspaceOrder(ctx, gen.UpdateWorkspaceOrderParams{
			Prev:   update.prev,
			Next:   update.next,
			ID:     update.id,
			UserID: userID,
		}); err != nil {
			return fmt.Errorf("failed to update workspace %s order: %w", update.id.String(), err)
		}
	}

	return nil
}

// GetMaxPosition returns the maximum position value for workspaces for a user
// For linked lists, this is the count of workspaces minus 1
func (r *WorkspaceMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
	// For workspaces, parentID is the user_id
	orderedWorkspaces, err := r.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   parentID,
		UserID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No workspaces means no max position
		}
		return 0, fmt.Errorf("failed to get workspaces in order: %w", err)
	}

	if len(orderedWorkspaces) == 0 {
		return -1, nil
	}

	return len(orderedWorkspaces) - 1, nil
}

// GetItemsByParent returns all workspaces under a user, ordered by position
func (r *WorkspaceMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
	// For workspaces, parentID is the user_id
	orderedWorkspaces, err := r.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   parentID,
		UserID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []movable.MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get workspaces in order: %w", err)
	}

	// Convert to MovableItem structs
	items := make([]movable.MovableItem, len(orderedWorkspaces))
	for i, ws := range orderedWorkspaces {
		items[i] = movable.MovableItem{
			ID:       idwrap.NewFromBytesMust(ws.ID),
			ParentID: &parentID, // user_id as parent
			Position: int(ws.Position),
			ListType: listType,
		}
	}

	return items, nil
}

// insertAtPosition inserts a workspace at a specific position in the linked list
// This is a helper method for operations that need to insert workspaces
func (r *WorkspaceMovableRepository) insertAtPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, userID idwrap.IDWrap, position int) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get ordered list of workspaces for user
	orderedWorkspaces, err := repo.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get workspaces in order: %w", err)
	}

	var newPrev, newNext *idwrap.IDWrap

	if len(orderedWorkspaces) == 0 {
		// First workspace for user
		newPrev = nil
		newNext = nil
	} else if position == 0 {
		// Insert at head
		newPrev = nil
		if len(orderedWorkspaces) > 0 {
			currentHeadID := idwrap.NewFromBytesMust(orderedWorkspaces[0].ID)
			newNext = &currentHeadID
			
			// Update the current head to point back to new item
			err = repo.queries.UpdateWorkspacePrev(ctx, gen.UpdateWorkspacePrevParams{
				Prev:   &itemID,        // Current head's prev now points to new item
				ID:     currentHeadID,
				UserID: userID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current head: %w", err)
			}
		}
	} else if position >= len(orderedWorkspaces) {
		// Insert at tail
		if len(orderedWorkspaces) > 0 {
			currentTailID := idwrap.NewFromBytesMust(orderedWorkspaces[len(orderedWorkspaces)-1].ID)
			newPrev = &currentTailID
			
			// Update the current tail to point forward to new item
			err = repo.queries.UpdateWorkspaceNext(ctx, gen.UpdateWorkspaceNextParams{
				Next:   &itemID,         // Current tail's next now points to new item
				ID:     currentTailID,
				UserID: userID,
			})
			if err != nil {
				return fmt.Errorf("failed to update current tail: %w", err)
			}
		}
		newNext = nil
	} else {
		// Insert in middle
		prevID := idwrap.NewFromBytesMust(orderedWorkspaces[position-1].ID)
		nextID := idwrap.NewFromBytesMust(orderedWorkspaces[position].ID)
		newPrev = &prevID
		newNext = &nextID
		
		// Update prev item to point to new item
		err = repo.queries.UpdateWorkspaceNext(ctx, gen.UpdateWorkspaceNextParams{
			Next:   &itemID,       // Point to new item
			ID:     prevID,
			UserID: userID,
		})
		if err != nil {
			return fmt.Errorf("failed to update prev item: %w", err)
		}
		
		// Update next item to point back to new item
		err = repo.queries.UpdateWorkspacePrev(ctx, gen.UpdateWorkspacePrevParams{
			Prev:   &itemID,       // Point to new item
			ID:     nextID,
			UserID: userID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next item: %w", err)
		}
	}

	// Set the new item's position
	return repo.queries.UpdateWorkspaceOrder(ctx, gen.UpdateWorkspaceOrderParams{
		Prev:   newPrev,
		Next:   newNext,
		ID:     itemID,
		UserID: userID,
	})
}

// removeFromPosition removes a workspace from its current position in the linked list
// This is a helper method for operations that need to remove workspaces
func (r *WorkspaceMovableRepository) removeFromPosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, userID idwrap.IDWrap) error {
	// Get repository with transaction support
	repo := r
	if tx != nil {
		repo = r.TX(tx)
	}

	// Get the workspace to remove and its current prev/next pointers
	workspace, err := repo.queries.GetWorkspaceByUserIDandWorkspaceID(ctx, gen.GetWorkspaceByUserIDandWorkspaceIDParams{
		WorkspaceID: itemID,
		UserID:      userID,
	})
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}

	// Update prev workspace's next pointer to skip the removed workspace
	if workspace.Prev != nil {
		err = repo.queries.UpdateWorkspaceNext(ctx, gen.UpdateWorkspaceNextParams{
			Next:   workspace.Next, // Point to the removed workspace's next
			ID:     *workspace.Prev,
			UserID: userID,
		})
		if err != nil {
			return fmt.Errorf("failed to update previous workspace's next pointer: %w", err)
		}
	}

	// Update next workspace's prev pointer to skip the removed workspace
	if workspace.Next != nil {
		err = repo.queries.UpdateWorkspacePrev(ctx, gen.UpdateWorkspacePrevParams{
			Prev:   workspace.Prev, // Point to the removed workspace's prev
			ID:     *workspace.Next,
			UserID: userID,
		})
		if err != nil {
			return fmt.Errorf("failed to update next workspace's prev pointer: %w", err)
		}
	}

	// Clear the removed workspace's pointers
	err = repo.queries.UpdateWorkspaceOrder(ctx, gen.UpdateWorkspaceOrderParams{
		Prev:   nil,
		Next:   nil,
		ID:     itemID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to clear removed workspace's pointers: %w", err)
	}

	return nil
}