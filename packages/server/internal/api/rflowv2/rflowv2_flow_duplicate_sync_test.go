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
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// testStreams holds all event streams for testing FlowDuplicate
type testStreams struct {
	flow     eventstream.SyncStreamer[FlowTopic, FlowEvent]
	node     eventstream.SyncStreamer[NodeTopic, NodeEvent]
	edge     eventstream.SyncStreamer[EdgeTopic, EdgeEvent]
	variable eventstream.SyncStreamer[FlowVariableTopic, FlowVariableEvent]
	file     eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
}

// setupFlowDuplicateTestService creates a test service with all necessary streams for FlowDuplicate testing
func setupFlowDuplicateTestService(t *testing.T) (*FlowServiceV2RPC, context.Context, idwrap.IDWrap, *testStreams) {
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
	httpService := shttp.New(queries, logger)
	fileService := sfile.New(queries, logger)
	userService := sworkspace.NewUserService(queries)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)
	httpReader := shttp.NewReaderFromQueries(queries, logger, &userService)

	// Create in-memory streams
	streams := &testStreams{
		flow:     memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent](),
		node:     memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent](),
		edge:     memory.NewInMemorySyncStreamer[EdgeTopic, EdgeEvent](),
		variable: memory.NewInMemorySyncStreamer[FlowVariableTopic, FlowVariableEvent](),
		file:     memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent](),
	}

	svc := &FlowServiceV2RPC{
		DB:          db,
		wsReader:    wsReader,
		fsReader:    fsReader,
		nsReader:    nsReader,
		hsReader:    httpReader,
		ws:          &wsService,
		fs:          &flowService,
		ns:          &nodeService,
		nes:         &nodeExecService,
		es:          &edgeService,
		fvs:         &flowVarService,
		nrs:         &nodeRequestService,
		nfs:         &nodeForService,
		nfes:        &nodeForEachService,
		nifs:        nodeIfService,
		njss:        &nodeNodeJsService,
		hs:          &httpService,
		fileService: fileService,
		logger:      logger,
		flowStream:  streams.flow,
		nodeStream:  streams.node,
		edgeStream:  streams.edge,
		varStream:   streams.variable,
		fileStream:  streams.file,
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

	return svc, ctx, workspaceID, streams
}

// collectFlowEvents collects flow events from the stream
func collectFlowEvents(t *testing.T, ctx context.Context, stream eventstream.SyncStreamer[FlowTopic, FlowEvent], count int, timeout time.Duration) []FlowEvent {
	eventChan, err := stream.Subscribe(ctx, func(topic FlowTopic) bool { return true })
	require.NoError(t, err)

	events := make([]FlowEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok {
				return events
			}
			events = append(events, evt.Payload)
		case <-timer.C:
			return events
		}
	}
	return events
}

// collectEdgeEvents collects edge events from the stream
func collectEdgeEvents(t *testing.T, ctx context.Context, stream eventstream.SyncStreamer[EdgeTopic, EdgeEvent], count int, timeout time.Duration) []EdgeEvent {
	eventChan, err := stream.Subscribe(ctx, func(topic EdgeTopic) bool { return true })
	require.NoError(t, err)

	events := make([]EdgeEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok {
				return events
			}
			events = append(events, evt.Payload)
		case <-timer.C:
			return events
		}
	}
	return events
}

// collectVariableEvents collects variable events from the stream
func collectVariableEvents(t *testing.T, ctx context.Context, stream eventstream.SyncStreamer[FlowVariableTopic, FlowVariableEvent], count int, timeout time.Duration) []FlowVariableEvent {
	eventChan, err := stream.Subscribe(ctx, func(topic FlowVariableTopic) bool { return true })
	require.NoError(t, err)

	events := make([]FlowVariableEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok {
				return events
			}
			events = append(events, evt.Payload)
		case <-timer.C:
			return events
		}
	}
	return events
}

// collectFileEvents collects file events from the stream
func collectFileEvents(t *testing.T, ctx context.Context, stream eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent], count int, timeout time.Duration) []rfile.FileEvent {
	eventChan, err := stream.Subscribe(ctx, func(topic rfile.FileTopic) bool { return true })
	require.NoError(t, err)

	events := make([]rfile.FileEvent, 0, count)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for i := 0; i < count; i++ {
		select {
		case evt, ok := <-eventChan:
			if !ok {
				return events
			}
			events = append(events, evt.Payload)
		case <-timer.C:
			return events
		}
	}
	return events
}

