package sorguser

import (
	"database/sql"
	"devtools-backend/pkg/model/morguser"
)

var (
	PreparedCreateOrgUser *sql.Stmt = nil
	PreparedGetOrgUser    *sql.Stmt = nil
	PreparedUpdateOrgUser *sql.Stmt = nil
	PreparedDeleteOrgUser *sql.Stmt = nil

	PreparedGetOrgUserByUserID *sql.Stmt = nil
	PreparedGetOrgUserByOrgID  *sql.Stmt = nil
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS org_users (
                        id TEXT PRIMARY KEY,
                        org_id TEXT NOT NULL REFERENCES orgs(id),
                        user_id TEXT NOT NULL REFERENCES users(id)
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
	err = PrepareCreateOrgUser(db)
	if err != nil {
		return err
	}
	err = PrepareGetOrgUser(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateOrgUser(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteOrgUser(db)
	if err != nil {
		return err
	}
	err = PrepareGetOrgUserByUserID(db)
	if err != nil {
		return err
	}
	err = PrepareGetOrgUserByOrgID(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateOrgUser(db *sql.DB) error {
	var err error
	PreparedCreateOrgUser, err = db.Prepare(`
                INSERT INTO org_users (id, org_id, user_id)
                VALUES (?, ?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetOrgUser(db *sql.DB) error {
	var err error
	PreparedGetOrgUser, err = db.Prepare(`
                SELECT id, org_id, user_id FROM org_users
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateOrgUser(db *sql.DB) error {
	var err error
	PreparedUpdateOrgUser, err = db.Prepare(`
                UPDATE org_users
                SET org_id = ?, user_id = ?
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteOrgUser(db *sql.DB) error {
	var err error
	PreparedDeleteOrgUser, err = db.Prepare(`
                DELETE FROM org_users
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetOrgUserByUserID(db *sql.DB) error {
	var err error
	PreparedGetOrgUserByUserID, err = db.Prepare(`
                SELECT id, org_id, user_id FROM org_users
                WHERE user_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetOrgUserByOrgID(db *sql.DB) error {
	var err error
	PreparedGetOrgUserByOrgID, err = db.Prepare(`
                SELECT id, org_id, user_id FROM org_users
                WHERE org_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func CreateOrgUser(user *morguser.OrgUser) error {
	_, err := PreparedCreateOrgUser.Exec(user.ID, user.OrgID, user.UserID)
	if err != nil {
		return err
	}
	return nil
}

func GetOrgUser(id string) (*morguser.OrgUser, error) {
	var orgUser morguser.OrgUser
	err := PreparedGetOrgUser.QueryRow(id).Scan(&orgUser.ID, &orgUser.OrgID, &orgUser.UserID)
	if err != nil {
		return nil, err
	}
	return &orgUser, nil
}

func UpdateOrgUser(user *morguser.OrgUser) error {
	_, err := PreparedUpdateOrgUser.Exec(user.OrgID, user.UserID, user.ID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteOrgUser(id string) error {
	_, err := PreparedDeleteOrgUser.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func GetOrgUserByUserID(userID string) (*morguser.OrgUser, error) {
	var orgUser morguser.OrgUser
	err := PreparedGetOrgUserByUserID.QueryRow(userID).Scan(&orgUser.ID, &orgUser.OrgID, &orgUser.UserID)
	if err != nil {
		return nil, err
	}
	return &orgUser, nil
}

func GetOrgUserByOrgID(orgID string) (*morguser.OrgUser, error) {
	var orgUser morguser.OrgUser
	err := PreparedGetOrgUserByOrgID.QueryRow(orgID).Scan(&orgUser.ID, &orgUser.OrgID, &orgUser.UserID)
	if err != nil {
		return nil, err
	}
	return &orgUser, nil
}
