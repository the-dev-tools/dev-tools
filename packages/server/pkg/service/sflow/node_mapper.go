package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
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
