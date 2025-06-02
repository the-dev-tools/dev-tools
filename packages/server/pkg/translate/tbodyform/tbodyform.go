package tbodyform

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
)

func SerializeFormModelToRPC(form mbodyform.BodyForm) *bodyv1.BodyForm {

	return &bodyv1.BodyForm{
		BodyId:      form.ID.Bytes(),
		Key:         form.BodyKey,
		Enabled:     form.Enable,
		Value:       form.Value,
		Description: form.Description,
	}
}

func SerializeFormModelToRPCItem(form mbodyform.BodyForm) *bodyv1.BodyFormListItem {

	return &bodyv1.BodyFormListItem{
		BodyId:      form.ID.Bytes(),
		Key:         form.BodyKey,
		Enabled:     form.Enable,
		Value:       form.Value,
		Description: form.Description,
	}
}

func SerializeFormRPCtoModel(form *bodyv1.BodyForm, ExampleID idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	var deltaParentIDPtr *idwrap.IDWrap

	b, err := SeralizeFormRPCToModelWithoutID(form, ExampleID, deltaParentIDPtr)
	if err != nil {
		return nil, err
	}
	ID, err := idwrap.NewFromBytes(form.GetBodyId())
	if err != nil {
		return nil, err
	}
	b.ID = ID
	return b, nil
}

func SeralizeFormRPCToModelWithoutID(form *bodyv1.BodyForm, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	return &mbodyform.BodyForm{
		ExampleID:     exampleID,
		DeltaParentID: deltaParentIDPtr,
		BodyKey:       form.Key,
		Description:   form.Description,
		Enable:        form.Enabled,
		Value:         form.Value,
	}, nil
}

func SerializeFormModelToRPCDeltaItem(form mbodyform.BodyForm) *bodyv1.BodyFormDeltaListItem {
	return &bodyv1.BodyFormDeltaListItem{
		BodyId:      form.ID.Bytes(),
		Key:         form.BodyKey,
		Enabled:     form.Enable,
		Value:       form.Value,
		Description: form.Description,
	}
}
