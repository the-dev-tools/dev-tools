package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type EdgeReader struct {
	queries *gen.Queries
}

func NewEdgeReader(db *sql.DB) *EdgeReader {
	return &EdgeReader{queries: gen.New(db)}
}

func NewEdgeReaderFromQueries(queries *gen.Queries) *EdgeReader {
	return &EdgeReader{queries: queries}
}

func (r *EdgeReader) GetEdge(ctx context.Context, id idwrap.IDWrap) (*mflow.Edge, error) {
	edge, err := r.queries.GetFlowEdge(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelEdge(edge), nil
}

func (r *EdgeReader) GetEdgesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.Edge, error) {
	edge, err := r.queries.GetFlowEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvertPtr(edge, ConvertToModelEdge), nil
}
