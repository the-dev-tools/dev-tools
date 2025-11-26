package rflowv2

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestFlowRun_DeltaOverride(t *testing.T) {
	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 1. Setup Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Method (Delta should override Base GET with POST)
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		// Verify Header (Delta should override Base header)
		if val := r.Header.Get("X-Test"); val != "Delta" {
			t.Errorf("Expected X-Test header 'Delta', got '%s'", val)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// 2. Setup DB
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1) // Ensure single connection for in-memory DB persistence
	defer db.Close()

	statements := strings.Split(deltaTestSchema, ";")
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		_, err = db.Exec(trimmed)
		require.NoError(t, err, "Failed to execute schema statement: %s", trimmed)
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	queries := gen.New(db)

	// 3. Setup Services
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)

	// shttp services (Used by FlowServiceV2RPC and for Data Creation)
	httpService := shttp.New(queries, logger)
	shttpHeaderSvc := shttp.NewHttpHeaderService(queries)
	shttpSearchParamSvc := shttp.NewHttpSearchParamService(queries)
	shttpBodyFormSvc := shttp.NewHttpBodyFormService(queries) // Returns Struct
	shttpBodyUrlencodedSvc := shttp.NewHttpBodyUrlencodedService(queries)
	shttpAssertSvc := shttp.NewHttpAssertService(queries)   // Returns Struct
	shttpBodyRawSvc := shttp.NewHttpBodyRawService(queries) // Shared

	// Independent services (Used by StandardResolver)
	resHeaderSvc := shttpheader.New(queries)
	resSearchParamSvc := shttpsearchparam.New(queries)
	resBodyFormSvc := shttpbodyform.New(queries)
	resBodyUrlencodedSvc := shttpbodyurlencoded.New(queries)
	resAssertSvc := shttpassert.New(queries)

	nodeRequestService := snoderequest.New(queries)

	// Node specific services needed for FlowServiceV2RPC
	nodeForService := snodefor.New(queries)
	nodeForEachService := snodeforeach.New(queries)
	nodeIfService := snodeif.New(queries) // Returns *NodeIfService
	nodeJsService := snodejs.New(queries)

	// Response services
	httpResponseService := shttp.NewHttpResponseService(queries)

	// Resolver
	res := resolver.NewStandardResolver(
		&httpService,
		&resHeaderSvc,
		&resSearchParamSvc,
		shttpBodyRawSvc,
		&resBodyFormSvc,
		&resBodyUrlencodedSvc,
		&resAssertSvc,
	)

	svc := New(
		&wsService,
		&flowService,
		&edgeService,
		&nodeService,
		&nodeRequestService,
		&nodeForService,
		&nodeForEachService,
		nodeIfService,
		&noopService,
		&nodeJsService,
		&nodeExecService,
		&flowVarService,
		&httpService,
		shttpHeaderSvc,
		shttpSearchParamSvc,
		&shttpBodyFormSvc,
		shttpBodyUrlencodedSvc,
		&shttpAssertSvc,
		shttpBodyRawSvc,
		res,
		logger,
		nil, // workspaceImportService
		httpResponseService,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // streams
	)

	// 4. Setup Data
	ctx := context.Background()
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// User
	_, err = db.Exec("INSERT INTO users (id, email) VALUES (?, ?)", userID.Bytes(), "test@example.com")
	require.NoError(t, err)

	// Workspace
	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: dbtime.DBNow(),
	}
	err = wsService.Create(ctx, &workspace)
	require.NoError(t, err)

	// Workspace User
	_, err = db.Exec("INSERT INTO workspaces_users (id, workspace_id, user_id, role) VALUES (?, ?, ?, ?)",
		idwrap.NewNow().Bytes(), workspaceID.Bytes(), userID.Bytes(), 1)
	require.NoError(t, err)

	// Flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Delta Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// --- HTTP Data ---

	// Base Request
	baseID := idwrap.NewNow()
	baseReq := mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Method:      "GET",
		Url:         ts.URL,
		BodyKind:    mhttp.HttpBodyKindNone,
	}
	err = httpService.Create(ctx, &baseReq)
	require.NoError(t, err)

	// Base Header
	baseHeaderID := idwrap.NewNow()
	baseHeader := mhttp.HTTPHeader{
		ID:          baseHeaderID,
		HttpID:      baseID,
		HeaderKey:   "X-Test",
		HeaderValue: "Base",
		Enabled:     true,
	}
	// Use shttpHeaderSvc to create data (it accepts mhttp models)
	_, err = shttpHeaderSvc.Create(ctx, baseHeader)
	require.NoError(t, err)

	// Delta Request
	deltaID := idwrap.NewNow()
	deltaReq := mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  workspaceID,
		Name:         "Delta Request",
		Method:       "POST",
		ParentHttpID: &baseID,
		IsDelta:      true,
		DeltaMethod:  func() *string { s := "POST"; return &s }(),
	}

	err = httpService.Create(ctx, &deltaReq)
	require.NoError(t, err)

	// Delta Header (Override)
	deltaHeaderID := idwrap.NewNow()
	deltaHeader := mhttp.HTTPHeader{
		ID:               deltaHeaderID,
		HttpID:           deltaID,
		HeaderKey:        "X-Test",
		ParentHeaderID:   &baseHeaderID,
		IsDelta:          true,
		DeltaHeaderValue: func() *string { s := "Delta"; return &s }(),
		DeltaEnabled:     func() *bool { b := true; return &b }(),
	}
	_, err = shttpHeaderSvc.Create(ctx, deltaHeader)
	require.NoError(t, err)

	// --- Flow Nodes ---

	// Start Node
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)
	err = noopService.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// HTTP Request Node
	requestNodeID := idwrap.NewNow()
	requestNode := mnnode.MNode{
		ID:        requestNodeID,
		FlowID:    flowID,
		Name:      "Delta Request Node",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, requestNode)
	require.NoError(t, err)

	// Link Node to HTTP (with Delta)
	err = nodeRequestService.CreateNodeRequest(ctx, mnrequest.MNRequest{
		FlowNodeID:     requestNodeID,
		HttpID:         baseID,
		DeltaExampleID: &deltaID, // Mapped to delta_http_id in DB
	})
	require.NoError(t, err)

	// Edge: Start -> Request
	edgeID := idwrap.NewNow()
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      requestNodeID,
		SourceHandler: edge.HandleUnspecified,
	})
	require.NoError(t, err)

	// 5. Execution
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(ctx, req)
	require.NoError(t, err)

	// 6. Verification
	// Check Node Execution
	exec, err := nodeExecService.GetLatestNodeExecutionByNodeID(ctx, requestNodeID)
	require.NoError(t, err)
	require.NotNil(t, exec, "Node execution not found for node %s", requestNodeID.String())
	assert.Equal(t, mnnode.NODE_STATE_SUCCESS, mnnode.NodeState(exec.State))
}

