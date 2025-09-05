package sworkspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"time"
)

var ErrNoWorkspaceFound = sql.ErrNoRows

type WorkspaceService struct {
	queries           *gen.Queries
	movableRepository *WorkspaceMovableRepository
}

func ConvertToDBWorkspace(workspace mworkspace.Workspace) gen.Workspace {
	return gen.Workspace{
		ID:              workspace.ID,
		Name:            workspace.Name,
		Updated:         workspace.Updated.Unix(),
		CollectionCount: workspace.CollectionCount,
		FlowCount:       workspace.FlowCount,
		ActiveEnv:       workspace.ActiveEnv,
		GlobalEnv:       workspace.GlobalEnv,
	}
}

func ConvertToModelWorkspace(workspace gen.Workspace) mworkspace.Workspace {
	return mworkspace.Workspace{
		ID:              workspace.ID,
		Name:            workspace.Name,
		Updated:         dbtime.DBTime(time.Unix(workspace.Updated, 0)),
		CollectionCount: workspace.CollectionCount,
		FlowCount:       workspace.FlowCount,
		ActiveEnv:       workspace.ActiveEnv,
		GlobalEnv:       workspace.GlobalEnv,
	}
}

func ConvertGetWorkspaceRowToModel(workspace gen.GetWorkspaceRow) mworkspace.Workspace {
	return mworkspace.Workspace{
		ID:              workspace.ID,
		Name:            workspace.Name,
		Updated:         dbtime.DBTime(time.Unix(workspace.Updated, 0)),
		CollectionCount: workspace.CollectionCount,
		FlowCount:       workspace.FlowCount,
		ActiveEnv:       workspace.ActiveEnv,
		GlobalEnv:       workspace.GlobalEnv,
	}
}

func ConvertGetWorkspacesByUserIDOrderedRowToModel(workspace gen.GetWorkspacesByUserIDOrderedRow) mworkspace.Workspace {
	return mworkspace.Workspace{
		ID:              idwrap.NewFromBytesMust(workspace.ID),
		Name:            workspace.Name,
		Updated:         dbtime.DBTime(time.Unix(workspace.Updated, 0)),
		CollectionCount: workspace.CollectionCount,
		FlowCount:       workspace.FlowCount,
		ActiveEnv:       idwrap.NewFromBytesMust(workspace.ActiveEnv),
		GlobalEnv:       idwrap.NewFromBytesMust(workspace.GlobalEnv),
	}
}

func New(queries *gen.Queries) WorkspaceService {
	movableRepo := NewWorkspaceMovableRepository(queries)

	return WorkspaceService{
		queries:           queries,
		movableRepository: movableRepo,
	}
}

func (ws WorkspaceService) TX(tx *sql.Tx) WorkspaceService {
	// Create new instances with transaction support
	txQueries := ws.queries.WithTx(tx)
	movableRepo := NewWorkspaceMovableRepository(txQueries)

	return WorkspaceService{
		queries:           txQueries,
		movableRepository: movableRepo,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*WorkspaceService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	movableRepo := NewWorkspaceMovableRepository(queries)

	return &WorkspaceService{
		queries:           queries,
		movableRepository: movableRepo,
	}, nil
}

func (ws WorkspaceService) Create(ctx context.Context, w *mworkspace.Workspace) error {
	dbWorkspace := ConvertToDBWorkspace(*w)

	// Create the workspace with initial NULL prev/next
	err := ws.queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              dbWorkspace.ID,
		Name:            dbWorkspace.Name,
		Updated:         dbWorkspace.Updated,
		CollectionCount: dbWorkspace.CollectionCount,
		FlowCount:       dbWorkspace.FlowCount,
		ActiveEnv:       dbWorkspace.ActiveEnv,
		GlobalEnv:       dbWorkspace.GlobalEnv,
		Prev:            nil, // Initially isolated
		Next:            nil, // Initially isolated
	})
	if err != nil {
		return err
	}

	// NOTE: Auto-linking will be handled by the RPC layer after workspace_user is created
	return nil
}

