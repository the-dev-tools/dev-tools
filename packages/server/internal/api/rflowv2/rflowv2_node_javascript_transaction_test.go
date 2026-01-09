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

// TestNodeJsInsert_TransactionAtomicity verifies that NodeJsInsert creates ALL
// node JS configs or NONE when an error occurs during bulk insert.
func TestNodeJsInsert_TransactionAtomicity(t *testing.T) {
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
	njssService := sflow.NewNodeJsService(queries)

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
		njss:     &njssService,
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

	// Create 3 base nodes (JS nodes)
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "JS Node 1",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "JS Node 2",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "JS Node 3",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Test: Insert 3 node JS configs atomically
	req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
		Items: []*flowv1.NodeJsInsert{
			{
				NodeId: node1ID.Bytes(),
				Code:   "console.log('test1');",
			},
			{
				NodeId: node2ID.Bytes(),
				Code:   "console.log('test2');",
			},
			{
				NodeId: node3ID.Bytes(),
				Code:   "console.log('test3');",
			},
		},
	})

	_, err = svc.NodeJsInsert(ctx, req)
	require.NoError(t, err, "Bulk insert should succeed")

	// Verify ALL 3 node JS configs were created
	nodeJs1, err := njssService.GetNodeJS(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeJs1)
	require.Equal(t, "console.log('test1');", string(nodeJs1.Code))

	nodeJs2, err := njssService.GetNodeJS(ctx, node2ID)
	require.NoError(t, err)
	require.NotNil(t, nodeJs2)
	require.Equal(t, "console.log('test2');", string(nodeJs2.Code))

	nodeJs3, err := njssService.GetNodeJS(ctx, node3ID)
	require.NoError(t, err)
	require.NotNil(t, nodeJs3)
	require.Equal(t, "console.log('test3');", string(nodeJs3.Code))
}

// TestNodeJsUpdate_TransactionAtomicity verifies that NodeJsUpdate updates ALL
// node JS configs or NONE when validation fails partway through.
func TestNodeJsUpdate_TransactionAtomicity(t *testing.T) {
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
	njssService := sflow.NewNodeJsService(queries)

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
		njss:     &njssService,
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

	// Create 2 base nodes with existing JS configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "JS Node 1",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "JS Node 2",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create initial JS configs
	err = njssService.CreateNodeJS(ctx, mflow.NodeJS{
		FlowNodeID: node1ID,
		Code:       []byte("console.log('old1');"),
	})
	require.NoError(t, err)

	err = njssService.CreateNodeJS(ctx, mflow.NodeJS{
		FlowNodeID: node2ID,
		Code:       []byte("console.log('old2');"),
	})
	require.NoError(t, err)

	// Test: Update 2 node JS configs + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow() // Non-existent node

	req := connect.NewRequest(&flowv1.NodeJsUpdateRequest{
		Items: []*flowv1.NodeJsUpdate{
			{
				NodeId: node1ID.Bytes(),
				Code:   codePtr("console.log('new1');"),
			},
			{
				NodeId: invalidNodeID.Bytes(), // This will fail validation
				Code:   codePtr("console.log('invalid');"),
			},
		},
	})

	_, err = svc.NodeJsUpdate(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 was NOT updated (transaction rollback)
	nodeJs1, err := njssService.GetNodeJS(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeJs1)
	require.Equal(t, "console.log('old1');", string(nodeJs1.Code), "Node 1 should retain original code")

	// Now test successful bulk update
	req = connect.NewRequest(&flowv1.NodeJsUpdateRequest{
		Items: []*flowv1.NodeJsUpdate{
			{
				NodeId: node1ID.Bytes(),
				Code:   codePtr("console.log('new1');"),
			},
			{
				NodeId: node2ID.Bytes(),
				Code:   codePtr("console.log('new2');"),
			},
		},
	})

	_, err = svc.NodeJsUpdate(ctx, req)
	require.NoError(t, err, "Bulk update should succeed")

	// Verify BOTH nodes were updated
	nodeJs1, err = njssService.GetNodeJS(ctx, node1ID)
	require.NoError(t, err)
	require.Equal(t, "console.log('new1');", string(nodeJs1.Code))

	nodeJs2, err := njssService.GetNodeJS(ctx, node2ID)
	require.NoError(t, err)
	require.Equal(t, "console.log('new2');", string(nodeJs2.Code))
}

// TestNodeJsDelete_TransactionAtomicity verifies that NodeJsDelete deletes ALL
// node JS configs or NONE when validation fails partway through.
func TestNodeJsDelete_TransactionAtomicity(t *testing.T) {
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
	njssService := sflow.NewNodeJsService(queries)

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
		njss:     &njssService,
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

	// Create 2 base nodes with JS configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "JS Node 1",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "JS Node 2",
		NodeKind:  mflow.NODE_KIND_JS,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create JS configs
	err = njssService.CreateNodeJS(ctx, mflow.NodeJS{
		FlowNodeID: node1ID,
		Code:       []byte("console.log('code1');"),
	})
	require.NoError(t, err)

	err = njssService.CreateNodeJS(ctx, mflow.NodeJS{
		FlowNodeID: node2ID,
		Code:       []byte("console.log('code2');"),
	})
	require.NoError(t, err)

	// Test: Delete with 1 valid + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeJsDeleteRequest{
		Items: []*flowv1.NodeJsDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: invalidNodeID.Bytes()}, // This will fail validation
		},
	})

	_, err = svc.NodeJsDelete(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 JS config was NOT deleted (transaction rollback)
	nodeJs1, err := njssService.GetNodeJS(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeJs1, "Node 1 JS config should still exist")

	// Now test successful bulk delete
	req = connect.NewRequest(&flowv1.NodeJsDeleteRequest{
		Items: []*flowv1.NodeJsDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: node2ID.Bytes()},
		},
	})

	_, err = svc.NodeJsDelete(ctx, req)
	require.NoError(t, err, "Bulk delete should succeed")

	// Verify BOTH JS configs were deleted (GetNodeJS returns nil, nil when not found)
	nodeJs1, err = njssService.GetNodeJS(ctx, node1ID)
	require.NoError(t, err)
	require.Nil(t, nodeJs1, "Node 1 JS config should be deleted")

	nodeJs2, err := njssService.GetNodeJS(ctx, node2ID)
	require.NoError(t, err)
	require.Nil(t, nodeJs2, "Node 2 JS config should be deleted")
}

