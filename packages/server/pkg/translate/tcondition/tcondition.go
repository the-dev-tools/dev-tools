package tcondition

import (
	"the-dev-tools/server/pkg/model/mcondition"
)

// Condition represents a condition that can be evaluated.
// TODO: Replace with actual protobuf type when available
type Condition struct {
	Comparison *Comparison `protobuf:"bytes,1,opt,name=comparison,proto3" json:"comparison,omitempty"`
}

// Comparison represents a comparison expression.
// TODO: Replace with actual protobuf type when available
type Comparison struct {
	Expression string `protobuf:"bytes,1,opt,name=expression,proto3" json:"expression,omitempty"`
}

func SeralizeConditionModelToRPC(c mcondition.Condition) *Condition {
	return &Condition{
		Comparison: SerializeComparisonModelToRPC(c.Comparisons),
	}
}

func DeserializeConditionRPCToModel(c *Condition) mcondition.Condition {
	if c == nil {
		return mcondition.Condition{}
	}
	return mcondition.Condition{
		Comparisons: DeserializeComparisonRPCToModel(c.Comparison),
	}
}

func SerializeComparisonModelToRPC(c mcondition.Comparison) *Comparison {

	return &Comparison{
		Expression: c.Expression,
	}
}

func DeserializeComparisonRPCToModel(c *Comparison) mcondition.Comparison {
	if c == nil {
		return mcondition.Comparison{}
	}
	return mcondition.Comparison{
		Expression: c.Expression,
	}
}