// AutoLinkWorkspaceToUserList links a newly created workspace into the user's existing workspace chain
// This prevents the workspace from being an isolated node that doesn't appear in the ordered query
// This method should be called after both workspace and workspace_user records are created
func (ws WorkspaceService) AutoLinkWorkspaceToUserList(ctx context.Context, workspaceID idwrap.IDWrap, userID idwrap.IDWrap) error {
	// If the new workspace already appears in the user's ordered chain (e.g., first workspace
	// was inserted and the list is empty so it becomes head automatically), skip linking.
	items, err := ws.movableRepository.GetItemsByParent(ctx, userID, movable.WorkspaceListTypeWorkspaces)
	if err == nil && movable.HasID(items, workspaceID) {
		return nil // already present; nothing to link
	}
	// Super-safe append using planner + guarded updates.
	plan, err := movable.BuildAppendPlanFromRepo(ctx, ws.movableRepository, userID, movable.WorkspaceListTypeWorkspaces, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace auto-link plan failed: %w", err)
	}

	// Patch tail.next if needed (best-effort; if race detected, bail quietly to avoid corruption).
	if plan.PrevID != nil {
		// Use UpdateWorkspaceNext to set tail's next.
		if err := ws.queries.UpdateWorkspaceNext(ctx, gen.UpdateWorkspaceNextParams{
			Next:   &workspaceID,
			ID:     *plan.PrevID,
			UserID: userID,
		}); err != nil {
			return fmt.Errorf("failed to set tail.next: %w", err)
		}
	}

	// Set new workspace's prev and next(nil) to become new tail.
	if err := ws.queries.UpdateWorkspaceOrder(ctx, gen.UpdateWorkspaceOrderParams{
		Prev:   plan.PrevID,
		Next:   nil,
		ID:     workspaceID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("failed to set new workspace prev/next: %w", err)
	}

	return nil
}

func (ws WorkspaceService) Get(ctx context.Context, id idwrap.IDWrap) (*mworkspace.Workspace, error) {
	workspaceRaw, err := ws.queries.GetWorkspace(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	workspace := ConvertGetWorkspaceRowToModel(workspaceRaw)
	return &workspace, nil
}

func (ws WorkspaceService) Update(ctx context.Context, org *mworkspace.Workspace) error {
	err := ws.queries.UpdateWorkspace(ctx, gen.UpdateWorkspaceParams{
		ID:              org.ID,
		Name:            org.Name,
		FlowCount:       org.FlowCount,
		CollectionCount: org.CollectionCount,
		Updated:         org.Updated.Unix(),
		ActiveEnv:       org.ActiveEnv,
	})
	if err == sql.ErrNoRows {
		return ErrNoWorkspaceFound
	}
	return err
}

func (ws WorkspaceService) UpdateUpdatedTime(ctx context.Context, org *mworkspace.Workspace) error {
	err := ws.queries.UpdateWorkspaceUpdatedTime(ctx, gen.UpdateWorkspaceUpdatedTimeParams{
		ID:      org.ID,
		Updated: org.Updated.Unix(),
	})
	if err == sql.ErrNoRows {
		return ErrNoWorkspaceFound
	}
	return err
}

func (ws WorkspaceService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	err := ws.queries.DeleteWorkspace(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoWorkspaceFound
	}
	return err
}

func (ws WorkspaceService) GetMultiByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	rawWorkspaces, err := ws.queries.GetWorkspacesByUserID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(rawWorkspaces, ConvertToModelWorkspace), nil
}

func (ws WorkspaceService) GetByIDandUserID(ctx context.Context, orgID, userID idwrap.IDWrap) (*mworkspace.Workspace, error) {
	workspaceRaw, err := ws.queries.GetWorkspaceByUserIDandWorkspaceID(ctx, gen.GetWorkspaceByUserIDandWorkspaceIDParams{
		UserID:      userID,
		WorkspaceID: orgID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	workspace := ConvertToModelWorkspace(workspaceRaw)
	return &workspace, nil
}

// GetWorkspacesByUserIDOrdered returns workspaces for a user in their proper order
func (ws WorkspaceService) GetWorkspacesByUserIDOrdered(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	rawWorkspaces, err := ws.queries.GetWorkspacesByUserIDOrdered(ctx, gen.GetWorkspacesByUserIDOrderedParams{
		UserID:   userID,
		UserID_2: userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(rawWorkspaces, ConvertGetWorkspacesByUserIDOrderedRowToModel), nil
}

// MoveWorkspace moves a workspace to a specific position for a user
func (ws WorkspaceService) MoveWorkspace(ctx context.Context, userID idwrap.IDWrap, workspaceID idwrap.IDWrap, position int) error {
	return ws.MoveWorkspaceTX(ctx, nil, userID, workspaceID, position)
}

// MoveWorkspaceTX moves a workspace to a specific position within a transaction
func (ws WorkspaceService) MoveWorkspaceTX(ctx context.Context, tx *sql.Tx, userID idwrap.IDWrap, workspaceID idwrap.IDWrap, position int) error {
	service := ws
	if tx != nil {
		service = ws.TX(tx)
	}

	// Validate user has access to the workspace
	_, err := service.GetByIDandUserID(ctx, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("user does not have access to workspace or workspace not found: %w", err)
	}

	// Use the movable repository to perform the position-based move
	err = service.movableRepository.UpdatePosition(ctx, tx, workspaceID, movable.WorkspaceListTypeWorkspaces, position)
	if err != nil {
		return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoWorkspaceFound, err)
	}

	return nil
}

// validateMoveOperation validates that a move operation is safe and valid
func (ws WorkspaceService) validateMoveOperation(ctx context.Context, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	if workspaceID.Compare(targetWorkspaceID) == 0 {
		return errors.New("cannot move workspace relative to itself")
	}

	// Validate user has access to both workspaces
	_, err := ws.GetByIDandUserID(ctx, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("user does not have access to source workspace: %w", err)
	}

	_, err = ws.GetByIDandUserID(ctx, targetWorkspaceID, userID)
	if err != nil {
		return fmt.Errorf("user does not have access to target workspace: %w", err)
	}

	return nil
}

// MoveWorkspaceAfter moves a workspace to be positioned after the target workspace
func (ws WorkspaceService) MoveWorkspaceAfter(ctx context.Context, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	return ws.MoveWorkspaceAfterTX(ctx, nil, userID, workspaceID, targetWorkspaceID)
}

// MoveWorkspaceAfterTX moves a workspace to be positioned after the target workspace within a transaction
func (ws WorkspaceService) MoveWorkspaceAfterTX(ctx context.Context, tx *sql.Tx, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	service := ws
	if tx != nil {
		service = ws.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, userID, workspaceID, targetWorkspaceID); err != nil {
		return err
	}

	// Get all workspaces for the user in order
	orderedWorkspaces, err := service.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get workspaces in order: %w", err)
	}

	// Find positions of source and target workspaces
	targetPosition := -1
	for i, workspace := range orderedWorkspaces {
		if workspace.ID.Compare(targetWorkspaceID) == 0 {
			targetPosition = i
			break
		}
	}

	if targetPosition == -1 {
		return fmt.Errorf("target workspace not found")
	}

	// Calculate new position: after target (position + 1, but clamped to end)
	newPosition := targetPosition + 1
	if newPosition >= len(orderedWorkspaces) {
		newPosition = len(orderedWorkspaces) - 1
	}

	// Execute move using movable repository
	err = service.movableRepository.UpdatePosition(ctx, tx, workspaceID, movable.WorkspaceListTypeWorkspaces, newPosition)
	if err != nil {
		return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoWorkspaceFound, err)
	}

	return nil
}

// MoveWorkspaceBefore moves a workspace to be positioned before the target workspace
func (ws WorkspaceService) MoveWorkspaceBefore(ctx context.Context, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	return ws.MoveWorkspaceBeforeTX(ctx, nil, userID, workspaceID, targetWorkspaceID)
}

// MoveWorkspaceBeforeTX moves a workspace to be positioned before the target workspace within a transaction
func (ws WorkspaceService) MoveWorkspaceBeforeTX(ctx context.Context, tx *sql.Tx, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	service := ws
	if tx != nil {
		service = ws.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, userID, workspaceID, targetWorkspaceID); err != nil {
		return err
	}

	// Get all workspaces for the user in order
	orderedWorkspaces, err := service.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get workspaces in order: %w", err)
	}

	// Find positions of source and target workspaces
	targetPosition := -1
	for i, workspace := range orderedWorkspaces {
		if workspace.ID.Compare(targetWorkspaceID) == 0 {
			targetPosition = i
			break
		}
	}

	if targetPosition == -1 {
		return fmt.Errorf("target workspace not found")
	}

	// Calculate new position: before target (same position, target will shift)
	newPosition := targetPosition

	// Execute move using movable repository
	err = service.movableRepository.UpdatePosition(ctx, tx, workspaceID, movable.WorkspaceListTypeWorkspaces, newPosition)
	if err != nil {
		return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoWorkspaceFound, err)
	}

	return nil
}

