package sworkspace

import (
	"context"
	"database/sql"
	"errors"
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

func (w *Writer) Create(ctx context.Context, ws *mworkspace.Workspace) error {
	dbWorkspace := ConvertToDBWorkspace(*ws)
	return w.queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams(dbWorkspace))
}

func (w *Writer) Update(ctx context.Context, org *mworkspace.Workspace) error {
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

func (w *Writer) UpdateUpdatedTime(ctx context.Context, org *mworkspace.Workspace) error {
	err := w.queries.UpdateWorkspaceUpdatedTime(ctx, gen.UpdateWorkspaceUpdatedTimeParams{
		ID:      org.ID,
		Updated: org.Updated.Unix(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoWorkspaceFound
	}
	return err
}

func (w *Writer) Delete(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteWorkspace(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoWorkspaceFound
	}
	return err
}
