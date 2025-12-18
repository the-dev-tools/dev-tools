package rflowv2

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
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

func TestNodeExecution_Collection(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.New(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	noopService := sflow.NewNodeNoopService(queries)
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
	wsReader := sworkspace.NewReaderFromQueries(queries)
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
		&noopService,
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
		nnos:     &noopService,
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
		Name:      "Test Node",
		NodeKind:  mflow.NODE_KIND_NO_OP,
		PositionX: 100,
		PositionY: 100,
	}
	err = nodeService.CreateNode(ctx, baseNode)
	require.NoError(t, err)

	// Create Execution
	executionID := idwrap.NewNow()
	completedAt := dbtime.DBNow().Unix()
	execution := mflow.NodeExecution{
		ID:          executionID,
		NodeID:      nodeID,
		Name:        "Execution 1",
		State:       int8(flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS),
		CompletedAt: &completedAt,
	}
	err = nodeExecService.CreateNodeExecution(ctx, execution)
	require.NoError(t, err)

	// Test Collection
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := svc.NodeExecutionCollection(ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Items, 1)
	assert.Equal(t, executionID.Bytes(), resp.Msg.Items[0].NodeExecutionId)
	assert.Equal(t, nodeID.Bytes(), resp.Msg.Items[0].NodeId)
	assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS, resp.Msg.Items[0].State)
}
