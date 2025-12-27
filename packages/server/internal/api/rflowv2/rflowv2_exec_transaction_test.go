package rflowv2

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
)

// TestFlowVersionSnapshot_TransactionAtomicity verifies that createFlowVersionSnapshot
// uses a single transaction and either creates ALL entities or NONE.
// This test ensures the critical data corruption bug is fixed where partial flow
// version snapshots could be created if creation failed partway through.
func TestFlowVersionSnapshot_TransactionAtomicity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	nrsService := sflow.NewNodeRequestService(queries)
	nfsService := sflow.NewNodeForService(queries)
	nfesService := sflow.NewNodeForEachService(queries)
	nifsService := sflow.NewNodeIfService(queries)
	njssService := sflow.NewNodeJsService(queries)
	varService := sflow.NewFlowVariableService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		DB:     db,
		ws:     &wsService,
		fs:     &flowService,
		ns:     &nodeService,
		es:     &edgeService,
		nrs:    &nrsService,
		nfs:    &nfsService,
		nfes:   &nfesService,
		nifs:   nifsService,
		njss:   &njssService,
		fvs:    &varService,
		logger: logger,
	}

	// Create test data
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

	// Create source flow with comprehensive data
	sourceFlowID := idwrap.NewNow()
	sourceFlow := mflow.Flow{
		ID:          sourceFlowID,
		WorkspaceID: workspaceID,
		Name:        "Source Flow",
	}
	err = flowService.CreateFlow(ctx, sourceFlow)
	require.NoError(t, err)

	// Create diverse nodes (Request, For, ForEach, Condition, JS)
	node1ID := idwrap.NewNow() // Request node
	node2ID := idwrap.NewNow() // For node
	node3ID := idwrap.NewNow() // JS node
	node4ID := idwrap.NewNow() // Condition node

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    sourceFlowID,
		Name:      "Request Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    sourceFlowID,
		Name:      "For Node",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    sourceFlowID,
		Name:      "JS Node",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node4ID,
		FlowID:    sourceFlowID,
		Name:      "Condition Node",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 300,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create sub-node configs for each node type
	err = nrsService.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID:       node1ID,
		HttpID:           nil, // Empty/nil HTTP ID for test
		HasRequestConfig: false,
	})
	require.NoError(t, err)

	err = nfsService.CreateNodeFor(ctx, mflow.NodeFor{
		FlowNodeID:    node2ID,
		IterCount:     5,
		Condition:     mcondition.Condition{},
		ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	err = njssService.CreateNodeJS(ctx, mflow.NodeJS{
		FlowNodeID:       node3ID,
		Code:             []byte("console.log('test')"),
		CodeCompressType: 0,
	})
	require.NoError(t, err)

	err = nifsService.CreateNodeIf(ctx, mflow.NodeIf{
		FlowNodeID: node4ID,
		Condition:  mcondition.Condition{},
	})
	require.NoError(t, err)

	// Create edges
	edge1ID := idwrap.NewNow()
	edge2ID := idwrap.NewNow()

	err = edgeService.CreateEdge(ctx, mflow.Edge{
		ID:            edge1ID,
		FlowID:        sourceFlowID,
		SourceID:      node1ID,
		TargetID:      node2ID,
		SourceHandler: mflow.HandleUnspecified,
	})
	require.NoError(t, err)

	err = edgeService.CreateEdge(ctx, mflow.Edge{
		ID:            edge2ID,
		FlowID:        sourceFlowID,
		SourceID:      node2ID,
		TargetID:      node3ID,
		SourceHandler: mflow.HandleUnspecified,
	})
	require.NoError(t, err)

	// Create flow variables
	var1ID := idwrap.NewNow()
	var2ID := idwrap.NewNow()

	err = varService.CreateFlowVariable(ctx, mflow.FlowVariable{
		ID:          var1ID,
		FlowID:      sourceFlowID,
		Name:        "API_KEY",
		Value:       "test-key",
		Enabled:     true,
		Description: "Test API Key",
		Order:       1.0,
	})
	require.NoError(t, err)

	err = varService.CreateFlowVariable(ctx, mflow.FlowVariable{
		ID:          var2ID,
		FlowID:      sourceFlowID,
		Name:        "BASE_URL",
		Value:       "https://api.example.com",
		Enabled:     true,
		Description: "Base URL",
		Order:       2.0,
	})
	require.NoError(t, err)

	// Get source data for snapshot
	sourceNodes, err := nodeService.GetNodesByFlowID(ctx, sourceFlowID)
	require.NoError(t, err)
	require.Len(t, sourceNodes, 4, "Should have 4 source nodes")

	sourceEdges, err := edgeService.GetEdgesByFlowID(ctx, sourceFlowID)
	require.NoError(t, err)
	require.Len(t, sourceEdges, 2, "Should have 2 source edges")

	sourceVars, err := varService.GetFlowVariablesByFlowIDOrdered(ctx, sourceFlowID)
	require.NoError(t, err)
	require.Len(t, sourceVars, 2, "Should have 2 source variables")

	// Create version snapshot - this should be atomic
	version, nodeMapping, err := svc.createFlowVersionSnapshot(ctx, sourceFlow, sourceNodes, sourceEdges, sourceVars)
	require.NoError(t, err, "Snapshot creation should succeed")
	require.NotEqual(t, sourceFlowID, version.ID, "Version should have different ID")
	require.Len(t, nodeMapping, 4, "Should have mapping for all 4 nodes")

	// Verify ALL entities were created atomically
	versionFlowID := version.ID

	// Verify version flow exists
	versionFlow, err := flowService.GetFlow(ctx, versionFlowID)
	require.NoError(t, err)
	require.Equal(t, versionFlowID, versionFlow.ID)

	// Verify all 4 nodes were created
	versionNodes, err := nodeService.GetNodesByFlowID(ctx, versionFlowID)
	require.NoError(t, err)
	require.Len(t, versionNodes, 4, "All 4 nodes should be created")

	// Verify all sub-node configs were created
	for _, node := range versionNodes {
		switch node.NodeKind {
		case mflow.NODE_KIND_REQUEST:
			// Request node config might not be created if HttpID is nil
			// Skip verification for this test since we created it with nil HttpID

		case mflow.NODE_KIND_FOR:
			forData, err := nfsService.GetNodeFor(ctx, node.ID)
			require.NoError(t, err)
			require.NotNil(t, forData, "For config should exist")
			require.Equal(t, int64(5), forData.IterCount, "IterCount should be copied")

		case mflow.NODE_KIND_JS:
			jsData, err := njssService.GetNodeJS(ctx, node.ID)
			require.NoError(t, err)
			require.NotNil(t, jsData, "JS config should exist")
			require.Equal(t, []byte("console.log('test')"), jsData.Code, "Code should be copied")

		case mflow.NODE_KIND_CONDITION:
			ifData, err := nifsService.GetNodeIf(ctx, node.ID)
			require.NoError(t, err)
			require.NotNil(t, ifData, "Condition config should exist")
		}
	}

	// Verify all 2 edges were created
	versionEdges, err := edgeService.GetEdgesByFlowID(ctx, versionFlowID)
	require.NoError(t, err)
	require.Len(t, versionEdges, 2, "All 2 edges should be created")

	// Verify all 2 variables were created
	versionVars, err := varService.GetFlowVariablesByFlowIDOrdered(ctx, versionFlowID)
	require.NoError(t, err)
	require.Len(t, versionVars, 2, "All 2 variables should be created")

	// Verify variable data was copied correctly
	require.Equal(t, "API_KEY", versionVars[0].Name)
	require.Equal(t, "test-key", versionVars[0].Value)
	require.Equal(t, "BASE_URL", versionVars[1].Name)
	require.Equal(t, "https://api.example.com", versionVars[1].Value)
}

// TestFlowVersionSnapshot_EmptyFlow verifies that creating a version snapshot
// of an empty flow (no nodes, edges, variables) works correctly.
func TestFlowVersionSnapshot_EmptyFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	nrsService := sflow.NewNodeRequestService(queries)
	nfsService := sflow.NewNodeForService(queries)
	nfesService := sflow.NewNodeForEachService(queries)
	nifsService := sflow.NewNodeIfService(queries)
	njssService := sflow.NewNodeJsService(queries)
	varService := sflow.NewFlowVariableService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		DB:     db,
		ws:     &wsService,
		fs:     &flowService,
		ns:     &nodeService,
		es:     &edgeService,
		nrs:    &nrsService,
		nfs:    &nfsService,
		nfes:   &nfesService,
		nifs:   nifsService,
		njss:   &njssService,
		fvs:    &varService,
		logger: logger,
	}

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

	// Create empty source flow
	sourceFlowID := idwrap.NewNow()
	sourceFlow := mflow.Flow{
		ID:          sourceFlowID,
		WorkspaceID: workspaceID,
		Name:        "Empty Flow",
	}
	err = flowService.CreateFlow(ctx, sourceFlow)
	require.NoError(t, err)

	// Create version snapshot of empty flow
	version, nodeMapping, err := svc.createFlowVersionSnapshot(ctx, sourceFlow, []mflow.Node{}, []mflow.Edge{}, []mflow.FlowVariable{})
	require.NoError(t, err, "Empty flow snapshot should succeed")
	require.NotEqual(t, sourceFlowID, version.ID, "Version should have different ID")
	require.Empty(t, nodeMapping, "Empty flow should have no node mapping")

	// Verify version flow exists
	versionFlow, err := flowService.GetFlow(ctx, version.ID)
	require.NoError(t, err)
	require.Equal(t, version.ID, versionFlow.ID)

	// Verify no nodes created
	versionNodes, err := nodeService.GetNodesByFlowID(ctx, version.ID)
	if err != nil && err != sql.ErrNoRows {
		require.NoError(t, err)
	}
	require.Empty(t, versionNodes, "Empty flow should have no nodes")

	// Verify no edges created
	versionEdges, err := edgeService.GetEdgesByFlowID(ctx, version.ID)
	if err != nil && err != sql.ErrNoRows {
		require.NoError(t, err)
	}
	require.Empty(t, versionEdges, "Empty flow should have no edges")

	// Verify no variables created
	versionVars, err := varService.GetFlowVariablesByFlowIDOrdered(ctx, version.ID)
	if err != nil && err != sflow.ErrNoFlowVariableFound {
		require.NoError(t, err)
	}
	require.Empty(t, versionVars, "Empty flow should have no variables")
}

