package secretresolver

import (
	"encoding/json"
	"fmt"
)

// ExtractFragment extracts a JSON field from a value if fragment is non-empty.
// If fragment is empty, returns the raw value as-is.
// If fragment is specified but the value is not valid JSON or the key is missing, returns an error.
func ExtractFragment(value, fragment string) (string, error) {
	if fragment == "" {
		return value, nil
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(value), &obj); err != nil {
		return "", fmt.Errorf("secret value is not valid JSON for fragment extraction: %w", err)
	}

	v, ok := obj[fragment]
	if !ok {
		return "", fmt.Errorf("fragment key %q not found in secret JSON", fragment)
	}

	switch val := v.(type) {
	case string:
		return val, nil
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return "", fmt.Errorf("cannot marshal fragment value: %w", err)
		}
		return string(data), nil
	}
}
