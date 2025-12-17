package sflowtag

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflowtag"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{queries: gen.New(tx)}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{queries: queries}
}

func (w *Writer) CreateFlowTag(ctx context.Context, ftag mflowtag.FlowTag) error {
	arg := ConvertModelToDB(ftag)
	err := w.queries.CreateFlowTag(ctx, gen.CreateFlowTagParams(arg))
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
}

func (w *Writer) DeleteFlowTag(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowTag(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
}
