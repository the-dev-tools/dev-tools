package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertNodeToDB(n mflow.Node) *gen.FlowNode {
	return &gen.FlowNode{
		ID:        n.ID,
		FlowID:    n.FlowID,
		Name:      n.Name,
		NodeKind:  n.NodeKind,
		PositionX: n.PositionX,
		PositionY: n.PositionY,
		State:     n.State,
	}
}

func ConvertNodeToModel(n gen.FlowNode) *mflow.Node {
	return &mflow.Node{
		ID:        n.ID,
		FlowID:    n.FlowID,
		Name:      n.Name,
		NodeKind:  n.NodeKind,
		PositionX: n.PositionX,
		PositionY: n.PositionY,
		State:     n.State,
	}
}
