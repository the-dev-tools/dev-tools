package rflowv2

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func TestNodeExecution_Collection(t *testing.T) {
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
		nil, // NodeAIService
		nil, // NodeAiProviderService
		nil, // NodeMemoryService
		nil, // NodeGraphQLService
		nil, // GraphQLService
		nil, // GraphQLHeaderService
		&wsService,
		&varService,
		&flowVarService,
		res,
		logger,
		nil, // LLMProviderFactory
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
		Name:      "Test Node",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
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

// TestNodeExecution_Collection_VersionFlow tests that NodeExecutionCollection correctly
// returns executions for version flows (snapshots) by using the node_id_mapping.
// Executions are stored under parent node IDs, but when querying a version flow,
// the mapping should be used to find and return executions with version node IDs.
func TestNodeExecution_Collection_VersionFlow(t *testing.T) {
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
		nil, // NodeAIService
		nil, // NodeAiProviderService
		nil, // NodeMemoryService
		nil, // NodeGraphQLService
		nil, // GraphQLService
		nil, // GraphQLHeaderService
		&wsService,
		&varService,
		&flowVarService,
		res,
		logger,
		nil, // LLMProviderFactory
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

	// Create parent flow
	parentFlowID := idwrap.NewNow()
	parentFlow := mflow.Flow{
		ID:          parentFlowID,
		WorkspaceID: workspaceID,
		Name:        "Parent Flow",
	}
	err = flowService.CreateFlow(ctx, parentFlow)
	require.NoError(t, err)

	// Create parent node
	parentNodeID := idwrap.NewNow()
	parentNode := mflow.Node{
		ID:        parentNodeID,
		FlowID:    parentFlowID,
		Name:      "Start Node",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 100,
		PositionY: 100,
	}
	err = nodeService.CreateNode(ctx, parentNode)
	require.NoError(t, err)

	// Create version flow (snapshot)
	versionFlowID := idwrap.NewNow()
	versionNodeID := idwrap.NewNow()

	// Create mapping: parent node ID -> version node ID
	nodeIDMapping := map[string]string{
		parentNodeID.String(): versionNodeID.String(),
	}
	mappingJSON, err := json.Marshal(nodeIDMapping)
	require.NoError(t, err)

	versionFlow := mflow.Flow{
		ID:              versionFlowID,
		WorkspaceID:     workspaceID,
		Name:            "Parent Flow",
		VersionParentID: &parentFlowID,
		NodeIDMapping:   mappingJSON,
	}
	err = flowService.CreateFlow(ctx, versionFlow)
	require.NoError(t, err)

	// Create version node (with different ID than parent)
	versionNode := mflow.Node{
		ID:        versionNodeID,
		FlowID:    versionFlowID,
		Name:      "Start Node",
		NodeKind:  mflow.NODE_KIND_MANUAL_START,
		PositionX: 100,
		PositionY: 100,
	}
	err = nodeService.CreateNode(ctx, versionNode)
	require.NoError(t, err)

	// Create execution under PARENT node ID (this is how executions are stored during flow run)
	executionID := idwrap.NewNow()
	completedAt := dbtime.DBNow().Unix()
	execution := mflow.NodeExecution{
		ID:          executionID,
		NodeID:      parentNodeID, // Stored under parent node ID!
		Name:        "Execution 1",
		State:       int8(flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS),
		CompletedAt: &completedAt,
	}
	err = nodeExecService.CreateNodeExecution(ctx, execution)
	require.NoError(t, err)

	// Test: NodeExecutionCollection should return the execution for the VERSION flow
	// with the NodeID remapped to the version node ID
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := svc.NodeExecutionCollection(ctx, req)
	require.NoError(t, err)

	// Find executions for version node
	var versionNodeExecutions []*flowv1.NodeExecution
	for _, exec := range resp.Msg.Items {
		if string(exec.NodeId) == string(versionNodeID.Bytes()) {
			versionNodeExecutions = append(versionNodeExecutions, exec)
		}
	}

	// Should find the execution with version node ID (remapped from parent)
	require.NotEmpty(t, versionNodeExecutions, "Version flow should have executions via node_id_mapping")
	assert.Equal(t, executionID.Bytes(), versionNodeExecutions[0].NodeExecutionId)
	assert.Equal(t, versionNodeID.Bytes(), versionNodeExecutions[0].NodeId, "Execution NodeID should be remapped to version node ID")
	assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS, versionNodeExecutions[0].State)

	t.Logf("✅ Version node %s correctly received execution from parent node %s via mapping",
		versionNodeID.String(), parentNodeID.String())
}
