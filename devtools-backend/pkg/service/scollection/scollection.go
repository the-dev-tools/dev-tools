package scollection

import (
	"database/sql"
	"devtools-backend/pkg/model/mcollection"

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
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS collections (
                        id TEXT PRIMARY KEY,
                        name TEXT
                )
        `)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
                CREATE TABLE IF NOT EXISTS collection_nodes (
                        id TEXT PRIMARY KEY,
                        collection_id TEXT,
                        name TEXT,
                        type TEXT,
                        parent_id TEXT,
                        data TEXT,
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
	return nil
}

func PrepareCreateCollection(db *sql.DB) error {
	var err error
	PreparedCreateCollection, err = db.Prepare(`
                INSERT INTO collections (id, name)
                VALUES (?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetCollection(db *sql.DB) error {
	var err error
	PreparedGetCollection, err = db.Prepare(`
                SELECT id, name
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
                SET name = ?
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
                SELECT id, name
                FROM collections
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
}

func ListCollections() ([]mcollection.Collection, error) {
	rows, err := PreparedListCollections.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var collections []mcollection.Collection
	for rows.Next() {
		var c mcollection.Collection
		err := rows.Scan(&c.ID, &c.Name)
		if err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, nil
}

func CreateCollection(collection *mcollection.Collection) error {
	_, err := PreparedCreateCollection.Exec(collection.ID, collection.Name)
	if err != nil {
		return err
	}
	return nil
}

func GetCollection(id ulid.ULID) (*mcollection.Collection, error) {
	c := mcollection.Collection{}
	err := PreparedGetCollection.QueryRow(id).Scan(&c.ID, &c.Name)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateCollection(collection *mcollection.Collection) error {
	_, err := PreparedUpdateCollection.Exec(collection.Name, collection.ID)
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
