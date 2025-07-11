package sexampleheader

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHeaderFound = errors.New("not header found")

type HeaderService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) HeaderService {
	return HeaderService{queries: queries}
}

func (h HeaderService) TX(tx *sql.Tx) HeaderService {
	return HeaderService{queries: h.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HeaderService, error) {
	queries := gen.New(tx)
	headerService := HeaderService{
		queries: queries,
	}

	return &headerService, nil
}

func SerializeHeaderModelToDB(header gen.ExampleHeader) mexampleheader.Header {
	return mexampleheader.Header{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
	}
}

func SerializeHeaderDBToModel(header mexampleheader.Header) gen.ExampleHeader {
	return gen.ExampleHeader{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
	}
}

func (h HeaderService) GetHeaderByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	dbHeaders, err := h.queries.GetHeadersByExampleID(ctx, exampleID)
	if err != nil {
		return nil, err
	}

	var headers []mexampleheader.Header
	for _, dbHeader := range dbHeaders {
		header := SerializeHeaderModelToDB(dbHeader)
		headers = append(headers, header)
	}

	return headers, nil
}

func (h HeaderService) GetHeaderByDeltaParentID(ctx context.Context, deltaParentID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	dbHeader, err := h.queries.GetHeaderByDeltaParentID(ctx, &deltaParentID)
	if err != nil {
		return nil, err
	}

	header := SerializeHeaderModelToDB(dbHeader)
	return []mexampleheader.Header{header}, nil
}

func (h HeaderService) GetHeaderByID(ctx context.Context, headerID idwrap.IDWrap) (mexampleheader.Header, error) {
	dbHeader, err := h.queries.GetHeader(ctx, headerID)
	if err != nil {
		return mexampleheader.Header{}, err
	}

	header := SerializeHeaderModelToDB(dbHeader)
	return header, nil
}

func (h HeaderService) CreateHeader(ctx context.Context, header mexampleheader.Header) error {
	return h.queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
	})
}

func (h HeaderService) CreateHeaderModel(ctx context.Context, header gen.ExampleHeader) error {
	return h.queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
	})
}

