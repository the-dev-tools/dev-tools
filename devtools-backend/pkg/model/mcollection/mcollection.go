package mcollection

import (
	"devtools-nodes/pkg/model/mnodedata"

	"github.com/oklog/ulid/v2"
)

const (
	CollectionNodeTypeRequest = "request"
	CollectionNodeTypeFolder  = "folder"
)

type Collection struct {
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
	Type         string
	ParentID     string
	Data         *mnodedata.NodeApiRestData
}

func NewCollectionNode(id ulid.ULID, collectionID ulid.ULID, name string, nodeType string, parentID string, data *mnodedata.NodeApiRestData) *CollectionNode {
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
