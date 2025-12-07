//nolint:revive // exported
package snodeif

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
)

type NodeIfService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) *NodeIfService {
	return &NodeIfService{
		queries: queries,
	}
}

func (nifs NodeIfService) TX(tx *sql.Tx) *NodeIfService {
	return &NodeIfService{queries: nifs.queries.WithTx(tx)}
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

func ConvertToDBNodeIf(ni mnif.MNIF) gen.FlowNodeCondition {
	return gen.FlowNodeCondition{
		FlowNodeID: ni.FlowNodeID,
		Expression: ni.Condition.Comparisons.Expression,
	}
}

func ConvertToModelNodeIf(ni gen.FlowNodeCondition) *mnif.MNIF {
	return &mnif.MNIF{
		FlowNodeID: ni.FlowNodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: ni.Expression,
			},
		},
	}
}

func (nifs NodeIfService) GetNodeIf(ctx context.Context, id idwrap.IDWrap) (*mnif.MNIF, error) {
	nodeIf, err := nifs.queries.GetFlowNodeCondition(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeIf(nodeIf), nil
}

func (nifs NodeIfService) CreateNodeIf(ctx context.Context, ni mnif.MNIF) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return nifs.queries.CreateFlowNodeCondition(ctx, gen.CreateFlowNodeConditionParams{
		FlowNodeID: nodeIf.FlowNodeID,
		Expression: ni.Condition.Comparisons.Expression,
	})
}

func (nifs NodeIfService) CreateNodeIfBulk(ctx context.Context, conditionNodes []mnif.MNIF) error {
	var err error
	for _, n := range conditionNodes {
		err = nifs.CreateNodeIf(ctx, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (nifs NodeIfService) UpdateNodeIf(ctx context.Context, ni mnif.MNIF) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return nifs.queries.UpdateFlowNodeCondition(ctx, gen.UpdateFlowNodeConditionParams{
		FlowNodeID: nodeIf.FlowNodeID,
		Expression: nodeIf.Expression,
	})
}

func (nifs NodeIfService) DeleteNodeIf(ctx context.Context, id idwrap.IDWrap) error {
	return nifs.queries.DeleteFlowNodeCondition(ctx, id)
}
