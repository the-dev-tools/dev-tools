package stag

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mtag"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
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
