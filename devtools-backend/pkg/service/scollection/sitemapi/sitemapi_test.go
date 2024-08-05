package sitemapi_test

import (
	"devtools-backend/pkg/model/mcollection/mitemapi"
	"devtools-backend/pkg/service/scollection/sitemapi"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/oklog/ulid/v2"
)

func TestCreateItemApi(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	/*
	   CREATE TABLE IF NOT EXISTS item_api (
	           id TEXT PRIMARY KEY,
	           collection_id TEXT,
	           name TEXT,
	           url TEXT,
	           method TEXT,
	           headers TEXT,
	           query_params TEXT,
	           body TEXT,
	   )
	*/

	query := `
                INSERT INTO item_api (id, collection_id, name, url, method, headers, query_params, body)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `

	id := ulid.Make()
	collectionID := ulid.Make()

	item := &mitemapi.ItemApi{
		ID:           id,
		CollectionID: collectionID,
		Name:         "name",
		Url:          "http://localhost:8080",
		Method:       "GET",
		Headers:      mitemapi.Headers{HeaderMap: map[string]string{"key": "value"}},
		Body:         []byte("hello"),
		QueryParams:  mitemapi.QueryParams{QueryMap: map[string]string{"key": "value"}},
	}

	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemapi.PrepareCreateItemApi(db)
	if err != nil {
		t.Fatal(err)
	}

	ExpectPrepare.
		ExpectExec().
		WithArgs(id, collectionID, item.Name, item.Url, item.Method, item.Headers, item.QueryParams, item.Body).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sitemapi.CreateItemApi(item)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetItemApi(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_api (
	           id TEXT PRIMARY KEY,
	           collection_id TEXT,
	           name TEXT,
	           url TEXT,
	           method TEXT,
	           headers TEXT,
	           query_params TEXT,
	           body TEXT,
	   )
	*/
	query := `
                SELECT id, collection_id, name, url, method, headers, query_params, body
                FROM item_api
                WHERE id = ?
        `
	id := ulid.Make()
	collectionID := ulid.Make()
	item := &mitemapi.ItemApi{
		ID:           id,
		CollectionID: collectionID,
		Name:         "name",
		Url:          "http://localhost:8080",
		Method:       "GET",
		Headers:      mitemapi.Headers{HeaderMap: map[string]string{"key": "value"}},
		Body:         []byte("hello"),
		QueryParams:  mitemapi.QueryParams{QueryMap: map[string]string{"key": "value"}},
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemapi.PrepareGetItemApi(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.ExpectQuery().WithArgs(id).
		WillReturnRows(mock.NewRows([]string{"id", "collection_id", "name", "url", "method", "headers", "query_params", "body"}).
			AddRow(item.ID, item.CollectionID, item.Name, item.Url, item.Method, item.Headers, item.QueryParams, item.Body))

	rows, err := sitemapi.GetItemApi(id)
	if err != nil {
		t.Fatal(err)
	}

	if rows.ID != id {
		t.Fatalf("expected %v but got %v", id, rows.ID)
	}
	if rows.CollectionID != collectionID {
		t.Fatalf("expected %v but got %v", collectionID, rows.CollectionID)
	}
}

func TestUpdateItemApi(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_api (
	           id TEXT PRIMARY KEY,
	           collection_id TEXT,
	           name TEXT,
	           url TEXT,
	           method TEXT,
	           headers TEXT,
	           query_params TEXT,
	           body TEXT,
	   )
	*/
	query := `
                UPDATE item_api
                SET collection_id = ?, name = ?, url = ?, method = ?, headers = ?, query_params = ?, body = ?
                WHERE id = ?
        `
	id := ulid.Make()
	collectionID := ulid.Make()
	item := &mitemapi.ItemApi{
		ID:           id,
		CollectionID: collectionID,
		Name:         "name",
		Url:          "http://localhost:8080",
		Method:       "GET",
		Headers:      mitemapi.Headers{HeaderMap: map[string]string{"key": "value"}},
		Body:         []byte("hello"),
		QueryParams:  mitemapi.QueryParams{QueryMap: map[string]string{"key": "value"}},
	}
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemapi.PrepareUpdateItemApi(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.ExpectExec().WithArgs(item.CollectionID, item.Name, item.Url, item.Method, item.Headers, item.QueryParams, item.Body, id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sitemapi.UpdateItemApi(item)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteItemApi(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	/*
	   CREATE TABLE IF NOT EXISTS item_api (
	           id TEXT PRIMARY KEY,
	           collection_id TEXT,
	           name TEXT,
	           url TEXT,
	           method TEXT,
	           headers TEXT,
	           query_params TEXT,
	           body TEXT,
	   )
	*/
	query := `
                DELETE FROM item_api
                WHERE id = ?
        `
	id := ulid.Make()
	ExpectPrepare := mock.ExpectPrepare(query)
	err = sitemapi.PrepareDeleteItemApi(db)
	if err != nil {
		t.Fatal(err)
	}
	ExpectPrepare.ExpectExec().WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = sitemapi.DeleteItemApi(id)
	if err != nil {
		t.Fatal(err)
	}
}
