//nolint:revive // exported
package varsystem

import (
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
	"maps"
	"os"
	"reflect"
	"strings"
)

var (
	ErrKeyNotFound = fmt.Errorf("key not found")
	ErrInvalidKey  = fmt.Errorf("invalid key")
)

type VarMap map[string]menv.Variable

func NewVarMap(vars []menv.Variable) VarMap {
	varMap := make(VarMap)
	for _, v := range vars {
		varMap[v.VarKey] = v
	}
	return varMap
}

func NewVarMapWithPrefix(vars []menv.Variable, prefix string) VarMap {
	varMap := make(VarMap)
	for _, v := range vars {
		varMap[prefix+v.VarKey] = v
	}
	return varMap
}

func NewVarMapFromAnyMap(anyMap map[string]any) VarMap {
	vars := make([]menv.Variable, 0)
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
// map[string]any{"foo": map[string]any{"bar": 1}} -> key: "foo.bar", value: 1
// []int{1} -> key: "1", value: 1

func HelperNewAny(vars *[]menv.Variable, target any, prefix string) {
	prefix = strings.TrimSpace(prefix)
	if target == nil {
		*vars = append(*vars, menv.Variable{
			VarKey: prefix,
			Value:  "",
		})
		return
	}
	reflectType := reflect.TypeOf(target)
	switch reflectType.Kind() {
	case reflect.Map:
		val := reflect.ValueOf(target)
		if val.Kind() == reflect.Map {
			for _, key := range val.MapKeys() {
				if !key.IsValid() {
					continue
				}
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
	case reflect.Ptr:
		val := reflect.ValueOf(target)
		if val.IsNil() {
			*vars = append(*vars, menv.Variable{
				VarKey: prefix,
				Value:  "",
			})
			return
		}
		HelperNewAny(vars, val.Elem().Interface(), prefix)
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64, reflect.Bool:
		*vars = append(*vars, menv.Variable{
			VarKey: prefix,
			Value:  fmt.Sprintf("%v", target),
		})
	case reflect.String:
		*vars = append(*vars, menv.Variable{
			VarKey: prefix,
			Value:  reflect.ValueOf(target).String(),
		})
	}
}

func (vm VarMap) ToSlice() []menv.Variable {
	return tgeneric.MapToSlice(vm)
}

func (vm VarMap) Get(varKey string) (menv.Variable, bool) {
	varKey = strings.TrimSpace(varKey)

	// Check if this is a file reference
	if IsFileReference(varKey) {
		fileContent, err := ReadFileContentAsString(varKey)
		if err != nil {
			return menv.Variable{}, false
		}
		return menv.Variable{
			VarKey: varKey,
			Value:  fileContent,
		}, true
	}

	val, ok := vm[varKey]
	if !ok {
		return menv.Variable{}, false
	}
	return val, true
}

// Helper functions
func MergeVars(global, current []menv.Variable) []menv.Variable {
	globalMap := make(map[string]menv.Variable, len(global))
	for _, globalVar := range global {
		globalMap[globalVar.VarKey] = globalVar
	}

	for _, currentVar := range current {
		globalMap[currentVar.VarKey] = currentVar
	}

	return tgeneric.MapToSlice(globalMap)
}

func FilterVars(vars []menv.Variable, filter func(menv.Variable) bool) []menv.Variable {
	filtered := make([]menv.Variable, 0, len(vars))
	for _, v := range vars {
		if filter(v) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// {{varKey}}
func GetVarKeyFromRaw(raw string) string {
	return raw[menv.PrefixSize : len(raw)-menv.SuffixSize]
}

func CheckIsVar(varKey string) bool {
	varKey = strings.TrimSpace(varKey)
	varKey = strings.ToLower(varKey)
	return CheckPrefix(varKey) && CheckSuffix(varKey)
}

func CheckPrefix(varKey string) bool {
	return len(varKey) >= menv.PrefixSize && varKey[:menv.PrefixSize] == menv.Prefix
}

func CheckSuffix(varKey string) bool {
	return len(varKey) >= menv.SuffixSize && varKey[len(varKey)-menv.SuffixSize:] == menv.Suffix
}

func CheckStringHasAnyVarKey(raw string) bool {
	return strings.Contains(raw, menv.Prefix) && strings.Contains(raw, menv.Suffix)
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
func (vmt *VarMapTracker) Get(varKey string) (menv.Variable, bool) {
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
		startIndex := strings.Index(raw, menv.Prefix)
		if startIndex == -1 {
			result += raw
			break
		}

		endIndex := strings.Index(raw[startIndex:], menv.Suffix)
		if endIndex == -1 {
			return "", ErrInvalidKey
		}

		rawVar := raw[startIndex : startIndex+endIndex+menv.SuffixSize]
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
		startIndex := strings.Index(raw, menv.Prefix)
		if startIndex == -1 {
			result += raw
			break
		}

		endIndex := strings.Index(raw[startIndex:], menv.Suffix)
		if endIndex == -1 {
			return "", ErrInvalidKey
		}

		rawVar := raw[startIndex : startIndex+endIndex+menv.SuffixSize]
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

// ExtractVarKeys extracts all variable keys from a string without resolving them.
// Returns a deduplicated list of variable keys (e.g., "nodeName.field", "userId").
// Skips special references like #env: and #file:.
func ExtractVarKeys(raw string) []string {
	if raw == "" {
		return nil
	}

	seen := make(map[string]bool)
	var result []string
	remaining := raw

	for {
		startIndex := strings.Index(remaining, menv.Prefix)
		if startIndex == -1 {
			break
		}

		endIndex := strings.Index(remaining[startIndex:], menv.Suffix)
		if endIndex == -1 {
			break
		}

		rawVar := remaining[startIndex : startIndex+endIndex+menv.SuffixSize]
		if CheckIsVar(rawVar) {
			key := strings.TrimSpace(GetVarKeyFromRaw(rawVar))
			// Skip special references
			if !IsFileReference(key) && !IsEnvReference(key) && key != "" {
				if !seen[key] {
					seen[key] = true
					result = append(result, key)
				}
			}
		}

		remaining = remaining[startIndex+len(rawVar):]
	}

	return result
}

// ExtractVarKeysFromMultiple extracts variable keys from multiple strings and returns a deduplicated list.
func ExtractVarKeysFromMultiple(strs ...string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, s := range strs {
		keys := ExtractVarKeys(s)
		for _, key := range keys {
			if !seen[key] {
				seen[key] = true
				result = append(result, key)
			}
		}
	}

	return result
}
