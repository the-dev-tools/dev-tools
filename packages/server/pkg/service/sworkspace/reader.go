package sworkspace

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type Reader struct {
	queries *gen.Queries
}

func NewReader(db *sql.DB) *Reader {
	return &Reader{
		queries: gen.New(db),
	}
}

func NewReaderFromQueries(queries *gen.Queries) *Reader {
	return &Reader{
		queries: queries,
	}
}

func (r *Reader) Get(ctx context.Context, id idwrap.IDWrap) (*mworkspace.Workspace, error) {
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

func (r *Reader) GetMultiByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	rawWorkspaces, err := r.queries.GetWorkspacesByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(rawWorkspaces, ConvertToModelWorkspace), nil
}

func (r *Reader) GetByIDandUserID(ctx context.Context, orgID, userID idwrap.IDWrap) (*mworkspace.Workspace, error) {
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

func (r *Reader) GetWorkspacesByUserIDOrdered(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	rawWorkspaces, err := r.queries.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(rawWorkspaces, ConvertToModelWorkspace), nil
}
