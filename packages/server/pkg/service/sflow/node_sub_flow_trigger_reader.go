package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeSubFlowTriggerReader struct {
	queries *gen.Queries
}

func NewNodeSubFlowTriggerReader(db *sql.DB) *NodeSubFlowTriggerReader {
	return &NodeSubFlowTriggerReader{queries: gen.New(db)}
}

func NewNodeSubFlowTriggerReaderFromQueries(queries *gen.Queries) *NodeSubFlowTriggerReader {
	return &NodeSubFlowTriggerReader{queries: queries}
}

func (r *NodeSubFlowTriggerReader) GetNodeSubFlowTrigger(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeSubFlowTrigger, error) {
	row, err := r.queries.GetFlowNodeSubFlowTrigger(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertDBToNodeSubFlowTrigger(row), nil
}
