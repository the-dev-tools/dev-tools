package sitemfolder

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mitemfolder"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type ItemFolderService struct {
	queries *gen.Queries
}

var ErrNoItemFolderFound = sql.ErrNoRows

func ConvertToDBItemFolder(folder mitemfolder.ItemFolder) gen.ItemFolder {
	return gen.ItemFolder{
		ID:           folder.ID,
		CollectionID: folder.CollectionID,
		ParentID:     folder.ParentID,
		Name:         folder.Name,
	}
}

func ConvertToModelItemFolder(folder gen.ItemFolder) mitemfolder.ItemFolder {
	return mitemfolder.ItemFolder{
		ID:           folder.ID,
		CollectionID: folder.CollectionID,
		ParentID:     folder.ParentID,
		Name:         folder.Name,
	}
}

func New(queries *gen.Queries) ItemFolderService {
	return ItemFolderService{queries: queries}
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

func (ifs ItemFolderService) GetFolder(ctx context.Context, id idwrap.IDWrap) (*mitemfolder.ItemFolder, error) {
	rawFolder, err := ifs.queries.GetItemFolder(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemFolderFound
		}
		return nil, err
	}
	folder := ConvertToModelItemFolder(rawFolder)
	return &folder, nil
}

func (ifs ItemFolderService) GetFoldersWithCollectionID(ctx context.Context, collectionID idwrap.IDWrap) ([]mitemfolder.ItemFolder, error) {
	rawFolders, err := ifs.queries.GetItemFoldersByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemfolder.ItemFolder{}, ErrNoItemFolderFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(rawFolders, ConvertToModelItemFolder), nil
}

func (ifs ItemFolderService) UpdateOrder(ctx context.Context, folder *mitemfolder.ItemFolder) error {
	folders, err := ifs.GetFoldersWithCollectionID(ctx, folder.CollectionID)
	if err != nil {
		return err
	}

	parentIDPtr := folder.ParentID
	if parentIDPtr == nil {
		tempID, err := idwrap.NewFromBytes(make([]byte, 0))
		if err != nil {
			return err
		}
		parentIDPtr = &tempID
	}
	parentFolders := make([]mitemfolder.ItemFolder, 0)
	parentID := *parentIDPtr
	for _, folder := range folders {
		if folder.ParentID.Compare(parentID) == 0 {
			parentFolders = append(parentFolders, folder)
		}
	}

	sameLevelFolderMap := make(map[idwrap.IDWrap]mitemfolder.ItemFolder)
	for _, folder := range parentFolders {
		sameLevelFolderMap[folder.ID] = folder
	}

	var folderPrev *mitemfolder.ItemFolder
	if folder.Prev != nil {
		parentFolder, ok := sameLevelFolderMap[*folder.Prev]
		if !ok {
			return ErrNoItemFolderFound
		}
		folderPrev = &parentFolder
	}
	var folderNext *mitemfolder.ItemFolder
	if folder.Next != nil {
		childFolder, ok := sameLevelFolderMap[*folder.Next]
		if !ok {
			return ErrNoItemFolderFound
		}
		folderNext = &childFolder
	}

	if folderPrev != nil {
		if folderPrev.Next != nil {
			if folder.Prev != nil {
				if folderPrev.Next.Compare(*folder.Prev) != 0 {
					return errors.New("ordering is not right next and prev not match")
				}
			} else {
				return errors.New("ordering is not right next and prev not match")
			}
		}
		if folderPrev.Next != folder.Next {
			return errors.New("ordering is not right next and prev not match")
		}
		folder.Prev = &folderPrev.ID
		folderPrev.Next = &folder.ID
	}
	if folderNext != nil {
		if folderNext.Prev != nil {
			if folder.Next != nil {
				if folderNext.Prev.Compare(*folder.Next) != 0 {
					return errors.New("ordering is not right next and prev not match")
				}
			} else {
				return errors.New("ordering is not right next and prev not match")
			}
		}
		folder.Next = &folderNext.ID
		folderNext.Prev = &folder.ID
	}

	err = ifs.UpdateItemFolder(ctx, folder)
	if err != nil {
		return err
	}
	err = ifs.UpdateItemFolder(ctx, folderPrev)
	if err != nil {
		return err
	}
	return ifs.UpdateItemFolder(ctx, folderNext)
}

func (ifs ItemFolderService) GetLastFolder(ctx context.Context, collectionID idwrap.IDWrap, parentIDPtr, targetIDPtr *idwrap.IDWrap) (*mitemfolder.ItemFolder, error) {
	folders, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	var folderParentSelected []mitemfolder.ItemFolder
	if parentIDPtr != nil {
		for _, folder := range folders {
			if folder.ParentID == nil {
				folderParentSelected = append(folderParentSelected, folder)
			}
		}
	} else {
		for _, folder := range folders {
			if folder.ParentID.Compare(*parentIDPtr) == 0 {
				folderParentSelected = append(folderParentSelected, folder)
			}
		}
	}
	if targetIDPtr == nil {
		for _, folder := range folderParentSelected {
			if folder.Next == nil {
				return &folder, nil
			}
		}
	} else {
		for _, folder := range folderParentSelected {
			if folder.ID.Compare(*targetIDPtr) == 0 {
				return &folder, nil
			}
		}
	}
	return nil, errors.New("no folder found")
}

func (ifs ItemFolderService) CreateItemFolder(ctx context.Context, folder *mitemfolder.ItemFolder) error {
	createParams := gen.CreateItemFolderParams{
		ID:           folder.ID,
		Name:         folder.Name,
		CollectionID: folder.CollectionID,
		ParentID:     folder.ParentID,
		Next:         folder.Next,
		Prev:         folder.Prev,
	}
	return ifs.queries.CreateItemFolder(ctx, createParams)
}

func (ifs ItemFolderService) CreateItemFolderBulk(ctx context.Context, items []mitemfolder.ItemFolder) error {
	const sizeOfChunks = 10

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

func (ifs ItemFolderService) UpdateItemFolder(ctx context.Context, folder *mitemfolder.ItemFolder) error {
	err := ifs.queries.UpdateItemFolder(ctx, gen.UpdateItemFolderParams{
		ID:       folder.ID,
		Name:     folder.Name,
		ParentID: folder.ParentID,
	})
	if err == sql.ErrNoRows {
		return ErrNoItemFolderFound
	}
	return err
}

func (ifs ItemFolderService) DeleteItemFolder(ctx context.Context, id idwrap.IDWrap) error {
	err := ifs.queries.DeleteItemFolder(ctx, id)
	if err == sql.ErrNoRows {
		return ErrNoItemFolderFound
	}
	return err
}

func (ifs ItemFolderService) GetOwnerID(ctx context.Context, folderID idwrap.IDWrap) (idwrap.IDWrap, error) {
	ownerID, err := ifs.queries.GetItemFolderOwnerID(ctx, folderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoItemFolderFound
		}
		return idwrap.IDWrap{}, err
	}
	return ownerID, err
}

func (ifs ItemFolderService) CheckOwnerID(ctx context.Context, folderID idwrap.IDWrap, ownerID idwrap.IDWrap) (bool, error) {
	CollectionOwnerID, err := ifs.GetOwnerID(ctx, folderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoItemFolderFound
		}
		return false, err
	}
	return folderID.Compare(CollectionOwnerID) == 0, nil
}
