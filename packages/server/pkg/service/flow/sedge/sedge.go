//nolint:revive // exported
package sedge

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type EdgeService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) EdgeService {
	return EdgeService{queries: queries}
}

func (es EdgeService) TX(tx *sql.Tx) EdgeService {
	return EdgeService{queries: es.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx gen.DBTX) (*EdgeService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &EdgeService{
		queries: queries,
	}, nil
}

func ConvertToDBEdge(e edge.Edge) gen.FlowEdge {
	return gen.FlowEdge{
		ID:           e.ID,
		FlowID:       e.FlowID,
		SourceID:     e.SourceID,
		TargetID:     e.TargetID,
		SourceHandle: int32(e.SourceHandler),
		EdgeKind:     e.Kind,
	}
}

func ConvertToModelEdge(e gen.FlowEdge) *edge.Edge {
	return &edge.Edge{
		ID:            e.ID,
		FlowID:        e.FlowID,
		SourceID:      e.SourceID,
		TargetID:      e.TargetID,
		SourceHandler: edge.EdgeHandle(e.SourceHandle),
		Kind:          e.EdgeKind,
	}
}

func (es EdgeService) GetEdge(ctx context.Context, id idwrap.IDWrap) (*edge.Edge, error) {
	edge, err := es.queries.GetFlowEdge(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelEdge(edge), nil
}

func (es EdgeService) GetEdgesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]edge.Edge, error) {
	edge, err := es.queries.GetFlowEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvertPtr(edge, ConvertToModelEdge), nil
}

func (es EdgeService) CreateEdge(ctx context.Context, e edge.Edge) error {
	edge := ConvertToDBEdge(e)
	return es.queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams(edge))
}

func (es EdgeService) CreateEdgeBulk(ctx context.Context, edges []edge.Edge) error {
	for _, e := range edges {
		err := es.CreateEdge(ctx, e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (es EdgeService) UpdateEdge(ctx context.Context, e edge.Edge) error {
	edge := ConvertToDBEdge(e)
	return es.queries.UpdateFlowEdge(ctx, gen.UpdateFlowEdgeParams{
		ID:           edge.ID,
		SourceID:     edge.SourceID,
		TargetID:     edge.TargetID,
		SourceHandle: edge.SourceHandle,
		EdgeKind:     edge.EdgeKind,
	})
}

func (es EdgeService) DeleteEdge(ctx context.Context, id idwrap.IDWrap) error {
	err := es.queries.DeleteFlowEdge(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
