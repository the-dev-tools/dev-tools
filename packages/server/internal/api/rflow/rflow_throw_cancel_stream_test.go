package rflow_test

import (
    "context"
    "testing"
    "time"
    "the-dev-tools/db/pkg/sqlc"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/pkg/flow/edge"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/mflow"
    "the-dev-tools/server/pkg/model/mnnode"
    "the-dev-tools/server/pkg/model/mnnode/mnfor"
    "the-dev-tools/server/pkg/model/mnnode/mnif"
    "the-dev-tools/server/pkg/model/mcondition"
    "the-dev-tools/server/pkg/model/mnnode/mnnoop"
    flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
    nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
    "the-dev-tools/server/pkg/service/snodeif"

    "connectrpc.com/connect"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestLoopThrowShowsCanceledInStream ensures that when an inner node errors and the FOR loop is configured
// to throw (propagate error), the streamed node state for the loop is CANCELED, not SUCCESS.
func TestLoopThrowShowsCanceledInStream(t *testing.T) {
    ctx := context.Background()
    services, base, wsID, userID := createTestService(t, ctx)
    defer sqlc.CloseQueriesAndLog(base.Queries)

    // Create flow
    flowID := idwrap.NewNow()
    require.NoError(t, services.fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: wsID, Name: "loop-throw-stream"}))

    // Start node
    startID := idwrap.NewNow()
    require.NoError(t, services.ns.CreateNode(ctx, mnnode.MNode{ID: startID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP}))
    require.NoError(t, services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START}))

    // FOR loop node
    forID := idwrap.NewNow()
    require.NoError(t, services.ns.CreateNode(ctx, mnnode.MNode{ID: forID, FlowID: flowID, Name: "For", NodeKind: mnnode.NODE_KIND_FOR}))
    // Create with default, then update to set ErrorHandling UNSPECIFIED (throw)
    require.NoError(t, services.fns.CreateNodeFor(ctx, mnfor.MNFor{FlowNodeID: forID, IterCount: 5}))
    require.NoError(t, services.fns.UpdateNodeFor(ctx, mnfor.MNFor{FlowNodeID: forID, IterCount: 5, ErrorHandling: mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED}))

    // IF node inside loop that will error during condition evaluation
    ifID := idwrap.NewNow()
    require.NoError(t, services.ns.CreateNode(ctx, mnnode.MNode{ID: ifID, FlowID: flowID, Name: "Breaker", NodeKind: mnnode.NODE_KIND_CONDITION}))
    // Expression references unknown variable to trigger evaluation error
    ins := snodeif.New(base.Queries)
    require.NoError(t, ins.CreateNodeIf(ctx, mnif.MNIF{FlowNodeID: ifID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "unknownVar > 0"}}}))

    // True/False sinks to satisfy IF branches (won't be reached)
    trueSinkID := idwrap.NewNow()
    require.NoError(t, services.ns.CreateNode(ctx, mnnode.MNode{ID: trueSinkID, FlowID: flowID, Name: "TrueSink", NodeKind: mnnode.NODE_KIND_NO_OP}))
    require.NoError(t, services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: trueSinkID, Type: mnnoop.NODE_NO_OP_KIND_LOOP}))

    falseSinkID := idwrap.NewNow()
    require.NoError(t, services.ns.CreateNode(ctx, mnnode.MNode{ID: falseSinkID, FlowID: flowID, Name: "FalseSink", NodeKind: mnnode.NODE_KIND_NO_OP}))
    require.NoError(t, services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: falseSinkID, Type: mnnoop.NODE_NO_OP_KIND_LOOP}))

    // Wire edges: start -> for ; for -> if (loop); if THEN -> trueSink ; if ELSE -> falseSink
    edges := []edge.Edge{
        {ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: forID, SourceHandler: edge.HandleUnspecified},
        {ID: idwrap.NewNow(), FlowID: flowID, SourceID: forID, TargetID: ifID, SourceHandler: edge.HandleLoop},
        {ID: idwrap.NewNow(), FlowID: flowID, SourceID: ifID, TargetID: trueSinkID, SourceHandler: edge.HandleThen},
        {ID: idwrap.NewNow(), FlowID: flowID, SourceID: ifID, TargetID: falseSinkID, SourceHandler: edge.HandleElse},
    }
    for _, e := range edges {
        require.NoError(t, services.fes.CreateEdge(ctx, e))
    }

    // Run
    req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes(), EnvironmentId: idwrap.NewNow().Bytes()})
    stream := NewCancellableStreamMock(t)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    errCh := make(chan error, 1)
    go func() { errCh <- services.serviceRPC.FlowRunAdHoc(authed, req, stream) }()

    // Wait for completion (errors expected due to throw)
    select {
    case <-time.After(5 * time.Second):
        t.Fatal("timeout waiting for flow to finish")
    case <-errCh:
    }

    // Inspect streamed messages for the FOR node
    all := stream.GetMessages()
    // Debug: dump all node state messages
    for _, m := range all {
        if m.GetNode() != nil {
            t.Logf("stream node id=%x state=%v", m.GetNode().GetNodeId(), m.GetNode().GetState())
        }
    }
    var states []nodev1.NodeState
    for _, m := range all {
        if m.GetNode() == nil {
            continue
        }
        if string(m.GetNode().GetNodeId()) == string(forID.Bytes()) {
            states = append(states, m.GetNode().GetState())
        }
    }

    if len(states) == 0 {
        t.Fatalf("no streamed states captured for FOR node")
    }
    t.Logf("streamed states for FOR: %v", states)

    // Expect RUNNING and then CANCELED, but not SUCCESS
    hasCanceled := false
    hasSuccess := false
    for _, st := range states {
        if st == nodev1.NodeState_NODE_STATE_CANCELED { hasCanceled = true }
        if st == nodev1.NodeState_NODE_STATE_SUCCESS { hasSuccess = true }
    }
    assert.True(t, hasCanceled, "FOR node should stream CANCELED state on throw")
    assert.False(t, hasSuccess, "FOR node should not stream SUCCESS when throw cancels the loop")
}
