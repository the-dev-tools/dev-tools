package rflow

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/reference"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
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
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

// childByKey returns the direct map child with the given key.
func childByKey(r reference.ReferenceTreeItem, key string) (reference.ReferenceTreeItem, bool) {
	if r.Kind != reference.ReferenceKind_REFERENCE_KIND_MAP {
		return reference.ReferenceTreeItem{}, false
	}
	for _, ch := range r.Map {
		if ch.Key.Kind == reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY && ch.Key.Key == key {
			return ch, true
		}
	}
	return reference.ReferenceTreeItem{}, false
}

// stringValue extracts the Value from a VALUE node.
func stringValue(r reference.ReferenceTreeItem) (string, bool) {
	if r.Kind != reference.ReferenceKind_REFERENCE_KIND_VALUE {
		return "", false
	}
	return r.Value, true
}

func TestBuildLogRefs_ErrorKindClassification(t *testing.T) {
	// Non-cancellation error should be labeled as "failed"
	refs := buildLogRefs("nodeA", "id-1", "FAILURE", errors.New("boom"), nil)
	if len(refs) == 0 {
		t.Fatalf("expected reference items")
	}
	root := refs[0]
	errRef, ok := childByKey(root, "error")
	if !ok {
		t.Fatalf("missing error map")
	}
	kindRef, ok := childByKey(errRef, "kind")
	if !ok {
		t.Fatalf("missing error.kind")
	}
	if kind, ok := stringValue(kindRef); !ok || kind != "failed" {
		t.Fatalf("expected kind 'failed'")
	}

	// Cancellation by throw should be labeled as "canceled"
	refs = buildLogRefs("nodeB", "id-2", "CANCELED", runner.ErrFlowCanceledByThrow, nil)
	root = refs[0]
	errRef, ok = childByKey(root, "error")
	if !ok {
		t.Fatalf("missing error map")
	}
	kindRef, ok = childByKey(errRef, "kind")
	if !ok {
		t.Fatalf("missing error.kind")
	}
	if kind, ok := stringValue(kindRef); !ok || kind != "canceled" {
		t.Fatalf("expected kind 'canceled' for throw")
	}

	// Context cancellation should be labeled as "canceled"
	refs = buildLogRefs("nodeC", "id-3", "CANCELED", context.Canceled, nil)
	root = refs[0]
	errRef, ok = childByKey(root, "error")
	if !ok {
		t.Fatalf("missing error map")
	}
	kindRef, ok = childByKey(errRef, "kind")
	if !ok {
		t.Fatalf("missing error.kind")
	}
	if kind, ok := stringValue(kindRef); !ok || kind != "canceled" {
		t.Fatalf("expected kind 'canceled' for context cancellation")
	}
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

type flowRunHarness struct {
	svc           *FlowServiceRPC
	authedCtx     context.Context
	req           *connect.Request[flowv1.FlowRunRequest]
	cleanup       func()
	logCh         chan logconsole.LogMessage
	requestNodeID idwrap.IDWrap
}

func setupFlowRunHarness(t *testing.T) flowRunHarness {
	t.Helper()

	ctx := context.Background()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)
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
	)
	flowSvc := &flowSvcValue

	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "harness-workspace",
		Updated:         time.Now(),
		ActiveEnv:       idwrap.NewNow(),
		GlobalEnv:       idwrap.NewNow(),
		FlowCount:       0,
		CollectionCount: 0,
	}
	if err := flowSvc.ws.Create(ctx, &workspace); err != nil {
		queries.Close()
		closeDB()
		t.Fatalf("create workspace: %v", err)
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
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})

	cleanup := func() {
		testServer.Close()
		logMap.DeleteLogChannel(userID)
		if err := queries.Close(); err != nil {
			t.Fatalf("close queries: %v", err)
		}
		closeDB()
	}

	return flowRunHarness{
		svc:           flowSvc,
		authedCtx:     authedCtx,
		req:           req,
		cleanup:       cleanup,
		logCh:         logCh,
		requestNodeID: requestNodeID,
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
