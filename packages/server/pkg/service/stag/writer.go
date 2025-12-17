package stag

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mtag"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{queries: gen.New(tx)}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{queries: queries}
}

func (w *Writer) CreateTag(ctx context.Context, ftag mtag.Tag) error {
	arg := ConvertModelToDB(ftag)
	err := w.queries.CreateTag(ctx, gen.CreateTagParams(arg))
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoTag, err)
}

func (w *Writer) UpdateTag(ctx context.Context, ftag mtag.Tag) error {
	arg := ConvertModelToDB(ftag)
	err := w.queries.UpdateTag(ctx, gen.UpdateTagParams{
		ID:    arg.ID,
		Name:  arg.Name,
		Color: arg.Color,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoTag, err)
}

func (w *Writer) DeleteTag(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteTag(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoTag, err)
}
