package sitemfolder

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type ItemFolderService struct {
	DB      *sql.DB
	queries *gen.Queries
}

var ErrNoItemFolderFound = sql.ErrNoRows

func MassConvert[T any, O any](item []T, convFunc func(T) *O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = *convFunc(v)
	}
	return arr
}

func ConvertToDBItemFolder(folder mitemfolder.ItemFolder) gen.ItemFolder {
	return gen.ItemFolder{
		ID:           folder.ID,
		CollectionID: folder.CollectionID,
		ParentID:     folder.ParentID,
		Name:         folder.Name,
	}
}

func ConvertToModelItemFolder(folder gen.ItemFolder) *mitemfolder.ItemFolder {
	return &mitemfolder.ItemFolder{
		ID:           folder.ID,
		CollectionID: folder.CollectionID,
		ParentID:     folder.ParentID,
		Name:         folder.Name,
	}
}

func New(ctx context.Context, db *sql.DB) (*ItemFolderService, error) {
	q, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}

	return &ItemFolderService{
		DB:      db,
		queries: q,
	}, nil
}

func (ifs ItemFolderService) GetFoldersWithCollectionID(ctx context.Context, collectionID ulid.ULID) ([]mitemfolder.ItemFolder, error) {
	rawFolders, err := ifs.queries.GetItemFolderByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemFolderFound
		}
		return nil, err
	}
	folders := make([]mitemfolder.ItemFolder, len(rawFolders))
	for i, rawFolder := range rawFolders {
		folders[i] = mitemfolder.ItemFolder{
			ID:           rawFolder.ID,
			CollectionID: rawFolder.CollectionID,
			ParentID:     rawFolder.ParentID,
			Name:         rawFolder.Name,
		}
	}
	return folders, nil
}

func (ifs ItemFolderService) CreateItemFolder(ctx context.Context, folder *mitemfolder.ItemFolder) error {
	createParams := gen.CreateItemFolderParams{
		ID:           folder.ID,
		Name:         folder.Name,
		CollectionID: folder.CollectionID,
		ParentID:     folder.ParentID,
	}
	return ifs.queries.CreateItemFolder(ctx, createParams)
}

func (ifs ItemFolderService) CreateItemApiBulk(ctx context.Context, items []mitemfolder.ItemFolder) error {
	itemLen := len(items)
	sizeOfChunks := 3
	index := 0

	if itemLen > 2 {
		for {

			item1 := items[index]
			item2 := items[index+1]
			item3 := items[index+2]
			params := gen.CreateItemFolderBulkParams{
				ID:             item1.ID,
				CollectionID:   item1.CollectionID,
				ParentID:       item1.ParentID,
				Name:           item1.Name,
				ID_2:           item2.ID,
				CollectionID_2: item2.CollectionID,
				ParentID_2:     item2.ParentID,
				Name_2:         item2.Name,
				ID_3:           item3.ID,
				CollectionID_3: item3.CollectionID,
				ParentID_3:     item3.ParentID,
				Name_3:         item3.Name,
			}

			if err := ifs.queries.CreateItemFolderBulk(ctx, params); err != nil {
				return err
			}

			index += sizeOfChunks
			if index >= itemLen {
				break
			}

		}
	}
	for _, item := range items[index:] {
		err := ifs.CreateItemFolder(ctx, &item)
		if err != nil {
			return err
		}

	}
	return nil
}

func (ifs ItemFolderService) GetItemFolder(ctx context.Context, id ulid.ULID) (*mitemfolder.ItemFolder, error) {
	rawFolder, err := ifs.queries.GetItemFolder(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemFolderFound
		}
		return nil, err
	}

	return &mitemfolder.ItemFolder{
		ID:           ulid.ULID(rawFolder.ID),
		CollectionID: ulid.ULID(rawFolder.CollectionID),
		ParentID:     rawFolder.ParentID,
		Name:         rawFolder.Name,
	}, nil
}

func (ifs ItemFolderService) UpdateItemFolder(ctx context.Context, folder *mitemfolder.ItemFolder) error {
	err := ifs.queries.UpdateItemFolder(ctx, gen.UpdateItemFolderParams{
		ID:   folder.ID,
		Name: folder.Name,
	})
	if err == sql.ErrNoRows {
		return ErrNoItemFolderFound
	}
	return err
}

func (ifs ItemFolderService) DeleteItemFolder(ctx context.Context, id ulid.ULID) error {
	err := ifs.queries.DeleteItemFolder(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoItemFolderFound
	}
	return err
}

func (ifs ItemFolderService) GetOwnerID(ctx context.Context, folderID ulid.ULID) (ulid.ULID, error) {
	ownerID, err := ifs.queries.GetItemFolderOwnerID(ctx, folderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ulid.ULID{}, ErrNoItemFolderFound
		}
		return ulid.ULID{}, err
	}
	return ulid.ULID(ownerID), err
}

func (ifs ItemFolderService) CheckOwnerID(ctx context.Context, folderID ulid.ULID, ownerID ulid.ULID) (bool, error) {
	CollectionOwnerID, err := ifs.GetOwnerID(ctx, folderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoItemFolderFound
		}
		return false, err
	}
	return folderID.Compare(CollectionOwnerID) == 0, nil
}
