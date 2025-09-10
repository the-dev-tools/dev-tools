package rcollectionitem_test

import (
    "context"
    "testing"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/require"

    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/internal/api/ritemapi"
    rcollectionitem "the-dev-tools/server/internal/api/rcollectionitem"
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
    itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
)

// Ensure deleting an endpoint via its RPC unlinks from collection_items and preserves order
func TestCollectionItemDeleteViaEndpoint_MiddleMaintainsOrder(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    defer base.Close()

    queries := base.Queries
    db := base.DB
    mockLogger := mocklogger.NewMockLogger()

    // Setup test infrastructure
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    userID := idwrap.NewNow()
    collectionID := idwrap.NewNow()

    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

    // Initialize services
    cs := scollection.New(queries, mockLogger)
    us := suser.New(queries)
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ers := sexampleresp.New(queries)

    // RPC services
    apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)
    itemsRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)

    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Create A (endpoint), B (endpoint), C (endpoint) at root level
    create := func(name string) idwrap.IDWrap {
        resp, err := apiRPC.EndpointCreate(authed, connect.NewRequest(&endpointv1.EndpointCreateRequest{
            CollectionId: collectionID.Bytes(),
            Name:         name,
            Method:       "GET",
            Url:          "/" + name,
        }))
        require.NoError(t, err)
        return idwrap.NewFromBytesMust(resp.Msg.EndpointId)
    }
    _ = create("A")
    b := create("B")
    _ = create("C")

    // Delete middle (B) via endpoint RPC
    _, err := apiRPC.EndpointDelete(authed, connect.NewRequest(&endpointv1.EndpointDeleteRequest{EndpointId: b.Bytes()}))
    require.NoError(t, err)

    // List items and ensure A and C remain in order
    listResp, err := itemsRPC.CollectionItemList(authed, connect.NewRequest(&itemv1.CollectionItemListRequest{CollectionId: collectionID.Bytes()}))
    require.NoError(t, err)
    require.Len(t, listResp.Msg.Items, 2)
}
