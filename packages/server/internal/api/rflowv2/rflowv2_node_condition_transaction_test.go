package rflowv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
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

// TestNodeConditionInsert_Concurrency verifies that concurrent NodeConditionInsert operations
// complete successfully without SQLite deadlocks.
//
// This test verifies the fix from commit f5f11fab which moved GetNode() calls outside
// of transactions to prevent SQLite lock contention.
func TestNodeConditionInsert_Concurrency(t *testing.T) {
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

	// Pre-create 20 base nodes BEFORE concurrency test (critical!)
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        nodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Condition Node %d", i),
			NodeKind:  mflow.NODE_KIND_CONDITION,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)
	}

	// Define test data structure
	type conditionInsertData struct {
		NodeID    idwrap.IDWrap
		Condition string
	}

	// Run concurrent node condition inserts
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *conditionInsertData {
			return &conditionInsertData{
				NodeID:    nodeIDs[i],
				Condition: fmt.Sprintf("status == %d", i),
			}
		},
		func(opCtx context.Context, data *conditionInsertData) error {
			req := connect.NewRequest(&flowv1.NodeConditionInsertRequest{
				Items: []*flowv1.NodeConditionInsert{
					{
						NodeId:    data.NodeID.Bytes(),
						Condition: data.Condition,
					},
				},
			})
			_, err := svc.NodeConditionInsert(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 600*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all condition configs were created
	for i, nodeID := range nodeIDs {
		nodeCondition, err := nifsService.GetNodeIf(ctx, nodeID)
		assert.NoError(t, err)
		assert.NotNil(t, nodeCondition)
		expectedCondition := fmt.Sprintf("status == %d", i)
		assert.Equal(t, expectedCondition, nodeCondition.Condition.Comparisons.Expression)
	}
}

// TestNodeConditionUpdate_Concurrency verifies that concurrent NodeConditionUpdate operations
// complete successfully without SQLite deadlocks.
func TestNodeConditionUpdate_Concurrency(t *testing.T) {
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

	// Pre-create 20 base nodes with condition configs BEFORE concurrency test
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        nodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Condition Node %d", i),
			NodeKind:  mflow.NODE_KIND_CONDITION,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)

		// Create initial condition config
		err = nifsService.CreateNodeIf(ctx, mflow.NodeIf{
			FlowNodeID: nodeIDs[i],
			Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: fmt.Sprintf("old condition %d", i)}},
		})
		require.NoError(t, err)
	}

	// Define test data structure
	type conditionUpdateData struct {
		NodeID    idwrap.IDWrap
		Condition string
	}

	// Run concurrent node condition updates
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentUpdates(ctx, t, config,
		func(i int) *conditionUpdateData {
			return &conditionUpdateData{
				NodeID:    nodeIDs[i],
				Condition: fmt.Sprintf("updated condition %d", i),
			}
		},
		func(opCtx context.Context, data *conditionUpdateData) error {
			req := connect.NewRequest(&flowv1.NodeConditionUpdateRequest{
				Items: []*flowv1.NodeConditionUpdate{
					{
						NodeId:    data.NodeID.Bytes(),
						Condition: &data.Condition,
					},
				},
			})
			_, err := svc.NodeConditionUpdate(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 600*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all conditions were updated
	for i, nodeID := range nodeIDs {
		nodeCondition, err := nifsService.GetNodeIf(ctx, nodeID)
		assert.NoError(t, err)
		assert.NotNil(t, nodeCondition)
		expectedCondition := fmt.Sprintf("updated condition %d", i)
		assert.Equal(t, expectedCondition, nodeCondition.Condition.Comparisons.Expression)
	}
}

// TestNodeConditionDelete_Concurrency verifies that concurrent NodeConditionDelete operations
// complete successfully without SQLite deadlocks.
func TestNodeConditionDelete_Concurrency(t *testing.T) {
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

	// Pre-create 20 base nodes with condition configs BEFORE concurrency test
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        nodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Condition Node %d", i),
			NodeKind:  mflow.NODE_KIND_CONDITION,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)

		// Create condition config to delete
		err = nifsService.CreateNodeIf(ctx, mflow.NodeIf{
			FlowNodeID: nodeIDs[i],
			Condition:  mcondition.Condition{Comparisons: mcondition.Comparison{Expression: fmt.Sprintf("condition %d", i)}},
		})
		require.NoError(t, err)
	}

	// Define test data structure
	type conditionDeleteData struct {
		NodeID idwrap.IDWrap
	}

	// Run concurrent node condition deletes
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentDeletes(ctx, t, config,
		func(i int) *conditionDeleteData {
			return &conditionDeleteData{
				NodeID: nodeIDs[i],
			}
		},
		func(opCtx context.Context, data *conditionDeleteData) error {
			req := connect.NewRequest(&flowv1.NodeConditionDeleteRequest{
				Items: []*flowv1.NodeConditionDelete{
					{
						NodeId: data.NodeID.Bytes(),
					},
				},
			})
			_, err := svc.NodeConditionDelete(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 600*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all condition configs were deleted
	for _, nodeID := range nodeIDs {
		nodeCondition, err := nifsService.GetNodeIf(ctx, nodeID)
		assert.NoError(t, err)
		assert.Nil(t, nodeCondition, "Condition config should be deleted")
	}
}

// Helper function to create condition string pointers
func conditionPtr(s string) *string {
	return &s
}
