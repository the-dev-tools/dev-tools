package texampleversion

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
)

// TODO: collection/item/example/v1 protobuf package doesn't exist. Stub types provided.

type ExampleVersionListItem struct {
	ExampleId      []byte
	Name           string
	LastResponseId []byte
}

func ModelToRPC(example mitemapiexample.ItemApiExample, responseID *idwrap.IDWrap) *ExampleVersionListItem {
	var responseIDBytes []byte
	if responseID != nil {
		responseIDBytes = responseID.Bytes()
	}

	return &ExampleVersionListItem{
		ExampleId:      example.ID.Bytes(),
		Name:           example.Name,
		LastResponseId: responseIDBytes,
	}
}

func RPCListToModel(items []*ExampleVersionListItem) ([]mitemapiexample.ItemApiExample, error) {
	if len(items) == 0 {
		return []mitemapiexample.ItemApiExample{}, nil
	}

	result := make([]mitemapiexample.ItemApiExample, 0, len(items))
	for _, item := range items {
		id, err := idwrap.NewFromBytes(item.ExampleId)
		if err != nil {
			return nil, err
		}

		result = append(result, mitemapiexample.ItemApiExample{
			ID:   id,
			Name: item.Name,
		})
	}
	return result, nil
}