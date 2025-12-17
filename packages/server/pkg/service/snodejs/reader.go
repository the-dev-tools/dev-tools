package snodejs

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
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

func (r *Reader) GetNodeJS(ctx context.Context, id idwrap.IDWrap) (mnjs.MNJS, error) {
	nodeJS, err := r.queries.GetFlowNodeJs(ctx, id)
	if err != nil {
		return mnjs.MNJS{}, err
	}
	return ConvertDBToModel(nodeJS), nil
}
