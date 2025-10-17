package tbreadcrumbs

import (
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplebreadcrumb"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/translate/tcollection"
	"the-dev-tools/server/pkg/translate/tfolder"
	"the-dev-tools/server/pkg/translate/titemapi"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
)

var (
	modelToProtoBreadcrumbKind = map[mexamplebreadcrumb.ExampleBreadcrumbKind]examplev1.ExampleBreadcrumbKind{
		mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_UNSPECIFIED: examplev1.ExampleBreadcrumbKind_EXAMPLE_BREADCRUMB_KIND_UNSPECIFIED,
		mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_COLLECTION:  examplev1.ExampleBreadcrumbKind_EXAMPLE_BREADCRUMB_KIND_COLLECTION,
		mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_FOLDER:      examplev1.ExampleBreadcrumbKind_EXAMPLE_BREADCRUMB_KIND_FOLDER,
		mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_ENDPOINT:    examplev1.ExampleBreadcrumbKind_EXAMPLE_BREADCRUMB_KIND_ENDPOINT,
	}

	protoToModelBreadcrumbKind = func() map[examplev1.ExampleBreadcrumbKind]mexamplebreadcrumb.ExampleBreadcrumbKind {
		inverse := make(map[examplev1.ExampleBreadcrumbKind]mexamplebreadcrumb.ExampleBreadcrumbKind, len(modelToProtoBreadcrumbKind))
		for model, proto := range modelToProtoBreadcrumbKind {
			if proto == fallbackExampleBreadcrumbKind() {
				continue
			}
			inverse[proto] = model
		}
		return inverse
	}()
)

func SerializeModelToRPC(breadCrumb mexamplebreadcrumb.ExampleBreadcrumb) *examplev1.ExampleBreadcrumb {
	kind := fallbackExampleBreadcrumbKind()
	if converted, err := modelBreadcrumbKindToProto(breadCrumb.Kind); err == nil {
		kind = converted
	}

	rpcBreadcrumb := &examplev1.ExampleBreadcrumb{
		Kind: kind,
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

func DeserializeRPCToModel(breadcrumb *examplev1.ExampleBreadcrumb) (mexamplebreadcrumb.ExampleBreadcrumb, error) {
	if breadcrumb == nil {
		return mexamplebreadcrumb.ExampleBreadcrumb{}, nil
	}

	kind, err := protoBreadcrumbKindToModel(breadcrumb.GetKind())
	if err != nil {
		return mexamplebreadcrumb.ExampleBreadcrumb{}, err
	}

	model := mexamplebreadcrumb.ExampleBreadcrumb{Kind: kind}

	switch kind {
	case mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_COLLECTION:
		collection := breadcrumb.GetCollection()
		if collection == nil {
			return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("collection breadcrumb missing payload")
		}

		converted, convErr := tcollection.SerializeCollectionRPCtoModel(collection)
		if convErr != nil {
			return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("convert breadcrumb collection: %w", convErr)
		}
		model.Collection = converted
	case mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_FOLDER:
		folder := breadcrumb.GetFolder()
		if folder == nil {
			return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("folder breadcrumb missing payload")
		}

		folderID, convErr := idwrap.NewFromBytes(folder.GetFolderId())
		if convErr != nil {
			return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("convert breadcrumb folder id: %w", convErr)
		}

		var parentID *idwrap.IDWrap
		if parentBytes := folder.GetParentFolderId(); len(parentBytes) > 0 {
			p, parentErr := idwrap.NewFromBytes(parentBytes)
			if parentErr != nil {
				return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("convert breadcrumb folder parent id: %w", parentErr)
			}
			parentID = &p
		}

		model.Folder = &mitemfolder.ItemFolder{
			ID:       folderID,
			ParentID: parentID,
			Name:     folder.GetName(),
		}
	case mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_ENDPOINT:
		endpoint := breadcrumb.GetEndpoint()
		if endpoint == nil {
			return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("endpoint breadcrumb missing payload")
		}

		endpointID, convErr := idwrap.NewFromBytes(endpoint.GetEndpointId())
		if convErr != nil {
			return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("convert breadcrumb endpoint id: %w", convErr)
		}

		var parentID *idwrap.IDWrap
		if parentBytes := endpoint.GetParentFolderId(); len(parentBytes) > 0 {
			p, parentErr := idwrap.NewFromBytes(parentBytes)
			if parentErr != nil {
				return mexamplebreadcrumb.ExampleBreadcrumb{}, fmt.Errorf("convert breadcrumb endpoint parent id: %w", parentErr)
			}
			parentID = &p
		}

		model.Endpoint = &mitemapi.ItemApi{
			ID:       endpointID,
			FolderID: parentID,
			Name:     endpoint.GetName(),
			Method:   endpoint.GetMethod(),
			Hidden:   endpoint.GetHidden(),
		}
	}

	return model, nil
}

func modelBreadcrumbKindToProto(kind mexamplebreadcrumb.ExampleBreadcrumbKind) (examplev1.ExampleBreadcrumbKind, error) {
	if proto, ok := modelToProtoBreadcrumbKind[kind]; ok {
		return proto, nil
	}

	return fallbackExampleBreadcrumbKind(), fmt.Errorf("unknown example breadcrumb kind %d", kind)
}

func protoBreadcrumbKindToModel(kind examplev1.ExampleBreadcrumbKind) (mexamplebreadcrumb.ExampleBreadcrumbKind, error) {
	if kind == fallbackExampleBreadcrumbKind() {
		return 0, fmt.Errorf("example breadcrumb kind cannot be unspecified")
	}

	if model, ok := protoToModelBreadcrumbKind[kind]; ok {
		return model, nil
	}

	return 0, fmt.Errorf("unknown example breadcrumb kind enum %v", kind)
}

// fallbackExampleBreadcrumbKind returns the enum value emitted when a model kind cannot be converted.
func fallbackExampleBreadcrumbKind() examplev1.ExampleBreadcrumbKind {
	return examplev1.ExampleBreadcrumbKind_EXAMPLE_BREADCRUMB_KIND_UNSPECIFIED
}
