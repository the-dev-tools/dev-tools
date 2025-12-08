//nolint:revive // exported
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

type EnvironmentService struct {
	queries *gen.Queries
	logger  *slog.Logger
}

// Backwards compatible alias for legacy code paths.
type EnvService = EnvironmentService

var (
	ErrNoEnvironmentFound = sql.ErrNoRows
	// Older call-sites use ErrNoEnvFound; keep the alias so we do not break them.
	ErrNoEnvFound = ErrNoEnvironmentFound
)

func New(queries *gen.Queries, logger *slog.Logger) EnvironmentService {
	if logger == nil {
		logger = slog.Default()
	}
	return EnvironmentService{
		queries: queries,
		logger:  logger,
	}
}

func (s EnvironmentService) TX(tx *sql.Tx) EnvironmentService {
	if tx == nil {
		return s
	}
	return EnvironmentService{
		queries: s.queries.WithTx(tx),
		logger:  s.logger,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*EnvironmentService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := EnvironmentService{
		queries: queries,
		logger:  slog.Default(),
	}
	return &service, nil
}

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

func (s EnvironmentService) GetEnvironment(ctx context.Context, id idwrap.IDWrap) (*menv.Env, error) {
	env, err := s.queries.GetEnvironment(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.DebugContext(ctx, "environment not found", "environment_id", id.String())
			return nil, ErrNoEnvironmentFound
		}
		return nil, err
	}
	return ConvertToModelEnv(env), nil
}

func (s EnvironmentService) ListEnvironments(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	envs, err := s.queries.GetEnvironmentsByWorkspaceIDOrdered(ctx, workspaceID)
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

func (s EnvironmentService) CreateEnvironment(ctx context.Context, env *menv.Env) error {
	if env.Order == 0 {
		nextOrder, err := s.nextDisplayOrder(ctx, env.WorkspaceID)
		if err != nil {
			return err
		}
		env.Order = nextOrder
	}

	dbEnv := ConvertToDBEnv(*env)
	return s.queries.CreateEnvironment(ctx, gen.CreateEnvironmentParams(dbEnv))
}

func (s EnvironmentService) UpdateEnvironment(ctx context.Context, env *menv.Env) error {
	if env.Order == 0 {
		current, err := s.queries.GetEnvironment(ctx, env.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNoEnvironmentFound
			}
			return err
		}
		env.Order = current.DisplayOrder
	}

	dbEnv := ConvertToDBEnv(*env)
	return s.queries.UpdateEnvironment(ctx, gen.UpdateEnvironmentParams{
		Type:         dbEnv.Type,
		ID:           dbEnv.ID,
		Name:         dbEnv.Name,
		Description:  dbEnv.Description,
		DisplayOrder: dbEnv.DisplayOrder,
	})
}

func (s EnvironmentService) DeleteEnvironment(ctx context.Context, id idwrap.IDWrap) error {
	if err := s.queries.DeleteEnvironment(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoEnvironmentFound
		}
		return err
	}
	return nil
}

// Backwards compatible wrappers ------------------------------------------------

func (s EnvironmentService) Create(ctx context.Context, env menv.Env) error {
	return s.CreateEnvironment(ctx, &env)
}

func (s EnvironmentService) Get(ctx context.Context, id idwrap.IDWrap) (*menv.Env, error) {
	return s.GetEnvironment(ctx, id)
}

func (s EnvironmentService) GetByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	return s.ListEnvironments(ctx, workspaceID)
}

func (s EnvironmentService) Update(ctx context.Context, env *menv.Env) error {
	return s.UpdateEnvironment(ctx, env)
}

func (s EnvironmentService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.DeleteEnvironment(ctx, id)
}

// Helpers ----------------------------------------------------------------------

func (s EnvironmentService) GetWorkspaceID(ctx context.Context, envID idwrap.IDWrap) (idwrap.IDWrap, error) {
	workspaceID, err := s.queries.GetEnvironmentWorkspaceID(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrNoEnvironmentFound
		}
		return idwrap.IDWrap{}, err
	}
	return workspaceID, nil
}

func (s EnvironmentService) CheckWorkspaceID(ctx context.Context, envID, ownerID idwrap.IDWrap) (bool, error) {
	workspaceID, err := s.GetWorkspaceID(ctx, envID)
	if err != nil {
		return false, err
	}
	return workspaceID.Compare(ownerID) == 0, nil
}

func (s EnvironmentService) nextDisplayOrder(ctx context.Context, workspaceID idwrap.IDWrap) (float64, error) {
	envs, err := s.queries.GetEnvironmentsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 1, nil
		}
		return 0, err
	}

	max := 0.0
	for _, env := range envs {
		if env.DisplayOrder > max {
			max = env.DisplayOrder
		}
	}
	return max + 1, nil
}
