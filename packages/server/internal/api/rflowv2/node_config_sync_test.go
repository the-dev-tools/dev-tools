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

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// setupTestServiceWithStreams creates a test service with all specialized streams configured
func setupTestServiceWithStreams(t *testing.T) (*FlowServiceV2RPC, context.Context, idwrap.IDWrap, idwrap.IDWrap, *testStreamRegistry) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	queries := gen.New(db)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup Services
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
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	// Create streams
	nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()
	forStream := memory.NewInMemorySyncStreamer[ForTopic, ForEvent]()
	conditionStream := memory.NewInMemorySyncStreamer[ConditionTopic, ConditionEvent]()
	forEachStream := memory.NewInMemorySyncStreamer[ForEachTopic, ForEachEvent]()
	jsStream := memory.NewInMemorySyncStreamer[JsTopic, JsEvent]()

	svc := &FlowServiceV2RPC{
		DB:              db,
		wsReader:        wsReader,
		fsReader:        fsReader,
		nsReader:        nsReader,
		ws:              &wsService,
		fs:              &flowService,
		ns:              &nodeService,
		nes:             &nodeExecService,
		es:              &edgeService,
		fvs:             &flowVarService,
		nrs:             &nodeRequestService,
		nfs:             &nodeForService,
		nfes:            &nodeForEachService,
		nifs:            nodeIfService,
		njss:            &nodeNodeJsService,
		logger:          logger,
		nodeStream:      nodeStream,
		forStream:       forStream,
		conditionStream: conditionStream,
		forEachStream:   forEachStream,
		jsStream:        jsStream,
	}

	// Setup user and workspace
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	err = wsService.Create(ctx, &mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: dbtime.DBNow(),
	})
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	registry := &testStreamRegistry{
		nodeEvents:      subscribeToNodeEvents(ctx, nodeStream),
		forEvents:       subscribeToForEvents(ctx, forStream),
		conditionEvents: subscribeToConditionEvents(ctx, conditionStream),
		forEachEvents:   subscribeToForEachEvents(ctx, forEachStream),
		jsEvents:        subscribeToJsEvents(ctx, jsStream),
	}

	return svc, ctx, userID, workspaceID, registry
}

type testStreamRegistry struct {
	nodeEvents      <-chan NodeEvent
	forEvents       <-chan ForEvent
	conditionEvents <-chan ConditionEvent
	forEachEvents   <-chan ForEachEvent
	jsEvents        <-chan JsEvent
}

func subscribeToNodeEvents(ctx context.Context, stream eventstream.SyncStreamer[NodeTopic, NodeEvent]) <-chan NodeEvent {
	ch, _ := stream.Subscribe(ctx, nil)
	out := make(chan NodeEvent, 100)
	go func() {
		for e := range ch {
			out <- e.Payload
		}
		close(out)
	}()
	return out
}

func subscribeToForEvents(ctx context.Context, stream eventstream.SyncStreamer[ForTopic, ForEvent]) <-chan ForEvent {
	ch, _ := stream.Subscribe(ctx, nil)
	out := make(chan ForEvent, 100)
	go func() {
		for e := range ch {
			out <- e.Payload
		}
		close(out)
	}()
	return out
}

func subscribeToConditionEvents(ctx context.Context, stream eventstream.SyncStreamer[ConditionTopic, ConditionEvent]) <-chan ConditionEvent {
	ch, _ := stream.Subscribe(ctx, nil)
	out := make(chan ConditionEvent, 100)
	go func() {
		for e := range ch {
			out <- e.Payload
		}
		close(out)
	}()
	return out
}

func subscribeToForEachEvents(ctx context.Context, stream eventstream.SyncStreamer[ForEachTopic, ForEachEvent]) <-chan ForEachEvent {
	ch, _ := stream.Subscribe(ctx, nil)
	out := make(chan ForEachEvent, 100)
	go func() {
		for e := range ch {
			out <- e.Payload
		}
		close(out)
	}()
	return out
}

func subscribeToJsEvents(ctx context.Context, stream eventstream.SyncStreamer[JsTopic, JsEvent]) <-chan JsEvent {
	ch, _ := stream.Subscribe(ctx, nil)
	out := make(chan JsEvent, 100)
	go func() {
		for e := range ch {
			out <- e.Payload
		}
		close(out)
	}()
	return out
}

func collectNodeEvents(eventChan <-chan NodeEvent, count int, timeout time.Duration) []NodeEvent {
	events := make([]NodeEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok { return events }
			events = append(events, evt)
		case <-timer.C: return events
		}
	}
	return events
}

func collectConditionEvents(eventChan <-chan ConditionEvent, count int, timeout time.Duration) []ConditionEvent {
	events := make([]ConditionEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok { return events }
			events = append(events, evt)
		case <-timer.C: return events
		}
	}
	return events
}

func collectForEachEvents(eventChan <-chan ForEachEvent, count int, timeout time.Duration) []ForEachEvent {
	events := make([]ForEachEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok { return events }
			events = append(events, evt)
		case <-timer.C: return events
		}
	}
	return events
}

func collectJsEvents(eventChan <-chan JsEvent, count int, timeout time.Duration) []JsEvent {
	events := make([]JsEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok { return events }
			events = append(events, evt)
		case <-timer.C: return events
		}
	}
	return events
}

