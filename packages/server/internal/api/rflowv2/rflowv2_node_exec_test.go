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
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/svar"
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
	flowService := sflow.New(queries)
	nodeService := snode.New(queries)
	nodeExecService := snodeexecution.New(queries)
	edgeService := sedge.New(queries)
	noopService := snodenoop.New(queries)
	flowVarService := sflowvariable.New(queries)
	ifService := snodeif.New(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Missing services for builder
	reqService := snoderequest.New(queries)
	forService := snodefor.New(queries)
	forEachService := snodeforeach.New(queries)
	jsService := snodejs.New(queries)
	varService := svar.New(queries, logger)

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
		ws:      &wsService,
		fs:      &flowService,
		ns:      &nodeService,
		nifs:    ifService,
		nes:     &nodeExecService,
		es:      &edgeService,
		nnos:    &noopService,
		fvs:     &flowVarService,
		logger:  logger,
		builder: builder,
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
	baseNode := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 100,
		PositionY: 100,
	}
	err = nodeService.CreateNode(ctx, baseNode)
	require.NoError(t, err)

	// Create Execution
	executionID := idwrap.NewNow()
	completedAt := dbtime.DBNow().Unix()
	execution := mnodeexecution.NodeExecution{
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
