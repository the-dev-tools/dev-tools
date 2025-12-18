//nolint:revive // exported
package sedge

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
)

type EdgeService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) EdgeService {
	return EdgeService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (es EdgeService) TX(tx *sql.Tx) EdgeService {
	newQueries := es.queries.WithTx(tx)
	return EdgeService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx gen.DBTX) (*EdgeService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &EdgeService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (es EdgeService) GetEdge(ctx context.Context, id idwrap.IDWrap) (*edge.Edge, error) {
	return es.reader.GetEdge(ctx, id)
}

func (es EdgeService) GetEdgesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]edge.Edge, error) {
	return es.reader.GetEdgesByFlowID(ctx, flowID)
}

func (es EdgeService) CreateEdge(ctx context.Context, e edge.Edge) error {
	return NewWriterFromQueries(es.queries).CreateEdge(ctx, e)
}

func (es EdgeService) CreateEdgeBulk(ctx context.Context, edges []edge.Edge) error {
	return NewWriterFromQueries(es.queries).CreateEdgeBulk(ctx, edges)
}

func (es EdgeService) UpdateEdge(ctx context.Context, e edge.Edge) error {
	return NewWriterFromQueries(es.queries).UpdateEdge(ctx, e)
}

func (es EdgeService) DeleteEdge(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(es.queries).DeleteEdge(ctx, id)
}

func (s EdgeService) Reader() *Reader { return s.reader }
