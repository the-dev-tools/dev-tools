package tbodyform

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyform"
	bodyv1 "dev-tools-services/gen/body/v1"
)

func SerializeFormModelToRPC(form mbodyform.BodyForm) *bodyv1.BodyFormItem {
	return &bodyv1.BodyFormItem{
		Id:          form.ID.String(),
		ExampleId:   form.ExampleID.String(),
		Key:         form.BodyKey,
		Enabled:     form.Enable,
		Value:       form.Value,
		Description: form.Description,
	}
}

func SerializeFormRPCtoModel(form *bodyv1.BodyFormItem) (*mbodyform.BodyForm, error) {
	ID, err := idwrap.NewWithParse(form.GetId())
	if err != nil {
		return nil, err
	}
	ExampleID, err := idwrap.NewWithParse(form.GetExampleId())
	if err != nil {
		return nil, err
	}

	return &mbodyform.BodyForm{
		ID:          ID,
		ExampleID:   ExampleID,
		BodyKey:     form.Key,
		Description: form.Description,
		Enable:      form.Enabled,
		Value:       form.Value,
	}, nil
}