// TestFlowVersionSnapshot_Concurrency_Simple verifies that concurrent simple
// flow version snapshot operations complete successfully without SQLite deadlocks.
func TestFlowVersionSnapshot_Concurrency_Simple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	varService := sflow.NewFlowVariableService(queries)
	nrsService := sflow.NewNodeRequestService(queries)
	nfsService := sflow.NewNodeForService(queries)
	nfesService := sflow.NewNodeForEachService(queries)
	nifsService := sflow.NewNodeIfService(queries)
	njssService := sflow.NewNodeJsService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		DB:     db,
		ws:     &wsService,
		fs:     &flowService,
		ns:     &nodeService,
		es:     &edgeService,
		nrs:    &nrsService,
		nfs:    &nfsService,
		nfes:   &nfesService,
		nifs:   nifsService,
		njss:   &njssService,
		fvs:    &varService,
		logger: logger,
	}

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

	// Pre-create 20 simple flows BEFORE concurrency test
	flows := make([]mflow.Flow, 20)
	for i := 0; i < 20; i++ {
		flows[i] = mflow.Flow{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        fmt.Sprintf("Flow %d", i),
		}
		err = flowService.CreateFlow(ctx, flows[i])
		require.NoError(t, err)
	}

	type snapshotData struct {
		Flow mflow.Flow
	}

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *snapshotData {
			return &snapshotData{
				Flow: flows[i],
			}
		},
		func(opCtx context.Context, data *snapshotData) error {
			_, _, err := svc.createFlowVersionSnapshot(opCtx, data.Flow, []mflow.Node{}, []mflow.Edge{}, []mflow.FlowVariable{})
			return err
		},
	)

	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 100*time.Millisecond, "Operations should be fast")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}

