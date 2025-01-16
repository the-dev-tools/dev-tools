package tcondition

import (
	"fmt"
	"strings"
	"the-dev-tools/backend/pkg/model/mcondition"
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"
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

func DeserializeConditionRPCToModel(c *conditionv1.Condition) *mcondition.Condition {
	return &mcondition.Condition{
		Comparisons: DeserializeComparisonRPCToModel(c.Comparison),
	}
}

func SerializeComparisonModelToRPC(c mcondition.Comparison) (*conditionv1.Comparison, error) {
	return &conditionv1.Comparison{}, nil
}

func DeserializeComparisonRPCToModel(c *conditionv1.Comparison) mcondition.Comparison {
	path := ""
	for _, p := range c.Path {
		switch p.Kind {
		case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY:
			if p.Key == nil {
				break
			}
			path += "." + *p.Key
		case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX:
			path += fmt.Sprintf("[%d]", p.Index)
		case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_ANY:
			path += ".any"
		}
	}
	path = strings.TrimLeft(path, ".")

	return mcondition.Comparison{
		Kind:  mcondition.ComparisonKind(c.Kind),
		Path:  path,
		Value: c.Value,
	}
}
