package tbodyurl

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyurl"
	bodyv1 "dev-tools-spec/dist/buf/go/collection/item/body/v1"
)

func SerializeURLModelToRPC(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncodedItem {
	return &bodyv1.BodyUrlEncodedItem{
		BodyId:      urlEncoded.ID.Bytes(),
		Key:         urlEncoded.BodyKey,
		Enabled:     urlEncoded.Enable,
		Value:       urlEncoded.Value,
		Description: urlEncoded.Description,
	}
}

func SerializeURLModelToRPCItem(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncodedItemListItem {
	return &bodyv1.BodyUrlEncodedItemListItem{
		BodyId:      urlEncoded.ID.Bytes(),
		Key:         urlEncoded.BodyKey,
		Enabled:     urlEncoded.Enable,
		Value:       urlEncoded.Value,
		Description: urlEncoded.Description,
	}
}

func SerializeURLRPCtoModel(urlEncoded *bodyv1.BodyUrlEncodedItem, exampleID idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	b, err := SeralizeURLRPCToModelWithoutID(urlEncoded, exampleID)
	if err != nil {
		return nil, err
	}
	ID, err := idwrap.NewFromBytes(urlEncoded.GetBodyId())
	if err != nil {
		return nil, err
	}
	b.ID = ID
	return b, nil
}

func SeralizeURLRPCToModelWithoutID(urlEncoded *bodyv1.BodyUrlEncodedItem, exampleID idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	return &mbodyurl.BodyURLEncoded{
		ExampleID:   exampleID,
		BodyKey:     urlEncoded.Key,
		Description: urlEncoded.Description,
		Enable:      urlEncoded.Enabled,
		Value:       urlEncoded.Value,
	}, nil
}
