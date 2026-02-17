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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// setupTestServiceWithForStream creates a test service with forStream configured
// so we can verify For node events are published correctly.
func setupTestServiceWithForStream(t *testing.T) (*FlowServiceV2RPC, context.Context, idwrap.IDWrap, idwrap.IDWrap, <-chan ForEvent) {
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

	// Create in-memory for stream
	forStream := memory.NewInMemorySyncStreamer[ForTopic, ForEvent]()

	svc := &FlowServiceV2RPC{
		DB:           db,
		wsReader:     wsReader,
		wsUserReader: sworkspace.NewUserReaderFromQueries(queries),
		fsReader:     fsReader,
		nsReader: nsReader,
		ws:       &wsService,
		fs:       &flowService,
		ns:       &nodeService,
		nes:      &nodeExecService,
		es:       &edgeService,
		fvs:      &flowVarService,
		nrs:      &nodeRequestService,
		nfs:      &nodeForService,
		nfes:     &nodeForEachService,
		nifs:     nodeIfService,
		njss:     &nodeNodeJsService,
		logger:   logger,
		// Only set forStream, not nodeStream
		forStream: forStream,
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
		Role:        3,
	})
	require.NoError(t, err)

	// Subscribe to for events
	eventChan, err := forStream.Subscribe(ctx, func(topic ForTopic) bool {
		return true // Accept all events
	})
	require.NoError(t, err)

	// Convert to channel of ForEvent
	forEventChan := make(chan ForEvent, 100)
	go func() {
		for evt := range eventChan {
			forEventChan <- evt.Payload
		}
		close(forEventChan)
	}()

	return svc, ctx, userID, workspaceID, forEventChan
}

// collectForEvents collects for events from the channel with a timeout
func collectForEvents(eventChan <-chan ForEvent, count int, timeout time.Duration) []ForEvent {
	events := make([]ForEvent, 0, count)
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

// TestForEventToSyncResponse_ZeroIterations verifies that zero iterations are correctly
// included in sync update responses. This is a regression test for a bug where
// setting iterations to 0 was not persisted because the check `iterations != 0`
// incorrectly excluded zero values from sync updates.
func TestForEventToSyncResponse_ZeroIterations(t *testing.T) {
	nodeID := idwrap.NewNow()

	t.Run("zero iterations included in update response", func(t *testing.T) {
		// Create a For node with zero iterations
		nodePB := &flowv1.NodeFor{
			NodeId:     nodeID.Bytes(),
			Iterations: 0, // Explicitly zero
			Condition:  "true",
		}

		event := ForEvent{
			Type:   forEventUpdate,
			FlowID: idwrap.NewNow(),
			Node:   nodePB,
		}

		resp := forEventToSyncResponse(event)

		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		item := resp.Items[0]
		require.NotNil(t, item.Value)
		assert.Equal(t, flowv1.NodeForSync_ValueUnion_KIND_UPDATE, item.Value.Kind)
		require.NotNil(t, item.Value.Update)

		update := item.Value.Update
		assert.Equal(t, nodeID.Bytes(), update.NodeId)

		// This is the critical assertion - iterations MUST be set even when 0
		require.NotNil(t, update.Iterations, "Iterations field must be set even when value is 0")
		assert.Equal(t, int32(0), *update.Iterations, "Iterations value must be 0")
	})

	t.Run("non-zero iterations included in update response", func(t *testing.T) {
		nodePB := &flowv1.NodeFor{
			NodeId:     nodeID.Bytes(),
			Iterations: 5,
			Condition:  "i < 10",
		}

		event := ForEvent{
			Type:   forEventUpdate,
			FlowID: idwrap.NewNow(),
			Node:   nodePB,
		}

		resp := forEventToSyncResponse(event)

		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.Update
		require.NotNil(t, update.Iterations)
		assert.Equal(t, int32(5), *update.Iterations)
	})

	t.Run("insert includes all fields", func(t *testing.T) {
		nodePB := &flowv1.NodeFor{
			NodeId:        nodeID.Bytes(),
			Iterations:    0,
			Condition:     "test",
			ErrorHandling: flowv1.ErrorHandling_ERROR_HANDLING_BREAK,
		}

		event := ForEvent{
			Type:   forEventInsert,
			FlowID: idwrap.NewNow(),
			Node:   nodePB,
		}

		resp := forEventToSyncResponse(event)

		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		item := resp.Items[0]
		assert.Equal(t, flowv1.NodeForSync_ValueUnion_KIND_INSERT, item.Value.Kind)
		require.NotNil(t, item.Value.Insert)

		insert := item.Value.Insert
		assert.Equal(t, int32(0), insert.Iterations)
		assert.Equal(t, "test", insert.Condition)
		assert.Equal(t, flowv1.ErrorHandling_ERROR_HANDLING_BREAK, insert.ErrorHandling)
	})
}

// TestNodeForSync_ZeroIterationsUpdate is an integration test verifying that
// updating a For node's iterations to 0 correctly publishes the update event
// with the zero value included.
func TestNodeForSync_ZeroIterationsUpdate(t *testing.T) {
	svc, ctx, _, workspaceID, eventChan := setupTestServiceWithForStream(t)

	// Create flow
	flowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (FOR kind)
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
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

	_, err = svc.NodeForInsert(ctx, insertReq)
	require.NoError(t, err)

	// Collect the insert event
	insertEvents := collectForEvents(eventChan, 1, 100*time.Millisecond)
	require.Len(t, insertEvents, 1)
	assert.Equal(t, forEventInsert, insertEvents[0].Type)

	t.Run("update iterations to zero", func(t *testing.T) {
		// Update iterations to 0
		zeroIterations := int32(0)
		updateReq := connect.NewRequest(&flowv1.NodeForUpdateRequest{
			Items: []*flowv1.NodeForUpdate{{
				NodeId:     nodeID.Bytes(),
				Iterations: &zeroIterations,
			}},
		})

		_, err := svc.NodeForUpdate(ctx, updateReq)
		require.NoError(t, err)

		// Collect the update event
		updateEvents := collectForEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, updateEvents, 1, "Should receive update event")

		event := updateEvents[0]
		assert.Equal(t, forEventUpdate, event.Type)
		require.NotNil(t, event.Node)

		// Convert to sync response and verify zero iterations is included
		resp := forEventToSyncResponse(event)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.Update
		require.NotNil(t, update, "Update should not be nil")
		require.NotNil(t, update.Iterations, "Iterations must be set in update (zero is valid)")
		assert.Equal(t, int32(0), *update.Iterations, "Iterations should be 0")
	})

	t.Run("update iterations from zero to non-zero", func(t *testing.T) {
		// Update iterations from 0 to 10
		tenIterations := int32(10)
		updateReq := connect.NewRequest(&flowv1.NodeForUpdateRequest{
			Items: []*flowv1.NodeForUpdate{{
				NodeId:     nodeID.Bytes(),
				Iterations: &tenIterations,
			}},
		})

		_, err := svc.NodeForUpdate(ctx, updateReq)
		require.NoError(t, err)

		// Collect the update event
		updateEvents := collectForEvents(eventChan, 1, 100*time.Millisecond)
		require.Len(t, updateEvents, 1, "Should receive update event")

		resp := forEventToSyncResponse(updateEvents[0])
		require.NotNil(t, resp)

		update := resp.Items[0].Value.Update
		require.NotNil(t, update.Iterations)
		assert.Equal(t, int32(10), *update.Iterations)
	})
}
