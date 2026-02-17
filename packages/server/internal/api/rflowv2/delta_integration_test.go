package rflowv2

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func TestFlowRun_DeltaOverride(t *testing.T) {
	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 1. Setup Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Method (Delta should override Base GET with POST)
		require.Equal(t, "POST", r.Method, "Expected method POST")

		// Verify Header (Delta should override Base header)
		require.Equal(t, "Delta", r.Header.Get("X-Test"), "Expected X-Test header 'Delta'")

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

	// 3. Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)

	// shttp services (Used by FlowServiceV2RPC and for Data Creation)
	httpService := shttp.New(queries, logger)
	shttpHeaderSvc := shttp.NewHttpHeaderService(queries)
	shttpBodyRawSvc := shttp.NewHttpBodyRawService(queries) // Shared

	// Independent services (Used by StandardResolver)
	resHeaderSvc := shttp.NewHttpHeaderService(queries)
	resSearchParamSvc := shttp.NewHttpSearchParamService(queries)
	resBodyFormSvc := shttp.NewHttpBodyFormService(queries)
	resBodyUrlencodedSvc := shttp.NewHttpBodyUrlEncodedService(queries)
	resAssertSvc := shttp.NewHttpAssertService(queries)

	nodeRequestService := sflow.NewNodeRequestService(queries)

	// Node specific services needed for FlowServiceV2RPC
	nodeForService := sflow.NewNodeForService(queries)
	nodeForEachService := sflow.NewNodeForEachService(queries)
	nodeIfService := sflow.NewNodeIfService(queries) // Returns *NodeIfService
	nodeNodeJsService := sflow.NewNodeJsService(queries)
	nodeAIService := sflow.NewNodeAIService(queries)
	nodeAiProviderService := sflow.NewNodeAiProviderService(queries)
	nodeMemoryService := sflow.NewNodeMemoryService(queries)

	// Response services
	httpResponseService := shttp.NewHttpResponseService(queries)

	// Environment and variable services
	envService := senv.NewEnvironmentService(queries, logger)
	varService := senv.NewVariableService(queries, logger)

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
			User:      sworkspace.NewUserReaderFromQueries(queries),
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
	})

	// 4. Setup Data
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// User
	err = queries.CreateUser(ctx, gen.CreateUserParams{
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
	err = wsService.Create(ctx, &workspace)
	require.NoError(t, err)

	// Workspace User
	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        3,
	})
	require.NoError(t, err)

	// Flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Delta Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// --- HTTP Data ---

	// Base Request
	baseID := idwrap.NewNow()
	baseReq := mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Method:      "GET",
		Url:         ts.URL,
		BodyKind:    mhttp.HttpBodyKindNone,
	}
	err = httpService.Create(ctx, &baseReq)
	require.NoError(t, err)

	// Base Header
	baseHeaderID := idwrap.NewNow()
	baseHeader := mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseID,
		Key:     "X-Test",
		Value:   "Base",
		Enabled: true,
	}
	// Use shttpHeaderSvc to create data (it accepts mhttp models)
	err = shttpHeaderSvc.Create(ctx, &baseHeader)
	require.NoError(t, err)

	// Delta Request
	deltaID := idwrap.NewNow()
	deltaReq := mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  workspaceID,
		Name:         "Delta Request",
		Method:       "POST",
		ParentHttpID: &baseID,
		IsDelta:      true,
		DeltaMethod:  func() *string { s := "POST"; return &s }(),
	}

	err = httpService.Create(ctx, &deltaReq)
	require.NoError(t, err)

	// Delta Header (Override)
	deltaHeaderID := idwrap.NewNow()
	deltaHeader := mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaID,
		Key:                "X-Test",
		ParentHttpHeaderID: &baseHeaderID,
		IsDelta:            true,
		DeltaValue:         func() *string { s := "Delta"; return &s }(),
		DeltaEnabled:       func() *bool { b := true; return &b }(),
	}
	err = shttpHeaderSvc.Create(ctx, &deltaHeader)
	require.NoError(t, err)

	// --- Flow Nodes ---

	// Start Node
	startNodeID := idwrap.NewNow()
	startNode := mflow.Node{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mflow.NODE_KIND_MANUAL_START,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)

	// HTTP Request Node
	requestNodeID := idwrap.NewNow()
	requestNode := mflow.Node{
		ID:        requestNodeID,
		FlowID:    flowID,
		Name:      "Delta Request Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, requestNode)
	require.NoError(t, err)

	// Link Node to HTTP (with Delta)
	err = nodeRequestService.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID:  requestNodeID,
		HttpID:      &baseID,
		DeltaHttpID: &deltaID, // Mapped to delta_http_id in DB
	})
	require.NoError(t, err)

	// Edge: Start -> Request
	edgeID := idwrap.NewNow()
	err = edgeService.CreateEdge(ctx, mflow.Edge{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      requestNodeID,
		SourceHandler: mflow.HandleUnspecified,
	})
	require.NoError(t, err)

	// 5. Execution
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(ctx, req)
	require.NoError(t, err)

	// 6. Verification
	// Check Node Execution (Poll for completion)
	var exec *mflow.NodeExecution
	for i := 0; i < 10; i++ {
		exec, err = nodeExecService.GetLatestNodeExecutionByNodeID(ctx, requestNodeID)
		if err == nil && exec != nil && mflow.NodeState(exec.State) == mflow.NODE_STATE_SUCCESS {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	require.NoError(t, err)
	require.NotNil(t, exec, "Node execution not found for node %s", requestNodeID.String())
	require.Equal(t, mflow.NODE_STATE_SUCCESS, mflow.NodeState(exec.State))
}
