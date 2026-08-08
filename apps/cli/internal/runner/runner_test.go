package runner_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/common"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/logconsole"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/swebsocket"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/private/node_js_executor/v1/node_js_executorv1connect"
)

// flowTestFixture provides a common test environment for flow execution tests
type flowTestFixture struct {
	t           *testing.T
	ctx         context.Context
	db          *sql.DB
	queries     *gen.Queries
	services    *common.Services
	builder     *flowbuilder.Builder
	mockServer  *httptest.Server
	workspaceID idwrap.IDWrap
	cleanup     func()
}

// newFlowTestFixture creates a new test fixture with in-memory database and mock HTTP server
func newFlowTestFixture(t *testing.T) *flowTestFixture {
	t.Helper()

	ctx := context.Background()

	// Create in-memory SQLite database
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create in-memory database: %v", err)
	}

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		cleanup()
		t.Fatalf("failed to prepare queries: %v", err)
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Initialize all services
	workspaceService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeRequestService := sflow.NewNodeRequestService(queries)
	nodeIfService := sflow.NewNodeIfService(queries)
	flowVariableService := sflow.NewFlowVariableService(queries)
	nodeForService := sflow.NewNodeForService(queries)
	nodeForEachService := sflow.NewNodeForEachService(queries)
	nodeJSService := sflow.NewNodeJsService(queries)
	edgeService := sflow.NewEdgeService(queries)

	// V2 HTTP services
	httpService := shttp.New(queries, logger)
	httpHeaderService := shttp.NewHttpHeaderService(queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(queries)
	httpBodyRawService := shttp.NewHttpBodyRawService(queries)
	httpAssertService := shttp.NewHttpAssertService(queries)

	// WebSocket services
	nodeWsConnectionService := sflow.NewNodeWsConnectionService(queries)
	nodeWsSendService := sflow.NewNodeWsSendService(queries)
	nodeWaitService := sflow.NewNodeWaitService(queries)
	webSocketService := swebsocket.New(queries, logger)
	webSocketHeaderService := swebsocket.NewWebSocketHeaderService(queries)

	// Additional services for builder
	varService := senv.NewVariableService(queries, logger)

	// Initialize resolver
	res := resolver.NewStandardResolver(
		&httpService,
		&httpHeaderService,
		httpSearchParamService,
		httpBodyRawService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
	)

	// Initialize builder
	builder := flowbuilder.New(
		&nodeService,
		&nodeRequestService,
		&nodeForService,
		&nodeForEachService,
		nodeIfService,
		&nodeJSService,
		nil, // NodeAIService - not needed for CLI tests
		nil, // NodeAiProviderService - not needed for CLI tests
		nil, // NodeMemoryService - not needed for CLI tests
		nil, // NodeGraphQLService - not needed for CLI tests
		&nodeWsConnectionService,
		&nodeWsSendService,
		&nodeWaitService,
		nil, // NodeSubFlowTriggerService - not needed for CLI tests
		nil, // NodeSubFlowReturnService - not needed for CLI tests
		nil, // NodeRunSubFlowService - not needed for CLI tests
		&webSocketService,
		&webSocketHeaderService,
		nil, // GraphQLService - not needed for CLI tests
		nil, // GraphQLHeaderService - not needed for CLI tests
		&workspaceService,
		&varService,
		&flowVariableService,
		res,
		nil, // GraphQLResolver - not needed for CLI tests
		logger,
		nil, // LLMProviderFactory - not needed for CLI tests
	)

	_ = logconsole.NewLogChanMap() // unused but kept for parity if needed

	services := &common.Services{
		DB:                 db,
		Queries:            queries,
		Workspace:          workspaceService,
		Environment:        senv.NewEnvironmentService(queries, logger),
		Variable:           varService,
		Flow:               flowService,
		FlowEdge:           edgeService,
		FlowVariable:       flowVariableService,
		Node:               nodeService,
		NodeRequest:        nodeRequestService,
		NodeFor:            nodeForService,
		NodeForEach:        nodeForEachService,
		NodeIf:             *nodeIfService,
		NodeJS:             nodeJSService,
		HTTP:               httpService,
		HTTPHeader:         httpHeaderService,
		HTTPSearchParam:    httpSearchParamService,
		HTTPBodyForm:       httpBodyFormService,
		HTTPBodyUrlEncoded: httpBodyUrlEncodedService,
		HTTPBodyRaw:        httpBodyRawService,
		HTTPAssert:         httpAssertService,
		NodeWsConnection:   nodeWsConnectionService,
		NodeWsSend:         nodeWsSendService,
		NodeWait:           nodeWaitService,
		WebSocket:          webSocketService,
		WebSocketHeader:    webSocketHeaderService,
		Logger:             logger,
	}

	// Create mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default response for all requests
		response := map[string]interface{}{
			"status": "ok",
			"method": r.Method,
			"path":   r.URL.Path,
			"query":  r.URL.RawQuery,
		}

		// Handle specific paths for different test scenarios
		switch {
		case strings.Contains(r.URL.Path, "/users"):
			response["name"] = "Test User"
			response["id"] = 1
		case strings.Contains(r.URL.Path, "/posts"):
			response["title"] = "Test Post"
			response["body"] = "Test Body"
			response["userId"] = 1
			response["id"] = 1
		case strings.Contains(r.URL.Path, "/todos"):
			response["title"] = "Test Todo"
			response["completed"] = false
			response["userId"] = 1
			response["id"] = 1
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))

	workspaceID := idwrap.NewNow()

	fixture := &flowTestFixture{
		t:           t,
		ctx:         ctx,
		db:          db,
		queries:     queries,
		services:    services,
		builder:     builder,
		mockServer:  mockServer,
		workspaceID: workspaceID,
		cleanup: func() {
			mockServer.Close()
			cleanup()
		},
	}

	t.Cleanup(fixture.cleanup)

	return fixture
}

