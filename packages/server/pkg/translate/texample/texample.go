package texample

import (
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
)

// TODO: These types don't exist in generated protobuf. Remove or replace when available.

// Stub types for missing protobuf
type BodyKind int32
const (
	BodyKind_BODY_KIND_UNSPECIFIED BodyKind = 0
	BodyKind_BODY_KIND_FORM_ARRAY BodyKind = 1
	BodyKind_BODY_KIND_URL_ENCODED_ARRAY BodyKind = 2
	BodyKind_BODY_KIND_RAW BodyKind = 3
)

type Example struct {
	ExampleId      []byte
	Name           string
	BodyKind       BodyKind
	LastResponseId []byte
	Breadcrumbs    []*ExampleBreadcrumb
}

type ExampleListItem struct {
	ExampleId      []byte
	Name           string
	LastResponseId []byte
}

type ExampleBreadcrumb struct {
	// TODO: Add fields when available
}

func modelBodyTypeToProto(bodyType mitemapiexample.BodyType) (BodyKind, error) {
	switch bodyType {
	case mitemapiexample.BodyTypeNone:
		return BodyKind_BODY_KIND_UNSPECIFIED, nil
	case mitemapiexample.BodyTypeForm:
		return BodyKind_BODY_KIND_FORM_ARRAY, nil
	case mitemapiexample.BodyTypeUrlencoded:
		return BodyKind_BODY_KIND_URL_ENCODED_ARRAY, nil
	case mitemapiexample.BodyTypeRaw:
		return BodyKind_BODY_KIND_RAW, nil
	default:
		return fallbackBodyKind(), fmt.Errorf("unknown body type %d", bodyType)
	}
}

func protoBodyKindToModel(kind BodyKind) (mitemapiexample.BodyType, error) {
	switch kind {
	case BodyKind_BODY_KIND_UNSPECIFIED:
		return mitemapiexample.BodyTypeNone, nil
	case BodyKind_BODY_KIND_FORM_ARRAY:
		return mitemapiexample.BodyTypeForm, nil
	case BodyKind_BODY_KIND_URL_ENCODED_ARRAY:
		return mitemapiexample.BodyTypeUrlencoded, nil
	case BodyKind_BODY_KIND_RAW:
		return mitemapiexample.BodyTypeRaw, nil
	default:
		return 0, fmt.Errorf("unknown body kind enum %v", kind)
	}
}

func fallbackBodyKind() BodyKind {
	return BodyKind_BODY_KIND_UNSPECIFIED
}

func SerializeModelToRPC(ex mitemapiexample.ItemApiExample, lastResp *idwrap.IDWrap, exampleBreadcrumbs []*ExampleBreadcrumb) *Example {
	var lastResponseBytes []byte
	if lastResp != nil {
		lastResponseBytes = lastResp.Bytes()
	}

	bodyKind := fallbackBodyKind()
	if converted, err := modelBodyTypeToProto(ex.BodyType); err == nil {
		bodyKind = converted
	}

	return &Example{
		ExampleId:      ex.ID.Bytes(),
		Name:           ex.Name,
		BodyKind:       bodyKind,
		LastResponseId: lastResponseBytes,
		Breadcrumbs:    exampleBreadcrumbs,
	}
}

func SerializeModelToRPCItem(ex mitemapiexample.ItemApiExample, lastRespID *idwrap.IDWrap) *ExampleListItem {
	var lastResp []byte = nil
	if lastRespID != nil {
		lastResp = lastRespID.Bytes()
	}

	return &ExampleListItem{
		ExampleId:      ex.ID.Bytes(),
		Name:           ex.Name,
		LastResponseId: lastResp,
	}
}

func DeserializeRPCToModel(ex *Example) (mitemapiexample.ItemApiExample, error) {
	if ex == nil {
		return mitemapiexample.ItemApiExample{}, nil
	}

	id, err := idwrap.NewFromBytes(ex.ExampleId)
	if err != nil {
		return mitemapiexample.ItemApiExample{}, err
	}

	bodyType, err := protoBodyKindToModel(ex.BodyKind)
	if err != nil {
		return mitemapiexample.ItemApiExample{}, err
	}

	return mitemapiexample.ItemApiExample{
		ID:       id,
		BodyType: bodyType,
		Name:     ex.Name,
	}, nil
}