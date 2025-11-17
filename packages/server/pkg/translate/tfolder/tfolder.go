package tfolder

import (
	"errors"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemfolder"
)

// Folder represents a folder with parent relationship.
// TODO: Replace with actual protobuf type when available
type Folder struct {
	FolderId       []byte `protobuf:"bytes,1,opt,name=folder_id,json=folderId,proto3" json:"folder_id,omitempty"`
	ParentFolderId []byte `protobuf:"bytes,2,opt,name=parent_folder_id,json=parentFolderId,proto3" json:"parent_folder_id,omitempty"`
	Name           string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
}

// FolderListItem represents a folder in a list.
// TODO: Replace with actual protobuf type when available
type FolderListItem struct {
	FolderId       []byte `protobuf:"bytes,1,opt,name=folder_id,json=folderId,proto3" json:"folder_id,omitempty"`
	ParentFolderId []byte `protobuf:"bytes,2,opt,name=parent_folder_id,json=parentFolderId,proto3" json:"parent_folder_id,omitempty"`
	Name           string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
}

// GetFolderId returns the folder ID
func (f *Folder) GetFolderId() []byte {
	if f != nil {
		return f.FolderId
	}
	return nil
}

// GetParentFolderId returns the parent folder ID
func (f *Folder) GetParentFolderId() []byte {
	if f != nil {
		return f.ParentFolderId
	}
	return nil
}

// GetName returns the folder name
func (f *Folder) GetName() string {
	if f != nil {
		return f.Name
	}
	return ""
}

func SeralizeRPCToModel(item *Folder, collectionID idwrap.IDWrap) (*mitemfolder.ItemFolder, error) {
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

func SeralizeRPCToModelWithoutID(item *Folder, collectionID idwrap.IDWrap) (*mitemfolder.ItemFolder, error) {
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

func DeseralizeModelToRPC(item mitemfolder.ItemFolder) *Folder {
	var parentIDBytes []byte = nil
	if item.ParentID != nil {
		parentIDBytes = item.ParentID.Bytes()
	}
	return &Folder{
		FolderId:       item.ID.Bytes(),
		ParentFolderId: parentIDBytes,
		Name:           item.Name,
	}
}

func SeralizeModelToRPCItem(item mitemfolder.ItemFolder) *FolderListItem {
	var parentIDBytes []byte = nil
	if item.ParentID != nil {
		parentIDBytes = item.ParentID.Bytes()
	}
	return &FolderListItem{
		FolderId:       item.ID.Bytes(),
		ParentFolderId: parentIDBytes,
		Name:           item.Name,
	}
}
