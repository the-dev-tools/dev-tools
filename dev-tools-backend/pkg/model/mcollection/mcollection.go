package mcollection

import (
	"dev-tools-nodes/pkg/model/mnodedata"
	"time"

	"github.com/oklog/ulid/v2"
)

const (
	CollectionNodeTypeUnspecified int32 = 0
	CollectionNodeTypeRequest     int32 = 1
	CollectionNodeTypeFolder      int32 = 2
)

type Collection struct {
	ID      ulid.ULID
	OwnerID ulid.ULID
	Name    string
	Updated time.Time
}

func (c Collection) GetCreatedTime() time.Time {
	return time.UnixMilli(int64(c.ID.Time()))
}

type MetaCollection struct {
	ID   ulid.ULID
	Name string
}

func (mc MetaCollection) GetCreatedTime() time.Time {
	return time.UnixMilli(int64(mc.ID.Time()))
}

type CollectionNode struct {
	ID           ulid.ULID
	CollectionID ulid.ULID
	Name         string
	Type         int32
	ParentID     string
	Data         *mnodedata.NodeApiRestData
}

func NewCollectionNode(id ulid.ULID, collectionID ulid.ULID, name, parentID string, nodeType int32, data *mnodedata.NodeApiRestData) *CollectionNode {
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
	ulidID := ulid.Make()
	collectionID := ulid.Make()

	var str string
	return &CollectionNode{
		ID:           ulidID,
		CollectionID: collectionID,
		ParentID:     str,
		Data:         &mnodedata.NodeApiRestData{},
	}
}
