package titemapi

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapi"
	endpointv1 "dev-tools-spec/dist/buf/go/collection/item/endpoint/v1"
	"errors"
)

func SeralizeRPCToModel(item *endpointv1.Endpoint, collectionID idwrap.IDWrap) (*mitemapi.ItemApi, error) {
	modelItem, err := SeralizeRPCToModelWithoutID(item, collectionID)
	if err != nil {
		return nil, err
	}
	id, err := idwrap.NewFromBytes(item.GetEndpointId())
	if err != nil {
		return nil, err
	}
	modelItem.ID = id
	return modelItem, nil
}

func SeralizeRPCToModelWithoutID(item *endpointv1.Endpoint, collectionID idwrap.IDWrap) (*mitemapi.ItemApi, error) {
	if item == nil {
		return nil, errors.New("item is nil")
	}
	var parentID *idwrap.IDWrap
	parentIDBytes := item.GetParentFolderId()
	if parentIDBytes != nil && len(parentIDBytes) > 0 {
		tempParentID, err := idwrap.NewFromBytes(parentIDBytes)
		if err != nil {
			return nil, err
		}
		parentID = &tempParentID
	}

	return &mitemapi.ItemApi{
		CollectionID: collectionID,
		ParentID:     parentID,
		Url:          item.GetUrl(),
		Name:         item.GetName(),
		Method:       item.GetMethod(),
	}, nil
}

func DeseralizeModelToRPC(item *mitemapi.ItemApi) *endpointv1.Endpoint {
	var parentID []byte = nil
	if item.ParentID != nil {
		parentID = item.ParentID.Bytes()
	}
	return &endpointv1.Endpoint{
		EndpointId:     item.ID.Bytes(),
		ParentFolderId: parentID,
		Name:           item.Name,
		Method:         item.Method,
		Url:            item.Url,
	}
}
