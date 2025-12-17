package snodeif

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
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

func (r *Reader) GetNodeIf(ctx context.Context, id idwrap.IDWrap) (*mnif.MNIF, error) {
	nodeIf, err := r.queries.GetFlowNodeCondition(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeIf(nodeIf), nil
}
