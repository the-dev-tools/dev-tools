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

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
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

func TestFlowExecutionStateReset(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create a flow with nodes and edges
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "State Reset Test Flow",
	}
	err := svc.fs.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create multiple nodes
	node1ID := idwrap.NewNow()
	node1 := mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Start Node",
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
		Name:      "Request Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 200,
	}
	err = svc.ns.CreateNode(ctx, node2)
	require.NoError(t, err)

	node3ID := idwrap.NewNow()
	node3 := mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "Condition Node",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 300,
		PositionY: 400,
	}
	err = svc.ns.CreateNode(ctx, node3)
	require.NoError(t, err)

	// Create edges
	edge1ID := idwrap.NewNow()
	edge1 := mflow.Edge{
		ID:            edge1ID,
		FlowID:        flowID,
		SourceID:      node1ID,
		TargetID:      node2ID,
		SourceHandler: mflow.HandleThen,
	}
	err = svc.es.CreateEdge(ctx, edge1)
	require.NoError(t, err)

	edge2ID := idwrap.NewNow()
	edge2 := mflow.Edge{
		ID:            edge2ID,
		FlowID:        flowID,
		SourceID:      node2ID,
		TargetID:      node3ID,
		SourceHandler: mflow.HandleThen,
	}
	err = svc.es.CreateEdge(ctx, edge2)
	require.NoError(t, err)

	// Set initial states to non-UNSPECIFIED values (simulating previous execution)
	err = svc.ns.UpdateNodeState(ctx, node1ID, mflow.NODE_STATE_SUCCESS)
	require.NoError(t, err)
	err = svc.ns.UpdateNodeState(ctx, node2ID, mflow.NODE_STATE_FAILURE)
	require.NoError(t, err)
	err = svc.ns.UpdateNodeState(ctx, node3ID, mflow.NODE_STATE_RUNNING)
	require.NoError(t, err)

	err = svc.es.UpdateEdgeState(ctx, edge1ID, mflow.NODE_STATE_SUCCESS)
	require.NoError(t, err)
	err = svc.es.UpdateEdgeState(ctx, edge2ID, mflow.NODE_STATE_FAILURE)
	require.NoError(t, err)

	// Verify initial states are set
	nodes, err := svc.ns.GetNodesByFlowID(ctx, flowID)
	require.NoError(t, err)
	assert.Len(t, nodes, 3)

	edges, err := svc.es.GetEdgesByFlowID(ctx, flowID)
	require.NoError(t, err)
	assert.Len(t, edges, 2)

	// Find nodes by ID for verification
	var node1Model, node2Model, node3Model *mflow.Node
	for i := range nodes {
		if nodes[i].ID == node1ID {
			node1Model = &nodes[i]
		} else if nodes[i].ID == node2ID {
			node2Model = &nodes[i]
		} else if nodes[i].ID == node3ID {
			node3Model = &nodes[i]
		}
	}

	require.NotNil(t, node1Model)
	require.NotNil(t, node2Model)
	require.NotNil(t, node3Model)

	assert.Equal(t, mflow.NODE_STATE_SUCCESS, node1Model.State)
	assert.Equal(t, mflow.NODE_STATE_FAILURE, node2Model.State)
	assert.Equal(t, mflow.NODE_STATE_RUNNING, node3Model.State)

	// Find edges by ID
	var edge1Model, edge2Model *mflow.Edge
	for i := range edges {
		if edges[i].ID == edge1ID {
			edge1Model = &edges[i]
		} else if edges[i].ID == edge2ID {
			edge2Model = &edges[i]
		}
	}

	require.NotNil(t, edge1Model)
	require.NotNil(t, edge2Model)

	assert.Equal(t, mflow.NODE_STATE_SUCCESS, edge1Model.State)
	assert.Equal(t, mflow.NODE_STATE_FAILURE, edge2Model.State)

	// Simulate the state reset that happens before execution
	// This is the code from executeFlow that resets states
	for _, node := range nodes {
		if err := svc.ns.UpdateNodeState(ctx, node.ID, mflow.NODE_STATE_UNSPECIFIED); err != nil {
			svc.logger.Error("failed to reset node state", "node_id", node.ID.String(), "error", err)
		}
	}

	for _, edge := range edges {
		if err := svc.es.UpdateEdgeState(ctx, edge.ID, mflow.NODE_STATE_UNSPECIFIED); err != nil {
			svc.logger.Error("failed to reset edge state", "edge_id", edge.ID.String(), "error", err)
		}
	}

	// Verify all node states were reset to UNSPECIFIED
	nodesAfterReset, err := svc.ns.GetNodesByFlowID(ctx, flowID)
	require.NoError(t, err)

	for _, node := range nodesAfterReset {
		assert.Equal(t, mflow.NODE_STATE_UNSPECIFIED, node.State,
			"Node %s should have UNSPECIFIED state after reset", node.ID.String())
	}

	// Verify all edge states were reset to UNSPECIFIED
	edgesAfterReset, err := svc.es.GetEdgesByFlowID(ctx, flowID)
	require.NoError(t, err)

	for _, edge := range edgesAfterReset {
		assert.Equal(t, mflow.NODE_STATE_UNSPECIFIED, edge.State,
			"Edge %s should have UNSPECIFIED state after reset", edge.ID.String())
	}
}

