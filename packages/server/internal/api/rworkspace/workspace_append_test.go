package rworkspace

import (
    "context"
    "log/slog"
    "testing"

    devtoolsdb "the-dev-tools/db"
    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/db/pkg/sqlitemem"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/muser"
    "the-dev-tools/server/pkg/service/senv"
    "the-dev-tools/server/pkg/service/suser"
    "the-dev-tools/server/pkg/service/sworkspace"
    sworkspacesusers "the-dev-tools/server/pkg/service/sworkspacesusers"
    workspacev1 "the-dev-tools/spec/dist/buf/go/workspace/v1"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/require"
    "google.golang.org/protobuf/types/known/emptypb"
)

func setupWorkspaceRPC(t *testing.T) (WorkspaceServiceRPC, context.Context, *gen.Queries, idwrap.IDWrap) {
    t.Helper()
    db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
    require.NoError(t, err)
    t.Cleanup(cleanup)

    queries, err := gen.Prepare(context.Background(), db)
    require.NoError(t, err)

    ws := sworkspace.New(queries)
    wus := sworkspacesusers.New(queries)
    us := suser.New(queries)
    es := senv.New(queries, slog.Default())

    srv := New(db, ws, wus, us, es)

    userID := idwrap.NewNow()
    ctx := mwauth.CreateAuthedContext(context.Background(), userID)
    // Create user
    require.NoError(t, us.CreateUser(ctx, &muser.User{ID: userID, Email: "t@ex", Password: []byte("x"), ProviderType: muser.Local, Status: muser.Active}))
    return srv, ctx, queries, userID
}

// NOTE: Import sworkspacesusers alias
//go:generate bash -lc ":"

// Cheap import alias fix
var _ = devtoolsdb.TxnRollback // keep import

func TestWorkspaceCreate_AppendsToEnd(t *testing.T) {
    t.Parallel()
    srv, ctx, _, _ := setupWorkspaceRPC(t)

    // Create two workspaces via RPC
    req1 := &connect.Request[workspacev1.WorkspaceCreateRequest]{Msg: &workspacev1.WorkspaceCreateRequest{Name: "W1"}}
    resp1, err := srv.WorkspaceCreate(ctx, req1)
    require.NoError(t, err)
    id1 := resp1.Msg.GetWorkspaceId()

    req2 := &connect.Request[workspacev1.WorkspaceCreateRequest]{Msg: &workspacev1.WorkspaceCreateRequest{Name: "W2"}}
    resp2, err := srv.WorkspaceCreate(ctx, req2)
    require.NoError(t, err)
    id2 := resp2.Msg.GetWorkspaceId()

    // List and assert order is [W1, W2]
    listResp, err := srv.WorkspaceList(ctx, &connect.Request[emptypb.Empty]{Msg: &emptypb.Empty{}})
    require.NoError(t, err)
    items := listResp.Msg.GetItems()
    require.GreaterOrEqual(t, len(items), 2)
    require.Equal(t, id1, items[0].GetWorkspaceId())
    require.Equal(t, id2, items[1].GetWorkspaceId())
}
