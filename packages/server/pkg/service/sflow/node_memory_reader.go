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

// NodeMemoryReader reads Memory nodes from the database.
type NodeMemoryReader struct {
	queries *gen.Queries
}

// NewNodeMemoryReader creates a new reader with db connection.
func NewNodeMemoryReader(db *sql.DB) *NodeMemoryReader {
	return &NodeMemoryReader{queries: gen.New(db)}
}

// NewNodeMemoryReaderFromQueries creates a reader from existing queries.
func NewNodeMemoryReaderFromQueries(queries *gen.Queries) *NodeMemoryReader {
	return &NodeMemoryReader{queries: queries}
}

// GetNodeMemory returns a Memory node by its flow node ID.
func (r *NodeMemoryReader) GetNodeMemory(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeMemory, error) {
	dbNode, err := r.queries.GetFlowNodeMemory(ctx, id.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToNodeMemory(dbNode), nil
}
