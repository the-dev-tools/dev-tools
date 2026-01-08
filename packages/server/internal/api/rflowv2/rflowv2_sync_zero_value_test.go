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

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// =============================================================================
// Common Test Infrastructure
// =============================================================================

// testServices holds all the service dependencies for testing
type testServices struct {
	svc         *FlowServiceV2RPC
	ctx         context.Context
	workspaceID idwrap.IDWrap
	flowID      idwrap.IDWrap
	nodeStream  eventstream.SyncStreamer[NodeTopic, NodeEvent]
	forStream   eventstream.SyncStreamer[ForTopic, ForEvent]
}

// setupZeroValueTestServices creates all test infrastructure
func setupZeroValueTestServices(t *testing.T) *testServices {
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

	// Create in-memory streams
	nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()
	forStream := memory.NewInMemorySyncStreamer[ForTopic, ForEvent]()

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
		conditionStream: memory.NewInMemorySyncStreamer[ConditionTopic, ConditionEvent](),
		forEachStream:   memory.NewInMemorySyncStreamer[ForEachTopic, ForEachEvent](),
		jsStream:        memory.NewInMemorySyncStreamer[JsTopic, JsEvent](),
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

	// Create flow
	flowID := idwrap.NewNow()
	err = svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	return &testServices{
		svc:         svc,
		ctx:         ctx,
		workspaceID: workspaceID,
		flowID:      flowID,
		nodeStream:  nodeStream,
		forStream:   forStream,
	}
}

// subscribeForEvents subscribes to the for stream and returns a channel of ForEvent
func (ts *testServices) subscribeForEvents(t *testing.T) <-chan ForEvent {
	eventChan, err := ts.forStream.Subscribe(ts.ctx, func(topic ForTopic) bool {
		return true
	})
	require.NoError(t, err)

	forEventChan := make(chan ForEvent, 100)
	go func() {
		for evt := range eventChan {
			forEventChan <- evt.Payload
		}
		close(forEventChan)
	}()

	return forEventChan
}

// subscribeNodeEvents subscribes to the node stream and returns a channel of NodeEvent
func (ts *testServices) subscribeNodeEvents(t *testing.T) <-chan NodeEvent {
	eventChan, err := ts.nodeStream.Subscribe(ts.ctx, func(topic NodeTopic) bool {
		return true
	})
	require.NoError(t, err)

	nodeEventChan := make(chan NodeEvent, 100)
	go func() {
		for evt := range eventChan {
			nodeEventChan <- evt.Payload
		}
		close(nodeEventChan)
	}()

	return nodeEventChan
}

// waitForEvent waits for an event with a timeout
func waitForEvent[T any](t *testing.T, ch <-chan T, timeout time.Duration) (T, bool) {
	select {
	case evt := <-ch:
		return evt, true
	case <-time.After(timeout):
		var zero T
		return zero, false
	}
}

// =============================================================================
// NodeFor Zero-Value Tests (iterations int32)
// =============================================================================

// TestNodeFor_ZeroIterations tests that setting iterations to 0 is correctly synced.
// This is a regression test for the bug where `if iterations != 0` incorrectly
// excluded zero values from sync updates.
func TestNodeFor_ZeroIterations(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create For node
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   ts.flowID,
		Name:     "For Node",
		NodeKind: mflow.NODE_KIND_FOR,
	})
	require.NoError(t, err)

	// Insert For node config with non-zero iterations
	insertReq := connect.NewRequest(&flowv1.NodeForInsertRequest{
		Items: []*flowv1.NodeForInsert{{
			NodeId:     nodeID.Bytes(),
			Iterations: 5,
			Condition:  "true",
		}},
	})
	_, err = ts.svc.NodeForInsert(ts.ctx, insertReq)
	require.NoError(t, err)

	// Subscribe to events after insert
	eventChan := ts.subscribeForEvents(t)
	time.Sleep(50 * time.Millisecond)

	t.Run("update iterations to zero", func(t *testing.T) {
		zeroIterations := int32(0)
		updateReq := connect.NewRequest(&flowv1.NodeForUpdateRequest{
			Items: []*flowv1.NodeForUpdate{{
				NodeId:     nodeID.Bytes(),
				Iterations: &zeroIterations,
			}},
		})

		_, err := ts.svc.NodeForUpdate(ts.ctx, updateReq)
		require.NoError(t, err)

		// Wait for sync event
		evt, ok := waitForEvent(t, eventChan, 2*time.Second)
		require.True(t, ok, "Should receive sync event for zero iterations")

		// Convert to sync response
		resp := forEventToSyncResponse(evt)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.Update
		require.NotNil(t, update, "Update should not be nil")
		require.NotNil(t, update.Iterations, "Iterations MUST be set even when zero")
		assert.Equal(t, int32(0), *update.Iterations)
	})

	t.Run("verify persisted value is zero", func(t *testing.T) {
		nodeFor, err := ts.svc.nfs.GetNodeFor(ts.ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), nodeFor.IterCount)
	})
}

