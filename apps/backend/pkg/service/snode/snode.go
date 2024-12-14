package snode

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

var ErrNoNodeFound error = sql.ErrNoRows

type NodeService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeService {
	return NodeService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeService{
		queries: queries,
	}, nil
}

func ConvertNodeToDB(n mnode.MNode) *gen.FlowNode {
	return &gen.FlowNode{
		ID:        n.ID,
		FlowID:    n.FlowID,
		NodeKind:  int32(n.NodeKind),
		PositionX: n.PositionX,
		PositionY: n.PositionY,
	}
}

func ConvertNodeToModel(n gen.FlowNode) *mnode.MNode {
	return &mnode.MNode{
		ID:        n.ID,
		FlowID:    n.FlowID,
		NodeKind:  mnode.NodeKind(n.NodeKind),
		PositionX: n.PositionX,
		PositionY: n.PositionY,
	}
}

func (ns NodeService) GetNode(ctx context.Context, id idwrap.IDWrap) (*mnode.MNode, error) {
	node, err := ns.queries.GetFlowNode(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertNodeToModel(node), nil
}

func (ns NodeService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnode.MNode, error) {
	nodes, err := ns.queries.GetFlowNodesByFlowID(ctx, flowID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mnode.MNode{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(nodes, ConvertNodeToModel), nil
}

func (ns NodeService) CreateNode(ctx context.Context, n mnode.MNode) (*mnode.MNode, error) {
	node := ConvertNodeToDB(n)
	err := ns.queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        node.ID,
		FlowID:    node.FlowID,
		NodeKind:  node.NodeKind,
		PositionX: node.PositionX,
		PositionY: node.PositionY,
	})
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (ns NodeService) UpdateNode(ctx context.Context, n mnode.MNode) (*mnode.MNode, error) {
	node := ConvertNodeToDB(n)
	err := ns.queries.UpdateFlowNode(ctx, gen.UpdateFlowNodeParams{
		ID:        node.ID,
		PositionX: node.PositionX,
		PositionY: node.PositionY,
	})
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (ns NodeService) DeleteNode(ctx context.Context, id idwrap.IDWrap) error {
	return ns.queries.DeleteFlowNode(ctx, id)
}
