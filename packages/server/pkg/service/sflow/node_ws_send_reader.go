package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWsSendReader struct {
	queries *gen.Queries
}

func NewNodeWsSendReader(db *sql.DB) *NodeWsSendReader {
	return &NodeWsSendReader{queries: gen.New(db)}
}

func NewNodeWsSendReaderFromQueries(queries *gen.Queries) *NodeWsSendReader {
	return &NodeWsSendReader{queries: queries}
}

func (r *NodeWsSendReader) GetNodeWsSend(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeWsSend, error) {
	nodeWsSend, err := r.queries.GetFlowNodeWsSend(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertToModelNodeWsSend(nodeWsSend), nil
}
