package sworkspacesusers

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
	"errors"
)

var ErrWorkspaceUserNotFound = errors.New("workspace user not found")

func ConvertToDBWorkspaceUser(wsuser mworkspaceuser.WorkspaceUser) gen.WorkspacesUser {
	return gen.WorkspacesUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        int8(wsuser.Role),
	}
}

func ConvertToModelWorkspaceUser(wsuser gen.WorkspacesUser) mworkspaceuser.WorkspaceUser {
	return mworkspaceuser.WorkspaceUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        mworkspaceuser.Role(wsuser.Role),
	}
}

type WorkspaceUserService struct {
	queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (WorkspaceUserService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return WorkspaceUserService{}, err
	}
	return WorkspaceUserService{
		queries: queries,
	}, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*WorkspaceUserService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &WorkspaceUserService{
		queries: queries,
	}, nil
}

func (wsu WorkspaceUserService) CreateWorkspaceUser(ctx context.Context, user *mworkspaceuser.WorkspaceUser) error {
	return wsu.queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          user.ID,
		WorkspaceID: user.WorkspaceID,
		UserID:      user.UserID,
		Role:        int8(user.Role),
	})
}

func (wsu WorkspaceUserService) GetWorkspaceUser(ctx context.Context, id idwrap.IDWrap) (*mworkspaceuser.WorkspaceUser, error) {
	wsuser, err := wsu.queries.GetWorkspaceUser(ctx, id)
	if err != nil {
		return nil, err
	}

	return &mworkspaceuser.WorkspaceUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        mworkspaceuser.Role(wsuser.Role),
	}, nil
}

func (wsu WorkspaceUserService) UpdateWorkspaceUser(ctx context.Context, wsuser *mworkspaceuser.WorkspaceUser) error {
	return wsu.queries.UpdateWorkspaceUser(ctx, gen.UpdateWorkspaceUserParams{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
	})
}

func (wsu WorkspaceUserService) DeleteWorkspaceUser(ctx context.Context, id idwrap.IDWrap) error {
	return wsu.queries.DeleteWorkspaceUser(ctx, id)
}

func (wsus WorkspaceUserService) GetWorkspaceUserByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspaceuser.WorkspaceUser, error) {
	rawWsUsers, err := wsus.queries.GetWorkspaceUserByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rawWsUsers, ConvertToModelWorkspaceUser), nil
}

func (wsus WorkspaceUserService) GetWorkspaceUserByWorkspaceID(ctx context.Context, wsID idwrap.IDWrap) ([]mworkspaceuser.WorkspaceUser, error) {
	rawWsUsers, err := wsus.queries.GetWorkspaceUserByWorkspaceID(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(rawWsUsers, ConvertToModelWorkspaceUser), nil
}

func (wsus WorkspaceUserService) GetWorkspaceUsersByWorkspaceIDAndUserID(ctx context.Context, wsID, userID idwrap.IDWrap) (*mworkspaceuser.WorkspaceUser, error) {
	wsu, err := wsus.queries.GetWorkspaceUserByWorkspaceIDAndUserID(ctx, gen.GetWorkspaceUserByWorkspaceIDAndUserIDParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	workspace := ConvertToModelWorkspaceUser(wsu)
	return &workspace, nil
}

// is a greater than b
func IsPermGreater(a, b *mworkspaceuser.WorkspaceUser) (bool, error) {
	if a.Role > mworkspaceuser.RoleOwner || b.Role > mworkspaceuser.RoleOwner {
		return false, errors.New("Invalid role")
	}
	return a.Role > b.Role, nil
}
