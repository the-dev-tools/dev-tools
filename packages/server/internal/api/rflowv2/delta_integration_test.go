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

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mnodeexecution"
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
	wsService := sworkspace.New(queries)
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)

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

	nodeRequestService := snoderequest.New(queries)

	// Node specific services needed for FlowServiceV2RPC
	nodeForService := snodefor.New(queries)
	nodeForEachService := snodeforeach.New(queries)
	nodeIfService := snodeif.New(queries) // Returns *NodeIfService
	nodeJsService := snodejs.New(queries)

	// Response services
	httpResponseService := shttp.NewHttpResponseService(queries)

	// Environment and variable services
	envService := senv.New(queries, logger)
	varService := svar.New(queries, logger)

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
		nil, // workspaceImportService
		httpResponseService,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // streams
		nil, // jsClient
	)

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
		Role:        1,
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
	startNode := mnnode.MNode{
		ID:       startNodeID,
		FlowID:   flowID,
		Name:     "Start",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}
	err = nodeService.CreateNode(ctx, startNode)
	require.NoError(t, err)
	err = noopService.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// HTTP Request Node
	requestNodeID := idwrap.NewNow()
	requestNode := mnnode.MNode{
		ID:        requestNodeID,
		FlowID:    flowID,
		Name:      "Delta Request Node",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	}
	err = nodeService.CreateNode(ctx, requestNode)
	require.NoError(t, err)

	// Link Node to HTTP (with Delta)
	err = nodeRequestService.CreateNodeRequest(ctx, mnrequest.MNRequest{
		FlowNodeID:  requestNodeID,
		HttpID:      &baseID,
		DeltaHttpID: &deltaID, // Mapped to delta_http_id in DB
	})
	require.NoError(t, err)

	// Edge: Start -> Request
	edgeID := idwrap.NewNow()
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      requestNodeID,
		SourceHandler: edge.HandleUnspecified,
	})
	require.NoError(t, err)

	// 5. Execution
	req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes()})
	_, err = svc.FlowRun(ctx, req)
	require.NoError(t, err)

	// 6. Verification
	// Check Node Execution (Poll for completion)
	var exec *mnodeexecution.NodeExecution
	for i := 0; i < 10; i++ {
		exec, err = nodeExecService.GetLatestNodeExecutionByNodeID(ctx, requestNodeID)
		if err == nil && exec != nil && mnnode.NodeState(exec.State) == mnnode.NODE_STATE_SUCCESS {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	require.NoError(t, err)
	require.NotNil(t, exec, "Node execution not found for node %s", requestNodeID.String())
	require.Equal(t, mnnode.NODE_STATE_SUCCESS, mnnode.NodeState(exec.State))
}
