package rflow

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowtag"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

func TestBuildLogPayload_ErrorKindClassification(t *testing.T) {
	// Non-cancellation error should be labeled as "failed"
	payload := buildLogPayload("nodeA", "id-1", "FAILURE", errors.New("boom"), nil)
	require.NotNil(t, payload)
	errMap, ok := payload["error"].(map[string]any)
	require.True(t, ok, "expected error map")
	require.Equal(t, "failed", errMap["kind"])

	// Cancellation by throw should be labeled as "canceled"
	payload = buildLogPayload("nodeB", "id-2", "CANCELED", runner.ErrFlowCanceledByThrow, nil)
	require.NotNil(t, payload)
	errMap, ok = payload["error"].(map[string]any)
	require.True(t, ok, "expected error map")
	require.Equal(t, "canceled", errMap["kind"])

	// Context cancellation should be labeled as "canceled"
	payload = buildLogPayload("nodeC", "id-3", "CANCELED", context.Canceled, nil)
	require.NotNil(t, payload)
	errMap, ok = payload["error"].(map[string]any)
	require.True(t, ok, "expected error map")
	require.Equal(t, "canceled", errMap["kind"])
}

func TestBuildLogPayload_ErrorIncludesRequestDetails(t *testing.T) {
	requestOutput := map[string]any{
		"request": map[string]any{
			"method": "POST",
			"url":    "https://example.test",
		},
		"response": map[string]any{
			"status": float64(500),
			"body":   "{\"error\":true}",
		},
	}

	payload := buildLogPayload("Request Node", "req-1", "FAILURE", errors.New("assertion failed: status == 200"), requestOutput)
	require.NotNil(t, payload)

	requestMap, ok := payload["request"].(map[string]any)
	require.True(t, ok, "missing request payload in error log")
	require.Equal(t, "POST", requestMap["method"])
	require.Equal(t, "https://example.test", requestMap["url"])

	responseMap, ok := payload["response"].(map[string]any)
	require.True(t, ok, "missing response payload in error log")
	require.Equal(t, float64(500), responseMap["status"])
	body, ok := responseMap["body"]
	require.True(t, ok, "missing response.body in error log")
	switch v := body.(type) {
	case string:
		require.Equal(t, "{\"error\":true}", v)
	case map[string]any:
		require.True(t, v["error"].(bool))
	default:
		t.Fatalf("unexpected response.body type: %T", body)
	}

	_, dup := payload["Request Node"]
	require.False(t, dup, "unexpected duplicate node map in payload: %#v", payload)
}

func TestBuildLoopNodeExecutionFromStatus(t *testing.T) {
	loopID := idwrap.NewNow()
	execID := idwrap.NewNow()
	status := runner.FlowNodeStatus{
		NodeID: loopID,
		Name:   "Loop",
		State:  mnnode.NODE_STATE_SUCCESS,
		OutputData: map[string]any{
			"completed": true,
		},
	}

	exec := buildLoopNodeExecutionFromStatus(status, execID)

	if exec.ID != execID {
		t.Fatalf("expected execution ID %s, got %s", execID, exec.ID)
	}
	if exec.NodeID != loopID {
		t.Fatalf("expected node ID %s, got %s", loopID, exec.NodeID)
	}
	if exec.Name != "Loop" {
		t.Fatalf("expected name 'Loop', got %s", exec.Name)
	}
	if exec.State != mnnode.NODE_STATE_SUCCESS {
		t.Fatalf("expected SUCCESS state, got %v", exec.State)
	}
	if exec.CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set")
	}

	if exec.Error != nil {
		t.Fatalf("expected no error, got %s", *exec.Error)
	}

	if len(exec.OutputData) == 0 {
		t.Fatalf("expected output data to be set")
	}

	statusWithError := runner.FlowNodeStatus{
		NodeID: loopID,
		Name:   "Loop",
		State:  mnnode.NODE_STATE_FAILURE,
		Error:  context.Canceled,
	}

	execErr := buildLoopNodeExecutionFromStatus(statusWithError, execID)
	if execErr.State != mnnode.NODE_STATE_FAILURE {
		t.Fatalf("expected FAILURE state, got %v", execErr.State)
	}
	if execErr.Error == nil || *execErr.Error == "" {
		t.Fatalf("expected error string to be set")
	}
}

