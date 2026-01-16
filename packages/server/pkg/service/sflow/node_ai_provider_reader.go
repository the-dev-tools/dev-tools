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

// NodeAiProviderReader reads AI Provider nodes from the database.
type NodeAiProviderReader struct {
	queries *gen.Queries
}

// NewNodeAiProviderReader creates a new reader with db connection.
func NewNodeAiProviderReader(db *sql.DB) *NodeAiProviderReader {
	return &NodeAiProviderReader{queries: gen.New(db)}
}

// NewNodeAiProviderReaderFromQueries creates a reader from existing queries.
func NewNodeAiProviderReaderFromQueries(queries *gen.Queries) *NodeAiProviderReader {
	return &NodeAiProviderReader{queries: queries}
}

// GetNodeAiProvider returns an AI Provider node by its flow node ID.
func (r *NodeAiProviderReader) GetNodeAiProvider(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeAiProvider, error) {
	dbNode, err := r.queries.GetFlowNodeAiProvider(ctx, id.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return ConvertDBToNodeAiProvider(dbNode), nil
}
