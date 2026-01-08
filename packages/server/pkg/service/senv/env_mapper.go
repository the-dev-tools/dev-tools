package senv

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
)

func ConvertToDBEnv(env menv.Env) gen.Environment {
	return gen.Environment{
		ID:           env.ID,
		WorkspaceID:  env.WorkspaceID,
		Type:         int8(env.Type),
		Name:         env.Name,
		Description:  env.Description,
		DisplayOrder: env.Order,
	}
}

func ConvertToModelEnv(env gen.Environment) *menv.Env {
	return &menv.Env{
		ID:          env.ID,
		WorkspaceID: env.WorkspaceID,
		Type:        menv.EnvType(env.Type),
		Name:        env.Name,
		Description: env.Description,
		Order:       env.DisplayOrder,
	}
}
