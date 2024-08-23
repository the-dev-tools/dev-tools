package sitemfolder

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	"dev-tools-db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
)

type ItemFolderService struct {
	DB      *sql.DB
	queries *gen.Queries
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
		return nil, err
	}
	var folders []mitemfolder.ItemFolder
	for _, rawFolder := range rawFolders {

		folder := mitemfolder.ItemFolder{
			ID:           rawFolder.ID,
			CollectionID: rawFolder.CollectionID,
			ParentID:     rawFolder.ParentID,
			Name:         rawFolder.Name,
		}
		folders = append(folders, folder)
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

func (ifs ItemFolderService) GetItemFolder(ctx context.Context, id ulid.ULID) (*mitemfolder.ItemFolder, error) {
	rawFolder, err := ifs.queries.GetItemFolder(ctx, id)
	if err != nil {
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
	return ifs.queries.UpdateItemFolder(ctx, gen.UpdateItemFolderParams{
		ID:   folder.ID,
		Name: folder.Name,
	})
}

func (ifs ItemFolderService) DeleteItemFolder(ctx context.Context, id ulid.ULID) error {
	return ifs.queries.DeleteItemFolder(ctx, id)
}

func (ifs ItemFolderService) GetOwnerID(ctx context.Context, folderID ulid.ULID) (ulid.ULID, error) {
	ownerID, err := ifs.queries.GetItemFolderOwnerID(ctx, folderID)
	if err != nil {
		return ulid.ULID{}, err
	}
	return ulid.ULID(ownerID), err
}

func (ifs ItemFolderService) CheckOwnerID(ctx context.Context, folderID ulid.ULID, ownerID ulid.ULID) (bool, error) {
	CollectionOwnerID, err := ifs.GetOwnerID(ctx, folderID)
	if err != nil {
		return false, err
	}
	return folderID.Compare(CollectionOwnerID) == 0, nil
}