// TestNodeHttpSync_PublishesEventsOnCRUD verifies that NodeHttp CRUD operations
// publish events to the node stream.
func TestNodeHttpSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, registry := setupTestServiceWithStreams(t)

	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: workspaceID, Name: "Test Flow"})
	require.NoError(t, err)

	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{ID: nodeID, FlowID: flowID, Name: "HTTP Node", NodeKind: mflow.NODE_KIND_REQUEST})
	require.NoError(t, err)

	t.Run("Insert publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
			Items: []*flowv1.NodeHttpInsert{{NodeId: nodeID.Bytes(), HttpId: idwrap.NewNow().Bytes()}},
		})
		_, err := svc.NodeHttpInsert(ctx, req)
		require.NoError(t, err)

		events := collectNodeEvents(registry.nodeEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{{NodeId: nodeID.Bytes(), HttpId: idwrap.NewNow().Bytes()}},
		})
		_, err := svc.NodeHttpUpdate(ctx, req)
		require.NoError(t, err)

		events := collectNodeEvents(registry.nodeEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
			Items: []*flowv1.NodeHttpDelete{{NodeId: nodeID.Bytes()}},
		})
		_, err := svc.NodeHttpDelete(ctx, req)
		require.NoError(t, err)

		events := collectNodeEvents(registry.nodeEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})
}

// TestNodeConditionSync_PublishesEventsOnCRUD verifies specialized sync events.
func TestNodeConditionSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, registry := setupTestServiceWithStreams(t)

	flowID := idwrap.NewNow()
	svc.fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: workspaceID, Name: "Test Flow"})

	nodeID := idwrap.NewNow()
	svc.ns.CreateNode(ctx, mflow.Node{ID: nodeID, FlowID: flowID, Name: "Cond Node", NodeKind: mflow.NODE_KIND_CONDITION})

	t.Run("Insert publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
			Items: []*flowv1.NodeConditionInsert{{NodeId: nodeID.Bytes(), Condition: "true"}},
		})
		svc.NodeConditionInsert(ctx, req)

		events := collectConditionEvents(registry.conditionEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, "insert", events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
			Items: []*flowv1.NodeConditionUpdate{{NodeId: nodeID.Bytes(), Condition: ptr("false")}},
		})
		svc.NodeConditionUpdate(ctx, req)

		events := collectConditionEvents(registry.conditionEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, "update", events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
			Items: []*flowv1.NodeConditionDelete{{NodeId: nodeID.Bytes()}},
		})
		svc.NodeConditionDelete(ctx, req)

		events := collectConditionEvents(registry.conditionEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, "delete", events[0].Type)
	})
}

// TestNodeForEachSync_PublishesEventsOnCRUD verifies specialized sync events.
func TestNodeForEachSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, registry := setupTestServiceWithStreams(t)

	flowID := idwrap.NewNow()
	svc.fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: workspaceID, Name: "Test Flow"})

	nodeID := idwrap.NewNow()
	svc.ns.CreateNode(ctx, mflow.Node{ID: nodeID, FlowID: flowID, Name: "ForEach Node", NodeKind: mflow.NODE_KIND_FOR_EACH})

	t.Run("Insert publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
			Items: []*flowv1.NodeForEachInsert{{NodeId: nodeID.Bytes(), Path: "items"}},
		})
		svc.NodeForEachInsert(ctx, req)

		events := collectForEachEvents(registry.forEachEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, "insert", events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
			Items: []*flowv1.NodeForEachUpdate{{NodeId: nodeID.Bytes(), Path: ptr("new.items")}},
		})
		svc.NodeForEachUpdate(ctx, req)

		events := collectForEachEvents(registry.forEachEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, "update", events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeForEachDeleteRequest{
			Items: []*flowv1.NodeForEachDelete{{NodeId: nodeID.Bytes()}},
		})
		svc.NodeForEachDelete(ctx, req)

		events := collectForEachEvents(registry.forEachEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, "delete", events[0].Type)
	})
}

// TestNodeJsSync_PublishesEventsOnCRUD verifies specialized sync events.
func TestNodeJsSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, registry := setupTestServiceWithStreams(t)

	flowID := idwrap.NewNow()
	svc.fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: workspaceID, Name: "Test Flow"})

	nodeID := idwrap.NewNow()
	svc.ns.CreateNode(ctx, mflow.Node{ID: nodeID, FlowID: flowID, Name: "JS Node", NodeKind: mflow.NODE_KIND_JS})

	t.Run("Insert publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
			Items: []*flowv1.NodeJsInsert{{NodeId: nodeID.Bytes(), Code: "console.log(1)"}},
		})
		svc.NodeJsInsert(ctx, req)

		events := collectJsEvents(registry.jsEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, jsEventInsert, events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeJsUpdateRequest{
			Items: []*flowv1.NodeJsUpdate{{NodeId: nodeID.Bytes(), Code: ptr("console.log(2)")}},
		})
		svc.NodeJsUpdate(ctx, req)

		events := collectJsEvents(registry.jsEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, jsEventUpdate, events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeJsDeleteRequest{
			Items: []*flowv1.NodeJsDelete{{NodeId: nodeID.Bytes()}},
		})
		svc.NodeJsDelete(ctx, req)

		events := collectJsEvents(registry.jsEvents, 1, 100*time.Millisecond)
		require.Len(t, events, 1)
		assert.Equal(t, jsEventDelete, events[0].Type)
	})
}
