package sitemapi

import (
	"context"
	"database/sql"
	"slices"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type ItemApiService struct {
	queries *gen.Queries
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

func New(queries *gen.Queries) ItemApiService {
	return ItemApiService{queries: queries}
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

func (ias ItemApiService) GetItemApi(ctx context.Context, id idwrap.IDWrap) (*mitemapi.ItemApi, error) {
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

func (ias ItemApiService) CreateItemGenApi(ctx context.Context, item *gen.ItemApi) error {
	return ias.queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           item.ID,
		CollectionID: item.CollectionID,
		ParentID:     item.ParentID,
		Name:         item.Name,
		Url:          item.Url,
		Method:       item.Method,
		Prev:         item.Prev,
		Next:         item.Next,
	})
}

func (ias ItemApiService) CreateItemApiBulk(ctx context.Context, items []mitemapi.ItemApi) error {
	sizeOfChunks := 10
	convertedItems := tgeneric.MassConvert(items, ConvertToDBItemApi)

	for chunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(chunk) < sizeOfChunks {
			for _, item := range chunk {
				err := ias.CreateItemGenApi(ctx, &item)
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
			// 4
			ID_4:           item4.ID,
			CollectionID_4: item4.CollectionID,
			ParentID_4:     item4.ParentID,
			Name_4:         item4.Name,
			Url_4:          item4.Url,
			Method_4:       item4.Method,
			Prev_4:         item4.Prev,
			Next_4:         item4.Next,
			// 5
			ID_5:           item5.ID,
			CollectionID_5: item5.CollectionID,
			ParentID_5:     item5.ParentID,
			Name_5:         item5.Name,
			Url_5:          item5.Url,
			Method_5:       item5.Method,
			Prev_5:         item5.Prev,
			Next_5:         item5.Next,
			// 6
			ID_6:           item6.ID,
			CollectionID_6: item6.CollectionID,
			ParentID_6:     item6.ParentID,
			Name_6:         item6.Name,
			Url_6:          item6.Url,
			Method_6:       item6.Method,
			Prev_6:         item6.Prev,
			Next_6:         item6.Next,
			// 7
			ID_7:           item7.ID,
			CollectionID_7: item7.CollectionID,
			ParentID_7:     item7.ParentID,
			Name_7:         item7.Name,
			Url_7:          item7.Url,
			Method_7:       item7.Method,
			Prev_7:         item7.Prev,
			Next_7:         item7.Next,
			// 8
			ID_8:           item8.ID,
			CollectionID_8: item8.CollectionID,
			ParentID_8:     item8.ParentID,
			Name_8:         item8.Name,
			Url_8:          item8.Url,
			Method_8:       item8.Method,
			Prev_8:         item8.Prev,
			Next_8:         item8.Next,
			// 9
			ID_9:           item9.ID,
			CollectionID_9: item9.CollectionID,
			ParentID_9:     item9.ParentID,
			Name_9:         item9.Name,
			Url_9:          item9.Url,
			Method_9:       item9.Method,
			Prev_9:         item9.Prev,
			Next_9:         item9.Next,
			// 10
			ID_10:           item10.ID,
			CollectionID_10: item10.CollectionID,
			ParentID_10:     item10.ParentID,
			Name_10:         item10.Name,
			Url_10:          item10.Url,
			Method_10:       item10.Method,
			Prev_10:         item10.Prev,
			Next_10:         item10.Next,
		}

		if err := ias.queries.CreateItemApiBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (ias ItemApiService) UpdateItemApi(ctx context.Context, item *mitemapi.ItemApi) error {
	err := ias.queries.UpdateItemApi(ctx, gen.UpdateItemApiParams{
		ID:       item.ID,
		ParentID: item.ParentID,
		Name:     item.Name,
		Url:      item.Url,
		Method:   item.Method,
	})
	if err == sql.ErrNoRows {
		return ErrNoItemApiFound
	}
	return err
}

func (ias ItemApiService) DeleteItemApi(ctx context.Context, id idwrap.IDWrap) error {
	return ias.queries.DeleteItemApi(ctx, id)
}

func (ias ItemApiService) GetApisWithCollectionID(ctx context.Context, collectionID idwrap.IDWrap) ([]mitemapi.ItemApi, error) {
	itemApis, err := ias.queries.GetItemsApiByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapi.ItemApi{}, ErrNoItemApiFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(itemApis, ConvertToModelItemApi), nil
}

func (ias ItemApiService) GetItemApiByCollectionIDAndNext(ctx context.Context, collectionID idwrap.IDWrap, nextID *idwrap.IDWrap) (mitemapi.ItemApi, error) {
	itemApi, err := ias.queries.GetItemApiByCollectionIDAndNextID(ctx, gen.GetItemApiByCollectionIDAndNextIDParams{
		Next:         nextID,
		CollectionID: collectionID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return mitemapi.ItemApi{}, ErrNoItemApiFound
		}
		return mitemapi.ItemApi{}, err
	}
	return ConvertToModelItemApi(itemApi), nil
}

func (ias ItemApiService) GetOwnerID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	ownerUlid, err := ias.queries.GetItemApiOwnerID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoItemApiFound
		}
		return idwrap.IDWrap{}, err
	}
	return ownerUlid, err
}

func (ias ItemApiService) CheckOwnerID(ctx context.Context, id, ownerID idwrap.IDWrap) (bool, error) {
	collectionOwnerID, err := ias.GetOwnerID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoItemApiFound
		}
		return false, err
	}
	return ownerID.Compare(collectionOwnerID) == 0, nil
}
