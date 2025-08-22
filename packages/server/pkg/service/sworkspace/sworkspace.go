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
	// TODO: Re-enable when Phase 1-2 database schema is implemented
	// "the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"time"
)

var ErrNoWorkspaceFound = sql.ErrNoRows

type WorkspaceService struct {
	queries           *gen.Queries
	// TODO: Re-enable when Phase 1-2 database schema is implemented
	// movableRepository *WorkspaceMovableRepository
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

func New(queries *gen.Queries) WorkspaceService {
	// TODO: Create the movable repository for workspaces when Phase 1-2 is implemented
	// movableRepo := NewWorkspaceMovableRepository(queries)
	
	return WorkspaceService{
		queries: queries,
		// movableRepository: movableRepo,
	}
}

func (ws WorkspaceService) TX(tx *sql.Tx) WorkspaceService {
	// Create new instances with transaction support
	txQueries := ws.queries.WithTx(tx)
	// TODO: Create movable repository when Phase 1-2 is implemented
	// movableRepo := NewWorkspaceMovableRepository(txQueries)
	
	return WorkspaceService{
		queries: txQueries,
		// movableRepository: movableRepo,
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
	
	// TODO: Create movable repository when Phase 1-2 is implemented
	// movableRepo := NewWorkspaceMovableRepository(queries)
	
	return &WorkspaceService{
		queries: queries,
		// movableRepository: movableRepo,
	}, nil
}

func (ws WorkspaceService) Create(ctx context.Context, w *mworkspace.Workspace) error {
	dbWorkspace := ConvertToDBWorkspace(*w)
	return ws.queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              dbWorkspace.ID,
		Name:            dbWorkspace.Name,
		Updated:         dbWorkspace.Updated,
		CollectionCount: dbWorkspace.CollectionCount,
		FlowCount:       dbWorkspace.FlowCount,
		ActiveEnv:       dbWorkspace.ActiveEnv,
		GlobalEnv:       dbWorkspace.GlobalEnv,
	})
}

func (ws WorkspaceService) Get(ctx context.Context, id idwrap.IDWrap) (*mworkspace.Workspace, error) {
	workspaceRaw, err := ws.queries.GetWorkspace(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	workspace := ConvertToModelWorkspace(workspaceRaw)
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
// Currently falls back to regular GetMultiByUserID until Phase 1-2 database schema is implemented
func (ws WorkspaceService) GetWorkspacesByUserIDOrdered(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	// TODO: Once Phase 1-2 is completed and workspace ordering tables exist, replace this with:
	// return ws.queries.GetWorkspacesByUserIDOrdered(ctx, userID)
	
	// For now, fall back to regular GetMultiByUserID since ordering tables don't exist yet
	return ws.GetMultiByUserID(ctx, userID)
}

// MoveWorkspace moves a workspace to a specific position for a user
func (ws WorkspaceService) MoveWorkspace(ctx context.Context, userID idwrap.IDWrap, workspaceID idwrap.IDWrap, position int) error {
	return ws.MoveWorkspaceTX(ctx, nil, userID, workspaceID, position)
}

// MoveWorkspaceTX moves a workspace to a specific position within a transaction
// Currently validates access but doesn't reorder until Phase 1-2 database schema is implemented
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

	// TODO: Once Phase 1-2 is completed and workspace ordering tables exist, implement actual position update:
	// Use the movable repository to perform the position-based move
	// err = service.movableRepository.UpdatePosition(ctx, tx, workspaceID, movable.WorkspaceListTypeWorkspaces, position)
	// if err != nil {
	//     return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoWorkspaceFound, err)
	// }

	// For now, just validate that the workspace exists and user has access
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
// Currently returns success without reordering until Phase 1-2 database schema is implemented
func (ws WorkspaceService) MoveWorkspaceAfter(ctx context.Context, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	return ws.MoveWorkspaceAfterTX(ctx, nil, userID, workspaceID, targetWorkspaceID)
}

// MoveWorkspaceAfterTX moves a workspace to be positioned after the target workspace within a transaction
// Currently validates access but doesn't reorder until Phase 1-2 database schema is implemented
func (ws WorkspaceService) MoveWorkspaceAfterTX(ctx context.Context, tx *sql.Tx, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	service := ws
	if tx != nil {
		service = ws.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, userID, workspaceID, targetWorkspaceID); err != nil {
		return err
	}

	// TODO: Once Phase 1-2 is completed and workspace ordering tables exist, implement actual reordering:
	// 1. Get all workspaces for the user in order
	// 2. Find positions of source and target workspaces  
	// 3. Calculate new order: move source to be after target
	// 4. Execute batch update using movable repository
	
	// For now, just validate that the operation is valid and return success
	return nil
}

// MoveWorkspaceBefore moves a workspace to be positioned before the target workspace
func (ws WorkspaceService) MoveWorkspaceBefore(ctx context.Context, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	return ws.MoveWorkspaceBeforeTX(ctx, nil, userID, workspaceID, targetWorkspaceID)
}

// MoveWorkspaceBeforeTX moves a workspace to be positioned before the target workspace within a transaction
// Currently validates access but doesn't reorder until Phase 1-2 database schema is implemented
func (ws WorkspaceService) MoveWorkspaceBeforeTX(ctx context.Context, tx *sql.Tx, userID, workspaceID, targetWorkspaceID idwrap.IDWrap) error {
	service := ws
	if tx != nil {
		service = ws.TX(tx)
	}

	// Validate the move operation
	if err := service.validateMoveOperation(ctx, userID, workspaceID, targetWorkspaceID); err != nil {
		return err
	}

	// TODO: Once Phase 1-2 is completed and workspace ordering tables exist, implement actual reordering:
	// 1. Get all workspaces for the user in order
	// 2. Find positions of source and target workspaces
	// 3. Calculate new order: move source to be before target
	// 4. Execute batch update using movable repository
	
	// For now, just validate that the operation is valid and return success
	return nil
}

// ReorderWorkspaces performs a bulk reorder of workspaces using the movable system
func (ws WorkspaceService) ReorderWorkspaces(ctx context.Context, userID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	return ws.ReorderWorkspacesTX(ctx, nil, userID, orderedIDs)
}

// ReorderWorkspacesTX performs a bulk reorder of workspaces using the movable system within a transaction
// Currently validates access but doesn't reorder until Phase 1-2 database schema is implemented
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

	// TODO: Once Phase 1-2 is completed and workspace ordering tables exist, implement actual reordering:
	// 1. Build position updates using the workspace list type
	// 2. Execute the batch update using the movable repository
	
	// For now, just validate that all workspaces are accessible and return success
	return nil
}

// Repository returns the movable repository for advanced operations
// TODO: Re-enable when Phase 1-2 database schema is implemented
// func (ws WorkspaceService) Repository() *WorkspaceMovableRepository {
//	return ws.movableRepository
// }
