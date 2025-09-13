package rcollectionitem_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"

	"connectrpc.com/connect"
)

func TestCollectionItemRPC_CollectionItemList(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	cs := scollection.New(queries, mockLogger)
	cis := scollectionitem.New(queries, mockLogger)
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
    authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

    // Create folders via RPC to ensure collection_items mapping is created
    const folderCount int = 100
    frpc := ritemfolder.New(db, ifs, us, cs, cis)
    for i := 0; i < folderCount; i++ {
        req := connect.NewRequest(&folderv1.FolderCreateRequest{
            CollectionId: CollectionID.Bytes(),
            Name:         "test",
        })
        if _, err := frpc.FolderCreate(authedCtx, req); err != nil {
            t.Fatalf("failed to create root folder: %v", err)
        }
    }
    const itemCount int = 100
    for i := 0; i < itemCount; i++ {
        req := connect.NewRequest(&folderv1.FolderCreateRequest{
            CollectionId: CollectionID.Bytes(),
            Name:         "test",
        })
        if _, err := frpc.FolderCreate(authedCtx, req); err != nil {
            t.Fatalf("failed to create root folder (items): %v", err)
        }
    }

    serviceRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, res)
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

    // Fetch folders to identify a parent for nested creation
    // Use service to get last created folder
    rootFolders, err := ifs.GetFoldersWithCollectionID(ctx, CollectionID)
    if err != nil {
        t.Fatal(err)
    }
    if len(rootFolders) == 0 {
        t.Fatalf("expected root folders to exist")
    }
    lastFolderID := rootFolders[len(rootFolders)-1]
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

    // Create nested folders via RPC as well
    for _, nf := range nestedFolders {
        req := connect.NewRequest(&folderv1.FolderCreateRequest{
            CollectionId:   CollectionID.Bytes(),
            Name:           nf.Name,
            ParentFolderId: nf.ParentID.Bytes(),
        })
        if _, err := frpc.FolderCreate(authedCtx, req); err != nil {
            t.Fatalf("failed to create nested folder: %v", err)
        }
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
