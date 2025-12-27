package rflowv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
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
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// TestNodeForEachInsert_TransactionAtomicity verifies that NodeForEachInsert creates ALL
// node ForEach configs or NONE when an error occurs during bulk insert.
func TestNodeForEachInsert_TransactionAtomicity(t *testing.T) {
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
	nfesService := sflow.NewNodeForEachService(queries)

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
		nfes:     &nfesService,
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

	// Create 3 base nodes (FOREACH nodes)
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "ForEach Node 1",
		NodeKind:  mflow.NODE_KIND_FOR_EACH,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "ForEach Node 2",
		NodeKind:  mflow.NODE_KIND_FOR_EACH,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "ForEach Node 3",
		NodeKind:  mflow.NODE_KIND_FOR_EACH,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Test: Insert 3 node ForEach configs atomically
	req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
		Items: []*flowv1.NodeForEachInsert{
			{
				NodeId:        node1ID.Bytes(),
				Path:          "$.items",
				Condition:     "item.active == true",
				ErrorHandling: flowv1.ErrorHandling_ERROR_HANDLING_BREAK,
			},
			{
				NodeId:        node2ID.Bytes(),
				Path:          "$.users",
				Condition:     "user.age > 18",
				ErrorHandling: flowv1.ErrorHandling_ERROR_HANDLING_BREAK,
			},
			{
				NodeId:        node3ID.Bytes(),
				Path:          "$.products",
				Condition:     "",
				ErrorHandling: flowv1.ErrorHandling_ERROR_HANDLING_BREAK,
			},
		},
	})

	_, err = svc.NodeForEachInsert(ctx, req)
	require.NoError(t, err, "Bulk insert should succeed")

	// Verify ALL 3 node ForEach configs were created
	nodeForEach1, err := nfesService.GetNodeForEach(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeForEach1)
	require.Equal(t, "$.items", nodeForEach1.IterExpression)

	nodeForEach2, err := nfesService.GetNodeForEach(ctx, node2ID)
	require.NoError(t, err)
	require.NotNil(t, nodeForEach2)
	require.Equal(t, "$.users", nodeForEach2.IterExpression)

	nodeForEach3, err := nfesService.GetNodeForEach(ctx, node3ID)
	require.NoError(t, err)
	require.NotNil(t, nodeForEach3)
	require.Equal(t, "$.products", nodeForEach3.IterExpression)
}

// TestNodeForEachUpdate_TransactionAtomicity verifies that NodeForEachUpdate updates ALL
// node ForEach configs or NONE when validation fails partway through.
func TestNodeForEachUpdate_TransactionAtomicity(t *testing.T) {
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
	nfesService := sflow.NewNodeForEachService(queries)

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
		nfes:     &nfesService,
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

	// Create 2 base nodes with existing ForEach configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "ForEach Node 1",
		NodeKind:  mflow.NODE_KIND_FOR_EACH,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "ForEach Node 2",
		NodeKind:  mflow.NODE_KIND_FOR_EACH,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create initial ForEach configs
	err = nfesService.CreateNodeForEach(ctx, mflow.NodeForEach{
		FlowNodeID:     node1ID,
		IterExpression: "$.items",
		Condition:      mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "item.active"}},
		ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	err = nfesService.CreateNodeForEach(ctx, mflow.NodeForEach{
		FlowNodeID:     node2ID,
		IterExpression: "$.users",
		Condition:      mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "user.admin"}},
		ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	// Test: Update 2 node ForEach configs + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow() // Non-existent node

	req := connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
		Items: []*flowv1.NodeForEachUpdate{
			{
				NodeId: node1ID.Bytes(),
				Path:   stringPtr("$.newItems"),
			},
			{
				NodeId: invalidNodeID.Bytes(), // This will fail validation
				Path:   stringPtr("$.invalid"),
			},
		},
	})

	_, err = svc.NodeForEachUpdate(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 was NOT updated (transaction rollback)
	nodeForEach1, err := nfesService.GetNodeForEach(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeForEach1)
	require.Equal(t, "$.items", nodeForEach1.IterExpression, "Node 1 should retain original path")

	// Now test successful bulk update
	req = connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
		Items: []*flowv1.NodeForEachUpdate{
			{
				NodeId: node1ID.Bytes(),
				Path:   stringPtr("$.newItems"),
			},
			{
				NodeId: node2ID.Bytes(),
				Path:   stringPtr("$.newUsers"),
			},
		},
	})

	_, err = svc.NodeForEachUpdate(ctx, req)
	require.NoError(t, err, "Bulk update should succeed")

	// Verify BOTH nodes were updated
	nodeForEach1, err = nfesService.GetNodeForEach(ctx, node1ID)
	require.NoError(t, err)
	require.Equal(t, "$.newItems", nodeForEach1.IterExpression)

	nodeForEach2, err := nfesService.GetNodeForEach(ctx, node2ID)
	require.NoError(t, err)
	require.Equal(t, "$.newUsers", nodeForEach2.IterExpression)
}

