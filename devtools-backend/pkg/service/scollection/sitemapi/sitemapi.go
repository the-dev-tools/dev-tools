package sitemapi

import (
	"database/sql"
	"devtools-backend/pkg/model/mcollection/mitemapi"

	"github.com/oklog/ulid/v2"
)

var (
	PreparedGetApisWithCollectionID *sql.Stmt = nil

	PreparedCreateItemApi *sql.Stmt = nil
	PreparedGetItemApi    *sql.Stmt = nil
	PreparedUpdateItemApi *sql.Stmt = nil
	PreparedDeleteItemApi *sql.Stmt = nil
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
                        FOREIGN KEY (collection_id) REFERENCES collections (id)
                        FOREIGN KEY (parent_id) REFERENCES item_folder (id)
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
