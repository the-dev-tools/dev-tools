package sworkspacesusers

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
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

func (r *Reader) GetWorkspaceUser(ctx context.Context, id idwrap.IDWrap) (*mworkspaceuser.WorkspaceUser, error) {
	wsuser, err := r.queries.GetWorkspaceUser(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceUserNotFound
		}
		return nil, err
	}

	return &mworkspaceuser.WorkspaceUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        mworkspaceuser.Role(wsuser.Role), // nolint:gosec // G115
	}, nil
}

func (r *Reader) GetWorkspaceUserByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspaceuser.WorkspaceUser, error) {
	rawWsUsers, err := r.queries.GetWorkspaceUserByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rawWsUsers, ConvertToModelWorkspaceUser), nil
}

func (r *Reader) GetWorkspaceUserByWorkspaceID(ctx context.Context, wsID idwrap.IDWrap) ([]mworkspaceuser.WorkspaceUser, error) {
	rawWsUsers, err := r.queries.GetWorkspaceUserByWorkspaceID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rawWsUsers, ConvertToModelWorkspaceUser), nil
}

func (r *Reader) GetWorkspaceUsersByWorkspaceIDAndUserID(ctx context.Context, wsID, userID idwrap.IDWrap) (*mworkspaceuser.WorkspaceUser, error) {
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
