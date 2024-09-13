package sexamplebodyform

import (
	"context"
	"dev-tools-backend/pkg/model/mexamplebodyform"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/ulidwrap"
	"dev-tools-db/pkg/sqlc/gen"
	"slices"
)

type BodyFormService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) *BodyFormService {
	return &BodyFormService{queries: queries}
}

func NewTX(queries *gen.Queries) *BodyFormService {
	return &BodyFormService{queries: queries}
}

func SerializeBodyFormDBToModel(bodyForm gen.ExampleBodyForm) mexamplebodyform.BodyForm {
	return mexamplebodyform.BodyForm{
		ID:          ulidwrap.New(bodyForm.ID),
		ExampleID:   ulidwrap.New(bodyForm.ExampleID),
		BodyKey:     bodyForm.BodyKey,
		Enable:      bodyForm.Enable,
		Description: bodyForm.Description,
		Value:       bodyForm.Value,
	}
}

func SerializeBodyFormModelToDB(bodyForm mexamplebodyform.BodyForm) gen.ExampleBodyForm {
	return gen.ExampleBodyForm{
		ID:          bodyForm.ID.GetUlid(),
		ExampleID:   bodyForm.ExampleID.GetUlid(),
		BodyKey:     bodyForm.BodyKey,
		Enable:      bodyForm.Enable,
		Description: bodyForm.Description,
		Value:       bodyForm.Value,
	}
}

func (s BodyFormService) GetBodyFormByID(ctx context.Context, id ulidwrap.ULIDWrap) (mexamplebodyform.BodyForm, error) {
	bodyForm, err := s.queries.GetBodyForm(ctx, id.GetUlid())
	if err != nil {
		return mexamplebodyform.BodyForm{}, err
	}
	return SerializeBodyFormDBToModel(bodyForm), nil
}

func (s BodyFormService) GetBodyFormByExampleID(ctx context.Context, exampleID ulidwrap.ULIDWrap) ([]mexamplebodyform.BodyForm, error) {
	bodyForms, err := s.queries.GetBodyFormsByExampleID(ctx, exampleID.GetUlid())
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(bodyForms, SerializeBodyFormDBToModel), nil
}

func (s BodyFormService) CreateBodyForm(ctx context.Context, bodyForm mexamplebodyform.BodyForm) error {
	arg := gen.CreateBodyFormParams{
		ID:          bodyForm.ID.GetUlid(),
		ExampleID:   bodyForm.ExampleID.GetUlid(),
		BodyKey:     bodyForm.BodyKey,
		Enable:      bodyForm.Enable,
		Description: bodyForm.Description,
		Value:       bodyForm.Value,
	}
	return s.queries.CreateBodyForm(ctx, arg)
}

func (s BodyFormService) CreateBodyFormDBModel(ctx context.Context, bodyForm gen.ExampleBodyForm) error {
	arg := gen.CreateBodyFormParams{
		ID:          bodyForm.ID,
		ExampleID:   bodyForm.ExampleID,
		BodyKey:     bodyForm.BodyKey,
		Enable:      bodyForm.Enable,
		Description: bodyForm.Description,
		Value:       bodyForm.Value,
	}
	return s.queries.CreateBodyForm(ctx, arg)
}

func (s BodyFormService) CreateBulkHeader(ctx context.Context, bodyForms []mexamplebodyform.BodyForm) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyForms, SerializeBodyFormModelToDB)
	for bodyFormChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(bodyFormChunk) < sizeOfChunks {
			for _, bodyForm := range bodyFormChunk {
				err := s.CreateBodyFormDBModel(ctx, bodyForm)
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
		if err := s.queries.CreateBodyFormBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (s BodyFormService) UpdateBodyForm(ctx context.Context, bodyForm mexamplebodyform.BodyForm) error {
	arg := gen.UpdateBodyFormParams{
		ID:          bodyForm.ID.GetUlid(),
		BodyKey:     bodyForm.BodyKey,
		Enable:      bodyForm.Enable,
		Description: bodyForm.Description,
		Value:       bodyForm.Value,
	}
	return s.queries.UpdateBodyForm(ctx, arg)
}

func (s BodyFormService) DeleteBodyForm(ctx context.Context, id ulidwrap.ULIDWrap) error {
	return s.queries.DeleteBodyForm(ctx, id.GetUlid())
}
