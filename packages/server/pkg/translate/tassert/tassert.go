package tassert

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/translate/tcondition"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) (*requestv1.Assert, error) {

	var deltaParentIDBytes []byte
	if a.DeltaParentID != nil {
		deltaParentIDBytes = a.DeltaParentID.Bytes()
	}

	return &requestv1.Assert{
		AssertId:       a.ID.Bytes(),
		ParentAssertId: deltaParentIDBytes,
		Condition:      tcondition.SeralizeConditionModelToRPC(a.Condition),
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

func SerializeAssertRPCToModel(rpcAssert *requestv1.Assert, exampleID idwrap.IDWrap) (massert.Assert, error) {
	id, err := idwrap.NewFromBytes(rpcAssert.GetAssertId())
	if err != nil {
		return massert.Assert{}, err
	}

	var deltaParentIDPtr *idwrap.IDWrap
	if len(rpcAssert.GetParentAssertId()) > 0 {
		deltaParentID, err := idwrap.NewFromBytes(rpcAssert.GetParentAssertId())
		if err != nil {
			return massert.Assert{}, err
		}
		deltaParentIDPtr = &deltaParentID
	}

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
