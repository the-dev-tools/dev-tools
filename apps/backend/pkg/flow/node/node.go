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

var (
	ErrVarNodeNotFound error = errors.New("node not found")
	ErrVarKeyNotFound  error = errors.New("key not found")
)

func AddNodeVar(a *FlowNodeRequest, v interface{}, nodeID idwrap.IDWrap, key string) error {
	a.ReadWriteLock.Lock()
	defer a.ReadWriteLock.Unlock()

	nodeStr := nodeID.String()

	oldV, ok := a.VarMap[nodeID.String()]
	if !ok {
		oldV = map[string]interface{}{}
	}

	mapV, ok := oldV.(map[string]interface{})
	if !ok {
		return errors.New("value is not a map")
	}

	mapV[key] = v
	a.VarMap[nodeStr] = mapV
	return nil
}

func ReadVarRaw(a *FlowNodeRequest, key string) (interface{}, error) {
	a.ReadWriteLock.RLock()
	defer a.ReadWriteLock.RUnlock()

	v, ok := a.VarMap[key]
	if !ok {
		return nil, ErrVarKeyNotFound
	}

	return v, nil
}

func ReadNodeVar(a *FlowNodeRequest, id idwrap.IDWrap, key string) (interface{}, error) {
	a.ReadWriteLock.RLock()
	defer a.ReadWriteLock.RUnlock()

	nodeVarMap, ok := a.VarMap[id.String()]
	if !ok {
		return nil, ErrVarNodeNotFound
	}

	castedNodeVarMap, ok := nodeVarMap.(map[string]interface{})
	if !ok {
		return nil, errors.New("value is not a map")
	}

	v, ok := castedNodeVarMap[key]
	if !ok {
		return nil, ErrVarKeyNotFound
	}

	return v, nil
}
