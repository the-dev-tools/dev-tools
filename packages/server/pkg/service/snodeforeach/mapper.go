package snodeforeach

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertToDBNodeFor(nf mflow.NodeForEach) gen.FlowNodeForEach {
	return gen.FlowNodeForEach{
		FlowNodeID:     nf.FlowNodeID,
		IterExpression: nf.IterExpression,
		ErrorHandling:  int8(nf.ErrorHandling),
		Expression:     nf.Condition.Comparisons.Expression,
	}
}

func ConvertToModelNodeFor(nf gen.FlowNodeForEach) *mflow.NodeForEach {
	return &mflow.NodeForEach{
		FlowNodeID:     nf.FlowNodeID,
		IterExpression: nf.IterExpression,
		ErrorHandling:  mflow.ErrorHandling(nf.ErrorHandling),
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: nf.Expression,
			},
		},
	}
}
