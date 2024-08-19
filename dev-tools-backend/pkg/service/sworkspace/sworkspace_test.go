package sworkspace_test

import (
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/service/sworkspace"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/oklog/ulid/v2"
)

func TestCreateOrg(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                INSERT INTO workspaces (id, name) VALUES (?, ?)
        `
	ulidID := ulid.Make()

	org := &mworkspace.Workspace{
		ID:   ulidID,
		Name: "name",
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareCreate(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(org.ID, org.Name).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sworkspace.Create(org)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetOrgBy(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                SELECT id, name FROM workspaces WHERE id = ?
        `
	ulidID := ulid.Make()
	org := &mworkspace.Workspace{
		ID:   ulidID,
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareGet(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(org.ID).
		WillReturnRows(rows)
	_, err = sworkspace.Get(org.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateOrg(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                UPDATE workspaces SET name = ? WHERE id = ?
        `
	ulidID := ulid.Make()
	org := &mworkspace.Workspace{
		ID:   ulidID,
		Name: "name",
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareUpdate(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(org.Name, org.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sworkspace.Update(org)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOrg(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                DELETE FROM workspaces WHERE id = ?
        `
	ulidID := ulid.Make()
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareDelete(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(ulidID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sworkspace.Delete(ulidID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetGetOrgByName(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                SELECT id, name FROM workspaces WHERE name = ?
        `
	org := &mworkspace.Workspace{
		ID:   ulid.Make(),
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareGetByName(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(org.Name).
		WillReturnRows(rows)
	_, err = sworkspace.GetByName(org.Name)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetOrgByUserID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                SELECT id, name FROM workspaces WHERE id = (SELECT workspace_id FROM workspaces_users WHERE user_id = ?)
        `
	userID := ulid.Make()
	org := &mworkspace.Workspace{
		ID:   ulid.Make(),
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareGetByUserID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(userID).
		WillReturnRows(rows)
	_, err = sworkspace.GetByUserID(userID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetOrgsByUserID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                 SELECT id, name FROM workspaces WHERE id IN (SELECT workspace_id FROM workspaces_users WHERE user_id = ?);
        `
	userID := ulid.Make()
	org := &mworkspace.Workspace{
		ID:   ulid.Make(),
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareGetMultiByUserID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(userID).
		WillReturnRows(rows)
	_, err = sworkspace.GetMultiByUserID(userID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetOrgByUserIDAndOrgID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	query := `
                SELECT id, name FROM workspaces WHERE id = (SELECT workspace_id FROM workspaces_users WHERE workspace_id = ? AND user_id = ? )
        `
	userID := ulid.Make()
	orgID := ulid.Make()
	org := &mworkspace.Workspace{
		ID:   orgID,
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sworkspace.PrepareGetByIDAndUserID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(orgID, userID).
		WillReturnRows(rows)
	_, err = sworkspace.GetByIDandUserID(orgID, userID)
	if err != nil {
		t.Fatal(err)
	}
}
