package varsystem

import (
	"dev-tools-backend/pkg/model/mvar"
	"dev-tools-backend/pkg/translate/tgeneric"
)

type VarMap map[string]mvar.Var

func NewVarMap(vars []mvar.Var) VarMap {
	varMap := make(VarMap)
	for _, v := range vars {
		varMap[v.VarKey] = v
	}
	return varMap
}

func (vm VarMap) ToSlice() []mvar.Var {
	return tgeneric.MapToSlice(vm)
}

func (vm VarMap) Get(varKey string) (mvar.Var, bool) {
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
	return CheckPrefix(varKey) && CheckSuffix(varKey)
}

func CheckPrefix(varKey string) bool {
	return len(varKey) >= mvar.PrefixSize && varKey[:mvar.PrefixSize] == mvar.Prefix
}

func CheckSuffix(varKey string) bool {
	return len(varKey) >= mvar.SuffixSize && varKey[len(varKey)-mvar.SuffixSize:] == mvar.Suffix
}