func TestFlowRunLogsUseExecutionName(t *testing.T) {
	harness := setupFlowRunHarness(t)
	defer harness.cleanup()

	stream := noopStream{}
	require.NoError(t, harness.svc.FlowRunAdHoc(harness.authedCtx, harness.req, stream))

	deadline := time.After(2 * time.Second)
	var nodeLog logconsole.LogMessage
	var seenNames []string
	for {
		select {
		case msg := <-harness.logCh:
			seenNames = append(seenNames, msg.Name)
			if strings.Contains(msg.Name, "Request 8") {
				nodeLog = msg
				goto haveLog
			}
		case <-deadline:
			t.Fatalf("timed out waiting for request node log; seen=%v", seenNames)
		}
	}

haveLog:
	execs, err := harness.svc.nes.GetNodeExecutionsByNodeID(context.Background(), harness.requestNodeID)
	require.NoError(t, err)
	require.NotEmpty(t, execs)
	execName := execs[0].Name
	require.NotEmpty(t, execName)
	require.Containsf(t, nodeLog.Name, execName, "log %q should reference execution %q", nodeLog.Name, execName)
}

func TestFlowRunAdHocMultipleSequentialRuns(t *testing.T) {
	harness := setupFlowRunHarness(t)
	defer harness.cleanup()

	flowID := harness.req.Msg.FlowId
	if len(flowID) == 0 {
		t.Fatalf("harness request missing flow id")
	}

	const runCount = 5
	for i := 0; i < runCount; i++ {
		iteration := i
		req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: append([]byte(nil), flowID...)})
		stream := noopStream{}

		resultCh := make(chan error, 1)
		go func() {
			resultCh <- harness.svc.FlowRunAdHoc(harness.authedCtx, req, stream)
		}()

		select {
		case err := <-resultCh:
			require.NoErrorf(t, err, "flow run %d returned error", iteration)
		case <-time.After(5 * time.Second):
			t.Fatalf("flow run %d timed out waiting for completion", iteration)
		}
	}
}

func TestFlowRunAdHoc_NodeExecutionsReachTerminalState(t *testing.T) {
	harness := setupFlowRunHarness(t)
	defer harness.cleanup()

	stream := noopStream{}
	require.NoError(t, harness.svc.FlowRunAdHoc(harness.authedCtx, harness.req, stream))

	ctx := context.Background()
	nodes, err := harness.svc.ns.GetNodesByFlowID(ctx, harness.flowID)
	require.NoError(t, err)
	require.NotEmpty(t, nodes, "expected harness flow to have nodes")

	terminalStates := map[mnnode.NodeState]struct{}{
		mnnode.NODE_STATE_SUCCESS:  {},
		mnnode.NODE_STATE_FAILURE:  {},
		mnnode.NODE_STATE_CANCELED: {},
	}

	for _, node := range nodes {
		execs, execErr := harness.svc.nes.GetNodeExecutionsByNodeID(ctx, node.ID)
		require.NoErrorf(t, execErr, "failed to load executions for node %s", node.Name)

		require.NotEmptyf(t, execs, "expected executions for node %s", node.Name)

		for _, exec := range execs {
			state := exec.State
			if _, ok := terminalStates[state]; !ok {
				t.Fatalf("node %s execution %s stuck in %s (CompletedAt=%v)", node.Name, exec.ID.String(), mnnode.StringNodeState(state), exec.CompletedAt)
			}

			require.NotNilf(t, exec.CompletedAt, "node %s execution %s missing completion timestamp", node.Name, exec.ID.String())
		}
	}
}

