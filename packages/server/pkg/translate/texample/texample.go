package texample

import (
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
)

func modelBodyTypeToProto(bodyType mitemapiexample.BodyType) (bodyv1.BodyKind, error) {
	switch bodyType {
	case mitemapiexample.BodyTypeNone:
		return bodyv1.BodyKind_BODY_KIND_UNSPECIFIED, nil
	case mitemapiexample.BodyTypeForm:
		return bodyv1.BodyKind_BODY_KIND_FORM_ARRAY, nil
	case mitemapiexample.BodyTypeUrlencoded:
		return bodyv1.BodyKind_BODY_KIND_URL_ENCODED_ARRAY, nil
	case mitemapiexample.BodyTypeRaw:
		return bodyv1.BodyKind_BODY_KIND_RAW, nil
	default:
		return fallbackBodyKind(), fmt.Errorf("unknown body type %d", bodyType)
	}
}

func protoBodyKindToModel(kind bodyv1.BodyKind) (mitemapiexample.BodyType, error) {
	switch kind {
	case bodyv1.BodyKind_BODY_KIND_UNSPECIFIED:
		return mitemapiexample.BodyTypeNone, nil
	case bodyv1.BodyKind_BODY_KIND_FORM_ARRAY:
		return mitemapiexample.BodyTypeForm, nil
	case bodyv1.BodyKind_BODY_KIND_URL_ENCODED_ARRAY:
		return mitemapiexample.BodyTypeUrlencoded, nil
	case bodyv1.BodyKind_BODY_KIND_RAW:
		return mitemapiexample.BodyTypeRaw, nil
	default:
		return 0, fmt.Errorf("unknown body kind enum %v", kind)
	}
}

// fallbackBodyKind returns the BodyKind value sent when the model enum is unknown.
func fallbackBodyKind() bodyv1.BodyKind {
	return bodyv1.BodyKind_BODY_KIND_UNSPECIFIED
}

func SerializeModelToRPC(ex mitemapiexample.ItemApiExample, lastResp *idwrap.IDWrap, exampleBreadcrumbs []*examplev1.ExampleBreadcrumb) *examplev1.Example {
	var lastResponseBytes []byte
	if lastResp != nil {
		lastResponseBytes = lastResp.Bytes()
	}

	bodyKind := fallbackBodyKind()
	if converted, err := modelBodyTypeToProto(ex.BodyType); err == nil {
		bodyKind = converted
	}

	return &examplev1.Example{
		ExampleId:      ex.ID.Bytes(),
		Name:           ex.Name,
		BodyKind:       bodyKind,
		LastResponseId: lastResponseBytes,
		Breadcrumbs:    exampleBreadcrumbs,
	}
}

func SerializeModelToRPCItem(ex mitemapiexample.ItemApiExample, lastRespID *idwrap.IDWrap) *examplev1.ExampleListItem {
	var lastResp []byte = nil
	if lastRespID != nil {
		lastResp = lastRespID.Bytes()
	}

	return &examplev1.ExampleListItem{
		ExampleId:      ex.ID.Bytes(),
		Name:           ex.Name,
		LastResponseId: lastResp,
	}
}

func DeserializeRPCToModel(ex *examplev1.Example) (mitemapiexample.ItemApiExample, error) {
	if ex == nil {
		return mitemapiexample.ItemApiExample{}, nil
	}

	id, err := idwrap.NewFromBytes(ex.GetExampleId())
	if err != nil {
		return mitemapiexample.ItemApiExample{}, err
	}

	bodyType, err := protoBodyKindToModel(ex.GetBodyKind())
	if err != nil {
		return mitemapiexample.ItemApiExample{}, err
	}

	return mitemapiexample.ItemApiExample{
		ID:       id,
		BodyType: bodyType,
		Name:     ex.Name,
	}, nil
}
