package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeNoopReader struct {
	queries *gen.Queries
}

func NewNodeNoopReader(db *sql.DB) *NodeNoopReader {
	return &NodeNoopReader{queries: gen.New(db)}
}

func NewNodeNoopReaderFromQueries(queries *gen.Queries) *NodeNoopReader {
	return &NodeNoopReader{queries: queries}
}

func (r *NodeNoopReader) GetNodeNoop(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeNoop, error) {
	nodeFor, err := r.queries.GetFlowNodeNoop(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeStart(nodeFor), nil
}

func (r *NodeNoopReader) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.NodeNoop, error) {
	// Since there's no dedicated query for getting all NoOp nodes by flow ID,
	// we'll need to implement this at a higher level or add a SQL query.
	// For now, return empty slice to make it work with the collection API.
	return []mflow.NodeNoop{}, nil
}