func (f *flowTestFixture) getRunnerServices(jsClient node_js_executorv1connect.NodeJsExecutorServiceClient) runner.RunnerServices {
	return runner.RunnerServices{
		NodeService:         f.services.Node,
		EdgeService:         f.services.FlowEdge,
		FlowVariableService: f.services.FlowVariable,
		Builder:             f.builder,
		JSClient:            jsClient,
	}
}

// importWorkspaceBundle imports a workspace bundle into the database
func (f *flowTestFixture) importWorkspaceBundle(bundle *ioworkspace.WorkspaceBundle) {
	f.t.Helper()

	ios := ioworkspace.New(f.queries, f.services.Logger)
	opts := ioworkspace.GetDefaultImportOptions(f.workspaceID)
	opts.PreserveIDs = true

	tx, err := f.db.BeginTx(f.ctx, nil)
	if err != nil {
		f.t.Fatalf("failed to begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := ios.Import(f.ctx, tx, bundle, opts); err != nil {
		f.t.Fatalf("failed to import bundle: %v", err)
	}

	if err := tx.Commit(); err != nil {
		f.t.Fatalf("failed to commit transaction: %v", err)
	}
}

// getFlowByName retrieves a flow by name from the workspace
func (f *flowTestFixture) getFlowByName(name string) *mflow.Flow {
	f.t.Helper()

	flows, err := f.services.Flow.GetFlowsByWorkspaceID(f.ctx, f.workspaceID)
	if err != nil {
		f.t.Fatalf("failed to get flows: %v", err)
	}

	for _, flow := range flows {
		if flow.Name == name {
			return &flow
		}
	}

	return nil
}

// TestFlowRun_SimpleYAML tests loading and executing a simple YAML flow
func TestFlowRun_SimpleYAML(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	// Create a simple flow without JS nodes for testing
	yamlContent := fmt.Sprintf(`workspace_name: Simple Test
flows:
  - name: SimpleFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: Request1
          method: GET
          url: %s/test
          depends_on: Start
`, fixture.mockServer.URL)

	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlContent), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	// Import the flow data
	fixture.importWorkspaceBundle(resolved)

	// Execute the flow
	flow := fixture.getFlowByName("SimpleFlow")
	if flow == nil {
		t.Fatal("SimpleFlow not found")
	}

	// Run the flow with a timeout context
	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	result, err := runner.RunFlow(ctx, flow, fixture.getRunnerServices(nil), nil)

	// Verify execution
	if err != nil {
		t.Errorf("flow execution failed: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("expected status 'success', got '%s'. Error: %s", result.Status, result.Error)
	}

	if result.FlowName != "SimpleFlow" {
		t.Errorf("expected flow name 'SimpleFlow', got '%s'", result.FlowName)
	}

	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}

	// Verify nodes were executed
	if len(result.Nodes) == 0 {
		t.Error("expected nodes to be executed")
	}
}