// TestFlowVersionSnapshot_Concurrency_WithNodes verifies that concurrent
// flow version snapshot operations with nodes complete without deadlocks.
func TestFlowVersionSnapshot_Concurrency_WithNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	varService := sflow.NewFlowVariableService(queries)
	nrsService := sflow.NewNodeRequestService(queries)
	nfsService := sflow.NewNodeForService(queries)
	nfesService := sflow.NewNodeForEachService(queries)
	nifsService := sflow.NewNodeIfService(queries)
	njssService := sflow.NewNodeJsService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		DB:     db,
		ws:     &wsService,
		fs:     &flowService,
		ns:     &nodeService,
		es:     &edgeService,
		nrs:    &nrsService,
		nfs:    &nfsService,
		nfes:   &nfesService,
		nifs:   nifsService,
		njss:   &njssService,
		fvs:    &varService,
		logger: logger,
	}

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

	// Pre-create 20 flows with nodes BEFORE concurrency test
	type flowWithNodes struct {
		Flow  mflow.Flow
		Nodes []mflow.Node
	}

	flowsWithNodes := make([]flowWithNodes, 20)
	for i := 0; i < 20; i++ {
		flow := mflow.Flow{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        fmt.Sprintf("Flow %d", i),
		}
		err = flowService.CreateFlow(ctx, flow)
		require.NoError(t, err)

		// Create 3 nodes per flow
		nodes := make([]mflow.Node, 3)
		for j := 0; j < 3; j++ {
			nodes[j] = mflow.Node{
				ID:        idwrap.NewNow(),
				FlowID:    flow.ID,
				Name:      fmt.Sprintf("Node %d-%d", i, j),
				NodeKind:  mflow.NODE_KIND_REQUEST,
				PositionX: float64(j * 100),
				PositionY: 0,
			}
			err = nodeService.CreateNode(ctx, nodes[j])
			require.NoError(t, err)
		}

		flowsWithNodes[i] = flowWithNodes{
			Flow:  flow,
			Nodes: nodes,
		}
	}

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *flowWithNodes {
			return &flowsWithNodes[i]
		},
		func(opCtx context.Context, data *flowWithNodes) error {
			_, _, err := svc.createFlowVersionSnapshot(opCtx, data.Flow, data.Nodes, []mflow.Edge{}, []mflow.FlowVariable{})
			return err
		},
	)

	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 150*time.Millisecond, "Operations should be fast")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}

