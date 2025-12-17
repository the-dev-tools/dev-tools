package snodeif

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
)

func ConvertToDBNodeIf(ni mnif.MNIF) gen.FlowNodeCondition {
	return gen.FlowNodeCondition{
		FlowNodeID: ni.FlowNodeID,
		Expression: ni.Condition.Comparisons.Expression,
	}
}

func ConvertToModelNodeIf(ni gen.FlowNodeCondition) *mnif.MNIF {
	return &mnif.MNIF{
		FlowNodeID: ni.FlowNodeID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: ni.Expression,
			},
		},
	}
}