// ReorderWorkspaces performs a bulk reorder of workspaces using the movable system
func (ws WorkspaceService) ReorderWorkspaces(ctx context.Context, userID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	return ws.ReorderWorkspacesTX(ctx, nil, userID, orderedIDs)
}

// ReorderWorkspacesTX performs a bulk reorder of workspaces using the movable system within a transaction
func (ws WorkspaceService) ReorderWorkspacesTX(ctx context.Context, tx *sql.Tx, userID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	service := ws
	if tx != nil {
		service = ws.TX(tx)
	}

	// Validate user has access to all workspaces
	for _, workspaceID := range orderedIDs {
		_, err := service.GetByIDandUserID(ctx, workspaceID, userID)
		if err != nil {
			return fmt.Errorf("user does not have access to workspace %s: %w", workspaceID.String(), err)
		}
	}

	// Build position updates using the workspace list type
	updates := make([]movable.PositionUpdate, len(orderedIDs))
	for i, workspaceID := range orderedIDs {
		updates[i] = movable.PositionUpdate{
			ItemID:   workspaceID,
			ListType: movable.WorkspaceListTypeWorkspaces,
			Position: i,
		}
	}

	// Execute the batch update using the movable repository
	err := service.movableRepository.UpdatePositions(ctx, tx, updates)
	if err != nil {
		return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoWorkspaceFound, err)
	}

	return nil
}

// Repository returns the movable repository for advanced operations
func (ws WorkspaceService) Repository() *WorkspaceMovableRepository {
	return ws.movableRepository
}
