package snodeif

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
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
