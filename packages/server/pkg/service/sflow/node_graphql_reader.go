package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeGraphQLReader struct {
	queries *gen.Queries
}

func NewNodeGraphQLReader(db *sql.DB) *NodeGraphQLReader {
	return &NodeGraphQLReader{queries: gen.New(db)}
}

func NewNodeGraphQLReaderFromQueries(queries *gen.Queries) *NodeGraphQLReader {
	return &NodeGraphQLReader{queries: queries}
}

func (r *NodeGraphQLReader) GetNodeGraphQL(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeGraphQL, error) {
	nodeGQL, err := r.queries.GetFlowNodeGraphQL(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertToModelNodeGraphQL(nodeGQL), nil
}
