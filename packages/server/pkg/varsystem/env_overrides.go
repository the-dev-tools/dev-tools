package varsystem

import "os"

// EnvOverridesKey is the run variable that carries a run's #env: values. It starts with '#', so
// expressions and {{ }} references cannot read it as a variable.
const EnvOverridesKey = "#env-overrides"

// EnvOverrides are values for #env: references that take precedence over the process
// environment for one run (for example a linked clone's .env). The values are kept out of JSON
// and fmt output, so copying the run's variables (to a JS node, an AI tool, logs) does not leak
// them.
type EnvOverrides struct {
	values map[string]string
}

// NewEnvOverrides copies values into EnvOverrides.
func NewEnvOverrides(values map[string]string) EnvOverrides {
	m := make(map[string]string, len(values))
	for k, v := range values {
		m[k] = v
	}
	return EnvOverrides{values: m}
}

// Lookup returns the override for name.
func (o EnvOverrides) Lookup(name string) (string, bool) {
	v, ok := o.values[name]
	return v, ok
}

// Len returns the number of overrides.
func (o EnvOverrides) Len() int {
	return len(o.values)
}

// String hides the values.
func (o EnvOverrides) String() string {
	return "[env overrides]"
}

// MarshalJSON hides the values.
func (o EnvOverrides) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

// WithEnvOverrides stores overrides in a run's variables. It does nothing when there are none.
func WithEnvOverrides(vars map[string]any, overrides EnvOverrides) {
	if vars == nil || overrides.Len() == 0 {
		return
	}
	vars[EnvOverridesKey] = overrides
}

// EnvOverridesFrom returns the overrides stored in a run's variables.
func EnvOverridesFrom(vars map[string]any) EnvOverrides {
	if o, ok := vars[EnvOverridesKey].(EnvOverrides); ok {
		return o
	}
	return EnvOverrides{}
}

// LookupEnv resolves the value of #env:name: the override first, then the process environment.
func LookupEnv(name string, overrides EnvOverrides) (string, bool) {
	if v, ok := overrides.Lookup(name); ok {
		return v, true
	}
	return os.LookupEnv(name)
}
