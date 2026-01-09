//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type FlowVariableService struct {
	reader  *FlowVariableReader
	queries *gen.Queries
}

var ErrNoFlowVariableFound = errors.New("no flow variable find")

func NewFlowVariableService(queries *gen.Queries) FlowVariableService {
	return FlowVariableService{
		reader:  NewFlowVariableReaderFromQueries(queries),
		queries: queries,
	}
}

func (s FlowVariableService) TX(tx *sql.Tx) FlowVariableService {
	newQueries := s.queries.WithTx(tx)
	return FlowVariableService{
		reader:  NewFlowVariableReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewFlowVariableServiceTX(ctx context.Context, tx *sql.Tx) (*FlowVariableService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}

	return &FlowVariableService{
		reader:  NewFlowVariableReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (s *FlowVariableService) GetFlowVariable(ctx context.Context, id idwrap.IDWrap) (mflow.FlowVariable, error) {
	return s.reader.GetFlowVariable(ctx, id)
}

func (s *FlowVariableService) GetFlowVariablesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.FlowVariable, error) {
	return s.reader.GetFlowVariablesByFlowID(ctx, flowID)
}

func (s *FlowVariableService) CreateFlowVariable(ctx context.Context, item mflow.FlowVariable) error {
	return NewFlowVariableWriterFromQueries(s.queries).CreateFlowVariable(ctx, item)
}

func (s *FlowVariableService) CreateFlowVariableBulk(ctx context.Context, variables []mflow.FlowVariable) error {
	return NewFlowVariableWriterFromQueries(s.queries).CreateFlowVariableBulk(ctx, variables)
}

func (s *FlowVariableService) UpdateFlowVariable(ctx context.Context, item mflow.FlowVariable) error {
	return NewFlowVariableWriterFromQueries(s.queries).UpdateFlowVariable(ctx, item)
}

func (s *FlowVariableService) DeleteFlowVariable(ctx context.Context, id idwrap.IDWrap) error {
	return NewFlowVariableWriterFromQueries(s.queries).DeleteFlowVariable(ctx, id)
}

// GetFlowVariablesByFlowIDOrdered returns flow variables in the flow ordered by display_order
func (s *FlowVariableService) GetFlowVariablesByFlowIDOrdered(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.FlowVariable, error) {
	return s.reader.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
}

// UpdateFlowVariableOrder updates the display_order for a single flow variable
func (s *FlowVariableService) UpdateFlowVariableOrder(ctx context.Context, id idwrap.IDWrap, order float64) error {
	return NewFlowVariableWriterFromQueries(s.queries).UpdateFlowVariableOrder(ctx, id, order)
}

// MoveFlowVariableAfter moves a flow variable to be positioned after the target variable
func (s *FlowVariableService) MoveFlowVariableAfter(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	return NewFlowVariableWriterFromQueries(s.queries).MoveFlowVariableAfter(ctx, variableID, targetVariableID)
}

// MoveFlowVariableAfterTX moves a flow variable to be positioned after the target variable within a transaction
func (s *FlowVariableService) MoveFlowVariableAfterTX(ctx context.Context, tx *sql.Tx, variableID, targetVariableID idwrap.IDWrap) error {
	var queries *gen.Queries
	if tx != nil {
		queries = s.queries.WithTx(tx)
	} else {
		queries = s.queries
	}
	return NewFlowVariableWriterFromQueries(queries).MoveFlowVariableAfter(ctx, variableID, targetVariableID)
}

// MoveFlowVariableBefore moves a flow variable to be positioned before the target variable
func (s *FlowVariableService) MoveFlowVariableBefore(ctx context.Context, variableID, targetVariableID idwrap.IDWrap) error {
	return NewFlowVariableWriterFromQueries(s.queries).MoveFlowVariableBefore(ctx, variableID, targetVariableID)
}

// MoveFlowVariableBeforeTX moves a flow variable to be positioned before the target variable within a transaction
func (s *FlowVariableService) MoveFlowVariableBeforeTX(ctx context.Context, tx *sql.Tx, variableID, targetVariableID idwrap.IDWrap) error {
	var queries *gen.Queries
	if tx != nil {
		queries = s.queries.WithTx(tx)
	} else {
		queries = s.queries
	}
	return NewFlowVariableWriterFromQueries(queries).MoveFlowVariableBefore(ctx, variableID, targetVariableID)
}

// ReorderFlowVariables performs a bulk reorder of flow variables by updating their display_order
func (s *FlowVariableService) ReorderFlowVariables(ctx context.Context, orderedIDs []idwrap.IDWrap) error {
	return NewFlowVariableWriterFromQueries(s.queries).ReorderFlowVariables(ctx, orderedIDs)
}

// ReorderFlowVariablesTX performs a bulk reorder of flow variables within a transaction
func (s *FlowVariableService) ReorderFlowVariablesTX(ctx context.Context, tx *sql.Tx, orderedIDs []idwrap.IDWrap) error {
	var queries *gen.Queries
	if tx != nil {
		queries = s.queries.WithTx(tx)
	} else {
		queries = s.queries
	}
	return NewFlowVariableWriterFromQueries(queries).ReorderFlowVariables(ctx, orderedIDs)
}

func (s FlowVariableService) Reader() *FlowVariableReader { return s.reader }
