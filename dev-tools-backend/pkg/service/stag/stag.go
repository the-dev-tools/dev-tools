package stag

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mftag"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
)

type TagService struct {
	queries *gen.Queries
}

var ErrNoFTag error = sql.ErrNoRows

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

func ConvertDBToModel(item gen.Ftag) mftag.FlowTag {
	return mftag.FlowTag{
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		Name:        item.Name,
	}
}

func ConvertModelToDB(item mftag.FlowTag) gen.Ftag {
	return gen.Ftag{
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		Name:        item.Name,
	}
}

func (s *TagService) GetFlowTag(ctx context.Context, id idwrap.IDWrap) (mftag.FlowTag, error) {
	item, err := s.queries.GetFTag(ctx, id)
	if err != nil {
		return mftag.FlowTag{}, err
	}
	return ConvertDBToModel(item), nil
}

func (s *TagService) CreateFlowTag(ctx context.Context, ftag mftag.FlowTag) error {
	arg := ConvertModelToDB(ftag)
	err := s.queries.CreateFTag(ctx, gen.CreateFTagParams{
		ID:          arg.ID,
		WorkspaceID: arg.WorkspaceID,
		Name:        arg.Name,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFTag, err)
}

func (s *TagService) UpdateFlowTag(ctx context.Context, ftag mftag.FlowTag) error {
	arg := ConvertModelToDB(ftag)
	err := s.queries.UpdateFTag(ctx, gen.UpdateFTagParams{
		ID:   arg.ID,
		Name: arg.Name,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFTag, err)
}

func (s *TagService) DeleteFlowTag(ctx context.Context, id idwrap.IDWrap) error {
	err := s.queries.DeleteFTag(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFTag, err)
}
