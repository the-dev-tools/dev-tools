//nolint:revive // exported
package sworkspace

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
)

var ErrWorkspaceUserNotFound = errors.New("workspace user not found")

type UserService struct {
	reader  *UserReader
	queries *gen.Queries
}

func NewUserService(queries *gen.Queries) UserService {
	return UserService{
		reader:  NewUserReaderFromQueries(queries),
		queries: queries,
	}
}

func (wsu UserService) TX(tx *sql.Tx) UserService {
	newQueries := wsu.queries.WithTx(tx)
	return UserService{
		reader:  NewUserReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewUserServiceTX(ctx context.Context, tx *sql.Tx) (*UserService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &UserService{
		reader:  NewUserReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (wsu UserService) CreateWorkspaceUser(ctx context.Context, user *mworkspace.WorkspaceUser) error {
	return NewUserWriterFromQueries(wsu.queries).CreateWorkspaceUser(ctx, user)
}

func (wsu UserService) GetWorkspaceUser(ctx context.Context, id idwrap.IDWrap) (*mworkspace.WorkspaceUser, error) {
	return wsu.reader.GetWorkspaceUser(ctx, id)
}

func (wsu UserService) UpdateWorkspaceUser(ctx context.Context, wsuser *mworkspace.WorkspaceUser) error {
	return NewUserWriterFromQueries(wsu.queries).UpdateWorkspaceUser(ctx, wsuser)
}

func (wsu UserService) DeleteWorkspaceUser(ctx context.Context, id idwrap.IDWrap) error {
	return NewUserWriterFromQueries(wsu.queries).DeleteWorkspaceUser(ctx, id)
}

func (wsus UserService) GetWorkspaceUserByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.WorkspaceUser, error) {
	return wsus.reader.GetWorkspaceUserByUserID(ctx, userID)
}

func (wsus UserService) GetWorkspaceUserByWorkspaceID(ctx context.Context, wsID idwrap.IDWrap) ([]mworkspace.WorkspaceUser, error) {
	return wsus.reader.GetWorkspaceUserByWorkspaceID(ctx, wsID)
}

func (wsus UserService) GetWorkspaceUsersByWorkspaceIDAndUserID(ctx context.Context, wsID, userID idwrap.IDWrap) (*mworkspace.WorkspaceUser, error) {
	return wsus.reader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, wsID, userID)
}

// is a greater than b
func IsPermGreater(a, b *mworkspace.WorkspaceUser) (bool, error) {
	if a.Role > mworkspace.RoleOwner || b.Role > mworkspace.RoleOwner {
		return false, errors.New("invalid role")
	}
	return a.Role > b.Role, nil
}

func (s UserService) Reader() *UserReader { return s.reader }
