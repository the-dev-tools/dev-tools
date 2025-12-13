//nolint:revive // exported
package snode

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/translate/tgeneric"
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
		ID:        n.ID,
		FlowID:    n.FlowID,
		Name:      n.Name,
		NodeKind:  n.NodeKind,
		PositionX: n.PositionX,
		PositionY: n.PositionY,
	}
}

func ConvertNodeToModel(n gen.FlowNode) *mnnode.MNode {
	return &mnnode.MNode{
		ID:        n.ID,
		FlowID:    n.FlowID,
		Name:      n.Name,
		NodeKind:  n.NodeKind,
		PositionX: n.PositionX,
		PositionY: n.PositionY,
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
		if errors.Is(err, sql.ErrNoRows) {
			return []mnnode.MNode{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(nodes, ConvertNodeToModel), nil
}

func (ns NodeService) CreateNode(ctx context.Context, n mnnode.MNode) error {
	node := ConvertNodeToDB(n)
	return ns.queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        node.ID,
		FlowID:    node.FlowID,
		Name:      node.Name,
		NodeKind:  node.NodeKind,
		PositionX: node.PositionX,
		PositionY: node.PositionY,
	})
}

func (ns NodeService) CreateNodeBulk(ctx context.Context, nodes []mnnode.MNode) error {
	batchSize := 10
	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}

		batch := nodes[i:end]
		if len(batch) == batchSize {
			arg := gen.CreateFlowNodesBulkParams{
				ID:           batch[0].ID,
				FlowID:       batch[0].FlowID,
				Name:         batch[0].Name,
				NodeKind:     batch[0].NodeKind,
				PositionX:    batch[0].PositionX,
				PositionY:    batch[0].PositionY,
				ID_2:         batch[1].ID,
				FlowID_2:     batch[1].FlowID,
				Name_2:       batch[1].Name,
				NodeKind_2:   batch[1].NodeKind,
				PositionX_2:  batch[1].PositionX,
				PositionY_2:  batch[1].PositionY,
				ID_3:         batch[2].ID,
				FlowID_3:     batch[2].FlowID,
				Name_3:       batch[2].Name,
				NodeKind_3:   batch[2].NodeKind,
				PositionX_3:  batch[2].PositionX,
				PositionY_3:  batch[2].PositionY,
				ID_4:         batch[3].ID,
				FlowID_4:     batch[3].FlowID,
				Name_4:       batch[3].Name,
				NodeKind_4:   batch[3].NodeKind,
				PositionX_4:  batch[3].PositionX,
				PositionY_4:  batch[3].PositionY,
				ID_5:         batch[4].ID,
				FlowID_5:     batch[4].FlowID,
				Name_5:       batch[4].Name,
				NodeKind_5:   batch[4].NodeKind,
				PositionX_5:  batch[4].PositionX,
				PositionY_5:  batch[4].PositionY,
				ID_6:         batch[5].ID,
				FlowID_6:     batch[5].FlowID,
				Name_6:       batch[5].Name,
				NodeKind_6:   batch[5].NodeKind,
				PositionX_6:  batch[5].PositionX,
				PositionY_6:  batch[5].PositionY,
				ID_7:         batch[6].ID,
				FlowID_7:     batch[6].FlowID,
				Name_7:       batch[6].Name,
				NodeKind_7:   batch[6].NodeKind,
				PositionX_7:  batch[6].PositionX,
				PositionY_7:  batch[6].PositionY,
				ID_8:         batch[7].ID,
				FlowID_8:     batch[7].FlowID,
				Name_8:       batch[7].Name,
				NodeKind_8:   batch[7].NodeKind,
				PositionX_8:  batch[7].PositionX,
				PositionY_8:  batch[7].PositionY,
				ID_9:         batch[8].ID,
				FlowID_9:     batch[8].FlowID,
				Name_9:       batch[8].Name,
				NodeKind_9:   batch[8].NodeKind,
				PositionX_9:  batch[8].PositionX,
				PositionY_9:  batch[8].PositionY,
				ID_10:        batch[9].ID,
				FlowID_10:    batch[9].FlowID,
				Name_10:      batch[9].Name,
				NodeKind_10:  batch[9].NodeKind,
				PositionX_10: batch[9].PositionX,
				PositionY_10: batch[9].PositionY,
			}
			err := ns.queries.CreateFlowNodesBulk(ctx, arg)
			if err != nil {
				return err
			}
		} else {
			for _, n := range batch {
				err := ns.CreateNode(ctx, n)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (ns NodeService) UpdateNode(ctx context.Context, n mnnode.MNode) error {
	node := ConvertNodeToDB(n)
	return ns.queries.UpdateFlowNode(ctx, gen.UpdateFlowNodeParams{
		ID:        node.ID,
		Name:      node.Name,
		PositionX: node.PositionX,
		PositionY: node.PositionY,
	})
}

func (ns NodeService) DeleteNode(ctx context.Context, id idwrap.IDWrap) error {
	return ns.queries.DeleteFlowNode(ctx, id)
}
