package tbreadcrumbs

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplebreadcrumb"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/translate/tcollection"
	"the-dev-tools/server/pkg/translate/tfolder"
	"the-dev-tools/server/pkg/translate/titemapi"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
)

func SerializeModelToRPC(breadCrumb mexamplebreadcrumb.ExampleBreadcrumb) *examplev1.ExampleBreadcrumb {
	// Split folder path into an array of folder names

	rpcBreadcrumb := &examplev1.ExampleBreadcrumb{
		Kind: examplev1.ExampleBreadcrumbKind(breadCrumb.Kind),
	}
	switch breadCrumb.Kind {
	case mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_COLLECTION:
		rpcBreadcrumb.Collection = tcollection.SerializeCollectionModelToRPC(*breadCrumb.Collection)
	case mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_FOLDER:
		rpcBreadcrumb.Folder = tfolder.SeralizeModelToRPCItem(*breadCrumb.Folder)
	case mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_ENDPOINT:
		rpcBreadcrumb.Endpoint = titemapi.SeralizeModelToRPCItem(breadCrumb.Endpoint)
	}

	return rpcBreadcrumb

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