func TestFlowExecutionStateReset_PreventsStuckStates(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create a flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Stuck States Test Flow",
	}
	err := svc.fs.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create a simple flow with one node
	nodeID := idwrap.NewNow()
	node := mflow.Node{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 200,
	}
	err = svc.ns.CreateNode(ctx, node)
	require.NoError(t, err)

	// Simulate first execution with FAILURE state
	err = svc.ns.UpdateNodeState(ctx, nodeID, mflow.NODE_STATE_FAILURE)
	require.NoError(t, err)

	// Verify state is FAILURE
	nodeAfterFirstExec, err := svc.ns.GetNode(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, mflow.NODE_STATE_FAILURE, nodeAfterFirstExec.State,
		"Node should have FAILURE state after first execution")

	// Simulate state reset before second execution
	err = svc.ns.UpdateNodeState(ctx, nodeID, mflow.NODE_STATE_UNSPECIFIED)
	require.NoError(t, err)

	// Verify state was reset
	nodeAfterReset, err := svc.ns.GetNode(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, mflow.NODE_STATE_UNSPECIFIED, nodeAfterReset.State,
		"Node should have UNSPECIFIED state after reset")

	// Simulate second execution with SUCCESS state
	err = svc.ns.UpdateNodeState(ctx, nodeID, mflow.NODE_STATE_SUCCESS)
	require.NoError(t, err)

	// Verify state is SUCCESS (not stuck on FAILURE)
	nodeAfterSecondExec, err := svc.ns.GetNode(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, mflow.NODE_STATE_SUCCESS, nodeAfterSecondExec.State,
		"Node should have SUCCESS state after second execution")
}

func TestEdgesBySourceMap_LookupOptimization(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create a flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Edge Map Test Flow",
	}
	err := svc.fs.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create nodes
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

	node3ID := idwrap.NewNow()
	node3 := mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "Node 3",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 300,
		PositionY: 400,
	}
	err = svc.ns.CreateNode(ctx, node3)
	require.NoError(t, err)

	// Create multiple edges from the same source (to test edgesBySource map)
	edgeThenID := idwrap.NewNow()
	edgeThen := mflow.Edge{
		ID:            edgeThenID,
		FlowID:        flowID,
		SourceID:      node1ID,
		TargetID:      node2ID,
		SourceHandler: mflow.HandleThen,
	}
	err = svc.es.CreateEdge(ctx, edgeThen)
	require.NoError(t, err)

	edgeElseID := idwrap.NewNow()
	edgeElse := mflow.Edge{
		ID:            edgeElseID,
		FlowID:        flowID,
		SourceID:      node1ID,
		TargetID:      node3ID,
		SourceHandler: mflow.HandleElse,
	}
	err = svc.es.CreateEdge(ctx, edgeElse)
	require.NoError(t, err)

	// Fetch all edges
	edges, err := svc.es.GetEdgesByFlowID(ctx, flowID)
	require.NoError(t, err)
	assert.Len(t, edges, 2)

	// Build edgesBySource map (O(1) lookup optimization)
	edgesBySource := make(map[idwrap.IDWrap][]mflow.Edge, len(edges))
	for _, edge := range edges {
		edgesBySource[edge.SourceID] = append(edgesBySource[edge.SourceID], edge)
	}

	// Test O(1) lookup for edges from node1ID
	edgesFromNode1 := edgesBySource[node1ID]
	assert.Len(t, edgesFromNode1, 2, "Should find 2 edges from node1")

	// Verify we found the correct edges
	var foundThen, foundElse bool
	for _, edge := range edgesFromNode1 {
		if edge.ID == edgeThenID && edge.SourceHandler == mflow.HandleThen {
			foundThen = true
		}
		if edge.ID == edgeElseID && edge.SourceHandler == mflow.HandleElse {
			foundElse = true
		}
	}

	assert.True(t, foundThen, "Should find THEN edge")
	assert.True(t, foundElse, "Should find ELSE edge")

	// Test lookup for node with no outgoing edges
	edgesFromNode2 := edgesBySource[node2ID]
	assert.Len(t, edgesFromNode2, 0, "Should find no edges from node2")

	// Test lookup for non-existent node
	nonExistentNodeID := idwrap.NewNow()
	edgesFromNonExistent := edgesBySource[nonExistentNodeID]
	assert.Len(t, edgesFromNonExistent, 0, "Should find no edges for non-existent node")
}
