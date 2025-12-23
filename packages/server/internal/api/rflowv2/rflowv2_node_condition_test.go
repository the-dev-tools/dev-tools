package rflowv2

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/flowbuilder"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestNodeCondition_CRUD(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	ifService := sflow.NewNodeIfService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Missing services for builder
	reqService := sflow.NewNodeRequestService(queries)
	forService := sflow.NewNodeForService(queries)
	forEachService := sflow.NewNodeForEachService(queries)
	jsService := sflow.NewNodeJsService(queries)
	varService := senv.NewVariableService(queries, logger)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	// Mock resolver
	res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

	builder := flowbuilder.New(
		&nodeService,
		&reqService,
		&forService,
		&forEachService,
		ifService,
		&jsService,
		&wsService,
		&varService,
		&flowVarService,
		res,
		logger,
	)

	svc := &FlowServiceV2RPC{
		DB:       db,
		wsReader: wsReader,
		fsReader: fsReader,
		nsReader: nsReader,
		ws:       &wsService,
		fs:       &flowService,
		ns:       &nodeService,
		nifs:     ifService,
		nes:      &nodeExecService,
		es:       &edgeService,
		fvs:      &flowVarService,
		logger:   logger,
		builder:  builder,
	}

	// Setup Data
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         dbtime.DBNow(),
		CollectionCount: 0,
		FlowCount:       0,
	}
	err = wsService.Create(ctx, &workspace)
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create Base Node
	nodeID := idwrap.NewNow()
	baseNode := mflow.Node{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Condition Node",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 100,
		PositionY: 100,
	}
	err = nodeService.CreateNode(ctx, baseNode)
	require.NoError(t, err)

	// 1. Insert Condition
	conditionExpr := "1 == 1"
	insertReq := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
		Items: []*flowv1.NodeConditionInsert{
			{
				NodeId:    nodeID.Bytes(),
				Condition: conditionExpr,
			},
		},
	})
	_, err = svc.NodeConditionInsert(ctx, insertReq)
	require.NoError(t, err)

	// Verify persistence
	nodeIf, err := ifService.GetNodeIf(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, conditionExpr, nodeIf.Condition.Comparisons.Expression)

	// 2. Collection
	collReq := connect.NewRequest(&emptypb.Empty{})
	collResp, err := svc.NodeConditionCollection(ctx, collReq)
	require.NoError(t, err)
	require.Len(t, collResp.Msg.Items, 1)
	assert.True(t, bytes.Equal(nodeID.Bytes(), collResp.Msg.Items[0].NodeId))
	assert.Equal(t, conditionExpr, collResp.Msg.Items[0].Condition)

	// 3. Update
	newConditionExpr := "2 > 1"
	updateReq := connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
		Items: []*flowv1.NodeConditionUpdate{
			{
				NodeId:    nodeID.Bytes(),
				Condition: &newConditionExpr,
			},
		},
	})
	_, err = svc.NodeConditionUpdate(ctx, updateReq)
	require.NoError(t, err)

	// Verify update
	nodeIf, err = ifService.GetNodeIf(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, newConditionExpr, nodeIf.Condition.Comparisons.Expression)

	// 4. Delete
	deleteReq := connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
		Items: []*flowv1.NodeConditionDelete{
			{
				NodeId: nodeID.Bytes(),
			},
		},
	})
	_, err = svc.NodeConditionDelete(ctx, deleteReq)
	require.NoError(t, err)

	// Verify deletion
	nodeIf, err = ifService.GetNodeIf(ctx, nodeID)
	require.NoError(t, err)
	assert.Nil(t, nodeIf)

	// Collection should be empty
	collResp, err = svc.NodeConditionCollection(ctx, collReq)
	require.NoError(t, err)
	require.Len(t, collResp.Msg.Items, 0)
}
