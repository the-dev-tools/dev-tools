package snode

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mnnode"
)

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