// =============================================================================
// NodeCondition Empty String Tests
// =============================================================================

// TestNodeCondition_EmptyCondition tests that setting condition to empty string
// is correctly synced.
func TestNodeCondition_EmptyCondition(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create Condition node
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   ts.flowID,
		Name:     "Condition Node",
		NodeKind: mflow.NODE_KIND_CONDITION,
	})
	require.NoError(t, err)

	// Insert Condition node config with non-empty condition
	insertReq := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
		Items: []*flowv1.NodeConditionInsert{{
			NodeId:    nodeID.Bytes(),
			Condition: "x > 0",
		}},
	})
	_, err = ts.svc.NodeConditionInsert(ts.ctx, insertReq)
	require.NoError(t, err)

	// Subscribe to events after insert
	time.Sleep(50 * time.Millisecond)

	t.Run("update condition to empty string", func(t *testing.T) {
		emptyCondition := ""
		updateReq := connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
			Items: []*flowv1.NodeConditionUpdate{{
				NodeId:    nodeID.Bytes(),
				Condition: &emptyCondition,
			}},
		})

		// Subscribe to specialized stream
		ch, _ := ts.svc.conditionStream.Subscribe(ts.ctx, nil)

		_, err := ts.svc.NodeConditionUpdate(ts.ctx, updateReq)
		require.NoError(t, err)

		// Wait for sync event
		select {
		case evt := <-ch:
			assert.Equal(t, "update", evt.Payload.Type)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for sync event")
		}
	})

	t.Run("verify persisted value is empty", func(t *testing.T) {
		nodeCondition, err := ts.svc.nifs.GetNodeIf(ts.ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, "", nodeCondition.Condition.Comparisons.Expression)
	})
}

// =============================================================================
// NodeForEach Empty String Tests
// =============================================================================

// TestNodeForEach_EmptyPath tests that setting path to empty string is correctly synced.
func TestNodeForEach_EmptyPath(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create ForEach node
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   ts.flowID,
		Name:     "ForEach Node",
		NodeKind: mflow.NODE_KIND_FOR_EACH,
	})
	require.NoError(t, err)

	// Insert ForEach node config with non-empty path
	insertReq := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
		Items: []*flowv1.NodeForEachInsert{{
			NodeId:    nodeID.Bytes(),
			Path:      "items",
			Condition: "true",
		}},
	})
	_, err = ts.svc.NodeForEachInsert(ts.ctx, insertReq)
	require.NoError(t, err)

	// Subscribe to events after insert
	time.Sleep(50 * time.Millisecond)

	t.Run("update path to empty string", func(t *testing.T) {
		emptyPath := ""
		updateReq := connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
			Items: []*flowv1.NodeForEachUpdate{{
				NodeId: nodeID.Bytes(),
				Path:   &emptyPath,
			}},
		})

		// Subscribe to specialized stream
		ch, _ := ts.svc.forEachStream.Subscribe(ts.ctx, nil)

		_, err := ts.svc.NodeForEachUpdate(ts.ctx, updateReq)
		require.NoError(t, err)

		// Wait for sync event
		select {
		case evt := <-ch:
			assert.Equal(t, "update", evt.Payload.Type)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for sync event")
		}
	})

	t.Run("verify persisted value is empty", func(t *testing.T) {
		nodeForEach, err := ts.svc.nfes.GetNodeForEach(ts.ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, "", nodeForEach.IterExpression)
	})
}

