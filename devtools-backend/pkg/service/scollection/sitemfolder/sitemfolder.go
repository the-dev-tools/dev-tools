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
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS item_folder (
                        id TEXT PRIMARY KEY,
                        name TEXT,
                        parent_id TEXT,
                        collection_id TEXT,
                        FOREIGN KEY (collection_id) REFERENCES collections (id)
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
