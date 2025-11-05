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
