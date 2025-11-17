package titemapi

import (
	"errors"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
)

// Endpoint represents an API endpoint.
// TODO: Replace with actual protobuf type when available
type Endpoint struct {
	EndpointId     []byte `protobuf:"bytes,1,opt,name=endpoint_id,json=endpointId,proto3" json:"endpoint_id,omitempty"`
	ParentFolderId []byte `protobuf:"bytes,2,opt,name=parent_folder_id,json=parentFolderId,proto3" json:"parent_folder_id,omitempty"`
	Name           string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Method         string `protobuf:"bytes,4,opt,name=method,proto3" json:"method,omitempty"`
	Url            string `protobuf:"bytes,5,opt,name=url,proto3" json:"url,omitempty"`
	Hidden         *bool  `protobuf:"varint,6,opt,name=hidden,proto3" json:"hidden,omitempty"`
}

// EndpointListItem represents an endpoint in a list.
// TODO: Replace with actual protobuf type when available
type EndpointListItem struct {
	EndpointId     []byte `protobuf:"bytes,1,opt,name=endpoint_id,json=endpointId,proto3" json:"endpoint_id,omitempty"`
	ParentFolderId []byte `protobuf:"bytes,2,opt,name=parent_folder_id,json=parentFolderId,proto3" json:"parent_folder_id,omitempty"`
	Name           string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Method         string `protobuf:"bytes,4,opt,name=method,proto3" json:"method,omitempty"`
	Hidden         *bool  `protobuf:"varint,5,opt,name=hidden,proto3" json:"hidden,omitempty"`
}

// GetEndpointId returns the endpoint ID
func (e *Endpoint) GetEndpointId() []byte {
	if e != nil {
		return e.EndpointId
	}
	return nil
}

// GetParentFolderId returns the parent folder ID
func (e *Endpoint) GetParentFolderId() []byte {
	if e != nil {
		return e.ParentFolderId
	}
	return nil
}

// GetName returns the endpoint name
func (e *Endpoint) GetName() string {
	if e != nil {
		return e.Name
	}
	return ""
}

// GetMethod returns the endpoint method
func (e *Endpoint) GetMethod() string {
	if e != nil {
		return e.Method
	}
	return ""
}

// GetUrl returns the endpoint URL
func (e *Endpoint) GetUrl() string {
	if e != nil {
		return e.Url
	}
	return ""
}

// GetHidden returns the hidden flag
func (e *Endpoint) GetHidden() *bool {
	if e != nil {
		return e.Hidden
	}
	return nil
}

func SeralizeRPCToModel(item *Endpoint, collectionID idwrap.IDWrap) (*mitemapi.ItemApi, error) {
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

func SeralizeRPCToModelWithoutID(item *Endpoint, collectionID idwrap.IDWrap) (*mitemapi.ItemApi, error) {
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

func DeseralizeModelToRPC(item *mitemapi.ItemApi) *Endpoint {
	var parentID []byte = nil
	if item.FolderID != nil {
		parentID = item.FolderID.Bytes()
	}
	return &Endpoint{
		EndpointId:     item.ID.Bytes(),
		ParentFolderId: parentID,
		Name:           item.Name,
		Method:         item.Method,
		Url:            item.Url,
		Hidden:         &item.Hidden,
	}
}

func SeralizeModelToRPCItem(item *mitemapi.ItemApi) *EndpointListItem {
	var parentID []byte = nil
	if item.FolderID != nil {
		parentID = item.FolderID.Bytes()
	}
	return &EndpointListItem{
		EndpointId:     item.ID.Bytes(),
		ParentFolderId: parentID,
		Name:           item.Name,
		Method:         item.Method,
		Hidden:         &item.Hidden,
	}
}
