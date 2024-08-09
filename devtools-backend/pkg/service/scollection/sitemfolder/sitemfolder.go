package sitemfolder

import (
	"database/sql"
	"devtools-backend/pkg/model/mcollection/mitemfolder"

	"github.com/oklog/ulid/v2"
)

var (
	PreparedGetFoldersWithCollectionID *sql.Stmt = nil
	PreparedCreateItemFolder           *sql.Stmt = nil
	PreparedGetItemFolder              *sql.Stmt = nil
	PreparedUpdateItemFolder           *sql.Stmt = nil
	PreparedDeleteItemFolder           *sql.Stmt = nil

	PreparedDeleteFoldersWithCollectionID *sql.Stmt = nil
	PrepaerdCheckOwnerID                  *sql.Stmt
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS item_folder (
                        id TEXT PRIMARY KEY,
                        name TEXT,
                        parent_id TEXT,
                        collection_id TEXT,
                        FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE
                )
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareStatements(db *sql.DB) error {
	var err error
	// Base Statements
	err = PrepareCreateItemFolder(db)
	if err != nil {
		return err
	}
	err = PrepareGetItemFolder(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateItemFolder(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteItemFolder(db)
	if err != nil {
		return err
	}
	err = PrepareGetFoldersWithCollectionID(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteFoldersWithCollectionID(db)
	if err != nil {
		return err
	}
	err = PrepareCheckOwnerID(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateItemFolder(db *sql.DB) error {
	var err error
	PreparedCreateItemFolder, err = db.Prepare(`
                INSERT INTO item_folder (id, name, parent_id, collection_id)
                VALUES (?, ?, ?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetItemFolder(db *sql.DB) error {
	var err error
	PreparedGetItemFolder, err = db.Prepare(`
                SELECT id, name, parent_id, collection_id
                FROM item_folder
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateItemFolder(db *sql.DB) error {
	var err error
	PreparedUpdateItemFolder, err = db.Prepare(`
                UPDATE item_folder
                SET name = ?
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteItemFolder(db *sql.DB) error {
	var err error
	PreparedDeleteItemFolder, err = db.Prepare(`
                DELETE FROM item_folder
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetFoldersWithCollectionID(db *sql.DB) error {
	var err error
	PreparedGetFoldersWithCollectionID, err = db.Prepare(`
                SELECT id, name, parent_id, collection_id
                FROM item_folder
                WHERE collection_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteFoldersWithCollectionID(db *sql.DB) error {
	var err error
	PreparedDeleteFoldersWithCollectionID, err = db.Prepare(`
                DELETE FROM item_folder
                WHERE collection_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCheckOwnerID(db *sql.DB) error {
	var err error

	// check owner_id from collections table and collection_id from item
	PrepaerdCheckOwnerID, err = db.Prepare(`
                SELECT c.owner_id FROM collections c
                JOIN item_folder i ON c.id = i.collection_id
                WHERE i.id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func GetFoldersWithCollectionID(collectionID ulid.ULID) ([]mitemfolder.ItemFolder, error) {
	rows, err := PreparedGetFoldersWithCollectionID.Query(collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var folders []mitemfolder.ItemFolder
	for rows.Next() {
		folder := mitemfolder.ItemFolder{}
		err := rows.Scan(&folder.ID, &folder.Name, &folder.ParentID, &folder.CollectionID)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, nil
}

func CreateItemFolder(folder *mitemfolder.ItemFolder) error {
	_, err := PreparedCreateItemFolder.Exec(folder.ID, folder.Name, folder.ParentID, folder.CollectionID)
	if err != nil {
		return err
	}
	return nil
}

func GetItemFolder(id ulid.ULID) (*mitemfolder.ItemFolder, error) {
	folder := mitemfolder.ItemFolder{}
	err := PreparedGetItemFolder.QueryRow(id).Scan(&folder.ID, &folder.Name, &folder.ParentID, &folder.CollectionID)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func UpdateItemFolder(folder *mitemfolder.ItemFolder) error {
	_, err := PreparedUpdateItemFolder.Exec(folder.Name, folder.ID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteItemFolder(id ulid.ULID) error {
	_, err := PreparedDeleteItemFolder.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func DeleteFoldersWithCollectionID(collectionID ulid.ULID) error {
	_, err := PreparedDeleteFoldersWithCollectionID.Exec(collectionID)
	if err != nil {
		return err
	}
	return nil
}

func GetOwnerID(folderID ulid.ULID) (ulid.ULID, error) {
	var collectionOwnerID ulid.ULID
	err := PrepaerdCheckOwnerID.QueryRow(folderID).Scan(&collectionOwnerID)
	return collectionOwnerID, err
}

func CheckOwnerID(folderID ulid.ULID, ownerID ulid.ULID) (bool, error) {
	CollectionOwnerID, err := GetOwnerID(folderID)
	if err != nil {
		return false, err
	}
	return folderID.Compare(CollectionOwnerID) == 0, nil
}
