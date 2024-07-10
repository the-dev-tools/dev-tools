package mnodemaster

import (
	"errors"

	"github.com/DevToolsGit/devtools-nodes/pkg/httpclient"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnode"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mstatus"
)

type NodeMaster struct {
	ID          string                 `json:"id"`
	StartNodeID string                 `json:"startNodeID"`
	Nodes       map[string]mnode.Node  `json:"nodes"`
	Vars        map[string]interface{} `json:"vars"`
	CurrentNode *mnode.Node            `json:"currentNode"`
	NextNodeID  string                 `json:"nextNode"`
	Resolver    func(nodeType string) (func(*NodeMaster) error, error)
	StateChan   chan mstatus.StatusUpdateData
	HttpClient  httpclient.HttpClient
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
