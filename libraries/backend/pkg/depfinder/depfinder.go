package depfinder

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"the-dev-tools/backend/pkg/idwrap"
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
	d.vars[value] = couple
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
			case string, float64, bool, nil:
				d.AddVar(val, VarCouple{Path: newPath, NodeID: couple.NodeID})
			}

			d.addJsonValue(val, VarCouple{Path: newPath, NodeID: couple.NodeID})
		}
	case []any:
		for i, val := range v {
			newPath := fmt.Sprintf("%s[%d]", couple.Path, i)

			// Only add primitive values to the vars map
			switch val.(type) {
			case string, float64, bool, nil:
				d.AddVar(val, VarCouple{Path: newPath, NodeID: couple.NodeID})
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

func (d DepFinder) TemplateJSON(jsonBytes []byte) ([]byte, error) {
	data := make(map[string]any)
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, err
	}

	// Process the JSON structure
	templated := d.ReplaceWithPaths(data)

	// Marshal back to JSON
	return json.Marshal(templated)
}

func (d DepFinder) ReplaceWithPaths(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			result[key] = d.ReplaceWithPaths(val)
		}
		return result

	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = d.ReplaceWithPaths(val)
		}
		return result

	case string, float64, bool, nil:
		// Check if this value exists in our vars map
		if couple, err := d.FindVar(v); err == nil {
			return fmt.Sprintf("{{ %s }}", couple.Path)
		}
	}

	// Return unchanged if not a recognized type or not found
	return value
}
