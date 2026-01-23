//nolint:revive // exported
package expression

import (
	"fmt"
	"strconv"
	"strings"
)

// ResolvePath looks up a value in nested maps using dot notation and array indexing.
// Supports paths like:
//   - "name" (simple key)
//   - "node.response.body" (nested path)
//   - "items[0]" (array index)
//   - "items[0].id" (array index with nested path)
//   - "nodes[0].response.headers.Content-Type" (mixed)
//
// For backwards compatibility with flat key maps (e.g., from varsystem), this function
// first checks if the path exists as a direct key in the map before attempting path resolution.
func ResolvePath(data map[string]any, path string) (any, bool) {
	if data == nil || path == "" {
		return nil, false
	}

	path = strings.TrimSpace(path)

	// Backwards compatibility: check if the path exists as a flat key first
	// This supports legacy varsystem patterns like {"response.userId": "123"}
	if val, exists := data[path]; exists {
		return val, true
	}

	segments := parsePath(path)
	if len(segments) == 0 {
		return nil, false
	}

	var current any = data
	for _, seg := range segments {
		switch s := seg.(type) {
		case keySegment:
			m, ok := current.(map[string]any)
			if !ok {
				return nil, false
			}
			val, exists := m[s.key]
			if !exists {
				return nil, false
			}
			current = val

		case indexSegment:
			switch arr := current.(type) {
			case []any:
				if s.index < 0 || s.index >= len(arr) {
					return nil, false
				}
				current = arr[s.index]
			case []map[string]any:
				if s.index < 0 || s.index >= len(arr) {
					return nil, false
				}
				current = arr[s.index]
			default:
				return nil, false
			}
		}
	}

	return current, true
}

// SetPath sets a value at a dotted path, creating intermediate maps as needed.
// Array indices must reference existing arrays - this function won't create arrays.
func SetPath(data map[string]any, path string, value any) error {
	if data == nil {
		return fmt.Errorf("cannot set path on nil map")
	}
	if path == "" {
		return fmt.Errorf("empty path")
	}

	path = strings.TrimSpace(path)
	segments := parsePath(path)
	if len(segments) == 0 {
		return fmt.Errorf("invalid path: %s", path)
	}

	var current any = data
	for i, seg := range segments[:len(segments)-1] {
		switch s := seg.(type) {
		case keySegment:
			m, ok := current.(map[string]any)
			if !ok {
				return fmt.Errorf("expected map at segment %d, got %T", i, current)
			}

			val, exists := m[s.key]
			if !exists {
				// Create intermediate map
				newMap := make(map[string]any)
				m[s.key] = newMap
				current = newMap
			} else {
				current = val
			}

		case indexSegment:
			switch arr := current.(type) {
			case []any:
				if s.index < 0 || s.index >= len(arr) {
					return fmt.Errorf("index %d out of bounds at segment %d", s.index, i)
				}
				current = arr[s.index]
			case []map[string]any:
				if s.index < 0 || s.index >= len(arr) {
					return fmt.Errorf("index %d out of bounds at segment %d", s.index, i)
				}
				current = arr[s.index]
			default:
				return fmt.Errorf("expected array at segment %d, got %T", i, current)
			}
		}
	}

	// Set the final value
	lastSeg := segments[len(segments)-1]
	switch s := lastSeg.(type) {
	case keySegment:
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("expected map at final segment, got %T", current)
		}
		m[s.key] = value

	case indexSegment:
		arr, ok := current.([]any)
		if !ok {
			return fmt.Errorf("expected array at final segment, got %T", current)
		}
		if s.index < 0 || s.index >= len(arr) {
			return fmt.Errorf("index %d out of bounds", s.index)
		}
		arr[s.index] = value
	}

	return nil
}

// pathSegment represents a single part of a path.
type pathSegment interface {
	isPathSegment()
}

type keySegment struct {
	key string
}

func (keySegment) isPathSegment() {}

type indexSegment struct {
	index int
}

func (indexSegment) isPathSegment() {}

// parsePath parses a path string into segments.
// Examples:
//
//	"name" -> [key("name")]
//	"node.response" -> [key("node"), key("response")]
//	"items[0]" -> [key("items"), index(0)]
//	"items[0].id" -> [key("items"), index(0), key("id")]
func parsePath(path string) []pathSegment {
	if path == "" {
		return nil
	}

	var segments []pathSegment
	current := strings.Builder{}

	flushKey := func() {
		if current.Len() > 0 {
			segments = append(segments, keySegment{key: current.String()})
			current.Reset()
		}
	}

	i := 0
	for i < len(path) {
		ch := path[i]

		switch ch {
		case '.':
			flushKey()
			i++

		case '[':
			flushKey()
			// Find closing bracket
			closeIdx := strings.Index(path[i:], "]")
			if closeIdx == -1 {
				// Invalid path, treat rest as key
				current.WriteString(path[i:])
				i = len(path)
				break
			}

			indexStr := path[i+1 : i+closeIdx]
			idx, err := strconv.Atoi(indexStr)
			if err != nil {
				// Invalid index, treat as key
				current.WriteByte(ch)
				i++
				break
			}

			segments = append(segments, indexSegment{index: idx})
			i += closeIdx + 1

		case ']':
			// Unexpected closing bracket, skip
			i++

		default:
			current.WriteByte(ch)
			i++
		}
	}

	flushKey()
	return segments
}

// FormatPath formats path segments back into a string.
func FormatPath(segments []pathSegment) string {
	if len(segments) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, seg := range segments {
		switch s := seg.(type) {
		case keySegment:
			if i > 0 {
				// Check if previous was not an index (add dot)
				if _, wasIndex := segments[i-1].(indexSegment); !wasIndex {
					sb.WriteByte('.')
				}
			}
			sb.WriteString(s.key)
		case indexSegment:
			sb.WriteString(fmt.Sprintf("[%d]", s.index))
		}
	}
	return sb.String()
}
