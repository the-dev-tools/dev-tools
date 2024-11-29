package mstatus

const (
	StatusTypeSetVar   = "setVar"
	StatusTypeNextNode = "nextNode"
	StatusTypeDone     = "done"
)

type NodeStatus struct {
	NodeID string
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
}

type NodeStatusSetVar struct {
	Key string      `json:"key"`
	Val interface{} `json:"val"`
}

type NodeStatusNextNode struct {
	NodeID string `json:"nodeID"`
}
