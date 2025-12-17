package stag

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mtag"
	"the-dev-tools/server/pkg/translate/tgeneric"
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

func (r *Reader) GetTag(ctx context.Context, id idwrap.IDWrap) (mtag.Tag, error) {
	item, err := r.queries.GetTag(ctx, id)
	if err != nil {
		return mtag.Tag{}, err
	}
	return ConvertDBToModel(item), nil
}

func (r *Reader) GetTagByWorkspace(ctx context.Context, id idwrap.IDWrap) ([]mtag.Tag, error) {
	item, err := r.queries.GetTagsByWorkspaceID(ctx, id)
	if err != nil {
		return []mtag.Tag{}, err
	}

	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}
