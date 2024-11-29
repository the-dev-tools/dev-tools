package sitemapiexample

import (
	"context"
	"database/sql"
	"slices"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type ItemApiExampleService struct {
	Queries *gen.Queries
}

var ErrNoItemApiExampleFound = sql.ErrNoRows

func New(queries *gen.Queries) ItemApiExampleService {
	return ItemApiExampleService{Queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ItemApiExampleService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ItemApiExampleService{
		Queries: queries,
	}, nil
}

func MassConvert[T any, O any](item []T, convFunc func(T) *O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = *convFunc(v)
	}
	return arr
}

func ConvertToDBItem(item mitemapiexample.ItemApiExample) gen.ItemApiExample {
	// TODO: add headers and query
	return gen.ItemApiExample{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		IsDefault:    item.IsDefault,
		BodyType:     int8(item.BodyType),
		Name:         item.Name,
		Prev:         item.Prev,
		Next:         item.Next,
	}
}

func ConvertToModelItem(item gen.ItemApiExample) *mitemapiexample.ItemApiExample {
	return &mitemapiexample.ItemApiExample{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		IsDefault:    item.IsDefault,
		BodyType:     mitemapiexample.BodyType(item.BodyType),
		Name:         item.Name,
		Prev:         item.Prev,
		Next:         item.Next,
	}
}

func (iaes ItemApiExampleService) GetApiExamples(ctx context.Context, apiUlid idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExamples(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetDefaultApiExample(ctx context.Context, apiUlid idwrap.IDWrap) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExampleDefault(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}

	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetApiExample(ctx context.Context, id idwrap.IDWrap) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExample(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetApiExampleByCollection(ctx context.Context, collectionID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExampleByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) CreateApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		IsDefault:    item.IsDefault,
		BodyType:     int8(item.BodyType),
		Name:         item.Name,
		Prev:         item.Prev,
		Next:         item.Next,
	})
}

func (iaes ItemApiExampleService) CreateApiExampleBulk(ctx context.Context, items []mitemapiexample.ItemApiExample) error {
	sizeOfChunks := 10

	for chunk := range slices.Chunk(items, sizeOfChunks) {
		if len(chunk) < sizeOfChunks {
			for _, item := range chunk {
				err := iaes.CreateApiExample(ctx, &item)
				if err != nil {
					return err
				}
			}
			continue
		}
		item1 := chunk[0]
		item2 := chunk[1]
		item3 := chunk[2]
		item4 := chunk[3]
		item5 := chunk[4]
		item6 := chunk[5]
		item7 := chunk[6]
		item8 := chunk[7]
		item9 := chunk[8]
		item10 := chunk[9]

		err := iaes.Queries.CreateItemApiExampleBulk(ctx, gen.CreateItemApiExampleBulkParams{
			// Item 1
			ID:           item1.ID,
			ItemApiID:    item1.ItemApiID,
			CollectionID: item1.CollectionID,
			IsDefault:    item1.IsDefault,
			BodyType:     int8(item1.BodyType),
			Name:         item1.Name,
			Prev:         item1.Prev,
			Next:         item1.Next,
			// Item 2
			ID_2:           item2.ID,
			ItemApiID_2:    item2.ItemApiID,
			CollectionID_2: item2.CollectionID,
			IsDefault_2:    item2.IsDefault,
			BodyType_2:     int8(item2.BodyType),
			Name_2:         item2.Name,
			Prev_2:         item2.Prev,
			Next_2:         item2.Next,
			// Item 3
			ID_3:           item3.ID,
			ItemApiID_3:    item3.ItemApiID,
			CollectionID_3: item3.CollectionID,
			IsDefault_3:    item3.IsDefault,
			BodyType_3:     int8(item3.BodyType),
			Name_3:         item3.Name,
			Prev_3:         item3.Prev,
			Next_3:         item3.Next,
			// Item 4
			ID_4:           item4.ID,
			ItemApiID_4:    item4.ItemApiID,
			CollectionID_4: item4.CollectionID,
			IsDefault_4:    item4.IsDefault,
			BodyType_4:     int8(item4.BodyType),
			Name_4:         item4.Name,
			Prev_4:         item4.Prev,
			Next_4:         item4.Next,
			// Item 5
			ID_5:           item5.ID,
			ItemApiID_5:    item5.ItemApiID,
			CollectionID_5: item5.CollectionID,
			IsDefault_5:    item5.IsDefault,
			BodyType_5:     int8(item5.BodyType),
			Name_5:         item5.Name,
			Prev_5:         item5.Prev,
			Next_5:         item5.Next,
			// Item 6
			ID_6:           item6.ID,
			ItemApiID_6:    item6.ItemApiID,
			CollectionID_6: item6.CollectionID,
			IsDefault_6:    item6.IsDefault,
			BodyType_6:     int8(item6.BodyType),
			Name_6:         item6.Name,
			Prev_6:         item6.Prev,
			Next_6:         item6.Next,
			// Item 7
			ID_7:           item7.ID,
			ItemApiID_7:    item7.ItemApiID,
			CollectionID_7: item7.CollectionID,
			IsDefault_7:    item7.IsDefault,
			BodyType_7:     int8(item7.BodyType),
			Name_7:         item7.Name,
			Prev_7:         item7.Prev,
			Next_7:         item7.Next,
			// Item 8
			ID_8:           item8.ID,
			ItemApiID_8:    item8.ItemApiID,
			CollectionID_8: item8.CollectionID,
			IsDefault_8:    item8.IsDefault,
			BodyType_8:     int8(item8.BodyType),
			Name_8:         item8.Name,
			Prev_8:         item8.Prev,
			Next_8:         item8.Next,
			// Item 9
			ID_9:           item9.ID,
			ItemApiID_9:    item9.ItemApiID,
			CollectionID_9: item9.CollectionID,
			IsDefault_9:    item9.IsDefault,
			BodyType_9:     int8(item9.BodyType),
			Name_9:         item9.Name,
			Prev_9:         item9.Prev,
			Next_9:         item9.Next,
			// Item 10
			ID_10:           item10.ID,
			ItemApiID_10:    item10.ItemApiID,
			CollectionID_10: item10.CollectionID,
			IsDefault_10:    item10.IsDefault,
			BodyType_10:     int8(item10.BodyType),
			Name_10:         item10.Name,
			Prev_10:         item10.Prev,
			Next_10:         item10.Next,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (iaes ItemApiExampleService) UpdateItemApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.UpdateItemApiExample(ctx, gen.UpdateItemApiExampleParams{
		ID:       item.ID,
		Name:     item.Name,
		BodyType: int8(item.BodyType),
	})
}

func (iaes ItemApiExampleService) DeleteApiExample(ctx context.Context, id idwrap.IDWrap) error {
	return iaes.Queries.DeleteItemApiExample(ctx, id)
}
