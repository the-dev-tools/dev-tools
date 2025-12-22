//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type EdgeService struct {
	reader  *EdgeReader
	queries *gen.Queries
}

func NewEdgeService(queries *gen.Queries) EdgeService {
	return EdgeService{
		reader:  NewEdgeReaderFromQueries(queries),
		queries: queries,
	}
}

func (es EdgeService) TX(tx *sql.Tx) EdgeService {
	newQueries := es.queries.WithTx(tx)
	return EdgeService{
		reader:  NewEdgeReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewEdgeServiceTX(ctx context.Context, tx gen.DBTX) (*EdgeService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &EdgeService{
		reader:  NewEdgeReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (es EdgeService) GetEdge(ctx context.Context, id idwrap.IDWrap) (*mflow.Edge, error) {
	return es.reader.GetEdge(ctx, id)
}

func (es EdgeService) GetEdgesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.Edge, error) {
	return es.reader.GetEdgesByFlowID(ctx, flowID)
}

func (es EdgeService) CreateEdge(ctx context.Context, e mflow.Edge) error {
	return NewEdgeWriterFromQueries(es.queries).CreateEdge(ctx, e)
}

func (es EdgeService) CreateEdgeBulk(ctx context.Context, edges []mflow.Edge) error {
	return NewEdgeWriterFromQueries(es.queries).CreateEdgeBulk(ctx, edges)
}

func (es EdgeService) UpdateEdge(ctx context.Context, e mflow.Edge) error {
	return NewEdgeWriterFromQueries(es.queries).UpdateEdge(ctx, e)
}

func (es EdgeService) DeleteEdge(ctx context.Context, id idwrap.IDWrap) error {
	return NewEdgeWriterFromQueries(es.queries).DeleteEdge(ctx, id)
}

func (es EdgeService) UpdateEdgeState(ctx context.Context, id idwrap.IDWrap, state mflow.NodeState) error {
	return NewEdgeWriterFromQueries(es.queries).UpdateEdgeState(ctx, id, state)
}

func (s EdgeService) Reader() *EdgeReader { return s.reader }
