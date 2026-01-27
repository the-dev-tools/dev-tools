//nolint:revive // exported
package expression

import "fmt"

// =============================================================================
// Built-in AI Expression Function (method on UnifiedEnv)
// =============================================================================
//
// The ai() function resolves a variable with metadata hints for AI.
// It behaves like {{ varName }} but includes description and type metadata.
//
// Usage: ai("varName", "description", "type")
// Returns: value if exists, error if not found

// helperAI returns the value of varName if it exists, otherwise returns an error.
// The description and varType parameters are metadata hints for AI tooling.
func (e *UnifiedEnv) helperAI(name, description, varType string) (any, error) {
	if name == "" {
		return nil, fmt.Errorf("ai: variable name is required")
	}

	if value, ok := e.Get(name); ok {
		return value, nil
	}

	return nil, fmt.Errorf("ai: variable %q not found", name)
}
