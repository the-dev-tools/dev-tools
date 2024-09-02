package sitemapiexample

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type ItemApiExampleService struct {
	DB      *sql.DB
	Queries *gen.Queries
}

var ErrNoItemApiExampleFound = sql.ErrNoRows

func New(ctx context.Context, db *sql.DB) (*ItemApiExampleService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}

	return &ItemApiExampleService{
		DB:      db,
		Queries: queries,
	}, nil
}

func (iaes ItemApiExampleService) GetApiExamples(ctx context.Context, apiUlid ulid.ULID) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExamples(ctx, apiUlid)
	if err != nil {
		return nil, err
	}
	itemApiExamplesList := make([]mitemapiexample.ItemApiExample, len(itemApiExamples))
	for i, itemApiExample := range itemApiExamples {
		itemApiExamplesList[i] = mitemapiexample.ItemApiExample{
			ID:           itemApiExample.ID,
			ItemApiID:    itemApiExample.ItemApiID,
			CollectionID: itemApiExample.CollectionID,
			Default:      itemApiExample.IsDefault,
			Name:         itemApiExample.Name,
			Headers:      itemApiExample.Headers,
			Query:        itemApiExample.Query,
			Body:         itemApiExample.Body,
			Created:      itemApiExample.Created,
			Updated:      itemApiExample.Updated,
		}
	}
	return itemApiExamplesList, nil
}

func (iaes ItemApiExampleService) GetDefaultApiExample(ctx context.Context, apiUlid ulid.ULID) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExampleDefault(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return &mitemapiexample.ItemApiExample{
		ID:           itemApiExample.ID,
		ItemApiID:    itemApiExample.ItemApiID,
		CollectionID: itemApiExample.CollectionID,
		Default:      itemApiExample.IsDefault,
		Name:         itemApiExample.Name,
		Headers:      itemApiExample.Headers,
		Query:        itemApiExample.Query,
		Body:         itemApiExample.Body,
		Created:      itemApiExample.Created,
		Updated:      itemApiExample.Updated,
	}, nil
}

func (iaes ItemApiExampleService) GetApiExample(ctx context.Context, id ulid.ULID) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExample(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return &mitemapiexample.ItemApiExample{
		ID:           itemApiExample.ID,
		ItemApiID:    itemApiExample.ItemApiID,
		CollectionID: itemApiExample.CollectionID,
		Default:      itemApiExample.IsDefault,
		Name:         itemApiExample.Name,
		Headers:      itemApiExample.Headers,
		Query:        itemApiExample.Query,
		Body:         itemApiExample.Body,
		Created:      itemApiExample.Created,
		Updated:      itemApiExample.Updated,
	}, nil
}

func (iaes ItemApiExampleService) GetApiExampleByCollection(ctx context.Context, collectionID ulid.ULID) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExampleByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, nil
		}
		return nil, err
	}
	itemApiExamplesList := make([]mitemapiexample.ItemApiExample, len(itemApiExamples))
	for i, itemApiExample := range itemApiExamples {
		itemApiExamplesList[i] = mitemapiexample.ItemApiExample{
			ID:           itemApiExample.ID,
			ItemApiID:    itemApiExample.ItemApiID,
			CollectionID: itemApiExample.CollectionID,
			Default:      itemApiExample.IsDefault,
			Name:         itemApiExample.Name,
			Headers:      itemApiExample.Headers,
			Query:        itemApiExample.Query,
			Body:         itemApiExample.Body,
			Created:      itemApiExample.Created,
			Updated:      itemApiExample.Updated,
		}
	}
	return itemApiExamplesList, nil
}

func (iaes ItemApiExampleService) CreateApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		IsDefault:    item.Default,
		CollectionID: item.CollectionID,
		Name:         item.Name,
		Headers:      item.Headers,
		Query:        item.Query,
		Body:         item.Body,
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
			ID:             item1.ID,
			ItemApiID:      item1.ItemApiID,
			CollectionID:   item1.CollectionID,
			IsDefault:      item1.Default,
			Name:           item1.Name,
			Headers:        item1.Headers,
			Query:          item1.Query,
			Body:           item1.Body,
			ID_2:           item2.ID,
			ItemApiID_2:    item2.ItemApiID,
			CollectionID_2: item2.CollectionID,
			IsDefault_2:    item2.Default,
			Name_2:         item2.Name,
			Headers_2:      item2.Headers,
			Query_2:        item2.Query,
			Body_2:         item2.Body,
			ID_3:           item3.ID,
			ItemApiID_3:    item3.ItemApiID,
			CollectionID_3: item3.CollectionID,
			IsDefault_3:    item3.Default,
			Name_3:         item3.Name,
			Headers_3:      item3.Headers,
			Query_3:        item3.Query,
			Body_3:         item3.Body,
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
		ID:      item.ID,
		Name:    item.Name,
		Headers: item.Headers,
		Query:   item.Query,
		Body:    item.Body,
	})
}

func (iaes ItemApiExampleService) DeleteApiExample(ctx context.Context, id ulid.ULID) error {
	return iaes.Queries.DeleteItemApiExample(ctx, id)
}
