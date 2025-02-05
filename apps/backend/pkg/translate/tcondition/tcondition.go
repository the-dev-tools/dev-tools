package tcondition

import (
	"the-dev-tools/backend/pkg/model/mcondition"
	"the-dev-tools/backend/pkg/reference"
	"the-dev-tools/backend/pkg/translate/tgeneric"
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
	refs, err := reference.ConvertStringPathToReferenceKeyArray(c.Path)
	if err != nil {
		return nil, err
	}

	rpcRefKeys := tgeneric.MassConvert(refs, reference.ConvertPkgKeyToRpc)

	return &conditionv1.Comparison{
		Kind:  conditionv1.ComparisonKind(c.Kind),
		Path:  rpcRefKeys,
		Value: c.Value,
	}, nil
}

func DeserializeComparisonRPCToModel(c *conditionv1.Comparison) (*mcondition.Comparison, error) {
	RefKeys := tgeneric.MassConvert(c.Path, reference.ConvertRpcKeyToPkgKey)
	compPath, err := reference.ConvertRefernceKeyArrayToStringPath(RefKeys)
	if err != nil {
		return nil, err
	}

	return &mcondition.Comparison{
		Kind:  mcondition.ComparisonKind(c.Kind),
		Path:  compPath,
		Value: c.Value,
	}, nil
}
