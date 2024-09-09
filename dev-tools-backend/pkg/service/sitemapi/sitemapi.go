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

func MassConvert[T any, O any](item []T, convFunc func(T) O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = convFunc(v)
	}
	return arr
}

func ConvertToDBItemApi(item mitemapi.ItemApi) gen.ItemApi {
	return gen.ItemApi{
		ID:           item.ID,
		CollectionID: item.CollectionID,
		ParentID:     item.ParentID,
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
		Prev:         item.Prev,
		Next:         item.Next,
	}
}

func ConvertToModelItemApi(item gen.ItemApi) mitemapi.ItemApi {
	return mitemapi.ItemApi{
		ID:           item.ID,
		CollectionID: item.CollectionID,
		ParentID:     item.ParentID,
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
		Prev:         item.Prev,
		Next:         item.Next,
	}
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

func NewTX(ctx context.Context, tx *sql.Tx) (*ItemApiService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ItemApiService{
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

	item := ConvertToModelItemApi(itemApi)
	return &item, nil
}

func (ias ItemApiService) CreateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	itemConverted := ConvertToDBItemApi(*item)
	return ias.queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           itemConverted.ID,
		CollectionID: itemConverted.CollectionID,
		ParentID:     itemConverted.ParentID,
		Name:         itemConverted.Name,
		Url:          itemConverted.Url,
		Method:       itemConverted.Method,
		Prev:         itemConverted.Prev,
		Next:         itemConverted.Next,
	})
}

func (ias ItemApiService) CreateItemApiBulk(ctx context.Context, items []mitemapi.ItemApi) error {
	itemLen := len(items)
	sizeOfChunks := 3
	index := 0
	convertedItems := MassConvert(items, ConvertToDBItemApi)

	if itemLen > 2 {
		for {
			item1 := convertedItems[index]
			item2 := convertedItems[index+1]
			item3 := convertedItems[index+2]
			params := gen.CreateItemApiBulkParams{
				// 1
				ID:           item1.ID,
				CollectionID: item1.CollectionID,
				ParentID:     item1.ParentID,
				Name:         item1.Name,
				Url:          item1.Url,
				Method:       item1.Method,
				Prev:         item1.Prev,
				Next:         item1.Next,
				// 2
				ID_2:           item2.ID,
				CollectionID_2: item2.CollectionID,
				ParentID_2:     item2.ParentID,
				Name_2:         item2.Name,
				Url_2:          item2.Url,
				Method_2:       item2.Method,
				Prev_2:         item2.Prev,
				Next_2:         item2.Next,
				// 3
				ID_3:           item3.ID,
				CollectionID_3: item3.CollectionID,
				ParentID_3:     item3.ParentID,
				Name_3:         item3.Name,
				Url_3:          item3.Url,
				Method_3:       item3.Method,
				Prev_3:         item3.Prev,
				Next_3:         item3.Next,
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
			return []mitemapi.ItemApi{}, ErrNoItemApiFound
		}
		return nil, err
	}
	return MassConvert(itemApis, ConvertToModelItemApi), nil
}

func (ias ItemApiService) GetOwnerID(ctx context.Context, id ulid.ULID) (ulid.ULID, error) {
	ownerUlid, err := ias.queries.GetItemApiOwnerID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ulid.ULID{}, ErrNoItemApiFound
		}
		return ulid.ULID{}, err
	}
	return ownerUlid, err
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
