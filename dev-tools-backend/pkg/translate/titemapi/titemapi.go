package titemapi

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapi"
	itemapiv1 "dev-tools-services/gen/itemapi/v1"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
)

func SeralizeRPCToModel(item *itemapiv1.ApiCall) (*mitemapi.ItemApi, error) {
	modelItem, err := SeralizeRPCToModelWithoutID(item)
	if err != nil {
		return nil, err
	}
	id, err := idwrap.NewWithParse(item.Meta.GetId())
	if err != nil {
		return nil, err
	}
	modelItem.ID = id
	return modelItem, nil
}

func SeralizeRPCToModelWithoutID(item *itemapiv1.ApiCall) (*mitemapi.ItemApi, error) {
	if item == nil {
		item = &itemapiv1.ApiCall{}
	}
	meta := item.GetMeta()
	if meta == nil {
		meta = &itemapiv1.ApiCallMeta{}
	}

	var parentID *idwrap.IDWrap = nil
	parentIDStr := item.GetParentId()
	if parentIDStr != "" {
		tempParentID, err := idwrap.NewWithParse(parentIDStr)
		if err != nil {
			return nil, err
		}
		parentID = &tempParentID
	}

	collectionID, err := idwrap.NewWithParse(item.GetCollectionId())
	if err != nil {
		return nil, err
	}

	return &mitemapi.ItemApi{
		CollectionID: collectionID,
		ParentID:     parentID,
		Url:          item.GetUrl(),
		Name:         meta.GetName(),
		Method:       meta.GetMethod(),
	}, nil
}

func DeseralizeModelToRPC(item *mitemapi.ItemApi, defaultExampleID idwrap.IDWrap, examples []*itemapiexamplev1.ApiExampleMeta) *itemapiv1.ApiCall {
	var parentID string
	if item.ParentID != nil {
		parentID = item.ParentID.String()
	}
	return &itemapiv1.ApiCall{
		Meta: &itemapiv1.ApiCallMeta{
			Id:       item.ID.String(),
			Name:     item.Name,
			Method:   item.Method,
			Examples: examples,
		},
		CollectionId: item.CollectionID.String(),
		ParentId:     parentID,
		Url:          item.Url,
	}
}
