package mstatus

const (
	StatusTypeSetVar   = "setVar"
	StatusTypeNextNode = "nextNode"
)

type StatusUpdateData struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type StatusDataSetVar struct {
	Key string      `json:"key"`
	Val interface{} `json:"val"`
}

type StatusDataNextNode struct {
	NodeID string `json:"nodeID"`
}
