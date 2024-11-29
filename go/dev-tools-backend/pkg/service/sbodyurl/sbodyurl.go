package sbodyurl

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyurl"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
	"slices"
)

type BodyURLEncodedService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) BodyURLEncodedService {
	return BodyURLEncodedService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*BodyURLEncodedService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := BodyURLEncodedService{queries: queries}
	return &service, nil
}

func SeralizeModeltoGen(body mbodyurl.BodyURLEncoded) gen.ExampleBodyUrlencoded {
	return gen.ExampleBodyUrlencoded{
		ID:          body.ID,
		ExampleID:   body.ExampleID,
		BodyKey:     body.BodyKey,
		Description: body.Description,
		Enable:      body.Enable,
		Value:       body.Value,
	}
}

func DeserializeGenToModel(body gen.ExampleBodyUrlencoded) mbodyurl.BodyURLEncoded {
	return mbodyurl.BodyURLEncoded{
		ID:          body.ID,
		ExampleID:   body.ExampleID,
		BodyKey:     body.BodyKey,
		Description: body.Description,
		Enable:      body.Enable,
		Value:       body.Value,
	}
}

func (bues BodyURLEncodedService) GetBodyURLEncoded(ctx context.Context, id idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	bodyURLEncoded, err := bues.queries.GetBodyUrlEncoded(ctx, id)
	if err != nil {
		return nil, err
	}
	body := DeserializeGenToModel(bodyURLEncoded)
	return &body, nil
}

func (bues BodyURLEncodedService) GetBodyURLEncodedByExampleID(ctx context.Context, id idwrap.IDWrap) ([]mbodyurl.BodyURLEncoded, error) {
	bodyURLEncoded, err := bues.queries.GetBodyUrlEncodedsByExampleID(ctx, id)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(bodyURLEncoded, DeserializeGenToModel), nil
}

func (bues BodyURLEncodedService) CreateBodyURLEncoded(ctx context.Context, body *mbodyurl.BodyURLEncoded) error {
	bue := SeralizeModeltoGen(*body)
	return bues.queries.CreateBodyUrlEncoded(ctx, gen.CreateBodyUrlEncodedParams{
		ID:          bue.ID,
		ExampleID:   bue.ExampleID,
		BodyKey:     bue.BodyKey,
		Description: bue.Description,
		Enable:      bue.Enable,
		Value:       bue.Value,
	})
}

func (bues BodyURLEncodedService) CreateBodyFormRaw(ctx context.Context, bodyForm gen.ExampleBodyUrlencoded) error {
	return bues.queries.CreateBodyUrlEncoded(ctx, gen.CreateBodyUrlEncodedParams{
		ID:          bodyForm.ID,
		ExampleID:   bodyForm.ExampleID,
		BodyKey:     bodyForm.BodyKey,
		Description: bodyForm.Description,
		Enable:      bodyForm.Enable,
		Value:       bodyForm.Value,
	})
}

func (bues BodyURLEncodedService) CreateBulkBodyURLEncoded(ctx context.Context, bodyForms []mbodyurl.BodyURLEncoded) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyForms, SeralizeModeltoGen)
	for bodyFormChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(bodyFormChunk) < sizeOfChunks {
			for _, bodyForm := range bodyFormChunk {
				err := bues.CreateBodyFormRaw(ctx, bodyForm)
				if err != nil {
					return err
				}
			}
			continue
		}
		item1 := bodyFormChunk[0]
		item2 := bodyFormChunk[1]
		item3 := bodyFormChunk[2]
		item4 := bodyFormChunk[3]
		item5 := bodyFormChunk[4]
		item6 := bodyFormChunk[5]
		item7 := bodyFormChunk[6]
		item8 := bodyFormChunk[7]
		item9 := bodyFormChunk[8]
		item10 := bodyFormChunk[9]

		params := gen.CreateBodyUrlEncodedBulkParams{
			// 1
			ID:          item1.ID,
			ExampleID:   item1.ExampleID,
			BodyKey:     item1.BodyKey,
			Enable:      item1.Enable,
			Description: item1.Description,
			Value:       item1.Value,
			// 2
			ID_2:          item2.ID,
			ExampleID_2:   item2.ExampleID,
			BodyKey_2:     item2.BodyKey,
			Enable_2:      item2.Enable,
			Description_2: item2.Description,
			Value_2:       item2.Value,
			// 3
			ID_3:          item3.ID,
			ExampleID_3:   item3.ExampleID,
			BodyKey_3:     item3.BodyKey,
			Enable_3:      item3.Enable,
			Description_3: item3.Description,
			Value_3:       item3.Value,
			// 4
			ID_4:          item4.ID,
			ExampleID_4:   item4.ExampleID,
			BodyKey_4:     item4.BodyKey,
			Enable_4:      item4.Enable,
			Description_4: item4.Description,
			Value_4:       item4.Value,
			// 5
			ID_5:          item5.ID,
			ExampleID_5:   item5.ExampleID,
			BodyKey_5:     item5.BodyKey,
			Enable_5:      item5.Enable,
			Description_5: item5.Description,
			Value_5:       item5.Value,
			// 6
			ID_6:          item6.ID,
			ExampleID_6:   item6.ExampleID,
			BodyKey_6:     item6.BodyKey,
			Enable_6:      item6.Enable,
			Description_6: item6.Description,
			Value_6:       item6.Value,
			// 7
			ID_7:          item7.ID,
			ExampleID_7:   item7.ExampleID,
			BodyKey_7:     item7.BodyKey,
			Enable_7:      item7.Enable,
			Description_7: item7.Description,
			Value_7:       item7.Value,
			// 8
			ID_8:          item8.ID,
			ExampleID_8:   item8.ExampleID,
			BodyKey_8:     item8.BodyKey,
			Enable_8:      item8.Enable,
			Description_8: item8.Description,
			Value_8:       item8.Value,
			// 9
			ID_9:          item9.ID,
			ExampleID_9:   item9.ExampleID,
			BodyKey_9:     item9.BodyKey,
			Enable_9:      item9.Enable,
			Description_9: item9.Description,
			Value_9:       item9.Value,
			// 10
			ID_10:          item10.ID,
			ExampleID_10:   item10.ExampleID,
			BodyKey_10:     item10.BodyKey,
			Enable_10:      item10.Enable,
			Description_10: item10.Description,
			Value_10:       item10.Value,
		}
		if err := bues.queries.CreateBodyUrlEncodedBulk(ctx, params); err != nil {
			return err
		}
	}
	return nil
}

func (bues BodyURLEncodedService) UpdateBodyURLEncoded(ctx context.Context, body *mbodyurl.BodyURLEncoded) error {
	bue := SeralizeModeltoGen(*body)
	return bues.queries.UpdateBodyUrlEncoded(ctx, gen.UpdateBodyUrlEncodedParams{
		ID:          bue.ID,
		BodyKey:     bue.BodyKey,
		Description: bue.Description,
		Enable:      bue.Enable,
		Value:       bue.Value,
	})
}

func (bues BodyURLEncodedService) DeleteBodyURLEncoded(ctx context.Context, id idwrap.IDWrap) error {
	return bues.queries.DeleteBodyURLEncoded(ctx, id)
}
