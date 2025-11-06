package rflowv2

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"strings"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
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
	flowVariableService := services.Fvs

	flowID := idwrap.NewNow()
	require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "example",
	}))

	startNodeID := idwrap.NewNow()
	require.NoError(t, nodeService.CreateNode(context.Background(), mnnode.MNode{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}))
	require.NoError(t, nodeNoOpService.CreateNodeNoop(context.Background(), mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}))

	nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()
	t.Cleanup(nodeStream.Shutdown)
	edgeStream := memory.NewInMemorySyncStreamer[EdgeTopic, EdgeEvent]()
	t.Cleanup(edgeStream.Shutdown)
	flowVarStream := memory.NewInMemorySyncStreamer[FlowVariableTopic, FlowVariableEvent]()
	t.Cleanup(flowVarStream.Shutdown)
	flowVersionStream := memory.NewInMemorySyncStreamer[FlowVersionTopic, FlowVersionEvent]()
	t.Cleanup(flowVersionStream.Shutdown)
	noopStream := memory.NewInMemorySyncStreamer[NoOpTopic, NoOpEvent]()
	t.Cleanup(noopStream.Shutdown)
	forStream := memory.NewInMemorySyncStreamer[ForTopic, ForEvent]()
	t.Cleanup(forStream.Shutdown)

	srv := New(
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
		&flowVariableService,
		&services.Hs,
		services.Hh,
		services.Hsp,
		services.Hbf,
		services.Hbu,
		services.Has,
		nodeStream,
		edgeStream,
		flowVarStream,
		flowVersionStream,
		noopStream,
		forStream,
	)

	ctx := mwauth.CreateAuthedContext(context.Background(), userID)

	t.Run("flow crud lifecycle", func(t *testing.T) {
		flowID := idwrap.NewNow()

		createReq := connect.NewRequest(&flowv1.FlowInsertRequest{
			Items: []*flowv1.FlowInsert{{
				FlowId: flowID.Bytes(),
				Name:   "new flow",
			}},
		})
		createReq.Header().Set("workspace-id", workspaceID.String())

		_, err := srv.FlowInsert(ctx, createReq)
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

	t.Run("flow run validates access", func(t *testing.T) {
		runReq := connect.NewRequest(&flowv1.FlowRunRequest{
			FlowId: flowID.Bytes(),
		})

		_, err := srv.FlowRun(ctx, runReq)
		require.NoError(t, err)

		_, err = srv.FlowRun(ctx, connect.NewRequest(&flowv1.FlowRunRequest{}))
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
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
		createReq := &flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{
				{
					FlowId: flowID.Bytes(),
					Name:   "noop",
					Kind:   flowv1.NodeKind_NODE_KIND_NO_OP,
				},
			},
		}

		_, err := srv.NodeInsert(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodesResp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		nodes := nonStartNodes(nodesResp.Msg.GetItems())
		require.Len(t, nodes, 1)
		require.Equal(t, "noop", nodes[0].GetName())

		nodeID, err := idwrap.NewFromBytes(nodes[0].GetNodeId())
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
		nodes = nonStartNodes(nodesResp.Msg.GetItems())
		require.Equal(t, "renamed", nodes[0].GetName())

		deleteReq := &flowv1.NodeDeleteRequest{
			Items: []*flowv1.NodeDelete{{NodeId: nodeID.Bytes()}},
		}

		_, err = srv.NodeDelete(ctx, connect.NewRequest(deleteReq))
		require.NoError(t, err)

		nodesResp, err = srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Empty(t, nonStartNodes(nodesResp.Msg.GetItems()))
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

		createReq := &flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{
				{
					FlowId: flowID.Bytes(),
					Name:   "request",
					Kind:   flowv1.NodeKind_NODE_KIND_REQUEST,
				},
			},
		}

		_, err := srv.NodeInsert(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodesResp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		var nodeID idwrap.IDWrap
		for _, item := range nonStartNodes(nodesResp.Msg.GetItems()) {
			if item.GetName() == "request" {
				nodeID, err = idwrap.NewFromBytes(item.GetNodeId())
				require.NoError(t, err)
				break
			}
		}
		require.NotEqual(t, idwrap.IDWrap{}, nodeID)

		_, err = srv.NodeHttpInsert(ctx, connect.NewRequest(&flowv1.NodeHttpInsertRequest{
			Items: []*flowv1.NodeHttpInsert{{
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
		createReq := &flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{{
				FlowId: flowID.Bytes(),
				Name:   "for-node",
				Kind:   flowv1.NodeKind_NODE_KIND_FOR,
			}},
		}

		_, err := srv.NodeInsert(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodesResp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		var nodeID idwrap.IDWrap
		for _, item := range nonStartNodes(nodesResp.Msg.GetItems()) {
			if item.GetName() == "for-node" {
				nodeID, err = idwrap.NewFromBytes(item.GetNodeId())
				require.NoError(t, err)
				break
			}
		}
		require.NotEqual(t, idwrap.IDWrap{}, nodeID)

		_, err = srv.NodeForInsert(ctx, connect.NewRequest(&flowv1.NodeForInsertRequest{
			Items: []*flowv1.NodeForInsert{{
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
		createReq := &flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{{
				FlowId: flowID.Bytes(),
				Name:   "foreach-node",
				Kind:   flowv1.NodeKind_NODE_KIND_FOR_EACH,
			}},
		}

		_, err := srv.NodeInsert(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodeID := findNodeIDByName(t, srv, ctx, "foreach-node")

		_, err = srv.NodeForEachInsert(ctx, connect.NewRequest(&flowv1.NodeForEachInsertRequest{
			Items: []*flowv1.NodeForEachInsert{{
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
		createReq := &flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{{
				FlowId: flowID.Bytes(),
				Name:   "condition-node",
				Kind:   flowv1.NodeKind_NODE_KIND_CONDITION,
			}},
		}

		_, err := srv.NodeInsert(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodeID := findNodeIDByName(t, srv, ctx, "condition-node")

		_, err = srv.NodeConditionInsert(ctx, connect.NewRequest(&flowv1.NodeConditionInsertRequest{
			Items: []*flowv1.NodeConditionInsert{{
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
		createReq := &flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{{
				FlowId: flowID.Bytes(),
				Name:   "js-node",
				Kind:   flowv1.NodeKind_NODE_KIND_JS,
			}},
		}

		_, err := srv.NodeInsert(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		nodeID := findNodeIDByName(t, srv, ctx, "js-node")

		_, err = srv.NodeJsInsert(ctx, connect.NewRequest(&flowv1.NodeJsInsertRequest{
			Items: []*flowv1.NodeJsInsert{{
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
		createReq := &flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{
				{FlowId: flowID.Bytes(), Name: "edge-source", Kind: flowv1.NodeKind_NODE_KIND_NO_OP},
				{FlowId: flowID.Bytes(), Name: "edge-target", Kind: flowv1.NodeKind_NODE_KIND_NO_OP},
			},
		}

		_, err := srv.NodeInsert(ctx, connect.NewRequest(createReq))
		require.NoError(t, err)

		sourceID := findNodeIDByName(t, srv, ctx, "edge-source")
		targetID := findNodeIDByName(t, srv, ctx, "edge-target")

		_, err = srv.EdgeInsert(ctx, connect.NewRequest(&flowv1.EdgeInsertRequest{
			Items: []*flowv1.EdgeInsert{{
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
		_, err = srv.NodeInsert(ctx, connect.NewRequest(&flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{{
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

	t.Run("node sync streams snapshot and updates", func(t *testing.T) {
		nodeUserID := idwrap.NewNow()
		require.NoError(t, services.Us.CreateUser(context.Background(), &muser.User{
			ID:           nodeUserID,
			Email:        "node-sync@example.com",
			Password:     []byte("secret"),
			ProviderType: muser.MagicLink,
		}))

		nodeWorkspaceID := createWorkspaceMembership(t, services.Ws, services.Wus, nodeUserID)

		nodeFlowID := idwrap.NewNow()
		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:          nodeFlowID,
			WorkspaceID: nodeWorkspaceID,
			Name:        "node-sync-flow",
		}))

		startNodeID := idwrap.NewNow()
		require.NoError(t, nodeService.CreateNode(context.Background(), mnnode.MNode{
			ID:        startNodeID,
			FlowID:    nodeFlowID,
			Name:      "Start",
			NodeKind:  mnnode.NODE_KIND_NO_OP,
			PositionX: 0,
			PositionY: 0,
		}))
		require.NoError(t, nodeNoOpService.CreateNodeNoop(context.Background(), mnnoop.NoopNode{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		}))

		syncCtx := mwauth.CreateAuthedContext(context.Background(), nodeUserID)

		syncNodeID := idwrap.NewNow()
		_, err := srv.NodeInsert(syncCtx, connect.NewRequest(&flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{{
				FlowId: nodeFlowID.Bytes(),
				NodeId: syncNodeID.Bytes(),
				Name:   "sync-node",
				Kind:   flowv1.NodeKind_NODE_KIND_NO_OP,
			}},
		}))
		require.NoError(t, err)

		received := make(chan *flowv1.NodeSyncResponse, 4)
		ctxSync, cancelSync := context.WithCancel(syncCtx)
		defer cancelSync()

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.streamNodeSync(ctxSync, func(resp *flowv1.NodeSyncResponse) error {
				received <- resp
				return nil
			})
		}()

		var first *flowv1.NodeSyncResponse
		select {
		case first = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for node sync snapshot")
		}

		require.Len(t, first.GetItems(), 1)
		firstValue := first.GetItems()[0].GetValue()
		require.NotNil(t, firstValue)
		require.Equal(t, flowv1.NodeSync_ValueUnion_KIND_INSERT, firstValue.GetKind())
		firstCreate := firstValue.GetCreate()
		require.NotNil(t, firstCreate)
		require.Equal(t, syncNodeID.Bytes(), firstCreate.GetNodeId())
		require.Equal(t, "sync-node", firstCreate.GetName())
		require.Equal(t, nodeFlowID.Bytes(), firstCreate.GetFlowId())

		newName := "sync-node-renamed"
		_, err = srv.NodeUpdate(syncCtx, connect.NewRequest(&flowv1.NodeUpdateRequest{
			Items: []*flowv1.NodeUpdate{{
				NodeId: syncNodeID.Bytes(),
				Name:   ptrString(newName),
			}},
		}))
		require.NoError(t, err)

		var update *flowv1.NodeSyncResponse
		select {
		case update = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for node sync update")
		}

		require.Len(t, update.GetItems(), 1)
		updateValue := update.GetItems()[0].GetValue()
		require.NotNil(t, updateValue)
		require.Equal(t, flowv1.NodeSync_ValueUnion_KIND_UPDATE, updateValue.GetKind())
		updateMsg := updateValue.GetUpdate()
		require.NotNil(t, updateMsg)
		require.Equal(t, syncNodeID.Bytes(), updateMsg.GetNodeId())
		require.Equal(t, newName, updateMsg.GetName())

		cancelSync()
		if err := <-errCh; err != nil {
			require.ErrorIs(t, err, context.Canceled)
		}
	})

	t.Run("edge sync streams snapshot and updates", func(t *testing.T) {
		edgeUserID := idwrap.NewNow()
		require.NoError(t, services.Us.CreateUser(context.Background(), &muser.User{
			ID:           edgeUserID,
			Email:        "edge-sync@example.com",
			Password:     []byte("secret"),
			ProviderType: muser.MagicLink,
		}))

		edgeWorkspaceID := createWorkspaceMembership(t, services.Ws, services.Wus, edgeUserID)

		edgeFlowID := idwrap.NewNow()
		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:          edgeFlowID,
			WorkspaceID: edgeWorkspaceID,
			Name:        "edge-sync-flow",
		}))

		startNodeID := idwrap.NewNow()
		require.NoError(t, nodeService.CreateNode(context.Background(), mnnode.MNode{
			ID:        startNodeID,
			FlowID:    edgeFlowID,
			Name:      "Start",
			NodeKind:  mnnode.NODE_KIND_NO_OP,
			PositionX: 0,
			PositionY: 0,
		}))
		require.NoError(t, nodeNoOpService.CreateNodeNoop(context.Background(), mnnoop.NoopNode{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		}))

		edgeCtx := mwauth.CreateAuthedContext(context.Background(), edgeUserID)

		sourceNodeID := idwrap.NewNow()
		targetNodeID := idwrap.NewNow()
		_, err := srv.NodeInsert(edgeCtx, connect.NewRequest(&flowv1.NodeInsertRequest{
			Items: []*flowv1.NodeInsert{
				{
					FlowId: edgeFlowID.Bytes(),
					NodeId: sourceNodeID.Bytes(),
					Name:   "edge-sync-source",
					Kind:   flowv1.NodeKind_NODE_KIND_NO_OP,
				},
				{
					FlowId: edgeFlowID.Bytes(),
					NodeId: targetNodeID.Bytes(),
					Name:   "edge-sync-target",
					Kind:   flowv1.NodeKind_NODE_KIND_NO_OP,
				},
			},
		}))
		require.NoError(t, err)

		edgeID := idwrap.NewNow()
		_, err = srv.EdgeInsert(edgeCtx, connect.NewRequest(&flowv1.EdgeInsertRequest{
			Items: []*flowv1.EdgeInsert{{
				EdgeId:       edgeID.Bytes(),
				FlowId:       edgeFlowID.Bytes(),
				SourceId:     sourceNodeID.Bytes(),
				TargetId:     targetNodeID.Bytes(),
				SourceHandle: flowv1.Handle_HANDLE_THEN,
				Kind:         flowv1.EdgeKind_EDGE_KIND_NO_OP,
			}},
		}))
		require.NoError(t, err)

		edgeReceived := make(chan *flowv1.EdgeSyncResponse, 6)
		ctxEdge, cancelEdge := context.WithCancel(edgeCtx)
		defer cancelEdge()

		edgeErrCh := make(chan error, 1)
		go func() {
			edgeErrCh <- srv.streamEdgeSync(ctxEdge, func(resp *flowv1.EdgeSyncResponse) error {
				edgeReceived <- resp
				return nil
			})
		}()

		var create *flowv1.EdgeSyncResponse
		select {
		case create = <-edgeReceived:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for edge sync snapshot")
		}

		require.Len(t, create.GetItems(), 1)
		createValue := create.GetItems()[0].GetValue()
		require.NotNil(t, createValue)
		require.Equal(t, flowv1.EdgeSync_ValueUnion_KIND_INSERT, createValue.GetKind())
		createMsg := createValue.GetCreate()
		require.NotNil(t, createMsg)
		require.Equal(t, edgeID.Bytes(), createMsg.GetEdgeId())
		require.Equal(t, edgeFlowID.Bytes(), createMsg.GetFlowId())

		_, err = srv.EdgeUpdate(edgeCtx, connect.NewRequest(&flowv1.EdgeUpdateRequest{
			Items: []*flowv1.EdgeUpdate{{
				EdgeId:       edgeID.Bytes(),
				SourceHandle: ptrHandle(flowv1.Handle_HANDLE_ELSE),
			}},
		}))
		require.NoError(t, err)

		var update *flowv1.EdgeSyncResponse
		select {
		case update = <-edgeReceived:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for edge sync update")
		}

		require.Len(t, update.GetItems(), 1)
		updateValue := update.GetItems()[0].GetValue()
		require.NotNil(t, updateValue)
		require.Equal(t, flowv1.EdgeSync_ValueUnion_KIND_UPDATE, updateValue.GetKind())
		updateMsg := updateValue.GetUpdate()
		require.NotNil(t, updateMsg)
		require.Equal(t, edgeID.Bytes(), updateMsg.GetEdgeId())
		require.Equal(t, flowv1.Handle_HANDLE_ELSE, updateMsg.GetSourceHandle())

		_, err = srv.EdgeDelete(edgeCtx, connect.NewRequest(&flowv1.EdgeDeleteRequest{
			Items: []*flowv1.EdgeDelete{{
				EdgeId: edgeID.Bytes(),
			}},
		}))
		require.NoError(t, err)

		var deleteResp *flowv1.EdgeSyncResponse
		select {
		case deleteResp = <-edgeReceived:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for edge sync delete")
		}

		require.Len(t, deleteResp.GetItems(), 1)
		deleteValue := deleteResp.GetItems()[0].GetValue()
		require.NotNil(t, deleteValue)
		require.Equal(t, flowv1.EdgeSync_ValueUnion_KIND_DELETE, deleteValue.GetKind())
		deleteMsg := deleteValue.GetDelete()
		require.NotNil(t, deleteMsg)
		require.Equal(t, edgeID.Bytes(), deleteMsg.GetEdgeId())

		cancelEdge()
		if err := <-edgeErrCh; err != nil {
			require.ErrorIs(t, err, context.Canceled)
		}
	})

	t.Run("flow variable lifecycle", func(t *testing.T) {
		newCollectionReq := func() *connect.Request[emptypb.Empty] {
			req := connect.NewRequest(&emptypb.Empty{})
			req.Header().Set("flow-id", flowID.String())
			return req
		}

		resp, err := srv.FlowVariableCollection(ctx, newCollectionReq())
		require.NoError(t, err)
		require.Empty(t, resp.Msg.GetItems())

		_, err = srv.FlowVariableInsert(ctx, connect.NewRequest(&flowv1.FlowVariableInsertRequest{
			Items: []*flowv1.FlowVariableInsert{{
				FlowId:      flowID.Bytes(),
				Key:         "API_KEY",
				Value:       "secret",
				Enabled:     true,
				Description: "initial",
			}},
		}))
		require.NoError(t, err)

		resp, err = srv.FlowVariableCollection(ctx, newCollectionReq())
		require.NoError(t, err)
		require.Len(t, resp.Msg.GetItems(), 1)

		variable := resp.Msg.GetItems()[0]
		require.Equal(t, "API_KEY", variable.GetKey())
		require.True(t, variable.GetEnabled())
		require.Equal(t, "secret", variable.GetValue())
		require.Equal(t, "initial", variable.GetDescription())

		_, err = srv.FlowVariableUpdate(ctx, connect.NewRequest(&flowv1.FlowVariableUpdateRequest{
			Items: []*flowv1.FlowVariableUpdate{{
				FlowVariableId: variable.GetFlowVariableId(),
				Key:            ptrString("UPDATED_KEY"),
				Value:          ptrString("updated"),
				Enabled:        ptrBool(false),
				Description:    ptrString("updated"),
			}},
		}))
		require.NoError(t, err)

		resp, err = srv.FlowVariableCollection(ctx, newCollectionReq())
		require.NoError(t, err)
		require.Len(t, resp.Msg.GetItems(), 1)

		updated := resp.Msg.GetItems()[0]
		require.Equal(t, "UPDATED_KEY", updated.GetKey())
		require.False(t, updated.GetEnabled())
		require.Equal(t, "updated", updated.GetValue())
		require.Equal(t, "updated", updated.GetDescription())

		_, err = srv.FlowVariableDelete(ctx, connect.NewRequest(&flowv1.FlowVariableDeleteRequest{
			Items: []*flowv1.FlowVariableDelete{{
				FlowVariableId: updated.GetFlowVariableId(),
			}},
		}))
		require.NoError(t, err)

		resp, err = srv.FlowVariableCollection(ctx, newCollectionReq())
		require.NoError(t, err)
		require.Empty(t, resp.Msg.GetItems())
	})

	t.Run("flow variable sync streams snapshot and updates", func(t *testing.T) {
		varUserID := idwrap.NewNow()
		require.NoError(t, services.Us.CreateUser(context.Background(), &muser.User{
			ID:           varUserID,
			Email:        "flowvar-sync@example.com",
			Password:     []byte("secret"),
			ProviderType: muser.MagicLink,
		}))

		varWorkspaceID := createWorkspaceMembership(t, services.Ws, services.Wus, varUserID)

		varFlowID := idwrap.NewNow()
		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:          varFlowID,
			WorkspaceID: varWorkspaceID,
			Name:        "flow-var-sync",
		}))

		varCtx := mwauth.CreateAuthedContext(context.Background(), varUserID)

		varID1 := idwrap.NewNow()
		_, err := srv.FlowVariableInsert(varCtx, connect.NewRequest(&flowv1.FlowVariableInsertRequest{
			Items: []*flowv1.FlowVariableInsert{
				{
					FlowId:         varFlowID.Bytes(),
					FlowVariableId: varID1.Bytes(),
					Key:            "API_KEY",
					Value:          "secret",
					Enabled:        true,
				},
			},
		}))
		require.NoError(t, err)

		received := make(chan *flowv1.FlowVariableSyncResponse, 6)
		ctxSync, cancelSync := context.WithCancel(varCtx)
		t.Cleanup(cancelSync)

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.streamFlowVariableSync(ctxSync, func(resp *flowv1.FlowVariableSyncResponse) error {
				received <- resp
				return nil
			})
		}()

		var snapshot *flowv1.FlowVariableSyncResponse
		select {
		case snapshot = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for flow variable snapshot")
		}

		require.Len(t, snapshot.GetItems(), 1)
		snapshotValue := snapshot.GetItems()[0].GetValue()
		require.NotNil(t, snapshotValue)
		require.Equal(t, flowv1.FlowVariableSync_ValueUnion_KIND_INSERT, snapshotValue.GetKind())
		snapshotCreate := snapshotValue.GetCreate()
		require.NotNil(t, snapshotCreate)
		require.Equal(t, varID1.Bytes(), snapshotCreate.GetFlowVariableId())
		require.Equal(t, varFlowID.Bytes(), snapshotCreate.GetFlowId())
		require.Equal(t, "API_KEY", snapshotCreate.GetKey())

		varID2 := idwrap.NewNow()
		_, err = srv.FlowVariableInsert(varCtx, connect.NewRequest(&flowv1.FlowVariableInsertRequest{
			Items: []*flowv1.FlowVariableInsert{
				{
					FlowId:         varFlowID.Bytes(),
					FlowVariableId: varID2.Bytes(),
					Key:            "SECOND",
					Value:          "value",
					Enabled:        false,
				},
			},
		}))
		require.NoError(t, err)

		var createEvent *flowv1.FlowVariableSyncResponse
		select {
		case createEvent = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for flow variable create event")
		}

		require.Len(t, createEvent.GetItems(), 1)
		createValue := createEvent.GetItems()[0].GetValue()
		require.NotNil(t, createValue)
		require.Equal(t, flowv1.FlowVariableSync_ValueUnion_KIND_INSERT, createValue.GetKind())
		require.Equal(t, varID2.Bytes(), createValue.GetCreate().GetFlowVariableId())

		_, err = srv.FlowVariableUpdate(varCtx, connect.NewRequest(&flowv1.FlowVariableUpdateRequest{
			Items: []*flowv1.FlowVariableUpdate{
				{
					FlowVariableId: varID2.Bytes(),
					Key:            ptrString("RENAMED"),
					Enabled:        ptrBool(true),
				},
			},
		}))
		require.NoError(t, err)

		var updateEvent *flowv1.FlowVariableSyncResponse
		select {
		case updateEvent = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for flow variable update event")
		}

		require.Len(t, updateEvent.GetItems(), 1)
		updateValue := updateEvent.GetItems()[0].GetValue()
		require.NotNil(t, updateValue)
		require.Equal(t, flowv1.FlowVariableSync_ValueUnion_KIND_UPDATE, updateValue.GetKind())
		require.Equal(t, varID2.Bytes(), updateValue.GetUpdate().GetFlowVariableId())
		require.Equal(t, "RENAMED", updateValue.GetUpdate().GetKey())
		require.True(t, updateValue.GetUpdate().GetEnabled())

		_, err = srv.FlowVariableDelete(varCtx, connect.NewRequest(&flowv1.FlowVariableDeleteRequest{
			Items: []*flowv1.FlowVariableDelete{
				{FlowVariableId: varID2.Bytes()},
			},
		}))
		require.NoError(t, err)

		var deleteEvent *flowv1.FlowVariableSyncResponse
		select {
		case deleteEvent = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for flow variable delete event")
		}

		require.Len(t, deleteEvent.GetItems(), 1)
		deleteValue := deleteEvent.GetItems()[0].GetValue()
		require.NotNil(t, deleteValue)
		require.Equal(t, flowv1.FlowVariableSync_ValueUnion_KIND_DELETE, deleteValue.GetKind())
		require.Equal(t, varID2.Bytes(), deleteValue.GetDelete().GetFlowVariableId())

		cancelSync()
		err = <-errCh
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("flow version sync streams snapshot and events", func(t *testing.T) {
		versionUserID := idwrap.NewNow()
		require.NoError(t, services.Us.CreateUser(context.Background(), &muser.User{
			ID:           versionUserID,
			Email:        "flowversion-sync@example.com",
			Password:     []byte("secret"),
			ProviderType: muser.MagicLink,
		}))

		versionWorkspaceID := createWorkspaceMembership(t, services.Ws, services.Wus, versionUserID)

		baseFlowID := idwrap.NewNow()
		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:          baseFlowID,
			WorkspaceID: versionWorkspaceID,
			Name:        "base",
		}))

		versionID := idwrap.NewNow()
		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:              versionID,
			WorkspaceID:     versionWorkspaceID,
			Name:            "v1",
			VersionParentID: &baseFlowID,
		}))

		versionCtx := mwauth.CreateAuthedContext(context.Background(), versionUserID)

		received := make(chan *flowv1.FlowVersionSyncResponse, 6)
		ctxSync, cancelSync := context.WithCancel(versionCtx)
		t.Cleanup(cancelSync)

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.streamFlowVersionSync(ctxSync, func(resp *flowv1.FlowVersionSyncResponse) error {
				received <- resp
				return nil
			})
		}()

		var snapshot *flowv1.FlowVersionSyncResponse
		select {
		case snapshot = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for flow version snapshot")
		}

		require.Len(t, snapshot.GetItems(), 1)
		snapshotValue := snapshot.GetItems()[0].GetValue()
		require.NotNil(t, snapshotValue)
		require.Equal(t, flowv1.FlowVersionSync_ValueUnion_KIND_INSERT, snapshotValue.GetKind())
		require.Equal(t, versionID.Bytes(), snapshotValue.GetCreate().GetFlowVersionId())
		require.Equal(t, baseFlowID.Bytes(), snapshotValue.GetCreate().GetFlowId())

		newVersionID := idwrap.NewNow()
		flowVersionStream.Publish(FlowVersionTopic{FlowID: baseFlowID}, FlowVersionEvent{
			Type:      flowVersionEventInsert,
			FlowID:    baseFlowID,
			VersionID: newVersionID,
		})

		var createEvent *flowv1.FlowVersionSyncResponse
		select {
		case createEvent = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for flow version create event")
		}

		require.Len(t, createEvent.GetItems(), 1)
		createValue := createEvent.GetItems()[0].GetValue()
		require.NotNil(t, createValue)
		require.Equal(t, flowv1.FlowVersionSync_ValueUnion_KIND_INSERT, createValue.GetKind())
		require.Equal(t, newVersionID.Bytes(), createValue.GetCreate().GetFlowVersionId())

		versionToDelete := idwrap.NewNow()
		require.NoError(t, flowService.CreateFlow(context.Background(), mflow.Flow{
			ID:              versionToDelete,
			WorkspaceID:     versionWorkspaceID,
			Name:            "v-delete",
			VersionParentID: &baseFlowID,
		}))

		deleteReq := connect.NewRequest(&flowv1.FlowDeleteRequest{
			Items: []*flowv1.FlowDelete{{FlowId: versionToDelete.Bytes()}},
		})
		_, err := srv.FlowDelete(versionCtx, deleteReq)
		require.NoError(t, err)

		var deleteEvent *flowv1.FlowVersionSyncResponse
		select {
		case deleteEvent = <-received:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for flow version delete event")
		}

		require.Len(t, deleteEvent.GetItems(), 1)
		deleteValue := deleteEvent.GetItems()[0].GetValue()
		require.NotNil(t, deleteValue)
		require.Equal(t, flowv1.FlowVersionSync_ValueUnion_KIND_DELETE, deleteValue.GetKind())
		require.Equal(t, versionToDelete.Bytes(), deleteValue.GetDelete().GetFlowVersionId())

		cancelSync()
		if err := <-errCh; err != nil {
			require.ErrorIs(t, err, context.Canceled)
		}
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

func ptrBool(v bool) *bool {
	return &v
}

func nonStartNodes(nodes []*flowv1.Node) []*flowv1.Node {
	filtered := make([]*flowv1.Node, 0, len(nodes))
	for _, node := range nodes {
		if strings.EqualFold(node.GetName(), "start") {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

func TestBuildFlowSyncCreates(t *testing.T) {
	idA, err := idwrap.NewText("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	require.NoError(t, err)
	idB, err := idwrap.NewText("01ARZ3NDEKTSV4RRFFQ69G5FAW")
	require.NoError(t, err)

	flows := []mflow.Flow{
		{ID: idB, Name: "second", Duration: 42},
		{ID: idA, Name: "first"},
	}

	items := buildFlowSyncCreates(flows)
	require.Len(t, items, 2)

	first := items[0]
	require.NotNil(t, first.GetValue())
	require.Equal(t, flowv1.FlowSync_ValueUnion_KIND_INSERT, first.GetValue().GetKind())
	firstCreate := first.GetValue().GetCreate()
	require.NotNil(t, firstCreate)
	require.Equal(t, idA.Bytes(), firstCreate.GetFlowId())
	require.Equal(t, "first", firstCreate.GetName())
	require.Equal(t, int32(0), firstCreate.GetDuration())

	second := items[1]
	require.NotNil(t, second.GetValue())
	require.Equal(t, flowv1.FlowSync_ValueUnion_KIND_INSERT, second.GetValue().GetKind())
	secondCreate := second.GetValue().GetCreate()
	require.NotNil(t, secondCreate)
	require.Equal(t, idB.Bytes(), secondCreate.GetFlowId())
	require.Equal(t, "second", secondCreate.GetName())
	require.Equal(t, int32(42), secondCreate.GetDuration())
}

func findNodeIDByName(t *testing.T, srv *FlowServiceV2RPC, ctx context.Context, name string) idwrap.IDWrap {
	resp, err := srv.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	for _, item := range nonStartNodes(resp.Msg.GetItems()) {
		if item.GetName() == name {
			id, err := idwrap.NewFromBytes(item.GetNodeId())
			require.NoError(t, err)
			return id
		}
	}
	t.Fatalf("node %s not found", name)
	return idwrap.IDWrap{}
}
