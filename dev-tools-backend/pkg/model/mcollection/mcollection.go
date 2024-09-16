package mcollection

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-nodes/pkg/model/mnodedata"
	"time"
)

const (
	CollectionNodeTypeUnspecified int32 = 0
	CollectionNodeTypeRequest     int32 = 1
	CollectionNodeTypeFolder      int32 = 2
)

type Collection struct {
	ID      idwrap.IDWrap
	OwnerID idwrap.IDWrap
	Name    string
	Updated time.Time
}

func (c Collection) GetCreatedTime() time.Time {
	return c.ID.Time()
}

func (c Collection) GetCreatedTimeUnix() int64 {
	return idwrap.GetUnixMilliFromULID(c.ID)
}

type MetaCollection struct {
	ID   idwrap.IDWrap
	Name string
}

func (mc MetaCollection) GetCreatedTime() time.Time {
	return mc.ID.Time()
}

type CollectionNode struct {
	ID           idwrap.IDWrap
	CollectionID idwrap.IDWrap
	Name         string
	Type         int32
	ParentID     string
	Data         *mnodedata.NodeApiRestData
}

func NewCollectionNode(id idwrap.IDWrap, collectionID idwrap.IDWrap, name, parentID string, nodeType int32, data *mnodedata.NodeApiRestData) *CollectionNode {
	return &CollectionNode{
		ID:           id,
		CollectionID: collectionID,
		Name:         name,
		Type:         nodeType,
		ParentID:     parentID,
		Data:         data,
	}
}

func NewEmptyCollectionNode() *CollectionNode {
	var str string
	return &CollectionNode{
		ID:           idwrap.NewNow(),
		CollectionID: idwrap.NewNow(),
		ParentID:     str,
		Data:         &mnodedata.NodeApiRestData{},
	}
}
