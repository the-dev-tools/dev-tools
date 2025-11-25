package node

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/tracking"
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

// LoopCoordinator marks nodes that orchestrate loop execution.
type LoopCoordinator interface {
	IsLoopCoordinator() bool
}

type FlowNodeRequest struct {
	VarMap           map[string]any
	ReadWriteLock    *sync.RWMutex
	NodeMap          map[idwrap.IDWrap]FlowNode
	EdgeSourceMap    edge.EdgesMap
	Timeout          time.Duration
	LogPushFunc      LogPushFunc
	PendingAtmoicMap map[idwrap.IDWrap]uint32
	VariableTracker  *tracking.VariableTracker // Optional tracking for input/output data
	IterationContext *runner.IterationContext  // For hierarchical execution naming in loops
	ExecutionID      idwrap.IDWrap             // Unique ID for this specific execution of the node
	Logger           *slog.Logger              // Optional structured logger for node diagnostics
}

type LogPushFunc func(status runner.FlowNodeStatus)

type FlowNodeResult struct {
	NextNodeID []idwrap.IDWrap
	Err        error
	// SkipFinalStatus tells the runner not to create a final execution status.
	// Used by FOR/FOREACH nodes that handle their own iteration status logging.
	SkipFinalStatus bool
	AuxiliaryID     *idwrap.IDWrap
}

var (
	ErrVarGroupNotFound error = errors.New("group not found")
	ErrVarNodeNotFound  error = errors.New("node not found")
	ErrVarKeyNotFound   error = errors.New("key not found")
)

// DeepCopyVarMap creates a deep copy of the VarMap to prevent concurrent access issues
func DeepCopyVarMap(req *FlowNodeRequest) map[string]any {
	req.ReadWriteLock.RLock()
	defer req.ReadWriteLock.RUnlock()

	return deepCopyMap(req.VarMap)
}

// deepCopyMap recursively copies a map[string]any
func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = DeepCopyValue(v)
	}
	return result
}

// DeepCopyValue creates a deep copy of any value
func DeepCopyValue(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = DeepCopyValue(item)
		}
		return result
	case []map[string]interface{}:
		result := make([]map[string]interface{}, len(val))
		for i, item := range val {
			if mapCopy, ok := DeepCopyValue(item).(map[string]interface{}); ok {
				result[i] = mapCopy
			}
		}
		return result
	default:
		// Primitive types (string, int, float, bool, etc.) are copied by value
		// This includes string, int, float, bool, time.Time, etc.
		return val
	}
}

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

// CloneIterationLabels returns a defensive copy of iteration labels to avoid slice aliasing.
func CloneIterationLabels(labels []runner.IterationLabel) []runner.IterationLabel {
	if len(labels) == 0 {
		return nil
	}
	copyLabels := make([]runner.IterationLabel, len(labels))
	copy(copyLabels, labels)
	return copyLabels
}

// FilterLoopEntryNodes removes loop targets that are reachable from other loop
// targets, ensuring we only return the true entry nodes for a loop body. This
// prevents downstream nodes from being re-executed when the loop handle fan-out
// includes both the body head and interior nodes (can happen after noop pruning).
//
// If filtering removes every target (e.g. due to a cycle), we fall back to the
// original slice so execution can still proceed.
func FilterLoopEntryNodes(edgeMap edge.EdgesMap, loopTargets []idwrap.IDWrap) []idwrap.IDWrap {
	if len(loopTargets) < 2 {
		return loopTargets
	}

	filtered := make([]idwrap.IDWrap, 0, len(loopTargets))
	for _, candidate := range loopTargets {
		skip := false
		for _, other := range loopTargets {
			if other == candidate {
				continue
			}
			if edge.IsNodeCheckTarget(edgeMap, other, candidate) == edge.NodeBefore {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, candidate)
		}
	}

	if len(filtered) == 0 {
		return loopTargets
	}

	return filtered
}

// BuildLoopExecutionEdgeMap returns an edge map suitable for executing loop
// bodies. It rewrites the loop handle to include only the provided entry
// targets so duplicate edges to downstream nodes do not participate in
// scheduling decisions.
//
// When the requested targets already match the existing loop edges, the
// original map is returned to avoid unnecessary allocations.
func BuildLoopExecutionEdgeMap(edgeMap edge.EdgesMap, loopNodeID idwrap.IDWrap, loopTargets []idwrap.IDWrap) edge.EdgesMap {
	if len(loopTargets) == 0 {
		return edgeMap
	}

	loopHandles, ok := edgeMap[loopNodeID]
	if ok {
		if current, ok := loopHandles[edge.HandleLoop]; ok && equalIDSlice(current, loopTargets) {
			return edgeMap
		}
	}

	cloned := make(edge.EdgesMap, len(edgeMap))
	for sourceID, handles := range edgeMap {
		handleMap := make(map[edge.EdgeHandle][]idwrap.IDWrap, len(handles))
		for handle, targets := range handles {
			if sourceID == loopNodeID && handle == edge.HandleLoop {
				handleMap[handle] = append([]idwrap.IDWrap(nil), loopTargets...)
				continue
			}
			handleMap[handle] = append([]idwrap.IDWrap(nil), targets...)
		}
		cloned[sourceID] = handleMap
	}

	if _, exists := cloned[loopNodeID]; !exists {
		cloned[loopNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleLoop: append([]idwrap.IDWrap(nil), loopTargets...),
		}
	} else if _, ok := cloned[loopNodeID][edge.HandleLoop]; !ok {
		cloned[loopNodeID][edge.HandleLoop] = append([]idwrap.IDWrap(nil), loopTargets...)
	}

	return cloned
}

