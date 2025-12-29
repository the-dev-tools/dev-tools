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
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// setupTestServiceWithStreams creates a test service with nodeStream configured
// so we can verify events are published.
func setupTestServiceWithStreams(t *testing.T) (*FlowServiceV2RPC, context.Context, idwrap.IDWrap, idwrap.IDWrap, <-chan NodeEvent) {
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

	// Create in-memory node stream
	nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()

	svc := &FlowServiceV2RPC{
		DB:         db,
		wsReader:   wsReader,
		fsReader:   fsReader,
		nsReader:   nsReader,
		ws:         &wsService,
		fs:         &flowService,
		ns:         &nodeService,
		nes:        &nodeExecService,
		es:         &edgeService,
		fvs:        &flowVarService,
		nrs:        &nodeRequestService,
		nfs:        &nodeForService,
		nfes:       &nodeForEachService,
		nifs:       nodeIfService,
		njss:       &nodeNodeJsService,
		logger:     logger,
		nodeStream: nodeStream,
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

	// Subscribe to node events
	eventChan, err := nodeStream.Subscribe(ctx, func(topic NodeTopic) bool {
		return true // Accept all events
	})
	require.NoError(t, err)

	// Convert to channel of NodeEvent
	nodeEventChan := make(chan NodeEvent, 100)
	go func() {
		for evt := range eventChan {
			nodeEventChan <- evt.Payload
		}
		close(nodeEventChan)
	}()

	return svc, ctx, userID, workspaceID, nodeEventChan
}

// collectEvents collects events from the channel with a timeout
func collectEvents(eventChan <-chan NodeEvent, count int, timeout time.Duration) []NodeEvent {
	events := make([]NodeEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok {
				return events
			}
			events = append(events, evt)
		case <-timer.C:
			return events
		}
	}
	return events
}

// TestNodeHttpSync_PublishesEventsOnCRUD verifies that NodeHttp CRUD operations
// publish events to the node stream.
func TestNodeHttpSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, eventChan := setupTestServiceWithStreams(t)

	// Create flow
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (REQUEST kind)
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "HTTP Node",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	httpID := idwrap.NewNow()

	t.Run("Insert publishes event", func(t *testing.T) {
		// Insert should publish event so sync clients re-fetch node with HTTP config
		req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
			Items: []*flowv1.NodeHttpInsert{{
				NodeId: nodeID.Bytes(),
				HttpId: httpID.Bytes(),
			}},
		})

		_, err := svc.NodeHttpInsert(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Insert")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		newHttpID := idwrap.NewNow()
		req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{{
				NodeId: nodeID.Bytes(),
				HttpId: newHttpID.Bytes(),
			}},
		})

		_, err := svc.NodeHttpUpdate(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Update")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
			Items: []*flowv1.NodeHttpDelete{{
				NodeId: nodeID.Bytes(),
			}},
		})

		_, err := svc.NodeHttpDelete(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Delete")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})
}

// TestNodeConditionSync_PublishesEventsOnCRUD verifies that NodeCondition operations
// publish events to the node stream.
func TestNodeConditionSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, eventChan := setupTestServiceWithStreams(t)

	// Create flow
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (CONDITION kind)
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "Condition Node",
		NodeKind: mflow.NODE_KIND_CONDITION,
	})
	require.NoError(t, err)

	t.Run("Insert publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
			Items: []*flowv1.NodeConditionInsert{{
				NodeId:    nodeID.Bytes(),
				Condition: "a > b",
			}},
		})

		_, err := svc.NodeConditionInsert(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Insert")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
			Items: []*flowv1.NodeConditionUpdate{{
				NodeId:    nodeID.Bytes(),
				Condition: ptr("x == y"),
			}},
		})

		_, err := svc.NodeConditionUpdate(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Update")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
			Items: []*flowv1.NodeConditionDelete{{
				NodeId: nodeID.Bytes(),
			}},
		})

		_, err := svc.NodeConditionDelete(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Delete")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})
}

// TestNodeForEachSync_PublishesEventsOnCRUD verifies that NodeForEach operations
// publish events to the node stream.
func TestNodeForEachSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, eventChan := setupTestServiceWithStreams(t)

	// Create flow
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (FOR_EACH kind)
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "ForEach Node",
		NodeKind: mflow.NODE_KIND_FOR_EACH,
	})
	require.NoError(t, err)

	t.Run("Insert publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
			Items: []*flowv1.NodeForEachInsert{{
				NodeId: nodeID.Bytes(),
				Path:   "items",
			}},
		})

		_, err := svc.NodeForEachInsert(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Insert")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
			Items: []*flowv1.NodeForEachUpdate{{
				NodeId: nodeID.Bytes(),
				Path:   ptr("data.items"),
			}},
		})

		_, err := svc.NodeForEachUpdate(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Update")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeForEachDeleteRequest{
			Items: []*flowv1.NodeForEachDelete{{
				NodeId: nodeID.Bytes(),
			}},
		})

		_, err := svc.NodeForEachDelete(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Delete")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})
}

// TestNodeJsSync_PublishesEventsOnCRUD verifies that NodeJs operations
// publish events to the node stream.
func TestNodeJsSync_PublishesEventsOnCRUD(t *testing.T) {
	svc, ctx, _, workspaceID, eventChan := setupTestServiceWithStreams(t)

	// Create flow
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (JS kind)
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "JS Node",
		NodeKind: mflow.NODE_KIND_JS,
	})
	require.NoError(t, err)

	t.Run("Insert publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
			Items: []*flowv1.NodeJsInsert{{
				NodeId: nodeID.Bytes(),
				Code:   "return 42;",
			}},
		})

		_, err := svc.NodeJsInsert(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Insert")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Update publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeJsUpdateRequest{
			Items: []*flowv1.NodeJsUpdate{{
				NodeId: nodeID.Bytes(),
				Code:   ptr("return 'hello';"),
			}},
		})

		_, err := svc.NodeJsUpdate(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Update")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})

	t.Run("Delete publishes event", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.NodeJsDeleteRequest{
			Items: []*flowv1.NodeJsDelete{{
				NodeId: nodeID.Bytes(),
			}},
		})

		_, err := svc.NodeJsDelete(ctx, req)
		require.NoError(t, err)

		events := collectEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, events, 1, "Should receive 1 event after Delete")
		assert.Equal(t, nodeEventUpdate, events[0].Type)
	})
}