func (h HeaderService) CreateBulkHeader(ctx context.Context, headers []mexampleheader.Header) error {
	const sizeOfChunks = 15
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
		item11 := headerChunk[10]
		item12 := headerChunk[11]
		item13 := headerChunk[12]
		item14 := headerChunk[13]
		item15 := headerChunk[14]

		params := gen.CreateHeaderBulkParams{
			// 1
			ID:            item1.ID,
			ExampleID:     item1.ExampleID,
			DeltaParentID: item1.DeltaParentID,
			HeaderKey:     item1.HeaderKey,
			Enable:        item1.Enable,
			Description:   item1.Description,
			Value:         item1.Value,
			// 2
			ID_2:            item2.ID,
			ExampleID_2:     item2.ExampleID,
			DeltaParentID_2: item2.DeltaParentID,
			HeaderKey_2:     item2.HeaderKey,
			Enable_2:        item2.Enable,
			Description_2:   item2.Description,
			Value_2:         item2.Value,
			// 3
			ID_3:            item3.ID,
			ExampleID_3:     item3.ExampleID,
			DeltaParentID_3: item3.DeltaParentID,
			HeaderKey_3:     item3.HeaderKey,
			Enable_3:        item3.Enable,
			Description_3:   item3.Description,
			Value_3:         item3.Value,
			// 4
			ID_4:            item4.ID,
			ExampleID_4:     item4.ExampleID,
			DeltaParentID_4: item4.DeltaParentID,
			HeaderKey_4:     item4.HeaderKey,
			Enable_4:        item4.Enable,
			Description_4:   item4.Description,
			Value_4:         item4.Value,
			// 5
			ID_5:            item5.ID,
			ExampleID_5:     item5.ExampleID,
			DeltaParentID_5: item5.DeltaParentID,
			HeaderKey_5:     item5.HeaderKey,
			Enable_5:        item5.Enable,
			Description_5:   item5.Description,
			Value_5:         item5.Value,
			// 6
			ID_6:            item6.ID,
			ExampleID_6:     item6.ExampleID,
			DeltaParentID_6: item6.DeltaParentID,
			HeaderKey_6:     item6.HeaderKey,
			Enable_6:        item6.Enable,
			Description_6:   item6.Description,
			Value_6:         item6.Value,
			// 7
			ID_7:            item7.ID,
			ExampleID_7:     item7.ExampleID,
			DeltaParentID_7: item7.DeltaParentID,
			HeaderKey_7:     item7.HeaderKey,
			Enable_7:        item7.Enable,
			Description_7:   item7.Description,
			Value_7:         item7.Value,
			// 8
			ID_8:            item8.ID,
			ExampleID_8:     item8.ExampleID,
			DeltaParentID_8: item8.DeltaParentID,
			HeaderKey_8:     item8.HeaderKey,
			Enable_8:        item8.Enable,
			Description_8:   item8.Description,
			Value_8:         item8.Value,
			// 9
			ID_9:            item9.ID,
			ExampleID_9:     item9.ExampleID,
			DeltaParentID_9: item9.DeltaParentID,
			HeaderKey_9:     item9.HeaderKey,
			Enable_9:        item9.Enable,
			Description_9:   item9.Description,
			Value_9:         item9.Value,
			// 10
			ID_10:            item10.ID,
			ExampleID_10:     item10.ExampleID,
			DeltaParentID_10: item10.DeltaParentID,
			HeaderKey_10:     item10.HeaderKey,
			Enable_10:        item10.Enable,
			Description_10:   item10.Description,
			Value_10:         item10.Value,
			// 11
			ID_11:            item11.ID,
			ExampleID_11:     item11.ExampleID,
			DeltaParentID_11: item11.DeltaParentID,
			HeaderKey_11:     item11.HeaderKey,
			Enable_11:        item11.Enable,
			Description_11:   item11.Description,
			Value_11:         item11.Value,
			// 12
			ID_12:            item12.ID,
			ExampleID_12:     item12.ExampleID,
			DeltaParentID_12: item12.DeltaParentID,
			HeaderKey_12:     item12.HeaderKey,
			Enable_12:        item12.Enable,
			Description_12:   item12.Description,
			Value_12:         item12.Value,
			// 13
			ID_13:            item13.ID,
			ExampleID_13:     item13.ExampleID,
			DeltaParentID_13: item13.DeltaParentID,
			HeaderKey_13:     item13.HeaderKey,
			Enable_13:        item13.Enable,
			Description_13:   item13.Description,
			Value_13:         item13.Value,
			// 14
			ID_14:            item14.ID,
			ExampleID_14:     item14.ExampleID,
			DeltaParentID_14: item14.DeltaParentID,
			HeaderKey_14:     item14.HeaderKey,
			Enable_14:        item14.Enable,
			Description_14:   item14.Description,
			Value_14:         item14.Value,
			// 15
			ID_15:            item15.ID,
			ExampleID_15:     item15.ExampleID,
			DeltaParentID_15: item15.DeltaParentID,
			HeaderKey_15:     item15.HeaderKey,
			Enable_15:        item15.Enable,
			Description_15:   item15.Description,
			Value_15:         item15.Value,
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

func (h HeaderService) DeleteHeader(ctx context.Context, headerID idwrap.IDWrap) error {
	return h.queries.DeleteHeader(ctx, headerID)
}

func (h HeaderService) ResetHeaderDelta(ctx context.Context, id idwrap.IDWrap) error {
	header, err := h.GetHeaderByID(ctx, id)
	if err != nil {
		return err
	}

	header.DeltaParentID = nil
	header.HeaderKey = ""
	header.Enable = false
	header.Description = ""
	header.Value = ""

	return h.UpdateHeader(ctx, header)
}
