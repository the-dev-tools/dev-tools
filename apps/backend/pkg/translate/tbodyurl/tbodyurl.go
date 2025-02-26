package tbodyurl

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
)

func SerializeURLModelToRPC(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncodedItem {
	var deltaParentIDBytes []byte
	if urlEncoded.DeltaParentID != nil {
		deltaParentIDBytes = urlEncoded.DeltaParentID.Bytes()
	}

	return &bodyv1.BodyUrlEncodedItem{
		BodyId:       urlEncoded.ID.Bytes(),
		ParentBodyId: deltaParentIDBytes,
		Key:          urlEncoded.BodyKey,
		Enabled:      urlEncoded.Enable,
		Value:        urlEncoded.Value,
		Description:  urlEncoded.Description,
	}
}

func SerializeURLModelToRPCItem(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncodedItemListItem {
	var deltaParentIDBytes []byte
	if urlEncoded.DeltaParentID != nil {
		deltaParentIDBytes = urlEncoded.DeltaParentID.Bytes()
	}

	return &bodyv1.BodyUrlEncodedItemListItem{
		BodyId:       urlEncoded.ID.Bytes(),
		ParentBodyId: deltaParentIDBytes,
		Key:          urlEncoded.BodyKey,
		Enabled:      urlEncoded.Enable,
		Value:        urlEncoded.Value,
		Description:  urlEncoded.Description,
	}
}

func SerializeURLRPCtoModel(urlEncoded *bodyv1.BodyUrlEncodedItem, exampleID idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	var deltaParentIDPtr *idwrap.IDWrap
	if len(urlEncoded.GetParentBodyId()) > 0 {
		deltaParentID, err := idwrap.NewFromBytes(urlEncoded.GetParentBodyId())
		if err != nil {
			return nil, err
		}
		deltaParentIDPtr = &deltaParentID
	}

	b, err := SeralizeURLRPCToModelWithoutID(urlEncoded, exampleID, deltaParentIDPtr)
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

func SeralizeURLRPCToModelWithoutID(urlEncoded *bodyv1.BodyUrlEncodedItem, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	return &mbodyurl.BodyURLEncoded{
		ExampleID:     exampleID,
		BodyKey:       urlEncoded.Key,
		DeltaParentID: deltaParentIDPtr,
		Description:   urlEncoded.Description,
		Enable:        urlEncoded.Enabled,
		Value:         urlEncoded.Value,
	}, nil
}
