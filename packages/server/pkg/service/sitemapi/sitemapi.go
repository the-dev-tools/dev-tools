package sitemapi

import (
	"context"
	"database/sql"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type ItemApiService struct {
	queries *gen.Queries
}

func ConvertToDBItemApi(item mitemapi.ItemApi) gen.ItemApi {
	return gen.ItemApi{
		ID:              item.ID,
		CollectionID:    item.CollectionID,
		FolderID:        item.FolderID,
		Name:            item.Name,
		Url:             item.Url,
		Method:          item.Method,
		VersionParentID: item.VersionParentID,
		DeltaParentID:   item.DeltaParentID,
		Hidden:          item.Hidden,
		Prev:            item.Prev,
		Next:            item.Next,
	}
}

func ConvertToModelItemApi(item gen.ItemApi) mitemapi.ItemApi {
	return mitemapi.ItemApi{
		ID:              item.ID,
		CollectionID:    item.CollectionID,
		FolderID:        item.FolderID,
		Name:            item.Name,
		Url:             item.Url,
		Method:          item.Method,
		VersionParentID: item.VersionParentID,
		DeltaParentID:   item.DeltaParentID,
		Hidden:          item.Hidden,
		Prev:            item.Prev,
		Next:            item.Next,
	}
}

var ErrNoItemApiFound = sql.ErrNoRows

func New(queries *gen.Queries) ItemApiService {
	return ItemApiService{queries: queries}
}

func (ias ItemApiService) TX(tx *sql.Tx) ItemApiService {
	return ItemApiService{queries: ias.queries.WithTx(tx)}
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
		ID:              itemConverted.ID,
		CollectionID:    itemConverted.CollectionID,
		FolderID:        itemConverted.FolderID,
		Name:            itemConverted.Name,
		Url:             itemConverted.Url,
		Method:          itemConverted.Method,
		Prev:            itemConverted.Prev,
		Next:            itemConverted.Next,
		VersionParentID: itemConverted.VersionParentID,
		DeltaParentID:   itemConverted.DeltaParentID,
		Hidden:          itemConverted.Hidden,
	})
}

