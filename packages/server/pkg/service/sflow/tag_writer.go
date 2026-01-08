package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type FlowTagWriter struct {
	queries *gen.Queries
}

func NewFlowTagWriter(tx gen.DBTX) *FlowTagWriter {
	return &FlowTagWriter{queries: gen.New(tx)}
}

func NewFlowTagWriterFromQueries(queries *gen.Queries) *FlowTagWriter {
	return &FlowTagWriter{queries: queries}
}

func (w *FlowTagWriter) CreateFlowTag(ctx context.Context, ftag mflow.FlowTag) error {
	arg := ConvertFlowTagToDB(ftag)
	err := w.queries.CreateFlowTag(ctx, gen.CreateFlowTagParams(arg))
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
}

func (w *FlowTagWriter) DeleteFlowTag(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowTag(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
}
