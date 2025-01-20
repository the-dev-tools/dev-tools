package node

import (
	"context"
	"errors"
	"sync"
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
	ReadWriteLock *sync.RWMutex
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

func AddVar(a *FlowNodeRequest, v interface{}, nodeID idwrap.IDWrap, key string) error {
	a.ReadWriteLock.Lock()
	defer a.ReadWriteLock.Unlock()

	oldV, ok := a.VarMap[key]
	if !ok {
		oldV = map[string]interface{}{}
	}

	mapV, ok := oldV.(map[string]interface{})
	if !ok {
		return errors.New("value is not a map")
	}

	mapV[key] = v
	a.VarMap[key] = mapV
	return nil
}

func ReadVar(a *FlowNodeRequest, key string) (interface{}, error) {
	a.ReadWriteLock.RLock()
	defer a.ReadWriteLock.RUnlock()

	v, ok := a.VarMap[key]
	if !ok {
		return nil, errors.New("key not found")
	}

	return v, nil
}
