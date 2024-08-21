package sworkspacesusers

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type WorkspaceUserService struct {
	DB      *sql.DB
	queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (*WorkspaceUserService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	return &WorkspaceUserService{
		DB:      db,
		queries: queries,
	}, nil
}

func (wsu WorkspaceUserService) CreateWorkspaceUser(ctx context.Context, user *mworkspaceuser.WorkspaceUser) error {
	return wsu.queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          user.ID.Bytes(),
		WorkspaceID: user.WorkspaceID.Bytes(),
		UserID:      user.UserID.Bytes(),
	})
}

func (wsu WorkspaceUserService) GetWorkspaceUser(ctx context.Context, id ulid.ULID) (*mworkspaceuser.WorkspaceUser, error) {
	wsuser, err := wsu.queries.GetWorkspaceUser(ctx, id.Bytes())
	if err != nil {
		return nil, err
	}

	return &mworkspaceuser.WorkspaceUser{
		ID:          ulid.MustParse(string(wsuser.ID)),
		WorkspaceID: ulid.MustParse(string(wsuser.WorkspaceID)),
		UserID:      ulid.MustParse(string(wsuser.UserID)),
	}, nil
}

func (wsu WorkspaceUserService) UpdateWorkspaceUser(ctx context.Context, wsuser *mworkspaceuser.WorkspaceUser) error {
	return wsu.queries.UpdateWorkspaceUser(ctx, gen.UpdateWorkspaceUserParams{
		ID:          wsuser.ID.Bytes(),
		WorkspaceID: wsuser.WorkspaceID.Bytes(),
		UserID:      wsuser.UserID.Bytes(),
	})
}

func (wsu WorkspaceUserService) DeleteWorkspaceUser(ctx context.Context, id ulid.ULID) error {
	return wsu.queries.DeleteWorkspaceUser(ctx, id.Bytes())
}

func (wsus WorkspaceUserService) GetWorkspaceUserByUserID(ctx context.Context, userID ulid.ULID) (*mworkspaceuser.WorkspaceUser, error) {
	wsUser, err := wsus.queries.GetWorkspaceUserByUserID(ctx, userID.Bytes())
	if err != nil {
		return nil, err
	}

	return &mworkspaceuser.WorkspaceUser{
		ID:          ulid.ULID(wsUser.ID),
		WorkspaceID: ulid.ULID(wsUser.WorkspaceID),
		UserID:      ulid.ULID(wsUser.UserID),
	}, nil
}

func (wsus WorkspaceUserService) GetWorkspaceUserByWorkspaceID(wsID ulid.ULID) (*mworkspaceuser.WorkspaceUser, error) {
	wsu, err := wsus.queries.GetWorkspaceUserByWorkspaceID(context.Background(), wsID.Bytes())
	if err != nil {
		return nil, err
	}
	return &mworkspaceuser.WorkspaceUser{
		ID:          ulid.ULID(wsu.ID),
		WorkspaceID: ulid.ULID(wsu.WorkspaceID),
		UserID:      ulid.ULID(wsu.UserID),
	}, nil
}
