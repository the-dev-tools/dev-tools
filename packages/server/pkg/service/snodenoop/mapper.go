package snodenoop

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
)

func ConvertToDBNodeStart(ns mnnoop.NoopNode) gen.FlowNodeNoop {
	return gen.FlowNodeNoop{
		FlowNodeID: ns.FlowNodeID,
		NodeType:   int16(ns.Type),
	}
}

func ConvertToModelNodeStart(ns gen.FlowNodeNoop) *mnnoop.NoopNode {
	return &mnnoop.NoopNode{
		FlowNodeID: ns.FlowNodeID,
		Type:       mnnoop.NoopTypes(ns.NodeType),
	}
}
