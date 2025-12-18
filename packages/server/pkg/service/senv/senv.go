//nolint:revive // exported
package senv

import (
	"context"
	"database/sql"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
)

type EnvironmentService struct {
	reader  *Reader
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
		reader:  NewReaderFromQueries(queries, logger),
		queries: queries,
		logger:  logger,
	}
}

func (s EnvironmentService) TX(tx *sql.Tx) EnvironmentService {
	if tx == nil {
		return s
	}
	newQueries := s.queries.WithTx(tx)
	return EnvironmentService{
		reader:  NewReaderFromQueries(newQueries, s.logger),
		queries: newQueries,
		logger:  s.logger,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*EnvironmentService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := EnvironmentService{
		reader:  NewReaderFromQueries(queries, nil),
		queries: queries,
		logger:  slog.Default(),
	}
	return &service, nil
}

func (s EnvironmentService) GetEnvironment(ctx context.Context, id idwrap.IDWrap) (*menv.Env, error) {
	return s.reader.GetEnvironment(ctx, id)
}

func (s EnvironmentService) ListEnvironments(ctx context.Context, workspaceID idwrap.IDWrap) ([]menv.Env, error) {
	return s.reader.ListEnvironments(ctx, workspaceID)
}

func (s EnvironmentService) CreateEnvironment(ctx context.Context, env *menv.Env) error {
	return NewWriterFromQueries(s.queries).CreateEnvironment(ctx, env)
}

func (s EnvironmentService) UpdateEnvironment(ctx context.Context, env *menv.Env) error {
	return NewWriterFromQueries(s.queries).UpdateEnvironment(ctx, env)
}

func (s EnvironmentService) DeleteEnvironment(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).DeleteEnvironment(ctx, id)
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
	return s.reader.GetWorkspaceID(ctx, envID)
}

func (s EnvironmentService) CheckWorkspaceID(ctx context.Context, envID, ownerID idwrap.IDWrap) (bool, error) {
	return s.reader.CheckWorkspaceID(ctx, envID, ownerID)
}

func (s EnvironmentService) Reader() *Reader { return s.reader }
