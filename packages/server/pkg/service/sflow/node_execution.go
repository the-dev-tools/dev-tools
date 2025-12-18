//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeExecutionService struct {
	reader  *NodeExecutionReader
	queries *gen.Queries
}

func NewNodeExecutionService(queries *gen.Queries) NodeExecutionService {
	return NodeExecutionService{
		reader:  NewNodeExecutionReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeExecutionService) TX(tx *sql.Tx) NodeExecutionService {
	newQueries := s.queries.WithTx(tx)
	return NodeExecutionService{
		reader:  NewNodeExecutionReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeExecutionServiceTX(ctx context.Context, tx *sql.Tx) (*NodeExecutionService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeExecutionService{
		reader:  NewNodeExecutionReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (s NodeExecutionService) CreateNodeExecution(ctx context.Context, ne mflow.NodeExecution) error {
	return NewNodeExecutionWriterFromQueries(s.queries).CreateNodeExecution(ctx, ne)
}

func (s NodeExecutionService) GetNodeExecution(ctx context.Context, executionID idwrap.IDWrap) (*mflow.NodeExecution, error) {
	return s.reader.GetNodeExecution(ctx, executionID)
}

func (s NodeExecutionService) GetNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mflow.NodeExecution, error) {
	return s.reader.GetNodeExecutionsByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) ListNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mflow.NodeExecution, error) {
	// For now, use the existing method - could add pagination later
	return s.reader.GetNodeExecutionsByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) GetLatestNodeExecutionByNodeID(ctx context.Context, nodeID idwrap.IDWrap) (*mflow.NodeExecution, error) {
	return s.reader.GetLatestNodeExecutionByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) UpdateNodeExecution(ctx context.Context, ne mflow.NodeExecution) error {
	return NewNodeExecutionWriterFromQueries(s.queries).UpdateNodeExecution(ctx, ne)
}

func (s NodeExecutionService) UpsertNodeExecution(ctx context.Context, ne mflow.NodeExecution) error {
	return NewNodeExecutionWriterFromQueries(s.queries).UpsertNodeExecution(ctx, ne)
}

func (s NodeExecutionService) DeleteNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) error {
	return NewNodeExecutionWriterFromQueries(s.queries).DeleteNodeExecutionsByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) DeleteNodeExecutionsByNodeIDs(ctx context.Context, nodeIDs []idwrap.IDWrap) error {
	return NewNodeExecutionWriterFromQueries(s.queries).DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)
}

func (s NodeExecutionService) Reader() *NodeExecutionReader { return s.reader }
