package sitemapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mcollection/mitemapi"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type ItemApiService struct {
	DB      *sql.DB
	queries *gen.Queries
}

func New(ctx context.Context, db *sql.DB) (*ItemApiService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}

	return &ItemApiService{
		DB:      db,
		queries: queries,
	}, nil
}

func (ias ItemApiService) GetItemApi(ctx context.Context, id ulid.ULID) (*mitemapi.ItemApi, error) {
	itemApi, err := ias.queries.GetItemApi(ctx, id)
	if err != nil {
		return nil, err
	}

	return &mitemapi.ItemApi{
		ID:           itemApi.ID,
		CollectionID: itemApi.CollectionID,
		ParentID:     itemApi.ParentID,
		Name:         itemApi.Name,
		Url:          itemApi.Url,
		Method:       itemApi.Method,
		Headers:      itemApi.Headers,
		Query:        itemApi.Query,
		Body:         itemApi.Body,
	}, nil
}

func (ias ItemApiService) CreateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	_, err := ias.queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           item.ID,
		CollectionID: item.CollectionID,
		ParentID:     item.ParentID,
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
		Headers:      item.Headers,
		Query:        item.Query,
		Body:         item.Body,
	})
	return err
}

func (ias ItemApiService) UpdateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	return ias.queries.UpdateItemApi(ctx, gen.UpdateItemApiParams{
		ID:           item.ID,
		CollectionID: item.CollectionID,
		ParentID:     item.ParentID,
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
		Headers:      item.Headers,
		Query:        item.Query,
		Body:         item.Body,
	})
}

func (ias ItemApiService) DeleteItemApi(ctx context.Context, id ulid.ULID) error {
	return ias.queries.DeleteItemApi(ctx, id)
}

func (ias ItemApiService) GetApisWithCollectionID(ctx context.Context, collectionID ulid.ULID) ([]mitemapi.ItemApi, error) {
	itemApis, err := ias.queries.GetItemsApiByCollectionID(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	items := []mitemapi.ItemApi{}
	for _, itemApi := range itemApis {
		items = append(items, mitemapi.ItemApi{
			ID:           ulid.ULID(itemApi.ID),
			CollectionID: ulid.ULID(itemApi.CollectionID),
			ParentID:     itemApi.ParentID,
			Name:         itemApi.Name,
			Url:          itemApi.Url,
			Method:       itemApi.Method,
			Headers:      itemApi.Headers,
			Query:        itemApi.Query,
			Body:         itemApi.Body,
		})
	}
	return items, nil
}

func (ias ItemApiService) GetOwnerID(ctx context.Context, id ulid.ULID) (ulid.ULID, error) {
	rawUlid, err := ias.queries.GetItemApiOwnerID(ctx, id)
	if err != nil {
		return ulid.ULID{}, err
	}
	ownerID := ulid.ULID(rawUlid)
	return ownerID, err
}

func (ias ItemApiService) CheckOwnerID(ctx context.Context, id ulid.ULID, ownerID ulid.ULID) (bool, error) {
	collectionOwnerID, err := ias.GetOwnerID(ctx, id)
	if err != nil {
		return false, err
	}
	return ownerID.Compare(collectionOwnerID) == 0, nil
}
