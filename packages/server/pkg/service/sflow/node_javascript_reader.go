package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
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
