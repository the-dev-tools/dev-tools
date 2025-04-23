package varsystem

import (
	"fmt"
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
		castedMap, ok := target.(map[string]any)
		if !ok {
			return
		}
		for k, v := range castedMap {
			HelperNewAny(vars, v, prefix+"."+k)
		}
	case reflect.Slice:
		for i, v := range target.([]any) {
			HelperNewAny(vars, v, fmt.Sprintf("%s[%d]", prefix, i))
		}
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64, reflect.Bool:
		*vars = append(*vars, mvar.Var{
			VarKey: prefix,
			Value:  fmt.Sprintf("%v", target),
		})
	case reflect.String:
		*vars = append(*vars, mvar.Var{
			VarKey: prefix,
			Value:  target.(string),
		})
	}
}

func (vm VarMap) ToSlice() []mvar.Var {
	return tgeneric.MapToSlice(vm)
}

func (vm VarMap) Get(varKey string) (mvar.Var, bool) {
	val, ok := vm[strings.TrimSpace(varKey)]
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
		val, ok := vm.Get(key)
		if !ok {
			return "", fmt.Errorf("%s %v", key, ErrKeyNotFound)
		}

		result += raw[:startIndex] + val.Value
		raw = raw[startIndex+len(rawVar):]
	}

	return result, nil
}
