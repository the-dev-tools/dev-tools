package tbodyurl

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyurl"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
)

func SerializeURLModelToRPC(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncoded {

	return &bodyv1.BodyUrlEncoded{
		BodyId:      urlEncoded.ID.Bytes(),
		Key:         urlEncoded.BodyKey,
		Enabled:     urlEncoded.Enable,
		Value:       urlEncoded.Value,
		Description: urlEncoded.Description,
	}
}

func SerializeURLModelToRPCItem(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncodedListItem {

	return &bodyv1.BodyUrlEncodedListItem{
		BodyId:      urlEncoded.ID.Bytes(),
		Key:         urlEncoded.BodyKey,
		Enabled:     urlEncoded.Enable,
		Value:       urlEncoded.Value,
		Description: urlEncoded.Description,
	}
}

func SerializeURLRPCtoModel(urlEncoded *bodyv1.BodyUrlEncoded, exampleID idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	var deltaParentIDPtr *idwrap.IDWrap
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

func SeralizeURLRPCToModelWithoutID(urlEncoded *bodyv1.BodyUrlEncoded, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	return &mbodyurl.BodyURLEncoded{
		ExampleID:     exampleID,
		BodyKey:       urlEncoded.Key,
		DeltaParentID: deltaParentIDPtr,
		Description:   urlEncoded.Description,
		Enable:        urlEncoded.Enabled,
		Value:         urlEncoded.Value,
		Source:        mbodyurl.BodyURLEncodedSourceOrigin, // Default to origin
	}, nil
}

func SeralizeURLRPCToModelWithoutIDForDelta(urlEncoded *bodyv1.BodyUrlEncoded, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	return &mbodyurl.BodyURLEncoded{
		ExampleID:     exampleID,
		BodyKey:       urlEncoded.Key,
		DeltaParentID: deltaParentIDPtr,
		Description:   urlEncoded.Description,
		Enable:        urlEncoded.Enabled,
		Value:         urlEncoded.Value,
		Source:        mbodyurl.BodyURLEncodedSourceDelta, // Set to delta for delta creation
	}, nil
}

func SerializeURLModelToRPCDeltaItem(urlEncoded mbodyurl.BodyURLEncoded) *bodyv1.BodyUrlEncodedDeltaListItem {
	sourceKind := urlEncoded.Source.ToSourceKind()
	return &bodyv1.BodyUrlEncodedDeltaListItem{
		BodyId:      urlEncoded.ID.Bytes(),
		Key:         urlEncoded.BodyKey,
		Enabled:     urlEncoded.Enable,
		Value:       urlEncoded.Value,
		Description: urlEncoded.Description,
		Source:      &sourceKind,
	}
}
