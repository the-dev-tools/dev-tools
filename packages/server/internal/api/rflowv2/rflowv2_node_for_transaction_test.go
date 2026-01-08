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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// TestNodeForInsert_TransactionAtomicity verifies that NodeForInsert creates ALL
// node For configs or NONE when an error occurs during bulk insert.
func TestNodeForInsert_TransactionAtomicity(t *testing.T) {
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
	nfsService := sflow.NewNodeForService(queries)

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
		nfs:      &nfsService,
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

	// Create 3 base nodes (FOR nodes)
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "For Node 1",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "For Node 2",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "For Node 3",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Test: Insert 3 node For configs atomically
	req := connect.NewRequest(&flowv1.NodeForInsertRequest{
		Items: []*flowv1.NodeForInsert{
			{
				NodeId:     node1ID.Bytes(),
				Iterations: 5,
			},
			{
				NodeId:     node2ID.Bytes(),
				Iterations: 10,
			},
			{
				NodeId:     node3ID.Bytes(),
				Iterations: 3,
			},
		},
	})

	_, err = svc.NodeForInsert(ctx, req)
	require.NoError(t, err, "Bulk insert should succeed")

	// Verify ALL 3 node For configs were created
	nodeFor1, err := nfsService.GetNodeFor(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeFor1)
	require.Equal(t, int64(5), nodeFor1.IterCount)

	nodeFor2, err := nfsService.GetNodeFor(ctx, node2ID)
	require.NoError(t, err)
	require.NotNil(t, nodeFor2)
	require.Equal(t, int64(10), nodeFor2.IterCount)

	nodeFor3, err := nfsService.GetNodeFor(ctx, node3ID)
	require.NoError(t, err)
	require.NotNil(t, nodeFor3)
	require.Equal(t, int64(3), nodeFor3.IterCount)
}

