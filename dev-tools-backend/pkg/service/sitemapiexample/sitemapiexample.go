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

func New(db *sql.DB) *ItemApiExampleService {
	return &ItemApiExampleService{
		DB:      db,
		Queries: gen.New(db),
	}
}

var ErrNoItemApiExampleFound = sql.ErrNoRows

func (iaes ItemApiExampleService) GetItemApiExample(ctx context.Context, id ulid.ULID) (*mitemapiexample.ItemApiExample, error) {
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

func (iaes ItemApiExampleService) CreateItemApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
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

func (iaes ItemApiExampleService) DeleteItemApiExample(ctx context.Context, id ulid.ULID) error {
	return iaes.Queries.DeleteItemApiExample(ctx, id)
}
