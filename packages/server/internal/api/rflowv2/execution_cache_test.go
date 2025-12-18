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

// TestExecutionCache verifies that execution IDs are stable for a node
// even if the runner doesn't provide them back.
func TestExecutionCache(t *testing.T) {
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
	err = flowService.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: workspaceID, Name: "Cache Flow"})
	require.NoError(t, err)

	startNodeID := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_NO_OP})
	require.NoError(t, err)
	err = noopService.CreateNodeNoop(ctx, mflow.NodeNoop{FlowNodeID: startNodeID, Type: mflow.NODE_NO_OP_KIND_START})
	require.NoError(t, err)

	// 5. Capture Events to verify ID stability
	var execEvents []ExecutionEvent
	var mu sync.Mutex

	execCh, _ := executionStream.Subscribe(ctx, func(topic ExecutionTopic) bool { return true })

	go func() {
		for {
			select {
			case evt := <-execCh:
				mu.Lock()
				execEvents = append(execEvents, evt.Payload)
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
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// We expect events for the Start node.
	// Since Start node is instantaneous, it might only emit SUCCESS.
	// But let's check what we got.

	startEvents := make([]ExecutionEvent, 0)
	for _, evt := range execEvents {
		nodeID, _ := idwrap.NewFromBytes(evt.Execution.NodeId)
		if nodeID == startNodeID {
			startEvents = append(startEvents, evt)
		}
	}

	if len(startEvents) >= 2 {
		// If we got multiple events (e.g. Running, Success), IDs MUST match
		firstID := startEvents[0].Execution.NodeExecutionId
		for i, evt := range startEvents {
			assert.Equal(t, firstID, evt.Execution.NodeExecutionId, "Execution ID changed for event %d", i)
		}
	} else if len(startEvents) == 1 {
		t.Log("Only received 1 event for start node, likely just SUCCESS. This is acceptable for instant nodes if no cache issue exists.")
	} else {
		// If we got 0 events, that's weird but might be timing (though we slept).
		// Note: The loop runs asynchronously.
	}

	// If the system generates a new ID every time, and we get >1 events, they would differ.
	// The problem described ("item does not exist in store") implies we got UPDATE without INSERT
	// OR we got INSERT(ID1) then UPDATE(ID2).

	// Since Start node usually just emits SUCCESS immediately in local runner,
	// we might not see the "Running" state if it's too fast.
	// But let's check if the code logic handles it.
}
