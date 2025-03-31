package snodeforeach

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcondition"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnnode/mnforeach"
	"the-dev-tools/db/pkg/sqlc/gen"
)

var ErrNoNodeForEachFound = errors.New("node foreach not found")

type NodeForEachService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeForEachService {
	return NodeForEachService{queries: queries}
}

func (nfs NodeForEachService) TX(tx *sql.Tx) NodeForEachService {
	return NodeForEachService{queries: nfs.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeForEachService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeForEachService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeFor(nf mnforeach.MNForEach) gen.FlowNodeForEach {
	return gen.FlowNodeForEach{
		FlowNodeID:    nf.FlowNodeID,
		IterPath:      nf.IterPath,
		ErrorHandling: int8(nf.ErrorHandling),
		ConditionType: int8(nf.Condition.Comparisons.Kind),
		ConditionPath: nf.Condition.Comparisons.Path,
		Value:         nf.Condition.Comparisons.Value,
	}
}

func ConvertToModelNodeFor(nf gen.FlowNodeForEach) *mnforeach.MNForEach {
	return &mnforeach.MNForEach{
		FlowNodeID:    nf.FlowNodeID,
		IterPath:      nf.IterPath,
		ErrorHandling: mnfor.ErrorHandling(nf.ErrorHandling),
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Kind:  mcondition.ComparisonKind(nf.ConditionType),
				Path:  nf.ConditionPath,
				Value: nf.Value,
			},
		},
	}
}

func (nfs NodeForEachService) GetNodeForEach(ctx context.Context, id idwrap.IDWrap) (*mnforeach.MNForEach, error) {
	nodeForEach, err := nfs.queries.GetFlowNodeForEach(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeFor(nodeForEach), nil
}

func (nfs NodeForEachService) CreateNodeForEach(ctx context.Context, nf mnforeach.MNForEach) error {
	nodeForEach := ConvertToDBNodeFor(nf)
	return nfs.queries.CreateFlowNodeForEach(ctx, gen.CreateFlowNodeForEachParams{
		FlowNodeID:    nodeForEach.FlowNodeID,
		IterPath:      nodeForEach.IterPath,
		ErrorHandling: nodeForEach.ErrorHandling,
		ConditionType: nodeForEach.ConditionType,
		ConditionPath: nodeForEach.ConditionPath,
		Value:         nodeForEach.Value,
	})
}

func (nfs NodeForEachService) CreateNodeForEachBulk(ctx context.Context, forEachNodes []mnforeach.MNForEach) error {
	var err error
	for _, forEachNode := range forEachNodes {
		err = nfs.CreateNodeForEach(ctx, forEachNode)
		if err != nil {
			break
		}
	}
	return err
}

func (nfs NodeForEachService) UpdateNodeForEach(ctx context.Context, nf mnforeach.MNForEach) error {
	nodeForEach := ConvertToDBNodeFor(nf)
	return nfs.queries.UpdateFlowNodeForEach(ctx, gen.UpdateFlowNodeForEachParams{
		FlowNodeID:    nodeForEach.FlowNodeID,
		IterPath:      nodeForEach.IterPath,
		ErrorHandling: nodeForEach.ErrorHandling,
		ConditionType: nodeForEach.ConditionType,
		ConditionPath: nodeForEach.ConditionPath,
		Value:         nodeForEach.Value,
	})
}

func (nfs NodeForEachService) DeleteNodeForEach(ctx context.Context, id idwrap.IDWrap) error {
	err := nfs.queries.DeleteFlowNodeForEach(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoNodeForEachFound
	}
	return err
}
