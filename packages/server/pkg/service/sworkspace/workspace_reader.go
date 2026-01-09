package sworkspace

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type WorkspaceReader struct {
	queries *gen.Queries
}

func NewWorkspaceReader(db *sql.DB) *WorkspaceReader {
	return &WorkspaceReader{
		queries: gen.New(db),
	}
}

func NewWorkspaceReaderFromQueries(queries *gen.Queries) *WorkspaceReader {
	return &WorkspaceReader{
		queries: queries,
	}
}

func (r *WorkspaceReader) Get(ctx context.Context, id idwrap.IDWrap) (*mworkspace.Workspace, error) {
	workspaceRaw, err := r.queries.GetWorkspace(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	workspace := ConvertToModelWorkspace(workspaceRaw)
	return &workspace, nil
}

func (r *WorkspaceReader) GetMultiByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	rawWorkspaces, err := r.queries.GetWorkspacesByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(rawWorkspaces, ConvertToModelWorkspace), nil
}

func (r *WorkspaceReader) GetByIDandUserID(ctx context.Context, orgID, userID idwrap.IDWrap) (*mworkspace.Workspace, error) {
	workspaceRaw, err := r.queries.GetWorkspaceByUserIDandWorkspaceID(ctx, gen.GetWorkspaceByUserIDandWorkspaceIDParams{
		UserID:      userID,
		WorkspaceID: orgID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	workspace := ConvertToModelWorkspace(workspaceRaw)
	return &workspace, nil
}

func (r *WorkspaceReader) GetWorkspacesByUserIDOrdered(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	rawWorkspaces, err := r.queries.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(rawWorkspaces, ConvertToModelWorkspace), nil
}
