package tbodyform

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyform"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
)

func SerializeFormModelToRPC(form mbodyform.BodyForm) *itemapiexamplev1.BodyFormDataItem {
	return &itemapiexamplev1.BodyFormDataItem{
		Id:          form.ID.String(),
		ExampleId:   form.ExampleID.String(),
		Key:         form.BodyKey,
		Enabled:     form.Enable,
		Value:       form.Value,
		Description: form.Description,
	}
}

func SerializeFormRPCtoModel(form *itemapiexamplev1.BodyFormDataItem) (*mbodyform.BodyForm, error) {
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
