package senv

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
)

func ConvertToDBVar(v menv.Variable) gen.Variable {
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

func ConvertToModelVar(v gen.Variable) *menv.Variable {
	return &menv.Variable{
		ID:          v.ID,
		EnvID:       v.EnvID,
		VarKey:      v.VarKey,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
		Order:       v.DisplayOrder,
	}
}
