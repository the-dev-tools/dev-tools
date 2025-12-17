package snodefor

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

func ConvertToDBNodeFor(nf mnfor.MNFor) gen.FlowNodeFor {
	return gen.FlowNodeFor{
		FlowNodeID:    nf.FlowNodeID,
		IterCount:     nf.IterCount,
		ErrorHandling: int8(nf.ErrorHandling),
		Expression:    nf.Condition.Comparisons.Expression,
	}
}

func ConvertToModelNodeFor(nf gen.FlowNodeFor) *mnfor.MNFor {
	return &mnfor.MNFor{
		FlowNodeID:    nf.FlowNodeID,
		IterCount:     nf.IterCount,
		ErrorHandling: mnfor.ErrorHandling(nf.ErrorHandling),
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: nf.Expression,
			},
		},
	}
}
