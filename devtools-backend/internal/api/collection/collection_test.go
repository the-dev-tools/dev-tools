package collection_test

import (
	"context"
	"devtools-backend/internal/api/collection"
	"devtools-backend/pkg/service/scollection"
	"devtools-nodes/pkg/model/mnodedata"
	collectionv1 "devtools-services/gen/collection/v1"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/oklog/ulid/v2"
)

func TestCollection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	service := collection.CollectionService{
		DB: db,
	}

	ctx := context.Background()

	id := ulid.Make()

	req := &connect.Request[collectionv1.GetNodeRequest]{
		Msg: &collectionv1.GetNodeRequest{
			Id: id.String(),
		},
	}

	/*
	   CREATE TABLE IF NOT EXISTS collection_nodes (
	           id TEXT PRIMARY KEY,
	           collection_id TEXT,
	           name TEXT,
	           type TEXT,
	           parent_id TEXT,
	           data TEXT,
	           FOREIGN KEY (collection_id) REFERENCES collections (id)
	   )
	*/
	// use collection_nodes table instead of nodes

	query := `
                SELECT id, collection_id, name, type, parent_id, data
                FROM collection_nodes
                WHERE id = ?
        `

	apiData := mnodedata.NodeApiRestData{
		Url:         "http://localhost:8080",
		QueryParams: map[string]string{"key": "value"},
		Method:      "GET",
		Headers:     map[string]string{"key": "value"},
		Body:        []byte("hello"),
	}

	byteData, err := json.Marshal(apiData)

	row := sqlmock.NewRows([]string{"id", "collection_id", "name", "type", "parent_id", "data"}).
		AddRow(id.Bytes(), id.Bytes(), "name", "type", "parent_id", byteData)

	ExpectPrepare := mock.ExpectPrepare(query)
	ExpectPrepare.
		WillBeClosed().
		ExpectQuery().
		WithArgs(id).
		WillReturnRows(row)

	/*
		apidata := mnodedata.NodeApiRestData{
			Url:         "http://localhost:8080",
			QueryParams: map[string]string{"key": "value"},
			Method:      "GET",
			Headers:     map[string]string{"key": "value"},
			Body:        []byte("hello"),
		}
	*/

	err = scollection.PrepareGetCollectionNode(db)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := service.GetNode(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Msg == nil {
		t.Fatalf("GetNode failed: invalid response")
	}

	if resp.Msg.Node == nil {
		t.Fatalf("GetNode failed: invalid response")
	}

	if resp.Msg.Node.Id != "1" {
		t.Fatalf("GetNode failed: invalid response")
	}
}
