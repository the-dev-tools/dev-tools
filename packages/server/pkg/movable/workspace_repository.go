package movable

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
)

// WorkspaceRepository implements MovableRepository for Workspaces using the new
// idiomatic Go patterns with composition instead of inheritance
type WorkspaceRepository struct {
	// Composition: use SimpleRepository instead of inheriting from BaseLinkedListRepository
	queries *gen.Queries
	config  MoveConfigV2
}

// NewWorkspaceRepository creates a new WorkspaceRepository using composition pattern
func NewWorkspaceRepository(db *sql.DB, queries *gen.Queries) *WorkspaceRepository {
	// Build workspace-specific configuration using V2 config system
	config, err := NewConfigBuilderV2().
		ForJoinTable("workspaces", "workspace_users", "workspace_id", "user_id").
		WithValidation(DefaultValidationRulesV2()).
		WithPerformance(DefaultPerformanceConfigV2()).
		Build()
	
	if err != nil {
		// In production, this would be handled differently
		panic(fmt.Sprintf("Failed to build workspace configuration: %v", err))
	}
	
	return &WorkspaceRepository{
		queries: queries,
		config:  config,
	}
}

// TX returns a new repository instance with transaction support
// Implements the required TX method for transaction handling
func (r *WorkspaceRepository) TX(tx *sql.Tx) MovableRepository {
	return &WorkspaceRepository{
		queries: r.queries.WithTx(tx),
		config:  r.config,
	}
}

// UpdatePosition updates the position of a workspace in the linked list
// Uses pure functions and configuration for workspace-specific operations
func (r *WorkspaceRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType ListType, position int) error {
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*WorkspaceRepository)
	}
	
	// Get workspace users to determine parent scope
	workspaceUsers, err := repo.queries.GetWorkspaceUserByWorkspaceID(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get workspace users: %w", err)
	}
	if len(workspaceUsers) == 0 {
		return fmt.Errorf("workspace has no users")
	}
	
	// Use first user as parent scope
	userID := workspaceUsers[0].UserID
	
	// Get ordered workspaces for this user
	workspaceRows, err := repo.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to get ordered workspaces: %w", err)
	}
	
	// Convert to pure function types for calculation
	pureItems := make([]PureOrderable, len(workspaceRows))
	for i, ws := range workspaceRows {
		wsID := idwrap.NewFromBytesMust(ws.ID)
		var prev, next *PureID
		if ws.Prev != nil {
			prevID := PureID(idwrap.NewFromBytesMust(ws.Prev).String())
			prev = &prevID
		}
		if ws.Next != nil {
			nextID := PureID(idwrap.NewFromBytesMust(ws.Next).String())
			next = &nextID
		}
		
		pureItems[i] = PureOrderable{
			ID:       PureID(wsID.String()),
			Prev:     prev,
			Next:     next,
			ParentID: PureID(userID.String()),
			Position: PurePosition(ws.Position),
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
	
	// Apply updates using workspace-specific persistence
	return repo.persistWorkspaceUpdates(ctx, moveResult, userID)
}

// UpdatePositions updates positions for multiple workspaces in batch
func (r *WorkspaceRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []PositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*WorkspaceRepository)
	}
	
	// For simplicity, process updates sequentially
	// In production, would group by user for atomic processing
	for _, update := range updates {
		err := repo.UpdatePosition(ctx, tx, update.ItemID, update.ListType, update.Position)
		if err != nil {
			return fmt.Errorf("failed to update workspace %s: %w", update.ItemID.String(), err)
		}
	}
	
	return nil
}

// GetMaxPosition returns the maximum position value for workspaces for a user
func (r *WorkspaceRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType ListType) (int, error) {
	// For workspaces, parentID is the user_id
	workspaceRows, err := r.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   parentID,
		UserID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // No workspaces means no max position
		}
		return 0, fmt.Errorf("failed to get workspaces: %w", err)
	}
	
	if len(workspaceRows) == 0 {
		return -1, nil
	}
	
	return len(workspaceRows) - 1, nil
}

// GetItemsByParent returns all workspaces under a user, ordered by position
func (r *WorkspaceRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]MovableItem, error) {
	// For workspaces, parentID is the user_id
	workspaceRows, err := r.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   parentID,
		UserID_2: parentID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []MovableItem{}, nil
		}
		return nil, fmt.Errorf("failed to get workspaces: %w", err)
	}
	
	// Convert to MovableItem structs
	items := make([]MovableItem, len(workspaceRows))
	for i, ws := range workspaceRows {
		wsID := idwrap.NewFromBytesMust(ws.ID)
		items[i] = MovableItem{
			ID:       wsID,
			ParentID: &parentID, // user_id as parent
			Position: int(ws.Position),
			ListType: listType,
		}
	}
	
	return items, nil
}

// persistWorkspaceUpdates applies workspace updates using pure functions
func (r *WorkspaceRepository) persistWorkspaceUpdates(ctx context.Context, items []PureOrderable, userID idwrap.IDWrap) error {
	// Ensure pointers are consistent using pure function
	consistentItems := UpdatePrevNextPointers(items)
	
	// Apply each update atomically
	for _, item := range consistentItems {
		workspaceID, err := idwrap.NewText(string(item.ID))
		if err != nil {
			return fmt.Errorf("failed to parse workspace ID %s: %w", item.ID, err)
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
		
		err = r.queries.UpdateWorkspaceOrder(ctx, gen.UpdateWorkspaceOrderParams{
			ID:     workspaceID,
			Prev:   prev,
			Next:   next,
			UserID: userID,
		})
		
		if err != nil {
			return fmt.Errorf("failed to update workspace %s: %w", workspaceID.String(), err)
		}
	}
	
	return nil
}