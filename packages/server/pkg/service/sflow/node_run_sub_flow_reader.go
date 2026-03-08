package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeRunSubFlowReader struct {
	queries *gen.Queries
}

func NewNodeRunSubFlowReader(db *sql.DB) *NodeRunSubFlowReader {
	return &NodeRunSubFlowReader{queries: gen.New(db)}
}

func NewNodeRunSubFlowReaderFromQueries(queries *gen.Queries) *NodeRunSubFlowReader {
	return &NodeRunSubFlowReader{queries: queries}
}

func (r *NodeRunSubFlowReader) GetNodeRunSubFlow(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeRunSubFlow, error) {
	row, err := r.queries.GetFlowNodeRunSubFlow(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertDBToNodeRunSubFlow(row), nil
}
