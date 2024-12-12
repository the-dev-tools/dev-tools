package snodefor

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnfor"
	"the-dev-tools/db/pkg/sqlc/gen"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeForService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeForService {
	return NodeForService{queries: queries}
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
		FlowNodeID: nf.FlowNodeID,
		Name:       nf.Name,
		IterCount:  nf.IterCount,
	}
}

func ConvertToModelNodeFor(nf gen.FlowNodeFor) *mnfor.MNFor {
	return &mnfor.MNFor{
		FlowNodeID: nf.FlowNodeID,
		Name:       nf.Name,
		IterCount:  nf.IterCount,
	}
}

func (nfs NodeForService) GetNodeFor(ctx context.Context, id idwrap.IDWrap) (*mnfor.MNFor, error) {
	nodeFor, err := nfs.queries.GetFlowNodeFor(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeFor(nodeFor), nil
}

func (nfs NodeForService) CreateNodeFor(ctx context.Context, nf mnfor.MNFor) (*mnfor.MNFor, error) {
	nodeFor := ConvertToDBNodeFor(nf)
	err := nfs.queries.CreateFlowNodeFor(ctx, gen.CreateFlowNodeForParams{
		FlowNodeID:      nodeFor.FlowNodeID,
		Name:            nodeFor.Name,
		IterCount:       nodeFor.IterCount,
		LoopStartNodeID: nodeFor.LoopStartNodeID,
		Next:            nodeFor.Next,
	})
	if err != nil {
		return nil, err
	}
	return &nf, nil
}

func (nfs NodeForService) UpdateNodeFor(ctx context.Context, nf mnfor.MNFor) (*mnfor.MNFor, error) {
	nodeFor := ConvertToDBNodeFor(nf)
	err := nfs.queries.UpdateFlowNodeFor(ctx, gen.UpdateFlowNodeForParams{
		FlowNodeID:      nodeFor.FlowNodeID,
		Name:            nodeFor.Name,
		IterCount:       nodeFor.IterCount,
		LoopStartNodeID: nodeFor.LoopStartNodeID,
		Next:            nodeFor.Next,
	})
	if err != nil {
		return nil, err
	}
	return &nf, nil
}

func (nfs NodeForService) DeleteNodeFor(ctx context.Context, id idwrap.IDWrap) error {
	err := nfs.queries.DeleteFlowNodeFor(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoNodeForFound
	}
	return err
}
