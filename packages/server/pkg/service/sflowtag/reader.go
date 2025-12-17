package sflowtag

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflowtag"
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

func (r *Reader) GetFlowTag(ctx context.Context, id idwrap.IDWrap) (mflowtag.FlowTag, error) {
	item, err := r.queries.GetFlowTag(ctx, id)
	if err != nil {
		return mflowtag.FlowTag{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
	}
	return ConvertDBToModel(item), nil
}

func (r *Reader) GetFlowTagsByTagID(ctx context.Context, tagID idwrap.IDWrap) ([]mflowtag.FlowTag, error) {
	items, err := r.queries.GetFlowTagsByTagID(ctx, tagID)
	if err != nil {
		return []mflowtag.FlowTag{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
	}
	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}
