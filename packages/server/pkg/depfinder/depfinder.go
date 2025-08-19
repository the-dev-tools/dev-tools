package depfinder

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"the-dev-tools/server/pkg/idwrap"
)

type VarCouple struct {
	Path   string
	NodeID idwrap.IDWrap
}

type DepFinder struct {
	vars map[any]VarCouple
}

func NewDepFinder() DepFinder {
	return DepFinder{vars: make(map[any]VarCouple)}
}

func (d DepFinder) AddVar(value any, couple VarCouple) {
	if _, exists := d.vars[value]; !exists {
		d.vars[value] = couple
	}
}

func (d DepFinder) AddJsonBytes(value []byte, couple VarCouple) error {
	var data any
	if err := json.Unmarshal(value, &data); err != nil {
		return err
	}
	d.addJsonValue(data, couple)
	return nil
}

var (
	ErrNotFound     = errors.New("variable not found")
	ErrTypeMismatch = errors.New("type mismatch")
)

func (d DepFinder) FindVar(value any) (VarCouple, error) {
	res, ok := d.vars[value]
	var err error = nil
	if !ok {
		err = ErrNotFound
	}
	return res, err
}

func (d DepFinder) addJsonValue(value any, couple VarCouple) {
	switch v := value.(type) {
	case map[string]any:
		for key, val := range v {
			newPath := couple.Path
			if newPath != "" {
				newPath += "."
			}
			newPath += key

			// Only add primitive values to the vars map
			switch val.(type) {
			case string, float64, bool, int, int64:
				d.AddVar(val, VarCouple{Path: newPath, NodeID: couple.NodeID})
				continue
			}

			d.addJsonValue(val, VarCouple{Path: newPath, NodeID: couple.NodeID})
		}
	case []any:
		for i, val := range v {
			newPath := fmt.Sprintf("%s[%d]", couple.Path, i)

			// Only add primitive values to the vars map
			switch val.(type) {
			case string, float64, bool, int, int64:
				d.AddVar(val, VarCouple{Path: newPath, NodeID: couple.NodeID})
				continue
			}

			d.addJsonValue(val, VarCouple{Path: newPath, NodeID: couple.NodeID})
		}
	}
}

func (d DepFinder) FindInJsonBytes(jsonBytes []byte, value interface{}) (string, error) {
	var data interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return "", err
	}

	path, matches := d.findJsonValue(data, "", value)
	if !matches {
		return "", ErrNotFound
	}
	return path, nil
}

func (d DepFinder) findJsonValue(jsonValue interface{}, path string, searchValue interface{}) (string, bool) {
	// Check if current value matches
	if reflect.DeepEqual(jsonValue, searchValue) {
		return path, true
	}

	switch v := jsonValue.(type) {
	case map[string]interface{}:
		for key, val := range v {
			newPath := path
			if path != "" {
				newPath += "."
			}
			newPath += key
			if reflect.DeepEqual(val, searchValue) {
				return newPath, true
			}
			if foundPath, found := d.findJsonValue(val, newPath, searchValue); found {
				return foundPath, true
			}
		}
	case []interface{}:
		for i, val := range v {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if reflect.DeepEqual(val, searchValue) {
				return newPath, true
			}
			if foundPath, found := d.findJsonValue(val, newPath, searchValue); found {
				return foundPath, true
			}
		}
	}

	return "", false
}

type TemplateJSONResult struct {
	FindAny bool
	Couples []VarCouple
	NewJson []byte
	Err     error
}

func (d DepFinder) TemplateJSON(jsonBytes []byte) TemplateJSONResult {
	data := make(map[string]any)
	// unmarshal the json bytes to a map

	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return TemplateJSONResult{Err: err}
	}

	// Process the JSON structure
	templated, findAny, couples := d.ReplaceWithPaths(data)

	// Marshal back to JSON
	jsonBytes, err := json.Marshal(templated)
	return TemplateJSONResult{FindAny: findAny, Couples: couples, NewJson: jsonBytes, Err: err}
}

// replace value with path if the value in vars
func (d DepFinder) ReplaceWithPaths(value any) (any, bool, []VarCouple) {
	return d.replaceWithPaths(value, false) // JSON mode: exact match only
}

// ReplaceWithPathsSubstring allows substring replacement for token templating
func (d DepFinder) ReplaceWithPathsSubstring(value any) (any, bool, []VarCouple) {
	return d.replaceWithPaths(value, true) // Token mode: allow substring replacement
}

func (d DepFinder) replaceWithPaths(value any, allowSubstring bool) (any, bool, []VarCouple) {
	var findAny bool
	var couples []VarCouple
	var couplesSub []VarCouple

	switch v := value.(type) {
	case map[string]any:
		// sort the map to make it deterministic
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := make(map[string]any)
		for _, key := range keys {
			val := v[key]
			result[key], findAny, couplesSub = d.replaceWithPaths(val, allowSubstring)
			couples = append(couples, couplesSub...)
		}
		return result, findAny, couples

	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i], findAny, couplesSub = d.replaceWithPaths(val, allowSubstring)
			couples = append(couples, couplesSub...)
		}
		return result, findAny, couples

	case string:
		// First try exact match
		if couple, err := d.FindVar(v); err == nil {
			return fmt.Sprintf("{{ %s }}", couple.Path), true, []VarCouple{couple}
		}

		// Try partial string replacement for substrings (only if enabled)
		if allowSubstring {
			result := v
			var foundAny bool
			var allCouples []VarCouple

			// Check each known variable to see if it appears as a substring
			for varValue, couple := range d.vars {
				if strValue, ok := varValue.(string); ok && len(strValue) > 0 {
					// Replace all occurrences of this token in the string
					if strings.Contains(result, strValue) {
						template := fmt.Sprintf("{{ %s }}", couple.Path)
						result = strings.ReplaceAll(result, strValue, template)
						foundAny = true
						allCouples = append(allCouples, couple)
					}
				}
			}

			if foundAny {
				return result, true, allCouples
			}
		}

		return v, false, nil

	case int, int64, float64:
		// Handle numeric values
		if couple, err := d.FindVar(v); err == nil {
			return fmt.Sprintf("{{ %s }}", couple.Path), true, []VarCouple{couple}
		}
		return v, false, nil

	case bool:
		// Handle boolean values
		if couple, err := d.FindVar(v); err == nil {
			return fmt.Sprintf("{{ %s }}", couple.Path), true, []VarCouple{couple}
		}
		return v, false, nil

	default:
		return v, false, nil
	}
}

// IsUUID checks if a string matches UUID format (8-4-4-4-12 hex characters)
func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, char := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if char != '-' {
				return false
			}
		} else {
			if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
				return false
			}
		}
	}
	return true
}

// ReplaceURLPathParams detects UUIDs in URL paths and replaces them with templated variables
func (d DepFinder) ReplaceURLPathParams(url string) (string, bool, []VarCouple) {
	var couples []VarCouple
	var foundAny bool

	// Split URL by '/' to get path segments
	parts := strings.Split(url, "/")

	for i, part := range parts {
		// Check if this part looks like a UUID
		if IsUUID(part) {
			// Try to find this UUID in our vars
			if couple, err := d.FindVar(part); err == nil {
				parts[i] = fmt.Sprintf("{{ %s }}", couple.Path)
				couples = append(couples, couple)
				foundAny = true
			}
		}
	}

	if foundAny {
		return strings.Join(parts, "/"), foundAny, couples
	}

	return url, false, nil
}
