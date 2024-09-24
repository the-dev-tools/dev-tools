package sexamplerespheader

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexamplerespheader"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
)

type ExampleRespHeaderService struct {
	Queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (*ExampleRespHeaderService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	return &ExampleRespHeaderService{
		Queries: queries,
	}, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ExampleRespHeaderService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ExampleRespHeaderService{
		Queries: queries,
	}, nil
}

func ConvertToDBExampleRespHeader(item mexamplerespheader.ExampleRespHeader) gen.ExampleRespHeader {
	return gen.ExampleRespHeader{
		ID:            item.ID,
		ExampleRespID: item.ExampleRespID,
		HeaderKey:     item.HeaderKey,
		Value:         item.Value,
	}
}

func ConvertFromDBExampleRespHeader(item gen.ExampleRespHeader) mexamplerespheader.ExampleRespHeader {
	return mexamplerespheader.ExampleRespHeader{
		ID:            item.ID,
		ExampleRespID: item.ExampleRespID,
		HeaderKey:     item.HeaderKey,
		Value:         item.Value,
	}
}

func (s *ExampleRespHeaderService) CreateExampleRespHeader(ctx context.Context, item mexamplerespheader.ExampleRespHeader) error {
	arg := ConvertToDBExampleRespHeader(item)
	return s.Queries.CreateExampleRespHeader(ctx, gen.CreateExampleRespHeaderParams{
		ID:            arg.ID,
		ExampleRespID: arg.ExampleRespID,
		HeaderKey:     arg.HeaderKey,
		Value:         arg.Value,
	})
}

func (s *ExampleRespHeaderService) GetExampleRespHeader(ctx context.Context, id idwrap.IDWrap) (mexamplerespheader.ExampleRespHeader, error) {
	item, err := s.Queries.GetExampleRespHeader(ctx, id)
	if err != nil {
		return mexamplerespheader.ExampleRespHeader{}, err
	}
	return ConvertFromDBExampleRespHeader(item), nil
}

func (s *ExampleRespHeaderService) GetHeaderByRespID(ctx context.Context, id idwrap.IDWrap) ([]mexamplerespheader.ExampleRespHeader, error) {
	items, err := s.Queries.GetExampleRespHeadersByRespID(ctx, id)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(items, ConvertFromDBExampleRespHeader), nil
}

func (s *ExampleRespHeaderService) UpdateExampleRespHeader(ctx context.Context, item mexamplerespheader.ExampleRespHeader) error {
	arg := ConvertToDBExampleRespHeader(item)
	return s.Queries.UpdateExampleRespHeader(ctx, gen.UpdateExampleRespHeaderParams{
		ID:        arg.ID,
		HeaderKey: arg.HeaderKey,
		Value:     arg.Value,
	})
}

func (s *ExampleRespHeaderService) DeleteExampleRespHeader(ctx context.Context, id idwrap.IDWrap) error {
	return s.Queries.DeleteExampleRespHeader(ctx, id)
}