// TestFlowRun_MultiFlow tests executing multiple flows with dependencies
func TestFlowRun_MultiFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	// Create multiple simple flows without JS nodes
	yamlContent := fmt.Sprintf(`workspace_name: Multi Flow Test
run:
  - flow: FlowA
  - flow: FlowB
    depends_on: FlowA
flows:
  - name: FlowA
    steps:
      - manual_start:
          name: StartA
      - request:
          name: RequestA
          method: GET
          url: %s/users/1
          depends_on: StartA
  - name: FlowB
    steps:
      - manual_start:
          name: StartB
      - request:
          name: RequestB
          method: GET
          url: %s/posts/1
          depends_on: StartB
`, fixture.mockServer.URL, fixture.mockServer.URL)

	fileData := []byte(yamlContent)
	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(fileData, yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	// Import the flow data
	fixture.importWorkspaceBundle(resolved)

	// Get all flows
	flows, err := fixture.services.Flow.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("failed to get flows: %v", err)
	}

	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}

	// Run multiple flows
	err = runner.RunMultipleFlows(fixture.ctx, fileData, flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)

	if err != nil {
		t.Errorf("multi-flow execution failed: %v", err)
	}
}

// TestFlowRun_RequestNode tests a flow with HTTP request nodes
func TestFlowRun_RequestNode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	// Track requests received by mock server
	requestCount := 0
	fixture.mockServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		response := map[string]interface{}{
			"status":       "ok",
			"method":       r.Method,
			"path":         r.URL.Path,
			"requestCount": requestCount,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	// Create a flow with HTTP request
	yamlContent := fmt.Sprintf(`workspace_name: Request Test
flows:
  - name: RequestFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: Req1
          method: GET
          url: %s/api/test
          depends_on: Start
`, fixture.mockServer.URL)

	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlContent), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	fixture.importWorkspaceBundle(resolved)

	flow := fixture.getFlowByName("RequestFlow")
	if flow == nil {
		t.Fatal("RequestFlow not found")
	}

	// Run flow
	result, err := runner.RunFlow(fixture.ctx, flow, fixture.getRunnerServices(nil), nil)

	// Verify execution
	if err != nil {
		t.Errorf("flow execution failed: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("expected status 'success', got '%s'. Error: %s", result.Status, result.Error)
	}

	// Verify HTTP requests were made
	if requestCount == 0 {
		t.Error("expected at least one HTTP request to mock server")
	}
}

// TestFlowRun_FlowNotFound tests error when requesting non-existent flow
func TestFlowRun_FlowNotFound(t *testing.T) {
	fixture := newFlowTestFixture(t)

	// Create a simple flow
	yamlContent := fmt.Sprintf(`workspace_name: Test
flows:
  - name: ExistingFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: Req1
          method: GET
          url: %s/test
          depends_on: Start
`, fixture.mockServer.URL)

	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlContent), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	fixture.importWorkspaceBundle(resolved)

	// Try to get a non-existent flow
	nonExistentFlow := fixture.getFlowByName("NonExistentFlow")

	if nonExistentFlow != nil {
		t.Error("expected nil for non-existent flow, but got a flow")
	}

	// Verify the workspace has flows (sanity check)
	flows, err := fixture.services.Flow.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("failed to get flows: %v", err)
	}

	if len(flows) == 0 {
		t.Fatal("expected flows to exist in workspace")
	}
}

