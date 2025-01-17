package node

import (
	"context"
	"errors"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/idwrap"
	"time"
)

var ErrNodeNotFound = errors.New("node not found")

type NodeStatus int8

const (
	NodeNone NodeStatus = iota
	NodeStarting
	NodeStatusRunning
	NodeStatusSuccess
	NodeStatusFailed
)

func (n NodeStatus) String() string {
	return [...]string{"None", "Starting", "Running", "Success", "Failed"}[n]
}

type FlowNode interface {
	GetID() idwrap.IDWrap

	// TODO: will implement streaming in the future
	RunSync(ctx context.Context, req *FlowNodeRequest) FlowNodeResult
	RunAsync(ctx context.Context, req *FlowNodeRequest, resultChan chan FlowNodeResult)
}

type FlowNodeRequest struct {
	VarMap        map[string]interface{}
	NodeMap       map[idwrap.IDWrap]FlowNode
	EdgeSourceMap edge.EdgesMap
	Timeout       time.Duration
	LogPushFunc   LogPushFunc
}

type LogPushFunc func(status NodeStatus, id idwrap.IDWrap)

type FlowNodeResult struct {
	NextNodeID []idwrap.IDWrap
	Err        error
}
