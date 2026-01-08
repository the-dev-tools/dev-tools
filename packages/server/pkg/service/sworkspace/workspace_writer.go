package sworkspace

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
)

type WorkspaceWriter struct {
	queries *gen.Queries
}

func NewWorkspaceWriter(tx gen.DBTX) *WorkspaceWriter {
	return &WorkspaceWriter{
		queries: gen.New(tx),
	}
}

func NewWorkspaceWriterFromQueries(queries *gen.Queries) *WorkspaceWriter {
	return &WorkspaceWriter{
		queries: queries,
	}
}

func (w *WorkspaceWriter) Create(ctx context.Context, ws *mworkspace.Workspace) error {
	dbWorkspace := ConvertToDBWorkspace(*ws)
	return w.queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams(dbWorkspace))
}

func (w *WorkspaceWriter) Update(ctx context.Context, org *mworkspace.Workspace) error {
	err := w.queries.UpdateWorkspace(ctx, gen.UpdateWorkspaceParams{
		ID:              org.ID,
		Name:            org.Name,
		FlowCount:       org.FlowCount,
		CollectionCount: org.CollectionCount,
		Updated:         org.Updated.Unix(),
		ActiveEnv:       org.ActiveEnv,
		DisplayOrder:    org.Order,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoWorkspaceFound
	}
	return err
}

func (w *WorkspaceWriter) UpdateUpdatedTime(ctx context.Context, org *mworkspace.Workspace) error {
	err := w.queries.UpdateWorkspaceUpdatedTime(ctx, gen.UpdateWorkspaceUpdatedTimeParams{
		ID:      org.ID,
		Updated: org.Updated.Unix(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoWorkspaceFound
	}
	return err
}

func (w *WorkspaceWriter) Delete(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteWorkspace(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoWorkspaceFound
	}
	return err
}
