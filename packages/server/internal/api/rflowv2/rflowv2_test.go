package rflowv2

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

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

func TestFlowRun_MultipleRuns(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

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
		nrs:          &reqService,
		nfs:          &forService,
		nfes:         &forEachService,
		nifs:         ifService,
		njss:         &jsService,
		logger:       logger,
		builder:      builder,
		runningFlows: make(map[string]context.CancelFunc),
	}

	// Setup Data
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

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create Start Node (ManualStart)
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	// Run Multiple Times
	runCount := 10
	for i := 0; i < runCount; i++ {
		req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
		_, err = svc.FlowRun(ctx, req)
		require.NoError(t, err, "Run %d failed", i)
	}

	// Verification (Poll for SUCCESS)
	collReq := connect.NewRequest(&emptypb.Empty{})
	var resp *connect.Response[flowv1.NodeCollectionResponse]

	// Poll until we see SUCCESS state on the original flow's node
	// Note: NodeCollection now returns nodes from all flows including versions
	// We expect 1 original node + 10 version nodes = 11 total
	expectedNodeCount := 1 + runCount
	for i := 0; i < 20; i++ {
		resp, err = svc.NodeCollection(ctx, collReq)
		require.NoError(t, err)
		require.Len(t, resp.Msg.Items, expectedNodeCount, "Should have %d nodes (1 original + %d versions)", expectedNodeCount, runCount)

		// Find the original node by ID
		var originalNode *flowv1.Node
		for _, item := range resp.Msg.Items {
			if bytes.Equal(item.NodeId, startNodeID.Bytes()) {
				originalNode = item
				break
			}
		}
		require.NotNil(t, originalNode, "Original node should be in collection")

		if originalNode.State == flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Find original node and verify its state
	var originalNode *flowv1.Node
	for _, item := range resp.Msg.Items {
		if bytes.Equal(item.NodeId, startNodeID.Bytes()) {
			originalNode = item
			break
		}
	}
	require.NotNil(t, originalNode, "Original node should be in collection")
	assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS, originalNode.State)
}

const schema = `
-- USERS
CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  status INT8 NOT NULL DEFAULT 0,
  UNIQUE (provider_type, provider_id)
);

-- WORK SPACES
CREATE TABLE workspaces (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  updated BIGINT NOT NULL DEFAULT (unixepoch()),
  collection_count INT NOT NULL DEFAULT 0,
  flow_count INT NOT NULL DEFAULT 0,
  active_env BLOB,
  global_env BLOB,
  display_order REAL NOT NULL DEFAULT 0
);

CREATE TABLE workspaces_users (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  user_id BLOB NOT NULL,
  role INT8 NOT NULL DEFAULT 1,
  UNIQUE (workspace_id, user_id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- FILES
CREATE TABLE files (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  folder_id BLOB,
  content_id BLOB,
  content_kind INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL,
  display_order REAL NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL
);

-- FLOW
CREATE TABLE flow (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  version_parent_id BLOB DEFAULT NULL,
  name TEXT NOT NULL,
  duration INT NOT NULL DEFAULT 0,
  running BOOLEAN NOT NULL DEFAULT FALSE,
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (version_parent_id) REFERENCES flow (id) ON DELETE CASCADE
);

-- FLOW NODE
CREATE TABLE flow_node (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  name TEXT NOT NULL,
  node_kind INT NOT NULL,
  position_x REAL NOT NULL,
  position_y REAL NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
);

-- FLOW EDGE
CREATE TABLE flow_edge (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  source_id BLOB NOT NULL,
  target_id BLOB NOT NULL,
  source_handle INT NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE,
  FOREIGN KEY (source_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (target_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

-- FLOW VARIABLE
CREATE TABLE flow_variable (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOL NOT NULL,
  description TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  UNIQUE (flow_id, key),
  UNIQUE (prev, next, flow_id),
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
);

-- NODE EXECUTION
CREATE TABLE node_execution (
  id BLOB NOT NULL PRIMARY KEY,
  node_id BLOB NOT NULL,
  name TEXT NOT NULL,
  state INT8 NOT NULL,
  error TEXT,
  input_data BLOB,
  input_data_compress_type INT8 NOT NULL DEFAULT 0,
  output_data BLOB,
  output_data_compress_type INT8 NOT NULL DEFAULT 0,
  http_response_id BLOB,
  completed_at BIGINT,
  FOREIGN KEY (node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);
`

func TestSubNodeInsert_WithoutBaseNode(t *testing.T) {
	// Setup DB - production schema doesn't have FK constraints on sub-node tables
	// so sub-nodes can be inserted before base nodes
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

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
		nrs:          &reqService,
		nfs:          &forService,
		nfes:         &forEachService,
		nifs:         ifService,
		njss:         &jsService,
		logger:       logger,
		builder:      builder,
		runningFlows: make(map[string]context.CancelFunc),
	}

	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// Setup user and workspace
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

	// Create a flow (needed for flow_node later)
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Test: Insert HTTP sub-node WITHOUT base node existing
	// This should succeed now that we removed ensureNodeAccess check
	nodeID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
		Items: []*flowv1.NodeHttpInsert{{
			NodeId: nodeID.Bytes(),
			HttpId: httpID.Bytes(),
		}},
	})

	_, err = svc.NodeHttpInsert(ctx, req)
	require.NoError(t, err, "NodeHttpInsert should succeed without base node")

	// Verify the sub-node was created
	nodeReq, err := reqService.GetNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, httpID, *nodeReq.HttpID)

	    // Now create the base node (simulating out-of-order arrival)
	    baseNode := mflow.Node{
	        ID:        nodeID,
	        FlowID:    flowID,
	        Name:      "Start",
	        NodeKind:  mflow.NODE_KIND_REQUEST,
	        PositionX: 0,
	        		PositionY: 0,
	        	}
	        	err = nodeService.CreateNode(ctx, baseNode)
	require.NoError(t, err, "Base node should be created after sub-node")

	t.Log("Successfully inserted sub-node before base node - decoupled insert works!")
}

