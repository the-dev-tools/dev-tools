package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertNodeForEachToDB(nf mflow.NodeForEach) gen.FlowNodeForEach {
	return gen.FlowNodeForEach{
		FlowNodeID:     nf.FlowNodeID,
		IterExpression: nf.IterExpression,
		ErrorHandling:  int8(nf.ErrorHandling),
		Expression:     nf.Condition.Comparisons.Expression,
	}
}

func ConvertDBToNodeForEach(nf gen.FlowNodeForEach) *mflow.NodeForEach {
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