// TestFlowRun_HTTPMethods tests different HTTP methods
func TestFlowRun_HTTPMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	// Track methods received
	methods := make(map[string]int)
	fixture.mockServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods[r.Method]++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"method": r.Method,
			"status": "ok",
		})
	})

	// Create flows with different HTTP methods
	yamlContent := fmt.Sprintf(`workspace_name: HTTP Methods Test
flows:
  - name: HTTPMethodsFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: GetRequest
          method: GET
          url: %s/test
          depends_on: Start
      - request:
          name: PostRequest
          method: POST
          url: %s/test
          depends_on: GetRequest
      - request:
          name: PutRequest
          method: PUT
          url: %s/test
          depends_on: PostRequest
`, fixture.mockServer.URL, fixture.mockServer.URL, fixture.mockServer.URL)

	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlContent), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	fixture.importWorkspaceBundle(resolved)

	// Get and run the flow
	flow := fixture.getFlowByName("HTTPMethodsFlow")
	if flow == nil {
		t.Fatal("HTTPMethodsFlow not found")
	}

	result, err := runner.RunFlow(fixture.ctx, flow, fixture.getRunnerServices(nil), nil)

	if err != nil {
		t.Errorf("flow execution failed: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("expected status 'success', got '%s'. Error: %s", result.Status, result.Error)
	}

	// Verify different methods were called
	if methods["GET"] == 0 {
		t.Error("expected GET request")
	}
	if methods["POST"] == 0 {
		t.Error("expected POST request")
	}
	if methods["PUT"] == 0 {
		t.Error("expected PUT request")
	}
}

// TestFlowRun_JSNode tests that the JS node infrastructure is properly wired up.
func TestFlowRun_JSNode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if Node.js is available
	jsRunner, err := runner.NewJSRunner()
	if err != nil {
		t.Skipf("skipping JS node test: %v", err)
	}
	defer jsRunner.Stop()

	fixture := newFlowTestFixture(t)

	// Create a flow with a JS node
	yamlContent := `workspace_name: JS Test
flows:
  - name: JSFlow
    variables:
      - name: inputValue
        value: 10
    steps:
      - manual_start:
          name: Start
      - js:
          name: ComputeResult
          code: |
            export default function(context) {
              return { result: "ok" };
            }
          depends_on: Start
`

	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlContent), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	// Import the flow data
	fixture.importWorkspaceBundle(resolved)

	// Get the flow
	flow := fixture.getFlowByName("JSFlow")
	if flow == nil {
		t.Fatal("JSFlow not found")
	}

	ctx, cancel := context.WithTimeout(fixture.ctx, 30*time.Second)
	defer cancel()

	if err := jsRunner.Start(ctx); err != nil {
		t.Fatalf("failed to start JS runner: %v", err)
	}

	// Verify the JS runner client was created
	if jsRunner.Client() == nil {
		t.Fatal("JS runner client is nil")
	}

	// Run the flow with JS client
	result, _ := runner.RunFlow(ctx, flow, fixture.getRunnerServices(jsRunner.Client()), nil)

	// Verify flow was attempted
	if result.FlowName != "JSFlow" {
		t.Errorf("expected flow name 'JSFlow', got '%s'", result.FlowName)
	}

	// Verify JS node was attempted (regardless of success/failure)
	foundJSNode := false
	for _, node := range result.Nodes {
		if node.Name == "ComputeResult" {
			foundJSNode = true
			t.Logf("JS node state: %s, error: %s", node.State, node.Error)
			break
		}
	}

	if !foundJSNode {
		t.Error("JS node 'ComputeResult' was not found in results")
	}
}

