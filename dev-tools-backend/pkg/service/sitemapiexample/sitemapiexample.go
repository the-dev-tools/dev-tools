package sitemapiexample

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type ItemApiExampleService struct {
	Queries *gen.Queries
}

var ErrNoItemApiExampleFound = sql.ErrNoRows

func New(ctx context.Context, db *sql.DB) (*ItemApiExampleService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}

	return &ItemApiExampleService{
		Queries: queries,
	}, nil
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
	return gen.ItemApiExample{
		ID:              item.ID,
		ItemApiID:       item.ItemApiID,
		CollectionID:    item.CollectionID,
		ParentExampleID: item.ParentExampleID,
		IsDefault:       item.IsDefault,
		Name:            item.Name,
		Headers:         item.Headers,
		Query:           item.Query,
		Compressed:      item.Compressed,
		Body:            item.Body,
	}
}

func ConvertToModelItem(item gen.ItemApiExample) *mitemapiexample.ItemApiExample {
	return &mitemapiexample.ItemApiExample{
		ID:              item.ID,
		ItemApiID:       item.ItemApiID,
		CollectionID:    item.CollectionID,
		ParentExampleID: item.ParentExampleID,
		IsDefault:       item.IsDefault,
		Name:            item.Name,
		Headers:         item.Headers,
		Query:           item.Query,
		Compressed:      item.Compressed,
		Body:            item.Body,
	}
}

func (iaes ItemApiExampleService) GetApiExamples(ctx context.Context, apiUlid ulid.ULID) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExamples(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetDefaultApiExample(ctx context.Context, apiUlid ulid.ULID) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExampleDefault(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}

	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetApiExample(ctx context.Context, id ulid.ULID) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExample(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetApiExampleByCollection(ctx context.Context, collectionID ulid.ULID) ([]mitemapiexample.ItemApiExample, error) {
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
		ID:              item.ID,
		ItemApiID:       item.ItemApiID,
		CollectionID:    item.CollectionID,
		IsDefault:       item.IsDefault,
		ParentExampleID: item.ParentExampleID,
		Name:            item.Name,
		Headers:         item.Headers,
		Query:           item.Query,
		Compressed:      item.Compressed,
		Body:            item.Body,
	})
}

func (iaes ItemApiExampleService) CreateApiExampleBulk(ctx context.Context, item []mitemapiexample.ItemApiExample) error {
	i := 0
	for {
		if i+3 > len(item) {
			break
		}
		item1 := item[i]
		item2 := item[i+1]
		item3 := item[i+2]

		err := iaes.Queries.CreateItemApiExampleBulk(ctx, gen.CreateItemApiExampleBulkParams{
			// Item 1
			ID:              item1.ID,
			ItemApiID:       item1.ItemApiID,
			CollectionID:    item1.CollectionID,
			ParentExampleID: item1.ParentExampleID,
			IsDefault:       item1.IsDefault,
			Name:            item1.Name,
			Headers:         item1.Headers,
			Query:           item1.Query,
			Compressed:      item1.Compressed,
			Body:            item1.Body,
			// Item 2
			ID_2:              item2.ID,
			ItemApiID_2:       item2.ItemApiID,
			CollectionID_2:    item2.CollectionID,
			ParentExampleID_2: item2.ParentExampleID,
			IsDefault_2:       item2.IsDefault,
			Name_2:            item2.Name,
			Headers_2:         item2.Headers,
			Query_2:           item2.Query,
			Compressed_2:      item2.Compressed,
			Body_2:            item2.Body,
			// Item 3
			ID_3:              item3.ID,
			ItemApiID_3:       item3.ItemApiID,
			CollectionID_3:    item3.CollectionID,
			ParentExampleID_3: item3.ParentExampleID,
			IsDefault_3:       item3.IsDefault,
			Name_3:            item3.Name,
			Headers_3:         item3.Headers,
			Query_3:           item3.Query,
			Compressed_3:      item3.Compressed,
			Body_3:            item3.Body,
		})
		if err != nil {
			return err
		}
		i += 3
	}
	if i < len(item) {
		for _, item := range item[i:] {
			err := iaes.CreateApiExample(ctx, &item)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (iaes ItemApiExampleService) UpdateItemApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.UpdateItemApiExample(ctx, gen.UpdateItemApiExampleParams{
		ID:         item.ID,
		Name:       item.Name,
		Headers:    item.Headers,
		Query:      item.Query,
		Compressed: item.Compressed,
		Body:       item.Body,
	})
}

func (iaes ItemApiExampleService) DeleteApiExample(ctx context.Context, id ulid.ULID) error {
	return iaes.Queries.DeleteItemApiExample(ctx, id)
}
