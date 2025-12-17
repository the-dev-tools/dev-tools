package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type Reader struct {
	queries *gen.Queries
}

func NewReader(db *sql.DB) *Reader {
	return &Reader{queries: gen.New(db)}
}

func NewReaderFromQueries(queries *gen.Queries) *Reader {
	return &Reader{queries: queries}
}

func (r *Reader) GetFlow(ctx context.Context, id idwrap.IDWrap) (mflow.Flow, error) {
	item, err := r.queries.GetFlow(ctx, id)
	if err != nil {
		return mflow.Flow{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return ConvertDBToModel(item), nil
}

func (r *Reader) GetFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}

// GetAllFlowsByWorkspaceID returns all flows including versions for TanStack DB sync
func (r *Reader) GetAllFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetAllFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}

func (r *Reader) GetFlowsByVersionParentID(ctx context.Context, versionParentID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetFlowsByVersionParentID(ctx, &versionParentID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}
