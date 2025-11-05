package rflowv2_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestFlowServiceV2_NodeLifecycle(t *testing.T) {
	base := testutil.CreateBaseDB(context.Background(), t)
	t.Cleanup(base.Close)

	services := base.GetBaseServices()

	userID := idwrap.NewNow()
	require.NoError(t, services.Us.CreateUser(context.Background(), &muser.User{
		ID:           userID,
		Email:        "user@example.com",
		Password:     []byte("secret"),
		ProviderType: muser.MagicLink,
	}))

	workspaceID := createWorkspaceMembership(t, services.Ws, services.Wus, userID)

	flowService := sflow.New(base.Queries)
	edgeService := sedge.New(base.Queries)
	nodeService := snode.New(base.Queries)
	nodeRequestService := snoderequest.New(base.Queries)
	nodeForService := snodefor.New(base.Queries)
	nodeForEachService := snodeforeach.New(base.Queries)
	nodeConditionService := snodeif.New(base.Queries)
	nodeNoOpService := snodenoop.New(base.Queries)
	nodeJsService := snodejs.New(base.Queries)

	flowID := idwrap.NewNow()
	require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "example",
	}))

	srv := rflowv2.New(
		&services.Ws,
		&flowService,
		&edgeService,
		&nodeService,
		&nodeRequestService,
		&nodeForService,
		&nodeForEachService,
		nodeConditionService,
		&nodeNoOpService,
		&nodeJsService,
	)

	ctx := mwauth.CreateAuthedContext(context.Background(), userID)

	t.Run("flow crud lifecycle", func(t *testing.T) {
		flowID := idwrap.NewNow()

		createReq := connect.NewRequest(&flowv1.FlowCreateRequest{
			Items: []*flowv1.FlowCreate{{
				FlowId: flowID.Bytes(),
				Name:   "new flow",
			}},
		})
		createReq.Header().Set("workspace-id", workspaceID.String())

		_, err := srv.FlowCreate(ctx, createReq)
		require.NoError(t, err)

		created, err := flowService.GetFlow(context.Background(), flowID)
		require.NoError(t, err)
		require.Equal(t, "new flow", created.Name)
		require.Equal(t, workspaceID, created.WorkspaceID)

		updateReq := connect.NewRequest(&flowv1.FlowUpdateRequest{
			Items: []*flowv1.FlowUpdate{{
				FlowId: flowID.Bytes(),
				Name:   ptrString("updated flow"),
			}},
		})
		updateReq.Header().Set("workspace-id", workspaceID.String())

		_, err = srv.FlowUpdate(ctx, updateReq)
		require.NoError(t, err)

		updated, err := flowService.GetFlow(context.Background(), flowID)
		require.NoError(t, err)
		require.Equal(t, "updated flow", updated.Name)

		deleteReq := connect.NewRequest(&flowv1.FlowDeleteRequest{
			Items: []*flowv1.FlowDelete{{FlowId: flowID.Bytes()}},
		})
		deleteReq.Header().Set("workspace-id", workspaceID.String())

		_, err = srv.FlowDelete(ctx, deleteReq)
		require.NoError(t, err)

		_, err = flowService.GetFlow(context.Background(), flowID)
		require.Error(t, err)
	})

	t.Run("flow version collection", func(t *testing.T) {
		parentID := idwrap.NewNow()
		versionID := idwrap.NewNow()

		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:          parentID,
			WorkspaceID: workspaceID,
			Name:        "parent",
		}))

		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:              versionID,
			WorkspaceID:     workspaceID,
			VersionParentID: &parentID,
			Name:            "version",
		}))

		versionReq := connect.NewRequest(&emptypb.Empty{})
		versionReq.Header().Set("flow-id", parentID.String())

		resp, err := srv.FlowVersionCollection(ctx, versionReq)
		require.NoError(t, err)
		require.Len(t, resp.Msg.GetItems(), 1)
		require.Equal(t, versionID.Bytes(), resp.Msg.GetItems()[0].GetFlowVersionId())
		require.Equal(t, parentID.Bytes(), resp.Msg.GetItems()[0].GetFlowId())
	})

	t.Run("base node lifecycle", func(t *testing.T) {
		createReq := &flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{
				{
					FlowId: flowID.Bytes(),
					Name:   "noop",
					Kind:   flowv1.NodeKind_NODE_KIND_NO_OP,
				},
			},
		}

		_, err := srv.NodeCreate(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodesResp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Len(t, nodesResp.Msg.GetItems(), 1)
		require.Equal(t, "noop", nodesResp.Msg.GetItems()[0].GetName())

		nodeID, err := idwrap.NewFromBytes(nodesResp.Msg.GetItems()[0].GetNodeId())
		require.NoError(t, err)

		updateReq := &flowv1.NodeUpdateRequest{
			Items: []*flowv1.NodeUpdate{
				{
					NodeId: nodeID.Bytes(),
					Name:   ptrString("renamed"),
				},
			},
		}

		_, err = srv.NodeUpdate(ctx, connect.NewRequest(updateReq))
		require.NoError(t, err)

		nodesResp, err = srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Equal(t, "renamed", nodesResp.Msg.GetItems()[0].GetName())

		deleteReq := &flowv1.NodeDeleteRequest{
			Items: []*flowv1.NodeDelete{{NodeId: nodeID.Bytes()}},
		}

		_, err = srv.NodeDelete(ctx, connect.NewRequest(deleteReq))
		require.NoError(t, err)

		nodesResp, err = srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Empty(t, nodesResp.Msg.GetItems())
	})

	t.Run("node http lifecycle", func(t *testing.T) {
		httpID1 := idwrap.NewNow()
		httpID2 := idwrap.NewNow()

		require.NoError(t, services.Hs.Create(context.Background(), &mhttp.HTTP{
			ID:          httpID1,
			WorkspaceID: workspaceID,
			Name:        "http-1",
			Url:         "https://example.com/1",
			Method:      "GET",
		}))

		require.NoError(t, services.Hs.Create(context.Background(), &mhttp.HTTP{
			ID:          httpID2,
			WorkspaceID: workspaceID,
			Name:        "http-2",
			Url:         "https://example.com/2",
			Method:      "POST",
		}))

		createReq := &flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{
				{
					FlowId: flowID.Bytes(),
					Name:   "request",
					Kind:   flowv1.NodeKind_NODE_KIND_REQUEST,
				},
			},
		}

		_, err := srv.NodeCreate(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodesResp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		var nodeID idwrap.IDWrap
		for _, item := range nodesResp.Msg.GetItems() {
			if item.GetName() == "request" {
				nodeID, err = idwrap.NewFromBytes(item.GetNodeId())
				require.NoError(t, err)
				break
			}
		}
		require.NotEqual(t, idwrap.IDWrap{}, nodeID)

		_, err = srv.NodeHttpCreate(ctx, connect.NewRequest(&flowv1.NodeHttpCreateRequest{
			Items: []*flowv1.NodeHttpCreate{{
				NodeId: nodeID.Bytes(),
				HttpId: httpID1.Bytes(),
			}},
		}))
		require.NoError(t, err)

		reqModel, err := nodeRequestService.GetNodeRequest(context.Background(), nodeID)
		require.NoError(t, err)
		require.NotNil(t, reqModel)
		require.Equal(t, httpID1, reqModel.HttpID)

		_, err = srv.NodeHttpUpdate(ctx, connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
			Items: []*flowv1.NodeHttpUpdate{{
				NodeId: nodeID.Bytes(),
				HttpId: httpID2.Bytes(),
			}},
		}))
		require.NoError(t, err)

		reqModel, err = nodeRequestService.GetNodeRequest(context.Background(), nodeID)
		require.NoError(t, err)
		require.NotNil(t, reqModel)
		require.Equal(t, httpID2, reqModel.HttpID)

		_, err = srv.NodeHttpDelete(ctx, connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
			Items: []*flowv1.NodeHttpDelete{{NodeId: nodeID.Bytes()}},
		}))
		require.NoError(t, err)

		reqModel, err = nodeRequestService.GetNodeRequest(context.Background(), nodeID)
		require.NoError(t, err)
		require.Nil(t, reqModel)
	})

	t.Run("node for lifecycle", func(t *testing.T) {
		createReq := &flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{{
				FlowId: flowID.Bytes(),
				Name:   "for-node",
				Kind:   flowv1.NodeKind_NODE_KIND_FOR,
			}},
		}

		_, err := srv.NodeCreate(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodesResp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		var nodeID idwrap.IDWrap
		for _, item := range nodesResp.Msg.GetItems() {
			if item.GetName() == "for-node" {
				nodeID, err = idwrap.NewFromBytes(item.GetNodeId())
				require.NoError(t, err)
				break
			}
		}
		require.NotEqual(t, idwrap.IDWrap{}, nodeID)

		_, err = srv.NodeForCreate(ctx, connect.NewRequest(&flowv1.NodeForCreateRequest{
			Items: []*flowv1.NodeForCreate{{
				NodeId:     nodeID.Bytes(),
				Iterations: 5,
				Condition:  "x < 5",
			}},
		}))
		require.NoError(t, err)

		forModel, err := nodeForService.GetNodeFor(context.Background(), nodeID)
		require.NoError(t, err)
		require.NotNil(t, forModel)
		require.EqualValues(t, 5, forModel.IterCount)
		require.Equal(t, "x < 5", forModel.Condition.Comparisons.Expression)

		_, err = srv.NodeForUpdate(ctx, connect.NewRequest(&flowv1.NodeForUpdateRequest{
			Items: []*flowv1.NodeForUpdate{{
				NodeId:     nodeID.Bytes(),
				Iterations: ptrInt32(7),
				Condition:  ptrString("x < 7"),
			}},
		}))
		require.NoError(t, err)

		forModel, err = nodeForService.GetNodeFor(context.Background(), nodeID)
		require.NoError(t, err)
		require.EqualValues(t, 7, forModel.IterCount)
		require.Equal(t, "x < 7", forModel.Condition.Comparisons.Expression)

		_, err = srv.NodeForDelete(ctx, connect.NewRequest(&flowv1.NodeForDeleteRequest{
			Items: []*flowv1.NodeForDelete{{NodeId: nodeID.Bytes()}},
		}))
		require.NoError(t, err)

		_, err = nodeForService.GetNodeFor(context.Background(), nodeID)
		require.Error(t, err)
	})

	t.Run("node foreach lifecycle", func(t *testing.T) {
		createReq := &flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{{
				FlowId: flowID.Bytes(),
				Name:   "foreach-node",
				Kind:   flowv1.NodeKind_NODE_KIND_FOR_EACH,
			}},
		}

		_, err := srv.NodeCreate(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodeID := findNodeIDByName(t, srv, ctx, "foreach-node")

		_, err = srv.NodeForEachCreate(ctx, connect.NewRequest(&flowv1.NodeForEachCreateRequest{
			Items: []*flowv1.NodeForEachCreate{{
				NodeId:    nodeID.Bytes(),
				Path:      "items",
				Condition: "len(items) > 0",
			}},
		}))
		require.NoError(t, err)

		feModel, err := nodeForEachService.GetNodeForEach(context.Background(), nodeID)
		require.NoError(t, err)
		require.Equal(t, "items", feModel.IterExpression)
		require.Equal(t, "len(items) > 0", feModel.Condition.Comparisons.Expression)

		_, err = srv.NodeForEachUpdate(ctx, connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
			Items: []*flowv1.NodeForEachUpdate{{
				NodeId:    nodeID.Bytes(),
				Path:      ptrString("items2"),
				Condition: ptrString("len(items2) > 0"),
			}},
		}))
		require.NoError(t, err)

		feModel, err = nodeForEachService.GetNodeForEach(context.Background(), nodeID)
		require.NoError(t, err)
		require.Equal(t, "items2", feModel.IterExpression)
		require.Equal(t, "len(items2) > 0", feModel.Condition.Comparisons.Expression)

		_, err = srv.NodeForEachDelete(ctx, connect.NewRequest(&flowv1.NodeForEachDeleteRequest{
			Items: []*flowv1.NodeForEachDelete{{NodeId: nodeID.Bytes()}},
		}))
		require.NoError(t, err)

		_, err = nodeForEachService.GetNodeForEach(context.Background(), nodeID)
		require.Error(t, err)
	})

	t.Run("node condition lifecycle", func(t *testing.T) {
		createReq := &flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{{
				FlowId: flowID.Bytes(),
				Name:   "condition-node",
				Kind:   flowv1.NodeKind_NODE_KIND_CONDITION,
			}},
		}

		_, err := srv.NodeCreate(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodeID := findNodeIDByName(t, srv, ctx, "condition-node")

		_, err = srv.NodeConditionCreate(ctx, connect.NewRequest(&flowv1.NodeConditionCreateRequest{
			Items: []*flowv1.NodeConditionCreate{{
				NodeId:    nodeID.Bytes(),
				Condition: "x == 1",
			}},
		}))
		require.NoError(t, err)

		condModel, err := nodeConditionService.GetNodeIf(context.Background(), nodeID)
		require.NoError(t, err)
		require.Equal(t, "x == 1", condModel.Condition.Comparisons.Expression)

		_, err = srv.NodeConditionUpdate(ctx, connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
			Items: []*flowv1.NodeConditionUpdate{{
				NodeId:    nodeID.Bytes(),
				Condition: ptrString("x == 2"),
			}},
		}))
		require.NoError(t, err)

		condModel, err = nodeConditionService.GetNodeIf(context.Background(), nodeID)
		require.NoError(t, err)
		require.Equal(t, "x == 2", condModel.Condition.Comparisons.Expression)

		_, err = srv.NodeConditionDelete(ctx, connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
			Items: []*flowv1.NodeConditionDelete{{NodeId: nodeID.Bytes()}},
		}))
		require.NoError(t, err)

		_, err = nodeConditionService.GetNodeIf(context.Background(), nodeID)
		require.Error(t, err)
	})

	t.Run("node js lifecycle", func(t *testing.T) {
		createReq := &flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{{
				FlowId: flowID.Bytes(),
				Name:   "js-node",
				Kind:   flowv1.NodeKind_NODE_KIND_JS,
			}},
		}

		_, err := srv.NodeCreate(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodeID := findNodeIDByName(t, srv, ctx, "js-node")

		_, err = srv.NodeJsCreate(ctx, connect.NewRequest(&flowv1.NodeJsCreateRequest{
			Items: []*flowv1.NodeJsCreate{{
				NodeId: nodeID.Bytes(),
				Code:   "console.log('hello')",
			}},
		}))
		require.NoError(t, err)

		jsModel, err := nodeJsService.GetNodeJS(context.Background(), nodeID)
		require.NoError(t, err)
		require.Equal(t, "console.log('hello')", string(jsModel.Code))

		_, err = srv.NodeJsUpdate(ctx, connect.NewRequest(&flowv1.NodeJsUpdateRequest{
			Items: []*flowv1.NodeJsUpdate{{
				NodeId: nodeID.Bytes(),
				Code:   ptrString("console.log('updated')"),
			}},
		}))
		require.NoError(t, err)

		jsModel, err = nodeJsService.GetNodeJS(context.Background(), nodeID)
		require.NoError(t, err)
		require.Equal(t, "console.log('updated')", string(jsModel.Code))

		_, err = srv.NodeJsDelete(ctx, connect.NewRequest(&flowv1.NodeJsDeleteRequest{
			Items: []*flowv1.NodeJsDelete{{NodeId: nodeID.Bytes()}},
		}))
		require.NoError(t, err)

		_, err = nodeJsService.GetNodeJS(context.Background(), nodeID)
		require.Error(t, err)
	})

	t.Run("edge lifecycle", func(t *testing.T) {
		createReq := &flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{
				{FlowId: flowID.Bytes(), Name: "edge-source", Kind: flowv1.NodeKind_NODE_KIND_NO_OP},
				{FlowId: flowID.Bytes(), Name: "edge-target", Kind: flowv1.NodeKind_NODE_KIND_NO_OP},
			},
		}

		_, err := srv.NodeCreate(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		sourceID := findNodeIDByName(t, srv, ctx, "edge-source")
		targetID := findNodeIDByName(t, srv, ctx, "edge-target")

		_, err = srv.EdgeCreate(ctx, connect.NewRequest(&flowv1.EdgeCreateRequest{
			Items: []*flowv1.EdgeCreate{{
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.Handle_HANDLE_THEN,
				Kind:         flowv1.EdgeKind_EDGE_KIND_NO_OP,
			}},
		}))
		require.NoError(t, err)

		edges, err := edgeService.GetEdgesByFlowID(context.Background(), flowID)
		require.NoError(t, err)
		require.Len(t, edges, 1)

		edgeID := edges[0].ID

		// create new target for update
		_, err = srv.NodeCreate(ctx, connect.NewRequest(&flowv1.NodeCreateRequest{
			Items: []*flowv1.NodeCreate{{
				FlowId: flowID.Bytes(),
				Name:   "edge-target-2",
				Kind:   flowv1.NodeKind_NODE_KIND_NO_OP,
			}},
		}))
		require.NoError(t, err)

		newTargetID := findNodeIDByName(t, srv, ctx, "edge-target-2")

		_, err = srv.EdgeUpdate(ctx, connect.NewRequest(&flowv1.EdgeUpdateRequest{
			Items: []*flowv1.EdgeUpdate{{
				EdgeId:       edgeID.Bytes(),
				TargetId:     newTargetID.Bytes(),
				SourceHandle: ptrHandle(flowv1.Handle_HANDLE_ELSE),
			}},
		}))
		require.NoError(t, err)

		edgeModel, err := edgeService.GetEdge(context.Background(), edgeID)
		require.NoError(t, err)
		require.Equal(t, newTargetID, edgeModel.TargetID)
		require.Equal(t, edge.EdgeHandle(flowv1.Handle_HANDLE_ELSE), edgeModel.SourceHandler)

		_, err = srv.EdgeDelete(ctx, connect.NewRequest(&flowv1.EdgeDeleteRequest{
			Items: []*flowv1.EdgeDelete{{EdgeId: edgeID.Bytes()}},
		}))
		require.NoError(t, err)

		_, err = edgeService.GetEdge(context.Background(), edgeID)
		require.Error(t, err)
	})
}

