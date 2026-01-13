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

// NodeAIReader reads AI nodes from the database.
type NodeAIReader struct {
	queries *gen.Queries
}

// NewNodeAIReader creates a new reader with db connection.
func NewNodeAIReader(db *sql.DB) *NodeAIReader {
	return &NodeAIReader{queries: gen.New(db)}
}

// NewNodeAIReaderFromQueries creates a reader from existing queries.
func NewNodeAIReaderFromQueries(queries *gen.Queries) *NodeAIReader {
	return &NodeAIReader{queries: queries}
}

// GetNodeAI returns an AI node by its flow node ID.
func (r *NodeAIReader) GetNodeAI(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeAI, error) {
	dbNode, err := r.queries.GetFlowNodeAI(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToNodeAi(dbNode), nil
}
