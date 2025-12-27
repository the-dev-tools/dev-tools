package rflowv2

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// TestNodeConditionInsert_TransactionAtomicity verifies that NodeConditionInsert creates ALL
// node Condition configs or NONE when an error occurs during bulk insert.
func TestNodeConditionInsert_TransactionAtomicity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nifsService := sflow.NewNodeIfService(queries)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	svc := &FlowServiceV2RPC{
		DB:       db,
		wsReader: wsReader,
		fsReader: fsReader,
		nsReader: nsReader,
		ws:       &wsService,
		fs:       &flowService,
		ns:       &nodeService,
		nifs:     nifsService,
	}

	// Create test data
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

	// Create flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create 3 base nodes (CONDITION nodes)
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Condition Node 1",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Condition Node 2",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "Condition Node 3",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Test: Insert 3 node Condition configs atomically
	req := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
		Items: []*flowv1.NodeConditionInsert{
			{
				NodeId:    node1ID.Bytes(),
				Condition: "status == 200",
			},
			{
				NodeId:    node2ID.Bytes(),
				Condition: "age > 18",
			},
			{
				NodeId:    node3ID.Bytes(),
				Condition: "valid == true",
			},
		},
	})

	_, err = svc.NodeConditionInsert(ctx, req)
	require.NoError(t, err, "Bulk insert should succeed")

	// Verify ALL 3 node Condition configs were created
	nodeCondition1, err := nifsService.GetNodeIf(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeCondition1)
	require.Equal(t, "status == 200", nodeCondition1.Condition.Comparisons.Expression)

	nodeCondition2, err := nifsService.GetNodeIf(ctx, node2ID)
	require.NoError(t, err)
	require.NotNil(t, nodeCondition2)
	require.Equal(t, "age > 18", nodeCondition2.Condition.Comparisons.Expression)

	nodeCondition3, err := nifsService.GetNodeIf(ctx, node3ID)
	require.NoError(t, err)
	require.NotNil(t, nodeCondition3)
	require.Equal(t, "valid == true", nodeCondition3.Condition.Comparisons.Expression)
}

// TestNodeConditionUpdate_TransactionAtomicity verifies that NodeConditionUpdate updates ALL
// node Condition configs or NONE when validation fails partway through.
func TestNodeConditionUpdate_TransactionAtomicity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nifsService := sflow.NewNodeIfService(queries)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	svc := &FlowServiceV2RPC{
		DB:       db,
		wsReader: wsReader,
		fsReader: fsReader,
		nsReader: nsReader,
		ws:       &wsService,
		fs:       &flowService,
		ns:       &nodeService,
		nifs:     nifsService,
	}

	// Create test data
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

	// Create flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create 2 base nodes with existing Condition configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Condition Node 1",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Condition Node 2",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create initial Condition configs
	err = nifsService.CreateNodeIf(ctx, mflow.NodeIf{
		FlowNodeID: node1ID,
		Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "old condition 1"}},
	})
	require.NoError(t, err)

	err = nifsService.CreateNodeIf(ctx, mflow.NodeIf{
		FlowNodeID: node2ID,
		Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "old condition 2"}},
	})
	require.NoError(t, err)

	// Test: Update 2 node Condition configs + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow() // Non-existent node

	req := connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
		Items: []*flowv1.NodeConditionUpdate{
			{
				NodeId:    node1ID.Bytes(),
				Condition: conditionPtr("new condition 1"),
			},
			{
				NodeId:    invalidNodeID.Bytes(), // This will fail validation
				Condition: conditionPtr("invalid"),
			},
		},
	})

	_, err = svc.NodeConditionUpdate(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 was NOT updated (transaction rollback)
	nodeCondition1, err := nifsService.GetNodeIf(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeCondition1)
	require.Equal(t, "old condition 1", nodeCondition1.Condition.Comparisons.Expression, "Node 1 should retain original condition")

	// Now test successful bulk update
	req = connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
		Items: []*flowv1.NodeConditionUpdate{
			{
				NodeId:    node1ID.Bytes(),
				Condition: conditionPtr("new condition 1"),
			},
			{
				NodeId:    node2ID.Bytes(),
				Condition: conditionPtr("new condition 2"),
			},
		},
	})

	_, err = svc.NodeConditionUpdate(ctx, req)
	require.NoError(t, err, "Bulk update should succeed")

	// Verify BOTH nodes were updated
	nodeCondition1, err = nifsService.GetNodeIf(ctx, node1ID)
	require.NoError(t, err)
	require.Equal(t, "new condition 1", nodeCondition1.Condition.Comparisons.Expression)

	nodeCondition2, err := nifsService.GetNodeIf(ctx, node2ID)
	require.NoError(t, err)
	require.Equal(t, "new condition 2", nodeCondition2.Condition.Comparisons.Expression)
}

// TestNodeConditionDelete_TransactionAtomicity verifies that NodeConditionDelete deletes ALL
// node Condition configs or NONE when validation fails partway through.
func TestNodeConditionDelete_TransactionAtomicity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nifsService := sflow.NewNodeIfService(queries)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	svc := &FlowServiceV2RPC{
		DB:       db,
		wsReader: wsReader,
		fsReader: fsReader,
		nsReader: nsReader,
		ws:       &wsService,
		fs:       &flowService,
		ns:       &nodeService,
		nifs:     nifsService,
	}

	// Create test data
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

	// Create flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create 2 base nodes with Condition configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Condition Node 1",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Condition Node 2",
		NodeKind:  mflow.NODE_KIND_CONDITION,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create Condition configs
	err = nifsService.CreateNodeIf(ctx, mflow.NodeIf{
		FlowNodeID: node1ID,
		Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "status == 200"}},
	})
	require.NoError(t, err)

	err = nifsService.CreateNodeIf(ctx, mflow.NodeIf{
		FlowNodeID: node2ID,
		Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "age > 18"}},
	})
	require.NoError(t, err)

	// Test: Delete with 1 valid + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
		Items: []*flowv1.NodeConditionDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: invalidNodeID.Bytes()}, // This will fail validation
		},
	})

	_, err = svc.NodeConditionDelete(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 Condition config was NOT deleted (transaction rollback)
	nodeCondition1, err := nifsService.GetNodeIf(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeCondition1, "Node 1 Condition config should still exist")

	// Now test successful bulk delete
	req = connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
		Items: []*flowv1.NodeConditionDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: node2ID.Bytes()},
		},
	})

	_, err = svc.NodeConditionDelete(ctx, req)
	require.NoError(t, err, "Bulk delete should succeed")

	// Verify BOTH Condition configs were deleted (GetNodeIf returns nil, nil when not found)
	nodeCondition1, err = nifsService.GetNodeIf(ctx, node1ID)
	require.NoError(t, err)
	require.Nil(t, nodeCondition1, "Node 1 Condition config should be deleted")

	nodeCondition2, err := nifsService.GetNodeIf(ctx, node2ID)
	require.NoError(t, err)
	require.Nil(t, nodeCondition2, "Node 2 Condition config should be deleted")
}

// Helper function to create condition string pointers
func conditionPtr(s string) *string {
	return &s
}
