package snodenoop

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
)

type Reader struct {
	queries *gen.Queries
}

func NewReader(db *sql.DB) *Reader {
	return &Reader{queries: gen.New(db)}
}

func NewReaderFromQueries(queries *gen.Queries) *Reader {
	return &Reader{queries: queries}
}

func (r *Reader) GetNodeNoop(ctx context.Context, id idwrap.IDWrap) (*mnnoop.NoopNode, error) {
	nodeFor, err := r.queries.GetFlowNodeNoop(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeStart(nodeFor), nil
}

func (r *Reader) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnnoop.NoopNode, error) {
	// Since there's no dedicated query for getting all NoOp nodes by flow ID,
	// we'll need to implement this at a higher level or add a SQL query.
	// For now, return empty slice to make it work with the collection API.
	return []mnnoop.NoopNode{}, nil
}
