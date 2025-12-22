package rflowv2

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/flowbuilder"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func setupTestService(t *testing.T) (*FlowServiceV2RPC, *gen.Queries, context.Context, idwrap.IDWrap, idwrap.IDWrap) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Missing services for builder
	reqService := sflow.NewNodeRequestService(queries)
	forService := sflow.NewNodeForService(queries)
	forEachService := sflow.NewNodeForEachService(queries)
	ifService := sflow.NewNodeIfService(queries)
	jsService := sflow.NewNodeJsService(queries)
	varService := senv.NewVariableService(queries, logger)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	// Mock resolver
	res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

	builder := flowbuilder.New(
		&nodeService,
		&reqService,
		&forService,
		&forEachService,
		ifService,
		&jsService,
		&wsService,
		&varService,
		&flowVarService,
		res,
		logger,
	)

	svc := &FlowServiceV2RPC{
		DB:           db,
		wsReader:     wsReader,
		fsReader:     fsReader,
		nsReader:     nsReader,
		ws:           &wsService,
		fs:           &flowService,
		ns:           &nodeService,
		nes:          &nodeExecService,
		es:           &edgeService,
		fvs:          &flowVarService,
		nrs:          &reqService, // Added missing services to struct
		nfs:          &forService,
		nfes:         &forEachService,
		nifs:         ifService,
		njss:         &jsService,
		logger:       logger,
		builder:      builder,
		runningFlows: make(map[string]context.CancelFunc),
	}

	// Setup User & Workspace
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         dbtime.DBNow(),
		CollectionCount: 0,
		FlowCount:       0,
	}
	err = wsService.Create(ctx, &workspace)
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	return svc, queries, ctx, userID, workspaceID
}

func TestFlowStop(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create a flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := svc.fs.CreateFlow(ctx, flow)
	require.NoError(t, err)

	t.Run("Stop running flow", func(t *testing.T) {
		// Manually add a cancellation function to runningFlows
		cancelled := false
		cancelFunc := func() {
			cancelled = true
		}

		svc.runningFlowsMu.Lock()
		svc.runningFlows[flowID.String()] = cancelFunc
		svc.runningFlowsMu.Unlock()

		req := connect.NewRequest(&flowv1.FlowStopRequest{
			FlowId: flowID.Bytes(),
		})

		resp, err := svc.FlowStop(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.IsType(t, &emptypb.Empty{}, resp.Msg)

		assert.True(t, cancelled, "Cancel function should have been called")
	})

	t.Run("Stop non-running flow (idempotent)", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.FlowStopRequest{
			FlowId: flowID.Bytes(),
		})

		resp, err := svc.FlowStop(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Invalid flow ID", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.FlowStopRequest{
			FlowId: []byte("invalid-id"),
		})

		_, err := svc.FlowStop(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Missing flow ID", func(t *testing.T) {
		req := connect.NewRequest(&flowv1.FlowStopRequest{})

		_, err := svc.FlowStop(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Unauthorized access", func(t *testing.T) {
		// Create another user context
		otherUserID := idwrap.NewNow()
		otherCtx := mwauth.CreateAuthedContext(context.Background(), otherUserID)

		req := connect.NewRequest(&flowv1.FlowStopRequest{
			FlowId: flowID.Bytes(),
		})

		_, err := svc.FlowStop(otherCtx, req)
		assert.Error(t, err)
		// Should fail because user doesn't exist or has no access
		// The EnsureFlowAccess check usually returns specific errors, let's just check it fails
	})
}

