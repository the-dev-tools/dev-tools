package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"connectrpc.com/connect"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rimportv2"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/flow/flowbuilder"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
	importv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
)

// TestE2E_HAR_To_CLI_Chain verifies the complete lifecycle:
// 1. HAR Import -> DB
// 2. DB -> YAML Export
// 3. YAML Import -> New DB (CLI Simulation)
// 4. Execution -> Mock Server Validation
func TestE2E_HAR_To_CLI_Chain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	// --- Phase 1: Setup Infrastructure & Mock Server ---

	// Mock Server that we expect to be hit by the final execution
	serverHit := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/e2e-target" && r.Method == "POST" {
			serverHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "received"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Generate HAR content targeting this mock server
	harContent := generateHAR(mockServer.URL + "/e2e-target")

	// --- Phase 2: Import HAR into Initial DB ---

	// Setup Initial DB (Server side)
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.Close()

	services := baseDB.GetBaseServices()

	// Create User & Workspace
	uid := idwrap.NewNow()
	wsID := idwrap.NewNow()

	providerID := "test-provider"
	err := services.Us.CreateUser(ctx, &muser.User{
		ID:           uid,
		Email:        "test@example.com",
		Status:       muser.Active,
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
	})
	require.NoError(t, err)

	authCtx := mwauth.CreateAuthedContext(ctx, uid)

	err = services.Ws.Create(authCtx, &mworkspace.Workspace{ID: wsID, Name: "Source Workspace"})
	require.NoError(t, err)

	err = services.Wus.CreateWorkspaceUser(authCtx, &mworkspaceuser.WorkspaceUser{ID: idwrap.NewNow(), WorkspaceID: wsID, UserID: uid, Role: mworkspaceuser.RoleOwner})
	require.NoError(t, err)

	// Setup Import Handler
	importHandler := setupImportHandler(t, baseDB, services)

	// Parse Mock URL to get domain
	// We need to whitelist the domain for import to succeed/process fully
	// or at least handle the MissingData response. Providing it upfront makes it seamless.
	// Note: httptest URL is like http://127.0.0.1:12345
	// HAR import logic might strip port or expect exact match.
	// Let's rely on simple string manipulation.
	mockURL := mockServer.URL
	var domain string
	if len(mockURL) > 7 {
		domain = mockURL[7:] // strip http://
	}

	// Perform Import
	importReq := &importv1.ImportRequest{
		WorkspaceId: wsID.Bytes(),
		Name:        "E2E Import",
		Data:        []byte(harContent),
		DomainData: []*importv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   domain,
				Variable: "baseUrl",
			},
		},
	}
	resp, err := importHandler.Import(authCtx, connect.NewRequest(importReq))
	require.NoError(t, err, "HAR Import failed")
	if resp.Msg.MissingData != importv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED {
		t.Logf("Import warning: Missing Data: %v", resp.Msg.MissingData)
	}

	// Verify Import Success (Sanity Check)
	flows, err := services.Fs.GetFlowsByWorkspaceID(ctx, wsID)
	require.NoError(t, err)
	require.NotEmpty(t, flows, "Should have imported at least one flow")
	sourceFlowID := flows[0].ID

	// --- Phase 3: Export to YAML ---

	// Read full bundle from DB
	ioService := ioworkspace.New(baseDB.Queries, nil) // nil logger ok for test

	exportOpts := ioworkspace.ExportOptions{
		WorkspaceID:         wsID,
		FilterByFlowIDs:     []idwrap.IDWrap{sourceFlowID},
		IncludeHTTP:         true,
		IncludeFlows:        true,
		IncludeEnvironments: true,
	}

	bundle, err := ioService.Export(ctx, exportOpts)
	require.NoError(t, err, "Exporting bundle failed")

	yamlBytes, err := yamlflowsimplev2.MarshalSimplifiedYAML(bundle)
	require.NoError(t, err, "Marshaling YAML failed")

	t.Logf("Exported YAML:\n%s", string(yamlBytes))

	// --- Phase 4: CLI Simulation (New DB + Execution) ---

	// Create FRESH In-Memory DB (simulating CLI environment)
	cliDB, cliDBCleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cliDBCleanup()

	// Parse YAML (CLI Step 1)
	// We need a dummy workspace ID for the CLI session
	cliWsID := idwrap.NewNow()

	resolvedBundle, err := yamlflowsimplev2.ConvertSimplifiedYAML(yamlBytes, yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: cliWsID,
	})
	require.NoError(t, err, "CLI YAML parsing failed")

	// Setup CLI Services
	cli, err := initializeCLIServices(ctx, t, cliDB)
	require.NoError(t, err)

	// Create CLI Workspace (Import assumes it exists)
	err = cli.Workspace.Create(ctx, &mworkspace.Workspace{
		ID:   cliWsID,
		Name: "CLI Simulation Workspace",
	})
	require.NoError(t, err, "Failed to create CLI workspace")

	// Import Bundle into CLI DB (CLI Step 2)
	cliIOService := ioworkspace.New(cli.Queries, cli.Logger)

	cliTx, err := cliDB.BeginTx(ctx, nil)
	require.NoError(t, err)

	importOpts := ioworkspace.GetDefaultImportOptions(cliWsID)
	importOpts.ImportFlows = true
	importOpts.ImportHTTP = true
	importOpts.ImportEnvironments = true
	// Preserve IDs is usually true in CLI import
	importOpts.PreserveIDs = true
	_, err = cliIOService.Import(ctx, cliTx, resolvedBundle, importOpts)
	require.NoError(t, err, "CLI DB Import failed")
	require.NoError(t, cliTx.Commit())

	// Fixup: Ensure the imported environment is active and variables are enabled
	// YAML conversion might bring in variables but we need to ensure they are linked to the workspace's active environment
	// or the workspace points to the imported environment.
	envs, err := cli.Environment.ListEnvironments(ctx, cliWsID)
	require.NoError(t, err)
	if len(envs) > 0 {
		activeEnv := envs[0]
		// Update workspace to use this env
		cliWs, err := cli.Workspace.Get(ctx, cliWsID)
		require.NoError(t, err)
		cliWs.ActiveEnv = activeEnv.ID
		cliWs.GlobalEnv = activeEnv.ID
		err = cli.Workspace.Update(ctx, cliWs)
		require.NoError(t, err)

		// Ensure variables are enabled
		vars, err := cli.Variable.GetVariableByEnvID(ctx, activeEnv.ID)
		require.NoError(t, err)
		for _, v := range vars {
			if !v.Enabled {
				v.Enabled = true
				err = cli.Variable.Update(ctx, &v)
				require.NoError(t, err)
			}
			t.Logf("CLI Env Var: %s = %v (Enabled: %v)", v.VarKey, v.Value, v.Enabled)
		}
	} else {
		t.Log("No environments found in CLI DB. baseUrl might be missing.")
	}

	// Execute Flow (CLI Step 3)
	// Find the flow in CLI DB
	cliFlows, err := cli.Flow.GetFlowsByWorkspaceID(ctx, cliWsID)
	require.NoError(t, err)
	require.Len(t, cliFlows, 1, "Should have exactly one flow in CLI DB")
	targetFlow := cliFlows[0]

	// Setup Resolver & Builder
	res := resolver.NewStandardResolver(
		cli.HTTP,
		cli.HTTPHeader,
		cli.HTTPSearchParam,
		cli.HTTPBodyRaw,
		cli.HTTPBodyForm,
		cli.HTTPBodyUrlEncoded,
		cli.HTTPAssert,
	)

	builder := flowbuilder.New(
		cli.Node,
		cli.NodeRequest,
		cli.NodeFor,
		cli.NodeForEach,
		cli.NodeIf,
		cli.NodeNoop,
		cli.NodeJS,
		cli.Workspace,
		cli.Variable,
		cli.FlowVariable,
		res,
		cli.Logger,
	)

	// Map log channels (required for execution)
	logMap := logconsole.NewLogChanMap()
	_ = logMap

	executeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Execute!
	err = executeFlow(executeCtx, &targetFlow, cli, builder)
	require.NoError(t, err, "Flow Execution failed")

	// --- Phase 5: Verification ---

	assert.True(t, serverHit, "Mock server should have been hit by the executed flow")
}

