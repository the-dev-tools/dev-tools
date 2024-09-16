package sworkspace

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
	"time"
)

var ErrNoWorkspaceFound = sql.ErrNoRows

type WorkspaceService struct {
	queries *gen.Queries
}

func ConvertToDBWorkspace(workspace mworkspace.Workspace) gen.Workspace {
	return gen.Workspace{
		ID:      workspace.ID,
		Name:    workspace.Name,
		Updated: workspace.Updated.Unix(),
	}
}

func ConvertToModelWorkspace(workspace gen.Workspace) mworkspace.Workspace {
	return mworkspace.Workspace{
		ID:      workspace.ID,
		Name:    workspace.Name,
		Updated: time.Unix(workspace.Updated, 0),
	}
}

func New(ctx context.Context, db *sql.DB) (*WorkspaceService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	return &WorkspaceService{
		queries: queries,
	}, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*WorkspaceService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return &WorkspaceService{
		queries: queries,
	}, nil
}

func (ws WorkspaceService) Create(ctx context.Context, w *mworkspace.Workspace) error {
	dbWorkspace := ConvertToDBWorkspace(*w)
	return ws.queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:      dbWorkspace.ID,
		Name:    dbWorkspace.Name,
		Updated: dbWorkspace.Updated,
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
		ID:   org.ID,
		Name: org.Name,
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