// TestFlowRun_OrphanNodesNotExecuted verifies that nodes without depends_on
// remain disconnected and don't execute when there's an explicit manual_start.
func TestFlowRun_OrphanNodesNotExecuted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	// Create a flow with explicit manual_start and orphan nodes (no depends_on)
	// Only the Start node should execute, OrphanRequest should NOT execute
	yamlContent := fmt.Sprintf(`workspace_name: Orphan Node Test
flows:
  - name: OrphanFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: ConnectedRequest
          method: GET
          url: %s/connected
          depends_on: Start
      - request:
          name: OrphanRequest
          method: GET
          url: %s/orphan
`, fixture.mockServer.URL, fixture.mockServer.URL)

	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlContent), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	// Verify edges were created correctly - OrphanRequest should not be connected
	orphanConnected := false
	for _, edge := range resolved.FlowEdges {
		for _, node := range resolved.FlowNodes {
			if node.Name == "OrphanRequest" && edge.TargetID == node.ID {
				orphanConnected = true
				break
			}
		}
	}
	if orphanConnected {
		t.Error("OrphanRequest should NOT have any incoming edges")
	}

	// Import the flow data
	fixture.importWorkspaceBundle(resolved)

	// Get the flow
	flow := fixture.getFlowByName("OrphanFlow")
	if flow == nil {
		t.Fatal("OrphanFlow not found")
	}

	// Run the flow
	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	result, err := runner.RunFlow(ctx, flow, fixture.getRunnerServices(nil), nil)
	if err != nil {
		t.Fatalf("flow execution failed: %v", err)
	}

	// Verify flow succeeded
	if result.Status != "success" {
		t.Errorf("expected status 'success', got '%s'. Error: %s", result.Status, result.Error)
	}

	// Verify ConnectedRequest was executed
	connectedExecuted := false
	orphanExecuted := false
	for _, node := range result.Nodes {
		if node.Name == "ConnectedRequest" {
			connectedExecuted = true
		}
		if node.Name == "OrphanRequest" {
			orphanExecuted = true
		}
	}

	if !connectedExecuted {
		t.Error("ConnectedRequest should have been executed")
	}

	if orphanExecuted {
		t.Error("OrphanRequest should NOT have been executed (it's an orphan node)")
	}
}

// echoWSServer creates a test WebSocket server that echoes messages back.
func echoWSServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "") //nolint:errcheck // best-effort cleanup
		for {
			typ, msg, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if err := conn.Write(r.Context(), typ, msg); err != nil {
				return
			}
		}
	}))
}

func wsURL(s *httptest.Server) string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

// TestFlowRun_WebSocket tests a flow with WebSocket connection and send nodes.
func TestFlowRun_WebSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	wsSrv := echoWSServer(t)
	defer wsSrv.Close()

	yamlContent := fmt.Sprintf(`workspace_name: WS Test
flows:
  - name: WSFlow
    steps:
      - manual_start:
          name: Start
      - ws_connection:
          name: MyWS
          depends_on: Start
          url: %s
      - ws_send:
          name: SendHello
          depends_on: MyWS
          ws_connection_node_name: MyWS
          message: '{"hello":"world"}'
`, wsURL(wsSrv))

	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlContent), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	fixture.importWorkspaceBundle(resolved)

	flow := fixture.getFlowByName("WSFlow")
	if flow == nil {
		t.Fatal("WSFlow not found")
	}

	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	result, err := runner.RunFlow(ctx, flow, fixture.getRunnerServices(nil), nil)

	if err != nil {
		t.Errorf("flow execution failed: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("expected status 'success', got '%s'. Error: %s", result.Status, result.Error)
	}

	// Verify both WS nodes were executed
	foundConn := false
	foundSend := false
	for _, node := range result.Nodes {
		switch node.Name {
		case "MyWS":
			foundConn = true
		case "SendHello":
			foundSend = true
		}
	}

	if !foundConn {
		t.Error("WS connection node 'MyWS' was not executed")
	}
	if !foundSend {
		t.Error("WS send node 'SendHello' was not executed")
	}
}

