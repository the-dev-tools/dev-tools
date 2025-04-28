package tcondition

import (
	"the-dev-tools/server/pkg/model/mcondition"
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
)

func SeralizeConditionModelToRPC(c mcondition.Condition) (*conditionv1.Condition, error) {
	comp, err := SerializeComparisonModelToRPC(c.Comparisons)
	if err != nil {
		return nil, err
	}

	return &conditionv1.Condition{
		Comparison: comp,
	}, nil
}

func DeserializeConditionRPCToModel(c *conditionv1.Condition) (*mcondition.Condition, error) {
	comp, err := DeserializeComparisonRPCToModel(c.Comparison)
	if err != nil {
		return nil, err
	}

	return &mcondition.Condition{
		Comparisons: *comp,
	}, nil
}

func SerializeComparisonModelToRPC(c mcondition.Comparison) (*conditionv1.Comparison, error) {

	return &conditionv1.Comparison{
		Kind:  conditionv1.ComparisonKind(c.Kind),
		Left:  c.Path,
		Right: c.Value,
	}, nil
}

func DeserializeComparisonRPCToModel(c *conditionv1.Comparison) (*mcondition.Comparison, error) {

	return &mcondition.Comparison{
		Kind:  mcondition.ComparisonKind(c.Kind),
		Path:  c.Left,
		Value: c.Right,
	}, nil
}
