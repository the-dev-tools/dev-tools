package snodeif

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnif"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type NodeIfService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) *NodeIfService {
	return &NodeIfService{
		queries: queries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeIfService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeIfService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeIf(ni mnif.MNIF) gen.FlowNodeIf {
	return gen.FlowNodeIf{
		FlowNodeID:    ni.FlowNodeID,
		Name:          ni.Name,
		ConditionType: int8(ni.ConditionType),
		Path:          ni.Path,
		Value:         ni.Value,
	}
}

func ConvertToModelNodeIf(ni gen.FlowNodeIf) *mnif.MNIF {
	return &mnif.MNIF{
		FlowNodeID:    ni.FlowNodeID,
		Name:          ni.Name,
		ConditionType: mnif.ConditionType(ni.ConditionType),
		Path:          ni.Path,
		Value:         ni.Value,
	}
}

func (nifs NodeIfService) GetNodeIf(ctx context.Context, id idwrap.IDWrap) (*mnif.MNIF, error) {
	nodeIf, err := nifs.queries.GetFlowNodeIf(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeIf(nodeIf), nil
}

func (nifs NodeIfService) CreateNodeIf(ctx context.Context, ni mnif.MNIF) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return nifs.queries.CreateFlowNodeIf(ctx, gen.CreateFlowNodeIfParams{
		FlowNodeID:    nodeIf.FlowNodeID,
		Name:          nodeIf.Name,
		ConditionType: nodeIf.ConditionType,
		Path:          nodeIf.Path,
		Value:         nodeIf.Value,
	})
}

func (nifs NodeIfService) UpdateNodeIf(ctx context.Context, ni mnif.MNIF) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return nifs.queries.UpdateFlowNodeIf(ctx, gen.UpdateFlowNodeIfParams{
		FlowNodeID:    nodeIf.FlowNodeID,
		Name:          nodeIf.Name,
		ConditionType: nodeIf.ConditionType,
		Path:          nodeIf.Path,
		Value:         nodeIf.Value,
	})
}

func (nifs NodeIfService) DeleteNodeIf(ctx context.Context, id idwrap.IDWrap) error {
	return nifs.queries.DeleteFlowNodeIf(ctx, id)
}
