package sorg

import (
	"database/sql"
	"devtools-backend/pkg/model/morg"

	"github.com/oklog/ulid/v2"
)

var (
	PreparedCreateOrg *sql.Stmt = nil
	PreparedGetOrg    *sql.Stmt = nil
	PreparedUpdateOrg *sql.Stmt = nil
	PreparedDeleteOrg *sql.Stmt = nil

	PreparedGetOrgByName *sql.Stmt = nil

	// User related
	PreparedGetOrgByUserID *sql.Stmt = nil
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS orgs (
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
	err = PrepareCreateOrg(db)
	if err != nil {
		return err
	}
	err = PrepareGetOrg(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateOrg(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteOrg(db)
	if err != nil {
		return err
	}
	err = PrepareGetOrgByName(db)
	if err != nil {
		return err
	}
	err = PrepareGetOrgByUserID(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateOrg(db *sql.DB) error {
	var err error
	PreparedCreateOrg, err = db.Prepare(`
                INSERT INTO orgs (id, name)
                VALUES (?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetOrg(db *sql.DB) error {
	var err error
	PreparedGetOrg, err = db.Prepare(`
                SELECT id, name
                FROM orgs
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateOrg(db *sql.DB) error {
	var err error
	PreparedUpdateOrg, err = db.Prepare(`
                UPDATE orgs
                SET name = ?
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteOrg(db *sql.DB) error {
	var err error
	PreparedDeleteOrg, err = db.Prepare(`
                DELETE FROM orgs
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetOrgByName(db *sql.DB) error {
	var err error
	PreparedGetOrgByName, err = db.Prepare(`
                SELECT id, name
                FROM orgs
                WHERE name = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetOrgByUserID(db *sql.DB) error {
	var err error
	PreparedGetOrgByUserID, err = db.Prepare(`
                SELECT o.id, o.name
                FROM orgs o
                JOIN user_orgs uo
                ON o.id = uo.org_id
                WHERE uo.user_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func CreateOrg(org *morg.Org) error {
	_, err := PreparedCreateOrg.Exec(org.ID, org.Name)
	if err != nil {
		return err
	}
	return nil
}

func GetOrg(id ulid.ULID) (*morg.Org, error) {
	var org morg.Org
	err := PreparedGetOrg.QueryRow(id).Scan(&org.ID, &org.Name)
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func UpdateOrg(org *morg.Org) error {
	_, err := PreparedUpdateOrg.Exec(org.Name, org.ID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteOrg(id ulid.ULID) error {
	_, err := PreparedDeleteOrg.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func GetOrgByName(name string) (*morg.Org, error) {
	var org morg.Org
	err := PreparedGetOrgByName.QueryRow(name).Scan(&org.ID, &org.Name)
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func GetOrgByUserID(userID ulid.ULID) (*morg.Org, error) {
	var org morg.Org
	err := PreparedGetOrgByUserID.QueryRow(userID).Scan(&org.ID, &org.Name)
	if err != nil {
		return nil, err
	}
	return &org, nil
}
