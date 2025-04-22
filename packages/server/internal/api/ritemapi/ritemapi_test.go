package ritemapi_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"

	"connectrpc.com/connect"
)

func TestCreateItemApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB
	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	expectedCollectionID := CollectionID.Bytes()
	expectedParentID := []byte(nil)
	expectedName := "test"
	expectedUrl := "test"
	expectedMethod := "GET"

	req := connect.NewRequest(&endpointv1.EndpointCreateRequest{
		CollectionId:   expectedCollectionID,
		Name:           expectedName,
		Url:            expectedUrl,
		Method:         expectedMethod,
		ParentFolderId: expectedParentID,
	})

	rpcItemApi := ritemapi.New(db, ias, cs, ifs, us, iaes, ers)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcItemApi.EndpointCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg
	itemApiID, err := idwrap.NewFromBytes(msg.GetEndpointId())
	if err != nil {
		t.Fatal(err)
	}

	item, err := ias.GetItemApi(ctx, itemApiID)
	if err != nil {
		t.Fatal(err)
	}

	if item.Name != expectedName {
		t.Errorf("expected name %s, got %s", expectedName, item.Name)
	}

	if item.Url != expectedUrl {
		t.Errorf("expected url %s, got %s", expectedUrl, item.Url)
	}

	if item.Method != expectedMethod {
		t.Errorf("expected method %s, got %s", expectedMethod, item.Method)
	}

	if item.CollectionID != CollectionID {
		t.Errorf("expected collection id %s, got %s", CollectionID, item.CollectionID)
	}

	if item.FolderID != nil {
		t.Errorf("expected parent id %v, got %v", nil, item.FolderID)
	}
}

func TestGetItemApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	expectedName := "test"
	expectedUrl := "dev.tools"
	expectedMethod := "GET"

	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         expectedName,
		Url:          expectedUrl,
		Method:       expectedMethod,
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&endpointv1.EndpointGetRequest{
		EndpointId: item.ID.Bytes(),
	})

	rpcItemApi := ritemapi.New(db, ias, cs, ifs, us, iaes, ers)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcItemApi.EndpointGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg

	if msg.Name != expectedName {
		t.Errorf("expected name %s, got %s", expectedName, item.Name)
	}

	if msg.Url != expectedUrl {
		t.Errorf("expected url %s, got %s", expectedUrl, item.Url)
	}

	if msg.Method != expectedMethod {
		t.Errorf("expected method %s, got %s", expectedMethod, item.Method)
	}
}

func TestUpdateItemApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	expectedName := "test"
	expectedUrl := "dev.tools"
	expectedMethod := "GET"

	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         expectedName,
		Url:          expectedUrl,
		Method:       expectedMethod,
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	updatedName := "test2"
	updatedUrl := "dev.tools2"
	updatedMethod := "POST"

	req := connect.NewRequest(&endpointv1.EndpointUpdateRequest{
		EndpointId:     item.ID.Bytes(),
		Name:           &updatedName,
		Url:            &updatedUrl,
		Method:         &updatedMethod,
		ParentFolderId: nil,
	})

	rpcItemApi := ritemapi.New(db, ias, cs, ifs, us, iaes, ers)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcItemApi.EndpointUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	item, err = ias.GetItemApi(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}

	if item.Name != updatedName {
		t.Errorf("expected name %s, got %s", expectedName, item.Name)
	}

	if item.Url != updatedUrl {
		t.Errorf("expected url %s, got %s", expectedUrl, item.Url)
	}

	if item.Method != updatedMethod {
		t.Errorf("expected method %s, got %s", expectedMethod, item.Method)
	}
}

func TestDeleteItemApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	expectedName := "test"
	expectedUrl := "dev.tools"
	expectedMethod := "GET"

	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         expectedName,
		Url:          expectedUrl,
		Method:       expectedMethod,
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&endpointv1.EndpointDeleteRequest{
		EndpointId: item.ID.Bytes(),
	})

	rpcItemApi := ritemapi.New(db, ias, cs, ifs, us, iaes, ers)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcItemApi.EndpointDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	item, err = ias.GetItemApi(ctx, item.ID)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if item != nil {
		t.Errorf("expected item to be nil, got %v", item)
	}
}
