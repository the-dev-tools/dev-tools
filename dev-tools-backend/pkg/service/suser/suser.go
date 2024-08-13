package suser

import (
	"database/sql"
	"devtools-backend/pkg/model/muser"

	"github.com/oklog/ulid/v2"
)

func PrepareTables(db *sql.DB) error {
	_, err := db.Exec(`
                CREATE TABLE IF NOT EXISTS users (
                        id TEXT PRIMARY KEY,
                        email TEXT,
                        password TEXT,
                        oauth_type INTEGER,
                        oauth_id TEXT
                )
        `)
	if err != nil {
		return err
	}
	return nil
}

var (
	PreparedCreateUser *sql.Stmt = nil
	PreparedGetUser    *sql.Stmt = nil
	PreparedUpdateUser *sql.Stmt = nil
	PreparedDeleteUser *sql.Stmt = nil

	PreparedGetUserWithOAuthIDAndType *sql.Stmt = nil
)

func PrepareStatements(db *sql.DB) error {
	var err error
	// Base Statements
	err = PrepareCreateUser(db)
	if err != nil {
		return err
	}
	err = PrepareGetUser(db)
	if err != nil {
		return err
	}
	err = PrepareUpdateUser(db)
	if err != nil {
		return err
	}
	err = PrepareDeleteUser(db)
	if err != nil {
		return err
	}
	err = PrepareGetUserWithOAuthIDAndType(db)
	if err != nil {
		return err
	}
	return nil
}

func PrepareCreateUser(db *sql.DB) error {
	var err error
	PreparedCreateUser, err = db.Prepare(`
                INSERT INTO users (id, email, password, oauth_type, oauth_id)
                VALUES (?, ?, ?, ?, ?)
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetUser(db *sql.DB) error {
	var err error
	PreparedGetUser, err = db.Prepare(`
                SELECT email, oauth_type, oauth_id
                FROM users
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareUpdateUser(db *sql.DB) error {
	var err error
	PreparedUpdateUser, err = db.Prepare(`
                UPDATE users
                SET email = ?, oauth_type = ?, oauth_id = ?
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareDeleteUser(db *sql.DB) error {
	var err error
	PreparedDeleteUser, err = db.Prepare(`
                DELETE FROM users
                WHERE id = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func PrepareGetUserWithOAuthIDAndType(db *sql.DB) error {
	var err error
	PreparedGetUserWithOAuthIDAndType, err = db.Prepare(`
                SELECT id, email, oauth_type, oauth_id
                FROM users
                WHERE oauth_id = ? AND oauth_type = ?
        `)
	if err != nil {
		return err
	}
	return nil
}

func CreateUser(user *muser.User) error {
	_, err := PreparedCreateUser.Exec(user.ID, user.Email, user.Password, user.OAuthType, user.OAuthID)
	if err != nil {
		return err
	}
	return nil
}

func GetUser(id ulid.ULID) (*muser.User, error) {
	user := &muser.User{}
	err := PreparedGetUser.QueryRow(id).Scan(&user.Email, &user.OAuthType, &user.OAuthID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func UpdateUser(user *muser.User) error {
	_, err := PreparedUpdateUser.Exec(user.Email, user.OAuthType, user.OAuthID, user.ID)
	if err != nil {
		return err
	}
	return nil
}

func DeleteUser(id ulid.ULID) error {
	_, err := PreparedDeleteUser.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func GetUserWithOAuthIDAndType(oauthID string, oauthType muser.OAuthType) (*muser.User, error) {
	var user muser.User
	err := PreparedGetUserWithOAuthIDAndType.QueryRow(oauthID, oauthType).Scan(&user.ID, &user.Email, &user.OAuthType, &user.OAuthID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
