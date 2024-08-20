package sworkspacesusers

import (
	"database/sql"
	"dev-tools-backend/pkg/model/mworkspaceuser"

	"github.com/oklog/ulid/v2"
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
                CREATE TABLE IF NOT EXISTS workspaces_users (
                        id TEXT PRIMARY KEY,

                        workspace_id BLOB NOT NULL,
                        user_id BLOB NOT NULL,

                        UNIQUE(workspace_id, user_id),
                        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
                        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
                )
        `)
	if err != nil {
		return err
	}
	/*
		        TODO: check if this is needed
			row := db.QueryRow(`
		                SELECT * FROM sqlite_master LIMIT 1
		                WHERE type= 'index' and tbl_name = 'workspaces_users' and name = 'Idx1';
		        `)
			if row.Err() == sql.ErrNoRows {
				_, err = db.Exec(`
		                CREATE INDEX Idx1 ON item_api(workspace_id, user_id);
		        `)
			}
	*/

	return nil
}

func PrepareStatements(db *sql.DB) error {
	var err error
	// Base Statements
	err = PrepareCreateWorkspaceUser(db)
	if err != nil {
		return err
	}
	err = PrepareGetWorkspaceUser(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateWorkspaceUser(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteWorkspaceUser(db)
	if err != nil {
		return err
	}
	err = PrepareGetWorkspaceUserByUserID(db)
	if err != nil {
		return err
	}
	err = PrepareGetWorkspaceUserByOrgID(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateWorkspaceUser(db *sql.DB) error {
	var err error
	PreparedCreateOrgUser, err = db.Prepare(`
                INSERT INTO workspaces_users (id, workspace_id, user_id)
                VALUES (?, ?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetWorkspaceUser(db *sql.DB) error {
	var err error
	PreparedGetOrgUser, err = db.Prepare(`
                SELECT id, workspace_id, user_id FROM workspaces_users
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateWorkspaceUser(db *sql.DB) error {
	var err error
	PreparedUpdateOrgUser, err = db.Prepare(`
                UPDATE workspaces_users
                SET workspace_id = ?, user_id = ?
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteWorkspaceUser(db *sql.DB) error {
	var err error
	PreparedDeleteOrgUser, err = db.Prepare(`
                DELETE FROM workspaces_users
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetWorkspaceUserByUserID(db *sql.DB) error {
	var err error
	PreparedGetOrgUserByUserID, err = db.Prepare(`
                SELECT id, workspace_id, user_id FROM workspaces_users
                WHERE user_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetWorkspaceUserByOrgID(db *sql.DB) error {
	var err error
	PreparedGetOrgUserByOrgID, err = db.Prepare(`
                SELECT id, workspace_id, user_id FROM workspaces_users
                WHERE workspace_id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func CreateWorkspaceUser(user *mworkspaceuser.WorkspaceUser) error {
	_, err := PreparedCreateOrgUser.Exec(user.ID, user.OrgID, user.UserID)
	if err != nil {
		return err
	}
	return nil
}

func GetWorkspaceUser(id ulid.ULID) (*mworkspaceuser.WorkspaceUser, error) {
	var orgUser mworkspaceuser.WorkspaceUser
	err := PreparedGetOrgUser.QueryRow(id).Scan(&orgUser.ID, &orgUser.OrgID, &orgUser.UserID)
	if err != nil {
		return nil, err
	}
	return &orgUser, nil
}

func UpdateWorkspaceUser(user *mworkspaceuser.WorkspaceUser) error {
	_, err := PreparedUpdateOrgUser.Exec(user.OrgID, user.UserID, user.ID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteWorkspaceUser(id ulid.ULID) error {
	_, err := PreparedDeleteOrgUser.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func GetWorkspaceUserByUserID(userID string) (*mworkspaceuser.WorkspaceUser, error) {
	var orgUser mworkspaceuser.WorkspaceUser
	err := PreparedGetOrgUserByUserID.QueryRow(userID).Scan(&orgUser.ID, &orgUser.OrgID, &orgUser.UserID)
	if err != nil {
		return nil, err
	}
	return &orgUser, nil
}

func GetWorkspaceUserByWorkspaceID(orgID string) (*mworkspaceuser.WorkspaceUser, error) {
	var orgUser mworkspaceuser.WorkspaceUser
	err := PreparedGetOrgUserByOrgID.QueryRow(orgID).Scan(&orgUser.ID, &orgUser.OrgID, &orgUser.UserID)
	if err != nil {
		return nil, err
	}
	return &orgUser, nil
}
