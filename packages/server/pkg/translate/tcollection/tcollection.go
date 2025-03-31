package tcollection

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"
)

func SerializeCollectionModelToRPC(collection mcollection.Collection) *collectionv1.CollectionListItem {
	return &collectionv1.CollectionListItem{
		CollectionId: collection.ID.Bytes(),
		Name:         collection.Name,
	}
}

func SerializeCollectionRPCtoModel(collection *collectionv1.CollectionListItem) (*mcollection.Collection, error) {
	ID, err := idwrap.NewFromBytes(collection.GetCollectionId())
	if err != nil {
		return nil, err
	}
	return &mcollection.Collection{
		ID:   ID,
		Name: collection.Name,
	}, nil
}
