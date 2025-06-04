package sbodyform

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type BodyFormService struct {
	queries *gen.Queries
}

var ErrNoBodyFormFound = errors.New("no form body found")

func SeralizeModeltoGen(body mbodyform.BodyForm) gen.ExampleBodyForm {
	return gen.ExampleBodyForm{
		ID:            body.ID,
		ExampleID:     body.ExampleID,
		DeltaParentID: body.DeltaParentID,
		BodyKey:       body.BodyKey,
		Description:   body.Description,
		Enable:        body.Enable,
		Value:         body.Value,
		Source:        int8(body.Source),
	}
}

func DeserializeGenToModel(body gen.ExampleBodyForm) mbodyform.BodyForm {
	return mbodyform.BodyForm{
		ID:            body.ID,
		ExampleID:     body.ExampleID,
		DeltaParentID: body.DeltaParentID,
		BodyKey:       body.BodyKey,
		Description:   body.Description,
		Enable:        body.Enable,
		Value:         body.Value,
		Source:        mbodyform.BodyFormSource(body.Source),
	}
}

func New(queries *gen.Queries) BodyFormService {
	return BodyFormService{queries: queries}
}

func (bfs BodyFormService) TX(tx *sql.Tx) BodyFormService {
	return BodyFormService{queries: bfs.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*BodyFormService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := BodyFormService{queries: queries}
	return &service, nil
}

func (bfs BodyFormService) GetBodyForm(ctx context.Context, id idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	bodyForm, err := bfs.queries.GetBodyForm(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyFormFound
		}
		return nil, err
	}
	body := DeserializeGenToModel(bodyForm)
	return &body, nil
}

