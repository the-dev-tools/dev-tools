//nolint:revive // exported
package sflowtag

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflowtag"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type FlowTagService struct {
	queries *gen.Queries
}

var ErrNoFlowTag error = sql.ErrNoRows

func New(queries *gen.Queries) FlowTagService {
	return FlowTagService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*FlowTagService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &FlowTagService{
		queries: queries,
	}, nil
}

func ConvertDBToModel(item gen.FlowTag) mflowtag.FlowTag {
	return mflowtag.FlowTag{
		ID:     item.ID,
		FlowID: item.FlowID,
		TagID:  item.TagID,
	}
}

func ConvertModelToDB(item mflowtag.FlowTag) gen.FlowTag {
	return gen.FlowTag{
		ID:     item.ID,
		FlowID: item.FlowID,
		TagID:  item.TagID,
	}
}

func (s *FlowTagService) GetFlowTag(ctx context.Context, id idwrap.IDWrap) (mflowtag.FlowTag, error) {
	item, err := s.queries.GetFlowTag(ctx, id)
	if err != nil {
		return mflowtag.FlowTag{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
	}
	return ConvertDBToModel(item), nil
}

func (s *FlowTagService) GetFlowTagsByTagID(ctx context.Context, tagID idwrap.IDWrap) ([]mflowtag.FlowTag, error) {
	items, err := s.queries.GetFlowTagsByTagID(ctx, tagID)
	if err != nil {
		return []mflowtag.FlowTag{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
	}
	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}

func (s *FlowTagService) CreateFlowTag(ctx context.Context, ftag mflowtag.FlowTag) error {
	arg := ConvertModelToDB(ftag)
	err := s.queries.CreateFlowTag(ctx, gen.CreateFlowTagParams(arg))
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
}

func (s *FlowTagService) DeleteFlowTag(ctx context.Context, id idwrap.IDWrap) error {
	err := s.queries.DeleteFlowTag(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowTag, err)
}
