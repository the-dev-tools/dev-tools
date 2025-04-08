package texample

import (
	"strings"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
)

func SerializeModelToRPC(ex mitemapiexample.ItemApiExample, lastResp *idwrap.IDWrap, exampleBreadcrumbs mitemapiexample.ExampleBreadcrumbs) *examplev1.Example {
	// Split folder path into an array of folder names

	breadcrumbs := []string{exampleBreadcrumbs.CollectionName}

	// Add folder path elements if they exist
	if exampleBreadcrumbs.FolderPath != nil {
		folderPathStrArr := strings.Split(*exampleBreadcrumbs.FolderPath, "/")
		if len(folderPathStrArr) > 0 && folderPathStrArr[0] != "" {
			breadcrumbs = append(breadcrumbs, folderPathStrArr...)
		}
	}

	// Add API name and example name
	breadcrumbs = append(breadcrumbs, exampleBreadcrumbs.ApiName)

	var lastResponseBytes []byte
	if lastResp != nil {
		lastResponseBytes = lastResp.Bytes()
	}

	return &examplev1.Example{
		ExampleId:      ex.ID.Bytes(),
		Name:           ex.Name,
		BodyKind:       bodyv1.BodyKind(ex.BodyType),
		LastResponseId: lastResponseBytes,
		Breadcrumbs:    breadcrumbs,
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
