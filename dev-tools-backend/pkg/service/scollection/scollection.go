package scollection

import (
	"database/sql"
	"dev-tools-backend/pkg/model/mcollection"

	"github.com/oklog/ulid/v2"
)

var (
	// List
	PreparedListCollections *sql.Stmt = nil

	// Base Statements
	PreparedCreateCollection *sql.Stmt = nil
	PreparedGetCollection    *sql.Stmt = nil
	PreparedUpdateCollection *sql.Stmt = nil
	PreparedDeleteCollection *sql.Stmt = nil

	PreparedDeleteApisWithCollectionID *sql.Stmt = nil

	PreparedCheckOwner *sql.Stmt = nil
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS collections (
                        id TEXT PRIMARY KEY,
                        owner_id TEXT,
                        name TEXT
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
	err = PrepareCreateCollection(db)
	if err != nil {
		return err
	}
	err = PrepareGetCollection(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateCollection(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteCollection(db)
	if err != nil {
		return err
	}
	// List
	err = PrepareListCollections(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteApisWithCollectionID(db)
	if err != nil {
		return err
	}
	err = PrepareCheckOwner(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateCollection(db *sql.DB) error {
	var err error
	PreparedCreateCollection, err = db.Prepare(`
                INSERT INTO collections (id, owner_id, name)
                VALUES (?, ?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetCollection(db *sql.DB) error {
	var err error
	PreparedGetCollection, err = db.Prepare(`
                SELECT id, owner_id, name
                FROM collections
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateCollection(db *sql.DB) error {
	var err error
	PreparedUpdateCollection, err = db.Prepare(`
                UPDATE collections
                SET name = ?, owner_id = ?
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteCollection(db *sql.DB) error {
	var err error
	PreparedDeleteCollection, err = db.Prepare(`
                DELETE FROM collections
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareListCollections(db *sql.DB) error {
	var err error
	PreparedListCollections, err = db.Prepare(`
                SELECT id, owner_id, name
                FROM collections
                WHERE owner_id = ?
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
                WHERE collection_id = ?, owner_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCheckOwner(db *sql.DB) error {
	var err error
	PreparedCheckOwner, err = db.Prepare(`
                SELECT owner_id
                FROM collections
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func CloseStatements() {
	if PreparedCreateCollection != nil {
		PreparedCreateCollection.Close()
	}
	if PreparedGetCollection != nil {
		PreparedGetCollection.Close()
	}
	if PreparedUpdateCollection != nil {
		PreparedUpdateCollection.Close()
	}
	if PreparedDeleteCollection != nil {
		PreparedDeleteCollection.Close()
	}
	if PreparedListCollections != nil {
		PreparedListCollections.Close()
	}
	if PreparedDeleteApisWithCollectionID != nil {
		PreparedDeleteApisWithCollectionID.Close()
	}
}

func ListCollections(ownerID ulid.ULID) ([]mcollection.Collection, error) {
	rows, err := PreparedListCollections.Query(ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var collections []mcollection.Collection
	for rows.Next() {
		var c mcollection.Collection
		err := rows.Scan(&c.ID, &c.OwnerID, &c.Name)
		if err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, nil
}

func CreateCollection(collection *mcollection.Collection) error {
	_, err := PreparedCreateCollection.Exec(collection.ID, collection.OwnerID, collection.Name)
	if err != nil {
		return err
	}
	return nil
}

func GetCollection(id ulid.ULID) (*mcollection.Collection, error) {
	c := mcollection.Collection{}
	err := PreparedGetCollection.QueryRow(id).Scan(&c.ID, &c.OwnerID, &c.Name)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateCollection(collection *mcollection.Collection) error {
	_, err := PreparedUpdateCollection.Exec(collection.Name, collection.OwnerID, collection.ID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteCollection(id ulid.ULID) error {
	_, err := PreparedDeleteCollection.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func DeleteApisWithCollectionID(collectionID ulid.ULID, owner_id ulid.ULID) error {
	_, err := PreparedDeleteApisWithCollectionID.Exec(collectionID, owner_id)
	if err != nil {
		return err
	}
	return nil
}

func GetOwner(id ulid.ULID) (ulid.ULID, error) {
	var ownerID ulid.ULID
	err := PreparedCheckOwner.QueryRow(id).Scan(&ownerID)
	if err != nil {
		return ownerID, err
	}
	return ownerID, nil
}

func CheckOwner(id ulid.ULID, ownerID ulid.ULID) (bool, error) {
	CollectionOwnerID, err := GetOwner(id)
	if err != nil {
		return false, err
	}
	return ownerID.Compare(CollectionOwnerID) == 0, nil
}
