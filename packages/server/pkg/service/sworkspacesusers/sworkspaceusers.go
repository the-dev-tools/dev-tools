//nolint:revive // exported
package sworkspacesusers

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
)

var ErrWorkspaceUserNotFound = errors.New("workspace user not found")

type WorkspaceUserService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) WorkspaceUserService {
	return WorkspaceUserService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (wsu WorkspaceUserService) TX(tx *sql.Tx) WorkspaceUserService {
	newQueries := wsu.queries.WithTx(tx)
	return WorkspaceUserService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*WorkspaceUserService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &WorkspaceUserService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (wsu WorkspaceUserService) CreateWorkspaceUser(ctx context.Context, user *mworkspaceuser.WorkspaceUser) error {
	return NewWriterFromQueries(wsu.queries).CreateWorkspaceUser(ctx, user)
}

func (wsu WorkspaceUserService) GetWorkspaceUser(ctx context.Context, id idwrap.IDWrap) (*mworkspaceuser.WorkspaceUser, error) {
	return wsu.reader.GetWorkspaceUser(ctx, id)
}

func (wsu WorkspaceUserService) UpdateWorkspaceUser(ctx context.Context, wsuser *mworkspaceuser.WorkspaceUser) error {
	return NewWriterFromQueries(wsu.queries).UpdateWorkspaceUser(ctx, wsuser)
}

func (wsu WorkspaceUserService) DeleteWorkspaceUser(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(wsu.queries).DeleteWorkspaceUser(ctx, id)
}

func (wsus WorkspaceUserService) GetWorkspaceUserByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspaceuser.WorkspaceUser, error) {
	return wsus.reader.GetWorkspaceUserByUserID(ctx, userID)
}

func (wsus WorkspaceUserService) GetWorkspaceUserByWorkspaceID(ctx context.Context, wsID idwrap.IDWrap) ([]mworkspaceuser.WorkspaceUser, error) {
	return wsus.reader.GetWorkspaceUserByWorkspaceID(ctx, wsID)
}

func (wsus WorkspaceUserService) GetWorkspaceUsersByWorkspaceIDAndUserID(ctx context.Context, wsID, userID idwrap.IDWrap) (*mworkspaceuser.WorkspaceUser, error) {
	return wsus.reader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, wsID, userID)
}

// is a greater than b
func IsPermGreater(a, b *mworkspaceuser.WorkspaceUser) (bool, error) {
	if a.Role > mworkspaceuser.RoleOwner || b.Role > mworkspaceuser.RoleOwner {
		return false, errors.New("invalid role")
	}
	return a.Role > b.Role, nil
}