// --- Helpers ---

func generateHAR(targetURL string) string {
	// Simple HAR with one POST request with a raw JSON body
	harStruct := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{"name": "E2ETest", "version": "1.0"},
			"entries": []map[string]interface{}{
				{
					"startedDateTime": time.Now().Format(time.RFC3339),
					"time":            100,
					"request": map[string]interface{}{
						"method":      "POST",
						"url":         targetURL,
						"httpVersion": "HTTP/1.1",
						"headers": []map[string]string{
							{"name": "Content-Type", "value": "application/json"},
						},
						"queryString": []interface{}{},
						"postData": map[string]interface{}{
							"mimeType": "application/json",
							"text":     `{"foo":"bar"}`,
						},
						"headersSize": -1,
						"bodySize":    -1,
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
						},
						"headersSize": -1,
						"bodySize":    -1,
					},
					"cache": map[string]interface{}{},
					"timings": map[string]interface{}{
						"send": 20, "wait": 20, "receive": 20,
					},
				},
			},
		},
	}
	bytes, _ := json.Marshal(harStruct)
	return string(bytes)
}

// cliServices bundle
type cliServices struct {
	Queries *gen.Queries
	Logger  *slog.Logger

	Workspace    *sworkspace.WorkspaceService
	Flow         *sflow.FlowService
	FlowEdge     *sflow.EdgeService
	FlowVariable *sflow.FlowVariableService
	Node         *sflow.NodeService
	NodeRequest  *sflow.NodeRequestService
	NodeFor      *sflow.NodeForService
	NodeForEach  *sflow.NodeForEachService
	NodeIf       *sflow.NodeIfService
	NodeNoop     *sflow.NodeNoopService
	NodeJS       *sflow.NodeJsService
	Environment  *senv.EnvironmentService
	Variable     *senv.VariableService

	HTTP               *shttp.HTTPService
	HTTPHeader         *shttp.HttpHeaderService
	HTTPSearchParam    *shttp.HttpSearchParamService
	HTTPBodyForm       *shttp.HttpBodyFormService
	HTTPBodyUrlEncoded *shttp.HttpBodyUrlEncodedService
	HTTPBodyRaw        *shttp.HttpBodyRawService
	HTTPAssert         *shttp.HttpAssertService
}

