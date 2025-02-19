package collectionitem_test

import (
	"context"
	"testing"
	"the-dev-tools/backend/internal/api/collectionitem"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mitemfolder"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sitemfolder"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"

	"connectrpc.com/connect"
)

func TestCollectionItemRPC_CollectionItemList(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	cs := scollection.New(queries)
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
	for i := 0; i < folderCount; i++ {
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
	for i := 0; i < folderCount; i++ {
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

	serviceRPC := collectionitem.New(db, cs, us, ifs, ias, iaes, res)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	t.Run("Root items", func(t *testing.T) {
		reqData := &itemv1.CollectionItemListRequest{
			CollectionId: CollectionID.Bytes(),
			FolderId:     nil,
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
			CollectionId: CollectionID.Bytes(),
			FolderId:     lastFolderID.ID.Bytes(),
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
