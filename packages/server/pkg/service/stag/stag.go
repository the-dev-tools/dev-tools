//nolint:revive // exported
package stag

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mtag"
)

type TagService struct {
	reader  *Reader
	queries *gen.Queries
}

var ErrNoTag error = sql.ErrNoRows

func New(queries *gen.Queries) TagService {
	return TagService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (s *TagService) TX(tx *sql.Tx) TagService {
	newQueries := s.queries.WithTx(tx)
	return TagService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*TagService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &TagService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (s *TagService) GetTag(ctx context.Context, id idwrap.IDWrap) (mtag.Tag, error) {
	return s.reader.GetTag(ctx, id)
}

func (s *TagService) GetTagByWorkspace(ctx context.Context, id idwrap.IDWrap) ([]mtag.Tag, error) {
	return s.reader.GetTagByWorkspace(ctx, id)
}

func (s *TagService) CreateTag(ctx context.Context, ftag mtag.Tag) error {
	return NewWriterFromQueries(s.queries).CreateTag(ctx, ftag)
}

func (s *TagService) UpdateTag(ctx context.Context, ftag mtag.Tag) error {
	return NewWriterFromQueries(s.queries).UpdateTag(ctx, ftag)
}

func (s *TagService) DeleteTag(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).DeleteTag(ctx, id)
}
