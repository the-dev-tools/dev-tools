//nolint:revive // exported
package expression

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// =============================================================================
// Built-in AI Expression Function (method on UnifiedEnv)
// =============================================================================
//
// The ai() function resolves a variable with metadata hints for AI.
// It behaves like {{ varName }} but includes description and type metadata.
//
// Usage: ai("varName", "description", "type")
// Returns: value if exists, error if not found

// helperUUID generates a new UUID string. Defaults to v4.
// Usage in expressions: uuid() or uuid("v4") or uuid("v7")
func helperUUID(args ...string) (string, error) {
	version := "v4"
	if len(args) > 0 {
		version = args[0]
	}

	switch version {
	case "v4":
		return uuid.New().String(), nil
	case "v7":
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("uuid: failed to generate v7: %w", err)
		}
		return id.String(), nil
	default:
		return "", fmt.Errorf("uuid: unsupported version %q, use \"v4\" or \"v7\"", version)
	}
}

// helperULID generates a new ULID string.
// Usage in expressions: ulid()
func helperULID() string {
	return ulid.Make().String()
}

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
