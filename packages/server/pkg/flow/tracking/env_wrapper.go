//nolint:revive // exported
package tracking

// TrackingEnv wraps an environment map to track variable access
type TrackingEnv struct {
	originalEnv map[string]any
	tracker     *VariableTracker
}

// NewTrackingEnv creates a new tracking environment wrapper
func NewTrackingEnv(env map[string]any, tracker *VariableTracker) *TrackingEnv {
	return &TrackingEnv{
		originalEnv: env,
		tracker:     tracker,
	}
}

// Get retrieves a value from the environment and tracks the read
func (te *TrackingEnv) Get(key string) (any, bool) {
	if te.originalEnv == nil {
		return nil, false
	}

	value, exists := te.originalEnv[key]
	if exists && te.tracker != nil {
		te.tracker.TrackRead(key, value)
	}

	return value, exists
}

// GetMap returns the underlying map for use with expr.Compile
// This is needed for expression compilation but doesn't track access
func (te *TrackingEnv) GetMap() map[string]any {
	if te.originalEnv == nil {
		return make(map[string]any)
	}
	return te.originalEnv
}

// TrackAllVariables tracks all variables in the environment as potentially accessed
// This is called for expression evaluation since we can't track individual variable access
func (te *TrackingEnv) TrackAllVariables() {
	if te.tracker == nil || te.originalEnv == nil {
		return
	}

	for key, value := range te.originalEnv {
		te.tracker.TrackRead(key, value)
	}
}
