package rflow_test

import (
  "context"
  "testing"
  "time"

  "the-dev-tools/db/pkg/sqlc"
  "the-dev-tools/server/internal/api/middleware/mwauth"
  "the-dev-tools/server/internal/api/rflow"
  "the-dev-tools/server/internal/api/rnode"
  "the-dev-tools/server/internal/api/rnodeexecution"
  "the-dev-tools/server/pkg/flow/edge"
  "the-dev-tools/server/pkg/idwrap"
  "the-dev-tools/server/pkg/logconsole"
  "the-dev-tools/server/pkg/model/mflow"
  "the-dev-tools/server/pkg/model/mnnode"
  "the-dev-tools/server/pkg/model/mnnode/mnfor"
  "the-dev-tools/server/pkg/model/mnnode/mnnoop"
  "the-dev-tools/server/pkg/service/flow/sedge"
  "the-dev-tools/server/pkg/service/sexampleresp"
  "the-dev-tools/server/pkg/service/sexamplerespheader"
  "the-dev-tools/server/pkg/service/sassert"
  "the-dev-tools/server/pkg/service/sassertres"
  "the-dev-tools/server/pkg/service/sbodyform"
  "the-dev-tools/server/pkg/service/sbodyraw"
  "the-dev-tools/server/pkg/service/sbodyurl"
  "the-dev-tools/server/pkg/service/sexampleheader"
  "the-dev-tools/server/pkg/service/sexamplequery"
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
  nodeexecutionv1 "the-dev-tools/spec/dist/buf/go/flow/node/execution/v1"

  "connectrpc.com/connect"
  "github.com/stretchr/testify/require"
)

// Minimal cancellable stream
type cancellableStream struct{ ctx context.Context }
func (s cancellableStream) Send(_ *flowv1.FlowRunResponse) error { return s.ctx.Err() }

// TestForLoopCancel_VerifyNoRunningAndCanceledRecords ensures that after cancel in a for-loop flow,
// NodeList shows no RUNNING and NodeExecutionList contains terminal records for both loop and inner nodes.
func TestForLoopCancel_VerifyNoRunningAndCanceledRecords(t *testing.T) {
  ctx := context.Background()
  base := testutil.CreateBaseDB(ctx, t)
  defer sqlc.CloseQueriesAndLog(base.Queries)
  q := base.Queries
  db := base.DB

  // Core services
  ws := sworkspace.New(q)
  us := suser.New(q)
  ts := stag.New(q)
  fs := sflow.New(q)
  fts := sflowtag.New(q)
  fes := sedge.New(q)
  fvs := sflowvariable.New(q)
  ias := sitemapi.New(q)
  es := sitemapiexample.New(q)
  qs := sexamplequery.New(q)
  hs := sexampleheader.New(q)
  ns := snode.New(q)
  rns := snoderequest.New(q)
  fns := snodefor.New(q)
  fens := snodeforeach.New(q)
  sns := snodenoop.New(q)
  ins := snodeif.New(q)
  jsns := snodejs.New(q)
  nes := snodeexecution.New(q)
  ers := sexampleresp.New(q)
  erhs := sexamplerespheader.New(q)
  brs := sbodyraw.New(q)
  bfs := sbodyform.New(q)
  bues := sbodyurl.New(q)

  logMap := logconsole.NewLogChanMapWith(100000)

  flowRPC := rflow.New(db, ws, us, ts,
    fs, fts, fes, fvs,
    ias, es, qs, hs,
    brs, bfs, bues,
    ers, erhs, sassert.New(q), sassertres.New(q),
    ns, rns, fns, fens,
    sns, *ins, jsns, nes,
    logMap,
  )

  // Workspace/user/collection
  wsID := idwrap.NewNow()
  wsUserID := idwrap.NewNow()
  userID := idwrap.NewNow()
  collectionID := idwrap.NewNow()
  base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsUserID, userID, collectionID)

  // Create flow: start -> for(large) -> inner no-op
  flowID := idwrap.NewNow()
  require.NoError(t, fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: wsID, Name: "loop-cancel"}))

  startID := idwrap.NewNow()
  require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: startID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP}))
  require.NoError(t, sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START}))

  forID := idwrap.NewNow()
  require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: forID, FlowID: flowID, Name: "For", NodeKind: mnnode.NODE_KIND_FOR}))
  require.NoError(t, fns.CreateNodeFor(ctx, mnfor.MNFor{FlowNodeID: forID, IterCount: 1000}))

  innerID := idwrap.NewNow()
  require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{ID: innerID, FlowID: flowID, Name: "Inner", NodeKind: mnnode.NODE_KIND_NO_OP}))
  require.NoError(t, sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: innerID, Type: mnnoop.NODE_NO_OP_KIND_LOOP}))

  require.NoError(t, fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: forID, SourceHandler: edge.HandleUnspecified}))
  require.NoError(t, fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: forID, TargetID: innerID, SourceHandler: edge.HandleLoop}))

  // Run & cancel mid-iterations
  req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes(), EnvironmentId: idwrap.NewNow().Bytes()})
  authed := mwauth.CreateAuthedContext(ctx, userID)

  cancelCtx, cancel := context.WithCancel(context.Background())
  stream := cancellableStream{ctx: cancelCtx}
  errCh := make(chan error, 1)
  go func() { errCh <- flowRPC.FlowRunAdHoc(authed, req, stream) }()
  time.Sleep(200 * time.Millisecond)
  cancel()
  select {
  case <-errCh:
  case <-time.After(5 * time.Second):
    t.Fatal("timeout waiting run cancel")
  }

  // NodeList: no node should be RUNNING
  nodeRPC := rnode.NewNodeServiceRPC(db, us, fs, *ins, rns, fns, fens, ns, sns, jsns, ias, es, qs, hs, brs, bfs, bues, nes)
  listReq := connect.NewRequest(&nodev1.NodeListRequest{FlowId: flowID.Bytes()})
  listResp, err := nodeRPC.NodeList(mwauth.CreateAuthedContext(ctx, userID), listReq)
  require.NoError(t, err)
  for _, it := range listResp.Msg.Items {
    if it.State == nodev1.NodeState_NODE_STATE_RUNNING {
      t.Fatalf("node %v still RUNNING after cancel", it.GetNodeId())
    }
  }

  // NodeExecutionList: for node should have a canceled record even if main loop skipped
  nodeExecRPC := rnodeexecution.New(&nes, &ns, &fs, &us, &ers, &rns)
  execList := func(nodeID idwrap.IDWrap) []*nodeexecutionv1.NodeExecutionListItem {
    lr := connect.NewRequest(&nodeexecutionv1.NodeExecutionListRequest{NodeId: nodeID.Bytes()})
    resp, e := nodeExecRPC.NodeExecutionList(mwauth.CreateAuthedContext(ctx, userID), lr)
    require.NoError(t, e)
    return resp.Msg.Items
  }

  // For node: expect at least one terminal record (CANCELED)
  forExecs := execList(forID)
  if len(forExecs) == 0 {
    t.Fatalf("expected at least one execution record for for node")
  }
  // Latest first; state must not be RUNNING
  if forExecs[0].State == nodev1.NodeState_NODE_STATE_RUNNING {
    t.Fatalf("for node latest state still RUNNING")
  }

  // Inner node: expect at least one execution, latest not RUNNING
  innerExecs := execList(innerID)
  if len(innerExecs) == 0 {
    t.Fatalf("expected at least one execution record for inner node")
  }
  if innerExecs[0].State == nodev1.NodeState_NODE_STATE_RUNNING {
    t.Fatalf("inner node latest state still RUNNING")
  }
}

// no-op
