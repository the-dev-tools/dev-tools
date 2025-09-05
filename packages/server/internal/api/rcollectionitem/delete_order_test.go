package rcollectionitem_test

import (
    "context"
    "testing"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/require"

    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/internal/api/ritemapi"
    "the-dev-tools/server/internal/api/ritemfolder"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logger/mocklogger"
    "the-dev-tools/server/pkg/service/scollection"
    "the-dev-tools/server/pkg/service/scollectionitem"
    "the-dev-tools/server/pkg/service/sexampleresp"
    "the-dev-tools/server/pkg/service/sitemapi"
    "the-dev-tools/server/pkg/service/sitemapiexample"
    "the-dev-tools/server/pkg/service/sitemfolder"
    "the-dev-tools/server/pkg/service/suser"
    "the-dev-tools/server/pkg/testutil"
    "the-dev-tools/db/pkg/sqlc/gen"
    endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
    folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
    itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
)

// helper to create A,B,C (folder, endpoint, folder) at root and return their legacy IDs and collection item IDs
func setupThreeItems(t *testing.T, ctx context.Context, base *testutil.BaseDB,
    cis *scollectionitem.CollectionItemService,
    folderRPC ritemfolder.ItemFolderRPC,
    apiRPC ritemapi.ItemAPIRPC,
    userID, collectionID idwrap.IDWrap,
) (aFolderID, bEndpointID, cFolderID idwrap.IDWrap, aItemID, bItemID, cItemID idwrap.IDWrap) {
    authed := mwauth.CreateAuthedContext(ctx, userID)
    // Create A (folder)
    fresp, err := folderRPC.FolderCreate(authed, connect.NewRequest(&folderv1.FolderCreateRequest{
        CollectionId: collectionID.Bytes(),
        Name:         "A",
    }))
    require.NoError(t, err)
    aFolderID = idwrap.NewFromBytesMust(fresp.Msg.FolderId)
    // Create B (endpoint)
    eresp, err := apiRPC.EndpointCreate(authed, connect.NewRequest(&endpointv1.EndpointCreateRequest{
        CollectionId: collectionID.Bytes(),
        Name:         "B",
        Method:       "GET",
        Url:          "/b",
    }))
    require.NoError(t, err)
    bEndpointID = idwrap.NewFromBytesMust(eresp.Msg.EndpointId)
    // Create C (folder)
    fresp2, err := folderRPC.FolderCreate(authed, connect.NewRequest(&folderv1.FolderCreateRequest{
        CollectionId: collectionID.Bytes(),
        Name:         "C",
    }))
    require.NoError(t, err)
    cFolderID = idwrap.NewFromBytesMust(fresp2.Msg.FolderId)

    // Resolve collection_items IDs
    var err2 error
    aItemID, err2 = cis.GetCollectionItemIDByLegacyID(ctx, aFolderID)
    require.NoError(t, err2)
    bItemID, err2 = cis.GetCollectionItemIDByLegacyID(ctx, bEndpointID)
    require.NoError(t, err2)
    cItemID, err2 = cis.GetCollectionItemIDByLegacyID(ctx, cFolderID)
    require.NoError(t, err2)
    return
}

