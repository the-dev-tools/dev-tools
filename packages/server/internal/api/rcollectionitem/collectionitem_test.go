package rcollectionitem_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"

	"connectrpc.com/connect"
)

func TestCollectionItemRPC_CollectionItemList(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	res := sexampleresp.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	var folders []mitemfolder.ItemFolder
	const folderCount int = 100
	for range folderCount {
		folder := mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			Name:         "test",
			CollectionID: CollectionID,
			ParentID:     nil,
		}
		folders = append(folders, folder)
	}

	var items []mitemfolder.ItemFolder
	const itemCount int = 100
	for range folderCount {
		item := mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			Name:         "test",
			CollectionID: CollectionID,
			ParentID:     nil,
		}
		items = append(items, item)
	}

	err := ifs.CreateItemFolderBulk(ctx, folders)
	if err != nil {
		t.Error(err)
	}

	err = ifs.CreateItemFolderBulk(ctx, items)
	if err != nil {
		t.Error(err)
	}

	serviceRPC := rcollectionitem.New(db, cs, us, ifs, ias, iaes, res)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	t.Run("Root items", func(t *testing.T) {
		reqData := &itemv1.CollectionItemListRequest{
			CollectionId:   CollectionID.Bytes(),
			ParentFolderId: nil,
		}
		req := connect.NewRequest(reqData)

		resp, err := serviceRPC.CollectionItemList(authedCtx, req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Msg == nil {
			t.Fatalf("CollectionItemList failed: invalid response")
		}

		if resp.Msg.Items == nil {
			t.Fatalf("CollectionItemList failed: invalid response")
		}

		if len(resp.Msg.Items) != folderCount+itemCount {
			t.Fatalf("CollectionItemList failed: invalid response")
		}
	})

	lastFolderID := folders[len(folders)-1]
	const nestedFolderCount int = 100
	var nestedFolders []mitemfolder.ItemFolder
	for i := 0; i < nestedFolderCount; i++ {
		folder := mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			Name:         "test",
			CollectionID: CollectionID,
			ParentID:     &lastFolderID.ID,
		}
		nestedFolders = append(nestedFolders, folder)
	}

	err = ifs.CreateItemFolderBulk(ctx, nestedFolders)
	if err != nil {
		t.Error(err)
	}

	t.Run("Nested items", func(t *testing.T) {
		reqData := &itemv1.CollectionItemListRequest{
			CollectionId:   CollectionID.Bytes(),
			ParentFolderId: lastFolderID.ID.Bytes(),
		}
		req := connect.NewRequest(reqData)

		resp, err := serviceRPC.CollectionItemList(authedCtx, req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Msg == nil {
			t.Fatalf("CollectionItemList failed: invalid response")
		}

		if resp.Msg.Items == nil {
			t.Fatalf("CollectionItemList failed: invalid response")
		}

		if len(resp.Msg.Items) != nestedFolderCount {
			t.Fatalf("CollectionItemList failed: invalid response")
		}
	})
}