// =============================================================================
// NodeJs Empty Code Tests
// =============================================================================

// TestNodeJs_EmptyCode tests that setting code to empty string is correctly synced.
func TestNodeJs_EmptyCode(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create JS node
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   ts.flowID,
		Name:     "JS Node",
		NodeKind: mflow.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Insert JS node config with non-empty code
	insertReq := connect.NewRequest(&flowv1.NodeJsInsertRequest{
		Items: []*flowv1.NodeJsInsert{{
			NodeId: nodeID.Bytes(),
			Code:   "return 42;",
		}},
	})
	_, err = ts.svc.NodeJsInsert(ts.ctx, insertReq)
	require.NoError(t, err)

	// Subscribe to events after insert
	time.Sleep(50 * time.Millisecond)

	t.Run("update code to empty string", func(t *testing.T) {
		emptyCode := ""
		updateReq := connect.NewRequest(&flowv1.NodeJsUpdateRequest{
			Items: []*flowv1.NodeJsUpdate{{
				NodeId: nodeID.Bytes(),
				Code:   &emptyCode,
			}},
		})

		// Subscribe to specialized stream
		ch, _ := ts.svc.jsStream.Subscribe(ts.ctx, nil)

		_, err := ts.svc.NodeJsUpdate(ts.ctx, updateReq)
		require.NoError(t, err)

		// Wait for sync event
		select {
		case evt := <-ch:
			assert.Equal(t, jsEventUpdate, evt.Payload.Type)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for sync event")
		}
	})

	t.Run("verify persisted value is empty", func(t *testing.T) {
		nodeJs, err := ts.svc.njss.GetNodeJS(ts.ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, "", string(nodeJs.Code))
	})
}

// =============================================================================
// Node Position Zero-Value Tests
// =============================================================================

// TestNode_ZeroPosition tests that setting position to (0,0) is correctly synced.
func TestNode_ZeroPosition(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create node with non-zero position
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:        nodeID,
		FlowID:    ts.flowID,
		Name:      "Test Node",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 100.0,
		PositionY: 200.0,
	})
	require.NoError(t, err)

	// Subscribe to events
	eventChan := ts.subscribeNodeEvents(t)
	time.Sleep(50 * time.Millisecond)

	t.Run("update position to zero", func(t *testing.T) {
		zeroPosition := &flowv1.Position{X: 0, Y: 0}
		updateReq := connect.NewRequest(&flowv1.NodeUpdateRequest{
			Items: []*flowv1.NodeUpdate{{
				NodeId:   nodeID.Bytes(),
				Position: zeroPosition,
			}},
		})

		_, err := ts.svc.NodeUpdate(ts.ctx, updateReq)
		require.NoError(t, err)

		// Wait for sync event
		evt, ok := waitForEvent(t, eventChan, 2*time.Second)
		require.True(t, ok, "Should receive sync event for zero position")
		assert.Equal(t, nodeEventUpdate, evt.Type)
	})

	t.Run("verify persisted position is zero", func(t *testing.T) {
		node, err := ts.svc.ns.GetNode(ts.ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, float64(0), node.PositionX)
		assert.Equal(t, float64(0), node.PositionY)
	})
}

// =============================================================================
// NodeHttp Sync Tests
// =============================================================================

// TestNodeHttp_UpdateHttpId tests that updating httpId triggers sync events.
func TestNodeHttp_UpdateHttpId(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create HTTP node (REQUEST kind)
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   ts.flowID,
		Name:     "HTTP Node",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Insert NodeHttp config with initial httpId
	initialHttpId := idwrap.NewNow()
	insertReq := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
		Items: []*flowv1.NodeHttpInsert{{
			NodeId: nodeID.Bytes(),
			HttpId: initialHttpId.Bytes(),
		}},
	})
	_, err = ts.svc.NodeHttpInsert(ts.ctx, insertReq)
	require.NoError(t, err)

	// Subscribe to events after insert
	eventChan := ts.subscribeNodeEvents(t)
	time.Sleep(50 * time.Millisecond)

	t.Run("update httpId to new value", func(t *testing.T) {
		newHttpId := idwrap.NewNow()
		updateReq := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{{
				NodeId: nodeID.Bytes(),
				HttpId: newHttpId.Bytes(),
			}},
		})

		_, err := ts.svc.NodeHttpUpdate(ts.ctx, updateReq)
		require.NoError(t, err)

		// Wait for sync event
		evt, ok := waitForEvent(t, eventChan, 2*time.Second)
		require.True(t, ok, "Should receive sync event for httpId update")
		assert.Equal(t, nodeEventUpdate, evt.Type)
	})

	t.Run("verify persisted httpId", func(t *testing.T) {
		nodeReq, err := ts.svc.nrs.GetNodeRequest(ts.ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, nodeReq.HttpID)
	})
}

