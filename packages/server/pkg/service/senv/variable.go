//nolint:revive // exported
package senv

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
)

type VariableService struct {
	reader  *VariableReader
	queries *gen.Queries
	logger  *slog.Logger
}

var (
	ErrNoVarFound                   = sql.ErrNoRows
	ErrEnvironmentBoundaryViolation = fmt.Errorf("variables must be in same environment")
	ErrSelfReferentialMove          = fmt.Errorf("cannot move variable relative to itself")
)

func NewVariableService(queries *gen.Queries, logger *slog.Logger) VariableService {
	if logger == nil {
		logger = slog.Default()
	}
	return VariableService{
		reader:  NewVariableReaderFromQueries(queries, logger),
		queries: queries,
		logger:  logger,
	}
}

func (s VariableService) TX(tx *sql.Tx) VariableService {
	if tx == nil {
		return s
	}
	newQueries := s.queries.WithTx(tx)
	return VariableService{
		reader:  NewVariableReaderFromQueries(newQueries, s.logger),
		queries: newQueries,
		logger:  s.logger,
	}
}

func NewVariableServiceTX(ctx context.Context, tx *sql.Tx) (*VariableService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := VariableService{
		reader:  NewVariableReaderFromQueries(queries, nil),
		queries: queries,
		logger:  slog.Default(),
	}
	return &service, nil
}

func (s VariableService) Get(ctx context.Context, id idwrap.IDWrap) (*menv.Variable, error) {
	return s.reader.Get(ctx, id)
}

func (s VariableService) GetVariableByEnvID(ctx context.Context, envID idwrap.IDWrap) ([]menv.Variable, error) {
	return s.reader.GetVariableByEnvID(ctx, envID)
}

func (s VariableService) Create(ctx context.Context, variable menv.Variable) error {
	return NewVariableWriterFromQueries(s.queries).Create(ctx, variable)
}

func (s VariableService) Update(ctx context.Context, variable *menv.Variable) error {
	return NewVariableWriterFromQueries(s.queries).Update(ctx, variable)
}

func (s VariableService) Upsert(ctx context.Context, variable menv.Variable) error {
	return NewVariableWriterFromQueries(s.queries).Upsert(ctx, variable)
}

func (s VariableService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewVariableWriterFromQueries(s.queries).Delete(ctx, id)
}

func (s VariableService) GetEnvID(ctx context.Context, varID idwrap.IDWrap) (idwrap.IDWrap, error) {
	return s.reader.GetEnvID(ctx, varID)
}

func (s VariableService) CheckEnvironmentBoundaries(ctx context.Context, varID, envID idwrap.IDWrap) (bool, error) {
	return s.reader.CheckEnvironmentBoundaries(ctx, varID, envID)
}

func (s VariableService) GetVariablesByEnvIDOrdered(ctx context.Context, envID idwrap.IDWrap) ([]menv.Variable, error) {
	return s.reader.GetVariablesByEnvIDOrdered(ctx, envID)
}

// Legacy move helpers ---------------------------------------------------------

func (s VariableService) MoveVariableAfter(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return NewVariableWriterFromQueries(s.queries).MoveVariableAfter(ctx, varID, targetVarID)
}

func (s VariableService) MoveVariableBefore(ctx context.Context, varID, targetVarID idwrap.IDWrap) error {
	return NewVariableWriterFromQueries(s.queries).MoveVariableBefore(ctx, varID, targetVarID)
}

func (s VariableService) Reader() *VariableReader { return s.reader }
