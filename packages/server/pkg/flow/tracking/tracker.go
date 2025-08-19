package tracking

import (
	"sync"
)

// VariableTracker tracks variable reads and writes during node execution
type VariableTracker struct {
	readVars    map[string]any
	writtenVars map[string]any
	mutex       sync.RWMutex
}

// NewVariableTracker creates a new variable tracker instance
func NewVariableTracker() *VariableTracker {
	return &VariableTracker{
		readVars:    make(map[string]any),
		writtenVars: make(map[string]any),
	}
}

// TrackRead records a variable read operation
func (vt *VariableTracker) TrackRead(key string, value any) {
	if vt == nil {
		return
	}

	vt.mutex.Lock()
	defer vt.mutex.Unlock()
	vt.readVars[key] = deepCopy(value)
}

// TrackWrite records a variable write operation
func (vt *VariableTracker) TrackWrite(key string, value any) {
	if vt == nil {
		return
	}

	vt.mutex.Lock()
	defer vt.mutex.Unlock()
	vt.writtenVars[key] = deepCopy(value)
}

// GetReadVars returns a copy of all tracked read variables
func (vt *VariableTracker) GetReadVars() map[string]any {
	if vt == nil {
		return make(map[string]any)
	}

	vt.mutex.RLock()
	defer vt.mutex.RUnlock()

	result := make(map[string]any, len(vt.readVars))
	for k, v := range vt.readVars {
		result[k] = deepCopy(v)
	}
	return result
}

// GetReadVarsAsTree returns read variables as a nested tree structure
func (vt *VariableTracker) GetReadVarsAsTree() map[string]any {
	flatVars := vt.GetReadVars()
	return BuildTree(flatVars)
}

// GetWrittenVars returns a copy of all tracked written variables
func (vt *VariableTracker) GetWrittenVars() map[string]any {
	if vt == nil {
		return make(map[string]any)
	}

	vt.mutex.RLock()
	defer vt.mutex.RUnlock()

	result := make(map[string]any, len(vt.writtenVars))
	for k, v := range vt.writtenVars {
		result[k] = deepCopy(v)
	}
	return result
}

// GetWrittenVarsAsTree returns written variables as a nested tree structure
func (vt *VariableTracker) GetWrittenVarsAsTree() map[string]any {
	flatVars := vt.GetWrittenVars()
	return BuildTree(flatVars)
}

// deepCopy creates a deep copy of the value to prevent external modifications
func deepCopy(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = deepCopy(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
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
		// Also handles map[string]interface{} and []interface{} through any
		return v
	}
}
