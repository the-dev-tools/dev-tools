package sworkspace

import (
	"database/sql"
	"dev-tools-backend/pkg/model/mworkspace"

	"github.com/oklog/ulid/v2"
)

var ErrOrgNotFound = sql.ErrNoRows

var (
	PreparedCreateOrg *sql.Stmt = nil
	PreparedGetOrg    *sql.Stmt = nil
	PreparedUpdateOrg *sql.Stmt = nil
	PreparedDeleteOrg *sql.Stmt = nil

	PreparedGetOrgByName *sql.Stmt = nil

	// User related
	PreparedGetOrgByUserID         *sql.Stmt = nil
	PreparedGetOrgsByUserID        *sql.Stmt = nil
	PreparedGetOrgByUserIDAndOrgID *sql.Stmt = nil
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS workspaces (
                        id TEXT PRIMARY KEY,
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
	err = PrepareCreate(db)
	if err != nil {
		return err
	}
	err = PrepareGet(db)
	if err != nil {
		return err
	}
	err = PrepareUpdate(db)
	if err != nil {
		return err
	}
	err = PrepareDelete(db)
	if err != nil {
		return err
	}
	err = PrepareGetByName(db)
	if err != nil {
		return err
	}
	err = PrepareGetByUserID(db)
	if err != nil {
		return err
	}
	err = PrepareGetMultiByUserID(db)
	if err != nil {
		return err
	}
	err = PrepareGetByIDAndUserID(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreate(db *sql.DB) error {
	var err error
	PreparedCreateOrg, err = db.Prepare(`
                INSERT INTO workspaces (id, name)
                VALUES (?, ?)
        `)
	return err
}

func PrepareGet(db *sql.DB) error {
	var err error
	PreparedGetOrg, err = db.Prepare(`
                SELECT id, name
                FROM workspaces
                WHERE id = ?
        `)
	return err
}

func PrepareUpdate(db *sql.DB) error {
	var err error
	PreparedUpdateOrg, err = db.Prepare(`
                UPDATE workspaces
                SET name = ?
                WHERE id = ?
        `)
	return err
}

func PrepareDelete(db *sql.DB) error {
	var err error
	PreparedDeleteOrg, err = db.Prepare(`
                DELETE FROM workspaces
                WHERE id = ?
        `)
	return err
}

func PrepareGetByName(db *sql.DB) error {
	var err error
	PreparedGetOrgByName, err = db.Prepare(`
                SELECT id, name
                FROM workspaces
                WHERE name = ?
        `)
	return err
}

func PrepareGetByUserID(db *sql.DB) error {
	var err error
	PreparedGetOrgByUserID, err = db.Prepare(`
                SELECT id, name FROM workspaces WHERE id = (SELECT workspace_id FROM workspaces_users WHERE user_id = ?)
        `)
	return err
}

func PrepareGetMultiByUserID(db *sql.DB) error {
	var err error
	PreparedGetOrgsByUserID, err = db.Prepare(`
                 SELECT id, name FROM workspaces WHERE id IN (SELECT workspace_id FROM workspaces_users WHERE user_id = ?);
        `)
	return err
}

func PrepareGetByIDAndUserID(db *sql.DB) error {
	var err error
	PreparedGetOrgByUserIDAndOrgID, err = db.Prepare(`
                SELECT id, name FROM workspaces WHERE id = (SELECT workspace_id FROM workspaces_users WHERE workspace_id = ? AND user_id = ? )
        `)
	return err
}

func Create(org *mworkspace.Workspace) error {
	_, err := PreparedCreateOrg.Exec(org.ID, org.Name)
	return err
}

func Get(id ulid.ULID) (*mworkspace.Workspace, error) {
	var org mworkspace.Workspace
	err := PreparedGetOrg.QueryRow(id).Scan(&org.ID, &org.Name)
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func Update(org *mworkspace.Workspace) error {
	_, err := PreparedUpdateOrg.Exec(org.Name, org.ID)
	if err != nil {
		return err
	}
	return nil
}

func Delete(id ulid.ULID) error {
	_, err := PreparedDeleteOrg.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func GetByName(name string) (*mworkspace.Workspace, error) {
	var org mworkspace.Workspace
	err := PreparedGetOrgByName.QueryRow(name).Scan(&org.ID, &org.Name)
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func GetByUserID(userID ulid.ULID) (*mworkspace.Workspace, error) {
	var org mworkspace.Workspace

	err := PreparedGetOrgByUserID.QueryRow(userID).Scan(&org.ID, &org.Name)
	if err != nil {
		return nil, err
	}

	return &org, nil
}

func GetMultiByUserID(userID ulid.ULID) ([]mworkspace.Workspace, error) {
	rows, err := PreparedGetOrgsByUserID.Query(userID)
	if err != nil {
		return nil, err
	}
	var workspaces []mworkspace.Workspace
	for rows.Next() {
		var org mworkspace.Workspace
		err = rows.Scan(&org.ID, &org.Name)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, org)
	}

	if err != nil {
		return nil, err
	}

	return workspaces, nil
}

func GetByIDandUserID(orgID, userID ulid.ULID) (*mworkspace.Workspace, error) {
	var org mworkspace.Workspace
	err := PreparedGetOrgByUserIDAndOrgID.QueryRow(orgID, userID).Scan(&org.ID, &org.Name)
	if err != nil {
		return nil, err
	}
	return &org, nil
}
