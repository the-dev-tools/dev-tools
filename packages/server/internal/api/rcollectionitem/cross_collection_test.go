package rcollectionitem_test

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

// TestCollectionItemMove_CrossCollectionFolder_Positive verifies that moving a folder
// to a different collection also makes its endpoints visible in the target collection.
func TestCollectionItemMove_CrossCollectionFolder_Positive(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    defer base.Close()

    queries := base.Queries
    db := base.DB
    logger := mocklogger.NewMockLogger()

    // Setup workspace, user, and two collections in the same workspace
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    userID := idwrap.NewNow()
    collectionA := idwrap.NewNow()
    collectionB := idwrap.NewNow()

    baseServices := base.GetBaseServices()
    baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionA)
    baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionB)

    // Initialize services
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

    // Create folder in A (root)
    folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
        CollectionId:   collectionA.Bytes(),
        Name:           "A Folder",
        ParentFolderId: nil,
    })
    folderResp, err := folderRPC.FolderCreate(authed, folderReq)
    require.NoError(t, err)
    folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

    // Create an endpoint inside that folder
    endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
        CollectionId:   collectionA.Bytes(),
        Name:           "Inside Endpoint",
        Method:         "GET",
        Url:            "/inside",
        ParentFolderId: folderID.Bytes(),
    })
    _, err = apiRPC.EndpointCreate(authed, endpointReq)
    require.NoError(t, err)

    // Move the folder to collection B (to B root)
    moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
        Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
        ItemId:             folderID.Bytes(),
        CollectionId:       collectionA.Bytes(),
        TargetCollectionId: collectionB.Bytes(),
    })
    _, err = collectionItemRPC.CollectionItemMove(authed, moveReq)
    require.NoError(t, err)

    // Verify folder appears at B root
    listBRootReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
        CollectionId:   collectionB.Bytes(),
        ParentFolderId: nil,
    })
    listBRoot, err := collectionItemRPC.CollectionItemList(authed, listBRootReq)
    require.NoError(t, err)
    foundFolder := false
    for _, it := range listBRoot.Msg.Items {
        if it.Kind == itemv1.ItemKind_ITEM_KIND_FOLDER && string(it.Folder.FolderId) == string(folderID.Bytes()) {
            foundFolder = true
            break
        }
    }
    assert.True(t, foundFolder, "moved folder should be visible at B root")

    // Verify endpoint inside moved folder is visible under collection B
    listBFolderReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
        CollectionId:   collectionB.Bytes(),
        ParentFolderId: folderID.Bytes(),
    })
    listBFolder, err := collectionItemRPC.CollectionItemList(authed, listBFolderReq)
    require.NoError(t, err)
    require.Len(t, listBFolder.Msg.Items, 1)
    assert.Equal(t, "Inside Endpoint", listBFolder.Msg.Items[0].Endpoint.Name)
}

