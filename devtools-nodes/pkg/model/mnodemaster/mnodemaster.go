package mnodemaster

import (
	"errors"

	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnode"
)

type NodeMaster struct {
	ID          string                 `json:"id"`
	StartNodeID string                 `json:"startNodeID"`
	Nodes       map[string]mnode.Node  `json:"nodes"`
	Vars        map[string]interface{} `json:"vars"`
	CurrentNode *mnode.Node            `json:"currentNode"`
	NextNodeID  string                 `json:"nextNode"`
}

var (
	ErrNodeNotFound    = errors.New("node not found")
	ErrInvalidDataType = errors.New("invalid data type")
)

const (
	ApiCallRest  = "apiCallRest"
	IFStatusCode = "ifStatusCode"
)

const (
	EdgeSuccess = "success"
	EdgeFailure = "failure"
)
