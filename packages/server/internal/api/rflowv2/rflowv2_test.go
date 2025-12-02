package rflowv2

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"log/slog"
	"os"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	_ "modernc.org/sqlite"

	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestFlowRun_MultipleRuns(t *testing.T) {
	// Setup DB
	// Use shared cache for in-memory DB to support concurrency
	db, err := sql.Open("sqlite", "file:multiple_runs_test?mode=memory&cache=shared")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(schema)
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		ws:           &wsService,
		fs:           &flowService,
		ns:           &nodeService,
		nes:          &nodeExecService,
		es:           &edgeService,
		nnos:         &noopService,
		fvs:          &flowVarService,
		logger:       logger,
		runningFlows: make(map[string]context.CancelFunc),
	}

	// Setup Data
	ctx := context.Background()
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	_, err = db.Exec("INSERT INTO users (id, email) VALUES (?, ?)", userID.Bytes(), "test@example.com")
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

	_, err = db.Exec("INSERT INTO workspaces_users (id, workspace_id, user_id, role) VALUES (?, ?, ?, ?)",
		idwrap.NewNow().Bytes(), workspaceID.Bytes(), userID.Bytes(), 1)
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create Start Node (NoOp)
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	err = noopService.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
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
  prev BLOB,
  next BLOB,
  FOREIGN KEY (prev) REFERENCES workspaces (id) ON DELETE SET NULL,
  FOREIGN KEY (next) REFERENCES workspaces (id) ON DELETE SET NULL
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
  edge_kind INT NOT NULL DEFAULT 0,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE,
  FOREIGN KEY (source_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (target_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

-- FLOW NODE NOOP
CREATE TABLE flow_node_noop (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  node_type TINYINT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
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

// schemaNoFK is the same as schema but without FK constraints on flow_node sub-tables
// This allows testing decoupled inserts where sub-nodes can be inserted before base nodes
const schemaNoFK = `
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
  prev BLOB,
  next BLOB,
  FOREIGN KEY (prev) REFERENCES workspaces (id) ON DELETE SET NULL,
  FOREIGN KEY (next) REFERENCES workspaces (id) ON DELETE SET NULL
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

-- FLOW NODE NOOP (no FK to flow_node)
CREATE TABLE flow_node_noop (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  node_type TINYINT NOT NULL
);

-- FLOW NODE FOR (no FK to flow_node)
CREATE TABLE flow_node_for (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_count BIGINT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL
);

-- FLOW NODE FOR EACH (no FK to flow_node)
CREATE TABLE flow_node_for_each (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_expression TEXT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL
);

-- FLOW NODE CONDITION (no FK to flow_node)
CREATE TABLE flow_node_condition (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  expression TEXT NOT NULL
);

-- FLOW NODE JS (no FK to flow_node)
CREATE TABLE flow_node_js (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  code BLOB NOT NULL,
  code_compress_type INT8 NOT NULL
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

-- NODE EXECUTION (no FK to flow_node)
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
  completed_at BIGINT
);
`

func TestSubNodeInsert_WithoutBaseNode(t *testing.T) {
	// Setup DB with schema that has no FK constraints on sub-node tables
	db, err := sql.Open("sqlite", "file:subnode_insert_test?mode=memory&cache=shared")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(schemaNoFK)
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		ws:           &wsService,
		fs:           &flowService,
		ns:           &nodeService,
		nes:          &nodeExecService,
		es:           &edgeService,
		nnos:         &noopService,
		fvs:          &flowVarService,
		logger:       logger,
		runningFlows: make(map[string]context.CancelFunc),
	}

	ctx := context.Background()
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// Setup user and workspace
	_, err = db.Exec("INSERT INTO users (id, email) VALUES (?, ?)", userID.Bytes(), "test@example.com")
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

	_, err = db.Exec("INSERT INTO workspaces_users (id, workspace_id, user_id, role) VALUES (?, ?, ?, ?)",
		idwrap.NewNow().Bytes(), workspaceID.Bytes(), userID.Bytes(), 1)
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

	// Test: Insert NoOp sub-node WITHOUT base node existing
	// This should succeed now that we removed ensureNodeAccess check
	nodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeNoOpInsertRequest{
		Items: []*flowv1.NodeNoOpInsert{{
			NodeId: nodeID.Bytes(),
			Kind:   flowv1.NodeNoOpKind_NODE_NO_OP_KIND_START,
		}},
	})

	_, err = svc.NodeNoOpInsert(ctx, req)
	require.NoError(t, err, "NodeNoOpInsert should succeed without base node")

	// Verify the sub-node was created
	noopNode, err := noopService.GetNodeNoop(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, mnnoop.NODE_NO_OP_KIND_START, noopNode.Type)

	// Now create the base node (simulating out-of-order arrival)
	baseNode := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, baseNode)
	require.NoError(t, err, "Base node should be created after sub-node")

	t.Log("Successfully inserted sub-node before base node - decoupled insert works!")
}

func TestFlowRun_CreatesVersionOnEveryRun(t *testing.T) {
	// Setup DB
	db, err := sql.Open("sqlite", "file:flow_version_test?mode=memory&cache=shared")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(schema)
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := &FlowServiceV2RPC{
		ws:           &wsService,
		fs:           &flowService,
		ns:           &nodeService,
		nes:          &nodeExecService,
		es:           &edgeService,
		nnos:         &noopService,
		fvs:          &flowVarService,
		logger:       logger,
		runningFlows: make(map[string]context.CancelFunc),
	}

	// Setup Data
	ctx := context.Background()
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	_, err = db.Exec("INSERT INTO users (id, email) VALUES (?, ?)", userID.Bytes(), "test@example.com")
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

	_, err = db.Exec("INSERT INTO workspaces_users (id, workspace_id, user_id, role) VALUES (?, ?, ?, ?)",
		idwrap.NewNow().Bytes(), workspaceID.Bytes(), userID.Bytes(), 1)
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create Start Node (NoOp)
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	err = noopService.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
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
