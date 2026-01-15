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

var ErrNoNodeModelFound = sql.ErrNoRows

// NodeModelService provides CRUD operations for Model nodes.
type NodeModelService struct {
	reader  *NodeModelReader
	queries *gen.Queries
}

// NewNodeModelService creates a new service with queries.
func NewNodeModelService(queries *gen.Queries) NodeModelService {
	return NodeModelService{
		reader:  NewNodeModelReaderFromQueries(queries),
		queries: queries,
	}
}

// TX returns a new service scoped to the transaction.
func (s NodeModelService) TX(tx *sql.Tx) NodeModelService {
	newQueries := s.queries.WithTx(tx)
	return NodeModelService{
		reader:  NewNodeModelReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

// GetNodeModel returns a Model node by its flow node ID.
func (s NodeModelService) GetNodeModel(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeModel, error) {
	return s.reader.GetNodeModel(ctx, id)
}

// CreateNodeModel creates a new Model node.
func (s NodeModelService) CreateNodeModel(ctx context.Context, n mflow.NodeModel) error {
	return NewNodeModelWriterFromQueries(s.queries).CreateNodeModel(ctx, n)
}

// UpdateNodeModel updates an existing Model node.
func (s NodeModelService) UpdateNodeModel(ctx context.Context, n mflow.NodeModel) error {
	return NewNodeModelWriterFromQueries(s.queries).UpdateNodeModel(ctx, n)
}

// DeleteNodeModel deletes a Model node by its flow node ID.
func (s NodeModelService) DeleteNodeModel(ctx context.Context, id idwrap.IDWrap) error {
	err := NewNodeModelWriterFromQueries(s.queries).DeleteNodeModel(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeModelFound
	}
	return err
}

// Reader returns the underlying reader.
func (s NodeModelService) Reader() *NodeModelReader { return s.reader }
