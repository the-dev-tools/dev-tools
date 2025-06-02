package tassert

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/translate/tcondition"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) (*requestv1.Assert, error) {

	return &requestv1.Assert{
		AssertId:  a.ID.Bytes(),
		Condition: tcondition.SeralizeConditionModelToRPC(a.Condition),
	}, nil
}

func SerializeAssertModelToRPCItem(a massert.Assert) (*requestv1.AssertListItem, error) {
	assertRpc, err := SerializeAssertModelToRPC(a)
	if err != nil {
		return nil, err
	}

	return &requestv1.AssertListItem{
		AssertId:  assertRpc.AssertId,
		Condition: assertRpc.Condition,
	}, nil
}

func SerializeAssertRPCToModel(rpcAssert *requestv1.Assert, exampleID idwrap.IDWrap) (massert.Assert, error) {
	id, err := idwrap.NewFromBytes(rpcAssert.GetAssertId())
	if err != nil {
		return massert.Assert{}, err
	}

	var deltaParentIDPtr *idwrap.IDWrap

	modelAssert := SerializeAssertRPCToModelWithoutID(rpcAssert, exampleID, deltaParentIDPtr)
	modelAssert.ID = id
	return modelAssert, nil
}

func SerializeAssertRPCToModelWithoutID(a *requestv1.Assert, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) massert.Assert {

	return massert.Assert{
		ExampleID:     exampleID,
		DeltaParentID: deltaParentIDPtr,
		Condition:     tcondition.DeserializeConditionRPCToModel(a.Condition),
	}
}

func SerializeAssertModelToRPCDeltaItem(a massert.Assert) (*requestv1.AssertDeltaListItem, error) {
	assertRpc, err := SerializeAssertModelToRPC(a)
	if err != nil {
		return nil, err
	}

	return &requestv1.AssertDeltaListItem{
		AssertId:  assertRpc.AssertId,
		Condition: assertRpc.Condition,
	}, nil
}
