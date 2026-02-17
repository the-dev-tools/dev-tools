package rflowv2

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// permIsolationFixture provides a two-user, two-workspace environment
// for testing that Collection and Sync endpoints properly isolate data.
type permIsolationFixture struct {
	svc *FlowServiceV2RPC

	// User A: member of workspaceA
	ctxA        context.Context
	userAID     idwrap.IDWrap
	workspaceA  idwrap.IDWrap
	flowA       idwrap.IDWrap
	startNodeA  idwrap.IDWrap
	edgeA       idwrap.IDWrap

	// User B: member of workspaceB, NOT a member of workspaceA
	ctxB        context.Context
	userBID     idwrap.IDWrap
	workspaceB  idwrap.IDWrap
	flowB       idwrap.IDWrap
	startNodeB  idwrap.IDWrap
	edgeB       idwrap.IDWrap
}

func newPermIsolationFixture(t *testing.T) *permIsolationFixture {
	t.Helper()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	queries := gen.New(db)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	nodeRequestService := sflow.NewNodeRequestService(queries)
	nodeForService := sflow.NewNodeForService(queries)
	nodeForEachService := sflow.NewNodeForEachService(queries)
	nodeIfService := sflow.NewNodeIfService(queries)
	nodeNodeJsService := sflow.NewNodeJsService(queries)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	userReader := sworkspace.NewUserReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	// Streams
	flowStream := memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent]()
	nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()
	edgeStream := memory.NewInMemorySyncStreamer[EdgeTopic, EdgeEvent]()

	// Mock resolver
	res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

	svc := &FlowServiceV2RPC{
		DB:             db,
		wsReader:       wsReader,
		wsUserReader:   userReader,
		fsReader:       fsReader,
		nsReader:       nsReader,
		flowEdgeReader: edgeService.Reader(),
		ws:             &wsService,
		fs:             &flowService,
		ns:             &nodeService,
		nes:            &nodeExecService,
		es:             &edgeService,
		fvs:            &flowVarService,
		nrs:            &nodeRequestService,
		nfs:            &nodeForService,
		nfes:           &nodeForEachService,
		nifs:           nodeIfService,
		njss:           &nodeNodeJsService,
		resolver:       res,
		logger:         logger,
		flowStream:     flowStream,
		nodeStream:     nodeStream,
		edgeStream:     edgeStream,
	}

	// --- User A ---
	userAID := idwrap.NewNow()
	err = queries.CreateUser(ctx, gen.CreateUserParams{ID: userAID, Email: "a@example.com"})
	require.NoError(t, err)
	ctxA := mwauth.CreateAuthedContext(ctx, userAID)

	workspaceA := idwrap.NewNow()
	err = wsService.Create(ctx, &mworkspace.Workspace{ID: workspaceA, Name: "Workspace A", Updated: dbtime.DBNow()})
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID: idwrap.NewNow(), WorkspaceID: workspaceA, UserID: userAID, Role: 3,
	})
	require.NoError(t, err)

	flowA := idwrap.NewNow()
	err = flowService.CreateFlow(ctx, mflow.Flow{ID: flowA, WorkspaceID: workspaceA, Name: "Flow A"})
	require.NoError(t, err)

	startNodeA := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: startNodeA, FlowID: flowA, Name: "Start A", NodeKind: mflow.NODE_KIND_MANUAL_START})
	require.NoError(t, err)

	requestNodeA := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: requestNodeA, FlowID: flowA, Name: "Node A", NodeKind: mflow.NODE_KIND_REQUEST, PositionX: 100})
	require.NoError(t, err)

	edgeA := idwrap.NewNow()
	err = edgeService.CreateEdge(ctx, mflow.Edge{ID: edgeA, FlowID: flowA, SourceID: startNodeA, TargetID: requestNodeA})
	require.NoError(t, err)

	// --- User B ---
	userBID := idwrap.NewNow()
	err = queries.CreateUser(ctx, gen.CreateUserParams{ID: userBID, Email: "b@example.com"})
	require.NoError(t, err)
	ctxB := mwauth.CreateAuthedContext(ctx, userBID)

	workspaceB := idwrap.NewNow()
	err = wsService.Create(ctx, &mworkspace.Workspace{ID: workspaceB, Name: "Workspace B", Updated: dbtime.DBNow()})
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID: idwrap.NewNow(), WorkspaceID: workspaceB, UserID: userBID, Role: 3,
	})
	require.NoError(t, err)

	flowB := idwrap.NewNow()
	err = flowService.CreateFlow(ctx, mflow.Flow{ID: flowB, WorkspaceID: workspaceB, Name: "Flow B"})
	require.NoError(t, err)

	startNodeB := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: startNodeB, FlowID: flowB, Name: "Start B", NodeKind: mflow.NODE_KIND_MANUAL_START})
	require.NoError(t, err)

	requestNodeB := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: requestNodeB, FlowID: flowB, Name: "Node B", NodeKind: mflow.NODE_KIND_REQUEST, PositionX: 100})
	require.NoError(t, err)

	edgeB := idwrap.NewNow()
	err = edgeService.CreateEdge(ctx, mflow.Edge{ID: edgeB, FlowID: flowB, SourceID: startNodeB, TargetID: requestNodeB})
	require.NoError(t, err)

	return &permIsolationFixture{
		svc:        svc,
		ctxA:       ctxA,
		userAID:    userAID,
		workspaceA: workspaceA,
		flowA:      flowA,
		startNodeA: startNodeA,
		edgeA:      edgeA,
		ctxB:       ctxB,
		userBID:    userBID,
		workspaceB: workspaceB,
		flowB:      flowB,
		startNodeB: startNodeB,
		edgeB:      edgeB,
	}
}

