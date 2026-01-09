package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeForEachReader struct {
	queries *gen.Queries
}

func NewNodeForEachReader(db *sql.DB) *NodeForEachReader {
	return &NodeForEachReader{queries: gen.New(db)}
}

func NewNodeForEachReaderFromQueries(queries *gen.Queries) *NodeForEachReader {
	return &NodeForEachReader{queries: queries}
}

func (r *NodeForEachReader) GetNodeForEach(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeForEach, error) {
	nodeForEach, err := r.queries.GetFlowNodeForEach(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertDBToNodeForEach(nodeForEach), nil
}
