package mcollection

import (
	"devtools-nodes/pkg/model/mnodedata"

	"github.com/oklog/ulid/v2"
)

const (
	CollectionNodeTypeUnspecified int32 = 0
	CollectionNodeTypeRequest     int32 = 1
	CollectionNodeTypeFolder      int32 = 2
)

type Collection struct {
	ID   ulid.ULID
	Name string
}

type MetaCollection struct {
	ID   ulid.ULID
	Name string
}

/*
   string id = 1;
   string name = 2;
   string type = 3; // such as "request", "folder"
   optional string parent_id = 4; // INFO: optional
   optional nodedata.v1.NodeApiCallData data = 5; // INFO: optional
*/

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
