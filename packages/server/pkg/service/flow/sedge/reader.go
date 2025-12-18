package sedge

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
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

func (r *Reader) GetEdge(ctx context.Context, id idwrap.IDWrap) (*edge.Edge, error) {
	edge, err := r.queries.GetFlowEdge(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelEdge(edge), nil
}

func (r *Reader) GetEdgesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]edge.Edge, error) {
	edge, err := r.queries.GetFlowEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvertPtr(edge, ConvertToModelEdge), nil
}
