package tfolder

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemfolder"
	itemfolderv1 "dev-tools-services/gen/itemfolder/v1"
	"errors"
)

func SeralizeRPCToModel(item *itemfolderv1.Folder) (*mitemfolder.ItemFolder, error) {
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

func SeralizeRPCToModelWithoutID(item *itemfolderv1.Folder) (*mitemfolder.ItemFolder, error) {
	var parentID *idwrap.IDWrap
	if item == nil {
		return nil, errors.New("folder is nil")
	}

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

	meta := item.GetMeta()
	if meta == nil {
		return nil, errors.New("meta is nil")
	}

	return &mitemfolder.ItemFolder{
		CollectionID: collectionID,
		ParentID:     parentID,
		Name:         meta.GetName(),
		Prev:         nil,
		Next:         nil,
	}, nil
}

func DeseralizeModelToRPC(item *mitemapi.ItemApi) *itemfolderv1.Folder {
	var parentID string
	if item.ParentID != nil {
		parentID = item.ParentID.String()
	}
	return &itemfolderv1.Folder{
		Meta: &itemfolderv1.FolderMeta{
			Id:   item.ID.String(),
			Name: item.Name,
		},
		ParentId:     parentID,
		CollectionId: item.CollectionID.String(),
		Items:        []*itemfolderv1.Item{},
	}
}