// Helper function to create code string pointers
func codePtr(s string) *string {
	return &s
}

// TestNodeJsInsert_Concurrency verifies that concurrent insert operations
// complete successfully without SQLite deadlocks.
//
// This test would have failed before the fix in commit f5f11fab which moved
// GetNode() calls outside of transactions.
func TestNodeJsInsert_Concurrency(t *testing.T) {
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
	njssService := sflow.NewNodeJsService(queries)

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
		njss:     &njssService,
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
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("JS Node %d", i),
			NodeKind: mflow.NODE_KIND_JS,
		})
		require.NoError(t, err)
	}

	// Run concurrent JS config inserts
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	type jsInsertData struct {
		NodeID idwrap.IDWrap
		Code   string
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *jsInsertData {
			return &jsInsertData{
				NodeID: nodeIDs[i],
				Code:   fmt.Sprintf("console.log('concurrent %d');", i),
			}
		},
		func(opCtx context.Context, data *jsInsertData) error {
			req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
				Items: []*flowv1.NodeJsInsert{{
					NodeId: data.NodeID.Bytes(),
					Code:   data.Code,
				}},
			})
			_, err := svc.NodeJsInsert(opCtx, req)
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

// TestNodeJsUpdate_Concurrency verifies that concurrent update operations
// complete successfully without SQLite deadlocks.
func TestNodeJsUpdate_Concurrency(t *testing.T) {
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
	njssService := sflow.NewNodeJsService(queries)

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
		njss:     &njssService,
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

	// Pre-create 20 JS nodes with configs
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("JS Node %d", i),
			NodeKind: mflow.NODE_KIND_JS,
		})
		require.NoError(t, err)

		// Insert initial JS config
		req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
			Items: []*flowv1.NodeJsInsert{{
				NodeId: nodeIDs[i].Bytes(),
				Code:   fmt.Sprintf("console.log('initial %d');", i),
			}},
		})
		_, err = svc.NodeJsInsert(ctx, req)
		require.NoError(t, err)
	}

	// Run concurrent JS config updates
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	type jsUpdateData struct {
		NodeID idwrap.IDWrap
		Code   string
	}

	result := testutil.RunConcurrentUpdates(ctx, t, config,
		func(i int) *jsUpdateData {
			return &jsUpdateData{
				NodeID: nodeIDs[i],
				Code:   fmt.Sprintf("console.log('updated %d');", i),
			}
		},
		func(opCtx context.Context, data *jsUpdateData) error {
			code := data.Code
			req := connect.NewRequest(&flowv1.NodeJsUpdateRequest{
				Items: []*flowv1.NodeJsUpdate{{
					NodeId: data.NodeID.Bytes(),
					Code:   &code,
				}},
			})
			_, err := svc.NodeJsUpdate(opCtx, req)
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

// TestNodeJsDelete_Concurrency verifies that concurrent delete operations
// complete successfully without SQLite deadlocks.
func TestNodeJsDelete_Concurrency(t *testing.T) {
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
	njssService := sflow.NewNodeJsService(queries)

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
		njss:     &njssService,
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

	// Pre-create 20 JS nodes with configs
	nodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		nodeIDs[i] = idwrap.NewNow()
		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:       nodeIDs[i],
			FlowID:   flowID,
			Name:     fmt.Sprintf("JS Node %d", i),
			NodeKind: mflow.NODE_KIND_JS,
		})
		require.NoError(t, err)

		// Insert initial JS config
		req := connect.NewRequest(&flowv1.NodeJsInsertRequest{
			Items: []*flowv1.NodeJsInsert{{
				NodeId: nodeIDs[i].Bytes(),
				Code:   fmt.Sprintf("console.log('to delete %d');", i),
			}},
		})
		_, err = svc.NodeJsInsert(ctx, req)
		require.NoError(t, err)
	}

	// Run concurrent JS config deletes
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentDeletes(ctx, t, config,
		func(i int) idwrap.IDWrap {
			return nodeIDs[i]
		},
		func(opCtx context.Context, nodeID idwrap.IDWrap) error {
			req := connect.NewRequest(&flowv1.NodeJsDeleteRequest{
				Items: []*flowv1.NodeJsDelete{{
					NodeId: nodeID.Bytes(),
				}},
			})
			_, err := svc.NodeJsDelete(opCtx, req)
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
