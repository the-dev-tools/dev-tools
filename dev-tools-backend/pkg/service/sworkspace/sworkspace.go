package sworkspace

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

var ErrNoWorkspaceFound = sql.ErrNoRows

type WorkspaceService struct {
	DB      *sql.DB
	queries *gen.Queries
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
		DB:      db,
		queries: queries,
	}, nil
}

func (ws WorkspaceService) Create(ctx context.Context, org *mworkspace.Workspace) error {
	return ws.queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:      org.ID,
		Name:    org.Name,
		Created: org.Created,
		Updated: org.Updated,
	})
}

func (ws WorkspaceService) Get(ctx context.Context, id ulid.ULID) (*mworkspace.Workspace, error) {
	workspace, err := ws.queries.GetWorkspace(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	return &mworkspace.Workspace{
		ID:      ulid.ULID(workspace.ID),
		Name:    workspace.Name,
		Created: workspace.Created,
		Updated: workspace.Updated,
	}, nil
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

func (ws WorkspaceService) Delete(ctx context.Context, id ulid.ULID) error {
	err := ws.queries.DeleteWorkspace(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoWorkspaceFound
	}
	return err
}

// TODO: this cannot be one to many should be many to many when queries
func (ws WorkspaceService) GetByUserID(ctx context.Context, userID ulid.ULID) (*mworkspace.Workspace, error) {
	workspace, err := ws.queries.GetWorkspaceByUserID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	return &mworkspace.Workspace{
		ID:   ulid.ULID(workspace.ID),
		Name: workspace.Name,
	}, nil
}

func (ws WorkspaceService) GetMultiByUserID(ctx context.Context, userID ulid.ULID) ([]mworkspace.Workspace, error) {
	rawWorkspaces, err := ws.queries.GetWorkspacesByUserID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	var workspaces []mworkspace.Workspace
	for _, rawWorkspace := range rawWorkspaces {
		workspaces = append(workspaces, mworkspace.Workspace{
			ID:   ulid.ULID(rawWorkspace.ID),
			Name: rawWorkspace.Name,
		})
	}

	return workspaces, nil
}

func (ws WorkspaceService) GetByIDandUserID(ctx context.Context, orgID, userID ulid.ULID) (*mworkspace.Workspace, error) {
	workspace, err := ws.queries.GetWorkspaceByUserIDandWorkspaceID(ctx, gen.GetWorkspaceByUserIDandWorkspaceIDParams{
		UserID:      userID,
		WorkspaceID: orgID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return &mworkspace.Workspace{
		ID:   ulid.ULID(workspace.ID),
		Name: workspace.Name,
	}, nil
}
