package senv

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
)

type EnvReader struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func NewEnvReader(db *sql.DB, logger *slog.Logger) *EnvReader {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnvReader{
		queries: gen.New(db),
		logger:  logger,
	}
}

func NewEnvReaderFromQueries(queries *gen.Queries, logger *slog.Logger) *EnvReader {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnvReader{
		queries: queries,
		logger:  logger,
	}
}

func (r *EnvReader) GetEnvironment(ctx context.Context, id idwrap.IDWrap) (*menv.Env, error) {
	env, err := r.queries.GetEnvironment(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.DebugContext(ctx, "environment not found", "environment_id", id.String())
			return nil, ErrNoEnvironmentFound
		}
		return nil, err
	}
	return ConvertToModelEnv(env), nil
}

func (r *EnvReader) ListEnvironments(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	envs, err := r.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []menv.Env{}, nil
		}
		return nil, err
	}

	result := make([]menv.Env, len(envs))
	for i, env := range envs {
		result[i] = *ConvertToModelEnv(env)
	}
	return result, nil
}

func (r *EnvReader) GetWorkspaceID(ctx context.Context, envID idwrap.IDWrap) (idwrap.IDWrap, error) {
	workspaceID, err := r.queries.GetEnvironmentWorkspaceID(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrNoEnvironmentFound
		}
		return idwrap.IDWrap{}, err
	}
	return workspaceID, nil
}

func (r *EnvReader) CheckWorkspaceID(ctx context.Context, envID, ownerID idwrap.IDWrap) (bool, error) {
	workspaceID, err := r.GetWorkspaceID(ctx, envID)
	if err != nil {
		return false, err
	}
	return workspaceID.Compare(ownerID) == 0, nil
}
