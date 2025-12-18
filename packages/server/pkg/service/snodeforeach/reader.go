package snodeforeach

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
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

func (r *Reader) GetNodeForEach(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeForEach, error) {
	nodeForEach, err := r.queries.GetFlowNodeForEach(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeFor(nodeForEach), nil
}
