package tcollection

import (
	"context"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mcollection"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/translate/titemnest"
	collectionv1 "dev-tools-services/gen/collection/v1"
	itemfolderv1 "dev-tools-services/gen/itemfolder/v1"
)

type CollectionTranslateService struct {
	folders  []mitemfolder.ItemFolder
	apis     []mitemapi.ItemApi
	examples []mitemapiexample.ItemApiExample
}

func New(folders []mitemfolder.ItemFolder, apis []mitemapi.ItemApi, examples []mitemapiexample.ItemApiExample) *CollectionTranslateService {
	return &CollectionTranslateService{
		folders:  folders,
		apis:     apis,
		examples: examples,
	}
}

type Item[I any] func(context.Context, idwrap.IDWrap) ([]I, error)

func NewWithFunc(ctx context.Context, id idwrap.IDWrap, folderFunc Item[mitemfolder.ItemFolder], apiFunc Item[mitemapi.ItemApi], exampleFunc Item[mitemapiexample.ItemApiExample]) (*CollectionTranslateService, error) {
	folders, err := folderFunc(ctx, id)
	if err != nil {
		return nil, err
	}
	apis, err := apiFunc(ctx, id)
	if err != nil {
		return nil, err
	}

	examples, err := exampleFunc(ctx, id)
	if err != nil {
		return nil, err
	}

	return &CollectionTranslateService{
		folders:  folders,
		apis:     apis,
		examples: examples,
	}, nil
}

func (c CollectionTranslateService) SerializeCollectionModelToRPC(collection mcollection.Collection) *collectionv1.CollectionMeta {
	return &collectionv1.CollectionMeta{
		Id:    collection.ID.String(),
		Name:  collection.Name,
		Items: c.GetItems(),
	}
}

func SerializeCollectionRPCtoModel(collection *collectionv1.CollectionMeta) (*mcollection.Collection, error) {
	ID, err := idwrap.NewWithParse(collection.GetId())
	if err != nil {
		return nil, err
	}
	return &mcollection.Collection{
		ID: ID,

		Name: collection.Name,
	}, nil
}

func (c CollectionTranslateService) SerializeCollectionRPCToModel(collection mcollection.Collection) *collectionv1.CollectionMeta {
	return &collectionv1.CollectionMeta{
		Name: collection.Name,
	}
}

func (c CollectionTranslateService) GetItems() []*itemfolderv1.ItemMeta {
	a, _ := titemnest.TranslateItemFolderNested(c.folders, c.apis, c.examples)
	return a.GetItemsMeta()
}
