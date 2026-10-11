package rflowv2

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	gqlresolver "github.com/the-dev-tools/dev-tools/packages/server/pkg/graphql/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// TestExecutionCache verifies that execution IDs are stable for a node
// even if the runner doesn't provide them back.
// TestFlowRunEnvOverrides runs a flow whose request reads #env: values: one present only in the
// FlowRun request's env, one also in the process environment, where the request's value wins.
func TestFlowRunEnvOverrides(t *testing.T) {
	t.Setenv("STUDIO_RUN_BOTH", "from-process")

	var (
		mu   sync.Mutex
		hits []string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.Path)
		mu.Unlock()
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
	nodeGraphQLService := sflow.NewNodeGraphQLService(queries)
	graphqlService := sgraphql.New(queries, logger)
	graphqlHeaderService := sgraphql.NewGraphQLHeaderService(queries)
	graphqlAssertService := sgraphql.NewGraphQLAssertService(queries)
	graphqlResolver := gqlresolver.NewStandardResolver(graphqlService.Reader(), &graphqlHeaderService, &graphqlAssertService)
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
			Workspace:      &wsService,
			Flow:           &flowService,
			Edge:           &edgeService,
			Node:           &nodeService,
			NodeRequest:    &nodeRequestService,
			NodeFor:        &nodeForService,
			NodeForEach:    &nodeForEachService,
			NodeIf:         nodeIfService,
			NodeJs:         &nodeNodeJsService,
			NodeAI:         &nodeAIService,
			NodeAiProvider: &nodeAiProviderService,
			NodeMemory:     &nodeMemoryService,
			NodeGraphQL:    &nodeGraphQLService,
			NodeExecution:  &nodeExecService,
			FlowVariable:   &flowVarService,
			Env:            &envService,
			Var:            &varService,
			Http:           &httpService,
			HttpBodyRaw:    shttpBodyRawSvc,
			HttpResponse:   httpResponseService,
		},
		Streamers: FlowServiceV2Streamers{
			Execution:          executionStream,
			HttpResponseAssert: assertStream,
		},
		Resolver:        res,
		GraphQLResolver: graphqlResolver,
		Logger:          logger,
	})

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
	err = nodeService.CreateNode(ctx, mflow.Node{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START})
	require.NoError(t, err)

	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Hit",
		Method:      "GET",
		Url:         "{{ #env:STUDIO_RUN_BASE }}/hit/{{ #env:STUDIO_RUN_BOTH }}",
	})
	require.NoError(t, err)

	requestNodeID := idwrap.NewNow()
	err = nodeService.CreateNode(ctx, mflow.Node{ID: requestNodeID, FlowID: flowID, Name: "Hit", NodeKind: mflow.NODE_KIND_REQUEST})
	require.NoError(t, err)
	err = nodeRequestService.CreateNodeRequest(ctx, mflow.NodeRequest{FlowNodeID: requestNodeID, HttpID: &httpID})
	require.NoError(t, err)
	err = edgeService.CreateEdge(ctx, mflow.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: requestNodeID})
	require.NoError(t, err)

	_, err = svc.FlowRun(ctx, connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId: flowID.Bytes(),
		Env: []*flowv1.FlowRunEnvVar{
			{Key: "STUDIO_RUN_BASE", Value: ts.URL},
			{Key: "STUDIO_RUN_BOTH", Value: "from-override"},
		},
	}))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(hits) > 0
	}, 10*time.Second, 50*time.Millisecond)
	mu.Lock()
	require.Equal(t, []string{"/hit/from-override"}, hits)
	mu.Unlock()
	require.Eventually(t, func() bool {
		f, err := flowService.GetFlow(ctx, flowID)
		return err == nil && !f.Running
	}, 10*time.Second, 50*time.Millisecond)

	// The run's env is not left behind in the process environment.
	_, set := os.LookupEnv("STUDIO_RUN_BASE")
	require.False(t, set)
	require.False(t, strings.Contains(os.Getenv("STUDIO_RUN_BOTH"), "override"))

	// A key without a name is refused.
	_, err = svc.FlowRun(ctx, connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId: flowID.Bytes(),
		Env:    []*flowv1.FlowRunEnvVar{{Key: " ", Value: "x"}},
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
