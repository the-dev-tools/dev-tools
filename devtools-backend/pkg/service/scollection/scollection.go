package scollection

import (
	"database/sql"
	"devtools-backend/pkg/model/mcollection"

	"github.com/oklog/ulid/v2"
)

var (
	// Base Statements
	PreparedCreateCollection *sql.Stmt = nil
	PreparedGetCollection    *sql.Stmt = nil
	PreparedUpdateCollection *sql.Stmt = nil
	PreparedDeleteCollection *sql.Stmt = nil

	// List
	PreparedListCollections *sql.Stmt = nil

	// Collection Node Statements
	PreparedCreateCollectionNode   *sql.Stmt = nil
	PreparedGetCollectionNode      *sql.Stmt = nil
	PreparedGetBulkCollectionNodes *sql.Stmt = nil
	PreparedUpdateCollectionNode   *sql.Stmt = nil
	PreparedDeleteCollectionNode   *sql.Stmt = nil

	// List
	PreparedListCollectionNodes *sql.Stmt = nil

	// Move Node
	PreparedMoveCollectionNode *sql.Stmt = nil
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
	// Collection Node Statements
	err = PrepareCreateCollectionNode(db)
	if err != nil {
		return err
	}
	err = PrepareGetCollectionNode(db)
	if err != nil {
		return err
	}
	err = PrepareGetBulkCollectionNodes(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateCollectionNode(db)
	if err != nil {
		return err
	}
	// List
	err = PrepareListCollectionNodes(db)
	if err != nil {
		return err
	}

	err = PrepareMoveCollectionNode(db)
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

func PrepareCreateCollectionNode(db *sql.DB) error {
	var err error
	PreparedCreateCollectionNode, err = db.Prepare(`
                INSERT INTO collection_nodes (id, collection_id, name, type, parent_id)
                VALUES (?, ?, ?, ?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetCollectionNode(db *sql.DB) error {
	var err error
	PreparedGetCollectionNode, err = db.Prepare(`
                SELECT id, collection_id, name, type, parent_id, data
                FROM collection_nodes
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateCollectionNode(db *sql.DB) error {
	var err error
	PreparedUpdateCollectionNode, err = db.Prepare(`
                UPDATE collection_nodes
                SET name = ?, type = ?, parent_id = ? 
                WHERE id = ? 
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteCollectionNode(db *sql.DB) error {
	var err error
	PreparedDeleteCollectionNode, err = db.Prepare(`
                DELETE FROM collection_nodes
                WHERE id = ? 
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareListCollectionNodes(db *sql.DB) error {
	var err error
	PreparedListCollectionNodes, err = db.Prepare(`
                SELECT id
                FROM collection_nodes
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetBulkCollectionNodes(db *sql.DB) error {
	var err error
	PreparedGetBulkCollectionNodes, err = db.Prepare(`
                SELECT id, collection_id, name, type, parent_id, data
                FROM collection_nodes
                WHERE id IN (?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareMoveCollectionNode(db *sql.DB) error {
	var err error
	PreparedMoveCollectionNode, err = db.Prepare(`
                UPDATE collection_nodes
                SET parent_id = ?, collection_id = ?
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
	if PreparedCreateCollectionNode != nil {
		PreparedCreateCollectionNode.Close()
	}
	if PreparedGetCollectionNode != nil {
		PreparedGetCollectionNode.Close()
	}
	if PreparedUpdateCollectionNode != nil {
		PreparedUpdateCollectionNode.Close()
	}
	if PreparedDeleteCollectionNode != nil {
		PreparedDeleteCollectionNode.Close()
	}
	if PreparedListCollectionNodes != nil {
		PreparedListCollectionNodes.Close()
	}
	if PreparedMoveCollectionNode != nil {
		PreparedMoveCollectionNode.Close()
	}
}

func CreateCollection(db *sql.DB, id ulid.ULID, name string) error {
	_, err := PreparedCreateCollection.Exec(id, name)
	return err
}

func GetCollection(db *sql.DB, id ulid.ULID) (*mcollection.Collection, error) {
	var collection mcollection.Collection
	err := PreparedGetCollection.QueryRow(id).Scan(&collection.ID, &collection.Name)
	return &collection, err
}

func UpdateCollection(db *sql.DB, id ulid.ULID, name string) error {
	_, err := PreparedUpdateCollection.Exec(name, id)
	return err
}

func DeleteCollection(db *sql.DB, id ulid.ULID) error {
	_, err := PreparedDeleteCollection.Exec(id)
	return err
}

func ListCollections(db *sql.DB) ([]ulid.ULID, []string, error) {
	rows, err := PreparedListCollections.Query()
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var collections []ulid.ULID
	var names []string
	for rows.Next() {
		var collection ulid.ULID
		var name string
		err = rows.Scan(&collection, &name)
		if err != nil {
			return nil, nil, err
		}
		collections = append(collections, collection)
		names = append(names, name)
	}
	return collections, names, nil
}

func GetCollectionNodeWithCollectionID(db *sql.DB, collectionID ulid.ULID) ([]ulid.ULID, error) {
	rows, err := PreparedListCollectionNodes.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodeIds []ulid.ULID
	for rows.Next() {
		var id ulid.ULID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		nodeIds = append(nodeIds, id)
	}
	return nodeIds, nil
}

func CreateCollectionNode(db *sql.DB, collectionNode mcollection.CollectionNode) error {
	_, err := PreparedCreateCollectionNode.Exec(collectionNode.ID, collectionNode.CollectionID, collectionNode.Name, collectionNode.Type, collectionNode.Data)
	return err
}

func GetCollectionNode(db *sql.DB, id ulid.ULID) (*mcollection.CollectionNode, error) {
	node := *mcollection.NewEmptyCollectionNode()
	row := PreparedGetCollectionNode.QueryRow(id)
	err := row.Scan(&node.ID, &node.CollectionID, &node.Name, &node.Type, &node.ParentID, node.Data)
	return &node, err
}

func UpdateCollectionNode(db *sql.DB, id ulid.ULID, name, nodeType string, parentID *string, data interface{}) error {
	_, err := PreparedUpdateCollectionNode.Exec(name, nodeType, parentID, id)
	return err
}

func DeleteCollectionNode(db *sql.DB, id ulid.ULID) error {
	_, err := PreparedDeleteCollectionNode.Exec(id)
	return err
}

func MoveCollectionNode(db *sql.DB, id ulid.ULID, parentID, collectionID ulid.ULID) error {
	_, err := PreparedMoveCollectionNode.Exec(parentID, collectionID, id)
	return err
}
