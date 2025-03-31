package tbodyform

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
)

func SerializeFormModelToRPC(form mbodyform.BodyForm) *bodyv1.BodyFormItem {
	var deltaParentIDBytes []byte
	if form.DeltaParentID == nil {
		deltaParentIDBytes = form.DeltaParentID.Bytes()
	}

	return &bodyv1.BodyFormItem{
		BodyId:       form.ID.Bytes(),
		ParentBodyId: deltaParentIDBytes,
		Key:          form.BodyKey,
		Enabled:      form.Enable,
		Value:        form.Value,
		Description:  form.Description,
	}
}

func SerializeFormModelToRPCItem(form mbodyform.BodyForm) *bodyv1.BodyFormItemListItem {
	var deltaParentIDBytes []byte
	if form.DeltaParentID != nil {
		deltaParentIDBytes = form.DeltaParentID.Bytes()
	}

	return &bodyv1.BodyFormItemListItem{
		BodyId:       form.ID.Bytes(),
		ParentBodyId: deltaParentIDBytes,
		Key:          form.BodyKey,
		Enabled:      form.Enable,
		Value:        form.Value,
		Description:  form.Description,
	}
}

func SerializeFormRPCtoModel(form *bodyv1.BodyFormItem, ExampleID idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	var deltaParentIDPtr *idwrap.IDWrap
	if len(form.GetParentBodyId()) > 0 {
		deltaParentID, err := idwrap.NewFromBytes(form.GetParentBodyId())
		if err != nil {
			return nil, err
		}
		deltaParentIDPtr = &deltaParentID
	}

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

func SeralizeFormRPCToModelWithoutID(form *bodyv1.BodyFormItem, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	return &mbodyform.BodyForm{
		ExampleID:     exampleID,
		DeltaParentID: deltaParentIDPtr,
		BodyKey:       form.Key,
		Description:   form.Description,
		Enable:        form.Enabled,
		Value:         form.Value,
	}, nil
}
