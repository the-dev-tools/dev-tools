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

var ErrNoNodeAiProviderFound = sql.ErrNoRows

// NodeAiProviderService provides CRUD operations for AI Provider nodes.
type NodeAiProviderService struct {
	reader  *NodeAiProviderReader
	queries *gen.Queries
}

// NewNodeAiProviderService creates a new service with queries.
func NewNodeAiProviderService(queries *gen.Queries) NodeAiProviderService {
	return NodeAiProviderService{
		reader:  NewNodeAiProviderReaderFromQueries(queries),
		queries: queries,
	}
}

// TX returns a new service scoped to the transaction.
func (s NodeAiProviderService) TX(tx *sql.Tx) NodeAiProviderService {
	newQueries := s.queries.WithTx(tx)
	return NodeAiProviderService{
		reader:  NewNodeAiProviderReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

// GetNodeAiProvider returns an AI Provider node by its flow node ID.
func (s NodeAiProviderService) GetNodeAiProvider(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeAiProvider, error) {
	return s.reader.GetNodeAiProvider(ctx, id)
}

// CreateNodeAiProvider creates a new AI Provider node.
func (s NodeAiProviderService) CreateNodeAiProvider(ctx context.Context, n mflow.NodeAiProvider) error {
	return NewNodeAiProviderWriterFromQueries(s.queries).CreateNodeAiProvider(ctx, n)
}

// UpdateNodeAiProvider updates an existing AI Provider node.
func (s NodeAiProviderService) UpdateNodeAiProvider(ctx context.Context, n mflow.NodeAiProvider) error {
	return NewNodeAiProviderWriterFromQueries(s.queries).UpdateNodeAiProvider(ctx, n)
}

// DeleteNodeAiProvider deletes an AI Provider node by its flow node ID.
func (s NodeAiProviderService) DeleteNodeAiProvider(ctx context.Context, id idwrap.IDWrap) error {
	err := NewNodeAiProviderWriterFromQueries(s.queries).DeleteNodeAiProvider(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeAiProviderFound
	}
	return err
}

// Reader returns the underlying reader.
func (s NodeAiProviderService) Reader() *NodeAiProviderReader { return s.reader }
