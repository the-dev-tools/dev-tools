//nolint:revive // exported
package expression

import (
	"maps"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/secretresolver"
)

// UnifiedEnv provides a unified interface for variable resolution, expression evaluation,
// and string interpolation. It operates on hierarchical (non-flattened) data.
type UnifiedEnv struct {
	data           map[string]any                  // Hierarchical data (not flattened)
	tracker        *tracking.VariableTracker       // Optional tracking
	customFuncs    map[string]any                  // Custom expr-lang functions
	secretResolver secretresolver.SecretResolver   // Optional: cloud secret resolution
}

// NewUnifiedEnv creates a new UnifiedEnv with the given hierarchical data.
func NewUnifiedEnv(data map[string]any) *UnifiedEnv {
	if data == nil {
		data = make(map[string]any)
	}
	return &UnifiedEnv{
		data:        data,
		customFuncs: make(map[string]any),
	}
}

// WithTracking returns a copy of the UnifiedEnv with tracking enabled.
// Variable reads will be recorded in the provided tracker.
func (e *UnifiedEnv) WithTracking(t *tracking.VariableTracker) *UnifiedEnv {
	clone := e.Clone()
	clone.tracker = t
	return clone
}

// WithFunc returns a copy of the UnifiedEnv with a custom function added.
// Custom functions are available in expr-lang expressions.
func (e *UnifiedEnv) WithFunc(name string, fn any) *UnifiedEnv {
	clone := e.Clone()
	clone.customFuncs[name] = fn
	return clone
}

// WithSecretResolver returns a copy of the UnifiedEnv with a secret resolver
// for cloud secret references (#gcp:, #aws:, #azure:).
func (e *UnifiedEnv) WithSecretResolver(r secretresolver.SecretResolver) *UnifiedEnv {
	clone := e.Clone()
	clone.secretResolver = r
	return clone
}

// Clone creates a deep copy of the UnifiedEnv.
func (e *UnifiedEnv) Clone() *UnifiedEnv {
	if e == nil {
		return NewUnifiedEnv(nil)
	}

	newData := make(map[string]any, len(e.data))
	maps.Copy(newData, e.data)

	newFuncs := make(map[string]any, len(e.customFuncs))
	maps.Copy(newFuncs, e.customFuncs)

	return &UnifiedEnv{
		data:           newData,
		tracker:        e.tracker,        // Share tracker reference
		customFuncs:    newFuncs,
		secretResolver: e.secretResolver, // Share resolver reference
	}
}

// GetData returns the underlying hierarchical data map.
func (e *UnifiedEnv) GetData() map[string]any {
	if e == nil {
		return make(map[string]any)
	}
	return e.data
}

// GetTracker returns the variable tracker (may be nil).
func (e *UnifiedEnv) GetTracker() *tracking.VariableTracker {
	if e == nil {
		return nil
	}
	return e.tracker
}

// Get retrieves a value at the given path and optionally tracks the read.
// The path can use dot notation (e.g., "node.response.body") and array indexing
// (e.g., "items[0].id").
func (e *UnifiedEnv) Get(path string) (any, bool) {
	if e == nil {
		return nil, false
	}

	value, ok := ResolvePath(e.data, path)
	if ok && e.tracker != nil {
		e.tracker.TrackRead(path, value)
	}
	return value, ok
}

// Set sets a value at the given path, creating intermediate maps as needed.
func (e *UnifiedEnv) Set(path string, value any) error {
	if e == nil {
		return nil
	}

	err := SetPath(e.data, path, value)
	if err != nil {
		return err
	}

	if e.tracker != nil {
		e.tracker.TrackWrite(path, value)
	}
	return nil
}

// Has returns true if a value exists at the given path.
func (e *UnifiedEnv) Has(path string) bool {
	_, ok := e.Get(path)
	return ok
}

// Merge combines another UnifiedEnv's data into this one.
// Values from other take precedence in case of conflicts.
func (e *UnifiedEnv) Merge(other *UnifiedEnv) *UnifiedEnv {
	if other == nil {
		return e.Clone()
	}

	clone := e.Clone()
	for k, v := range other.data {
		clone.data[k] = v
	}
	return clone
}