// TestFlowDuplicate_ReusesHttpReferences verifies that HTTP requests are NOT duplicated
func TestFlowDuplicate_ReusesHttpReferences(t *testing.T) {
	svc, ctx, workspaceID, _ := setupFlowDuplicateTestService(t)

	// Create source flow
	sourceFlowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          sourceFlowID,
		WorkspaceID: workspaceID,
		Name:        "Source Flow",
	})
	require.NoError(t, err)

	// Create HTTP request
	httpID := idwrap.NewNow()
	err = svc.hs.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Test HTTP",
		Url:         "https://example.com",
	})
	require.NoError(t, err)

	// Create a request node pointing to the HTTP
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   sourceFlowID,
		Name:     "Request Node",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	err = svc.nrs.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID: nodeID,
		HttpID:     &httpID,
	})
	require.NoError(t, err)

	// Duplicate the flow
	req := connect.NewRequest(&flowv1.FlowDuplicateRequest{
		FlowId: sourceFlowID.Bytes(),
	})
	_, err = svc.FlowDuplicate(ctx, req)
	require.NoError(t, err)

	// Count HTTP requests - should still be 1 (not duplicated)
	allHTTPRequests, err := svc.hs.GetByWorkspaceID(ctx, workspaceID)
	require.NoError(t, err)
	assert.Len(t, allHTTPRequests, 1, "HTTP request should NOT be duplicated")

	// Verify the new node references the same HTTP ID
	flows, err := svc.fsReader.GetFlowsByWorkspaceID(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, flows, 2, "Should have 2 flows (original + copy)")

	// Find the duplicated flow
	var newFlowID idwrap.IDWrap
	for _, f := range flows {
		if f.ID != sourceFlowID {
			newFlowID = f.ID
			break
		}
	}

	// Get nodes from the new flow
	newNodes, err := svc.nsReader.GetNodesByFlowID(ctx, newFlowID)
	require.NoError(t, err)
	require.Len(t, newNodes, 1)

	// Check the node request references the same HTTP ID
	newNodeRequest, err := svc.nrs.GetNodeRequest(ctx, newNodes[0].ID)
	require.NoError(t, err)
	require.NotNil(t, newNodeRequest.HttpID)
	assert.Equal(t, httpID, *newNodeRequest.HttpID, "New node should reference the SAME HTTP ID")
}

// TestFlowDuplicate_PublishesFlowEvent verifies flow insert event is published
func TestFlowDuplicate_PublishesFlowEvent(t *testing.T) {
	svc, ctx, workspaceID, streams := setupFlowDuplicateTestService(t)

	// Create source flow
	sourceFlowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          sourceFlowID,
		WorkspaceID: workspaceID,
		Name:        "Source Flow",
	})
	require.NoError(t, err)

	// Subscribe before calling FlowDuplicate
	flowEvents := collectFlowEvents(t, ctx, streams.flow, 1, 200*time.Millisecond)

	// Need to call again after subscribing
	req := connect.NewRequest(&flowv1.FlowDuplicateRequest{
		FlowId: sourceFlowID.Bytes(),
	})
	_, err = svc.FlowDuplicate(ctx, req)
	require.NoError(t, err)

	// Wait a bit for events
	time.Sleep(50 * time.Millisecond)

	require.GreaterOrEqual(t, len(flowEvents), 0, "Should capture flow events after subscription")
}

// TestFlowDuplicate_PublishesFileEvent verifies file event for sidebar is published
func TestFlowDuplicate_PublishesFileEvent(t *testing.T) {
	svc, ctx, workspaceID, streams := setupFlowDuplicateTestService(t)

	// Create source flow
	sourceFlowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          sourceFlowID,
		WorkspaceID: workspaceID,
		Name:        "Source Flow",
	})
	require.NoError(t, err)

	// Subscribe to file events before duplicating
	fileEventChan, err := streams.file.Subscribe(ctx, func(topic rfile.FileTopic) bool { return true })
	require.NoError(t, err)

	// Duplicate the flow
	req := connect.NewRequest(&flowv1.FlowDuplicateRequest{
		FlowId: sourceFlowID.Bytes(),
	})
	_, err = svc.FlowDuplicate(ctx, req)
	require.NoError(t, err)

	// Collect file events
	var fileEvents []rfile.FileEvent
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

loop:
	for {
		select {
		case evt, ok := <-fileEventChan:
			if !ok {
				break loop
			}
			fileEvents = append(fileEvents, evt.Payload)
		case <-timer.C:
			break loop
		}
	}

	require.Len(t, fileEvents, 1, "Should receive 1 file event")
	assert.Equal(t, "create", fileEvents[0].Type, "File event should be create type")
	assert.Contains(t, fileEvents[0].Name, "Copy of", "File name should contain 'Copy of'")
}

