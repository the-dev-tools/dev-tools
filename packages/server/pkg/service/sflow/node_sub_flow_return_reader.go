package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeSubFlowReturnReader struct {
	queries *gen.Queries
}

func NewNodeSubFlowReturnReader(db *sql.DB) *NodeSubFlowReturnReader {
	return &NodeSubFlowReturnReader{queries: gen.New(db)}
}

func NewNodeSubFlowReturnReaderFromQueries(queries *gen.Queries) *NodeSubFlowReturnReader {
	return &NodeSubFlowReturnReader{queries: queries}
}

func (r *NodeSubFlowReturnReader) GetNodeSubFlowReturn(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeSubFlowReturn, error) {
	row, err := r.queries.GetFlowNodeSubFlowReturn(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertDBToNodeSubFlowReturn(row), nil
}
