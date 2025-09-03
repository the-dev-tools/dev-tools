package rflow_test

import (
  "context"
  "testing"
  "time"

  "the-dev-tools/db/pkg/sqlc"
  "the-dev-tools/server/internal/api/middleware/mwauth"
  "the-dev-tools/server/internal/api/rflow"
  "the-dev-tools/server/internal/api/rnode"
  "the-dev-tools/server/pkg/flow/edge"
  "the-dev-tools/server/pkg/idwrap"
  "the-dev-tools/server/pkg/logconsole"
  "the-dev-tools/server/pkg/model/mflow"
  "the-dev-tools/server/pkg/model/mnnode"
  "the-dev-tools/server/pkg/model/mnnode/mnnoop"
  "the-dev-tools/server/pkg/model/mnnode/mnfor"
  "the-dev-tools/server/pkg/service/flow/sedge"
  "the-dev-tools/server/pkg/service/sbodyform"
  "the-dev-tools/server/pkg/service/sbodyraw"
  "the-dev-tools/server/pkg/service/sbodyurl"
  "the-dev-tools/server/pkg/service/sexampleheader"
  "the-dev-tools/server/pkg/service/sexamplequery"
  "the-dev-tools/server/pkg/service/sassert"
  "the-dev-tools/server/pkg/service/sassertres"
  "the-dev-tools/server/pkg/service/sexampleresp"
  "the-dev-tools/server/pkg/service/sexamplerespheader"
  "the-dev-tools/server/pkg/service/sflow"
  "the-dev-tools/server/pkg/service/sflowtag"
  "the-dev-tools/server/pkg/service/sflowvariable"
  "the-dev-tools/server/pkg/service/sitemapi"
  "the-dev-tools/server/pkg/service/sitemapiexample"
  "the-dev-tools/server/pkg/service/snode"
  "the-dev-tools/server/pkg/service/snodeexecution"
  "the-dev-tools/server/pkg/service/snodefor"
  "the-dev-tools/server/pkg/service/snodeforeach"
  "the-dev-tools/server/pkg/service/snodeif"
  "the-dev-tools/server/pkg/service/snodejs"
  "the-dev-tools/server/pkg/service/snodenoop"
  "the-dev-tools/server/pkg/service/snoderequest"
  "the-dev-tools/server/pkg/service/stag"
  "the-dev-tools/server/pkg/service/suser"
  "the-dev-tools/server/pkg/service/sworkspace"
  "the-dev-tools/server/pkg/testutil"
  flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
  nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"

  "connectrpc.com/connect"
  "github.com/stretchr/testify/require"
)

// minimal stream mock that supports cancellation via context
type cancelStreamMock struct{ ctx context.Context }

func (s cancelStreamMock) Send(_ *flowv1.FlowRunResponse) error { return s.ctx.Err() }

// TestCancelDoesNotLeaveRunningState verifies that after cancel, NodeList does not return RUNNING states
func TestCancelDoesNotLeaveRunningState(t *testing.T) {
  ctx := context.Background()

  base := testutil.CreateBaseDB(ctx, t)
  defer sqlc.CloseQueriesAndLog(base.Queries)
  queries := base.Queries
  db := base.DB

  // Core services
  ws := sworkspace.New(queries)
  us := suser.New(queries)
  ts := stag.New(queries)
  fs := sflow.New(queries)
  fts := sflowtag.New(queries)
  fes := sedge.New(queries)
  fvs := sflowvariable.New(queries)
  ias := sitemapi.New(queries)
  es := sitemapiexample.New(queries)
  qs := sexamplequery.New(queries)
  hs := sexampleheader.New(queries)
  brs := sbodyraw.New(queries)
  bfs := sbodyform.New(queries)
  bues := sbodyurl.New(queries)
  ers := sexampleresp.New(queries)
  erhs := sexamplerespheader.New(queries)
  ns := snode.New(queries)
  rns := snoderequest.New(queries)
  fns := snodefor.New(queries)
  fens := snodeforeach.New(queries)
  sns := snodenoop.New(queries)
  ins := snodeif.New(queries)
  jsns := snodejs.New(queries)
  nes := snodeexecution.New(queries)

  logChanMap := logconsole.NewLogChanMapWith(10000)

  flowSrv := rflow.New(db, ws, us, ts,
    fs, fts, fes, fvs,
    ias, es, qs, hs,
    brs, bfs, bues,
    ers, erhs, sassert.New(queries), sassertres.New(queries),
    ns, rns, fns, fens,
    sns, *ins, jsns, nes,
    logChanMap,
  )

  // Node RPC for NodeList state verification
  nodeRPC := rnode.NewNodeServiceRPC(db, us, fs, *ins, rns, fns, fens, ns, sns, jsns, ias, es, qs, hs, brs, bfs, bues, nes)

  // Workspace and user
  wsID := idwrap.NewNow()
  wsUserID := idwrap.NewNow()
  userID := idwrap.NewNow()
  collectionID := idwrap.NewNow()
  base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsUserID, userID, collectionID)

  // Flow with start -> FOR(loop) -> inner noop
  flowID := idwrap.NewNow()
  require.NoError(t, fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: wsID, Name: "cancel_e2e"}))

  startID := idwrap.NewNow()
  require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: startID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP}))
  require.NoError(t, sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START}))

  forID := idwrap.NewNow()
  require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: forID, FlowID: flowID, Name: "Loop", NodeKind: mnnode.NODE_KIND_FOR}))
  require.NoError(t, fns.CreateNodeFor(ctx, mnfor.MNFor{FlowNodeID: forID, IterCount: 1000}))

  innerID := idwrap.NewNow()
  require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: innerID, FlowID: flowID, Name: "Inner", NodeKind: mnnode.NODE_KIND_NO_OP}))
  require.NoError(t, sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: innerID, Type: mnnoop.NODE_NO_OP_KIND_LOOP}))

  require.NoError(t, fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: forID, SourceHandler: edge.HandleUnspecified}))
  require.NoError(t, fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: forID, TargetID: innerID, SourceHandler: edge.HandleLoop}))

  // Run flow and cancel immediately after it starts
  req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes(), EnvironmentId: idwrap.NewNow().Bytes()})
  authed := mwauth.CreateAuthedContext(ctx, userID)

  cancelCtx, cancel := context.WithCancel(context.Background())
  stream := cancelStreamMock{ctx: cancelCtx}

  errCh := make(chan error, 1)
  go func() { errCh <- flowSrv.FlowRunAdHoc(authed, req, stream) }()

  // give it a brief moment to start iterations, then cancel
  time.Sleep(200 * time.Millisecond)
  cancel()

  select {
  case <-errCh:
    // proceed
  case <-time.After(3 * time.Second):
    t.Fatal("flow run did not finish after cancellation")
  }

  // Query NodeList and assert no RUNNING states remain
  listReq := connect.NewRequest(&nodev1.NodeListRequest{FlowId: flowID.Bytes()})
  listResp, err := nodeRPC.NodeList(mwauth.CreateAuthedContext(ctx, userID), listReq)
  require.NoError(t, err)

  for _, it := range listResp.Msg.Items {
    if it.State == nodev1.NodeState_NODE_STATE_RUNNING {
      t.Fatalf("node still RUNNING after cancel")
    }
  }
}

// no-op
