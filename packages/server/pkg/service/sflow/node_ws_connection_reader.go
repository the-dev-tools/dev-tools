package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWsConnectionReader struct {
	queries *gen.Queries
}

func NewNodeWsConnectionReaderFromQueries(queries *gen.Queries) *NodeWsConnectionReader {
	return &NodeWsConnectionReader{queries: queries}
}

func (r *NodeWsConnectionReader) GetNodeWsConnection(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeWsConnection, error) {
	nodeWsConn, err := r.queries.GetFlowNodeWsConnection(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertToModelNodeWsConnection(nodeWsConn), nil
}
