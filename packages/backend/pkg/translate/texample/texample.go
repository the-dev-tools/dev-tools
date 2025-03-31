package texample

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
)

func SerializeModelToRPC(ex mitemapiexample.ItemApiExample, lastResp idwrap.IDWrap) *examplev1.Example {
	return &examplev1.Example{
		ExampleId:      ex.ID.Bytes(),
		Name:           ex.Name,
		BodyKind:       bodyv1.BodyKind(ex.BodyType),
		LastResponseId: lastResp.Bytes(),
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

	return mitemapiexample.ItemApiExample{
		ID:       id,
		BodyType: mitemapiexample.BodyType(ex.BodyKind),
		Name:     ex.Name,
	}, nil
}