// initializeCLIServices mimics the CLI startup logic
func initializeCLIServices(ctx context.Context, t *testing.T, db *sql.DB) (*cliServices, error) {
	q, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Instantiate services (handling Value vs Pointer returns)
	ws := sworkspace.New(q)
	fs := sflow.NewFlowService(q)
	fes := sflow.NewEdgeService(q)
	fvs := sflow.NewFlowVariableService(q)
	ns := sflow.NewNodeService(q)
	nrs := sflow.NewNodeRequestService(q)
	nfs := sflow.NewNodeForService(q)
	nfes := sflow.NewNodeForEachService(q)
	nif := sflow.NewNodeIfService(q)
	nnos := sflow.NewNodeNoopService(q)
	njs := sflow.NewNodeJsService(q)
	env := senv.NewEnvironmentService(q, logger)
	vs := senv.NewVariableService(q, logger)

	hs := shttp.New(q, logger)
	hh := shttp.NewHttpHeaderService(q)
	hsp := shttp.NewHttpSearchParamService(q)
	hbf := shttp.NewHttpBodyFormService(q)
	hbu := shttp.NewHttpBodyUrlEncodedService(q)
	hbr := shttp.NewHttpBodyRawService(q)
	has := shttp.NewHttpAssertService(q)

	return &cliServices{
		Queries: q,
		Logger:  logger,

		Workspace:    &ws,
		Flow:         &fs,
		FlowEdge:     &fes,
		FlowVariable: &fvs,
		Node:         &ns,
		NodeRequest:  &nrs,
		NodeFor:      &nfs,
		NodeForEach:  &nfes,
		NodeIf:       nif, // Already a pointer
		NodeNoop:     &nnos,
		NodeJS:       &njs,
		Environment:  &env,
		Variable:     &vs,

		HTTP:               &hs,
		HTTPHeader:         &hh,
		HTTPSearchParam:    hsp, // Already a pointer
		HTTPBodyForm:       hbf, // Already a pointer
		HTTPBodyUrlEncoded: hbu, // Already a pointer
		HTTPBodyRaw:        hbr, // Already a pointer
		HTTPAssert:         has, // Already a pointer
	}, nil
}

