package sitemapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type ItemApiService struct {
	DB      *sql.DB
	queries *gen.Queries
}

var ErrNoItemApiFound = sql.ErrNoRows

func New(ctx context.Context, db *sql.DB) (*ItemApiService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiFound
		}
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
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiFound
		}
		return nil, err
	}

	return &mitemapi.ItemApi{
		ID:           itemApi.ID,
		CollectionID: itemApi.CollectionID,
		ParentID:     itemApi.ParentID,
		Name:         itemApi.Name,
		Url:          itemApi.Url,
		Method:       itemApi.Method,
	}, nil
}

func (ias ItemApiService) CreateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	return ias.queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           item.ID,
		CollectionID: item.CollectionID,
		ParentID:     item.ParentID,
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
	})
}

func (ias ItemApiService) CreateItemApiBulk(ctx context.Context, items []mitemapi.ItemApi) error {
	itemLen := len(items)
	sizeOfChunks := 3
	index := 0
	if itemLen > 2 {
		for {
			item1 := items[index]
			item2 := items[index+1]
			item3 := items[index+2]
			params := gen.CreateItemApiBulkParams{
				ID:             item1.ID,
				CollectionID:   item1.CollectionID,
				ParentID:       item1.ParentID,
				Name:           item1.Name,
				Url:            item1.Url,
				Method:         item1.Method,
				ID_2:           item2.ID,
				CollectionID_2: item2.CollectionID,
				ParentID_2:     item2.ParentID,
				Name_2:         item2.Name,
				Url_2:          item2.Url,
				Method_2:       item2.Method,
				ID_3:           item3.ID,
				CollectionID_3: item3.CollectionID,
				ParentID_3:     item3.ParentID,
				Name_3:         item3.Name,
				Url_3:          item3.Url,
				Method_3:       item3.Method,
			}

			if err := ias.queries.CreateItemApiBulk(ctx, params); err != nil {
				return err
			}

			index += sizeOfChunks
			if index >= itemLen {
				break
			}

		}
	}
	for _, item := range items[index:] {
		err := ias.CreateItemApi(ctx, &item)
		if err != nil {
			return err
		}

	}

	return nil
}

func (ias ItemApiService) UpdateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	err := ias.queries.UpdateItemApi(ctx, gen.UpdateItemApiParams{
		ID:           item.ID,
		CollectionID: item.CollectionID,
		ParentID:     item.ParentID,
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
	})
	if err == sql.ErrNoRows {
		return ErrNoItemApiFound
	}
	return err
}

func (ias ItemApiService) DeleteItemApi(ctx context.Context, id ulid.ULID) error {
	return ias.queries.DeleteItemApi(ctx, id)
}

func (ias ItemApiService) GetApisWithCollectionID(ctx context.Context, collectionID ulid.ULID) ([]mitemapi.ItemApi, error) {
	itemApis, err := ias.queries.GetItemsApiByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiFound
		}
		return nil, err
	}

	items := make([]mitemapi.ItemApi, len(itemApis))
	for i, itemApi := range itemApis {
		items[i] = mitemapi.ItemApi{
			ID:           ulid.ULID(itemApi.ID),
			CollectionID: ulid.ULID(itemApi.CollectionID),
			ParentID:     itemApi.ParentID,
			Name:         itemApi.Name,
			Url:          itemApi.Url,
			Method:       itemApi.Method,
		}
	}
	return items, nil
}

func (ias ItemApiService) GetOwnerID(ctx context.Context, id ulid.ULID) (ulid.ULID, error) {
	rawUlid, err := ias.queries.GetItemApiOwnerID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ulid.ULID{}, ErrNoItemApiFound
		}
		return ulid.ULID{}, err
	}
	ownerID := ulid.ULID(rawUlid)
	return ownerID, err
}

func (ias ItemApiService) CheckOwnerID(ctx context.Context, id ulid.ULID, ownerID ulid.ULID) (bool, error) {
	collectionOwnerID, err := ias.GetOwnerID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoItemApiFound
		}
		return false, err
	}
	return ownerID.Compare(collectionOwnerID) == 0, nil
}
