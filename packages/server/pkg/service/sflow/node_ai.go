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

var ErrNoNodeAIFound = sql.ErrNoRows

// NodeAIService provides CRUD operations for AI nodes.
type NodeAIService struct {
	reader  *NodeAIReader
	queries *gen.Queries
}

// NewNodeAIService creates a new service with queries.
func NewNodeAIService(queries *gen.Queries) NodeAIService {
	return NodeAIService{
		reader:  NewNodeAIReaderFromQueries(queries),
		queries: queries,
	}
}

// TX returns a new service scoped to the transaction.
func (s NodeAIService) TX(tx *sql.Tx) NodeAIService {
	newQueries := s.queries.WithTx(tx)
	return NodeAIService{
		reader:  NewNodeAIReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

// GetNodeAI returns an AI node by its flow node ID.
func (s NodeAIService) GetNodeAI(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeAI, error) {
	return s.reader.GetNodeAI(ctx, id)
}

// CreateNodeAI creates a new AI node.
func (s NodeAIService) CreateNodeAI(ctx context.Context, n mflow.NodeAI) error {
	return NewNodeAIWriterFromQueries(s.queries).CreateNodeAI(ctx, n)
}

// UpdateNodeAI updates an existing AI node.
func (s NodeAIService) UpdateNodeAI(ctx context.Context, n mflow.NodeAI) error {
	return NewNodeAIWriterFromQueries(s.queries).UpdateNodeAI(ctx, n)
}

// DeleteNodeAI deletes an AI node by its flow node ID.
func (s NodeAIService) DeleteNodeAI(ctx context.Context, id idwrap.IDWrap) error {
	err := NewNodeAIWriterFromQueries(s.queries).DeleteNodeAI(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeAIFound
	}
	return err
}

// Reader returns the underlying reader.
func (s NodeAIService) Reader() *NodeAIReader { return s.reader }
