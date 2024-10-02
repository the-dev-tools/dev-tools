package senv

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
)

type EnvService struct {
	queries *gen.Queries
}

var ErrNoEnvFound error = sql.ErrNoRows

func New(ctx context.Context, db *sql.DB) (EnvService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return EnvService{}, err
	}
	service := EnvService{queries: queries}
	return service, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*EnvService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := EnvService{queries: queries}
	return &service, nil
}

func ConvertToDBEnv(env menv.Env) gen.Environment {
	return gen.Environment{
		ID:          env.ID,
		WorkspaceID: env.WorkspaceID,
		Type:        int8(env.Type),
		Name:        env.Name,
		Description: env.Description,
	}
}

func ConvertToModelEnv(env gen.Environment) *menv.Env {
	return &menv.Env{
		ID:          env.ID,
		WorkspaceID: env.WorkspaceID,
		Type:        menv.EnvType(env.Type),
		Name:        env.Name,
		Description: env.Description,
	}
}

func (e EnvService) Get(ctx context.Context, id idwrap.IDWrap) (*menv.Env, error) {
	env, err := e.queries.GetEnvironment(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoEnvFound
		}
		return nil, err
	}
	return ConvertToModelEnv(env), nil
}

func (e EnvService) GetByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	envs, err := e.queries.GetEnvironmentsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []menv.Env{}, ErrNoEnvFound
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(envs, ConvertToModelEnv), nil
}

func (e EnvService) Create(ctx context.Context, env menv.Env) error {
	dbEnv := ConvertToDBEnv(env)
	return e.queries.CreateEnvironment(ctx, gen.CreateEnvironmentParams{
		ID:          dbEnv.ID,
		WorkspaceID: dbEnv.WorkspaceID,
		Type:        dbEnv.Type,
		Name:        dbEnv.Name,
		Description: dbEnv.Description,
	})
}

func (e EnvService) Update(ctx context.Context, env *menv.Env) error {
	dbEnv := ConvertToDBEnv(*env)
	return e.queries.UpdateEnvironment(ctx, gen.UpdateEnvironmentParams{
		ID:          dbEnv.ID,
		Name:        dbEnv.Name,
		Description: dbEnv.Description,
	})
}

func (e EnvService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return e.queries.DeleteEnvironment(ctx, id)
}
