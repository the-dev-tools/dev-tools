package sflow

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeRequestReader struct {
	queries *gen.Queries
}

func NewNodeRequestReader(db *sql.DB) *NodeRequestReader {
	return &NodeRequestReader{queries: gen.New(db)}
}

func NewNodeRequestReaderFromQueries(queries *gen.Queries) *NodeRequestReader {
	return &NodeRequestReader{queries: queries}
}

func (r *NodeRequestReader) GetNodeRequest(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeRequest, error) {
	nodeHTTP, err := r.queries.GetFlowNodeHTTP(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertToModelNodeHTTP(nodeHTTP), nil
}