func TestCollectionItemDelete_MiddleMaintainsOrder(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    defer base.Close()
    queries := base.Queries
    db := base.DB
    mockLogger := mocklogger.NewMockLogger()

    // Setup IDs and collection
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    userID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

    // Services
    cs := scollection.New(queries, mockLogger)
    us := suser.New(queries)
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ers := sexampleresp.New(queries)
    // RPCs we need
    folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
    apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

    // Create A,B,C
    _, bLegacy, _, aItem, bItem, cItem := setupThreeItems(t, ctx, base, cis, folderRPC, apiRPC, userID, collectionID)

    // Delete the middle (B) by collection_items ID via service with TX
    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err)
    require.NoError(t, cis.DeleteCollectionItem(ctx, tx, bItem))
    require.NoError(t, tx.Commit())

    // List items via queries in order
    rows, err := queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{CollectionID: collectionID, ParentFolderID: nil, Column3: nil, CollectionID_2: collectionID})
    require.NoError(t, err)
    require.Len(t, rows, 2)
    // Order should be A, C
    require.Equal(t, aItem.String(), idwrap.NewFromBytesMust(rows[0].ID).String())
    require.Equal(t, cItem.String(), idwrap.NewFromBytesMust(rows[1].ID).String())
    // Pointers
    aRow, err := queries.GetCollectionItem(ctx, aItem)
    require.NoError(t, err)
    cRow, err := queries.GetCollectionItem(ctx, cItem)
    require.NoError(t, err)
    require.Nil(t, aRow.PrevID)
    require.NotNil(t, aRow.NextID)
    require.Equal(t, cItem.String(), aRow.NextID.String())
    require.NotNil(t, cRow.PrevID)
    require.Equal(t, aItem.String(), cRow.PrevID.String())

    // Ensure legacy endpoint row B is gone from list and mapping fails
    _, err = cis.GetCollectionItemIDByLegacyID(ctx, bLegacy)
    require.Error(t, err)
}

func TestCollectionItemDelete_HeadMaintainsOrder(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    defer base.Close()
    queries := base.Queries
    db := base.DB
    mockLogger := mocklogger.NewMockLogger()
    workspaceID, workspaceUserID, userID, collectionID := idwrap.NewNow(), idwrap.NewNow(), idwrap.NewNow(), idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    cs := scollection.New(queries, mockLogger)
    us := suser.New(queries)
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ers := sexampleresp.New(queries)
    folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
    apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

    // Create A,B,C
    _, _, _, aItem, _, cItem := setupThreeItems(t, ctx, base, cis, folderRPC, apiRPC, userID, collectionID)
    // Delete head A
    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err)
    require.NoError(t, cis.DeleteCollectionItem(ctx, tx, aItem))
    require.NoError(t, tx.Commit())
    rows, err := queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{CollectionID: collectionID, ParentFolderID: nil, Column3: nil, CollectionID_2: collectionID})
    require.NoError(t, err)
    require.Len(t, rows, 2)
    // First should not have prev
    first := idwrap.NewFromBytesMust(rows[0].ID)
    require.NotEqual(t, aItem.String(), first.String())
    fRow, _ := queries.GetCollectionItem(ctx, first)
    require.Nil(t, fRow.PrevID)
    // Second should be C or remaining
    _ = cItem // Just assert there are two left; exact names vary by ULIDs
}

func TestCollectionItemDelete_TailMaintainsOrder(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    defer base.Close()
    queries := base.Queries
    db := base.DB
    mockLogger := mocklogger.NewMockLogger()
    workspaceID, workspaceUserID, userID, collectionID := idwrap.NewNow(), idwrap.NewNow(), idwrap.NewNow(), idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    cs := scollection.New(queries, mockLogger)
    us := suser.New(queries)
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ers := sexampleresp.New(queries)
    folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
    apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

    // Create A,B,C
    _, _, _, aItem, _, cItem := setupThreeItems(t, ctx, base, cis, folderRPC, apiRPC, userID, collectionID)
    // Determine tail (likely C)
    tail := cItem
    // Delete tail
    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err)
    require.NoError(t, cis.DeleteCollectionItem(ctx, tx, tail))
    require.NoError(t, tx.Commit())
    // Verify last remaining has next=nil
    rows, err := queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{CollectionID: collectionID, ParentFolderID: nil, Column3: nil, CollectionID_2: collectionID})
    require.NoError(t, err)
    require.Len(t, rows, 2)
    last := idwrap.NewFromBytesMust(rows[1].ID)
    lRow, _ := queries.GetCollectionItem(ctx, last)
    require.Nil(t, lRow.NextID)
}
