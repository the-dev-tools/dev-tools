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
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)
	nodeRequestService := snoderequest.New(queries)
	
	httpService := shttp.New(queries, logger)
	shttpBodyRawSvc := shttp.NewHttpBodyRawService(queries)
	resAssertSvc := shttp.NewHttpAssertService(queries)
	httpResponseService := shttp.NewHttpResponseService(queries)
	
	resHeaderSvc := shttp.NewHttpHeaderService(queries)
	resSearchParamSvc := shttp.NewHttpSearchParamService(queries)
	resBodyFormSvc := shttp.NewHttpBodyFormService(queries)
	resBodyUrlencodedSvc := shttp.NewHttpBodyUrlEncodedService(queries)
	
	nodeForService := snodefor.New(queries)
	nodeForEachService := snodeforeach.New(queries)
	nodeIfService := snodeif.New(queries)
	nodeJsService := snodejs.New(queries)
	envService := senv.New(queries, logger)
	varService := svar.New(queries, logger)

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
		nil,
		httpResponseService,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		executionStream,
		nil, nil,
		assertStream,
		nil, nil,
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
		ID: assertID,
		HttpID: httpID,
		Value: "response.status == 200", // Standard simplified assertion syntax often used
		Enabled: true,
	})
	require.NoError(t, err)

	startNodeID := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mnnode.MNode{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP})
	require.NoError(t, err)
	err = noopService.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: startNodeID, Type: mnnoop.NODE_NO_OP_KIND_START})
	require.NoError(t, err)

	requestNodeID := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mnnode.MNode{ID: requestNodeID, FlowID: flowID, Name: "Request Node", NodeKind: mnnode.NODE_KIND_REQUEST})
	require.NoError(t, err)
	err = nodeRequestService.CreateNodeRequest(ctx, mnrequest.MNRequest{FlowNodeID: requestNodeID, HttpID: &httpID})
	require.NoError(t, err)

	err = edgeService.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: requestNodeID, SourceHandler: edge.HandleUnspecified})
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
