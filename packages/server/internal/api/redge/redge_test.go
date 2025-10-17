package redge

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/testutil"
	edgev1 "the-dev-tools/spec/dist/buf/go/flow/edge/v1"
)

type edgeServiceTestEnv struct {
	ctx       context.Context
	authedCtx context.Context
	svc       *EdgeServiceRPC
	edgeSvc   sedge.EdgeService
	flowID    idwrap.IDWrap
	sourceID  idwrap.IDWrap
	targetID  idwrap.IDWrap
}

func newEdgeServiceTestEnv(t *testing.T) edgeServiceTestEnv {
	t.Helper()

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	queries := base.Queries
	baseServices := base.GetBaseServices()

	flowSvc := sflow.New(queries)
	nodeSvc := snode.New(queries)
	edgeSvc := sedge.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	flowID := idwrap.NewNow()
	require.NoError(t, flowSvc.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Flow",
	}))

	sourceNodeID := idwrap.NewNow()
	targetNodeID := idwrap.NewNow()
	require.NoError(t, nodeSvc.CreateNode(ctx, mnnode.MNode{
		ID:       sourceNodeID,
		FlowID:   flowID,
		Name:     "Source",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}))
	require.NoError(t, nodeSvc.CreateNode(ctx, mnnode.MNode{
		ID:       targetNodeID,
		FlowID:   flowID,
		Name:     "Target",
		NodeKind: mnnode.NODE_KIND_NO_OP,
	}))

	svc := NewEdgeServiceRPC(base.DB, flowSvc, baseServices.Us, edgeSvc, nodeSvc)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return edgeServiceTestEnv{
		ctx:       ctx,
		authedCtx: authedCtx,
		svc:       svc,
		edgeSvc:   edgeSvc,
		flowID:    flowID,
		sourceID:  sourceNodeID,
		targetID:  targetNodeID,
	}
}

func TestEdgeListConversionFallback(t *testing.T) {
	env := newEdgeServiceTestEnv(t)

	edgeID := idwrap.NewNow()
	require.NoError(t, env.edgeSvc.CreateEdge(env.ctx, edge.Edge{
		ID:            edgeID,
		FlowID:        env.flowID,
		SourceID:      env.sourceID,
		TargetID:      env.targetID,
		SourceHandler: edge.EdgeHandle(99),
		Kind:          99,
	}))

	resp, err := env.svc.EdgeList(env.authedCtx, connect.NewRequest(&edgev1.EdgeListRequest{
		FlowId: env.flowID.Bytes(),
	}))
	require.Error(t, err)
	require.NotNil(t, resp)
	connectErr := &connect.Error{}
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInternal, connectErr.Code())
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, protoHandleFallback, resp.Msg.Items[0].SourceHandle)
	require.Equal(t, protoEdgeKindFallback, resp.Msg.Items[0].Kind)
}

func TestEdgeGetConversionFallback(t *testing.T) {
	env := newEdgeServiceTestEnv(t)

	edgeID := idwrap.NewNow()
	require.NoError(t, env.edgeSvc.CreateEdge(env.ctx, edge.Edge{
		ID:            edgeID,
		FlowID:        env.flowID,
		SourceID:      env.sourceID,
		TargetID:      env.targetID,
		SourceHandler: edge.EdgeHandle(99),
		Kind:          99,
	}))

	resp, err := env.svc.EdgeGet(env.authedCtx, connect.NewRequest(&edgev1.EdgeGetRequest{
		EdgeId: edgeID.Bytes(),
	}))
	require.Error(t, err)
	require.NotNil(t, resp)
	connectErr := &connect.Error{}
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInternal, connectErr.Code())
	require.Equal(t, protoHandleFallback, resp.Msg.SourceHandle)
	require.Equal(t, protoEdgeKindFallback, resp.Msg.Kind)
}

func TestEdgeCreateRejectsInvalidEnums(t *testing.T) {
	env := newEdgeServiceTestEnv(t)

	t.Run("source handle", func(t *testing.T) {
		_, err := env.svc.EdgeCreate(env.authedCtx, connect.NewRequest(&edgev1.EdgeCreateRequest{
			FlowId:       env.flowID.Bytes(),
			SourceId:     env.sourceID.Bytes(),
			TargetId:     env.targetID.Bytes(),
			SourceHandle: edgev1.Handle(99),
			Kind:         edgev1.EdgeKind_EDGE_KIND_NO_OP,
		}))
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("edge kind", func(t *testing.T) {
		_, err := env.svc.EdgeCreate(env.authedCtx, connect.NewRequest(&edgev1.EdgeCreateRequest{
			FlowId:       env.flowID.Bytes(),
			SourceId:     env.sourceID.Bytes(),
			TargetId:     env.targetID.Bytes(),
			SourceHandle: edgev1.Handle_HANDLE_THEN,
			Kind:         edgev1.EdgeKind(99),
		}))
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})
}

func TestEdgeUpdateRejectsInvalidEnums(t *testing.T) {
	env := newEdgeServiceTestEnv(t)

	edgeID := idwrap.NewNow()
	require.NoError(t, env.edgeSvc.CreateEdge(env.ctx, edge.Edge{
		ID:            edgeID,
		FlowID:        env.flowID,
		SourceID:      env.sourceID,
		TargetID:      env.targetID,
		SourceHandler: edge.HandleThen,
		Kind:          int32(edge.EdgeKindNoOp),
	}))

	invalidKind := edgev1.EdgeKind(99)
	resp, err := env.svc.EdgeUpdate(env.authedCtx, connect.NewRequest(&edgev1.EdgeUpdateRequest{
		EdgeId:   edgeID.Bytes(),
		SourceId: env.sourceID.Bytes(),
		TargetId: env.targetID.Bytes(),
		Kind:     &invalidKind,
	}))
	require.Error(t, err)
	require.Nil(t, resp)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	stored, getErr := env.edgeSvc.GetEdge(env.ctx, edgeID)
	require.NoError(t, getErr)
	require.Equal(t, int32(edge.EdgeKindNoOp), stored.Kind)
}
