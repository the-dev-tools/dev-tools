package tassert

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/translate/tcondition"
)

// Assert represents an assertion that can be made.
// TODO: Replace with actual protobuf type when available
type Assert struct {
	AssertId  []byte                 `protobuf:"bytes,1,opt,name=assert_id,json=assertId,proto3" json:"assert_id,omitempty"`
	Condition *tcondition.Condition  `protobuf:"bytes,2,opt,name=condition,proto3" json:"condition,omitempty"`
}

// GetAssertId returns the assert ID
func (a *Assert) GetAssertId() []byte {
	if a != nil {
		return a.AssertId
	}
	return nil
}

// AssertListItem represents an assert in a list.
// TODO: Replace with actual protobuf type when available
type AssertListItem struct {
	AssertId  []byte                 `protobuf:"bytes,1,opt,name=assert_id,json=assertId,proto3" json:"assert_id,omitempty"`
	Condition *tcondition.Condition  `protobuf:"bytes,2,opt,name=condition,proto3" json:"condition,omitempty"`
}

// AssertDeltaListItem represents an assert in a delta list.
// TODO: Replace with actual protobuf type when available
type AssertDeltaListItem struct {
	AssertId  []byte                 `protobuf:"bytes,1,opt,name=assert_id,json=assertId,proto3" json:"assert_id,omitempty"`
	Condition *tcondition.Condition  `protobuf:"bytes,2,opt,name=condition,proto3" json:"condition,omitempty"`
}

func SerializeAssertModelToRPC(a massert.Assert) (*Assert, error) {

	return &Assert{
		AssertId:  a.ID.Bytes(),
		Condition: tcondition.SeralizeConditionModelToRPC(a.Condition),
	}, nil
}

func SerializeAssertModelToRPCItem(a massert.Assert) (*AssertListItem, error) {
	assertRpc, err := SerializeAssertModelToRPC(a)
	if err != nil {
		return nil, err
	}

	return &AssertListItem{
		AssertId:  assertRpc.AssertId,
		Condition: assertRpc.Condition,
	}, nil
}

func SerializeAssertRPCToModel(rpcAssert *Assert, exampleID idwrap.IDWrap) (massert.Assert, error) {
	id, err := idwrap.NewFromBytes(rpcAssert.GetAssertId())
	if err != nil {
		return massert.Assert{}, err
	}

	var deltaParentIDPtr *idwrap.IDWrap

	modelAssert := SerializeAssertRPCToModelWithoutID(rpcAssert, exampleID, deltaParentIDPtr)
	modelAssert.ID = id
	return modelAssert, nil
}

func SerializeAssertRPCToModelWithoutID(a *Assert, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) massert.Assert {

	return massert.Assert{
		ExampleID:     exampleID,
		DeltaParentID: deltaParentIDPtr,
		Condition:     tcondition.DeserializeConditionRPCToModel(a.Condition),
	}
}

func SerializeAssertModelToRPCDeltaItem(a massert.Assert) (*AssertDeltaListItem, error) {
	assertRpc, err := SerializeAssertModelToRPC(a)
	if err != nil {
		return nil, err
	}

	return &AssertDeltaListItem{
		AssertId:  assertRpc.AssertId,
		Condition: assertRpc.Condition,
	}, nil
}