func TestFlowRunAdHoc_RequestFailureTransitions(t *testing.T) {
	harness := setupFlowRunHarness(t)
	defer harness.cleanup()
	ctx := context.Background()

	failingAssert := massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: harness.exampleID,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}},
		Enable:    true,
	}
	require.NoError(t, harness.svc.as.CreateAssert(ctx, failingAssert))

	stream := noopStream{}
	err := harness.svc.FlowRunAdHoc(harness.authedCtx, harness.req, stream)
	require.Error(t, err, "expected flow run to fail when assertion fails")

	execs, execErr := harness.svc.nes.GetNodeExecutionsByNodeID(ctx, harness.requestNodeID)
	require.NoError(t, execErr)
	require.NotEmpty(t, execs)

	var sawFailure bool
	for _, exec := range execs {
		require.NotEqualf(t, mnnode.NODE_STATE_RUNNING, exec.State, "execution %s should not remain running", exec.ID.String())
		if exec.State == mnnode.NODE_STATE_FAILURE {
			sawFailure = true
			require.NotNilf(t, exec.CompletedAt, "failure execution %s missing completion timestamp", exec.ID.String())
			require.NotNilf(t, exec.ResponseID, "failure execution %s missing response id", exec.ID.String())

			resp, err := harness.svc.ers.GetExampleResp(ctx, *exec.ResponseID)
			require.NoErrorf(t, err, "expected response %s to be persisted", exec.ResponseID.String())
			require.NotNilf(t, resp, "expected response %s lookup to succeed", exec.ResponseID.String())
		}
	}

	require.True(t, sawFailure, "expected failure execution to be recorded")
}

func TestFlowRunAdHoc_PersistsRequestOutput(t *testing.T) {
	harness := setupFlowRunHarness(t)
	defer harness.cleanup()

	stream := noopStream{}
	require.NoError(t, harness.svc.FlowRunAdHoc(harness.authedCtx, harness.req, stream))

	execs, err := harness.svc.nes.GetNodeExecutionsByNodeID(context.Background(), harness.requestNodeID)
	require.NoError(t, err)
	require.NotEmpty(t, execs, "expected at least one request node execution")

	sawResponse := false
	for _, exec := range execs {
		raw, err := exec.GetOutputJSON()
		require.NoErrorf(t, err, "get output json for execution %s", exec.ID)
		require.NotNilf(t, raw, "execution %s output json nil", exec.ID)
		require.Greaterf(t, len(raw), 2, "execution %s output unexpectedly empty", exec.ID)

		var output map[string]any
		require.NoError(t, json.Unmarshal(raw, &output))
		require.NotEmptyf(t, output, "execution %s output map empty", exec.ID)

		if reqVal, ok := output[nrequest.OUTPUT_REQUEST_NAME]; ok {
			reqMap, ok := reqVal.(map[string]any)
			require.True(t, ok, "request payload shape mismatch")
			method, ok := reqMap["method"].(string)
			require.True(t, ok, "request.method missing")
			require.NotEmpty(t, method)
		}

		if respVal, ok := output[nrequest.OUTPUT_RESPONE_NAME]; ok {
			reqResp, ok := respVal.(map[string]any)
			require.True(t, ok, "response payload shape mismatch")
			status, ok := reqResp["status"].(float64)
			require.True(t, ok, "response.status missing or wrong type")
			require.Greater(t, status, float64(0))
			sawResponse = true
		}
	}

	require.True(t, sawResponse, "expected to observe response output for request node")
}

func TestFlowRunAdHoc_RequestSuccessHasResponseID(t *testing.T) {
	harness := setupFlowRunHarness(t)
	defer harness.cleanup()

	stream := noopStream{}
	require.NoError(t, harness.svc.FlowRunAdHoc(harness.authedCtx, harness.req, stream))

	ctx := context.Background()
	execs, err := harness.svc.nes.GetNodeExecutionsByNodeID(ctx, harness.requestNodeID)
	require.NoError(t, err)
	require.NotEmpty(t, execs, "expected at least one request node execution")

	var sawSuccess bool
	for _, exec := range execs {
		if exec.State != mnnode.NODE_STATE_SUCCESS {
			continue
		}
		sawSuccess = true
		require.NotNilf(t, exec.ResponseID, "success execution %s missing response id", exec.ID.String())

		resp, respErr := harness.svc.ers.GetExampleResp(ctx, *exec.ResponseID)
		require.NoErrorf(t, respErr, "expected response %s to be persisted", exec.ResponseID.String())
		require.NotNil(t, resp)
		require.Equalf(t, harness.exampleID, resp.ExampleID, "response %s should belong to harness example", resp.ID.String())
	}

	require.True(t, sawSuccess, "expected to find successful request execution")
}

