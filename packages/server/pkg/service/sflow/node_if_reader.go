package sflow

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeIfReader struct {
	queries *gen.Queries
}

func NewNodeIfReader(db *sql.DB) *NodeIfReader {
	return &NodeIfReader{queries: gen.New(db)}
}

func NewNodeIfReaderFromQueries(queries *gen.Queries) *NodeIfReader {
	return &NodeIfReader{queries: queries}
}

func (r *NodeIfReader) GetNodeIf(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeIf, error) {
	nodeIf, err := r.queries.GetFlowNodeCondition(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertToModelNodeIf(nodeIf), nil
}
