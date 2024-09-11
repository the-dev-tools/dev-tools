package sitemfolder

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-db/pkg/sqlc/gen"
	"slices"

	"github.com/oklog/ulid/v2"
)

type ItemFolderService struct {
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
		queries: q,
	}, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ItemFolderService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ItemFolderService{
		queries: queries,
	}, nil
}

func (ifs ItemFolderService) GetFoldersWithCollectionID(ctx context.Context, collectionID ulid.ULID) ([]mitemfolder.ItemFolder, error) {
	rawFolders, err := ifs.queries.GetItemFolderByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemfolder.ItemFolder{}, ErrNoItemFolderFound
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

func (ifs ItemFolderService) CreateItemFolderBulk(ctx context.Context, items []mitemfolder.ItemFolder) error {
	sizeOfChunks := 10

	for chunk := range slices.Chunk(items, sizeOfChunks) {
		if len(chunk) < sizeOfChunks {
			for _, item := range chunk {
				err := ifs.CreateItemFolder(ctx, &item)
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
		params := gen.CreateItemFolderBulkParams{
			// Item 1
			ID:           item1.ID,
			CollectionID: item1.CollectionID,
			ParentID:     item1.ParentID,
			Name:         item1.Name,
			Prev:         item1.Prev,
			Next:         item1.Next,
			// Item 2
			ID_2:           item2.ID,
			CollectionID_2: item2.CollectionID,
			ParentID_2:     item2.ParentID,
			Name_2:         item2.Name,
			Prev_2:         item2.Prev,
			Next_2:         item2.Next,
			// Item 3
			ID_3:           item3.ID,
			CollectionID_3: item3.CollectionID,
			ParentID_3:     item3.ParentID,
			Name_3:         item3.Name,
			Prev_3:         item3.Prev,
			Next_3:         item3.Next,
			// Item 4
			ID_4:           item4.ID,
			CollectionID_4: item4.CollectionID,
			ParentID_4:     item4.ParentID,
			Name_4:         item4.Name,
			Prev_4:         item4.Prev,
			Next_4:         item4.Next,
			// Item 5
			ID_5:           item5.ID,
			CollectionID_5: item5.CollectionID,
			ParentID_5:     item5.ParentID,
			Name_5:         item5.Name,
			Prev_5:         item5.Prev,
			Next_5:         item5.Next,
			// Item 6
			ID_6:           item6.ID,
			CollectionID_6: item6.CollectionID,
			ParentID_6:     item6.ParentID,
			Name_6:         item6.Name,
			Prev_6:         item6.Prev,
			Next_6:         item6.Next,
			// Item 7
			ID_7:           item7.ID,
			CollectionID_7: item7.CollectionID,
			ParentID_7:     item7.ParentID,
			Name_7:         item7.Name,
			Prev_7:         item7.Prev,
			Next_7:         item7.Next,
			// Item 8
			ID_8:           item8.ID,
			CollectionID_8: item8.CollectionID,
			ParentID_8:     item8.ParentID,
			Name_8:         item8.Name,
			Prev_8:         item8.Prev,
			Next_8:         item8.Next,
			// Item 9
			ID_9:           item9.ID,
			CollectionID_9: item9.CollectionID,
			ParentID_9:     item9.ParentID,
			Name_9:         item9.Name,
			Prev_9:         item9.Prev,
			Next_9:         item9.Next,
			// Item 10
			ID_10:           item10.ID,
			CollectionID_10: item10.CollectionID,
			ParentID_10:     item10.ParentID,
			Name_10:         item10.Name,
			Prev_10:         item10.Prev,
			Next_10:         item10.Next,
		}

		if err := ifs.queries.CreateItemFolderBulk(ctx, params); err != nil {
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
