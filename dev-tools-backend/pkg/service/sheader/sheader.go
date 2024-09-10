package sheader

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"

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

func (h HeaderService) CreateBulkHeader(ctx context.Context, headers []mexampleheader.Header) error {
	itemLen := len(headers)
	sizeOfChunks := 3
	index := 0
	convertedItems := tgeneric.MassConvert(headers, SerializeHeaderDBToModel)

	if itemLen > 2 {
		for {
			item1 := convertedItems[index]
			item2 := convertedItems[index+1]
			item3 := convertedItems[index+2]
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
			}

			if err := h.queries.CreateHeaderBulk(ctx, params); err != nil {
				return err
			}

			index += sizeOfChunks
			if index >= itemLen {
				break
			}

		}
	}
	for _, header := range headers[index:] {
		err := h.CreateHeader(ctx, header)
		if err != nil {
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