// --- Collection Isolation Tests ---

func TestFlowCollection_OnlyReturnsUserWorkspaceFlows(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User A should only see Flow A
	respA, err := f.svc.FlowCollection(f.ctxA, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	flowIDs := make([]idwrap.IDWrap, 0, len(respA.Msg.Items))
	for _, item := range respA.Msg.Items {
		id, err := idwrap.NewFromBytes(item.FlowId)
		require.NoError(t, err)
		flowIDs = append(flowIDs, id)
	}
	assert.Contains(t, flowIDs, f.flowA, "User A should see Flow A")
	assert.NotContains(t, flowIDs, f.flowB, "User A should NOT see Flow B")

	// User B should only see Flow B
	respB, err := f.svc.FlowCollection(f.ctxB, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	flowIDs = flowIDs[:0]
	for _, item := range respB.Msg.Items {
		id, err := idwrap.NewFromBytes(item.FlowId)
		require.NoError(t, err)
		flowIDs = append(flowIDs, id)
	}
	assert.Contains(t, flowIDs, f.flowB, "User B should see Flow B")
	assert.NotContains(t, flowIDs, f.flowA, "User B should NOT see Flow A")
}

func TestNodeCollection_OnlyReturnsUserWorkspaceNodes(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User A should only see nodes from Flow A
	respA, err := f.svc.NodeCollection(f.ctxA, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	nodeFlowIDs := make(map[idwrap.IDWrap]bool)
	for _, item := range respA.Msg.Items {
		flowID, err := idwrap.NewFromBytes(item.FlowId)
		require.NoError(t, err)
		nodeFlowIDs[flowID] = true
	}
	assert.True(t, nodeFlowIDs[f.flowA], "User A should see nodes from Flow A")
	assert.False(t, nodeFlowIDs[f.flowB], "User A should NOT see nodes from Flow B")

	// User B should only see nodes from Flow B
	respB, err := f.svc.NodeCollection(f.ctxB, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	nodeFlowIDs = make(map[idwrap.IDWrap]bool)
	for _, item := range respB.Msg.Items {
		flowID, err := idwrap.NewFromBytes(item.FlowId)
		require.NoError(t, err)
		nodeFlowIDs[flowID] = true
	}
	assert.True(t, nodeFlowIDs[f.flowB], "User B should see nodes from Flow B")
	assert.False(t, nodeFlowIDs[f.flowA], "User B should NOT see nodes from Flow A")
}

func TestEdgeCollection_OnlyReturnsUserWorkspaceEdges(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User A should only see edges from Flow A
	respA, err := f.svc.EdgeCollection(f.ctxA, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	edgeIDs := make([]idwrap.IDWrap, 0, len(respA.Msg.Items))
	for _, item := range respA.Msg.Items {
		id, err := idwrap.NewFromBytes(item.EdgeId)
		require.NoError(t, err)
		edgeIDs = append(edgeIDs, id)
	}
	assert.Contains(t, edgeIDs, f.edgeA, "User A should see Edge A")
	assert.NotContains(t, edgeIDs, f.edgeB, "User A should NOT see Edge B")

	// User B should only see edges from Flow B
	respB, err := f.svc.EdgeCollection(f.ctxB, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	edgeIDs = edgeIDs[:0]
	for _, item := range respB.Msg.Items {
		id, err := idwrap.NewFromBytes(item.EdgeId)
		require.NoError(t, err)
		edgeIDs = append(edgeIDs, id)
	}
	assert.Contains(t, edgeIDs, f.edgeB, "User B should see Edge B")
	assert.NotContains(t, edgeIDs, f.edgeA, "User B should NOT see Edge A")
}

// --- Sync Stream Filtering Tests ---

func TestFlowSyncFiltersUnauthorizedWorkspace(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	ctx, cancel := context.WithCancel(f.ctxA)
	defer cancel()

	msgCh := make(chan *flowv1.FlowSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.svc.streamFlowSync(ctx, func(resp *flowv1.FlowSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
	}()

	// Give stream time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Publish event for workspace B (User A is NOT a member)
	f.svc.flowStream.Publish(FlowTopic{WorkspaceID: f.workspaceB}, FlowEvent{
		Type: flowEventInsert,
		Flow: &flowv1.Flow{
			FlowId:      idwrap.NewNow().Bytes(),
			WorkspaceId: f.workspaceB.Bytes(),
			Name:        "hidden-flow",
		},
	})

	select {
	case resp := <-msgCh:
		require.FailNow(t, "unexpected event for unauthorized workspace", "%+v", resp)
	case <-time.After(150 * time.Millisecond):
		// success: no events delivered for unauthorized workspace
	}

	// Publish event for workspace A (User A IS a member) — should be delivered
	f.svc.flowStream.Publish(FlowTopic{WorkspaceID: f.workspaceA}, FlowEvent{
		Type: flowEventInsert,
		Flow: &flowv1.Flow{
			FlowId:      idwrap.NewNow().Bytes(),
			WorkspaceId: f.workspaceA.Bytes(),
			Name:        "visible-flow",
		},
	})

	select {
	case resp := <-msgCh:
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)
		assert.Equal(t, "visible-flow", resp.Items[0].GetValue().GetInsert().GetName())
	case <-time.After(500 * time.Millisecond):
		require.FailNow(t, "expected event for authorized workspace but got none")
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestNodeSyncFiltersUnauthorizedFlow(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	ctx, cancel := context.WithCancel(f.ctxA)
	defer cancel()

	msgCh := make(chan *flowv1.NodeSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.svc.streamNodeSync(ctx, func(resp *flowv1.NodeSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish node event for flow B (User A has no access)
	f.svc.nodeStream.Publish(NodeTopic{FlowID: f.flowB}, NodeEvent{
		Type:   nodeEventInsert,
		FlowID: f.flowB,
		Node: &flowv1.Node{
			NodeId: idwrap.NewNow().Bytes(),
			FlowId: f.flowB.Bytes(),
			Name:   "hidden-node",
			Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
		},
	})

	select {
	case resp := <-msgCh:
		require.FailNow(t, "unexpected node event for unauthorized flow", "%+v", resp)
	case <-time.After(150 * time.Millisecond):
		// success
	}

	// Publish node event for flow A (User A has access) — should be delivered
	f.svc.nodeStream.Publish(NodeTopic{FlowID: f.flowA}, NodeEvent{
		Type:   nodeEventInsert,
		FlowID: f.flowA,
		Node: &flowv1.Node{
			NodeId: idwrap.NewNow().Bytes(),
			FlowId: f.flowA.Bytes(),
			Name:   "visible-node",
			Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
		},
	})

	select {
	case resp := <-msgCh:
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)
		assert.Equal(t, "visible-node", resp.Items[0].GetValue().GetInsert().GetName())
	case <-time.After(500 * time.Millisecond):
		require.FailNow(t, "expected node event for authorized flow but got none")
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestEdgeSyncFiltersUnauthorizedFlow(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	ctx, cancel := context.WithCancel(f.ctxA)
	defer cancel()

	msgCh := make(chan *flowv1.EdgeSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.svc.streamEdgeSync(ctx, func(resp *flowv1.EdgeSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish edge event for flow B (User A has no access)
	f.svc.edgeStream.Publish(EdgeTopic{FlowID: f.flowB}, EdgeEvent{
		Type:   edgeEventInsert,
		FlowID: f.flowB,
		Edge: &flowv1.Edge{
			EdgeId: idwrap.NewNow().Bytes(),
			FlowId: f.flowB.Bytes(),
		},
	})

	select {
	case resp := <-msgCh:
		require.FailNow(t, "unexpected edge event for unauthorized flow", "%+v", resp)
	case <-time.After(150 * time.Millisecond):
		// success
	}

	// Publish edge event for flow A (User A has access) — should be delivered
	visibleEdgeID := idwrap.NewNow()
	f.svc.edgeStream.Publish(EdgeTopic{FlowID: f.flowA}, EdgeEvent{
		Type:   edgeEventInsert,
		FlowID: f.flowA,
		Edge: &flowv1.Edge{
			EdgeId:   visibleEdgeID.Bytes(),
			FlowId:   f.flowA.Bytes(),
			SourceId: f.startNodeA.Bytes(),
			TargetId: idwrap.NewNow().Bytes(),
		},
	})

	select {
	case resp := <-msgCh:
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)
		gotEdgeID, err := idwrap.NewFromBytes(resp.Items[0].GetValue().GetInsert().GetEdgeId())
		require.NoError(t, err)
		assert.Equal(t, visibleEdgeID, gotEdgeID)
	case <-time.After(500 * time.Millisecond):
		require.FailNow(t, "expected edge event for authorized flow but got none")
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

// --- Mutation Access Denied Tests ---

func TestFlowInsert_DeniedForNonMember(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User B tries to insert a flow into workspace A
	req := connect.NewRequest(&flowv1.FlowInsertRequest{
		Items: []*flowv1.FlowInsert{{
			FlowId:      idwrap.NewNow().Bytes(),
			WorkspaceId: f.workspaceA.Bytes(),
			Name:        "Unauthorized Flow",
		}},
	})
	_, err := f.svc.FlowInsert(f.ctxB, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "non-member should get NotFound to prevent enumeration")
}

func TestFlowUpdate_DeniedForNonMember(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User B tries to update Flow A
	newName := "Hacked"
	req := connect.NewRequest(&flowv1.FlowUpdateRequest{
		Items: []*flowv1.FlowUpdate{{
			FlowId: f.flowA.Bytes(),
			Name:   &newName,
		}},
	})
	_, err := f.svc.FlowUpdate(f.ctxB, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestFlowDelete_DeniedForNonMember(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User B tries to delete Flow A
	req := connect.NewRequest(&flowv1.FlowDeleteRequest{
		Items: []*flowv1.FlowDelete{{
			FlowId: f.flowA.Bytes(),
		}},
	})
	_, err := f.svc.FlowDelete(f.ctxB, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestNodeInsert_DeniedForNonMember(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User B tries to insert a node into Flow A
	req := connect.NewRequest(&flowv1.NodeInsertRequest{
		Items: []*flowv1.NodeInsert{{
			NodeId: idwrap.NewNow().Bytes(),
			FlowId: f.flowA.Bytes(),
			Name:   "Unauthorized Node",
			Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
		}},
	})
	_, err := f.svc.NodeInsert(f.ctxB, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestNodeDelete_DeniedForNonMember(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User B tries to delete a node from Flow A
	req := connect.NewRequest(&flowv1.NodeDeleteRequest{
		Items: []*flowv1.NodeDelete{{
			NodeId: f.startNodeA.Bytes(),
		}},
	})
	_, err := f.svc.NodeDelete(f.ctxB, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestEdgeDelete_DeniedForNonMember(t *testing.T) {
	t.Parallel()
	f := newPermIsolationFixture(t)

	// User B tries to delete an edge from Flow A
	req := connect.NewRequest(&flowv1.EdgeDeleteRequest{
		Items: []*flowv1.EdgeDelete{{
			EdgeId: f.edgeA.Bytes(),
		}},
	})
	_, err := f.svc.EdgeDelete(f.ctxB, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
