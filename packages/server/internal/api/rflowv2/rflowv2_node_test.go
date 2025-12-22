package rflowv2

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/flow/flowbuilder"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func setupNodeTest(t *testing.T) (*FlowServiceV2RPC, context.Context, *testutil.BaseDBQueries, idwrap.IDWrap, idwrap.IDWrap) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	queries := baseDB.Queries

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Additional services for builder
	reqService := sflow.NewNodeRequestService(queries)
	forService := sflow.NewNodeForService(queries)
	forEachService := sflow.NewNodeForEachService(queries)
	ifService := sflow.NewNodeIfService(queries)
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
		DB:       baseDB.DB,
		wsReader: wsReader,
		fsReader: fsReader,
		nsReader: nsReader,
		ws:       &wsService,
		fs:       &flowService,
		ns:       &nodeService,
		nes:      &nodeExecService,
		es:       &edgeService,
		fvs:      &flowVarService,
		logger:   logger,
		builder:  builder,
		// No streams needed for basic CRUD
	}

	// Setup Data: User, Workspace, Flow
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err := queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	workspaceID, err := baseDB.GetBaseServices().CreateTempCollection(ctx, userID, "Test Workspace")
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	return svc, ctx, baseDB, workspaceID, flowID
}

func TestNodeInsert(t *testing.T) {
	svc, ctx, baseDB, _, flowID := setupNodeTest(t)
	defer baseDB.Close()

	nodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeInsertRequest{
		Items: []*flowv1.NodeInsert{{
			NodeId: nodeID.Bytes(),
			FlowId: flowID.Bytes(),
			Name:   "New Node",
			Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
			Position: &flowv1.Position{
				X: 100,
				Y: 200,
			},
		}},
	})

	_, err := svc.NodeInsert(ctx, req)
	require.NoError(t, err)

	// Verify node exists in DB
	node, err := svc.ns.GetNode(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, "New Node", node.Name)
	assert.Equal(t, mflow.NODE_KIND_REQUEST, node.NodeKind)
	assert.Equal(t, 100.0, node.PositionX)
	assert.Equal(t, 200.0, node.PositionY)
	assert.Equal(t, flowID, node.FlowID)
}

func TestNodeUpdate(t *testing.T) {
	svc, ctx, baseDB, _, flowID := setupNodeTest(t)
	defer baseDB.Close()

	// Create initial node
	nodeID := idwrap.NewNow()
	initialNode := mflow.Node{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Initial Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	}
	err := svc.ns.CreateNode(ctx, initialNode)
	require.NoError(t, err)

	// 1. Success Update
	newName := "Updated Node"
	req := connect.NewRequest(&flowv1.NodeUpdateRequest{
		Items: []*flowv1.NodeUpdate{{
			NodeId: nodeID.Bytes(),
			Name:   &newName,
			Position: &flowv1.Position{
				X: 50,
				Y: 60,
			},
		}},
	})

	_, err = svc.NodeUpdate(ctx, req)
	require.NoError(t, err)

	// Verify update
	node, err := svc.ns.GetNode(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Node", node.Name)
	assert.Equal(t, 50.0, node.PositionX)
	assert.Equal(t, 60.0, node.PositionY)

	// 2. Unsupported Update: Kind
	kind := flowv1.NodeKind_NODE_KIND_HTTP
	reqKind := connect.NewRequest(&flowv1.NodeUpdateRequest{
		Items: []*flowv1.NodeUpdate{{
			NodeId: nodeID.Bytes(),
			Kind:   &kind,
		}},
	})
	_, err = svc.NodeUpdate(ctx, reqKind)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node kind updates are not supported")

	// 3. Unsupported Update: Flow Reassignment
	reqFlow := connect.NewRequest(&flowv1.NodeUpdateRequest{
		Items: []*flowv1.NodeUpdate{{
			NodeId: nodeID.Bytes(),
			FlowId: idwrap.NewNow().Bytes(),
		}},
	})
	_, err = svc.NodeUpdate(ctx, reqFlow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node flow reassignment is not supported")
}

func TestNodeDelete(t *testing.T) {
	svc, ctx, baseDB, _, flowID := setupNodeTest(t)
	defer baseDB.Close()

	// Create node to delete
	nodeID := idwrap.NewNow()
	node := mflow.Node{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Node To Delete",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	}
	err := svc.ns.CreateNode(ctx, node)
	require.NoError(t, err)

	// Delete Node
	req := connect.NewRequest(&flowv1.NodeDeleteRequest{
		Items: []*flowv1.NodeDelete{{
			NodeId: nodeID.Bytes(),
		}},
	})

	_, err = svc.NodeDelete(ctx, req)
	require.NoError(t, err)

	// Verify node is gone
	_, err = svc.ns.GetNode(ctx, nodeID)
	require.Error(t, err)
	// Depending on implementation, GetNode might return error or nil.
	// Usually sql.ErrNoRows wrapped.
}