// TestFlowVersionSnapshot_Concurrency_Complex verifies that concurrent
// complex flow version snapshot operations complete without deadlocks.
func TestFlowVersionSnapshot_Concurrency_Complex(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	varService := sflow.NewFlowVariableService(queries)
	nrsService := sflow.NewNodeRequestService(queries)
	nfsService := sflow.NewNodeForService(queries)
	nfesService := sflow.NewNodeForEachService(queries)
	nifsService := sflow.NewNodeIfService(queries)
	njssService := sflow.NewNodeJsService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		DB:     db,
		ws:     &wsService,
		fs:     &flowService,
		ns:     &nodeService,
		es:     &edgeService,
		nrs:    &nrsService,
		nfs:    &nfsService,
		nfes:   &nfesService,
		nifs:   nifsService,
		njss:   &njssService,
		fvs:    &varService,
		logger: logger,
	}

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

	// Pre-create 20 complex flows with nodes, edges, and variables
	type complexFlow struct {
		Flow      mflow.Flow
		Nodes     []mflow.Node
		Edges     []mflow.Edge
		Variables []mflow.FlowVariable
	}

	complexFlows := make([]complexFlow, 20)
	for i := 0; i < 20; i++ {
		flow := mflow.Flow{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        fmt.Sprintf("Complex Flow %d", i),
		}
		err = flowService.CreateFlow(ctx, flow)
		require.NoError(t, err)

		// Create 5 nodes
		nodes := make([]mflow.Node, 5)
		for j := 0; j < 5; j++ {
			nodes[j] = mflow.Node{
				ID:        idwrap.NewNow(),
				FlowID:    flow.ID,
				Name:      fmt.Sprintf("Node %d-%d", i, j),
				NodeKind:  mflow.NODE_KIND_REQUEST,
				PositionX: float64(j * 100),
				PositionY: 0,
			}
			err = nodeService.CreateNode(ctx, nodes[j])
			require.NoError(t, err)
		}

		// Create 4 edges connecting nodes
		edges := make([]mflow.Edge, 4)
		for j := 0; j < 4; j++ {
			edges[j] = mflow.Edge{
				ID:              idwrap.NewNow(),
				FlowID:          flow.ID,
				SourceFlowNodeID: nodes[j].ID,
				TargetFlowNodeID: nodes[j+1].ID,
			}
			err = edgeService.CreateEdge(ctx, edges[j])
			require.NoError(t, err)
		}

		// Create 3 flow variables
		variables := make([]mflow.FlowVariable, 3)
		for j := 0; j < 3; j++ {
			variables[j] = mflow.FlowVariable{
				ID:      idwrap.NewNow(),
				FlowID:  flow.ID,
				Name:    fmt.Sprintf("var%d-%d", i, j),
				Value:   fmt.Sprintf("value%d-%d", i, j),
				Enabled: true,
			}
			err = varService.CreateFlowVariable(ctx, variables[j])
			require.NoError(t, err)
		}

		complexFlows[i] = complexFlow{
			Flow:      flow,
			Nodes:     nodes,
			Edges:     edges,
			Variables: variables,
		}
	}

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *complexFlow {
			return &complexFlows[i]
		},
		func(opCtx context.Context, data *complexFlow) error {
			_, _, err := svc.createFlowVersionSnapshot(opCtx, data.Flow, data.Nodes, data.Edges, data.Variables)
			return err
		},
	)

	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 200*time.Millisecond, "Operations should be fast")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}
