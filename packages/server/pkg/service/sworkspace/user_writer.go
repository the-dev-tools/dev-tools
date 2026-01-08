package sworkspace

import (
	"context"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
)

type UserWriter struct {
	queries *gen.Queries
}

func NewUserWriter(tx gen.DBTX) *UserWriter {
	return &UserWriter{
		queries: gen.New(tx),
	}
}

func NewUserWriterFromQueries(queries *gen.Queries) *UserWriter {
	return &UserWriter{
		queries: queries,
	}
}

func (w *UserWriter) CreateWorkspaceUser(ctx context.Context, user *mworkspace.WorkspaceUser) error {
	return w.queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          user.ID,
		WorkspaceID: user.WorkspaceID,
		UserID:      user.UserID,
		Role:        int8(user.Role), // nolint:gosec // G115
	})
}

func (w *UserWriter) UpdateWorkspaceUser(ctx context.Context, wsuser *mworkspace.WorkspaceUser) error {
	return w.queries.UpdateWorkspaceUser(ctx, gen.UpdateWorkspaceUserParams{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
	})
}

func (w *UserWriter) DeleteWorkspaceUser(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteWorkspaceUser(ctx, id)
}
