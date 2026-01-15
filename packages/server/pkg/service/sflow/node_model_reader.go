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

// NodeModelReader reads Model nodes from the database.
type NodeModelReader struct {
	queries *gen.Queries
}

// NewNodeModelReader creates a new reader with db connection.
func NewNodeModelReader(db *sql.DB) *NodeModelReader {
	return &NodeModelReader{queries: gen.New(db)}
}

// NewNodeModelReaderFromQueries creates a reader from existing queries.
func NewNodeModelReaderFromQueries(queries *gen.Queries) *NodeModelReader {
	return &NodeModelReader{queries: queries}
}

// GetNodeModel returns a Model node by its flow node ID.
func (r *NodeModelReader) GetNodeModel(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeModel, error) {
	dbNode, err := r.queries.GetFlowNodeModel(ctx, id.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToNodeModel(dbNode), nil
}
