package mnodemaster

import (
	"context"
	"devtools-nodes/pkg/httpclient"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mstatus"
	"errors"
	"log"
)

type NodeMaster struct {
	Ctx             context.Context
	ID              string                 `json:"id"`
	StartNodeID     string                 `json:"startNodeID"`
	Nodes           map[string]mnode.Node  `json:"nodes"`
	Vars            map[string]interface{} `json:"vars"`
	CurrentNode     *mnode.Node            `json:"currentNode"`
	NextNodeID      string                 `json:"nextNode"`
	StopNodeID      string
	Resolver        Resolver
	ExecuteNodeFunc ExcuteNodeFunc
	Logger          *log.Logger
	StateChan       chan mstatus.StatusUpdateData
	HttpClient      httpclient.HttpClient
}

type (
	Resolver       func(nodeType string) (func(*NodeMaster) error, error)
	ExcuteNodeFunc func(context.Context, *NodeMaster, Resolver) error
)

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
