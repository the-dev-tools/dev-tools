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

type VariableReader struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func NewVariableReader(db *sql.DB, logger *slog.Logger) *VariableReader {
	if logger == nil {
		logger = slog.Default()
	}
	return &VariableReader{
		queries: gen.New(db),
		logger:  logger,
	}
}

func NewVariableReaderFromQueries(queries *gen.Queries, logger *slog.Logger) *VariableReader {
	if logger == nil {
		logger = slog.Default()
	}
	return &VariableReader{
		queries: queries,
		logger:  logger,
	}
}

func (r *VariableReader) Get(ctx context.Context, id idwrap.IDWrap) (*menv.Variable, error) {
	variable, err := r.queries.GetVariable(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoVarFound
		}
		return nil, err
	}
	return ConvertToModelVar(variable), nil
}

func (r *VariableReader) GetVariableByEnvID(ctx context.Context, envID idwrap.IDWrap) ([]menv.Variable, error) {
	rows, err := r.queries.GetVariablesByEnvironmentIDOrdered(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []menv.Variable{}, nil
		}
		return nil, err
	}

	vars := make([]menv.Variable, len(rows))
	for i, row := range rows {
		vars[i] = *ConvertToModelVar(row)
	}
	return vars, nil
}

func (r *VariableReader) GetEnvID(ctx context.Context, varID idwrap.IDWrap) (idwrap.IDWrap, error) {
	variable, err := r.queries.GetVariable(ctx, varID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrNoVarFound
		}
		return idwrap.IDWrap{}, err
	}
	return variable.EnvID, nil
}

func (r *VariableReader) CheckEnvironmentBoundaries(ctx context.Context, varID, envID idwrap.IDWrap) (bool, error) {
	actualEnvID, err := r.GetEnvID(ctx, varID)
	if err != nil {
		return false, err
	}
	return actualEnvID.Compare(envID) == 0, nil
}

func (r *VariableReader) GetVariablesByEnvIDOrdered(ctx context.Context, envID idwrap.IDWrap) ([]menv.Variable, error) {
	return r.GetVariableByEnvID(ctx, envID)
}

func (r *VariableReader) GetVariablesByEnvironmentID(ctx context.Context, envID idwrap.IDWrap) ([]menv.Variable, error) {
	vars, err := r.queries.GetVariablesByEnvironmentID(ctx, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []menv.Variable{}, nil
		}
		return nil, err
	}

	result := make([]menv.Variable, len(vars))
	for i, v := range vars {
		result[i] = *ConvertToModelVar(v)
	}
	return result, nil
}
