package snode

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnnode"
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

func (s NodeService) TX(tx *sql.Tx) NodeService {
	return NodeService{queries: s.queries.WithTx(tx)}
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

func ConvertNodeToDB(n mnnode.MNode) *gen.FlowNode {
	return &gen.FlowNode{
		ID:                    n.ID,
		FlowID:                n.FlowID,
		Name:                  n.Name,
		NodeKind:              int32(n.NodeKind),
		State:                 n.State,
		StateData:             n.StateData,
		StateDataCompressType: n.StateDataCompressType,
		PositionX:             n.PositionX,
		PositionY:             n.PositionY,
	}
}

func ConvertNodeToModel(n gen.FlowNode) *mnnode.MNode {
	return &mnnode.MNode{
		ID:                    n.ID,
		FlowID:                n.FlowID,
		Name:                  n.Name,
		NodeKind:              mnnode.NodeKind(n.NodeKind),
		State:                 n.State,
		StateData:             n.StateData,
		StateDataCompressType: n.StateDataCompressType,
		PositionX:             n.PositionX,
		PositionY:             n.PositionY,
	}
}

func (ns NodeService) GetNode(ctx context.Context, id idwrap.IDWrap) (*mnnode.MNode, error) {
	node, err := ns.queries.GetFlowNode(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertNodeToModel(node), nil
}

func (ns NodeService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnnode.MNode, error) {
	nodes, err := ns.queries.GetFlowNodesByFlowID(ctx, flowID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mnnode.MNode{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(nodes, ConvertNodeToModel), nil
}

func (ns NodeService) CreateNode(ctx context.Context, n mnnode.MNode) error {
	node := ConvertNodeToDB(n)
	return ns.queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:                    node.ID,
		FlowID:                node.FlowID,
		Name:                  node.Name,
		NodeKind:              node.NodeKind,
		State:                 n.State,
		StateData:             n.StateData,
		StateDataCompressType: n.StateDataCompressType,
		PositionX:             node.PositionX,
		PositionY:             node.PositionY,
	})
}

func (ns NodeService) CreateNodeBulk(ctx context.Context, nodes []mnnode.MNode) error {
	for _, n := range nodes {
		node := ConvertNodeToDB(n)
		err := ns.queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
			ID:                    node.ID,
			FlowID:                node.FlowID,
			Name:                  node.Name,
			NodeKind:              node.NodeKind,
			State:                 n.State,
			StateData:             n.StateData,
			StateDataCompressType: n.StateDataCompressType,
			PositionX:             node.PositionX,
			PositionY:             node.PositionY,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ns NodeService) UpdateNode(ctx context.Context, n mnnode.MNode) error {
	node := ConvertNodeToDB(n)
	return ns.queries.UpdateFlowNode(ctx, gen.UpdateFlowNodeParams{
		ID:                    node.ID,
		Name:                  node.Name,
		State:                 n.State,
		StateData:             n.StateData,
		StateDataCompressType: n.StateDataCompressType,
		PositionX:             node.PositionX,
		PositionY:             node.PositionY,
	})
}

func (ns NodeService) UpdateNodeState(ctx context.Context, n mnnode.MNode) error {
	node := ConvertNodeToDB(n)
	return ns.queries.UpdateFlowState(ctx, gen.UpdateFlowStateParams{
		ID:                    node.ID,
		State:                 n.State,
		StateData:             n.StateData,
		StateDataCompressType: n.StateDataCompressType,
	})
}

func (ns NodeService) DeleteNode(ctx context.Context, id idwrap.IDWrap) error {
	return ns.queries.DeleteFlowNode(ctx, id)
}
