package sworkspacesusers

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-db/pkg/sqlc/gen"
	"errors"

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
		ID:          user.ID,
		WorkspaceID: user.WorkspaceID,
		UserID:      user.UserID,
	})
}

func (wsu WorkspaceUserService) GetWorkspaceUser(ctx context.Context, id ulid.ULID) (*mworkspaceuser.WorkspaceUser, error) {
	wsuser, err := wsu.queries.GetWorkspaceUser(ctx, id)
	if err != nil {
		return nil, err
	}

	return &mworkspaceuser.WorkspaceUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
	}, nil
}

func (wsu WorkspaceUserService) UpdateWorkspaceUser(ctx context.Context, wsuser *mworkspaceuser.WorkspaceUser) error {
	return wsu.queries.UpdateWorkspaceUser(ctx, gen.UpdateWorkspaceUserParams{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
	})
}

func (wsu WorkspaceUserService) DeleteWorkspaceUser(ctx context.Context, id ulid.ULID) error {
	return wsu.queries.DeleteWorkspaceUser(ctx, id)
}

func (wsus WorkspaceUserService) GetWorkspaceUserByUserID(ctx context.Context, userID ulid.ULID) ([]mworkspaceuser.WorkspaceUser, error) {
	rawWsUsers, err := wsus.queries.GetWorkspaceUserByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	wsUsers := make([]mworkspaceuser.WorkspaceUser, len(rawWsUsers))
	for i, rawWsUser := range rawWsUsers {
		wsUsers[i] = mworkspaceuser.WorkspaceUser{
			ID:          ulid.ULID(rawWsUser.ID),
			WorkspaceID: ulid.ULID(rawWsUser.WorkspaceID),
			UserID:      ulid.ULID(rawWsUser.UserID),
		}
	}
	return wsUsers, nil
}

func (wsus WorkspaceUserService) GetWorkspaceUserByWorkspaceID(ctx context.Context, wsID ulid.ULID) ([]mworkspaceuser.WorkspaceUser, error) {
	rawWsUsers, err := wsus.queries.GetWorkspaceUserByWorkspaceID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	wsUsers := make([]mworkspaceuser.WorkspaceUser, len(rawWsUsers))
	for i, rawWsUser := range rawWsUsers {
		wsUsers[i] = mworkspaceuser.WorkspaceUser{
			ID:          ulid.ULID(rawWsUser.ID),
			WorkspaceID: ulid.ULID(rawWsUser.WorkspaceID),
			UserID:      ulid.ULID(rawWsUser.UserID),
		}
	}
	return wsUsers, nil
}

func (wsus WorkspaceUserService) GetWorkspaceUsersByWorkspaceIDAndUserID(ctx context.Context, wsID, userID ulid.ULID) (*mworkspaceuser.WorkspaceUser, error) {
	wsu, err := wsus.queries.GetWorkspaceUserByWorkspaceIDAndUserID(ctx, gen.GetWorkspaceUserByWorkspaceIDAndUserIDParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	return &mworkspaceuser.WorkspaceUser{
		ID:          ulid.ULID(wsu.ID),
		WorkspaceID: ulid.ULID(wsu.WorkspaceID),
		UserID:      ulid.ULID(wsu.UserID),
	}, nil
}

// is a greater than b
func IsPermGreater(a, b *mworkspaceuser.WorkspaceUser) (bool, error) {
	if a.Role > mworkspaceuser.RoleOwner || b.Role > mworkspaceuser.RoleOwner {
		return false, errors.New("Invalid role")
	}
	return a.Role > b.Role, nil
}
