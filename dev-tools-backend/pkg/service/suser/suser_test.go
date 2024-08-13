package suser_test

import (
	"devtools-backend/pkg/model/muser"
	"devtools-backend/pkg/service/suser"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/oklog/ulid/v2"
)

func TestCreateUser(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	user := &muser.User{
		ID:        ulid.Make(),
		Email:     "email",
		Password:  []byte("password"),
		OAuthType: muser.NoOauth,
		OAuthID:   ulid.Make().String(),
	}
	query := `
                INSERT INTO users (id, email, password, oauth_type, oauth_id)
                VALUES (?, ?, ?, ?, ?)
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = suser.PrepareCreateUser(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(user.ID, user.Email, user.Password, user.OAuthType, user.OAuthID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = suser.CreateUser(user)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetUser(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	user := &muser.User{
		ID:        ulid.Make(),
		Email:     "email",
		Password:  []byte("password"),
		OAuthType: muser.NoOauth,
		OAuthID:   ulid.Make().String(),
	}
	query := `
                SELECT email, oauth_type, oauth_id
                FROM users
                WHERE id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = suser.PrepareGetUser(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(user.ID).
		WillReturnRows(sqlmock.NewRows([]string{"email", "oauth_type", "oauth_id"}).
			AddRow(user.Email, user.OAuthType, user.OAuthID))
	_, err = suser.GetUser(user.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUser(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	user := &muser.User{
		ID:        ulid.Make(),
		Email:     "email",
		Password:  []byte("password"),
		OAuthType: muser.NoOauth,
		OAuthID:   ulid.Make().String(),
	}
	query := `
                UPDATE users
                SET email = ?, oauth_type = ?, oauth_id = ?
                WHERE id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = suser.PrepareUpdateUser(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(user.Email, user.OAuthType, user.OAuthID, user.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = suser.UpdateUser(user)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUser(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	user := &muser.User{
		ID: ulid.Make(),
	}
	query := `
                DELETE FROM users
                WHERE id = ?
        `
	ExpectPrepare := mock.ExpectPrepare(query)
	err = suser.PrepareDeleteUser(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(user.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = suser.DeleteUser(user.ID)
	if err != nil {
		t.Fatal(err)
	}
}
