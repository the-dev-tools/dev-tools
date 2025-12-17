//nolint:revive // exported
package svar

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
)

type VarService struct {
	reader  *Reader
	queries *gen.Queries
	logger  *slog.Logger
}

var (
	ErrNoVarFound                   = sql.ErrNoRows
	ErrEnvironmentBoundaryViolation = fmt.Errorf("variables must be in same environment")
	ErrSelfReferentialMove          = fmt.Errorf("cannot move variable relative to itself")
)

func New(queries *gen.Queries, logger *slog.Logger) VarService {
	if logger == nil {
		logger = slog.Default()
	}
	return VarService{
		reader:  NewReaderFromQueries(queries, logger),
		queries: queries,
		logger:  logger,
	}
}

func (s VarService) TX(tx *sql.Tx) VarService {
	if tx == nil {
		return s
	}
	newQueries := s.queries.WithTx(tx)
	return VarService{
		reader:  NewReaderFromQueries(newQueries, s.logger),
		queries: newQueries,
		logger:  s.logger,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*VarService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := VarService{
		reader:  NewReaderFromQueries(queries, nil),
		queries: queries,
		logger:  slog.Default(),
	}
	return &service, nil
}

func (s VarService) Get(ctx context.Context, id idwrap.IDWrap) (*mvar.Var, error) {
	return s.reader.Get(ctx, id)
}

func (s VarService) GetVariableByEnvID(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	return s.reader.GetVariableByEnvID(ctx, envID)
}

func (s VarService) Create(ctx context.Context, variable mvar.Var) error {
	return NewWriterFromQueries(s.queries).Create(ctx, variable)
}

func (s VarService) Update(ctx context.Context, variable *mvar.Var) error {
	return NewWriterFromQueries(s.queries).Update(ctx, variable)
}

func (s VarService) Upsert(ctx context.Context, variable mvar.Var) error {
	return NewWriterFromQueries(s.queries).Upsert(ctx, variable)
}

func (s VarService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).Delete(ctx, id)
}

func (s VarService) GetEnvID(ctx context.Context, varID idwrap.IDWrap) (idwrap.IDWrap, error) {
	return s.reader.GetEnvID(ctx, varID)
}

func (s VarService) CheckEnvironmentBoundaries(ctx context.Context, varID, envID idwrap.IDWrap) (bool, error) {
	return s.reader.CheckEnvironmentBoundaries(ctx, varID, envID)
}

func (s VarService) GetVariablesByEnvIDOrdered(ctx context.Context, envID idwrap.IDWrap) ([]mvar.Var, error) {
	return s.reader.GetVariablesByEnvIDOrdered(ctx, envID)
}

// Legacy move helpers ---------------------------------------------------------

func (s VarService) MoveVariableAfter(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).MoveVariableAfter(ctx, varID, targetVarID)
}

func (s VarService) MoveVariableBefore(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).MoveVariableBefore(ctx, varID, targetVarID)
}