// TestNodeForEachDelete_TransactionAtomicity verifies that NodeForEachDelete deletes ALL
// node ForEach configs or NONE when validation fails partway through.
func TestNodeForEachDelete_TransactionAtomicity(t *testing.T) {
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
	nfesService := sflow.NewNodeForEachService(queries)

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
		nfes:     &nfesService,
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

	// Create 2 base nodes with ForEach configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "ForEach Node 1",
		NodeKind:  mflow.NODE_KIND_FOR_EACH,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "ForEach Node 2",
		NodeKind:  mflow.NODE_KIND_FOR_EACH,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create ForEach configs
	err = nfesService.CreateNodeForEach(ctx, mflow.NodeForEach{
		FlowNodeID:     node1ID,
		IterExpression: "$.items",
		Condition:      mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "item.active"}},
		ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	err = nfesService.CreateNodeForEach(ctx, mflow.NodeForEach{
		FlowNodeID:     node2ID,
		IterExpression: "$.users",
		Condition:      mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "user.admin"}},
		ErrorHandling:  mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	// Test: Delete with 1 valid + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeForEachDeleteRequest{
		Items: []*flowv1.NodeForEachDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: invalidNodeID.Bytes()}, // This will fail validation
		},
	})

	_, err = svc.NodeForEachDelete(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 ForEach config was NOT deleted (transaction rollback)
	nodeForEach1, err := nfesService.GetNodeForEach(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeForEach1, "Node 1 ForEach config should still exist")

	// Now test successful bulk delete
	req = connect.NewRequest(&flowv1.NodeForEachDeleteRequest{
		Items: []*flowv1.NodeForEachDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: node2ID.Bytes()},
		},
	})

	_, err = svc.NodeForEachDelete(ctx, req)
	require.NoError(t, err, "Bulk delete should succeed")

	// Verify BOTH ForEach configs were deleted (GetNodeForEach returns nil, nil when not found)
	nodeForEach1, err = nfesService.GetNodeForEach(ctx, node1ID)
	require.NoError(t, err)
	require.Nil(t, nodeForEach1, "Node 1 ForEach config should be deleted")

	nodeForEach2, err := nfesService.GetNodeForEach(ctx, node2ID)
	require.NoError(t, err)
	require.Nil(t, nodeForEach2, "Node 2 ForEach config should be deleted")
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// TestNodeForEachInsert_Concurrency verifies that concurrent insert operations
// complete successfully without SQLite deadlocks.
func TestNodeForEachInsert_Concurrency(t *testing.T) {
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
	nfesService := sflow.NewNodeForEachService(queries)

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
		nfes:     &nfesService,
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

	// Pre-create 20 base nodes BEFORE concurrency test
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("ForEach Node %d", i),
			NodeKind: mflow.NODE_KIND_FOR_EACH,
		})
		require.NoError(t, err)
	}

	// Run concurrent ForEach config inserts
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	type forEachInsertData struct {
		NodeID idwrap.IDWrap
		Path   string
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *forEachInsertData {
			return &forEachInsertData{
				NodeID: nodeIDs[i],
				Path:   fmt.Sprintf("items[%d]", i),
			}
		},
		func(opCtx context.Context, data *forEachInsertData) error {
			req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
				Items: []*flowv1.NodeForEachInsert{{
					NodeId: data.NodeID.Bytes(),
					Path:   data.Path,
				}},
			})
			_, err := svc.NodeForEachInsert(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Equal(t, 0, result.ErrorCount, "No errors expected")
	assert.Less(t, result.AverageDuration, 200*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}

// TestNodeForEachUpdate_Concurrency verifies that concurrent update operations
// complete successfully without SQLite deadlocks.
func TestNodeForEachUpdate_Concurrency(t *testing.T) {
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
	nfesService := sflow.NewNodeForEachService(queries)

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
		nfes:     &nfesService,
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

	// Pre-create 20 ForEach nodes with configs
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("ForEach Node %d", i),
			NodeKind: mflow.NODE_KIND_FOR_EACH,
		})
		require.NoError(t, err)

		// Insert initial ForEach config
		req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
			Items: []*flowv1.NodeForEachInsert{{
				NodeId: nodeIDs[i].Bytes(),
				Path:   fmt.Sprintf("items[%d]", i),
			}},
		})
		_, err = svc.NodeForEachInsert(ctx, req)
		require.NoError(t, err)
	}

	// Run concurrent ForEach config updates
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	type forEachUpdateData struct {
		NodeID idwrap.IDWrap
		Path   string
	}

	result := testutil.RunConcurrentUpdates(ctx, t, config,
		func(i int) *forEachUpdateData {
			return &forEachUpdateData{
				NodeID: nodeIDs[i],
				Path:   fmt.Sprintf("updated[%d]", i),
			}
		},
		func(opCtx context.Context, data *forEachUpdateData) error {
			path := data.Path
			req := connect.NewRequest(&flowv1.NodeForEachUpdateRequest{
				Items: []*flowv1.NodeForEachUpdate{{
					NodeId: data.NodeID.Bytes(),
					Path:   &path,
				}},
			})
			_, err := svc.NodeForEachUpdate(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Equal(t, 0, result.ErrorCount, "No errors expected")
	assert.Less(t, result.AverageDuration, 200*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}

// TestNodeForEachDelete_Concurrency verifies that concurrent delete operations
// complete successfully without SQLite deadlocks.
func TestNodeForEachDelete_Concurrency(t *testing.T) {
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
	nfesService := sflow.NewNodeForEachService(queries)

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
		nfes:     &nfesService,
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

	// Pre-create 20 ForEach nodes with configs
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("ForEach Node %d", i),
			NodeKind: mflow.NODE_KIND_FOR_EACH,
		})
		require.NoError(t, err)

		// Insert initial ForEach config
		req := connect.NewRequest(&flowv1.NodeForEachInsertRequest{
			Items: []*flowv1.NodeForEachInsert{{
				NodeId: nodeIDs[i].Bytes(),
				Path:   fmt.Sprintf("items[%d]", i),
			}},
		})
		_, err = svc.NodeForEachInsert(ctx, req)
		require.NoError(t, err)
	}

	// Run concurrent ForEach config deletes
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentDeletes(ctx, t, config,
		func(i int) idwrap.IDWrap {
			return nodeIDs[i]
		},
		func(opCtx context.Context, nodeID idwrap.IDWrap) error {
			req := connect.NewRequest(&flowv1.NodeForEachDeleteRequest{
				Items: []*flowv1.NodeForEachDelete{{
					NodeId: nodeID.Bytes(),
				}},
			})
			_, err := svc.NodeForEachDelete(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Equal(t, 0, result.ErrorCount, "No errors expected")
	assert.Less(t, result.AverageDuration, 200*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}