// setupMultiFlowFixture converts, imports, and returns the flows for a
// run:-block test. It is a thin helper shared by the RunMultipleFlows
// ordering/validation tests below.
func setupMultiFlowFixture(t *testing.T, fixture *flowTestFixture, yamlContent string) []mflow.Flow {
	t.Helper()

	fileData := []byte(yamlContent)
	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(fileData, yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("failed to convert YAML: %v", err)
	}

	fixture.importWorkspaceBundle(resolved)

	flows, err := fixture.services.Flow.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("failed to get flows: %v", err)
	}
	return flows
}

// TestRunMultipleFlows_ExecutesInDependencyOrder verifies that RunMultipleFlows
// executes flows in dependency order rather than run: block declaration
// order. The run: block deliberately lists B (which depends on A) before A.
func TestRunMultipleFlows_ExecutesInDependencyOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	var mu sync.Mutex
	var requestOrder []string
	fixture.mockServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestOrder = append(requestOrder, r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	yamlContent := fmt.Sprintf(`workspace_name: Order Test
run:
  - flow: B
    depends_on: A
  - flow: A
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
      - request:
          name: RequestA
          method: GET
          url: %s/a
          depends_on: Start
  - name: B
    steps:
      - manual_start:
          name: Start
      - request:
          name: RequestB
          method: GET
          url: %s/b
          depends_on: Start
`, fixture.mockServer.URL, fixture.mockServer.URL)

	flows := setupMultiFlowFixture(t, fixture, yamlContent)

	ctx, cancel := context.WithTimeout(fixture.ctx, 15*time.Second)
	defer cancel()

	if err := runner.RunMultipleFlows(ctx, []byte(yamlContent), flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil); err != nil {
		t.Fatalf("multi-flow execution failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestOrder) != 2 {
		t.Fatalf("expected 2 requests, got %d: %v", len(requestOrder), requestOrder)
	}
	if requestOrder[0] != "/a" || requestOrder[1] != "/b" {
		t.Errorf("expected requests in dependency order [/a, /b] (A before B), got %v", requestOrder)
	}
}

// TestRunMultipleFlows_UnknownDependencyError verifies that a depends_on
// entry naming a flow that is not part of the run: block produces a specific,
// actionable error rather than being silently ignored.
func TestRunMultipleFlows_UnknownDependencyError(t *testing.T) {
	fixture := newFlowTestFixture(t)

	yamlContent := `workspace_name: Unknown Dep Test
run:
  - flow: A
  - flow: B
    depends_on: Missing
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
  - name: B
    steps:
      - manual_start:
          name: Start
`
	flows := setupMultiFlowFixture(t, fixture, yamlContent)

	err := runner.RunMultipleFlows(fixture.ctx, []byte(yamlContent), flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown dependency, got nil")
	}

	const want = `unknown dependency "Missing" in run block (known flows: A, B)`
	if err.Error() != want {
		t.Errorf("unexpected error message:\n got:  %q\n want: %q", err.Error(), want)
	}
}

// TestRunMultipleFlows_CyclicDependencyError verifies that a dependency cycle
// in the run: block is rejected with a message naming the cycle, instead of
// hanging or silently running flows in an arbitrary order.
func TestRunMultipleFlows_CyclicDependencyError(t *testing.T) {
	fixture := newFlowTestFixture(t)

	yamlContent := `workspace_name: Cycle Test
run:
  - flow: A
    depends_on: B
  - flow: B
    depends_on: A
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
  - name: B
    steps:
      - manual_start:
          name: Start
`
	flows := setupMultiFlowFixture(t, fixture, yamlContent)

	err := runner.RunMultipleFlows(fixture.ctx, []byte(yamlContent), flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)
	if err == nil {
		t.Fatal("expected a dependency cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "dependency cycle in run block") {
		t.Errorf("expected a dependency cycle error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "A") || !strings.Contains(err.Error(), "B") {
		t.Errorf("expected the cycle description to name both A and B, got: %v", err)
	}
}

// TestRunMultipleFlows_EmptyRunBlockErrors verifies the pre-existing behavior
// (unchanged by dependency-ordering support) that an empty run: block is
// rejected rather than silently doing nothing.
func TestRunMultipleFlows_EmptyRunBlockErrors(t *testing.T) {
	fixture := newFlowTestFixture(t)

	yamlContent := `workspace_name: Empty Run Test
run: []
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
`
	flows := setupMultiFlowFixture(t, fixture, yamlContent)

	err := runner.RunMultipleFlows(fixture.ctx, []byte(yamlContent), flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)
	if err == nil {
		t.Fatal("expected an error for an empty run block, got nil")
	}
}

// TestRunMultipleFlows_FailedDependencySkipsDependent verifies that when a
// flow fails, flows that depend on it are reported as skipped (with a reason
// naming the failed dependency) instead of being attempted, and the overall
// call still returns a non-nil error. This preserves today's failure-gate
// semantics (a dependent never runs after its dependency fails) while
// replacing the old silent early-return with an explicit, reported skip.
func TestRunMultipleFlows_FailedDependencySkipsDependent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	requestCount := 0
	fixture.mockServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	// A's request targets an address nothing listens on (port 1 is reserved),
	// so the connection is refused and the flow fails fast. B depends on A
	// and must never reach the mock server.
	yamlContent := fmt.Sprintf(`workspace_name: Failure Gate Test
run:
  - flow: A
  - flow: B
    depends_on: A
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
      - request:
          name: RequestA
          method: GET
          url: http://127.0.0.1:1/unreachable
          depends_on: Start
  - name: B
    steps:
      - manual_start:
          name: Start
      - request:
          name: RequestB
          method: GET
          url: %s/b
          depends_on: Start
`, fixture.mockServer.URL)

	flows := setupMultiFlowFixture(t, fixture, yamlContent)

	ctx, cancel := context.WithTimeout(fixture.ctx, 15*time.Second)
	defer cancel()

	err := runner.RunMultipleFlows(ctx, []byte(yamlContent), flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)
	if err == nil {
		t.Fatal("expected a non-nil error when a dependency fails, got nil")
	}
	if !strings.Contains(err.Error(), "B") {
		t.Errorf("expected the error to mention flow B, got: %v", err)
	}
	if !strings.Contains(err.Error(), "skipped") {
		t.Errorf("expected the error to report B as skipped, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"A"`) {
		t.Errorf("expected the error to name A as the failed dependency, got: %v", err)
	}

	if requestCount != 0 {
		t.Errorf("expected RequestB to never be attempted since A failed, but the mock server received %d request(s)", requestCount)
	}
}

// The three tests below pin behavior deltas identified in code review as
// unavoidable, in-scope side effects of moving run: parsing/execution onto
// the typed yamlflowsimplev2 structs and a pre-flight topological sort (see
// the "run: execution behavior deltas" section of task-1-report.md). None of
// these are new production code changes - they lock in what
// TestRunMultipleFlows_UnknownDependencyError, _CyclicDependencyError, etc.
// already exercise indirectly, made explicit and named.

// TestRunMultipleFlows_UnknownFlowPreventsAnyExecution pins delta #1: the
// "flow not found" check now runs as a pre-flight pass over every run:
// entry before anything executes, rather than being discovered mid-loop.
// Previously `run: [A, Bogus]` would run A and only then fail on Bogus;
// now nothing runs. RequestA's hit count on the mock server is the proof.
func TestRunMultipleFlows_UnknownFlowPreventsAnyExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	requestCount := 0
	fixture.mockServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	// Import only flow A - a valid, self-contained document. "Bogus" is
	// intentionally never defined under flows: anywhere.
	importYAML := fmt.Sprintf(`workspace_name: Unknown Flow Test
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
      - request:
          name: RequestA
          method: GET
          url: %s/a
          depends_on: Start
`, fixture.mockServer.URL)

	flows := setupMultiFlowFixture(t, fixture, importYAML)

	// Fed directly to RunMultipleFlows (bypassing ConvertSimplifiedYAML,
	// which would itself reject a run: entry naming an undefined flow) to
	// isolate exactly the check under test: RunMultipleFlows's own
	// cross-check of the run: block against the flows it was actually
	// given.
	runYAML := []byte(`workspace_name: Unknown Flow Test
run:
  - flow: A
  - flow: Bogus
`)

	err := runner.RunMultipleFlows(fixture.ctx, runYAML, flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown flow in the run block, got nil")
	}

	if requestCount != 0 {
		t.Errorf("expected A to never execute since Bogus is unknown, but the mock server received %d request(s)", requestCount)
	}
}

// TestRunMultipleFlows_SingleFailureMessageShape pins delta #2: the
// aggregate error format. It used to be exactly `multi-flow execution
// failed: <err>` (and picked *which* err via unordered map iteration when
// more than one flow failed). It is now `multi-flow execution failed:
// <flow>: <err> (<status>)`, built from the deterministic run order. The
// prefix/suffix produced by RunMultipleFlows are asserted exactly; the
// infix (the OS-level dial error text from the refused connection) is only
// asserted to be present, since its exact wording is not something this
// package controls or should pin.
func TestRunMultipleFlows_SingleFailureMessageShape(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := newFlowTestFixture(t)

	// Port 1 is reserved; nothing listens there, so the connection is
	// refused immediately and deterministically.
	yamlContent := `workspace_name: Single Failure Message Shape Test
run:
  - flow: A
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
      - request:
          name: RequestA
          method: GET
          url: http://127.0.0.1:1/unreachable
          depends_on: Start
`
	flows := setupMultiFlowFixture(t, fixture, yamlContent)

	ctx, cancel := context.WithTimeout(fixture.ctx, 15*time.Second)
	defer cancel()

	err := runner.RunMultipleFlows(ctx, []byte(yamlContent), flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)
	if err == nil {
		t.Fatal("expected a non-nil error when the only flow fails, got nil")
	}

	const wantPrefix = `multi-flow execution failed: A: `
	const wantSuffix = ` (failed)`
	got := err.Error()
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("expected error to start with %q, got: %q", wantPrefix, got)
	}
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("expected error to end with %q, got: %q", wantSuffix, got)
	}
	if !strings.Contains(got, "127.0.0.1:1") {
		t.Errorf("expected error to include the underlying RunFlow error, got: %q", got)
	}
}

