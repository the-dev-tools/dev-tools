package sitemapi

import (
	"database/sql"
	"dev-tools-backend/pkg/model/mcollection/mitemapi"

	"github.com/oklog/ulid/v2"
)

var (
	PreparedGetApisWithCollectionID *sql.Stmt = nil

	PreparedCreateItemApi *sql.Stmt = nil
	PreparedGetItemApi    *sql.Stmt = nil
	PreparedUpdateItemApi *sql.Stmt = nil
	PreparedDeleteItemApi *sql.Stmt = nil

	PreparedDeleteApisWithCollectionID *sql.Stmt = nil
	PreparedCheckOwnerID               *sql.Stmt
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS item_api (
                        id TEXT PRIMARY KEY,
                        collection_id TEXT,
                        parent_id TEXT,
                        name TEXT,
                        url TEXT,
                        method TEXT,
                        headers TEXT,
                        query_params TEXT,
                        body TEXT,
                        FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
                        FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
                )
        `)
	if err != nil {
		return err
	}

	row := db.QueryRow(`
                SELECT * FROM sqlite_master LIMIT 1
                WHERE type= 'index' and tbl_name = 'item_api' and name = 'Idx1';
        `)
	if row.Err() == sql.ErrNoRows {
		_, err = db.Exec(`
                CREATE INDEX Idx1 ON item_api(collection_id);
        `)
	}
	if err != nil {
		return err
	}

	return nil
}

func PrepareStatements(db *sql.DB) error {
	var err error
	// Base Statements
	err = PrepareCreateItemApi(db)
	if err != nil {
		return err
	}
	err = PrepareGetItemApi(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateItemApi(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteItemApi(db)
	if err != nil {
		return err
	}
	err = PrepareGetItemsWithCollectionID(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteApisWithCollectionID(db)
	if err != nil {
		return err
	}
	err = PrepareCheckOwnerID(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateItemApi(db *sql.DB) error {
	var err error
	PreparedCreateItemApi, err = db.Prepare(`
        INSERT INTO item_api (id, collection_id, parent_id, name, url, method, headers, query_params, body)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetItemApi(db *sql.DB) error {
	var err error
	PreparedGetItemApi, err = db.Prepare(`
                SELECT id, collection_id, parent_id, name, url, method, headers, query_params, body
                FROM item_api
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateItemApi(db *sql.DB) error {
	var err error
	PreparedUpdateItemApi, err = db.Prepare(`
                UPDATE item_api
                SET collection_id = ?, parent_id = ?, name = ?, url = ?, method = ?, headers = ?, query_params = ?, body = ?
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteItemApi(db *sql.DB) error {
	var err error
	PreparedDeleteItemApi, err = db.Prepare(`
                DELETE FROM item_api
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetItemsWithCollectionID(db *sql.DB) error {
	var err error
	PreparedGetApisWithCollectionID, err = db.Prepare(`
                SELECT id, collection_id, parent_id, name, url, method, headers, query_params, body
                FROM item_api
                WHERE collection_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteApisWithCollectionID(db *sql.DB) error {
	var err error
	PreparedDeleteApisWithCollectionID, err = db.Prepare(`
                DELETE FROM item_api
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
	PreparedCheckOwnerID, err = db.Prepare(`
                SELECT c.owner_id FROM collections c
                JOIN item_api i ON c.id = i.collection_id
                WHERE i.id = ?

        `)
	if err != nil {
		return err
	}
	return nil
}

func GetItemApi(id ulid.ULID) (*mitemapi.ItemApi, error) {
	item := mitemapi.ItemApi{}
	err := PreparedGetItemApi.QueryRow(id).Scan(&item.ID, &item.CollectionID, &item.ParentID, &item.Name, &item.Url, &item.Method, &item.Headers, &item.QueryParams, &item.Body)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func CreateItemApi(item *mitemapi.ItemApi) error {
	_, err := PreparedCreateItemApi.Exec(item.ID, item.CollectionID, item.ParentID, item.Name, item.Url, item.Method, item.Headers, item.QueryParams, item.Body)
	if err != nil {
		return err
	}
	return nil
}

func UpdateItemApi(item *mitemapi.ItemApi) error {
	_, err := PreparedUpdateItemApi.Exec(item.CollectionID, item.ParentID, item.Name, item.Url, item.Method, item.Headers, item.QueryParams, item.Body, item.ID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteItemApi(id ulid.ULID) error {
	_, err := PreparedDeleteItemApi.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func GetApisWithCollectionID(collectionID ulid.ULID) ([]mitemapi.ItemApi, error) {
	items := []mitemapi.ItemApi{}
	rows, err := PreparedGetApisWithCollectionID.Query(collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item := mitemapi.ItemApi{}
		err = rows.Scan(&item.ID, &item.CollectionID, &item.ParentID, &item.Name, &item.Url, &item.Method, &item.Headers, &item.QueryParams, &item.Body)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func DeleteApisWithCollectionID(collectionID ulid.ULID) error {
	_, err := PreparedDeleteApisWithCollectionID.Exec(collectionID)
	if err != nil {
		return err
	}
	return nil
}

func GetOwnerID(id ulid.ULID) (ulid.ULID, error) {
	var ownerID ulid.ULID
	err := PreparedCheckOwnerID.QueryRow(id).Scan(&ownerID)
	return ownerID, err
}

func CheckOwnerID(id ulid.ULID, ownerID ulid.ULID) (bool, error) {
	collectionOwnerID, err := GetOwnerID(id)
	if err != nil {
		return false, err
	}
	return ownerID.Compare(collectionOwnerID) == 0, nil
}