// executeFlow mimics flowRun from apps/cli/cmd/flow_run.go
func executeFlow(ctx context.Context, flowPtr *mflow.Flow, c *cliServices, builder *flowbuilder.Builder) error {
	latestFlowID := flowPtr.ID

	nodes, err := c.Node.GetNodesByFlowID(ctx, latestFlowID)
	if err != nil {
		return err
	}

	edges, err := c.FlowEdge.GetEdgesByFlowID(ctx, latestFlowID)
	if err != nil {
		return err
	}
	fmt.Printf("DEBUG: Found %d edges for flow %s\n", len(edges), latestFlowID)
	edgeMap := mflow.NewEdgesMap(edges)

	flowVars, err := c.FlowVariable.GetFlowVariablesByFlowID(ctx, latestFlowID)
	if err != nil {
		return err
	}

	// Build flow variables
	flowVarsMap, err := builder.BuildVariables(ctx, flowPtr.WorkspaceID, flowVars)
	if err != nil {
		return err
	}

	// Set default timeout
	nodeTimeout := time.Second * 30

	// Initialize resources for request nodes
	httpClient := httpclient.New()
	requestBufferSize := len(nodes) * 10
	requestRespChan := make(chan nrequest.NodeRequestSideResp, requestBufferSize)

	go func() {
		for resp := range requestRespChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer close(requestRespChan)

	// Build flow node map
	flowNodeMap, startNodeID, err := builder.BuildNodes(
		ctx,
		*flowPtr,
		nodes,
		nodeTimeout,
		httpClient,
		requestRespChan,
		nil, // No JS client needed for this test
	)
	if err != nil {
		return err
	}

	// Create runner
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), latestFlowID, startNodeID, flowNodeMap, edgeMap, nodeTimeout, nil)

	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Run
	go runnerInst.Run(subCtx, flowNodeStatusChan, flowStatusChan, flowVarsMap)

	// Wait
	var nodeErrors []string
	for {
		select {
		case nodeStatus := <-flowNodeStatusChan:
			if nodeStatus.Error != nil {
				nodeErrors = append(nodeErrors, fmt.Sprintf("Node %s failed: %s", nodeStatus.NodeID, nodeStatus.Error.Error()))
			}
		case flowStatus := <-flowStatusChan:
			if runner.IsFlowStatusDone(flowStatus) {
				if flowStatus == runner.FlowStatusSuccess {
					return nil
				}
				if len(nodeErrors) > 0 {
					return fmt.Errorf("flow failed with status: %s. Node Errors: %v", runner.FlowStatusString(flowStatus), nodeErrors)
				}
				return fmt.Errorf("flow failed with status: %s", runner.FlowStatusString(flowStatus))
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// setupImportHandler re-implements the factory logic for ImportV2RPC
func setupImportHandler(t *testing.T, baseDB *testutil.BaseDBQueries, s testutil.BaseTestServices) *rimportv2.ImportV2RPC {
	mockLogger := mocklogger.NewMockLogger()

	// Child entity services
	httpHeaderService := shttp.NewHttpHeaderService(baseDB.Queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(baseDB.Queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(baseDB.Queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(baseDB.Queries)
	bodyService := shttp.NewHttpBodyRawService(baseDB.Queries)
	httpAssertService := shttp.NewHttpAssertService(baseDB.Queries)

	// Node services
	nodeService := sflow.NewNodeService(baseDB.Queries)
	nodeRequestService := sflow.NewNodeRequestService(baseDB.Queries)
	nodeNoopService := sflow.NewNodeNoopService(baseDB.Queries)
	edgeService := sflow.NewEdgeService(baseDB.Queries)

	// Environment and variable services
	envService := senv.NewEnvironmentService(baseDB.Queries, mockLogger)
	varService := senv.NewVariableService(baseDB.Queries, mockLogger)
	fileService := sfile.New(baseDB.Queries, mockLogger)

	// Create streamers
	flowStream := memory.NewInMemorySyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]()
	nodeStream := memory.NewInMemorySyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]()
	edgeStream := memory.NewInMemorySyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]()
	noopStream := memory.NewInMemorySyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent]()
	stream := memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]()
	httpHeaderStream := memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]()
	httpSearchParamStream := memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]()
	httpBodyFormStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]()
	httpBodyUrlEncodedStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]()
	httpBodyRawStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]()
	httpAssertStream := memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]()
	fileStream := memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent]()

	// Create import handler
	importHandler := rimportv2.NewImportV2RPC(
		baseDB.DB,
		mockLogger,
		rimportv2.ImportServices{
			Workspace:          s.Ws,
			User:               s.Us,
			Http:               &s.Hs,
			Flow:               &s.Fs,
			File:               fileService,
			Env:                envService,
			Var:                varService,
			HttpHeader:         httpHeaderService,
			HttpSearchParam:    httpSearchParamService,
			HttpBodyForm:       httpBodyFormService,
			HttpBodyUrlEncoded: httpBodyUrlEncodedService,
			HttpBodyRaw:        bodyService,
			HttpAssert:         httpAssertService,
			Node:               &nodeService,
			NodeRequest:        &nodeRequestService,
			NodeNoop:           &nodeNoopService,
			Edge:               &edgeService,
		},
		rimportv2.ImportStreamers{
			Flow:               flowStream,
			Node:               nodeStream,
			Edge:               edgeStream,
			Noop:               noopStream,
			Http:               stream,
			HttpHeader:         httpHeaderStream,
			HttpSearchParam:    httpSearchParamStream,
			HttpBodyForm:       httpBodyFormStream,
			HttpBodyUrlEncoded: httpBodyUrlEncodedStream,
			HttpBodyRaw:        httpBodyRawStream,
			HttpAssert:         httpAssertStream,
			File:               fileStream,
		},
	)

	return importHandler
}
