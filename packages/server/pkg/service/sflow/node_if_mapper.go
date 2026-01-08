package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertToDBNodeIf(ni mflow.NodeIf) gen.FlowNodeCondition {
	return gen.FlowNodeCondition{
		FlowNodeID: ni.FlowNodeID,
		Expression: ni.Condition.Comparisons.Expression,
	}
}

func ConvertToModelNodeIf(ni gen.FlowNodeCondition) *mflow.NodeIf {
	return &mflow.NodeIf{
		FlowNodeID: ni.FlowNodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: ni.Expression,
			},
		},
	}
}