func createWorkspaceMembership(t *testing.T, ws sworkspace.WorkspaceService, wus sworkspacesusers.WorkspaceUserService, userID idwrap.IDWrap) idwrap.IDWrap {
	t.Helper()

	wsID := idwrap.NewNow()
	envID := idwrap.NewNow()

	err := ws.Create(context.Background(), &mworkspace.Workspace{
		ID:        wsID,
		Name:      "workspace",
		Updated:   dbtime.DBNow(),
		ActiveEnv: envID,
		GlobalEnv: envID,
	})
	require.NoError(t, err)

	member := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}
	require.NoError(t, wus.CreateWorkspaceUser(context.Background(), member))
	require.NoError(t, ws.AutoLinkWorkspaceToUserList(context.Background(), wsID, userID))
	return wsID
}

func ptrString(s string) *string {
	return &s
}

func ptrInt32(v int32) *int32 {
	return &v
}

func ptrHandle(v flowv1.Handle) *flowv1.Handle {
	return &v
}

func findNodeIDByName(t *testing.T, srv *rflowv2.FlowServiceV2RPC, ctx context.Context, name string) idwrap.IDWrap {
	resp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	for _, item := range resp.Msg.GetItems() {
		if item.GetName() == name {
			id, err := idwrap.NewFromBytes(item.GetNodeId())
			require.NoError(t, err)
			return id
		}
	}
	t.Fatalf("node %s not found", name)
	return idwrap.IDWrap{}
}