// BuildPendingMap constructs a PendingAtmoicMap compatible with the runner by
// counting predecessors for each node. Only entries with more than one
// predecessor are retained to match runner expectations.
func BuildPendingMap(predecessors map[idwrap.IDWrap][]idwrap.IDWrap) map[idwrap.IDWrap]uint32 {
	if len(predecessors) == 0 {
		return nil
	}

	pending := make(map[idwrap.IDWrap]uint32)
	for nodeID, preds := range predecessors {
		if len(preds) > 1 {
			pending[nodeID] = uint32(len(preds))
		}
	}

	if len(pending) == 0 {
		return nil
	}

	return pending
}

// ClonePendingMap makes a shallow copy of a PendingAtmoicMap. It returns nil
// when the source is empty to keep downstream checks simple.
func ClonePendingMap(src map[idwrap.IDWrap]uint32) map[idwrap.IDWrap]uint32 {
	if len(src) == 0 {
		return nil
	}
	clone := make(map[idwrap.IDWrap]uint32, len(src))
	for k, v := range src {
		clone[k] = v
	}
	return clone
}

func equalIDSlice(a, b []idwrap.IDWrap) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ReadVarRaw(a *FlowNodeRequest, key string) (interface{}, error) {
	a.ReadWriteLock.RLock()
	v, ok := a.VarMap[key]
	a.ReadWriteLock.RUnlock()

	if !ok {
		return nil, ErrVarKeyNotFound
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

	return v, nil
}

// WriteNodeVarWithTracking writes a node variable with optional tracking
func WriteNodeVarWithTracking(a *FlowNodeRequest, name string, key string, v interface{}, tracker *tracking.VariableTracker) error {
	// First perform the regular write
	err := WriteNodeVar(a, name, key, v)
	if err != nil {
		return err
	}

	// Track the write if tracker is provided
	if tracker != nil {
		nodeKey := name
		fullKey := nodeKey + "." + key
		tracker.TrackWrite(fullKey, v)
	}

	return nil
}

// WriteNodeVarRawWithTracking writes a raw node variable with optional tracking
func WriteNodeVarRawWithTracking(a *FlowNodeRequest, name string, v interface{}, tracker *tracking.VariableTracker) error {
	// First perform the regular write
	err := WriteNodeVarRaw(a, name, v)
	if err != nil {
		return err
	}

	// Track the write if tracker is provided
	if tracker != nil {
		tracker.TrackWrite(name, v)
	}

	return nil
}

// WriteNodeVarBulkWithTracking writes bulk node variables with optional tracking
func WriteNodeVarBulkWithTracking(a *FlowNodeRequest, name string, v map[string]interface{}, tracker *tracking.VariableTracker) error {
	// First perform the regular write
	err := WriteNodeVarBulk(a, name, v)
	if err != nil {
		return err
	}

	// Track each write if tracker is provided
	if tracker != nil {
		nodeKey := name
		for key, value := range v {
			fullKey := nodeKey + "." + key
			tracker.TrackWrite(fullKey, value)
		}
	}

	return nil
}

// ReadVarRawWithTracking reads a raw variable with optional tracking
func ReadVarRawWithTracking(a *FlowNodeRequest, key string, tracker *tracking.VariableTracker) (interface{}, error) {
	// First perform the regular read
	v, err := ReadVarRaw(a, key)
	if err != nil {
		return nil, err
	}

	// Track the read if tracker is provided
	if tracker != nil {
		tracker.TrackRead(key, v)
	}

	return v, nil
}

// ReadNodeVarWithTracking reads a node variable with optional tracking
func ReadNodeVarWithTracking(a *FlowNodeRequest, name, key string, tracker *tracking.VariableTracker) (interface{}, error) {
	// First perform the regular read
	v, err := ReadNodeVar(a, name, key)
	if err != nil {
		return nil, err
	}

	// Track the read if tracker is provided
	if tracker != nil {
		nodeKey := name
		fullKey := nodeKey + "." + key
		tracker.TrackRead(fullKey, v)
	}

	return v, nil
}
