//nolint:revive // exported
package varsystem

import (
	"fmt"
	"maps"
	"os"
	"reflect"
	"strings"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var (
	ErrKeyNotFound = fmt.Errorf("key not found")
	ErrInvalidKey  = fmt.Errorf("invalid key")
)

type VarMap map[string]mvar.Var

func NewVarMap(vars []mvar.Var) VarMap {
	varMap := make(VarMap)
	for _, v := range vars {
		varMap[v.VarKey] = v
	}
	return varMap
}

func NewVarMapWithPrefix(vars []mvar.Var, prefix string) VarMap {
	varMap := make(VarMap)
	for _, v := range vars {
		varMap[prefix+v.VarKey] = v
	}
	return varMap
}

func NewVarMapFromAnyMap(anyMap map[string]any) VarMap {
	vars := make([]mvar.Var, 0)
	for k, v := range anyMap {
		HelperNewAny(&vars, v, k)
	}
	return NewVarMap(vars)
}

// MergeVarMap merges two var maps
// it creates a new var map and does not modify the original var maps
func MergeVarMap(varMap1, varMap2 VarMap) VarMap {
	varMap := make(VarMap)
	maps.Copy(varMap, varMap1)
	maps.Copy(varMap, varMap2)

	return varMap
}

// should convert
// map[string]any{"something": map[string]any{"something": 1}} -> key: "something.something", value: 1
// []int{1} -> key: "1", value: 1

func HelperNewAny(vars *[]mvar.Var, target any, prefix string) {
	prefix = strings.TrimSpace(prefix)
	if target == nil {
		return
	}
	reflectType := reflect.TypeOf(target)
	switch reflectType.Kind() {
	case reflect.Map:
		val := reflect.ValueOf(target)
		if val.Kind() == reflect.Map {
			for _, key := range val.MapKeys() {
				// Convert key to string for the variable name
				keyStr := fmt.Sprintf("%v", key.Interface())
				value := val.MapIndex(key).Interface()
				HelperNewAny(vars, value, prefix+"."+keyStr)
			}
		}
	case reflect.Slice:
		val := reflect.ValueOf(target)
		if val.Kind() == reflect.Slice {
			for i := range val.Len() {
				HelperNewAny(vars, val.Index(i).Interface(), fmt.Sprintf("%s[%d]", prefix, i))
			}
		}
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64, reflect.Bool:
		*vars = append(*vars, mvar.Var{
			VarKey: prefix,
			Value:  fmt.Sprintf("%v", target),
		})
	case reflect.String:
		*vars = append(*vars, mvar.Var{
			VarKey: prefix,
			Value:  reflect.ValueOf(target).String(),
		})
	}
}

func (vm VarMap) ToSlice() []mvar.Var {
	return tgeneric.MapToSlice(vm)
}

func (vm VarMap) Get(varKey string) (mvar.Var, bool) {
	varKey = strings.TrimSpace(varKey)

	// Check if this is a file reference
	if IsFileReference(varKey) {
		fileContent, err := ReadFileContentAsString(varKey)
		if err != nil {
			return mvar.Var{}, false
		}
		return mvar.Var{
			VarKey: varKey,
			Value:  fileContent,
		}, true
	}

	val, ok := vm[varKey]
	if !ok {
		return mvar.Var{}, false
	}
	return val, true
}

// Helper functions
func MergeVars(global, current []mvar.Var) []mvar.Var {
	globalMap := make(map[string]mvar.Var, len(global))
	for _, globalVar := range global {
		globalMap[globalVar.VarKey] = globalVar
	}

	for _, currentVar := range current {
		globalMap[currentVar.VarKey] = currentVar
	}

	return tgeneric.MapToSlice(globalMap)
}

func FilterVars(vars []mvar.Var, filter func(mvar.Var) bool) []mvar.Var {
	filtered := make([]mvar.Var, 0, len(vars))
	for _, v := range vars {
		if filter(v) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// {{varKey}}
func GetVarKeyFromRaw(raw string) string {
	return raw[mvar.PrefixSize : len(raw)-mvar.SuffixSize]
}

func CheckIsVar(varKey string) bool {
	varKey = strings.TrimSpace(varKey)
	varKey = strings.ToLower(varKey)
	return CheckPrefix(varKey) && CheckSuffix(varKey)
}

func CheckPrefix(varKey string) bool {
	return len(varKey) >= mvar.PrefixSize && varKey[:mvar.PrefixSize] == mvar.Prefix
}

func CheckSuffix(varKey string) bool {
	return len(varKey) >= mvar.SuffixSize && varKey[len(varKey)-mvar.SuffixSize:] == mvar.Suffix
}

func CheckStringHasAnyVarKey(raw string) bool {
	return strings.Contains(raw, mvar.Prefix) && strings.Contains(raw, mvar.Suffix)
}

// IsFileReference checks if a variable key refers to a file (starts with "file:")
func IsFileReference(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), "#file:")
}

// IsEnvReference checks if a variable key refers to an environment variable (starts with "#env:")
func IsEnvReference(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), "#env:")
}

// VarMapTracker wraps a VarMap and tracks variable reads
type VarMapTracker struct {
	VarMap   VarMap
	ReadVars map[string]string // stores variable key -> resolved value
}