// TestNodeForUpdate_TransactionAtomicity verifies that NodeForUpdate updates ALL
// node For configs or NONE when validation fails partway through.
func TestNodeForUpdate_TransactionAtomicity(t *testing.T) {
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
	nfsService := sflow.NewNodeForService(queries)

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
		nfs:      &nfsService,
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

	// Create 2 base nodes with existing For configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "For Node 1",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "For Node 2",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create initial For configs
	err = nfsService.CreateNodeFor(ctx, mflow.NodeFor{
		FlowNodeID:    node1ID,
		IterCount:     5,
		ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	err = nfsService.CreateNodeFor(ctx, mflow.NodeFor{
		FlowNodeID:    node2ID,
		IterCount:     10,
		ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	// Test: Update 2 node For configs + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow() // Non-existent node

	req := connect.NewRequest(&flowv1.NodeForUpdateRequest{
		Items: []*flowv1.NodeForUpdate{
			{
				NodeId:     node1ID.Bytes(),
				Iterations: intPtr(15),
			},
			{
				NodeId:     invalidNodeID.Bytes(), // This will fail validation
				Iterations: intPtr(20),
			},
		},
	})

	_, err = svc.NodeForUpdate(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 was NOT updated (transaction rollback)
	nodeFor1, err := nfsService.GetNodeFor(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeFor1)
	require.Equal(t, int64(5), nodeFor1.IterCount, "Node 1 should retain original IterCount")

	// Now test successful bulk update
	req = connect.NewRequest(&flowv1.NodeForUpdateRequest{
		Items: []*flowv1.NodeForUpdate{
			{
				NodeId:     node1ID.Bytes(),
				Iterations: intPtr(15),
			},
			{
				NodeId:     node2ID.Bytes(),
				Iterations: intPtr(20),
			},
		},
	})

	_, err = svc.NodeForUpdate(ctx, req)
	require.NoError(t, err, "Bulk update should succeed")

	// Verify BOTH nodes were updated
	nodeFor1, err = nfsService.GetNodeFor(ctx, node1ID)
	require.NoError(t, err)
	require.Equal(t, int64(15), nodeFor1.IterCount)

	nodeFor2, err := nfsService.GetNodeFor(ctx, node2ID)
	require.NoError(t, err)
	require.Equal(t, int64(20), nodeFor2.IterCount)
}

// TestNodeForDelete_TransactionAtomicity verifies that NodeForDelete deletes ALL
// node For configs or NONE when validation fails partway through.
func TestNodeForDelete_TransactionAtomicity(t *testing.T) {
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
	nfsService := sflow.NewNodeForService(queries)

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
		nfs:      &nfsService,
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

	// Create 2 base nodes with For configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "For Node 1",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "For Node 2",
		NodeKind:  mflow.NODE_KIND_FOR,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create For configs
	err = nfsService.CreateNodeFor(ctx, mflow.NodeFor{
		FlowNodeID:    node1ID,
		IterCount:     5,
		ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	err = nfsService.CreateNodeFor(ctx, mflow.NodeFor{
		FlowNodeID:    node2ID,
		IterCount:     10,
		ErrorHandling: mflow.ErrorHandling_ERROR_HANDLING_BREAK,
	})
	require.NoError(t, err)

	// Test: Delete with 1 valid + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeForDeleteRequest{
		Items: []*flowv1.NodeForDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: invalidNodeID.Bytes()}, // This will fail validation
		},
	})

	_, err = svc.NodeForDelete(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 For config was NOT deleted (transaction rollback)
	nodeFor1, err := nfsService.GetNodeFor(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeFor1, "Node 1 For config should still exist")

	// Now test successful bulk delete
	req = connect.NewRequest(&flowv1.NodeForDeleteRequest{
		Items: []*flowv1.NodeForDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: node2ID.Bytes()},
		},
	})

	_, err = svc.NodeForDelete(ctx, req)
	require.NoError(t, err, "Bulk delete should succeed")

	// Verify BOTH For configs were deleted (GetNodeFor returns nil, nil when not found)
	nodeFor1, err = nfsService.GetNodeFor(ctx, node1ID)
	require.NoError(t, err)
	require.Nil(t, nodeFor1, "Node 1 For config should be deleted")

	nodeFor2, err := nfsService.GetNodeFor(ctx, node2ID)
	require.NoError(t, err)
	require.Nil(t, nodeFor2, "Node 2 For config should be deleted")
}

// Helper function to create int pointers
func intPtr(i int32) *int32 {
	return &i
}

// TestNodeForInsert_Concurrency verifies that concurrent insert operations
// complete successfully without SQLite deadlocks.
func TestNodeForInsert_Concurrency(t *testing.T) {
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
	nfsService := sflow.NewNodeForService(queries)

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
		nfs:      &nfsService,
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
			Name:     fmt.Sprintf("For Node %d", i),
			NodeKind: mflow.NODE_KIND_FOR,
		})
		require.NoError(t, err)
	}

	// Run concurrent For config inserts
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	type forInsertData struct {
		NodeID     idwrap.IDWrap
		Iterations int32
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *forInsertData {
			return &forInsertData{
				NodeID:     nodeIDs[i],
				Iterations: int32(i + 1),
			}
		},
		func(opCtx context.Context, data *forInsertData) error {
			req := connect.NewRequest(&flowv1.NodeForInsertRequest{
				Items: []*flowv1.NodeForInsert{{
					NodeId:     data.NodeID.Bytes(),
					Iterations: data.Iterations,
				}},
			})
			_, err := svc.NodeForInsert(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Equal(t, 0, result.ErrorCount, "No errors expected")
	assert.Less(t, result.AverageDuration, 600*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}

// TestNodeForUpdate_Concurrency verifies that concurrent update operations
// complete successfully without SQLite deadlocks.
func TestNodeForUpdate_Concurrency(t *testing.T) {
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
	nfsService := sflow.NewNodeForService(queries)

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
		nfs:      &nfsService,
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

	// Pre-create 20 For nodes with configs
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("For Node %d", i),
			NodeKind: mflow.NODE_KIND_FOR,
		})
		require.NoError(t, err)

		// Insert initial For config
		req := connect.NewRequest(&flowv1.NodeForInsertRequest{
			Items: []*flowv1.NodeForInsert{{
				NodeId:     nodeIDs[i].Bytes(),
				Iterations: int32(i + 1),
			}},
		})
		_, err = svc.NodeForInsert(ctx, req)
		require.NoError(t, err)
	}

	// Run concurrent For config updates
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	type forUpdateData struct {
		NodeID     idwrap.IDWrap
		Iterations int32
	}

	result := testutil.RunConcurrentUpdates(ctx, t, config,
		func(i int) *forUpdateData {
			return &forUpdateData{
				NodeID:     nodeIDs[i],
				Iterations: int32((i + 1) * 10), // Update to 10x
			}
		},
		func(opCtx context.Context, data *forUpdateData) error {
			iterations := data.Iterations
			req := connect.NewRequest(&flowv1.NodeForUpdateRequest{
				Items: []*flowv1.NodeForUpdate{{
					NodeId:     data.NodeID.Bytes(),
					Iterations: &iterations,
				}},
			})
			_, err := svc.NodeForUpdate(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Equal(t, 0, result.ErrorCount, "No errors expected")
	assert.Less(t, result.AverageDuration, 600*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}

// TestNodeForDelete_Concurrency verifies that concurrent delete operations
// complete successfully without SQLite deadlocks.
func TestNodeForDelete_Concurrency(t *testing.T) {
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
	nfsService := sflow.NewNodeForService(queries)

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
		nfs:      &nfsService,
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

	// Pre-create 20 For nodes with configs
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("For Node %d", i),
			NodeKind: mflow.NODE_KIND_FOR,
		})
		require.NoError(t, err)

		// Insert initial For config
		req := connect.NewRequest(&flowv1.NodeForInsertRequest{
			Items: []*flowv1.NodeForInsert{{
				NodeId:     nodeIDs[i].Bytes(),
				Iterations: int32(i + 1),
			}},
		})
		_, err = svc.NodeForInsert(ctx, req)
		require.NoError(t, err)
	}

	// Run concurrent For config deletes
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentDeletes(ctx, t, config,
		func(i int) idwrap.IDWrap {
			return nodeIDs[i]
		},
		func(opCtx context.Context, nodeID idwrap.IDWrap) error {
			req := connect.NewRequest(&flowv1.NodeForDeleteRequest{
				Items: []*flowv1.NodeForDelete{{
					NodeId: nodeID.Bytes(),
				}},
			})
			_, err := svc.NodeForDelete(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Equal(t, 0, result.ErrorCount, "No errors expected")
	assert.Less(t, result.AverageDuration, 600*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)
}
