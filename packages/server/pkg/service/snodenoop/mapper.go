package snodenoop

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertToDBNodeStart(ns mflow.NodeNoop) gen.FlowNodeNoop {
	return gen.FlowNodeNoop{
		FlowNodeID: ns.FlowNodeID,
		NodeType:   int16(ns.Type),
	}
}

func ConvertToModelNodeStart(ns gen.FlowNodeNoop) *mflow.NodeNoop {
	return &mflow.NodeNoop{
		FlowNodeID: ns.FlowNodeID,
		Type:       mflow.NoopTypes(ns.NodeType),
	}
}
