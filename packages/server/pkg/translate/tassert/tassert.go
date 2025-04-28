package tassert

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) (*requestv1.Assert, error) {

	var deltaParentIDBytes []byte
	if a.DeltaParentID != nil {
		deltaParentIDBytes = a.DeltaParentID.Bytes()
	}

	return &requestv1.Assert{
		AssertId:       a.ID.Bytes(),
		ParentAssertId: deltaParentIDBytes,
		Condition: &conditionv1.Condition{
			Comparison: &conditionv1.Comparison{
				Kind:  conditionv1.ComparisonKind(a.Type),
				Left:  a.Path,
				Right: a.Value,
			},
		},
	}, nil
}

func SerializeAssertModelToRPCItem(a massert.Assert) (*requestv1.AssertListItem, error) {
	assertRpc, err := SerializeAssertModelToRPC(a)
	if err != nil {
		return nil, err
	}

	return &requestv1.AssertListItem{
		AssertId:       assertRpc.AssertId,
		ParentAssertId: assertRpc.ParentAssertId,
		Condition:      assertRpc.Condition,
	}, nil
}

func SerializeAssertRPCToModel(assert *requestv1.Assert, exampleID idwrap.IDWrap) (massert.Assert, error) {
	id, err := idwrap.NewFromBytes(assert.GetAssertId())
	if err != nil {
		return massert.Assert{}, err
	}

	var deltaParentIDPtr *idwrap.IDWrap
	if len(assert.GetParentAssertId()) > 0 {
		deltaParentID, err := idwrap.NewFromBytes(assert.GetParentAssertId())
		if err != nil {
			return massert.Assert{}, err
		}
		deltaParentIDPtr = &deltaParentID
	}

	a := SerializeAssertRPCToModelWithoutID(assert, exampleID, deltaParentIDPtr)
	a.ID = id
	return a, nil
}

func SerializeAssertRPCToModelWithoutID(a *requestv1.Assert, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) massert.Assert {
	var path, value string
	massertType := massert.AssertTypeUndefined
	if a.Condition != nil {
		if a.Condition.Comparison != nil {
			comp := a.Condition.Comparison
			path = comp.Left
			value = comp.Right
			massertType = massert.AssertType(comp.Kind)
		}
	}

	return massert.Assert{
		ExampleID:     exampleID,
		Path:          path,
		Value:         value,
		Type:          massertType,
		DeltaParentID: deltaParentIDPtr,
	}
}
