package tbodyurl

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyurl"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func SerializeURLModelToRPC(urlEncoded mbodyurl.BodyURLEncoded) *httpv1.HttpBodyUrlEncoded {

	return &httpv1.HttpBodyUrlEncoded{
		HttpBodyUrlEncodedId: urlEncoded.ID.Bytes(),
		Key:                  urlEncoded.BodyKey,
		Enabled:              urlEncoded.Enable,
		Value:                urlEncoded.Value,
		Description:          urlEncoded.Description,
	}
}

// BodyUrlEncodedListItem represents a URL encoded body in a list.
// TODO: Replace with actual protobuf type when available
type BodyUrlEncodedListItem struct {
	HttpBodyUrlEncodedId []byte `protobuf:"bytes,1,opt,name=http_body_url_encoded_id,json=httpBodyUrlEncodedId,proto3" json:"http_body_url_encoded_id,omitempty"`
	Key                  string `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	Enabled              bool   `protobuf:"varint,3,opt,name=enabled,proto3" json:"enabled,omitempty"`
	Value                string `protobuf:"bytes,4,opt,name=value,proto3" json:"value,omitempty"`
	Description          string `protobuf:"bytes,5,opt,name=description,proto3" json:"description,omitempty"`
}

func SerializeURLModelToRPCItem(urlEncoded mbodyurl.BodyURLEncoded) *BodyUrlEncodedListItem {

	return &BodyUrlEncodedListItem{
		HttpBodyUrlEncodedId: urlEncoded.ID.Bytes(),
		Key:                  urlEncoded.BodyKey,
		Enabled:              urlEncoded.Enable,
		Value:                urlEncoded.Value,
		Description:          urlEncoded.Description,
	}
}

func SerializeURLRPCtoModel(urlEncoded *httpv1.HttpBodyUrlEncoded, exampleID idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	var deltaParentIDPtr *idwrap.IDWrap
	b, err := SeralizeURLRPCToModelWithoutID(urlEncoded, exampleID, deltaParentIDPtr)
	if err != nil {
		return nil, err
	}
	ID, err := idwrap.NewFromBytes(urlEncoded.HttpBodyUrlEncodedId)
	if err != nil {
		return nil, err
	}
	b.ID = ID
	return b, nil
}

func SeralizeURLRPCToModelWithoutID(urlEncoded *httpv1.HttpBodyUrlEncoded, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	return &mbodyurl.BodyURLEncoded{
		ExampleID:     exampleID,
		BodyKey:       urlEncoded.Key,
		DeltaParentID: deltaParentIDPtr,
		Description:   urlEncoded.Description,
		Enable:        urlEncoded.Enabled,
		Value:         urlEncoded.Value,
	}, nil
}

func SeralizeURLRPCToModelWithoutIDForDelta(urlEncoded *httpv1.HttpBodyUrlEncoded, exampleID idwrap.IDWrap, deltaParentIDPtr *idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	return &mbodyurl.BodyURLEncoded{
		ExampleID:     exampleID,
		BodyKey:       urlEncoded.Key,
		DeltaParentID: deltaParentIDPtr,
		Description:   urlEncoded.Description,
		Enable:        urlEncoded.Enabled,
		Value:         urlEncoded.Value,
	}, nil
}

// BodyUrlEncodedDeltaListItem represents a URL encoded body delta in a list.
// TODO: Replace with actual protobuf type when available
type BodyUrlEncodedDeltaListItem struct {
	BodyId      []byte `protobuf:"bytes,1,opt,name=body_id,json=bodyId,proto3" json:"body_id,omitempty"`
	Key         string `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	Enabled     bool   `protobuf:"varint,3,opt,name=enabled,proto3" json:"enabled,omitempty"`
	Value       string `protobuf:"bytes,4,opt,name=value,proto3" json:"value,omitempty"`
	Description string `protobuf:"bytes,5,opt,name=description,proto3" json:"description,omitempty"`
}

func SerializeURLModelToRPCDeltaItem(urlEncoded mbodyurl.BodyURLEncoded) *BodyUrlEncodedDeltaListItem {
	// Note: sourceKind should be determined dynamically in the caller using DetermineDeltaType
	return &BodyUrlEncodedDeltaListItem{
		BodyId:      urlEncoded.ID.Bytes(),
		Key:         urlEncoded.BodyKey,
		Enabled:     urlEncoded.Enable,
		Value:       urlEncoded.Value,
		Description: urlEncoded.Description,
		// Source field should be set by the caller
	}
}
