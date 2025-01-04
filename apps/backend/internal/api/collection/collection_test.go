package collection_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"the-dev-tools/backend/internal/api/collection"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcollection"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/testutil"
	collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"
	"time"

	"connectrpc.com/connect"
)

func TestCollectionGet(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	us := suser.New(queries)

	serviceRPC := collection.New(db, cs, ws, us)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	testCollectionID := idwrap.NewNow()
	collectionData := mcollection.Collection{
		ID:      testCollectionID,
		OwnerID: wsID,
		Name:    "test",
		Updated: time.Now(),
	}

	err := cs.CreateCollection(ctx, &collectionData)
	if err != nil {
		t.Error(err)
	}

	req := connect.NewRequest(
		&collectionv1.CollectionGetRequest{
			CollectionId: testCollectionID.Bytes(),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.CollectionGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}
	msg := resp.Msg

	if msg.CollectionId == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}

	respCollectionID, err := idwrap.NewFromBytes(msg.CollectionId)
	if err != nil {
		t.Fatal(err)
	}

	if testCollectionID.Compare(respCollectionID) != 0 {
		t.Fatalf("CollectionGet failed: id mismatch")
	}

	if msg.Name != collectionData.Name {
		t.Fatalf("CollectionGet failed: invalid response")
	}
}

func TestCollectionCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	us := suser.New(queries)

	serviceRPC := collection.New(db, cs, ws, us)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	collectionName := "test"
	req := connect.NewRequest(
		&collectionv1.CollectionCreateRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        collectionName,
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.CollectionCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}
	msg := resp.Msg

	if msg.CollectionId == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}
	id, err := idwrap.NewFromBytes(msg.CollectionId)
	if err != nil {
		t.Fatal(err)
	}
	collection, err := cs.GetCollection(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	if collection.Name != collectionName {
		t.Error("CollectionCreate failed: invalid response")
	}
}

func TestCollectionUpdate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	us := suser.New(queries)

	serviceRPC := collection.New(db, cs, ws, us)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	testCollectionID := idwrap.NewNow()
	collectionData := mcollection.Collection{
		ID:      testCollectionID,
		OwnerID: wsID,
		Name:    "test",
		Updated: time.Now(),
	}

	err := cs.CreateCollection(ctx, &collectionData)
	if err != nil {
		t.Error(err)
	}

	collectionNewName := "newName"

	req := connect.NewRequest(
		&collectionv1.CollectionUpdateRequest{
			CollectionId: testCollectionID.Bytes(),
			Name:         collectionNewName,
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.CollectionUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}

	collection, err := cs.GetCollection(ctx, testCollectionID)
	if err != nil {
		t.Fatal(err)
	}

	if collection.Name != collectionNewName {
		t.Error("name is not updated")
	}
}

func TestCollectionDelete(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	us := suser.New(queries)

	serviceRPC := collection.New(db, cs, ws, us)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	testCollectionID := idwrap.NewNow()
	collectionData := mcollection.Collection{
		ID:      testCollectionID,
		OwnerID: wsID,
		Name:    "test",
		Updated: time.Now(),
	}

	err := cs.CreateCollection(ctx, &collectionData)
	if err != nil {
		t.Error(err)
	}

	req := connect.NewRequest(
		&collectionv1.CollectionDeleteRequest{
			CollectionId: testCollectionID.Bytes(),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.CollectionDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}

	collection, err := cs.GetCollection(ctx, testCollectionID)
	if err == nil {
		t.Fatalf("collection is not deleted")
	}
	if err != scollection.ErrNoCollectionFound {
		t.Fatalf("returned error is not ErrNoCollectionFound")
	}
	if collection != nil {
		t.Fatalf("collection is not deleted")
	}
}

func TestCollectionImportHar(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	us := suser.New(queries)

	serviceRPC := collection.New(db, cs, ws, us)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	// print the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Current working directory: ", currentDir)

	filePath := "../../../test/har/test2.har"
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	harFileData := fileData

	req := connect.NewRequest(
		&collectionv1.CollectionImportHarRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        "test",
			Data:        harFileData,
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.CollectionImportHar(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}

	testCollectionID, err := idwrap.NewFromBytes(resp.Msg.CollectionId)
	if err != nil {
		t.Fatal(err)
	}

	collection, err := cs.GetCollection(ctx, testCollectionID)
	if err != nil {
		t.Fatal(err)
	}
	if collection == nil {
		t.Fatal("collection is nil")
	}
}
