package tbodyurl

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyurl"
	bodyv1 "dev-tools-services/gen/body/v1"
)

func SerializeURLModelToRPC(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncodedItem {
	return &bodyv1.BodyUrlEncodedItem{
		Id:          urlEncoded.ID.String(),
		ExampleId:   urlEncoded.ExampleID.String(),
		Key:         urlEncoded.BodyKey,
		Enabled:     urlEncoded.Enable,
		Value:       urlEncoded.Value,
		Description: urlEncoded.Description,
	}
}

func SerializeURLRPCtoModel(urlEncoded *bodyv1.BodyUrlEncodedItem) (*mbodyurl.BodyURLEncoded, error) {
	ID, err := idwrap.NewWithParse(urlEncoded.GetId())
	if err != nil {
		return nil, err
	}
	ExampleID, err := idwrap.NewWithParse(urlEncoded.GetExampleId())
	if err != nil {
		return nil, err
	}

	return &mbodyurl.BodyURLEncoded{
		ID:          ID,
		ExampleID:   ExampleID,
		BodyKey:     urlEncoded.Key,
		Description: urlEncoded.Description,
		Enable:      urlEncoded.Enabled,
		Value:       urlEncoded.Value,
	}, nil
}

func SeralizeURLRPCToModelWithoutID(urlEncoded *bodyv1.BodyUrlEncodedItem) (*mbodyurl.BodyURLEncoded, error) {
	ExampleID, err := idwrap.NewWithParse(urlEncoded.GetExampleId())
	if err != nil {
		return nil, err
	}
	return &mbodyurl.BodyURLEncoded{
		ExampleID:   ExampleID,
		BodyKey:     urlEncoded.Key,
		Description: urlEncoded.Description,
		Enable:      urlEncoded.Enabled,
		Value:       urlEncoded.Value,
	}, nil
}
