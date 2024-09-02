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
	for _, itemApiExample := range itemApiExamples {
		itemApiExamplesList = append(itemApiExamplesList, mitemapiexample.ItemApiExample{
			ID:           itemApiExample.ID,
			ItemApiID:    itemApiExample.ItemApiID,
			CollectionID: itemApiExample.CollectionID,
			Default:      itemApiExample.Default,
			Name:         itemApiExample.Name,
			Headers:      itemApiExample.Headers,
			Query:        itemApiExample.Query,
			Body:         itemApiExample.Body,
			Created:      itemApiExample.Created,
			Updated:      itemApiExample.Updated,
		})
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
		Default:      itemApiExample.Default,
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
		Name:         itemApiExample.Name,
		Headers:      itemApiExample.Headers,
		Query:        itemApiExample.Query,
		Body:         itemApiExample.Body,
		Created:      itemApiExample.Created,
		Updated:      itemApiExample.Updated,
	}, nil
}

func (iaes ItemApiExampleService) CreateApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		Name:         item.Name,
		Headers:      item.Headers,
		Query:        item.Query,
		Body:         item.Body,
	})
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
