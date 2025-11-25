package rflowv2

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

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
	"the-dev-tools/server/pkg/model/mnodeexecution"
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

// TestNodeCollection_StatePopulation verifies that NodeCollection populates State and Info from latest execution
func TestNodeCollection_StatePopulation(t *testing.T) {
	// Setup DB
	db, err := sql.Open("sqlite", ":memory:")
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

	svc := &FlowServiceV2RPC{
		ws:  &wsService,
		fs:  &flowService,
		ns:  &nodeService,
		nes: &nodeExecService,
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

	nodeID := idwrap.NewNow()
	node := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 200,
	}
	err = nodeService.CreateNode(ctx, node)
	require.NoError(t, err)

	// Create Node Execution
	execID := idwrap.NewNow()
	errStr := "Some error occurred"
	now := time.Now().Unix()
	exec := mnodeexecution.NodeExecution{
		ID:          execID,
		NodeID:      nodeID,
		Name:        "Test Node",
		State:       mnnode.NODE_STATE_FAILURE,
		Error:       &errStr,
		CompletedAt: &now,
	}
	err = nodeExecService.CreateNodeExecution(ctx, exec)
	require.NoError(t, err)

	// Call NodeCollection
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := svc.NodeCollection(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Assert
	require.Len(t, resp.Msg.Items, 1)
	item := resp.Msg.Items[0]
	assert.Equal(t, nodeID.Bytes(), item.NodeId)
	assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_FAILURE, item.State)
	require.NotNil(t, item.Info)
	assert.Equal(t, "Some error occurred", *item.Info)
}

// TestFlowRun_PersistsState verifies that FlowRun persists node execution state to the DB
func TestFlowRun_PersistsState(t *testing.T) {
	// Setup DB
	db, err := sql.Open("sqlite", ":memory:")
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

	svc := &FlowServiceV2RPC{
		ws:   &wsService,
		fs:   &flowService,
		ns:   &nodeService,
		nes:  &nodeExecService,
		es:   &edgeService,
		nnos: &noopService,
		fvs:  &flowVarService,
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

	// Call FlowRun
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(ctx, req)
	require.NoError(t, err)

	// Verify NodeCollection
	collReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := svc.NodeCollection(ctx, collReq)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Items, 1)
	item := resp.Msg.Items[0]
	
	if !bytes.Equal(item.NodeId, startNodeID.Bytes()) {
		t.Fatalf("Expected node ID %s, got %s", startNodeID.String(), idwrap.NewFromBytesMust(item.NodeId).String())
	}

	// Check if state is populated from FlowRun persistence
	// Start node should succeed
	assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS, item.State)
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