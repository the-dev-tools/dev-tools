package scollection_test

import (
	"devtools-backend/pkg/model/mcollection"
	"devtools-backend/pkg/service/scollection"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/oklog/ulid/v2"
)

func TestCreateCollection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS collections (
	           id TEXT PRIMARY KEY,
	           name TEXT
	   )
	*/
	query := `
                INSERT INTO collections (id, name)
                VALUES (?, ?)
        `
	id := ulid.Make()

	collection := &mcollection.Collection{
		ID:   id,
		Name: "name",
	}

	ExpectPrepare := mock.ExpectPrepare(query)
	err = scollection.PrepareCreateCollection(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(collection.ID, collection.Name).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = scollection.CreateCollection(collection)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetCollection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS collections (
	           id TEXT PRIMARY KEY,
	           name TEXT
	   )
	*/
	query := `
                SELECT id, name
                FROM collections
                WHERE id = ?
        `

	id := ulid.Make()

	collection := &mcollection.Collection{
		ID:   id,
		Name: "name",
	}

	ExpectPrepare := mock.ExpectPrepare(query)
	err = scollection.PrepareGetCollection(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectQuery().
		WithArgs(collection.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(collection.ID, collection.Name))
	collectionReturned, err := scollection.GetCollection(id)
	if err != nil {
		t.Fatal(err)
	}
	if collectionReturned.ID != collection.ID {
		t.Fatal("ID not matching")
	}

	if collectionReturned.Name != collection.Name {
		t.Fatal("Name not matching")
	}
}

func TestUpdateCollection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS collections (
	           id TEXT PRIMARY KEY,
	           name TEXT
	   )
	*/
	query := `
                UPDATE collections
                SET name = ?
                WHERE id = ?
        `
	id := ulid.Make()
	collection := &mcollection.Collection{
		ID:   id,
		Name: "name",
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = scollection.PrepareUpdateCollection(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(collection.Name, collection.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = scollection.UpdateCollection(collection)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCollection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS collections (
	           id TEXT PRIMARY KEY,
	           name TEXT
	   )
	*/

	query := `
                DELETE FROM collections
                WHERE id = ?
        `
	id := ulid.Make()
	ExpectPrepare := mock.ExpectPrepare(query)
	err = scollection.PrepareDeleteCollection(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.
		ExpectExec().
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = scollection.DeleteCollection(id)
	if err != nil {
		t.Fatal(err)
	}
}
