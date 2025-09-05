package snodefor

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeForService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeForService {
	return NodeForService{queries: queries}
}

func (nfs NodeForService) TX(tx *sql.Tx) NodeForService {
	return NodeForService{queries: nfs.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeForService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeForService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeFor(nf mnfor.MNFor) gen.FlowNodeFor {
	return gen.FlowNodeFor{
		FlowNodeID:    nf.FlowNodeID,
		IterCount:     nf.IterCount,
		ErrorHandling: int8(nf.ErrorHandling),
		Expression:    nf.Condition.Comparisons.Expression,
	}
}

func ConvertToModelNodeFor(nf gen.FlowNodeFor) *mnfor.MNFor {
	return &mnfor.MNFor{
		FlowNodeID:    nf.FlowNodeID,
		IterCount:     nf.IterCount,
		ErrorHandling: mnfor.ErrorHandling(nf.ErrorHandling),
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: nf.Expression,
			},
		},
	}
}

func (nfs NodeForService) GetNodeFor(ctx context.Context, id idwrap.IDWrap) (*mnfor.MNFor, error) {
	nodeFor, err := nfs.queries.GetFlowNodeFor(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeFor(nodeFor), nil
}

func (nfs NodeForService) CreateNodeFor(ctx context.Context, nf mnfor.MNFor) error {
	// Default ErrorHandling to IGNORE if unspecified
	if nf.ErrorHandling == mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED {
		nf.ErrorHandling = mnfor.ErrorHandling_ERROR_HANDLING_IGNORE
	}
	nodeFor := ConvertToDBNodeFor(nf)
	return nfs.queries.CreateFlowNodeFor(ctx, gen.CreateFlowNodeForParams{
		FlowNodeID:    nodeFor.FlowNodeID,
		IterCount:     nodeFor.IterCount,
		ErrorHandling: nodeFor.ErrorHandling,
		Expression:    nodeFor.Expression,
	})
}

func (nfs NodeForService) CreateNodeForBulk(ctx context.Context, nf []mnfor.MNFor) error {
	var err error
	for _, n := range nf {
		err = nfs.CreateNodeFor(ctx, n)
		if err != nil {
			break
		}
	}
	return err
}

func (nfs NodeForService) UpdateNodeFor(ctx context.Context, nf mnfor.MNFor) error {
	nodeFor := ConvertToDBNodeFor(nf)
	return nfs.queries.UpdateFlowNodeFor(ctx, gen.UpdateFlowNodeForParams{
		FlowNodeID:    nodeFor.FlowNodeID,
		IterCount:     nodeFor.IterCount,
		ErrorHandling: nodeFor.ErrorHandling,
		Expression:    nodeFor.Expression,
	})
}

func (nfs NodeForService) DeleteNodeFor(ctx context.Context, id idwrap.IDWrap) error {
	err := nfs.queries.DeleteFlowNodeFor(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoNodeForFound
	}
	return err
}
