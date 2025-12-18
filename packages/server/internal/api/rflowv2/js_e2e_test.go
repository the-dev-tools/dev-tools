package rflowv2

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	"the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
)

// TestJSNodeExecution_E2E is an end-to-end test that verifies JS node execution
// by starting the worker-js service and running a flow with a JS node.
//
// Prerequisites:
// - worker-js must be built: pnpm nx run worker-js:build
// - Node.js must be available with --experimental-vm-modules support
func TestJSNodeExecution_E2E(t *testing.T) {
	// Skip if SKIP_E2E is set (useful for CI without worker-js)
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}

	// Find the worker-js bundle
	workerJSPath := findWorkerJSPath(t)
	if workerJSPath == "" {
		t.Skip("worker-js bundle not found - run 'pnpm nx run worker-js:build' first")
	}

	// Create temp directory for Unix socket
	socketDir := t.TempDir()
	socketPath := filepath.Join(socketDir, "worker-js.socket")
	t.Logf("Using socket path: %s", socketPath)

	// Start worker-js
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCmd := startWorkerJS(t, ctx, workerJSPath, socketPath)
	defer func() {
		if workerCmd.Process != nil {
			_ = workerCmd.Process.Kill()
		}
	}()

	// Wait for worker-js to be ready
	waitForWorkerJS(t, socketPath)

	// Create HTTP client that connects via Unix socket
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	// Create JS client (URL doesn't matter for Unix socket, but must be valid)
	jsClient := node_js_executorv1connect.NewNodeJsExecutorServiceClient(
		httpClient,
		"http://localhost",
	)

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup DB
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)
	nodeJsService := snodejs.New(queries)

	// HTTP services (for resolver)
	httpService := shttp.New(queries, logger)
	shttpHeaderSvc := shttp.NewHttpHeaderService(queries)
	shttpBodyRawSvc := shttp.NewHttpBodyRawService(queries)
	resSearchParamSvc := shttp.NewHttpSearchParamService(queries)
	resBodyFormSvc := shttp.NewHttpBodyFormService(queries)
	resBodyUrlencodedSvc := shttp.NewHttpBodyUrlEncodedService(queries)
	resAssertSvc := shttp.NewHttpAssertService(queries)
	httpResponseService := shttp.NewHttpResponseService(queries)

	// Node specific services
	nodeRequestService := snoderequest.New(queries)
	nodeForService := snodefor.New(queries)
	nodeForEachService := snodeforeach.New(queries)
	nodeIfService := snodeif.New(queries)

	// Environment and variable services
	envService := senv.New(queries, logger)
	varService := svar.New(queries, logger)

	// Resolver
	res := resolver.NewStandardResolver(
		&httpService,
		&shttpHeaderSvc,
		resSearchParamSvc,
		shttpBodyRawSvc,
		resBodyFormSvc,
		resBodyUrlencodedSvc,
		resAssertSvc,
	)

	svc := New(
		db,
		wsService.Reader(),
		flowService.Reader(),
		nodeService.Reader(),
		varService.Reader(),
		httpService.Reader(),
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
		&envService,
		&varService,
		&httpService,
		shttpBodyRawSvc,
		res,
		logger,
		nil, // workspaceImportService
		httpResponseService,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // streams
		jsClient, // JS client!
	)

	// Setup Data
	userID := idwrap.NewNow()
	testCtx := mwauth.CreateAuthedContext(ctx, userID)

	// User
	err = queries.CreateUser(testCtx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	// Workspace
	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: dbtime.DBNow(),
	}
	err = wsService.Create(testCtx, &workspace)
	require.NoError(t, err)

	// Workspace User
	err = queries.CreateWorkspaceUser(testCtx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	// Flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "JS Test Flow",
	}
	err = flowService.CreateFlow(testCtx, flow)
	require.NoError(t, err)

	// --- Create Flow Nodes ---

	// Start Node (NoOp)
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mflow.NODE_KIND_NO_OP,
	}
	err = nodeService.CreateNode(testCtx, startNode)
	require.NoError(t, err)
	err = noopService.CreateNodeNoop(testCtx, mflow.NodeNoop{
		FlowNodeID: startNodeID,
		Type:       mflow.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// JS Node
	jsNodeID := idwrap.NewNow()
	jsNode := mflow.Node{
		ID:        jsNodeID,
		FlowID:    flowID,
		Name:      "JS Node",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 100,
		PositionY: 0,
	}
	err = nodeService.CreateNode(testCtx, jsNode)
	require.NoError(t, err)

	// JS Code that returns a value
	// The code must export a default function or value
	jsCode := `export default function(ctx) { return { result: "hello from js", computed: 42 }; }`
	err = nodeJsService.CreateNodeJS(testCtx, mflow.NodeJS{
		FlowNodeID: jsNodeID,
		Code:       []byte(jsCode),
	})
	require.NoError(t, err)

	// --- Connect Edges ---

	// Edge: Start -> JS (no further edges - flow ends after JS node)
	edgeID := idwrap.NewNow()
	err = edgeService.CreateEdge(testCtx, edge.Edge{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      jsNodeID,
		SourceHandler: edge.HandleUnspecified,
	})
	require.NoError(t, err)

	// --- Execute Flow ---
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(testCtx, req)
	require.NoError(t, err)

	// --- Verify Execution ---
	// Poll for JS node execution to complete
	var exec *mflow.NodeExecution
	for i := 0; i < 30; i++ {
		exec, err = nodeExecService.GetLatestNodeExecutionByNodeID(testCtx, jsNodeID)
		if err == nil && exec != nil && mflow.NodeState(exec.State) == mflow.NODE_STATE_SUCCESS {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	require.NoError(t, err)
	require.NotNil(t, exec, "JS node execution not found")
	errMsg := ""
	if exec.Error != nil {
		errMsg = *exec.Error
	}
	require.Equal(t, mflow.NODE_STATE_SUCCESS, mflow.NodeState(exec.State),
		"JS node should have SUCCESS state, got: %v, error: %s", exec.State, errMsg)

	t.Log("✅ JS node executed successfully via worker-js!")
}

// findWorkerJSPath finds the compiled worker-js bundle
func findWorkerJSPath(t *testing.T) string {
	t.Helper()

	// Try relative paths from the test directory
	paths := []string{
		"../../../../../packages/worker-js/dist/main.cjs",
		"../../../../../../packages/worker-js/dist/main.cjs",
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err == nil {
		t.Logf("Current working directory: %s", cwd)
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			t.Logf("Found worker-js at: %s", p)
			return p
		}
	}

	// Try absolute path from monorepo root
	monorepoRoot := os.Getenv("MONOREPO_ROOT")
	if monorepoRoot != "" {
		absPath := fmt.Sprintf("%s/packages/worker-js/dist/main.cjs", monorepoRoot)
		if _, err := os.Stat(absPath); err == nil {
			t.Logf("Found worker-js at: %s", absPath)
			return absPath
		}
	}

	return ""
}

// startWorkerJS starts the worker-js process with Unix socket mode
func startWorkerJS(t *testing.T, ctx context.Context, bundlePath string, socketPath string) *exec.Cmd {
	t.Helper()

	cmd := exec.CommandContext(ctx, "node",
		"--experimental-vm-modules",
		"--disable-warning=ExperimentalWarning",
		bundlePath,
	)
	// Use UDS mode (default) with custom socket path
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("WORKER_SOCKET_PATH=%s", socketPath),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	require.NoError(t, err, "Failed to start worker-js")

	t.Logf("Started worker-js with PID: %d", cmd.Process.Pid)
	return cmd
}

// waitForWorkerJS waits for the worker-js Unix socket to be ready
func waitForWorkerJS(t *testing.T, socketPath string) {
	t.Helper()

	for i := 0; i < 50; i++ {
		// Try to connect to the Unix socket
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			t.Log("worker-js is ready (Unix socket)")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal("worker-js did not start in time")
}
