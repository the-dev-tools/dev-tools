package tracking

import (
	"strings"
)

// BuildTree converts flat key-value pairs with dot notation into a nested tree structure
// Example: {"a.b.c": "value"} becomes {"a": {"b": {"c": "value"}}}
func BuildTree(flatMap map[string]any) map[string]any {
	if len(flatMap) == 0 {
		return make(map[string]any)
	}
	
	result := make(map[string]any)
	
	for key, value := range flatMap {
		setNestedValue(result, key, value)
	}
	
	return result
}

// setNestedValue sets a value in a nested map structure using dot notation
func setNestedValue(target map[string]any, path string, value any) {
	// Handle edge case of empty path
	if path == "" {
		return
	}
	
	// Remove leading/trailing spaces from path
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	
	// Split the path by dots
	parts := strings.Split(path, ".")
	
	// Navigate/create the nested structure
	current := target
	for i, part := range parts {
		// Remove leading/trailing spaces from each part
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// If this is the last part, set the value
		if i == len(parts)-1 {
			current[part] = deepCopyValue(value)
			return
		}
		
		// Create or navigate to the next level
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}
		
		// Type assertion to continue navigation
		if nextLevel, ok := current[part].(map[string]any); ok {
			current = nextLevel
		} else {
			// If the current value is not a map, we can't navigate further
			// This could happen if there's a conflict in the tree structure
			// For now, just overwrite with a new map
			current[part] = make(map[string]any)
			current = current[part].(map[string]any)
		}
	}
}

// deepCopyValue creates a deep copy of a value to prevent external modifications
func deepCopyValue(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = deepCopyValue(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = deepCopyValue(v)
		}
		return result
	case []map[string]interface{}:
		result := make([]map[string]interface{}, len(val))
		for i, v := range val {
			if mapCopy, ok := deepCopyValue(v).(map[string]interface{}); ok {
				result[i] = mapCopy
			}
		}
		return result
	default:
		// For primitive types and other types, return as is
		// This includes string, int, float, bool, etc.
		// Note: map[string]interface{} and []interface{} are handled by map[string]any and []any
		return v
	}
}

// MergeTreesPreferFirst merges two tree structures, preferring values from the first tree when conflicts occur
func MergeTreesPreferFirst(first, second map[string]any) map[string]any {
	if len(first) == 0 {
		return deepCopyTree(second)
	}
	if len(second) == 0 {
		return deepCopyTree(first)
	}
	
	result := deepCopyTree(first)
	
	for key, value := range second {
		if _, exists := result[key]; !exists {
			result[key] = deepCopyValue(value)
		} else {
			// If both are maps, recursively merge
			if firstMap, ok := result[key].(map[string]any); ok {
				if secondMap, ok := value.(map[string]any); ok {
					result[key] = MergeTreesPreferFirst(firstMap, secondMap)
				}
				// If first is map but second isn't, keep first (prefer first)
			}
			// If first exists and is not a map, keep first (prefer first)
		}
	}
	
	return result
}

// deepCopyTree creates a deep copy of a tree structure
func deepCopyTree(tree map[string]any) map[string]any {
	if tree == nil {
		return make(map[string]any)
	}
	
	result := make(map[string]any, len(tree))
	for k, v := range tree {
		result[k] = deepCopyValue(v)
	}
	return result
}