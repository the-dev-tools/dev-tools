package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type FlowTagReader struct {
	queries *gen.Queries
}

func NewFlowTagReader(db *sql.DB) *FlowTagReader {
	return &FlowTagReader{queries: gen.New(db)}
}

func NewFlowTagReaderFromQueries(queries *gen.Queries) *FlowTagReader {
	return &FlowTagReader{queries: queries}
}

func (r *FlowTagReader) GetFlowTag(ctx context.Context, id idwrap.IDWrap) (mflow.FlowTag, error) {
	item, err := r.queries.GetFlowTag(ctx, id)
	if err != nil {
		return mflow.FlowTag{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
	}
	return ConvertDBToFlowTag(item), nil
}

func (r *FlowTagReader) GetFlowTagsByTagID(ctx context.Context, tagID idwrap.IDWrap) ([]mflow.FlowTag, error) {
	items, err := r.queries.GetFlowTagsByTagID(ctx, tagID)
	if err != nil {
		return []mflow.FlowTag{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
	}
	return tgeneric.MassConvert(items, ConvertDBToFlowTag), nil
}
