package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeForReader struct {
	queries *gen.Queries
}

func NewNodeForReader(db *sql.DB) *NodeForReader {
	return &NodeForReader{queries: gen.New(db)}
}

func NewNodeForReaderFromQueries(queries *gen.Queries) *NodeForReader {
	return &NodeForReader{queries: queries}
}

func (r *NodeForReader) GetNodeFor(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeFor, error) {
	nodeFor, err := r.queries.GetFlowNodeFor(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertDBToNodeFor(nodeFor), nil
}
