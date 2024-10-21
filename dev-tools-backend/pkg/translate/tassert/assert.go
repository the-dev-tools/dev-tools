package tassert

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/massert"
	requestv1 "dev-tools-spec/dist/buf/go/collection/item/request/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) *requestv1.Assert {
	return &requestv1.Assert{
		AssertId: a.ID.Bytes(),
		Value:    a.Value,
		Type:     requestv1.AssertType(a.Type),
	}
}

func SerializeAssertModelToRPCItem(a massert.Assert) *requestv1.AssertListItem {
	return &requestv1.AssertListItem{
		AssertId: a.ID.Bytes(),
		Path:     a.Path,
		Value:    a.Value,
		Type:     requestv1.AssertType(a.Type),
	}
}

func SerializeAssertRPCToModel(assert *requestv1.Assert, exampleID idwrap.IDWrap) (massert.Assert, error) {
	id, err := idwrap.NewFromBytes(assert.GetAssertId())
	if err != nil {
		return massert.Assert{}, err
	}
	a := SerializeAssertRPCToModelWithoutID(assert, exampleID)
	a.ID = id
	return a, nil
}

func SerializeAssertRPCToModelWithoutID(a *requestv1.Assert, exampleID idwrap.IDWrap) massert.Assert {
	return massert.Assert{
		ExampleID: exampleID,
		Path:      a.Path,
		Value:     a.Value,
		Type:      massert.AssertType(a.Type),
	}
}
