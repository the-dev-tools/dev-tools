package rflowv2

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

type capturedEvent struct {
	Type      string
	NodeID    string
	State     string
	Timestamp time.Time
}

func TestFlowRun_AssertionOrder(t *testing.T) {
	// 1. Setup Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// 2. Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer db.Close()

	queries := gen.New(db)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 3. Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	noopService := sflow.NewNodeNoopService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	nodeRequestService := sflow.NewNodeRequestService(queries)

	httpService := shttp.New(queries, logger)
	shttpBodyRawSvc := shttp.NewHttpBodyRawService(queries)
	resAssertSvc := shttp.NewHttpAssertService(queries)
	httpResponseService := shttp.NewHttpResponseService(queries)

	resHeaderSvc := shttp.NewHttpHeaderService(queries)
	resSearchParamSvc := shttp.NewHttpSearchParamService(queries)
	resBodyFormSvc := shttp.NewHttpBodyFormService(queries)
	resBodyUrlencodedSvc := shttp.NewHttpBodyUrlEncodedService(queries)

	nodeForService := sflow.NewNodeForService(queries)
	nodeForEachService := sflow.NewNodeForEachService(queries)
	nodeIfService := sflow.NewNodeIfService(queries)
	nodeNodeJsService := sflow.NewNodeJsService(queries)
	envService := senv.NewEnvironmentService(queries, logger)
	varService := senv.NewVariableService(queries, logger)

	// Streams
	executionStream := memory.NewInMemorySyncStreamer[ExecutionTopic, ExecutionEvent]()
	assertStream := memory.NewInMemorySyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]()

	// Resolver
	res := resolver.NewStandardResolver(
		&httpService,
		&resHeaderSvc,
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
		envService.Reader(),
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
		&nodeNodeJsService,
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
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // 1-10
		executionStream, // 11
		nil, nil,        // 12-13
		assertStream, // 14
		nil,          // 15 (logStream)
		nil,          // jsClient
	)

	// 4. Setup Data
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{ID: userID, Email: "test@example.com"})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	err = wsService.Create(ctx, &mworkspace.Workspace{ID: workspaceID, Name: "Test Workspace", Updated: dbtime.DBNow()})
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{ID: idwrap.NewNow(), WorkspaceID: workspaceID, UserID: userID, Role: 1})
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	err = flowService.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: workspaceID, Name: "Assertion Flow"})
	require.NoError(t, err)

	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{ID: httpID, WorkspaceID: workspaceID, Name: "Req", Method: "GET", Url: ts.URL, BodyKind: mhttp.HttpBodyKindNone})
	require.NoError(t, err)

	// Add Assertion
	assertID := idwrap.NewNow()
	err = resAssertSvc.Create(ctx, &mhttp.HTTPAssert{
		ID:      assertID,
		HttpID:  httpID,
		Value:   "response.status == 200", // Standard simplified assertion syntax often used
		Enabled: true,
	})
	require.NoError(t, err)

	startNodeID := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_NO_OP})
	require.NoError(t, err)
	err = noopService.CreateNodeNoop(ctx, mflow.NodeNoop{FlowNodeID: startNodeID, Type: mflow.NODE_NO_OP_KIND_START})
	require.NoError(t, err)

	requestNodeID := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: requestNodeID, FlowID: flowID, Name: "Request Node", NodeKind: mflow.NODE_KIND_REQUEST})
	require.NoError(t, err)
	err = nodeRequestService.CreateNodeRequest(ctx, mflow.NodeRequest{FlowNodeID: requestNodeID, HttpID: &httpID})
	require.NoError(t, err)

	err = edgeService.CreateEdge(ctx, mflow.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: requestNodeID, SourceHandler: mflow.HandleUnspecified})
	require.NoError(t, err)

	// 5. Capture Events
	var events []capturedEvent
	var mu sync.Mutex

	assertCh, _ := assertStream.Subscribe(ctx, func(topic rhttp.HttpResponseAssertTopic) bool { return true })
	execCh, _ := executionStream.Subscribe(ctx, func(topic ExecutionTopic) bool { return true })

	go func() {
		for {
			select {
			case <-assertCh:
				mu.Lock()
				events = append(events, capturedEvent{Type: "assertion", Timestamp: time.Now()})
				mu.Unlock()
			case evt := <-execCh:
				mu.Lock()
				nodeID, _ := idwrap.NewFromBytes(evt.Payload.Execution.NodeId)

				stateStr := "UNKNOWN"
				switch evt.Payload.Execution.State {
				case flowv1.FlowItemState_FLOW_ITEM_STATE_RUNNING:
					stateStr = "RUNNING"
				case flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS:
					stateStr = "SUCCESS"
				}

				events = append(events, capturedEvent{
					Type:      "execution",
					NodeID:    nodeID.String(),
					State:     stateStr,
					Timestamp: time.Now(),
				})
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// 6. Run Flow
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(ctx, req)
	require.NoError(t, err)

	// 7. Wait and Verify
	time.Sleep(1 * time.Second)

	mu.Lock()
	defer mu.Unlock()

	assertionIndex := -1
	requestSuccessIndex := -1

	for i, evt := range events {
		if evt.Type == "assertion" {
			if assertionIndex == -1 {
				assertionIndex = i
			}
		}
		if evt.Type == "execution" {
			if evt.NodeID == requestNodeID.String() && evt.State == "SUCCESS" {
				requestSuccessIndex = i
			}
		}
	}

	assert.NotEqual(t, -1, assertionIndex, "Should receive assertion event")
	assert.NotEqual(t, -1, requestSuccessIndex, "Should receive success execution event for request node")

	if assertionIndex != -1 && requestSuccessIndex != -1 {
		assert.Less(t, assertionIndex, requestSuccessIndex, "Assertion event should arrive before Request Node Success event")
	}
}
