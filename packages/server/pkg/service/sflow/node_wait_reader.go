package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWaitReader struct {
	queries *gen.Queries
}

func NewNodeWaitReader(db *sql.DB) *NodeWaitReader {
	return &NodeWaitReader{queries: gen.New(db)}
}

func NewNodeWaitReaderFromQueries(queries *gen.Queries) *NodeWaitReader {
	return &NodeWaitReader{queries: queries}
}

func (r *NodeWaitReader) GetNodeWait(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeWait, error) {
	nodeWait, err := r.queries.GetFlowNodeWait(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertDBToNodeWait(nodeWait), nil
}
