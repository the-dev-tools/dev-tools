//nolint:revive // exported
package snodeexecution

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnodeexecution"
)

type NodeExecutionService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeExecutionService {
	return NodeExecutionService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeExecutionService) TX(tx *sql.Tx) NodeExecutionService {
	newQueries := s.queries.WithTx(tx)
	return NodeExecutionService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeExecutionService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeExecutionService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (s NodeExecutionService) CreateNodeExecution(ctx context.Context, ne mnodeexecution.NodeExecution) error {
	return NewWriterFromQueries(s.queries).CreateNodeExecution(ctx, ne)
}

func (s NodeExecutionService) GetNodeExecution(ctx context.Context, executionID idwrap.IDWrap) (*mnodeexecution.NodeExecution, error) {
	return s.reader.GetNodeExecution(ctx, executionID)
}

func (s NodeExecutionService) GetNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mnodeexecution.NodeExecution, error) {
	return s.reader.GetNodeExecutionsByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) ListNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mnodeexecution.NodeExecution, error) {
	// For now, use the existing method - could add pagination later
	return s.reader.GetNodeExecutionsByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) GetLatestNodeExecutionByNodeID(ctx context.Context, nodeID idwrap.IDWrap) (*mnodeexecution.NodeExecution, error) {
	return s.reader.GetLatestNodeExecutionByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) UpdateNodeExecution(ctx context.Context, ne mnodeexecution.NodeExecution) error {
	return NewWriterFromQueries(s.queries).UpdateNodeExecution(ctx, ne)
}

func (s NodeExecutionService) UpsertNodeExecution(ctx context.Context, ne mnodeexecution.NodeExecution) error {
	return NewWriterFromQueries(s.queries).UpsertNodeExecution(ctx, ne)
}

func (s NodeExecutionService) DeleteNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).DeleteNodeExecutionsByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) DeleteNodeExecutionsByNodeIDs(ctx context.Context, nodeIDs []idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)
}