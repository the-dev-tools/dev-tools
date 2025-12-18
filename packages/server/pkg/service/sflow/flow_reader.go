package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type FlowReader struct {
	queries *gen.Queries
}

func NewFlowReader(db *sql.DB) *FlowReader {
	return &FlowReader{queries: gen.New(db)}
}

func NewFlowReaderFromQueries(queries *gen.Queries) *FlowReader {
	return &FlowReader{queries: queries}
}

func (r *FlowReader) GetFlow(ctx context.Context, id idwrap.IDWrap) (mflow.Flow, error) {
	item, err := r.queries.GetFlow(ctx, id)
	if err != nil {
		return mflow.Flow{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return ConvertDBToFlow(item), nil
}

func (r *FlowReader) GetFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToFlow), nil
}

// GetAllFlowsByWorkspaceID returns all flows including versions for TanStack DB sync
func (r *FlowReader) GetAllFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetAllFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToFlow), nil
}

func (r *FlowReader) GetFlowsByVersionParentID(ctx context.Context, versionParentID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetFlowsByVersionParentID(ctx, &versionParentID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToFlow), nil
}
