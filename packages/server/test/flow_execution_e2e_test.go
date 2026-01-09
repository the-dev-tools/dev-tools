package test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
)

type flowExecutionFixture struct {
	ctx      context.Context
	services testutil.BaseTestServices

	// Additional services needed for execution
	httpService          shttp.HTTPService
	nodeService          sflow.NodeService
	nodeRequestService   sflow.NodeRequestService
	nodeExecutionService sflow.NodeExecutionService
	httpHeaderService    shttp.HttpHeaderService

	userID      idwrap.IDWrap
	workspaceID idwrap.IDWrap

	mockServer *httptest.Server
	serverURL  string
}

func newFlowExecutionFixture(t *testing.T) *flowExecutionFixture {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	services := base.GetBaseServices()

	// Create User
	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	err := services.UserService.CreateUser(ctx, &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err)

	// Create Workspace
	workspaceID, err := services.CreateTempCollection(ctx, userID, "E2E Workspace")
	require.NoError(t, err)

	// Initialize specific services
	httpService := shttp.New(base.Queries, base.Logger())
	nodeService := sflow.NewNodeService(base.Queries)
	nodeRequestService := sflow.NewNodeRequestService(base.Queries)
	nodeExecutionService := sflow.NewNodeExecutionService(base.Queries)
	httpHeaderService := shttp.NewHttpHeaderService(base.Queries)

	// Start Mock Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo Headers
		for k, v := range r.Header {
			for _, val := range v {
				w.Header().Add(k, val)
			}
		}

		// Echo Body
		w.WriteHeader(http.StatusOK)

		// Simple JSON response
		response := map[string]string{
			"status": "success",
			"url":    r.URL.String(),
			"method": r.Method,
		}
		json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(mockServer.Close)

	return &flowExecutionFixture{
		ctx:                  ctx,
		services:             services,
		httpService:          httpService,
		nodeService:          nodeService,
		nodeRequestService:   nodeRequestService,
		nodeExecutionService: nodeExecutionService,
		httpHeaderService:    httpHeaderService,
		userID:               userID,
		workspaceID:          workspaceID,
		mockServer:           mockServer,
		serverURL:            mockServer.URL,
	}
}

func TestFlowExecution_ChainedRequests(t *testing.T) {
	// This test simulates a "Chained Request" scenario:
	// 1. Request A -> Mock Server
	// 2. Request B -> Mock Server

	f := newFlowExecutionFixture(t)

	// 1. Create Flow
	flowID := idwrap.NewNow()
	err := f.services.FlowService.CreateFlow(f.ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: f.workspaceID,
		Name:        "E2E Test Flow",
	})
	require.NoError(t, err)

	// 2. Create Start Node
	startNodeID := idwrap.NewNow()
	err = f.nodeService.CreateNode(f.ctx, mflow.Node{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	// 3. Create Request Node A
	reqNodeAID := idwrap.NewNow()
	httpAID := idwrap.NewNow()

	// Create HTTP entry for A
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          httpAID,
		WorkspaceID: f.workspaceID,
		Name:        "Request A",
		Url:         f.serverURL + "/a",
		Method:      "GET",
	})
	require.NoError(t, err)

	// Create Node A
	err = f.nodeService.CreateNode(f.ctx, mflow.Node{
		ID:        reqNodeAID,
		FlowID:    flowID,
		Name:      "Request A Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Link Node A to HTTP A
	err = f.nodeRequestService.CreateNodeRequest(f.ctx, mflow.NodeRequest{
		FlowNodeID: reqNodeAID,
		HttpID:     &httpAID,
	})
	require.NoError(t, err)

	// 4. Create Request Node B
	reqNodeBID := idwrap.NewNow()
	httpBID := idwrap.NewNow()

	// Create HTTP entry for B
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          httpBID,
		WorkspaceID: f.workspaceID,
		Name:        "Request B",
		Url:         f.serverURL + "/b",
		Method:      "POST",
	})
	require.NoError(t, err)

	// Create Node B
	err = f.nodeService.CreateNode(f.ctx, mflow.Node{
		ID:        reqNodeBID,
		FlowID:    flowID,
		Name:      "Request B Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 400,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Link Node B to HTTP B
	err = f.nodeRequestService.CreateNodeRequest(f.ctx, mflow.NodeRequest{
		FlowNodeID: reqNodeBID,
		HttpID:     &httpBID,
	})
	require.NoError(t, err)

	// 5. Create Edges (Start -> A -> B)
	// Note: Edge service is usually needed here
	edgeService := sflow.NewEdgeService(f.services.Queries) // Assuming we can access Queries

	// Start -> A
	edge1ID := idwrap.NewNow()
	err = edgeService.CreateEdge(f.ctx, mflow.Edge{
		ID:            edge1ID,
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      reqNodeAID,
		SourceHandler: mflow.HandleUnspecified,
	})
	require.NoError(t, err)

	// A -> B
	edge2ID := idwrap.NewNow()
	err = edgeService.CreateEdge(f.ctx, mflow.Edge{
		ID:            edge2ID,
		FlowID:        flowID,
		SourceID:      reqNodeAID,
		TargetID:      reqNodeBID,
		SourceHandler: mflow.HandleUnspecified,
	})
	require.NoError(t, err)

	// 6. Verify Structure via Queries
	// Check that Flow has 3 nodes
	nodes, err := f.nodeService.GetNodesByFlowID(f.ctx, flowID)
	require.NoError(t, err)
	require.Len(t, nodes, 3)

	// Check HTTP linkage
	reqA, err := f.nodeRequestService.GetNodeRequest(f.ctx, reqNodeAID)
	require.NoError(t, err)
	require.Equal(t, httpAID, *reqA.HttpID)

	reqB, err := f.nodeRequestService.GetNodeRequest(f.ctx, reqNodeBID)
	require.NoError(t, err)
	require.Equal(t, httpBID, *reqB.HttpID)

	// Check Edges
	edges, err := edgeService.GetEdgesByFlowID(f.ctx, flowID)
	require.NoError(t, err)
	require.Len(t, edges, 2)
}
