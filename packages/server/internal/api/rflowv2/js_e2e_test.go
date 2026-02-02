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

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
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
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	nodeNodeJsService := sflow.NewNodeJsService(queries)

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
	nodeRequestService := sflow.NewNodeRequestService(queries)
	nodeForService := sflow.NewNodeForService(queries)
	nodeForEachService := sflow.NewNodeForEachService(queries)
	nodeIfService := sflow.NewNodeIfService(queries)
	nodeAIService := sflow.NewNodeAIService(queries)
	nodeAiProviderService := sflow.NewNodeAiProviderService(queries)
	nodeMemoryService := sflow.NewNodeMemoryService(queries)

	// Environment and variable services
	envService := senv.NewEnvironmentService(queries, logger)
	varService := senv.NewVariableService(queries, logger)

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

	svc := New(FlowServiceV2Deps{
		DB: db,
		Readers: FlowServiceV2Readers{
			Workspace: wsService.Reader(),
			Flow:      flowService.Reader(),
			Node:      nodeService.Reader(),
			Env:       envService.Reader(),
			Http:      httpService.Reader(),
			Edge:      edgeService.Reader(),
		},
		Services: FlowServiceV2Services{
			Workspace:     &wsService,
			Flow:          &flowService,
			Edge:          &edgeService,
			Node:          &nodeService,
			NodeRequest:   &nodeRequestService,
			NodeFor:       &nodeForService,
			NodeForEach:   &nodeForEachService,
			NodeIf:        nodeIfService,
			NodeJs:        &nodeNodeJsService,
			NodeAI:        &nodeAIService,
			NodeAiProvider:     &nodeAiProviderService,
			NodeMemory:    &nodeMemoryService,
			NodeExecution: &nodeExecService,
			FlowVariable:  &flowVarService,
			Env:           &envService,
			Var:           &varService,
			Http:          &httpService,
			HttpBodyRaw:   shttpBodyRawSvc,
			HttpResponse:  httpResponseService,
		},
		Resolver: res,
		Logger:   logger,
		JsClient: jsClient,
	})

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

	// Start Node
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mflow.NODE_KIND_MANUAL_START,
	}
	err = nodeService.CreateNode(testCtx, startNode)
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
	err = nodeNodeJsService.CreateNodeJS(testCtx, mflow.NodeJS{
		FlowNodeID: jsNodeID,
		Code:       []byte(jsCode),
	})
	require.NoError(t, err)

	// --- Connect Edges ---

	// Edge: Start -> JS (no further edges - flow ends after JS node)
	edgeID := idwrap.NewNow()
	err = edgeService.CreateEdge(testCtx, mflow.Edge{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      jsNodeID,
		SourceHandler: mflow.HandleUnspecified,
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