func (ias ItemApiService) CreateItemGenApi(ctx context.Context, item *gen.ItemApi) error {
	return ias.queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:              item.ID,
		CollectionID:    item.CollectionID,
		FolderID:        item.FolderID,
		Name:            item.Name,
		Url:             item.Url,
		Method:          item.Method,
		Prev:            item.Prev,
		Next:            item.Next,
		VersionParentID: item.VersionParentID,
		DeltaParentID:   item.DeltaParentID,
		Hidden:          item.Hidden,
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
			ID:              item1.ID,
			CollectionID:    item1.CollectionID,
			FolderID:        item1.FolderID,
			Name:            item1.Name,
			Url:             item1.Url,
			Method:          item1.Method,
			VersionParentID: item1.VersionParentID,
			DeltaParentID:   item1.DeltaParentID,
			Prev:            item1.Prev,
			Next:            item1.Next,
			Hidden:          item1.Hidden,
			// 2
			ID_2:              item2.ID,
			CollectionID_2:    item2.CollectionID,
			FolderID_2:        item2.FolderID,
			Name_2:            item2.Name,
			Url_2:             item2.Url,
			Method_2:          item2.Method,
			VersionParentID_2: item2.VersionParentID,
			DeltaParentID_2:   item2.DeltaParentID,
			Prev_2:            item2.Prev,
			Next_2:            item2.Next,
			Hidden_2:          item2.Hidden,
			// 3
			ID_3:              item3.ID,
			CollectionID_3:    item3.CollectionID,
			FolderID_3:        item3.FolderID,
			Name_3:            item3.Name,
			Url_3:             item3.Url,
			Method_3:          item3.Method,
			VersionParentID_3: item3.VersionParentID,
			DeltaParentID_3:   item3.DeltaParentID,
			Prev_3:            item3.Prev,
			Next_3:            item3.Next,
			Hidden_3:          item3.Hidden,
			// 4
			ID_4:              item4.ID,
			CollectionID_4:    item4.CollectionID,
			FolderID_4:        item4.FolderID,
			Name_4:            item4.Name,
			Url_4:             item4.Url,
			Method_4:          item4.Method,
			VersionParentID_4: item4.VersionParentID,
			DeltaParentID_4:   item4.DeltaParentID,
			Prev_4:            item4.Prev,
			Next_4:            item4.Next,
			Hidden_4:          item4.Hidden,
			// 5
			ID_5:              item5.ID,
			CollectionID_5:    item5.CollectionID,
			FolderID_5:        item5.FolderID,
			Name_5:            item5.Name,
			Url_5:             item5.Url,
			Method_5:          item5.Method,
			VersionParentID_5: item5.VersionParentID,
			DeltaParentID_5:   item5.DeltaParentID,
			Prev_5:            item5.Prev,
			Next_5:            item5.Next,
			Hidden_5:          item5.Hidden,
			// 6
			ID_6:              item6.ID,
			CollectionID_6:    item6.CollectionID,
			FolderID_6:        item6.FolderID,
			Name_6:            item6.Name,
			Url_6:             item6.Url,
			Method_6:          item6.Method,
			VersionParentID_6: item6.VersionParentID,
			DeltaParentID_6:   item6.DeltaParentID,
			Prev_6:            item6.Prev,
			Next_6:            item6.Next,
			Hidden_6:          item6.Hidden,
			// 7
			ID_7:              item7.ID,
			CollectionID_7:    item7.CollectionID,
			FolderID_7:        item7.FolderID,
			Name_7:            item7.Name,
			Url_7:             item7.Url,
			Method_7:          item7.Method,
			VersionParentID_7: item7.VersionParentID,
			DeltaParentID_7:   item7.DeltaParentID,
			Prev_7:            item7.Prev,
			Next_7:            item7.Next,
			Hidden_7:          item7.Hidden,
			// 8
			ID_8:              item8.ID,
			CollectionID_8:    item8.CollectionID,
			FolderID_8:        item8.FolderID,
			Name_8:            item8.Name,
			Url_8:             item8.Url,
			Method_8:          item8.Method,
			VersionParentID_8: item8.VersionParentID,
			DeltaParentID_8:   item8.DeltaParentID,
			Prev_8:            item8.Prev,
			Next_8:            item8.Next,
			Hidden_8:          item8.Hidden,
			// 9
			ID_9:              item9.ID,
			CollectionID_9:    item9.CollectionID,
			FolderID_9:        item9.FolderID,
			Name_9:            item9.Name,
			Url_9:             item9.Url,
			Method_9:          item9.Method,
			VersionParentID_9: item9.VersionParentID,
			DeltaParentID_9:   item9.DeltaParentID,
			Prev_9:            item9.Prev,
			Next_9:            item9.Next,
			Hidden_9:          item9.Hidden,
			// 10
			ID_10:              item10.ID,
			CollectionID_10:    item10.CollectionID,
			FolderID_10:        item10.FolderID,
			Name_10:            item10.Name,
			Url_10:             item10.Url,
			Method_10:          item10.Method,
			VersionParentID_10: item10.VersionParentID,
			DeltaParentID_10:   item10.DeltaParentID,
			Prev_10:            item10.Prev,
			Next_10:            item10.Next,
			Hidden_10:          item10.Hidden,
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
		FolderID: item.FolderID,
		Name:     item.Name,
		Url:      item.Url,
		Method:   item.Method,
		Hidden:   item.Hidden,
	})
	if err == sql.ErrNoRows {
		return ErrNoItemApiFound
	}
	return err
}

func (ias ItemApiService) UpdateItemApiOrder(ctx context.Context, item *mitemapi.ItemApi) error {
	err := ias.queries.UpdateItemApiOrder(ctx, gen.UpdateItemApiOrderParams{
		Next: item.Next,
		Prev: item.Prev,
		ID:   item.ID,
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

func (ias ItemApiService) GetAllApisWithCollectionID(ctx context.Context, collectionID idwrap.IDWrap) ([]mitemapi.ItemApi, error) {
	itemApis, err := ias.queries.GetAllItemsApiByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapi.ItemApi{}, ErrNoItemApiFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(itemApis, ConvertToModelItemApi), nil
}

func (ias ItemApiService) GetItemApiByCollectionIDAndNextIDAndParentID(ctx context.Context, collectionID idwrap.IDWrap, nextID, parentID *idwrap.IDWrap) (mitemapi.ItemApi, error) {
	itemApi, err := ias.queries.GetItemApiByCollectionIDAndNextIDAndParentID(ctx, gen.GetItemApiByCollectionIDAndNextIDAndParentIDParams{
		Next:         nextID,
		FolderID:     parentID,
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

func (ias ItemApiService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	ownerUlid, err := ias.queries.GetItemApiWorkspaceID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoItemApiFound
		}
		return idwrap.IDWrap{}, err
	}
	return ownerUlid, err
}

func (ias ItemApiService) CheckWorkspaceID(ctx context.Context, id, ownerID idwrap.IDWrap) (bool, error) {
	collectionWorkspaceID, err := ias.GetWorkspaceID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoItemApiFound
		}
		return false, err
	}
	return ownerID.Compare(collectionWorkspaceID) == 0, nil
}
