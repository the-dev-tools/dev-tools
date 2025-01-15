package snodenoop

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnnoop"
	"the-dev-tools/db/pkg/sqlc/gen"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeNoopService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeNoopService {
	return NodeNoopService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeNoopService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeNoopService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeStart(ns mnnoop.NoopNode) gen.FlowNodeNoop {
	return gen.FlowNodeNoop{
		FlowNodeID: ns.FlowNodeID,
		NodeType:   int16(ns.Type),
		Name:       ns.Name,
	}
}

func ConvertToModelNodeStart(ns gen.FlowNodeNoop) *mnnoop.NoopNode {
	return &mnnoop.NoopNode{
		FlowNodeID: ns.FlowNodeID,
		Type:       mnnoop.NoopTypes(ns.NodeType),
		Name:       ns.Name,
	}
}

func (nfs NodeNoopService) GetNodeNoop(ctx context.Context, id idwrap.IDWrap) (*mnnoop.NoopNode, error) {
	nodeFor, err := nfs.queries.GetFlowNodeNoop(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeStart(nodeFor), nil
}

func (nfs NodeNoopService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnnoop.NoopNode, error) {
	return nil, nil
}

func (nfs NodeNoopService) CreateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	convertedNode := ConvertToDBNodeStart(nf)
	return nfs.queries.CreateFlowNodeNoop(ctx, gen.CreateFlowNodeNoopParams{
		FlowNodeID: convertedNode.FlowNodeID,
		NodeType:   convertedNode.NodeType,
		Name:       convertedNode.Name,
	})
}

func (nfs NodeNoopService) UpdateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	convertedNode := ConvertToDBNodeStart(nf)
	return nfs.queries.UpdateFlowNodeNoop(ctx, gen.UpdateFlowNodeNoopParams{
		FlowNodeID: convertedNode.FlowNodeID,
		Name:       convertedNode.Name,
	})
}

func (nfs NodeNoopService) DeleteNodeNoop(ctx context.Context, id idwrap.IDWrap) error {
	err := nfs.queries.DeleteFlowNodeFor(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoNodeForFound
	}
	return err
}
