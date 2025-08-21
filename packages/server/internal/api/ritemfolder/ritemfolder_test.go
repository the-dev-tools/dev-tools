package ritemfolder_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"

	"connectrpc.com/connect"
)

func TestCreateItemFolder(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()
	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, mockLogger)

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

	req := connect.NewRequest(&folderv1.FolderCreateRequest{
		CollectionId:   expectedCollectionID,
		Name:           expectedName,
		ParentFolderId: expectedParentID,
	})

	rpcItemFolder := ritemfolder.New(db, ifs, us, cs, cis)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcItemFolder.FolderCreate(authedCtx, req)
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
	itemApiID, err := idwrap.NewFromBytes(msg.GetFolderId())
	if err != nil {
		t.Fatal(err)
	}

	item, err := ifs.GetFolder(ctx, itemApiID)
	if err != nil {
		t.Fatal(err)
	}

	if item.Name != expectedName {
		t.Errorf("expected name %s, got %s", expectedName, item.Name)
	}

	if item.CollectionID != CollectionID {
		t.Errorf("expected collection id %s, got %s", CollectionID, item.CollectionID)
	}

	if item.ParentID != nil {
		t.Errorf("expected parent id %v, got %v", nil, item.ParentID)
	}
}

func TestUpdateItemFolder(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, mockLogger)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	expectedName := "test"

	item := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		Name:         expectedName,
		CollectionID: CollectionID,
		ParentID:     nil,
	}

	err := ifs.CreateItemFolder(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	updatedName := "test2"

	req := connect.NewRequest(&folderv1.FolderUpdateRequest{
		FolderId:       item.ID.Bytes(),
		Name:           &updatedName,
		ParentFolderId: nil,
	})

	rpcItemFolder := ritemfolder.New(db, ifs, us, cs, cis)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcItemFolder.FolderUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	item, err = ifs.GetFolder(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}

	if item.Name != updatedName {
		t.Errorf("expected name %s, got %s", expectedName, item.Name)
	}
}

func TestDeleteItemFolder(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, mockLogger)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	item := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		Name:         "test",
		CollectionID: CollectionID,
		ParentID:     nil,
	}

	err := ifs.CreateItemFolder(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&folderv1.FolderDeleteRequest{
		FolderId: item.ID.Bytes(),
	})

	rpcItemFolder := ritemfolder.New(db, ifs, us, cs, cis)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcItemFolder.FolderDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	item, err = ifs.GetFolder(ctx, item.ID)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if item != nil {
		t.Errorf("expected item to be nil, got %v", item)
	}
}
