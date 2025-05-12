package tcondition

import (
	"the-dev-tools/server/pkg/model/mcondition"
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
)

func SeralizeConditionModelToRPC(c mcondition.Condition) *conditionv1.Condition {
	return &conditionv1.Condition{
		Comparison: SerializeComparisonModelToRPC(c.Comparisons),
	}
}

func DeserializeConditionRPCToModel(c *conditionv1.Condition) mcondition.Condition {
	if c == nil {
		return mcondition.Condition{}
	}
	return mcondition.Condition{
		Comparisons: DeserializeComparisonRPCToModel(c.Comparison),
	}
}

func SerializeComparisonModelToRPC(c mcondition.Comparison) *conditionv1.Comparison {

	return &conditionv1.Comparison{
		Expression: c.Expression,
	}
}

func DeserializeComparisonRPCToModel(c *conditionv1.Comparison) mcondition.Comparison {
	if c == nil {
		return mcondition.Comparison{}
	}
	return mcondition.Comparison{
		Expression: c.Expression,
	}
}
