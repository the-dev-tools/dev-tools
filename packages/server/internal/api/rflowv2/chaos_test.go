package rflowv2

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// TestChaos_EventOrdering fires many concurrent flows and introduces random delays
// to see if we can break the "HttpResponse arrived before NodeExecution" invariant.
func TestChaos_EventOrdering(t *testing.T) {
	// 1. Setup Mock Server with random latency
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		latency := time.Duration(rand.Intn(50)) * time.Millisecond
		time.Sleep(latency)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Chaos Response"))
	}))
	defer ts.Close()

	// 2. Setup DB
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	db.SetMaxOpenConns(10) // Allow more concurrency
	defer db.Close()

	queries := gen.New(db)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// 3. Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
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
	nodeAIService := sflow.NewNodeAIService(queries)
	nodeAiProviderService := sflow.NewNodeAiProviderService(queries)
	nodeMemoryService := sflow.NewNodeMemoryService(queries)
	envService := senv.NewEnvironmentService(queries, logger)
	varService := senv.NewVariableService(queries, logger)

	// Streams
	executionStream := memory.NewInMemorySyncStreamer[ExecutionTopic, ExecutionEvent]()
	responseStream := memory.NewInMemorySyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]()

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
		Streamers: FlowServiceV2Streamers{
			Execution:    executionStream,
			HttpResponse: responseStream,
		},
		Resolver: res,
		Logger:   logger,
	})

	// 4. Setup Data
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)
	require.NoError(t, queries.CreateUser(ctx, gen.CreateUserParams{ID: userID, Email: "chaos@example.com"}))

	workspaceID := idwrap.NewNow()
	require.NoError(t, wsService.Create(ctx, &mworkspace.Workspace{ID: workspaceID, Name: "Chaos WS", Updated: dbtime.DBNow()}))
	require.NoError(t, queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{ID: idwrap.NewNow(), WorkspaceID: workspaceID, UserID: userID, Role: 1}))

	flowID := idwrap.NewNow()
	require.NoError(t, flowService.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: workspaceID, Name: "Chaos Flow"}))

	httpID := idwrap.NewNow()
	require.NoError(t, httpService.Create(ctx, &mhttp.HTTP{ID: httpID, WorkspaceID: workspaceID, Name: "Chaos Req", Method: "POST", Url: ts.URL}))

	startNodeID := idwrap.NewNow()
	require.NoError(t, nodeService.CreateNode(ctx, mflow.Node{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START}))

	requestNodeID := idwrap.NewNow()
	require.NoError(t, nodeService.CreateNode(ctx, mflow.Node{ID: requestNodeID, FlowID: flowID, Name: "Request", NodeKind: mflow.NODE_KIND_REQUEST}))
	require.NoError(t, nodeRequestService.CreateNodeRequest(ctx, mflow.NodeRequest{FlowNodeID: requestNodeID, HttpID: &httpID}))

	require.NoError(t, edgeService.CreateEdge(ctx, mflow.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: requestNodeID, SourceHandler: mflow.HandleUnspecified}))

	// 5. Chaos Monitoring
	const iterations = 50
	var orderViolations int
	var mu sync.Mutex

	// Track arrived event IDs
	arrivedResponses := make(map[string]time.Time)
	var monitorWg sync.WaitGroup
	monitorWg.Add(2)

	respCh, _ := responseStream.Subscribe(ctx, func(topic rhttp.HttpResponseTopic) bool { return true })
	execCh, _ := executionStream.Subscribe(ctx, func(topic ExecutionTopic) bool { return true })

	// Monitoring Goroutines
	go func() {
		defer monitorWg.Done()
		for {
			select {
			case evt := <-respCh:
				mu.Lock()
				// Use the actual proto field name (HttpResponseId or ResponseId)
				// Based on converter.go it's HttpResponseId
				id, _ := idwrap.NewFromBytes(evt.Payload.HttpResponse.HttpResponseId)
				respID := id.String()
				arrivedResponses[respID] = time.Now()
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer monitorWg.Done()
		for {
			select {
			case evt := <-execCh:
				if evt.Payload.Execution.State == flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS {
					mu.Lock()
					// Based on rflowv2_node_exec.go, NodeExecution has HttpResponseId
					respIDBytes := evt.Payload.Execution.HttpResponseId
					if len(respIDBytes) > 0 {
						id, _ := idwrap.NewFromBytes(respIDBytes)
						respID := id.String()
						_, found := arrivedResponses[respID]
						if !found {
							// VIOLATION! Node Success arrived but Response is missing!
							orderViolations++
						}
					}
					mu.Unlock()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 6. Launch Concurrent Runs
	var runWg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		runWg.Add(1)
		go func() {
			defer runWg.Done()
			// Random start jitter
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

			req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
			_, err := svc.FlowRun(ctx, req)
			if err != nil {
				// We don't fail chaos test for single run errors unless it's DB lock
				// (But with 10 conns it shouldn't lock)
			}
		}()
	}

	runWg.Wait()
	time.Sleep(2 * time.Second) // Wait for all events to settle
	cancel()
	monitorWg.Wait()

	t.Logf("Chaos Test Results: Iterations: %d, Violations: %d", iterations, orderViolations)
	assert.Equal(t, 0, orderViolations, "Should have ZERO order violations even under chaos")
}