// NewVarMapTracker creates a new tracking wrapper around a VarMap
func NewVarMapTracker(varMap VarMap) *VarMapTracker {
	return &VarMapTracker{
		VarMap:   varMap,
		ReadVars: make(map[string]string),
	}
}

// Get tracks variable access and delegates to the underlying VarMap
func (vmt *VarMapTracker) Get(varKey string) (mvar.Var, bool) {
	val, ok := vmt.VarMap.Get(varKey)
	if ok {
		// Track this variable read
		trimmedKey := strings.TrimSpace(varKey)
		vmt.ReadVars[trimmedKey] = val.Value
	}
	return val, ok
}

// ReplaceVars tracks all variable reads during replacement and delegates to underlying VarMap
func (vmt *VarMapTracker) ReplaceVars(raw string) (string, error) {
	var result string
	for {
		startIndex := strings.Index(raw, mvar.Prefix)
		if startIndex == -1 {
			result += raw
			break
		}

		endIndex := strings.Index(raw[startIndex:], mvar.Suffix)
		if endIndex == -1 {
			return "", ErrInvalidKey
		}

		rawVar := raw[startIndex : startIndex+endIndex+mvar.SuffixSize]
		if !CheckIsVar(rawVar) {
			return "", ErrInvalidKey
		}

		// Check if key is present in the map
		key := strings.TrimSpace(GetVarKeyFromRaw(rawVar))

		// Check if this is a file reference
		switch {
		case IsFileReference(key):
			fileContent, err := ReadFileContentAsString(key)
			if err != nil {
				return "", err
			}
			// Track file reference read
			vmt.ReadVars[key] = fileContent
			result += raw[:startIndex] + fileContent
		case IsEnvReference(key):
			envValue, err := ReadEnvValueAsString(key)
			if err != nil {
				return "", err
			}
			vmt.ReadVars[key] = envValue
			result += raw[:startIndex] + envValue
		default:
			val, ok := vmt.VarMap.Get(key)
			if !ok {
				return "", fmt.Errorf("%s %w", key, ErrKeyNotFound)
			}
			// Track variable read
			value, err := resolveIndirectValue(val.Value)
			if err != nil {
				return "", err
			}
			vmt.ReadVars[key] = value
			result += raw[:startIndex] + value
		}

		raw = raw[startIndex+len(rawVar):]
	}

	return result, nil
}

// GetReadVars returns a copy of all tracked variable reads
func (vmt *VarMapTracker) GetReadVars() map[string]string {
	result := make(map[string]string, len(vmt.ReadVars))
	for k, v := range vmt.ReadVars {
		result[k] = v
	}
	return result
}

// ReadFileContentAsString reads the content of a file at the given path
func ReadFileContentAsString(filePath string) (string, error) {
	data, err := os.ReadFile(strings.TrimPrefix(strings.TrimSpace(filePath), "#file:"))
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(data), nil
}

func GetIsFileReferencePath(filePath string) string {
	path := strings.TrimPrefix(strings.TrimSpace(filePath), "#file:")
	return path
}

// ReadEnvValueAsString resolves a #env: reference to its environment value.
func ReadEnvValueAsString(ref string) (string, error) {
	name := strings.TrimPrefix(strings.TrimSpace(ref), "#env:")
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("invalid environment reference")
	}
	if value, ok := os.LookupEnv(name); ok {
		return value, nil
	}
	return "", fmt.Errorf("environment variable %s: %w", name, ErrKeyNotFound)
}

// Get {{ url }}/api/{{ version }}/path or {{url}}/api/{{version}}/path
// returns google.com/api/v1/path
func (vm VarMap) ReplaceVars(raw string) (string, error) {
	var result string
	for {
		startIndex := strings.Index(raw, mvar.Prefix)
		if startIndex == -1 {
			result += raw
			break
		}

		endIndex := strings.Index(raw[startIndex:], mvar.Suffix)
		if endIndex == -1 {
			return "", ErrInvalidKey
		}

		rawVar := raw[startIndex : startIndex+endIndex+mvar.SuffixSize]
		if !CheckIsVar(rawVar) {
			return "", ErrInvalidKey
		}

		// Check if key is present in the map
		key := GetVarKeyFromRaw(rawVar)

		// Check if this is a file reference
		switch {
		case IsFileReference(key):
			fileContent, err := ReadFileContentAsString(key)
			if err != nil {
				return "", err
			}
			result += raw[:startIndex] + fileContent
		case IsEnvReference(key):
			envValue, err := ReadEnvValueAsString(key)
			if err != nil {
				return "", err
			}
			result += raw[:startIndex] + envValue
		default:
			val, ok := vm.Get(key)
			if !ok {
				return "", fmt.Errorf("%s %w", key, ErrKeyNotFound)
			}
			value, err := resolveIndirectValue(val.Value)
			if err != nil {
				return "", err
			}
			result += raw[:startIndex] + value
		}

		raw = raw[startIndex+len(rawVar):]
	}

	return result, nil
}

func resolveIndirectValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if IsEnvReference(trimmed) {
		return ReadEnvValueAsString(trimmed)
	}
	return value, nil
}
