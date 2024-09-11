package sheader

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
	"slices"

	"github.com/oklog/ulid/v2"
)

var ErrNoHeaderFound = sql.ErrNoRows

type HeaderService struct {
	queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (*HeaderService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	service := HeaderService{queries: queries}
	return &service, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HeaderService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HeaderService{queries: queries}
	return &service, nil
}

func SerializeHeaderModelToDB(header gen.ExampleHeader) mexampleheader.Header {
	return mexampleheader.Header{
		ID:          header.ID,
		ExampleID:   header.ExampleID,
		HeaderKey:   header.HeaderKey,
		Enable:      header.Enable,
		Description: header.Description,
		Value:       header.Value,
	}
}

func SerializeHeaderDBToModel(header mexampleheader.Header) gen.ExampleHeader {
	return gen.ExampleHeader{
		ID:          header.ID,
		ExampleID:   header.ExampleID,
		HeaderKey:   header.HeaderKey,
		Enable:      header.Enable,
		Description: header.Description,
		Value:       header.Value,
	}
}

func (h HeaderService) GetHeaderByExampleID(ctx context.Context, exampleID ulid.ULID) ([]mexampleheader.Header, error) {
	header, err := h.queries.GetHeadersByExampleID(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mexampleheader.Header{}, ErrNoHeaderFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(header, SerializeHeaderModelToDB), nil
}

func (h HeaderService) GetHeaderByID(ctx context.Context, headerID ulid.ULID) (mexampleheader.Header, error) {
	header, err := h.queries.GetHeader(ctx, headerID)
	if err != nil {
		return mexampleheader.Header{}, err
	}
	return SerializeHeaderModelToDB(header), nil
}

func (h HeaderService) CreateHeader(ctx context.Context, header mexampleheader.Header) error {
	return h.queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:          header.ID,
		ExampleID:   header.ExampleID,
		HeaderKey:   header.HeaderKey,
		Enable:      header.Enable,
		Description: header.Description,
		Value:       header.Value,
	})
}

func (h HeaderService) CreateHeaderModel(ctx context.Context, header gen.ExampleHeader) error {
	return h.queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:          header.ID,
		ExampleID:   header.ExampleID,
		HeaderKey:   header.HeaderKey,
		Enable:      header.Enable,
		Description: header.Description,
		Value:       header.Value,
	})
}

func (h HeaderService) CreateBulkHeader(ctx context.Context, headers []mexampleheader.Header) error {
	sizeOfChunks := 10
	convertedItems := tgeneric.MassConvert(headers, SerializeHeaderDBToModel)
	for headerChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(headerChunk) < sizeOfChunks {
			for _, header := range headerChunk {
				err := h.CreateHeaderModel(ctx, header)
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
		item6 := headerChunk[5]
		item7 := headerChunk[6]
		item8 := headerChunk[7]
		item9 := headerChunk[8]
		item10 := headerChunk[9]

		params := gen.CreateHeaderBulkParams{
			// 1
			ID:          item1.ID,
			ExampleID:   item1.ExampleID,
			HeaderKey:   item1.HeaderKey,
			Enable:      item1.Enable,
			Description: item1.Description,
			Value:       item1.Value,
			// 2
			ID_2:          item2.ID,
			ExampleID_2:   item2.ExampleID,
			HeaderKey_2:   item2.HeaderKey,
			Enable_2:      item2.Enable,
			Description_2: item2.Description,
			Value_2:       item2.Value,
			// 3
			ID_3:          item3.ID,
			ExampleID_3:   item3.ExampleID,
			HeaderKey_3:   item3.HeaderKey,
			Enable_3:      item3.Enable,
			Description_3: item3.Description,
			Value_3:       item3.Value,
			// 4
			ID_4:          item4.ID,
			ExampleID_4:   item4.ExampleID,
			HeaderKey_4:   item4.HeaderKey,
			Enable_4:      item4.Enable,
			Description_4: item4.Description,
			Value_4:       item4.Value,
			// 5
			ID_5:          item5.ID,
			ExampleID_5:   item5.ExampleID,
			HeaderKey_5:   item5.HeaderKey,
			Enable_5:      item5.Enable,
			Description_5: item5.Description,
			Value_5:       item5.Value,
			// 6
			ID_6:          item6.ID,
			ExampleID_6:   item6.ExampleID,
			HeaderKey_6:   item6.HeaderKey,
			Enable_6:      item6.Enable,
			Description_6: item6.Description,
			Value_6:       item6.Value,
			// 7
			ID_7:          item7.ID,
			ExampleID_7:   item7.ExampleID,
			HeaderKey_7:   item7.HeaderKey,
			Enable_7:      item7.Enable,
			Description_7: item7.Description,
			Value_7:       item7.Value,
			// 8
			ID_8:          item8.ID,
			ExampleID_8:   item8.ExampleID,
			HeaderKey_8:   item8.HeaderKey,
			Enable_8:      item8.Enable,
			Description_8: item8.Description,
			Value_8:       item8.Value,
			// 9
			ID_9:          item9.ID,
			ExampleID_9:   item9.ExampleID,
			HeaderKey_9:   item9.HeaderKey,
			Enable_9:      item9.Enable,
			Description_9: item9.Description,
			Value_9:       item9.Value,
			// 10
			ID_10:          item10.ID,
			ExampleID_10:   item10.ExampleID,
			HeaderKey_10:   item10.HeaderKey,
			Enable_10:      item10.Enable,
			Description_10: item10.Description,
			Value_10:       item10.Value,
		}
		if err := h.queries.CreateHeaderBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (h HeaderService) UpdateHeader(ctx context.Context, header mexampleheader.Header) error {
	return h.queries.UpdateHeader(ctx, gen.UpdateHeaderParams{
		ID:          header.ID,
		HeaderKey:   header.HeaderKey,
		Enable:      header.Enable,
		Description: header.Description,
		Value:       header.Value,
	})
}

func (h HeaderService) DeleteHeader(ctx context.Context, headerID ulid.ULID) error {
	return h.queries.DeleteHeader(ctx, headerID)
}
