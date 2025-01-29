package tfolder

import (
	"errors"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mitemfolder"
	folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
)

func SeralizeRPCToModel(item *folderv1.Folder, collectionID idwrap.IDWrap) (*mitemfolder.ItemFolder, error) {
	modelItem, err := SeralizeRPCToModelWithoutID(item, collectionID)
	if err != nil {
		return nil, err
	}
	id, err := idwrap.NewFromBytes(item.GetFolderId())
	if err != nil {
		return nil, err
	}
	modelItem.ID = id
	return modelItem, nil
}

func SeralizeRPCToModelWithoutID(item *folderv1.Folder, collectionID idwrap.IDWrap) (*mitemfolder.ItemFolder, error) {
	var parentID *idwrap.IDWrap
	if item == nil {
		return nil, errors.New("folder is nil")
	}

	parentIDBytes := item.GetParentFolderId()
	if parentIDBytes != nil {
		tempParentID, err := idwrap.NewFromBytes(parentIDBytes)
		if err != nil {
			return nil, err
		}
		parentID = &tempParentID
	}

	return &mitemfolder.ItemFolder{
		CollectionID: collectionID,
		ParentID:     parentID,
		Name:         item.Name,
		Prev:         nil,
		Next:         nil,
	}, nil
}

func DeseralizeModelToRPC(item mitemfolder.ItemFolder) *folderv1.Folder {
	var parentIDBytes []byte = nil
	if item.ParentID != nil {
		parentIDBytes = item.ParentID.Bytes()
	}
	return &folderv1.Folder{
		FolderId:       item.ID.Bytes(),
		ParentFolderId: parentIDBytes,
		Name:           item.Name,
	}
}

func SeralizeModelToRPCItem(item mitemfolder.ItemFolder) *folderv1.FolderListItem {
	var parentIDBytes []byte = nil
	if item.ParentID != nil {
		parentIDBytes = item.ParentID.Bytes()
	}
	return &folderv1.FolderListItem{
		FolderId:       item.ID.Bytes(),
		ParentFolderId: parentIDBytes,
		Name:           item.Name,
	}
}