func TestFlowRunAdHoc_RequestResponseIDPersistsAcrossRuns(t *testing.T) {
	const iterations = 10

	harness := setupFlowRunHarness(t)
	defer harness.cleanup()

	ctx := context.Background()
	drainCtx, cancelDrain := context.WithCancel(context.Background())
	var drainWG sync.WaitGroup
	drainWG.Add(1)
	go func() {
		defer drainWG.Done()
		for {
			select {
			case <-drainCtx.Done():
				return
			case _, ok := <-harness.logCh:
				if !ok {
					return
				}
			}
		}
	}()
	defer func() {
		cancelDrain()
		drainWG.Wait()
	}()

	var previous idwrap.IDWrap

	for i := 0; i < iterations; i++ {
		stream := noopStream{}
		require.NoError(t, harness.svc.FlowRunAdHoc(harness.authedCtx, harness.req, stream))

		latest, err := harness.svc.nes.GetLatestNodeExecutionByNodeID(ctx, harness.requestNodeID)
		require.NoError(t, err)
		require.NotNilf(t, latest, "iteration %d expected latest node execution", i)

		require.NotNilf(t, latest.CompletedAt, "iteration %d latest execution missing completion timestamp", i)
		require.NotEqualf(t, mnnode.NODE_STATE_RUNNING, latest.State, "iteration %d latest execution stuck running", i)
		require.NotNilf(t, latest.ResponseID, "iteration %d execution %s missing response id", i, latest.ID.String())

		resp, respErr := harness.svc.ers.GetExampleResp(ctx, *latest.ResponseID)
		require.NoErrorf(t, respErr, "iteration %d expected response lookup to succeed for %s", i, latest.ResponseID.String())
		require.NotNilf(t, resp, "iteration %d expected response %s to exist", i, latest.ResponseID.String())

		if previous != (idwrap.IDWrap{}) {
			require.Greaterf(t, latest.ID.Compare(previous), 0, "iteration %d latest execution did not advance ULID ordering", i)
		}
		previous = latest.ID
	}
}

func TestFlowRunAdHoc_SelectedEnvironmentOverridesVariables(t *testing.T) {
	harness := setupFlowRunHarness(t)
	defer harness.cleanup()

	ctx := context.Background()

	flowVar := mflowvariable.FlowVariable{
		ID:      idwrap.NewNow(),
		FlowID:  harness.flowID,
		Name:    "authToken",
		Value:   "flow-value",
		Enabled: true,
	}
	require.NoError(t, harness.svc.fvs.CreateFlowVariable(ctx, flowVar))

	globalVar := mvar.Var{
		ID:      idwrap.NewNow(),
		EnvID:   harness.globalEnvID,
		VarKey:  "authToken",
		Value:   "global-value",
		Enabled: true,
	}
	require.NoError(t, harness.svc.vars.Create(ctx, globalVar))

	selectedVar := mvar.Var{
		ID:      idwrap.NewNow(),
		EnvID:   harness.environmentID,
		VarKey:  "authToken",
		Value:   "selected-value",
		Enabled: true,
	}
	require.NoError(t, harness.svc.vars.Create(ctx, selectedVar))

	query := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: harness.exampleID,
		QueryKey:  "token",
		Value:     "{{authToken}}",
		Enable:    true,
	}
	require.NoError(t, harness.svc.qs.CreateExampleQuery(ctx, query))

	stream := noopStream{}
	require.NoError(t, harness.svc.FlowRunAdHoc(harness.authedCtx, harness.req, stream))

	var observedReq *http.Request
	select {
	case observedReq = <-harness.requestRecorder:
	case <-time.After(2 * time.Second):
		t.Fatalf("request not captured")
	}
	require.NotNil(t, observedReq)
	require.Equal(t, "selected-value", observedReq.URL.Query().Get("token"))
}

