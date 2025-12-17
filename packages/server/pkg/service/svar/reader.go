package svar

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
)

type Reader struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func NewReader(db *sql.DB, logger *slog.Logger) *Reader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{
		queries: gen.New(db),
		logger:  logger,
	}
}

func NewReaderFromQueries(queries *gen.Queries, logger *slog.Logger) *Reader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{
		queries: queries,
		logger:  logger,
	}
}

func (r *Reader) Get(ctx context.Context, id idwrap.IDWrap) (*mvar.Var, error) {
	variable, err := r.queries.GetVariable(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoVarFound
		}
		return nil, err
	}
	return ConvertToModelVar(variable), nil
}

func (r *Reader) GetVariableByEnvID(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	rows, err := r.queries.GetVariablesByEnvironmentIDOrdered(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mvar.Var{}, nil
		}
		return nil, err
	}

	vars := make([]mvar.Var, len(rows))
	for i, row := range rows {
		vars[i] = *ConvertToModelVar(row)
	}
	return vars, nil
}

func (r *Reader) GetEnvID(ctx context.Context, varID idwrap.IDWrap) (idwrap.IDWrap, error) {
	variable, err := r.queries.GetVariable(ctx, varID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrNoVarFound
		}
		return idwrap.IDWrap{}, err
	}
	return variable.EnvID, nil
}

func (r *Reader) CheckEnvironmentBoundaries(ctx context.Context, varID, envID idwrap.IDWrap) (bool, error) {
	actualEnvID, err := r.GetEnvID(ctx, varID)
	if err != nil {
		return false, err
	}
	return actualEnvID.Compare(envID) == 0, nil
}

func (r *Reader) GetVariablesByEnvIDOrdered(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	return r.GetVariableByEnvID(ctx, envID)
}

func (r *Reader) GetVariablesByEnvironmentID(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	vars, err := r.queries.GetVariablesByEnvironmentID(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mvar.Var{}, nil
		}
		return nil, err
	}

	result := make([]mvar.Var, len(vars))
	for i, v := range vars {
		result[i] = *ConvertToModelVar(v)
	}
	return result, nil
}
