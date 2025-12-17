package senv

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{
		queries: gen.New(tx),
	}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{
		queries: queries,
	}
}

func (w *Writer) CreateEnvironment(ctx context.Context, env *menv.Env) error {
	if env.Order == 0 {
		nextOrder, err := w.nextDisplayOrder(ctx, env.WorkspaceID)
		if err != nil {
			return err
		}
		env.Order = nextOrder
	}

	dbEnv := ConvertToDBEnv(*env)
	return w.queries.CreateEnvironment(ctx, gen.CreateEnvironmentParams(dbEnv))
}

func (w *Writer) UpdateEnvironment(ctx context.Context, env *menv.Env) error {
	if env.Order == 0 {
		current, err := w.queries.GetEnvironment(ctx, env.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNoEnvironmentFound
			}
			return err
		}
		env.Order = current.DisplayOrder
	}

	dbEnv := ConvertToDBEnv(*env)
	return w.queries.UpdateEnvironment(ctx, gen.UpdateEnvironmentParams{
		Type:         dbEnv.Type,
		ID:           dbEnv.ID,
		Name:         dbEnv.Name,
		Description:  dbEnv.Description,
		DisplayOrder: dbEnv.DisplayOrder,
	})
}

func (w *Writer) DeleteEnvironment(ctx context.Context, id idwrap.IDWrap) error {
	if err := w.queries.DeleteEnvironment(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoEnvironmentFound
		}
		return err
	}
	return nil
}

func (w *Writer) nextDisplayOrder(ctx context.Context, workspaceID idwrap.IDWrap) (float64, error) {
	envs, err := w.queries.GetEnvironmentsByWorkspaceID(ctx, workspaceID)
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
