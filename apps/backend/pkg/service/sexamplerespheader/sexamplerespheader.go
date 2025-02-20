package sexamplerespheader

import (
	"context"
	"database/sql"
	"slices"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mexamplerespheader"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type ExampleRespHeaderService struct {
	queries *gen.Queries
}

var ErrNoRespHeaderFound error = sql.ErrNoRows

func New(queries *gen.Queries) ExampleRespHeaderService {
	return ExampleRespHeaderService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ExampleRespHeaderService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ExampleRespHeaderService{
		queries: queries,
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
	return s.queries.CreateExampleRespHeader(ctx, gen.CreateExampleRespHeaderParams{
		ID:            arg.ID,
		ExampleRespID: arg.ExampleRespID,
		HeaderKey:     arg.HeaderKey,
		Value:         arg.Value,
	})
}

func (s *ExampleRespHeaderService) CreateExampleRespHeaderRaw(ctx context.Context, arg gen.ExampleRespHeader) error {
	return s.queries.CreateExampleRespHeader(ctx, gen.CreateExampleRespHeaderParams{
		ID:            arg.ID,
		ExampleRespID: arg.ExampleRespID,
		HeaderKey:     arg.HeaderKey,
		Value:         arg.Value,
	})
}

func (s *ExampleRespHeaderService) CreateExampleRespHeaderBulk(ctx context.Context, headers []mexamplerespheader.ExampleRespHeader) error {
	const sizeOfChunks = 5
	convertedItems := tgeneric.MassConvert(headers, ConvertToDBExampleRespHeader)
	for headerChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(headerChunk) < sizeOfChunks {
			for _, header := range headerChunk {
				err := s.CreateExampleRespHeaderRaw(ctx, header)
				if err != nil {
					return err
				}
			}
			continue
		}
		item1 := headerChunk[0]
		item2 := headerChunk[1]
		item3 := headerChunk[2]
		item4 := headerChunk[3]
		item5 := headerChunk[4]

		params := gen.CreateExampleRespHeaderBulkParams{
			// 1
			ID:            item1.ID,
			ExampleRespID: item1.ExampleRespID,
			HeaderKey:     item1.HeaderKey,
			Value:         item1.Value,
			// 2
			ID_2:            item2.ID,
			ExampleRespID_2: item2.ExampleRespID,
			HeaderKey_2:     item2.HeaderKey,
			Value_2:         item2.Value,
			// 3
			ID_3:            item3.ID,
			ExampleRespID_3: item3.ExampleRespID,
			HeaderKey_3:     item3.HeaderKey,
			Value_3:         item3.Value,
			// 4
			ID_4:            item4.ID,
			ExampleRespID_4: item4.ExampleRespID,
			HeaderKey_4:     item4.HeaderKey,
			Value_4:         item4.Value,
			// 5
			ID_5:            item5.ID,
			ExampleRespID_5: item5.ExampleRespID,
			HeaderKey_5:     item5.HeaderKey,
			Value_5:         item5.Value,
		}
		if err := s.queries.CreateExampleRespHeaderBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (s *ExampleRespHeaderService) GetExampleRespHeader(ctx context.Context, id idwrap.IDWrap) (mexamplerespheader.ExampleRespHeader, error) {
	item, err := s.queries.GetExampleRespHeader(ctx, id)
	if err != nil {
		return mexamplerespheader.ExampleRespHeader{}, err
	}
	return ConvertFromDBExampleRespHeader(item), nil
}

func (s *ExampleRespHeaderService) GetHeaderByRespID(ctx context.Context, id idwrap.IDWrap) ([]mexamplerespheader.ExampleRespHeader, error) {
	items, err := s.queries.GetExampleRespHeadersByRespID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mexamplerespheader.ExampleRespHeader{}, nil
		}
		return nil, err
	}

	return tgeneric.MassConvert(items, ConvertFromDBExampleRespHeader), nil
}

func (s *ExampleRespHeaderService) UpdateExampleRespHeader(ctx context.Context, item mexamplerespheader.ExampleRespHeader) error {
	arg := ConvertToDBExampleRespHeader(item)
	return s.queries.UpdateExampleRespHeader(ctx, gen.UpdateExampleRespHeaderParams{
		ID:        arg.ID,
		HeaderKey: arg.HeaderKey,
		Value:     arg.Value,
	})
}

func (s *ExampleRespHeaderService) UpdateExampleRespHeaderBulk(ctx context.Context, items []mexamplerespheader.ExampleRespHeader) error {
	var err error
	for _, item := range items {
		err = s.UpdateExampleRespHeader(ctx, item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ExampleRespHeaderService) DeleteExampleRespHeader(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteExampleRespHeader(ctx, id)
}

func (s *ExampleRespHeaderService) DeleteExampleRespHeaderBulk(ctx context.Context, id []idwrap.IDWrap) error {
	for _, item := range id {
		err := s.DeleteExampleRespHeader(ctx, item)
		if err != nil {
			return err
		}
	}
	return nil
}
