package tbodyform

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyform"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
)

func SerializeFormModelToRPC(form mbodyform.BodyForm) *bodyv1.BodyFormItem {
	return &bodyv1.BodyFormItem{
		BodyId:      form.ID.Bytes(),
		Key:         form.BodyKey,
		Enabled:     form.Enable,
		Value:       form.Value,
		Description: form.Description,
	}
}

func SerializeFormModelToRPCItem(form mbodyform.BodyForm) *bodyv1.BodyFormItemListItem {
	return &bodyv1.BodyFormItemListItem{
		BodyId:      form.ID.Bytes(),
		Key:         form.BodyKey,
		Enabled:     form.Enable,
		Value:       form.Value,
		Description: form.Description,
	}
}

func SerializeFormRPCtoModel(form *bodyv1.BodyFormItem, ExampleID idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	b, err := SeralizeFormRPCToModelWithoutID(form, ExampleID)
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

func SeralizeFormRPCToModelWithoutID(form *bodyv1.BodyFormItem, ExampleID idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	return &mbodyform.BodyForm{
		ExampleID:   ExampleID,
		BodyKey:     form.Key,
		Description: form.Description,
		Enable:      form.Enabled,
		Value:       form.Value,
	}, nil
}
