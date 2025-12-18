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

type UserReader struct {
	queries *gen.Queries
}

func NewUserReader(db *sql.DB) *UserReader {
	return &UserReader{
		queries: gen.New(db),
	}
}

func NewUserReaderFromQueries(queries *gen.Queries) *UserReader {
	return &UserReader{
		queries: queries,
	}
}

func (r *UserReader) GetWorkspaceUser(ctx context.Context, id idwrap.IDWrap) (*mworkspace.WorkspaceUser, error) {
	wsuser, err := r.queries.GetWorkspaceUser(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceUserNotFound
		}
		return nil, err
	}

	return &mworkspace.WorkspaceUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        mworkspace.Role(wsuser.Role), // nolint:gosec // G115
	}, nil
}

func (r *UserReader) GetWorkspaceUserByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.WorkspaceUser, error) {
	rawWsUsers, err := r.queries.GetWorkspaceUserByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rawWsUsers, ConvertToModelWorkspaceUser), nil
}

func (r *UserReader) GetWorkspaceUserByWorkspaceID(ctx context.Context, wsID idwrap.IDWrap) ([]mworkspace.WorkspaceUser, error) {
	rawWsUsers, err := r.queries.GetWorkspaceUserByWorkspaceID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rawWsUsers, ConvertToModelWorkspaceUser), nil
}

func (r *UserReader) GetWorkspaceUsersByWorkspaceIDAndUserID(ctx context.Context, wsID, userID idwrap.IDWrap) (*mworkspace.WorkspaceUser, error) {
	wsu, err := r.queries.GetWorkspaceUserByWorkspaceIDAndUserID(ctx, gen.GetWorkspaceUserByWorkspaceIDAndUserIDParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceUserNotFound
		}
		return nil, err
	}
	workspace := ConvertToModelWorkspaceUser(wsu)
	return &workspace, nil
}
