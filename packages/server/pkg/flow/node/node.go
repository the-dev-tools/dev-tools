package node

import (
	"context"
	"errors"
	"sync"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

var ErrNodeNotFound = errors.New("node not found")

// INFO: this is workaround for expr lang
const NodeVarPrefix = "node"

type FlowNode interface {
	GetID() idwrap.IDWrap
	GetName() string

	// TODO: will implement streaming in the future
	RunSync(ctx context.Context, req *FlowNodeRequest) FlowNodeResult
	RunAsync(ctx context.Context, req *FlowNodeRequest, resultChan chan FlowNodeResult)
}

type FlowNodeRequest struct {
	VarMap           map[string]any
	ReadWriteLock    *sync.RWMutex
	NodeMap          map[idwrap.IDWrap]FlowNode
	EdgeSourceMap    edge.EdgesMap
	Timeout          time.Duration
	LogPushFunc      LogPushFunc
	PendingAtmoicMap map[idwrap.IDWrap]uint32
	// Read tracking fields
	ReadTracker      map[string]any
	ReadTrackerMutex *sync.Mutex
	CurrentNodeID    idwrap.IDWrap
}

type LogPushFunc func(status runner.FlowNodeStatus)

type FlowNodeResult struct {
	NextNodeID []idwrap.IDWrap
	Err        error
}

var (
	ErrVarGroupNotFound error = errors.New("group not found")
	ErrVarNodeNotFound  error = errors.New("node not found")
	ErrVarKeyNotFound   error = errors.New("key not found")
)

func WriteNodeVar(a *FlowNodeRequest, name string, key string, v interface{}) error {
	a.ReadWriteLock.Lock()
	defer a.ReadWriteLock.Unlock()

	nodeKey := name

	oldV, ok := a.VarMap[nodeKey]
	if !ok {
		oldV = map[string]interface{}{}
	}

	mapV, ok := oldV.(map[string]interface{})
	if !ok {
		return errors.New("value is not a map")
	}

	mapV[key] = v
	a.VarMap[nodeKey] = mapV
	return nil
}

func WriteNodeVarRaw(a *FlowNodeRequest, name string, v interface{}) error {
	a.ReadWriteLock.Lock()
	defer a.ReadWriteLock.Unlock()

	nodeKey := name

	a.VarMap[nodeKey] = v
	return nil
}

func WriteNodeVarBulk(a *FlowNodeRequest, name string, v map[string]interface{}) error {
	a.ReadWriteLock.Lock()
	defer a.ReadWriteLock.Unlock()

	nodeKey := name

	oldV, ok := a.VarMap[nodeKey]
	if !ok {
		oldV = map[string]interface{}{}
	}

	mapV, ok := oldV.(map[string]interface{})
	if !ok {
		return errors.New("value is not a map")
	}

	for key, value := range v {
		mapV[key] = value
	}

	a.VarMap[nodeKey] = mapV
	return nil
}

func ReadVarRaw(a *FlowNodeRequest, key string) (interface{}, error) {
	a.ReadWriteLock.RLock()
	v, ok := a.VarMap[key]
	a.ReadWriteLock.RUnlock()

	if !ok {
		return nil, ErrVarKeyNotFound
	}

	// Track the read if tracking is enabled
	if a.ReadTracker != nil && a.ReadTrackerMutex != nil {
		a.ReadTrackerMutex.Lock()
		a.ReadTracker[key] = deepCopy(v)
		a.ReadTrackerMutex.Unlock()
	}

	return v, nil
}

func ReadNodeVar(a *FlowNodeRequest, name, key string) (interface{}, error) {
	a.ReadWriteLock.RLock()
	nodeKey := name
	nodeVarMap, ok := a.VarMap[nodeKey]
	a.ReadWriteLock.RUnlock()

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

	// Track the entire node data if tracking is enabled
	if a.ReadTracker != nil && a.ReadTrackerMutex != nil {
		a.ReadTrackerMutex.Lock()
		a.ReadTracker[nodeKey] = deepCopy(nodeVarMap)
		a.ReadTrackerMutex.Unlock()
	}

	return v, nil
}

// deepCopy creates a deep copy of the value to prevent external modifications
func deepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, v := range val {
			result[k] = deepCopy(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = deepCopy(v)
		}
		return result
	case []map[string]interface{}:
		result := make([]map[string]interface{}, len(val))
		for i, v := range val {
			if mapCopy, ok := deepCopy(v).(map[string]interface{}); ok {
				result[i] = mapCopy
			}
		}
		return result
	default:
		// For primitive types and other types, return as is
		// This includes string, int, float, bool, etc.
		return v
	}
}