type flowRunHarness struct {
	svc             *FlowServiceRPC
	authedCtx       context.Context
	req             *connect.Request[flowv1.FlowRunRequest]
	cleanup         func()
	logCh           chan logconsole.LogMessage
	requestNodeID   idwrap.IDWrap
	startNodeID     idwrap.IDWrap
	flowID          idwrap.IDWrap
	exampleID       idwrap.IDWrap
	environmentID   idwrap.IDWrap
	globalEnvID     idwrap.IDWrap
	requestRecorder chan *http.Request
}

func setupFlowRunHarness(t *testing.T) flowRunHarness {
	t.Helper()

	ctx := context.Background()

	requestRecorder := make(chan *http.Request, 1)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqClone := r.Clone(context.Background())
		reqClone.Body = http.NoBody
		select {
		case requestRecorder <- reqClone:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))

	db, closeDB, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("open sqlite memory DB: %v", err)
	}

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		closeDB()
		t.Fatalf("prepare queries: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)
	envs := senv.New(queries, logger)
	vs := svar.New(queries, logger)
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	fns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := *snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)
	wus := sworkspacesusers.New(queries)

	logMap := logconsole.NewLogChanMap()

	flowSvcValue := New(
		db,
		ws,
		us,
		ts,
		fs,
		fts,
		fes,
		fvs,
		envs,
		vs,
		ias,
		es,
		qs,
		hs,
		brs,
		bfs,
		bues,
		ers,
		erhs,
		as,
		ars,
		ns,
		rns,
		fns,
		fens,
		sns,
		ins,
		jsns,
		nes,
		logMap,
		nil,
	)
	flowSvc := &flowSvcValue

	workspaceID := idwrap.NewNow()
	activeEnvID := idwrap.NewNow()
	globalEnvID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "harness-workspace",
		Updated:         time.Now(),
		ActiveEnv:       activeEnvID,
		GlobalEnv:       globalEnvID,
		FlowCount:       0,
		CollectionCount: 0,
	}
	if err := flowSvc.ws.Create(ctx, &workspace); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create workspace: %v", err)
	}

	globalEnv := menv.Env{
		ID:          globalEnvID,
		WorkspaceID: workspaceID,
		Name:        "Harness Global",
	}
	if err := envs.CreateEnvironment(ctx, &globalEnv); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create global environment: %v", err)
	}
	selectedEnv := menv.Env{
		ID:          activeEnvID,
		WorkspaceID: workspaceID,
		Name:        "Harness Selected",
	}
	if err := envs.CreateEnvironment(ctx, &selectedEnv); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create selected environment: %v", err)
	}

	userID := idwrap.NewNow()
	providerID := "harness-provider"
	user := muser.User{
		ID:           userID,
		Email:        "harness@dev.tools",
		Password:     []byte("harness"),
		ProviderType: muser.MagicLink,
		ProviderID:   &providerID,
		Status:       muser.Active,
	}
	if err := flowSvc.us.CreateUser(ctx, &user); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create user: %v", err)
	}

	workspaceUser := mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}
	if err := wus.CreateWorkspaceUser(ctx, &workspaceUser); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create workspace user: %v", err)
	}

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Harness Flow",
	}
	if err := flowSvc.fs.CreateFlow(ctx, flow); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create flow: %v", err)
	}

	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	if err := flowSvc.ns.CreateNode(ctx, startNode); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create start node: %v", err)
	}

	startNoop := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}
	if err := flowSvc.sns.CreateNodeNoop(ctx, startNoop); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create start noop: %v", err)
	}

	collectionID := idwrap.NewNow()
	if err := queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Harness Collection",
	}); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create collection: %v", err)
	}

	endpointID := idwrap.NewNow()
	endpoint := &mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Harness Endpoint",
		Url:          testServer.URL,
		Method:       "GET",
	}
	if err := flowSvc.ias.CreateItemApi(ctx, endpoint); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create endpoint: %v", err)
	}

	exampleID := idwrap.NewNow()
	if err := queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		IsDefault:    true,
		BodyType:     int8(mitemapiexample.BodyTypeRaw),
		Name:         "Harness Example",
	}); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create example: %v", err)
	}

	if err := flowSvc.brs.CreateBodyRaw(ctx, mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		VisualizeMode: mbodyraw.VisualizeModeJSON,
		CompressType:  compress.CompressTypeNone,
		Data:          []byte(`{"message":"hello"}`),
	}); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create raw body: %v", err)
	}

	if err := flowSvc.ers.CreateExampleResp(ctx, mexampleresp.ExampleResp{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		Status:    200,
		Body:      []byte("{}"),
	}); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create example response: %v", err)
	}

	requestNodeID := idwrap.NewNow()
	requestNode := mnnode.MNode{
		ID:       requestNodeID,
		FlowID:   flowID,
		Name:     "Request 8",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}
	if err := flowSvc.ns.CreateNode(ctx, requestNode); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create request node: %v", err)
	}

	if err := flowSvc.rns.CreateNodeRequest(ctx, mnrequest.MNRequest{
		FlowNodeID:       requestNodeID,
		EndpointID:       &endpointID,
		ExampleID:        &exampleID,
		HasRequestConfig: true,
	}); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create node request: %v", err)
	}

	if err := flowSvc.fes.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      requestNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          int32(edge.EdgeKindNoOp),
	}); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create edge: %v", err)
	}

	logCh := logMap.AddLogChannel(userID)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes(), EnvironmentId: activeEnvID.Bytes()})

	cleanup := func() {
		testServer.Close()
		logMap.DeleteLogChannel(userID)
		close(requestRecorder)
		if err := queries.Close(); err != nil {
			t.Fatalf("close queries: %v", err)
		}
		closeDB()
	}

	return flowRunHarness{
		svc:             flowSvc,
		authedCtx:       authedCtx,
		req:             req,
		cleanup:         cleanup,
		logCh:           logCh,
		requestNodeID:   requestNodeID,
		startNodeID:     startNodeID,
		flowID:          flowID,
		exampleID:       exampleID,
		environmentID:   activeEnvID,
		globalEnvID:     globalEnvID,
		requestRecorder: requestRecorder,
	}
}

