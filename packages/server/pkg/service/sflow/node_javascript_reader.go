package sflow

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeJsReader struct {
	queries *gen.Queries
}

func NewNodeJsReader(db *sql.DB) *NodeJsReader {
	return &NodeJsReader{queries: gen.New(db)}
}

func NewNodeJsReaderFromQueries(queries *gen.Queries) *NodeJsReader {
	return &NodeJsReader{queries: queries}
}

func (r *NodeJsReader) GetNodeJS(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeJS, error) {
	nodeJS, err := r.queries.GetFlowNodeJs(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertDBToNodeJs(nodeJS), nil
}
