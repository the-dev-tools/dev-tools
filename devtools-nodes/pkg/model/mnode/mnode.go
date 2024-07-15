package mnode

import "devtools-nodes/pkg/model/medge"

type Node struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
	OwnerID string      `json:"ownerID"`
	GroupID string      `json:"groupID"`
	Edges   medge.Edges `json:"edges"`
}