// TestRunMultipleFlows_MalformedRunEntrySurfacesError pins delta #3: a run:
// entry that is not a {flow: ...} mapping (a bare scalar here) used to be
// silently dropped by the old hand-rolled map[string]interface{} parser (a
// failed type assertion just `continue`d past it, so the workflow quietly
// ran fewer flows than declared). Parsing run: through the typed
// yamlflowsimplev2.YamlRunEntryV2 struct means the same document now fails
// to unmarshal, surfacing the malformed entry as an error instead of
// silently ignoring it.
func TestRunMultipleFlows_MalformedRunEntrySurfacesError(t *testing.T) {
	fixture := newFlowTestFixture(t)

	importYAML := `workspace_name: Malformed Run Entry Test
flows:
  - name: A
    steps:
      - manual_start:
          name: Start
`
	flows := setupMultiFlowFixture(t, fixture, importYAML)

	runYAML := []byte(`workspace_name: Malformed Run Entry Test
run:
  - flow: A
  - just-a-string
`)

	err := runner.RunMultipleFlows(fixture.ctx, runYAML, flows, fixture.getRunnerServices(nil), fixture.services.Logger, nil)
	if err == nil {
		t.Fatal("expected a YAML unmarshal error for the malformed run: entry, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected an unmarshal error naming the parse failure, got: %v", err)
	}
}
