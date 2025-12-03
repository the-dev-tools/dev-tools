package cmd

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
	"testing"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
)

// flowTestFixture provides a common test environment for flow execution tests
type flowTestFixture struct {
	t           *testing.T
	ctx         context.Context
	db          *sql.DB
	queries     *gen.Queries
	services    FlowServiceLocal
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
	workspaceService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeRequestService := snoderequest.New(queries)
	nodeIfService := snodeif.New(queries)
	nodeNoopService := snodenoop.New(queries)
	flowVariableService := sflowvariable.New(queries)
	nodeForService := snodefor.New(queries)
	nodeForEachService := snodeforeach.New(queries)
	nodeJSService := snodejs.New(queries)
	edgeService := sedge.New(queries)

	// V2 HTTP services
	httpService := shttp.New(queries, logger)
	httpHeaderService := shttp.NewHttpHeaderService(queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(queries)
	httpBodyRawService := shttp.NewHttpBodyRawService(queries)
	httpAssertService := shttp.NewHttpAssertService(queries)

	logMap := logconsole.NewLogChanMap()

	services := FlowServiceLocal{
		DB:      db,
		ws:      workspaceService,
		fs:      flowService,
		fes:     edgeService,
		fvs:     flowVariableService,
		ns:      nodeService,
		rns:     nodeRequestService,
		fns:     nodeForService,
		fens:    nodeForEachService,
		sns:     nodeNoopService,
		ins:     *nodeIfService,
		jsns:    nodeJSService,
		hs:      httpService,
		hh:      httpHeaderService,
		hsp:     httpSearchParamService,
		hbf:     httpBodyFormService,
		hbu:     httpBodyUrlEncodedService,
		hbr:     httpBodyRawService,
		has:     httpAssertService,
		logger:  logger,
		logChanMap: logMap,
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

// importWorkspaceBundle imports a workspace bundle into the database
func (f *flowTestFixture) importWorkspaceBundle(bundle *ioworkspace.WorkspaceBundle) {
	f.t.Helper()

	// Import HTTP requests first
	for i := range bundle.HTTPRequests {
		err := f.services.hs.Create(f.ctx, &bundle.HTTPRequests[i])
		if err != nil {
			f.t.Fatalf("failed to create HTTP request: %v", err)
		}
	}

	// Import HTTP headers
	for i := range bundle.HTTPHeaders {
		err := f.services.hh.Create(f.ctx, &bundle.HTTPHeaders[i])
		if err != nil {
			f.t.Fatalf("failed to create HTTP header: %v", err)
		}
	}

	// Import HTTP search params
	for i := range bundle.HTTPSearchParams {
		err := f.services.hsp.Create(f.ctx, &bundle.HTTPSearchParams[i])
		if err != nil {
			f.t.Fatalf("failed to create HTTP search param: %v", err)
		}
	}

	// Import HTTP body forms
	for i := range bundle.HTTPBodyForms {
		err := f.services.hbf.Create(f.ctx, &bundle.HTTPBodyForms[i])
		if err != nil {
			f.t.Fatalf("failed to create HTTP body form: %v", err)
		}
	}

	// Import HTTP body urlencoded
	for i := range bundle.HTTPBodyUrlencoded {
		err := f.services.hbu.Create(f.ctx, &bundle.HTTPBodyUrlencoded[i])
		if err != nil {
			f.t.Fatalf("failed to create HTTP body urlencoded: %v", err)
		}
	}

	// Import HTTP body raw
	for i := range bundle.HTTPBodyRaw {
		raw := bundle.HTTPBodyRaw[i]
		_, err := f.services.hbr.Create(f.ctx, raw.HttpID, raw.RawData, raw.ContentType)
		if err != nil {
			f.t.Fatalf("failed to create HTTP body raw: %v", err)
		}
	}

	// Import HTTP asserts
	for i := range bundle.HTTPAsserts {
		err := f.services.has.Create(f.ctx, &bundle.HTTPAsserts[i])
		if err != nil {
			f.t.Fatalf("failed to create HTTP assert: %v", err)
		}
	}

	// Import flows
	for _, flow := range bundle.Flows {
		err := f.services.fs.CreateFlow(f.ctx, flow)
		if err != nil {
			f.t.Fatalf("failed to create flow %s: %v", flow.Name, err)
		}
	}

	// Import flow variables
	for _, flowVar := range bundle.FlowVariables {
		err := f.services.fvs.CreateFlowVariable(f.ctx, flowVar)
		if err != nil {
			f.t.Fatalf("failed to create flow variable: %v", err)
		}
	}

	// Import flow nodes
	for _, node := range bundle.FlowNodes {
		err := f.services.ns.CreateNode(f.ctx, node)
		if err != nil {
			f.t.Fatalf("failed to create node: %v", err)
		}
	}

	// Import node implementations
	for _, reqNode := range bundle.FlowRequestNodes {
		err := f.services.rns.CreateNodeRequest(f.ctx, reqNode)
		if err != nil {
			f.t.Fatalf("failed to create request node: %v", err)
		}
	}

	for _, noopNode := range bundle.FlowNoopNodes {
		err := f.services.sns.CreateNodeNoop(f.ctx, noopNode)
		if err != nil {
			f.t.Fatalf("failed to create noop node: %v", err)
		}
	}

	for _, ifNode := range bundle.FlowConditionNodes {
		err := f.services.ins.CreateNodeIf(f.ctx, ifNode)
		if err != nil {
			f.t.Fatalf("failed to create if node: %v", err)
		}
	}

	for _, forNode := range bundle.FlowForNodes {
		err := f.services.fns.CreateNodeFor(f.ctx, forNode)
		if err != nil {
			f.t.Fatalf("failed to create for node: %v", err)
		}
	}

	for _, forEachNode := range bundle.FlowForEachNodes {
		err := f.services.fens.CreateNodeForEach(f.ctx, forEachNode)
		if err != nil {
			f.t.Fatalf("failed to create for each node: %v", err)
		}
	}

	for _, jsNode := range bundle.FlowJSNodes {
		err := f.services.jsns.CreateNodeJS(f.ctx, jsNode)
		if err != nil {
			f.t.Fatalf("failed to create JS node: %v", err)
		}
	}

	// Import edges
	for _, edge := range bundle.FlowEdges {
		err := f.services.fes.CreateEdge(f.ctx, edge)
		if err != nil {
			f.t.Fatalf("failed to create edge: %v", err)
		}
	}
}

// getFlowByName retrieves a flow by name from the workspace
func (f *flowTestFixture) getFlowByName(name string) *mflow.Flow {
	f.t.Helper()

	flows, err := f.services.fs.GetFlowsByWorkspaceID(f.ctx, f.workspaceID)
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
      - request:
          name: Request1
          method: GET
          url: %s/test
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

	result, err := flowRun(ctx, flow, fixture.services, nil)

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
      - request:
          name: RequestA
          method: GET
          url: %s/users/1
  - name: FlowB
    steps:
      - request:
          name: RequestB
          method: GET
          url: %s/posts/1
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
	flows, err := fixture.services.fs.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("failed to get flows: %v", err)
	}

	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}

	// Run multiple flows using the runMultipleFlows function
	err = runMultipleFlows(fixture.ctx, fileData, flows, fixture.services, fixture.services.logger, nil)

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
      - request:
          name: Req1
          method: GET
          url: %s/api/test
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
	result, err := flowRun(fixture.ctx, flow, fixture.services, nil)

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
      - request:
          name: Req1
          method: GET
          url: %s/test
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
	flows, err := fixture.services.fs.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
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
      - request:
          name: GetRequest
          method: GET
          url: %s/test
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

	result, err := flowRun(fixture.ctx, flow, fixture.services, nil)

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
