package sitemfolder_test

import (
	"dev-tools-backend/pkg/model/mcollection/mitemfolder"
	"dev-tools-backend/pkg/service/scollection/sitemfolder"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/oklog/ulid/v2"
)

func TestCreateFolder(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	/*
	   CREATE TABLE IF NOT EXISTS item_folder (
	           id TEXT PRIMARY KEY,
	           name TEXT,
	           parent_id TEXT,
	           collection_id TEXT,
	           FOREIGN KEY (collection_id) REFERENCES collections (id)
	   )
	*/

	folder := &mitemfolder.ItemFolder{
		ID:           ulid.Make(),
		CollectionID: ulid.Make(),
		Name:         "name",
		ParentID:     nil,
	}

	query := `
                INSERT INTO item_folder (id, name, parent_id, collection_id)
                VALUES (?, ?, ?, ?)
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemfolder.PrepareCreateItemFolder(db)
	if err != nil {
		t.Fatal(err)
	}

	ExpectPrepare.
		ExpectExec().
		WithArgs(folder.ID, folder.Name, folder.ParentID, folder.CollectionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sitemfolder.CreateItemFolder(folder)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetFolder(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_folder (
	           id TEXT PRIMARY KEY,
	           name TEXT,
	           parent_id TEXT,
	           collection_id TEXT,
	           FOREIGN KEY (collection_id) REFERENCES collections (id)
	   )
	*/
	folder := &mitemfolder.ItemFolder{
		ID:           ulid.Make(),
		CollectionID: ulid.Make(),
		Name:         "name",
		ParentID:     nil,
	}
	query := `
                SELECT id, name, parent_id, collection_id
                FROM item_folder
                WHERE id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemfolder.PrepareGetItemFolder(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(folder.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "parent_id", "collection_id"}).
			AddRow(folder.ID, folder.Name, folder.ParentID, folder.CollectionID))
	folderData, err := sitemfolder.GetItemFolder(folder.ID)
	if err != nil {
		t.Fatal(err)
	}
	if folderData.ID != folder.ID {
		t.Fatal("ID not matching")
	}

	if folderData.Name != folder.Name {
		t.Fatal("Name not matching")
	}

	if folderData.ParentID != folder.ParentID {
		t.Fatal("ParentID not matching")
	}
}

func TestUpdateFolder(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_folder (
	           id TEXT PRIMARY KEY,
	           name TEXT,
	           parent_id TEXT,
	           collection_id TEXT,
	           FOREIGN KEY (collection_id) REFERENCES collections (id)
	   )
	*/
	folder := &mitemfolder.ItemFolder{
		ID:           ulid.Make(),
		CollectionID: ulid.Make(),
		Name:         "name",
		ParentID:     nil,
	}
	query := `
                UPDATE item_folder
                SET name = ?
                WHERE id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemfolder.PrepareUpdateItemFolder(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(folder.Name, folder.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sitemfolder.UpdateItemFolder(folder)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteFolder(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_folder (
	           id TEXT PRIMARY KEY,
	           name TEXT,
	           parent_id TEXT,
	           collection_id TEXT,
	           FOREIGN KEY (collection_id) REFERENCES collections (id)
	   )
	*/
	id := ulid.Make()
	query := `
                DELETE FROM item_folder
                WHERE id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemfolder.PrepareDeleteItemFolder(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sitemfolder.DeleteItemFolder(id)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetFoldersWithCollectionID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_folder (
	           id TEXT PRIMARY KEY,
	           name TEXT,
	           parent_id TEXT,
	           collection_id TEXT,
	           FOREIGN KEY (collection_id) REFERENCES collections (id)
	   )
	*/
	collectionID := ulid.Make()
	query := `
                SELECT id, name, parent_id, collection_id
                FROM item_folder
                WHERE collection_id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemfolder.PrepareGetFoldersWithCollectionID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(collectionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "parent_id", "collection_id"}).
			AddRow(ulid.Make(), "name", nil, collectionID))
	folders, err := sitemfolder.GetFoldersWithCollectionID(collectionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 {
		t.Fatal("Folder not found")
	}
}

func TestDeleteFoldersWithCollectionID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_folder (
	           id TEXT PRIMARY KEY,
	           name TEXT,
	           parent_id TEXT,
	           collection_id TEXT,
	           FOREIGN KEY (collection_id) REFERENCES collections (id)
	   )
	*/
	collectionID := ulid.Make()
	query := `
                DELETE FROM item_folder
                WHERE collection_id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemfolder.PrepareDeleteFoldersWithCollectionID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(collectionID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sitemfolder.DeleteFoldersWithCollectionID(collectionID)
	if err != nil {
		t.Fatal(err)
	}
}
