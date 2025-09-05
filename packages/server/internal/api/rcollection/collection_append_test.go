package rcollection

import (
    "context"
    "log/slog"
    "os"
    "testing"

    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/db/pkg/sqlitemem"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/muser"
    "the-dev-tools/server/pkg/model/mworkspace"
    "the-dev-tools/server/pkg/service/scollection"
    "the-dev-tools/server/pkg/service/suser"
    "the-dev-tools/server/pkg/service/sworkspace"
    collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/require"
)

// Lightweight RPC setup for append behavior tests
func setupAppendRPC(t *testing.T) (*CollectionServiceRPC, context.Context, idwrap.IDWrap, idwrap.IDWrap) {
    t.Helper()
    db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
    require.NoError(t, err)
    t.Cleanup(cleanup)

    queries, err := gen.Prepare(context.Background(), db)
    require.NoError(t, err)

    logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
    cs := scollection.New(queries, logger)
    ws := sworkspace.New(queries)
    us := suser.New(queries)

    rpc := New(db, cs, ws, us)

    userID := idwrap.NewNow()
    workspaceID := idwrap.NewNow()
    ctx := mwauth.CreateAuthedContext(context.Background(), userID)

    // Seed user + workspace + membership
    require.NoError(t, us.CreateUser(ctx, &muser.User{ID: userID, Email: "t@ex", Password: []byte("x"), ProviderType: muser.Local, Status: muser.Active}))
    require.NoError(t, ws.Create(ctx, &mworkspace.Workspace{ID: workspaceID, Name: "WS"}))
    require.NoError(t, queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{ID: idwrap.NewNow(), WorkspaceID: workspaceID, UserID: userID, Role: 1}))

    return &rpc, ctx, userID, workspaceID
}

func TestCollectionCreate_AppendsToEnd(t *testing.T) {
    t.Parallel()
    rpc, ctx, _, workspaceID := setupAppendRPC(t)

    // Create first collection
    req1 := &connect.Request[collectionv1.CollectionCreateRequest]{
        Msg: &collectionv1.CollectionCreateRequest{WorkspaceId: workspaceID.Bytes(), Name: "A"},
    }
    resp1, err := rpc.CollectionCreate(ctx, req1)
    require.NoError(t, err)
    id1 := resp1.Msg.GetCollectionId()

    // Create second collection
    req2 := &connect.Request[collectionv1.CollectionCreateRequest]{
        Msg: &collectionv1.CollectionCreateRequest{WorkspaceId: workspaceID.Bytes(), Name: "B"},
    }
    resp2, err := rpc.CollectionCreate(ctx, req2)
    require.NoError(t, err)
    id2 := resp2.Msg.GetCollectionId()

    // List and assert order is [A, B]
    listReq := &connect.Request[collectionv1.CollectionListRequest]{
        Msg: &collectionv1.CollectionListRequest{WorkspaceId: workspaceID.Bytes()},
    }
    listResp, err := rpc.CollectionList(ctx, listReq)
    require.NoError(t, err)
    items := listResp.Msg.GetItems()
    require.Len(t, items, 2)
    require.Equal(t, id1, items[0].GetCollectionId())
    require.Equal(t, id2, items[1].GetCollectionId())
}

