package ritemapi_test

import (
    "context"
    "testing"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/internal/api/rcollectionitem"
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
    endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
    folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
    itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
)

// TestCrossCollectionFolderMoveRequestsVisible reproduces the bug where
// requests (endpoints) don’t show up after moving a folder to a different collection.
// It verifies that after moving a folder from collection A to collection B,
// the endpoints inside that folder are listed under collection B.
func TestCrossCollectionFolderMoveRequestsVisible(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    defer base.Close()

    queries := base.Queries
    db := base.DB
    logger := mocklogger.NewMockLogger()

    // Seed a workspace, user, and two collections in the same workspace
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    userID := idwrap.NewNow()
    collectionA := idwrap.NewNow()
    collectionB := idwrap.NewNow()

    baseServices := base.GetBaseServices()
    baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionA)
    // Reuse same workspace and user; Create second collection in same workspace
    baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionB)

    // Initialize services sharing same DB handle
    cs := scollection.New(queries, logger)
    us := suser.New(queries)
    cis := scollectionitem.New(queries, logger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ers := sexampleresp.New(queries)

    // RPCs
    collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)
    folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
    apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Create folder in collection A
    folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
        CollectionId:   collectionA.Bytes(),
        Name:           "Moved Folder",
        ParentFolderId: nil, // root in A
    })
    folderResp, err := folderRPC.FolderCreate(authed, folderReq)
    require.NoError(t, err)
    folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

    // Create endpoint inside that folder (in collection A)
    epReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
        CollectionId:   collectionA.Bytes(),
        Name:           "Inside Endpoint",
        Method:         "GET",
        Url:            "/inside",
        ParentFolderId: folderID.Bytes(),
    })
    _, err = apiRPC.EndpointCreate(authed, epReq)
    require.NoError(t, err)

    // Sanity: list items under folder in collection A
    listA := connect.NewRequest(&itemv1.CollectionItemListRequest{
        CollectionId:   collectionA.Bytes(),
        ParentFolderId: folderID.Bytes(),
    })
    listAResp, err := collectionItemRPC.CollectionItemList(authed, listA)
    require.NoError(t, err)
    require.Len(t, listAResp.Msg.Items, 1)
    assert.Equal(t, "Inside Endpoint", listAResp.Msg.Items[0].Endpoint.Name)

    // Cross-collection move: move the folder from A to B (to B root, end of list)
    moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
        // The RPC expects legacy IDs; server maps to collection_items IDs internally
        Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
        ItemId:             folderID.Bytes(),
        CollectionId:       collectionA.Bytes(),
        TargetCollectionId: collectionB.Bytes(),
        // No target item or parent folder -> add to end at B root
    })
    _, err = collectionItemRPC.CollectionItemMove(authed, moveReq)
    require.NoError(t, err)

    // Verify folder appears at collection B root
    listBRoot := connect.NewRequest(&itemv1.CollectionItemListRequest{
        CollectionId:   collectionB.Bytes(),
        ParentFolderId: nil,
    })
    listBRootResp, err := collectionItemRPC.CollectionItemList(authed, listBRoot)
    require.NoError(t, err)
    foundFolder := false
    for _, it := range listBRootResp.Msg.Items {
        if it.Kind == itemv1.ItemKind_ITEM_KIND_FOLDER && string(it.Folder.FolderId) == string(folderID.Bytes()) {
            foundFolder = true
            break
        }
    }
    assert.True(t, foundFolder, "moved folder should be visible at B root")

    // Critical assertion: endpoints inside moved folder should be visible in collection B
    listBFolder := connect.NewRequest(&itemv1.CollectionItemListRequest{
        CollectionId:   collectionB.Bytes(),
        ParentFolderId: folderID.Bytes(),
    })
    listBFolderResp, err := collectionItemRPC.CollectionItemList(authed, listBFolder)
    require.NoError(t, err)

    // This is the behavior we expect; currently this may fail if children
    // were not updated during cross-collection move.
    assert.Len(t, listBFolderResp.Msg.Items, 1, "endpoint should be listed under moved folder in target collection")
    if len(listBFolderResp.Msg.Items) > 0 {
        assert.Equal(t, "Inside Endpoint", listBFolderResp.Msg.Items[0].Endpoint.Name)
    }
}