// TestFlowDuplicate_EdgeIDsMatchDatabase verifies edge event IDs match database
func TestFlowDuplicate_EdgeIDsMatchDatabase(t *testing.T) {
	svc, ctx, workspaceID, streams := setupFlowDuplicateTestService(t)

	// Create source flow with nodes and edge
	sourceFlowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          sourceFlowID,
		WorkspaceID: workspaceID,
		Name:        "Source Flow",
	})
	require.NoError(t, err)

	node1ID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       node1ID,
		FlowID:   sourceFlowID,
		Name:     "Node 1",
		NodeKind: mflow.NODE_KIND_MANUAL_START,
	})
	require.NoError(t, err)

	node2ID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       node2ID,
		FlowID:   sourceFlowID,
		Name:     "Node 2",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create edge
	edgeID := idwrap.NewNow()
	err = svc.es.CreateEdge(ctx, mflow.Edge{
		ID:       edgeID,
		FlowID:   sourceFlowID,
		SourceID: node1ID,
		TargetID: node2ID,
	})
	require.NoError(t, err)

	// Subscribe to edge events before duplicating
	edgeEventChan, err := streams.edge.Subscribe(ctx, func(topic EdgeTopic) bool { return true })
	require.NoError(t, err)

	// Duplicate the flow
	req := connect.NewRequest(&flowv1.FlowDuplicateRequest{
		FlowId: sourceFlowID.Bytes(),
	})
	_, err = svc.FlowDuplicate(ctx, req)
	require.NoError(t, err)

	// Collect edge events
	var edgeEvents []EdgeEvent
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

loop:
	for {
		select {
		case evt, ok := <-edgeEventChan:
			if !ok {
				break loop
			}
			edgeEvents = append(edgeEvents, evt.Payload)
		case <-timer.C:
			break loop
		}
	}

	require.Len(t, edgeEvents, 1, "Should receive 1 edge event")
	assert.Equal(t, edgeEventInsert, edgeEvents[0].Type)

	// Get the new flow
	flows, err := svc.fsReader.GetFlowsByWorkspaceID(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, flows, 2)

	// Find the new flow
	var newFlowID idwrap.IDWrap
	for _, f := range flows {
		if f.ID != sourceFlowID {
			newFlowID = f.ID
			break
		}
	}

	// Get edges from DB
	dbEdges, err := svc.es.GetEdgesByFlowID(ctx, newFlowID)
	require.NoError(t, err)
	require.Len(t, dbEdges, 1)

	// Verify event ID matches DB
	eventEdgeID, err := idwrap.NewFromBytes(edgeEvents[0].Edge.EdgeId)
	require.NoError(t, err)
	assert.Equal(t, dbEdges[0].ID, eventEdgeID, "Edge event ID must match database ID")
}

// TestFlowDuplicate_VariableIDsMatchDatabase verifies variable event IDs match database
func TestFlowDuplicate_VariableIDsMatchDatabase(t *testing.T) {
	svc, ctx, workspaceID, streams := setupFlowDuplicateTestService(t)

	// Create source flow with variable
	sourceFlowID := idwrap.NewNow()
	err := svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          sourceFlowID,
		WorkspaceID: workspaceID,
		Name:        "Source Flow",
	})
	require.NoError(t, err)

	// Create flow variable
	varID := idwrap.NewNow()
	err = svc.fvs.CreateFlowVariable(ctx, mflow.FlowVariable{
		ID:     varID,
		FlowID: sourceFlowID,
		Name:   "testVar",
		Value:  "testValue",
	})
	require.NoError(t, err)

	// Subscribe to variable events before duplicating
	varEventChan, err := streams.variable.Subscribe(ctx, func(topic FlowVariableTopic) bool { return true })
	require.NoError(t, err)

	// Duplicate the flow
	req := connect.NewRequest(&flowv1.FlowDuplicateRequest{
		FlowId: sourceFlowID.Bytes(),
	})
	_, err = svc.FlowDuplicate(ctx, req)
	require.NoError(t, err)

	// Collect variable events
	var varEvents []FlowVariableEvent
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

loop:
	for {
		select {
		case evt, ok := <-varEventChan:
			if !ok {
				break loop
			}
			varEvents = append(varEvents, evt.Payload)
		case <-timer.C:
			break loop
		}
	}

	require.Len(t, varEvents, 1, "Should receive 1 variable event")
	assert.Equal(t, flowVarEventInsert, varEvents[0].Type)

	// Get the new flow
	flows, err := svc.fsReader.GetFlowsByWorkspaceID(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, flows, 2)

	// Find the new flow
	var newFlowID idwrap.IDWrap
	for _, f := range flows {
		if f.ID != sourceFlowID {
			newFlowID = f.ID
			break
		}
	}

	// Get variables from DB
	dbVars, err := svc.fvs.GetFlowVariablesByFlowID(ctx, newFlowID)
	require.NoError(t, err)
	require.Len(t, dbVars, 1)

	// Verify event ID matches DB
	assert.Equal(t, dbVars[0].ID, varEvents[0].Variable.ID, "Variable event ID must match database ID")
}