func (bfs BodyFormService) CreateBulkBodyForm(ctx context.Context, bodyForms []mbodyform.BodyForm) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyForms, SeralizeModeltoGen)
	for bodyFormChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(bodyFormChunk) < sizeOfChunks {
			for _, bodyForm := range bodyFormChunk {
				err := bfs.CreateBodyFormRaw(ctx, bodyForm)
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

		params := gen.CreateBodyFormBulkParams{
			// 1
			ID:            item1.ID,
			ExampleID:     item1.ExampleID,
			DeltaParentID: item1.DeltaParentID,
			BodyKey:       item1.BodyKey,
			Enable:        item1.Enable,
			Description:   item1.Description,
			Value:         item1.Value,
			Source:        item1.Source,
			// 2
			ID_2:            item2.ID,
			ExampleID_2:     item2.ExampleID,
			DeltaParentID_2: item2.DeltaParentID,
			BodyKey_2:       item2.BodyKey,
			Enable_2:        item2.Enable,
			Description_2:   item2.Description,
			Value_2:         item2.Value,
			Source_2:        item2.Source,
			// 3
			ID_3:            item3.ID,
			ExampleID_3:     item3.ExampleID,
			DeltaParentID_3: item3.DeltaParentID,
			BodyKey_3:       item3.BodyKey,
			Enable_3:        item3.Enable,
			Description_3:   item3.Description,
			Value_3:         item3.Value,
			Source_3:        item3.Source,
			// 4
			ID_4:            item4.ID,
			ExampleID_4:     item4.ExampleID,
			DeltaParentID_4: item4.DeltaParentID,
			BodyKey_4:       item4.BodyKey,
			Enable_4:        item4.Enable,
			Description_4:   item4.Description,
			Value_4:         item4.Value,
			Source_4:        item4.Source,
			// 5
			ID_5:            item5.ID,
			ExampleID_5:     item5.ExampleID,
			DeltaParentID_5: item5.DeltaParentID,
			BodyKey_5:       item5.BodyKey,
			Enable_5:        item5.Enable,
			Description_5:   item5.Description,
			Value_5:         item5.Value,
			Source_5:        item5.Source,
			// 6
			ID_6:            item6.ID,
			ExampleID_6:     item6.ExampleID,
			DeltaParentID_6: item6.DeltaParentID,
			BodyKey_6:       item6.BodyKey,
			Enable_6:        item6.Enable,
			Description_6:   item6.Description,
			Value_6:         item6.Value,
			Source_6:        item6.Source,
			// 7
			ID_7:            item7.ID,
			ExampleID_7:     item7.ExampleID,
			DeltaParentID_7: item7.DeltaParentID,
			BodyKey_7:       item7.BodyKey,
			Enable_7:        item7.Enable,
			Description_7:   item7.Description,
			Value_7:         item7.Value,
			Source_7:        item7.Source,
			// 8
			ID_8:            item8.ID,
			ExampleID_8:     item8.ExampleID,
			DeltaParentID_8: item8.DeltaParentID,
			BodyKey_8:       item8.BodyKey,
			Enable_8:        item8.Enable,
			Description_8:   item8.Description,
			Value_8:         item8.Value,
			Source_8:        item8.Source,
			// 9
			ID_9:            item9.ID,
			ExampleID_9:     item9.ExampleID,
			DeltaParentID_9: item9.DeltaParentID,
			BodyKey_9:       item9.BodyKey,
			Enable_9:        item9.Enable,
			Description_9:   item9.Description,
			Value_9:         item9.Value,
			Source_9:        item9.Source,
			// 10
			ID_10:            item10.ID,
			ExampleID_10:     item10.ExampleID,
			DeltaParentID_10: item10.DeltaParentID,
			BodyKey_10:       item10.BodyKey,
			Enable_10:        item10.Enable,
			Description_10:   item10.Description,
			Value_10:         item10.Value,
			Source_10:        item10.Source,
		}
		if err := bfs.queries.CreateBodyFormBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (bfs BodyFormService) GetBodyFormsByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mbodyform.BodyForm, error) {
	bodyForms, err := bfs.queries.GetBodyFormsByExampleID(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyFormFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(bodyForms, DeserializeGenToModel), nil
}

func (bfs BodyFormService) CreateBodyForm(ctx context.Context, body *mbodyform.BodyForm) error {
	bf := SeralizeModeltoGen(*body)
	return bfs.queries.CreateBodyForm(ctx, gen.CreateBodyFormParams{
		ID:            bf.ID,
		ExampleID:     bf.ExampleID,
		DeltaParentID: bf.DeltaParentID,
		BodyKey:       bf.BodyKey,
		Description:   bf.Description,
		Enable:        bf.Enable,
		Value:         bf.Value,
		Source:        bf.Source,
	})
}

func (bfs BodyFormService) CreateBodyFormRaw(ctx context.Context, bf gen.ExampleBodyForm) error {
	return bfs.queries.CreateBodyForm(ctx, gen.CreateBodyFormParams{
		ID:            bf.ID,
		ExampleID:     bf.ExampleID,
		DeltaParentID: bf.DeltaParentID,
		BodyKey:       bf.BodyKey,
		Description:   bf.Description,
		Enable:        bf.Enable,
		Value:         bf.Value,
		Source:        bf.Source,
	})
}

func (bfs BodyFormService) UpdateBodyForm(ctx context.Context, body *mbodyform.BodyForm) error {
	bf := SeralizeModeltoGen(*body)
	return bfs.queries.UpdateBodyForm(ctx, gen.UpdateBodyFormParams{
		ID:          bf.ID,
		BodyKey:     bf.BodyKey,
		Description: bf.Description,
		Enable:      bf.Enable,
		Value:       bf.Value,
		Source:      bf.Source,
	})
}

func (bfs BodyFormService) DeleteBodyForm(ctx context.Context, id idwrap.IDWrap) error {
	return bfs.queries.DeleteBodyForm(ctx, id)
}

func (bfs BodyFormService) ResetBodyFormDelta(ctx context.Context, id idwrap.IDWrap) error {
	bodyForm, err := bfs.GetBodyForm(ctx, id)
	if err != nil {
		return err
	}

	bodyForm.DeltaParentID = nil
	bodyForm.BodyKey = ""
	bodyForm.Enable = false
	bodyForm.Description = ""
	bodyForm.Value = ""
	bodyForm.Source = mbodyform.BodyFormSourceOrigin

	return bfs.UpdateBodyForm(ctx, bodyForm)
}

func (bfs BodyFormService) GetBodyFormsByDeltaParentID(ctx context.Context, deltaParentID idwrap.IDWrap) ([]mbodyform.BodyForm, error) {
	bodyForms, err := bfs.queries.GetBodyFormsByDeltaParentID(ctx, &deltaParentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mbodyform.BodyForm{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvert(bodyForms, DeserializeGenToModel), nil
}
