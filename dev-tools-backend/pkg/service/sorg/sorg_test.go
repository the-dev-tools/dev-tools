package sorg_test

import (
	"dev-tools-backend/pkg/model/morg"
	"dev-tools-backend/pkg/service/sorg"
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
                INSERT INTO orgs (id, name) VALUES (?, ?)
        `
	ulidID := ulid.Make()

	org := &morg.Org{
		ID:   ulidID,
		Name: "name",
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareCreateOrg(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(org.ID, org.Name).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sorg.CreateOrg(org)
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
                SELECT id, name FROM orgs WHERE id = ?
        `
	ulidID := ulid.Make()
	org := &morg.Org{
		ID:   ulidID,
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareGetOrg(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(org.ID).
		WillReturnRows(rows)
	_, err = sorg.GetOrg(org.ID)
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
                UPDATE orgs SET name = ? WHERE id = ?
        `
	ulidID := ulid.Make()
	org := &morg.Org{
		ID:   ulidID,
		Name: "name",
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareUpdateOrg(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(org.Name, org.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sorg.UpdateOrg(org)
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
                DELETE FROM orgs WHERE id = ?
        `
	ulidID := ulid.Make()
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareDeleteOrg(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(ulidID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sorg.DeleteOrg(ulidID)
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
                SELECT id, name FROM orgs WHERE name = ?
        `
	org := &morg.Org{
		ID:   ulid.Make(),
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareGetOrgByName(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(org.Name).
		WillReturnRows(rows)
	_, err = sorg.GetOrgByName(org.Name)
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
                SELECT id, name FROM orgs WHERE id = (SELECT org_id FROM org_users WHERE user_id = ?)
        `
	userID := ulid.Make()
	org := &morg.Org{
		ID:   ulid.Make(),
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareGetOrgByUserID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(userID).
		WillReturnRows(rows)
	_, err = sorg.GetOrgByUserID(userID)
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
                 SELECT id, name FROM orgs WHERE id IN (SELECT org_id FROM org_users WHERE user_id = ?);
        `
	userID := ulid.Make()
	org := &morg.Org{
		ID:   ulid.Make(),
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareGetOrgsByUserID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(userID).
		WillReturnRows(rows)
	_, err = sorg.GetOrgsByUserID(userID)
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
                SELECT id, name FROM orgs WHERE id = (SELECT org_id FROM org_users WHERE user_id = ? AND org_id = ? )
        `
	userID := ulid.Make()
	orgID := ulid.Make()
	org := &morg.Org{
		ID:   orgID,
		Name: "name",
	}
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(org.ID, org.Name)
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sorg.PrepareGetOrgByUserIDAndOrgID(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(userID, orgID).
		WillReturnRows(rows)
	_, err = sorg.GetOrgByUserIDAndOrgID(userID, orgID)
	if err != nil {
		t.Fatal(err)
	}
}
