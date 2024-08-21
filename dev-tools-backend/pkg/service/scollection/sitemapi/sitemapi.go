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
	itemApi, err := ias.queries.GetItemApi(ctx, id.Bytes())
	if err != nil {
		return nil, err
	}
	ParentID := ulid.ULID(itemApi.ParentID)
	var h mitemapi.Headers
	h.HeaderMap = make(map[string]string)
	h.Scan(itemApi.Headers)

	var q mitemapi.Query
	q.QueryMap = make(map[string]string)
	q.Scan(itemApi.Query)

	return &mitemapi.ItemApi{
		ID:           ulid.ULID(itemApi.ID),
		CollectionID: ulid.ULID(itemApi.CollectionID),
		ParentID:     &ParentID,
		Name:         itemApi.Name,
		Url:          itemApi.Url,
		Method:       itemApi.Method,
		Headers:      h,
		Query:        q,
		Body:         itemApi.Body,
	}, nil
}

func (ias ItemApiService) CreateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	_, err := ias.queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           item.ID.Bytes(),
		CollectionID: item.CollectionID.Bytes(),
		ParentID:     item.ParentID.Bytes(),
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
		Headers:      item.Headers.Bytes(),
		Query:        item.Query.Bytes(),
		Body:         item.Body,
	})
	return err
}

func (ias ItemApiService) UpdateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	return ias.queries.UpdateItemApi(ctx, gen.UpdateItemApiParams{
		ID:           item.ID.Bytes(),
		CollectionID: item.CollectionID.Bytes(),
		ParentID:     item.ParentID.Bytes(),
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
		Headers:      item.Headers.Bytes(),
		Query:        item.Query.Bytes(),
		Body:         item.Body,
	})
}

func (ias ItemApiService) DeleteItemApi(ctx context.Context, id ulid.ULID) error {
	return ias.queries.DeleteItemApi(ctx, id.Bytes())
}

func (ias ItemApiService) GetApisWithCollectionID(ctx context.Context, collectionID ulid.ULID) ([]mitemapi.ItemApi, error) {
	itemApis, err := ias.queries.GetItemsApiByCollectionID(ctx, collectionID.Bytes())
	if err != nil {
		return nil, err
	}

	items := []mitemapi.ItemApi{}
	for _, itemApi := range itemApis {
		ParentID := ulid.ULID(itemApi.ParentID)
		var h mitemapi.Headers
		h.HeaderMap = make(map[string]string)
		err = h.Scan(itemApi.Headers)
		if err != nil {
			return nil, err
		}
		q := mitemapi.Query{}
		q.QueryMap = make(map[string]string)
		err = q.Scan(itemApi.Query)
		if err != nil {
			return nil, err
		}

		items = append(items, mitemapi.ItemApi{
			ID:           ulid.ULID(itemApi.ID),
			CollectionID: ulid.ULID(itemApi.CollectionID),
			ParentID:     &ParentID,
			Name:         itemApi.Name,
			Url:          itemApi.Url,
			Method:       itemApi.Method,
			Headers:      h,
			Query:        q,
			Body:         itemApi.Body,
		})
	}
	return items, nil
}

func (ias ItemApiService) GetOwnerID(ctx context.Context, id ulid.ULID) (ulid.ULID, error) {
	rawUlid, err := ias.queries.GetItemApiOwnerID(ctx, id.Bytes())
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