func TestFlowRun_CreatesVersionOnEveryRun(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

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
		nrs:          &reqService,
		nfs:          &forService,
		nfes:         &forEachService,
		nifs:         ifService,
		njss:         &jsService,
		logger:       logger,
		builder:      builder,
		runningFlows: make(map[string]context.CancelFunc),
	}

	// Setup Data
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

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create Start Node (ManualStart)
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	// Run the flow multiple times
	runCount := 5
	for i := 0; i < runCount; i++ {
		req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
		_, err = svc.FlowRun(ctx, req)
		require.NoError(t, err, "Run %d failed", i)
	}

	// Wait for async execution to complete
	time.Sleep(500 * time.Millisecond)

	// Verify versions were created in DB
	versions, err := flowService.GetFlowsByVersionParentID(ctx, flowID)
	require.NoError(t, err)
	require.Equal(t, runCount, len(versions), "Should have %d versions in database (one per run)", runCount)

	// Verify each version has the correct parent
	for _, version := range versions {
		require.NotNil(t, version.VersionParentID, "Version should have VersionParentID set")
		assert.Equal(t, flowID, *version.VersionParentID, "Version should reference the parent flow")
		assert.Equal(t, workspaceID, version.WorkspaceID, "Version should be in the same workspace")
		assert.Equal(t, "Test Flow", version.Name, "Version should have the same name as parent")
	}

	t.Logf("Successfully created %d flow versions from %d runs", len(versions), runCount)
}

// TestFlowVersionNodes_HaveStateAndExecutions verifies that flow version nodes
// have correct state in NodeCollection and have execution records in NodeExecutionCollection
func TestFlowVersionNodes_HaveStateAndExecutions(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

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
		nrs:          &reqService,
		nfs:          &forService,
		nfes:         &forEachService,
		nifs:         ifService,
		njss:         &jsService,
		logger:       logger,
		builder:      builder,
		runningFlows: make(map[string]context.CancelFunc),
	}

	// Setup Data
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

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create Start Node (ManualStart)
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	// Run the flow once
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(ctx, req)
	require.NoError(t, err)

	// Wait for async execution to complete
	time.Sleep(200 * time.Millisecond)

	// Get the flow version
	versions, err := flowService.GetFlowsByVersionParentID(ctx, flowID)
	require.NoError(t, err)
	require.Len(t, versions, 1, "Should have 1 version")
	versionFlowID := versions[0].ID

	// Get version nodes
	versionNodes, err := nodeService.GetNodesByFlowID(ctx, versionFlowID)
	require.NoError(t, err)
	require.Len(t, versionNodes, 1, "Version should have 1 node")
	versionNodeID := versionNodes[0].ID

	// TEST 1: Verify NodeCollection returns version node with SUCCESS state
	collReq := connect.NewRequest(&emptypb.Empty{})
	var nodeCollResp *connect.Response[flowv1.NodeCollectionResponse]

	// Poll until we see SUCCESS state
	for i := 0; i < 20; i++ {
		nodeCollResp, err = svc.NodeCollection(ctx, collReq)
		require.NoError(t, err)

		// Find the version node
		var versionNode *flowv1.Node
		for _, item := range nodeCollResp.Msg.Items {
			if bytes.Equal(item.NodeId, versionNodeID.Bytes()) {
				versionNode = item
				break
			}
		}
		if versionNode != nil && versionNode.State == flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify version node has SUCCESS state
	var versionNode *flowv1.Node
	for _, item := range nodeCollResp.Msg.Items {
		if bytes.Equal(item.NodeId, versionNodeID.Bytes()) {
			versionNode = item
			break
		}
	}
	require.NotNil(t, versionNode, "Version node should be in NodeCollection")
	assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS, versionNode.State,
		"Version node should have SUCCESS state")

	// TEST 2: Verify NodeExecutionCollection has executions for version node
	execCollResp, err := svc.NodeExecutionCollection(ctx, collReq)
	require.NoError(t, err)

	// Find executions for the version node
	var versionNodeExecutions []*flowv1.NodeExecution
	for _, exec := range execCollResp.Msg.Items {
		if bytes.Equal(exec.NodeId, versionNodeID.Bytes()) {
			versionNodeExecutions = append(versionNodeExecutions, exec)
		}
	}
	require.NotEmpty(t, versionNodeExecutions, "Version node should have execution records")

	// Verify the execution has SUCCESS state
	latestExec := versionNodeExecutions[0]
	assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS, latestExec.State,
		"Version node execution should have SUCCESS state")

	t.Logf("Version node %s has state=%v with %d execution(s)",
		versionNodeID.String(), versionNode.State, len(versionNodeExecutions))
}
