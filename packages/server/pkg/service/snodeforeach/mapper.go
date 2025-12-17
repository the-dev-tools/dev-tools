package snodeforeach

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
)

func ConvertToDBNodeFor(nf mnforeach.MNForEach) gen.FlowNodeForEach {
	return gen.FlowNodeForEach{
		FlowNodeID:     nf.FlowNodeID,
		IterExpression: nf.IterExpression,
		ErrorHandling:  int8(nf.ErrorHandling),
		Expression:     nf.Condition.Comparisons.Expression,
	}
}

func ConvertToModelNodeFor(nf gen.FlowNodeForEach) *mnforeach.MNForEach {
	return &mnforeach.MNForEach{
		FlowNodeID:     nf.FlowNodeID,
		IterExpression: nf.IterExpression,
		ErrorHandling:  mnfor.ErrorHandling(nf.ErrorHandling),
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: nf.Expression,
			},
		},
	}
}
