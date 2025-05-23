package texampleversion

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
)

func ModelToRPC(example mitemapiexample.ItemApiExample, responseID *idwrap.IDWrap) *examplev1.ExampleVersionsItem {
	var responseIDBytes []byte
	if responseID != nil {
		responseIDBytes = responseID.Bytes()
	}
	return &examplev1.ExampleVersionsItem{
		ExampleId:      example.ID.Bytes(),
		LastResponseId: responseIDBytes,
	}
}
