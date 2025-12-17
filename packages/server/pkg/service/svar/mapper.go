package svar

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mvar"
)

func ConvertToDBVar(v mvar.Var) gen.Variable {
	return gen.Variable{
		ID:           v.ID,
		EnvID:        v.EnvID,
		VarKey:       v.VarKey,
		Value:        v.Value,
		Enabled:      v.Enabled,
		Description:  v.Description,
		DisplayOrder: v.Order,
	}
}

func ConvertToModelVar(v gen.Variable) *mvar.Var {
	return &mvar.Var{
		ID:          v.ID,
		EnvID:       v.EnvID,
		VarKey:      v.VarKey,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
		Order:       v.DisplayOrder,
	}
}
