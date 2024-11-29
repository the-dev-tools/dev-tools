package stag

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mtag"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
)

type TagService struct {
	queries *gen.Queries
}

var ErrNoTag error = sql.ErrNoRows

func New(queries *gen.Queries) TagService {
	return TagService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*TagService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &TagService{
		queries: queries,
	}, nil
}

func ConvertDBToModel(item gen.Tag) mtag.Tag {
	return mtag.Tag{
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		Name:        item.Name,
		Color:       uint8(item.Color),
	}
}

func ConvertModelToDB(item mtag.Tag) gen.Tag {
	return gen.Tag{
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		Name:        item.Name,
		Color:       int8(item.Color),
	}
}

func (s *TagService) GetTag(ctx context.Context, id idwrap.IDWrap) (mtag.Tag, error) {
	item, err := s.queries.GetTag(ctx, id)
	if err != nil {
		return mtag.Tag{}, err
	}
	return ConvertDBToModel(item), nil
}

func (s *TagService) GetTagByWorkspace(ctx context.Context, id idwrap.IDWrap) ([]mtag.Tag, error) {
	item, err := s.queries.GetTagsByWorkspaceID(ctx, id)
	if err != nil {
		return []mtag.Tag{}, err
	}

	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}

func (s *TagService) CreateTag(ctx context.Context, ftag mtag.Tag) error {
	arg := ConvertModelToDB(ftag)
	err := s.queries.CreateTag(ctx, gen.CreateTagParams{
		ID:          arg.ID,
		WorkspaceID: arg.WorkspaceID,
		Name:        arg.Name,
		Color:       arg.Color,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoTag, err)
}

func (s *TagService) UpdateTag(ctx context.Context, ftag mtag.Tag) error {
	arg := ConvertModelToDB(ftag)
	err := s.queries.UpdateTag(ctx, gen.UpdateTagParams{
		ID:    arg.ID,
		Name:  arg.Name,
		Color: arg.Color,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoTag, err)
}

func (s *TagService) DeleteTag(ctx context.Context, id idwrap.IDWrap) error {
	err := s.queries.DeleteTag(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoTag, err)
}