// TestNodeHttp_ClearDeltaHttpId tests that clearing deltaHttpId triggers sync events.
func TestNodeHttp_ClearDeltaHttpId(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create HTTP node (REQUEST kind)
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   ts.flowID,
		Name:     "HTTP Node",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Insert NodeHttp config with initial httpId and deltaHttpId
	initialHttpId := idwrap.NewNow()
	initialDeltaHttpId := idwrap.NewNow()
	insertReq := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
		Items: []*flowv1.NodeHttpInsert{{
			NodeId:      nodeID.Bytes(),
			HttpId:      initialHttpId.Bytes(),
			DeltaHttpId: initialDeltaHttpId.Bytes(),
		}},
	})
	_, err = ts.svc.NodeHttpInsert(ts.ctx, insertReq)
	require.NoError(t, err)

	// Subscribe to events after insert
	eventChan := ts.subscribeNodeEvents(t)
	time.Sleep(50 * time.Millisecond)

	t.Run("update deltaHttpId to new value", func(t *testing.T) {
		newDeltaHttpId := idwrap.NewNow()
		updateReq := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{{
				NodeId: nodeID.Bytes(),
				HttpId: initialHttpId.Bytes(),
				DeltaHttpId: &flowv1.NodeHttpUpdate_DeltaHttpIdUnion{
					Kind:  flowv1.NodeHttpUpdate_DeltaHttpIdUnion_KIND_VALUE,
					Value: newDeltaHttpId.Bytes(),
				},
			}},
		})

		_, err := ts.svc.NodeHttpUpdate(ts.ctx, updateReq)
		require.NoError(t, err)

		// Wait for sync event
		evt, ok := waitForEvent(t, eventChan, 2*time.Second)
		require.True(t, ok, "Should receive sync event for deltaHttpId update")
		assert.Equal(t, nodeEventUpdate, evt.Type)
	})

	t.Run("verify persisted deltaHttpId", func(t *testing.T) {
		nodeReq, err := ts.svc.nrs.GetNodeRequest(ts.ctx, nodeID)
		require.NoError(t, err)
		require.NotNil(t, nodeReq.DeltaHttpID)
	})
}

// TestNodeHttp_DeleteConfig tests that deleting NodeHttp config triggers sync events.
func TestNodeHttp_DeleteConfig(t *testing.T) {
	ts := setupZeroValueTestServices(t)

	// Create HTTP node (REQUEST kind)
	nodeID := idwrap.NewNow()
	err := ts.svc.ns.CreateNode(ts.ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   ts.flowID,
		Name:     "HTTP Node",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Insert NodeHttp config
	initialHttpId := idwrap.NewNow()
	insertReq := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
		Items: []*flowv1.NodeHttpInsert{{
			NodeId: nodeID.Bytes(),
			HttpId: initialHttpId.Bytes(),
		}},
	})
	_, err = ts.svc.NodeHttpInsert(ts.ctx, insertReq)
	require.NoError(t, err)

	// Subscribe to events after insert
	eventChan := ts.subscribeNodeEvents(t)
	time.Sleep(50 * time.Millisecond)

	t.Run("delete NodeHttp config", func(t *testing.T) {
		deleteReq := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
			Items: []*flowv1.NodeHttpDelete{{
				NodeId: nodeID.Bytes(),
			}},
		})

		_, err := ts.svc.NodeHttpDelete(ts.ctx, deleteReq)
		require.NoError(t, err)

		// Wait for sync event
		evt, ok := waitForEvent(t, eventChan, 2*time.Second)
		require.True(t, ok, "Should receive sync event for NodeHttp delete")
		assert.Equal(t, nodeEventUpdate, evt.Type)
	})
}
