package snodeexecution

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type Reader struct {
	queries *gen.Queries
}

func NewReader(db *sql.DB) *Reader {
	return &Reader{queries: gen.New(db)}
}

func NewReaderFromQueries(queries *gen.Queries) *Reader {
	return &Reader{queries: queries}
}

func (r *Reader) GetNodeExecution(ctx context.Context, executionID idwrap.IDWrap) (*mflow.NodeExecution, error) {
	execution, err := r.queries.GetNodeExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return ConvertNodeExecutionToModel(execution), nil
}

func (r *Reader) GetNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mflow.NodeExecution, error) {
	executions, err := r.queries.GetNodeExecutionsByNodeID(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mflow.NodeExecution{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(executions, ConvertNodeExecutionToModel), nil
}

func (r *Reader) GetLatestNodeExecutionByNodeID(ctx context.Context, nodeID idwrap.IDWrap) (*mflow.NodeExecution, error) {
	execution, err := r.queries.GetLatestNodeExecutionByNodeID(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertNodeExecutionToModel(execution), nil
}
