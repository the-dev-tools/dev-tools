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
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
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

	flowID := idwrap.NewNow()
	require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "example",
	}))

	srv := rflowv2.New(&services.Ws, &flowService, &edgeService, &nodeService, &nodeRequestService)

	ctx := mwauth.CreateAuthedContext(context.Background(), userID)

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
