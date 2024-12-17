package sedge

import (
	"context"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type EdgeService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) EdgeService {
	return EdgeService{queries: queries}
}

func ConvertToDBEdge(e edge.Edge) gen.FlowEdge {
	return gen.FlowEdge{
		ID:           e.ID,
		FlowID:       e.FlowID,
		SourceID:     e.SourceID,
		TargetID:     e.TargetID,
		SourceHandle: int32(e.SourceHandler),
	}
}

func ConvertToModelEdge(e gen.FlowEdge) *edge.Edge {
	return &edge.Edge{
		ID:            e.ID,
		FlowID:        e.FlowID,
		SourceID:      e.SourceID,
		TargetID:      e.TargetID,
		SourceHandler: edge.EdgeHandle(e.SourceHandle),
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
	return es.queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
		ID:           edge.ID,
		FlowID:       edge.FlowID,
		SourceID:     edge.SourceID,
		TargetID:     edge.TargetID,
		SourceHandle: edge.SourceHandle,
	})
}

func (es EdgeService) UpdateEdge(ctx context.Context, e edge.Edge) error {
	edge := ConvertToDBEdge(e)
	return es.queries.UpdateFlowEdge(ctx, gen.UpdateFlowEdgeParams{
		ID:           edge.ID,
		SourceID:     edge.SourceID,
		TargetID:     edge.TargetID,
		SourceHandle: edge.SourceHandle,
	})
}

func (es EdgeService) DeleteEdge(ctx context.Context, id idwrap.IDWrap) error {
	err := es.queries.DeleteFlowEdge(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
