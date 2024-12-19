package snodestart

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnstart"
	"the-dev-tools/db/pkg/sqlc/gen"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeStartService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeStartService {
	return NodeStartService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeStartService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeStartService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeStart(ns mnstart.StartNode) gen.FlowNodeStart {
	return gen.FlowNodeStart{
		FlowNodeID: ns.FlowNodeID,
		Name:       ns.Name,
	}
}

func ConvertToModelNodeStart(ns gen.FlowNodeStart) *mnstart.StartNode {
	return &mnstart.StartNode{
		FlowNodeID: ns.FlowNodeID,
		Name:       ns.Name,
	}
}

func (nfs NodeStartService) GetNodeStart(ctx context.Context, id idwrap.IDWrap) (*mnstart.StartNode, error) {
	nodeFor, err := nfs.queries.GetFlowNodeStart(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeStart(nodeFor), nil
}

func (nfs NodeStartService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnstart.StartNode, error) {
	return nil, nil
}

func (nfs NodeStartService) CreateNodeStart(ctx context.Context, nf mnstart.StartNode) error {
	convertedNode := ConvertToDBNodeStart(nf)
	return nfs.queries.CreateFlowNodeStart(ctx, gen.CreateFlowNodeStartParams{
		FlowNodeID: convertedNode.FlowNodeID,
		Name:       convertedNode.Name,
	})
}

func (nfs NodeStartService) UpdateNodeStart(ctx context.Context, nf mnstart.StartNode) error {
	convertedNode := ConvertToDBNodeStart(nf)
	return nfs.queries.UpdateFlowNodeStart(ctx, gen.UpdateFlowNodeStartParams{
		FlowNodeID: convertedNode.FlowNodeID,
		Name:       convertedNode.Name,
	})
}

func (nfs NodeStartService) DeleteNodeStart(ctx context.Context, id idwrap.IDWrap) error {
	err := nfs.queries.DeleteFlowNodeFor(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoNodeForFound
	}
	return err
}