func TestCreateFlowVersionSnapshot(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create a flow
	flowID := idwrap.NewNow()
	sourceFlow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Snapshot Test Flow",
	}
	err := svc.fs.CreateFlow(ctx, sourceFlow)
	require.NoError(t, err)

	// Create Nodes
	node1ID := idwrap.NewNow()
	node1 := mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Node 1",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 10,
		PositionY: 20,
	}
	err = svc.ns.CreateNode(ctx, node1)
	require.NoError(t, err)

	node2ID := idwrap.NewNow()
	node2 := mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Node 2",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 200,
	}
	err = svc.ns.CreateNode(ctx, node2)
	require.NoError(t, err)

	httpID := idwrap.NewNow()
	err = svc.nrs.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID: node2ID,
		HttpID:     &httpID,
	})
	require.NoError(t, err)

	// Create Edge
	edgeID := idwrap.NewNow()
	sourceEdge := mflow.Edge{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      node1ID,
		TargetID:      node2ID,
		SourceHandler: 0,
	}
	err = svc.es.CreateEdge(ctx, sourceEdge)
	require.NoError(t, err)

	// Create Variable
	varID := idwrap.NewNow()
	sourceVar := mflow.FlowVariable{
		ID:          varID,
		FlowID:      flowID,
		Name:        "TestVar",
		Value:       "TestValue",
		Enabled:     true,
		Description: "A test variable",
		Order:       1,
	}
	err = svc.fvs.CreateFlowVariable(ctx, sourceVar)
	require.NoError(t, err)

	// Prepare inputs for createFlowVersionSnapshot
	sourceNodes := []mflow.Node{node1, node2}
	sourceEdges := []mflow.Edge{sourceEdge}
	sourceVars := []mflow.FlowVariable{sourceVar}

	// EXECUTE
	versionFlow, nodeMapping, err := svc.createFlowVersionSnapshot(ctx, sourceFlow, sourceNodes, sourceEdges, sourceVars)

	// ASSERT
	require.NoError(t, err)
	assert.NotEqual(t, flowID, versionFlow.ID, "Version flow ID should be different")
	assert.Equal(t, *versionFlow.VersionParentID, flowID, "Version parent ID should match original flow ID")
	assert.Equal(t, sourceFlow.Name, versionFlow.Name)

	// Verify nodes
	versionNodes, err := svc.ns.GetNodesByFlowID(ctx, versionFlow.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(versionNodes))

	// Check node mapping
	assert.Equal(t, 2, len(nodeMapping))
	assert.Contains(t, nodeMapping, node1ID.String())
	assert.Contains(t, nodeMapping, node2ID.String())

	// Verify mapped nodes exist
	mappedNode1ID := nodeMapping[node1ID.String()]
	mappedNode2ID := nodeMapping[node2ID.String()]

	foundNode1 := false
	foundNode2 := false
	for _, n := range versionNodes {
		if n.ID == mappedNode1ID {
			foundNode1 = true
			assert.Equal(t, node1.Name, n.Name)
			assert.Equal(t, node1.NodeKind, n.NodeKind)
		} else if n.ID == mappedNode2ID {
			foundNode2 = true
			assert.Equal(t, node2.Name, n.Name)
			assert.Equal(t, node2.NodeKind, n.NodeKind)
		}
	}
	assert.True(t, foundNode1, "Mapped node 1 not found in version nodes")
	assert.True(t, foundNode2, "Mapped node 2 not found in version nodes")

	// Verify edges
	versionEdges, err := svc.es.GetEdgesByFlowID(ctx, versionFlow.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, len(versionEdges))
	assert.Equal(t, mappedNode1ID, versionEdges[0].SourceID)
	assert.Equal(t, mappedNode2ID, versionEdges[0].TargetID)

	// Verify variables
	versionVars, err := svc.fvs.GetFlowVariablesByFlowID(ctx, versionFlow.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, len(versionVars))
	assert.Equal(t, sourceVar.Name, versionVars[0].Name)
	assert.Equal(t, sourceVar.Value, versionVars[0].Value)

	// Verify sub-node data (Request)
	req2, err := svc.nrs.GetNodeRequest(ctx, mappedNode2ID)
	require.NoError(t, err)
	assert.Equal(t, httpID, *req2.HttpID)
}

func TestCreateFlowVersionSnapshot_ErrorHandling(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create a flow with invalid data structure to trigger errors
	// (hard to do with just structs, but we can try to pass invalid parent flow ID if we were mocking)
	// Since we are using a real DB, we can try to inject errors via context cancellation or by passing inconsistent data
	// But `createFlowVersionSnapshot` takes structs, not IDs, so it doesn't fetch.
	// However, `CreateFlowVersion` calls the DB.

	// We can try to close the DB connection to force errors?
	// Or pass a context that is already canceled?

	t.Run("Context canceled", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		flow := mflow.Flow{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        "Test Flow",
		}

		_, _, err := svc.createFlowVersionSnapshot(cancelledCtx, flow, nil, nil, nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}
