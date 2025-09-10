package rcollectionitem

import (
    "context"
    "testing"
    "time"

    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/db/pkg/sqlitemem"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    rcollection "the-dev-tools/server/internal/api/rcollection"
    collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"
    ritemapi "the-dev-tools/server/internal/api/ritemapi"
    ritemfolder "the-dev-tools/server/internal/api/ritemfolder"
    "the-dev-tools/server/pkg/dbtime"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/muser"
    "the-dev-tools/server/pkg/model/mworkspace"
    scollection "the-dev-tools/server/pkg/service/scollection"
    sworkspace "the-dev-tools/server/pkg/service/sworkspace"
    "the-dev-tools/server/pkg/service/scollectionitem"
    sitemapi "the-dev-tools/server/pkg/service/sitemapi"
    sitemapiexample "the-dev-tools/server/pkg/service/sitemapiexample"
    sitemfolder "the-dev-tools/server/pkg/service/sitemfolder"
    suser "the-dev-tools/server/pkg/service/suser"
    sexampleresp "the-dev-tools/server/pkg/service/sexampleresp"
    itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
    folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
    endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/require"
)

// setup a minimal environment exposing collection item RPC
func setupCI(t *testing.T) (CollectionItemRPC, context.Context, idwrap.IDWrap, idwrap.IDWrap, *gen.Queries) {
    t.Helper()
    db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
    require.NoError(t, err)
    t.Cleanup(cleanup)

    queries, err := gen.Prepare(context.Background(), db)
    require.NoError(t, err)

    // base services
    cs := scollection.New(queries, nil)
    ws := sworkspace.New(queries)
    us := suser.New(queries)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    cis := scollectionitem.New(queries, nil)
    res := sexampleresp.New(queries)

    rpc := New(db, cs, cis, us, ifs, ias, iaes, res)

    user := idwrap.NewNow()
    ctx := mwauth.CreateAuthedContext(context.Background(), user)

    // seed user + workspace + membership
    require.NoError(t, us.CreateUser(ctx, &muser.User{ID: user, Email: "t@ex", Password: []byte("x"), ProviderType: muser.Local, Status: muser.Active}))
    wsid := idwrap.NewNow()
    require.NoError(t, ws.Create(ctx, &mworkspace.Workspace{ID: wsid, Name: "WS", Updated: dbtime.DBTime(time.Now())}))
    require.NoError(t, queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{ID: idwrap.NewNow(), WorkspaceID: wsid, UserID: user, Role: 1}))

    // seed a collection
    crpc := rcollection.New(db, cs, ws, us)
    _, err = crpc.CollectionCreate(ctx, &connect.Request[collectionv1.CollectionCreateRequest]{Msg: &collectionv1.CollectionCreateRequest{WorkspaceId: wsid.Bytes(), Name: "C"}})
    require.NoError(t, err)
    cols, err := cs.GetCollectionsOrdered(ctx, wsid)
    require.NoError(t, err)
    require.Len(t, cols, 1)
    return rpc, ctx, wsid, cols[0].ID, queries
}

func TestAppend_Folder_Then_Endpoint_Order(t *testing.T) {
    t.Parallel()
    rpc, ctx, _, collectionID, _ := setupCI(t)

    // Create Folder A then Folder B
    frpc := ritemfolder.New(rpc.DB, rpc.ifs, rpc.us, rpc.cs, rpc.cis)
    reqFA := &connect.Request[folderv1.FolderCreateRequest]{Msg: &folderv1.FolderCreateRequest{CollectionId: collectionID.Bytes(), Name: "A"}}
    _, err := frpc.FolderCreate(ctx, reqFA)
    require.NoError(t, err)
    reqFB := &connect.Request[folderv1.FolderCreateRequest]{Msg: &folderv1.FolderCreateRequest{CollectionId: collectionID.Bytes(), Name: "B"}}
    _, err = frpc.FolderCreate(ctx, reqFB)
    require.NoError(t, err)

    // Create Endpoint E (should append to end after folders)
    erpc := ritemapi.New(rpc.DB, rpc.ias, rpc.cs, rpc.ifs, rpc.us, rpc.iaes, rpc.res, rpc.cis)
    _, err = erpc.EndpointCreate(ctx, &connect.Request[endpointv1.EndpointCreateRequest]{Msg: &endpointv1.EndpointCreateRequest{CollectionId: collectionID.Bytes(), Name: "E", Url: "/", Method: "GET"}})
    require.NoError(t, err)

    // List collection items (ordered) and assert sequence is Folder A, Folder B, Endpoint E
    listResp, err := rpc.CollectionItemList(ctx, &connect.Request[itemv1.CollectionItemListRequest]{Msg: &itemv1.CollectionItemListRequest{CollectionId: collectionID.Bytes()}})
    require.NoError(t, err)
    items := listResp.Msg.GetItems()
    require.GreaterOrEqual(t, len(items), 3)
    require.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, items[0].GetKind())
    require.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, items[1].GetKind())
    require.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, items[2].GetKind())
}
