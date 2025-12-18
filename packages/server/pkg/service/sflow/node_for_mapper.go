package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertNodeForToDB(nf mflow.NodeFor) gen.FlowNodeFor {
	return gen.FlowNodeFor{
		FlowNodeID:    nf.FlowNodeID,
		IterCount:     nf.IterCount,
		ErrorHandling: int8(nf.ErrorHandling),
		Expression:    nf.Condition.Comparisons.Expression,
	}
}

func ConvertDBToNodeFor(nf gen.FlowNodeFor) *mflow.NodeFor {
	return &mflow.NodeFor{
		FlowNodeID:    nf.FlowNodeID,
		IterCount:     nf.IterCount,
		ErrorHandling: mflow.ErrorHandling(nf.ErrorHandling),
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: nf.Expression,
			},
		},
	}
}
