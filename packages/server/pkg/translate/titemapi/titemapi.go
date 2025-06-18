package titemapi

import (
	"errors"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
	endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
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
	if len(parentIDBytes) > 0 {
		tempParentID, err := idwrap.NewFromBytes(parentIDBytes)
		if err != nil {
			return nil, err
		}
		parentID = &tempParentID
	}

	var hidden bool
	if item.Hidden != nil {
		hidden = *item.Hidden
	}

	return &mitemapi.ItemApi{
		CollectionID: collectionID,
		FolderID:     parentID,
		Url:          item.GetUrl(),
		Name:         item.GetName(),
		Method:       item.GetMethod(),
		Hidden:       hidden,
	}, nil
}

func DeseralizeModelToRPC(item *mitemapi.ItemApi) *endpointv1.Endpoint {
	var parentID []byte = nil
	if item.FolderID != nil {
		parentID = item.FolderID.Bytes()
	}
	return &endpointv1.Endpoint{
		EndpointId:     item.ID.Bytes(),
		ParentFolderId: parentID,
		Name:           item.Name,
		Method:         item.Method,
		Url:            item.Url,
		Hidden:         &item.Hidden,
	}
}

func SeralizeModelToRPCItem(item *mitemapi.ItemApi) *endpointv1.EndpointListItem {
	var parentID []byte = nil
	if item.FolderID != nil {
		parentID = item.FolderID.Bytes()
	}
	return &endpointv1.EndpointListItem{
		EndpointId:     item.ID.Bytes(),
		ParentFolderId: parentID,
		Name:           item.Name,
		Method:         item.Method,
		Hidden:         &item.Hidden,
	}
}