func TestFlowDurationLifecycle(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	baseServices := base.GetBaseServices()
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	flowSvc := sflow.New(base.Queries)
	initialDuration := int32(1500)
	flowID := idwrap.NewNow()
	require.NoError(t, flowSvc.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Latency Test",
		Duration:    initialDuration,
	}))

	svc := FlowServiceRPC{
		DB:         base.DB,
		ws:         baseServices.Ws,
		us:         baseServices.Us,
		fs:         flowSvc,
		logChanMap: logconsole.NewLogChanMap(),
	}

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	getResp, err := svc.FlowGet(authedCtx, connect.NewRequest(&flowv1.FlowGetRequest{FlowId: flowID.Bytes()}))
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Duration)
	require.Equal(t, initialDuration, getResp.Msg.GetDuration())

	listResp, err := svc.FlowList(authedCtx, connect.NewRequest(&flowv1.FlowListRequest{WorkspaceId: workspaceID.Bytes()}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 1)
	require.NotNil(t, listResp.Msg.Items[0].Duration)
	require.Equal(t, initialDuration, listResp.Msg.Items[0].GetDuration())

	updatedDuration := int32(2750)
	_, err = svc.FlowUpdate(authedCtx, connect.NewRequest(&flowv1.FlowUpdateRequest{
		FlowId:   flowID.Bytes(),
		Duration: &updatedDuration,
	}))
	require.NoError(t, err)

	persisted, err := flowSvc.GetFlow(ctx, flowID)
	require.NoError(t, err)
	require.Equal(t, updatedDuration, persisted.Duration)
}
