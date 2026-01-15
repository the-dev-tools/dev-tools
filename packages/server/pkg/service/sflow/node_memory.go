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

var ErrNoNodeMemoryFound = sql.ErrNoRows

// NodeMemoryService provides CRUD operations for Memory nodes.
type NodeMemoryService struct {
	reader  *NodeMemoryReader
	queries *gen.Queries
}

// NewNodeMemoryService creates a new service with queries.
func NewNodeMemoryService(queries *gen.Queries) NodeMemoryService {
	return NodeMemoryService{
		reader:  NewNodeMemoryReaderFromQueries(queries),
		queries: queries,
	}
}

// TX returns a new service scoped to the transaction.
func (s NodeMemoryService) TX(tx *sql.Tx) NodeMemoryService {
	newQueries := s.queries.WithTx(tx)
	return NodeMemoryService{
		reader:  NewNodeMemoryReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

// GetNodeMemory returns a Memory node by its flow node ID.
func (s NodeMemoryService) GetNodeMemory(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeMemory, error) {
	return s.reader.GetNodeMemory(ctx, id)
}

// CreateNodeMemory creates a new Memory node.
func (s NodeMemoryService) CreateNodeMemory(ctx context.Context, n mflow.NodeMemory) error {
	return NewNodeMemoryWriterFromQueries(s.queries).CreateNodeMemory(ctx, n)
}

// UpdateNodeMemory updates an existing Memory node.
func (s NodeMemoryService) UpdateNodeMemory(ctx context.Context, n mflow.NodeMemory) error {
	return NewNodeMemoryWriterFromQueries(s.queries).UpdateNodeMemory(ctx, n)
}

// DeleteNodeMemory deletes a Memory node by its flow node ID.
func (s NodeMemoryService) DeleteNodeMemory(ctx context.Context, id idwrap.IDWrap) error {
	err := NewNodeMemoryWriterFromQueries(s.queries).DeleteNodeMemory(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeMemoryFound
	}
	return err
}

// Reader returns the underlying reader.
func (s NodeMemoryService) Reader() *NodeMemoryReader { return s.reader }