// Schema with Delta support
const deltaTestSchema = `
PRAGMA foreign_keys = OFF;

CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  status INT8 NOT NULL DEFAULT 0,
  UNIQUE (provider_type, provider_id)
);

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

CREATE TABLE flow_node (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  name TEXT NOT NULL,
  node_kind INT NOT NULL,
  position_x REAL NOT NULL,
  position_y REAL NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
);

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

CREATE TABLE flow_node_noop (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  node_type TINYINT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

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

CREATE TABLE http (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  folder_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method TEXT NOT NULL,
  body_kind INT8 NOT NULL DEFAULT 2,
  description TEXT NOT NULL DEFAULT '',
  
  parent_http_id BLOB DEFAULT NULL,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  delta_name TEXT NULL,
  delta_url TEXT NULL,
  delta_method TEXT NULL,
  delta_body_kind INT8 NULL,
  delta_description TEXT NULL,

  last_run_at BIGINT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL,
  FOREIGN KEY (parent_http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_header (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  header_key TEXT NOT NULL,
  header_value TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  
  parent_header_id BLOB DEFAULT NULL,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  delta_header_key TEXT NULL,
  delta_header_value TEXT NULL,
  delta_description TEXT NULL,
  delta_enabled BOOLEAN NULL,
  
  prev BLOB DEFAULT NULL,
  next BLOB DEFAULT NULL,
  
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_header_id) REFERENCES http_header (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_http (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  delta_http_id BLOB,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_http_id) REFERENCES http (id) ON DELETE SET NULL
);

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

CREATE TABLE http_search_param (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_search_param_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_body_form (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_body_form_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_body_urlencoded (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_body_urlencoded_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_body_raw (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  raw_data BLOB,
  content_type TEXT NOT NULL DEFAULT '',
  compression_type INT8 NOT NULL DEFAULT 0,
  parent_body_raw_id BLOB DEFAULT NULL,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_raw_data BLOB NULL,
  delta_content_type TEXT NULL,
  delta_compression_type INT8 NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_assert (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_assert_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_for (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_count BIGINT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_for_each (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_expression TEXT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_condition (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  expression TEXT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_js (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  code BLOB NOT NULL,
  code_compress_type INT8 NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE http_response (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  status INT32 NOT NULL,
  body BLOB,
  time DATETIME NOT NULL,
  duration INT32 NOT NULL,
  size INT32 NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_response_header (
  id BLOB NOT NULL PRIMARY KEY,
  response_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (response_id) REFERENCES http_response (id) ON DELETE CASCADE
);

CREATE TABLE http_response_assert (
  id BLOB NOT NULL PRIMARY KEY,
  response_id BLOB NOT NULL,
  value TEXT NOT NULL,
  success BOOLEAN NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (response_id) REFERENCES http_response (id) ON DELETE CASCADE
);
`
