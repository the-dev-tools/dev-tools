package sworkspacesusers

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{
		queries: gen.New(tx),
	}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{
		queries: queries,
	}
}

func (w *Writer) CreateWorkspaceUser(ctx context.Context, user *mworkspace.WorkspaceUser) error {
	return w.queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          user.ID,
		WorkspaceID: user.WorkspaceID,
		UserID:      user.UserID,
		Role:        int8(user.Role), // nolint:gosec // G115
	})
}

func (w *Writer) UpdateWorkspaceUser(ctx context.Context, wsuser *mworkspace.WorkspaceUser) error {
	return w.queries.UpdateWorkspaceUser(ctx, gen.UpdateWorkspaceUserParams{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
	})
}

func (w *Writer) DeleteWorkspaceUser(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteWorkspaceUser(ctx, id)
}
