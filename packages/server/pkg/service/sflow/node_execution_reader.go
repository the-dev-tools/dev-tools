package sflow

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type NodeExecutionReader struct {
	queries *gen.Queries
}

func NewNodeExecutionReader(db *sql.DB) *NodeExecutionReader {
	return &NodeExecutionReader{queries: gen.New(db)}
}

func NewNodeExecutionReaderFromQueries(queries *gen.Queries) *NodeExecutionReader {
	return &NodeExecutionReader{queries: queries}
}

func (r *NodeExecutionReader) GetNodeExecution(ctx context.Context, executionID idwrap.IDWrap) (*mflow.NodeExecution, error) {
	execution, err := r.queries.GetNodeExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return ConvertNodeExecutionToModel(execution), nil
}

func (r *NodeExecutionReader) GetNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mflow.NodeExecution, error) {
	executions, err := r.queries.GetNodeExecutionsByNodeID(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mflow.NodeExecution{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(executions, ConvertNodeExecutionToModel), nil
}

func (r *NodeExecutionReader) GetLatestNodeExecutionByNodeID(ctx context.Context, nodeID idwrap.IDWrap) (*mflow.NodeExecution, error) {
	execution, err := r.queries.GetLatestNodeExecutionByNodeID(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertNodeExecutionToModel(execution), nil
}
