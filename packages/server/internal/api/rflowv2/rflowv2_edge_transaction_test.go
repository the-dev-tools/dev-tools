package rflowv2

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// TestEdgeInsert_TransactionRollback verifies that if inserting multiple edges fails,
// ALL edges are rolled back (not just the ones after the failure).
// This test ensures the critical transaction safety bug is fixed.
func TestEdgeInsert_TransactionRollback(t *testing.T) {
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
	edgeService := sflow.NewEdgeService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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
		es:       &edgeService,
		logger:   logger,
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

	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Create nodes for the edges
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Node 1",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Node 2",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "Node 3",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Attempt to insert 3 edges, but the 2nd one will fail due to invalid flow access
	invalidFlowID := idwrap.NewNow() // User doesn't have access to this flow

	edge1ID := idwrap.NewNow()
	edge2ID := idwrap.NewNow()
	edge3ID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.EdgeInsertRequest{
		Items: []*flowv1.EdgeInsert{
			{
				EdgeId:   edge1ID.Bytes(),
				FlowId:   flowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
			{
				EdgeId:   edge2ID.Bytes(),
				FlowId:   invalidFlowID.Bytes(), // Invalid - user doesn't have access
				SourceId: node2ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
			{
				EdgeId:   edge3ID.Bytes(),
				FlowId:   flowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
		},
	})

	// Execute the insert - this should fail validation before transaction
	_, err = svc.EdgeInsert(ctx, req)
	require.Error(t, err, "Insert should fail due to invalid flow access")

	// Verify NO edges were inserted (validation happens before transaction)
	edges, err := edgeService.GetEdgesByFlowID(ctx, flowID)
	require.NoError(t, err)
	require.Empty(t, edges, "No edges should be inserted when validation fails")
}

// TestEdgeInsert_PartialSuccess_ValidatesFirst verifies that all items are validated
// before the transaction begins, so we never get partial inserts.
func TestEdgeInsert_PartialSuccess_ValidatesFirst(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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
		es:       &edgeService,
		logger:   logger,
	}

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

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Node 1",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Node 2",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Try to insert edges where item #2 has invalid flow ID
	edge1ID := idwrap.NewNow()
	edge2ID := idwrap.NewNow()
	invalidFlowID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.EdgeInsertRequest{
		Items: []*flowv1.EdgeInsert{
			{
				EdgeId:   edge1ID.Bytes(),
				FlowId:   flowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
			{
				EdgeId:   edge2ID.Bytes(),
				FlowId:   invalidFlowID.Bytes(), // Invalid flow - user doesn't have access
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
		},
	})

	_, err = svc.EdgeInsert(ctx, req)
	require.Error(t, err, "Insert should fail due to invalid flow access")

	// Verify edge1 was NOT inserted (validation happens before transaction)
	edges, err := edgeService.GetEdgesByFlowID(ctx, flowID)
	require.NoError(t, err)
	require.Empty(t, edges, "Edge 1 should NOT be inserted when edge 2 validation fails")
}

// TestEdgeInsert_AllOrNothing verifies successful batch insert
func TestEdgeInsert_AllOrNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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
		es:       &edgeService,
		logger:   logger,
	}

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

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Node 1",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Node 2",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "Node 3",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Insert 3 valid edges
	edge1ID := idwrap.NewNow()
	edge2ID := idwrap.NewNow()
	edge3ID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.EdgeInsertRequest{
		Items: []*flowv1.EdgeInsert{
			{
				EdgeId:   edge1ID.Bytes(),
				FlowId:   flowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
			{
				EdgeId:   edge2ID.Bytes(),
				FlowId:   flowID.Bytes(),
				SourceId: node2ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
			{
				EdgeId:   edge3ID.Bytes(),
				FlowId:   flowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
		},
	})

	_, err = svc.EdgeInsert(ctx, req)
	require.NoError(t, err, "All valid edges should insert successfully")

	// Verify ALL 3 edges were inserted
	edges, err := edgeService.GetEdgesByFlowID(ctx, flowID)
	require.NoError(t, err)
	require.Len(t, edges, 3, "All 3 edges should be inserted")

	// Verify the edge IDs
	edgeIDs := make(map[string]bool)
	for _, edge := range edges {
		edgeIDs[edge.ID.String()] = true
	}

	require.True(t, edgeIDs[edge1ID.String()], "Edge 1 should exist")
	require.True(t, edgeIDs[edge2ID.String()], "Edge 2 should exist")
	require.True(t, edgeIDs[edge3ID.String()], "Edge 3 should exist")
}

// TestEdgeInsert_Concurrency verifies that concurrent EdgeInsert operations
// complete successfully without SQLite deadlocks.
func TestEdgeInsert_Concurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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
		es:       &edgeService,
		logger:   logger,
	}

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

	// Pre-create 40 nodes (20 pairs) BEFORE concurrency test
	sourceNodeIDs := make([]idwrap.IDWrap, 20)
	targetNodeIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		sourceNodeIDs[i] = idwrap.NewNow()
		targetNodeIDs[i] = idwrap.NewNow()

		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        sourceNodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Source Node %d", i),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)

		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        targetNodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Target Node %d", i),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i*100 + 50),
			PositionY: 100,
		})
		require.NoError(t, err)
	}

	type edgeInsertData struct {
		EdgeID   idwrap.IDWrap
		SourceID idwrap.IDWrap
		TargetID idwrap.IDWrap
	}

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *edgeInsertData {
			return &edgeInsertData{
				EdgeID:   idwrap.NewNow(),
				SourceID: sourceNodeIDs[i],
				TargetID: targetNodeIDs[i],
			}
		},
		func(opCtx context.Context, data *edgeInsertData) error {
			req := connect.NewRequest(&flowv1.EdgeInsertRequest{
				Items: []*flowv1.EdgeInsert{
					{
						EdgeId:   data.EdgeID.Bytes(),
						FlowId:   flowID.Bytes(),
						SourceId: data.SourceID.Bytes(),
						TargetId: data.TargetID.Bytes(),
					},
				},
			})
			_, err := svc.EdgeInsert(opCtx, req)
			return err
		},
	)

	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 50*time.Millisecond, "Operations should be fast")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all edges were created
	edges, err := edgeService.GetEdgesByFlowID(ctx, flowID)
	assert.NoError(t, err)
	assert.Equal(t, 20, len(edges), "All 20 edges should be created")
}

// TestEdgeUpdate_Concurrency verifies that concurrent EdgeUpdate operations
// complete successfully without SQLite deadlocks.
func TestEdgeUpdate_Concurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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
		es:       &edgeService,
		logger:   logger,
	}

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

	// Pre-create 60 nodes (20 source, 20 target, 20 new target) and 20 edges BEFORE concurrency test
	sourceNodeIDs := make([]idwrap.IDWrap, 20)
	oldTargetNodeIDs := make([]idwrap.IDWrap, 20)
	newTargetNodeIDs := make([]idwrap.IDWrap, 20)
	edgeIDs := make([]idwrap.IDWrap, 20)

	for i := 0; i < 20; i++ {
		sourceNodeIDs[i] = idwrap.NewNow()
		oldTargetNodeIDs[i] = idwrap.NewNow()
		newTargetNodeIDs[i] = idwrap.NewNow()
		edgeIDs[i] = idwrap.NewNow()

		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        sourceNodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Source %d", i),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 150),
			PositionY: 0,
		})
		require.NoError(t, err)

		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        oldTargetNodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Old Target %d", i),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 150),
			PositionY: 100,
		})
		require.NoError(t, err)

		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        newTargetNodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("New Target %d", i),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 150),
			PositionY: 200,
		})
		require.NoError(t, err)

		// Create initial edge
		err = edgeService.CreateEdge(ctx, mflow.Edge{
			ID:              edgeIDs[i],
			FlowID:          flowID,
			SourceFlowNodeID: sourceNodeIDs[i],
			TargetFlowNodeID: oldTargetNodeIDs[i],
		})
		require.NoError(t, err)
	}

	type edgeUpdateData struct {
		EdgeID   idwrap.IDWrap
		TargetID idwrap.IDWrap
	}

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentUpdates(ctx, t, config,
		func(i int) *edgeUpdateData {
			return &edgeUpdateData{
				EdgeID:   edgeIDs[i],
				TargetID: newTargetNodeIDs[i],
			}
		},
		func(opCtx context.Context, data *edgeUpdateData) error {
			req := connect.NewRequest(&flowv1.EdgeUpdateRequest{
				Items: []*flowv1.EdgeUpdate{
					{
						EdgeId:   data.EdgeID.Bytes(),
						TargetId: data.TargetID.Bytes(),
					},
				},
			})
			_, err := svc.EdgeUpdate(opCtx, req)
			return err
		},
	)

	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 50*time.Millisecond, "Operations should be fast")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all edges were updated to new targets
	for i, edgeID := range edgeIDs {
		edge, err := edgeService.GetEdge(ctx, edgeID)
		assert.NoError(t, err)
		assert.Equal(t, newTargetNodeIDs[i], edge.TargetFlowNodeID, "Edge should point to new target")
	}
}

// TestEdgeDelete_Concurrency verifies that concurrent EdgeDelete operations
// complete successfully without SQLite deadlocks.
func TestEdgeDelete_Concurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	edgeService := sflow.NewEdgeService(queries)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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
		es:       &edgeService,
		logger:   logger,
	}

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

	// Pre-create 40 nodes and 20 edges BEFORE concurrency test
	sourceNodeIDs := make([]idwrap.IDWrap, 20)
	targetNodeIDs := make([]idwrap.IDWrap, 20)
	edgeIDs := make([]idwrap.IDWrap, 20)

	for i := 0; i < 20; i++ {
		sourceNodeIDs[i] = idwrap.NewNow()
		targetNodeIDs[i] = idwrap.NewNow()
		edgeIDs[i] = idwrap.NewNow()

		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        sourceNodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Source %d", i),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)

		err = nodeService.CreateNode(ctx, mflow.Node{
			ID:        targetNodeIDs[i],
			FlowID:    flowID,
			Name:      fmt.Sprintf("Target %d", i),
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 100),
			PositionY: 100,
		})
		require.NoError(t, err)

		// Create edge to delete
		err = edgeService.CreateEdge(ctx, mflow.Edge{
			ID:              edgeIDs[i],
			FlowID:          flowID,
			SourceFlowNodeID: sourceNodeIDs[i],
			TargetFlowNodeID: targetNodeIDs[i],
		})
		require.NoError(t, err)
	}

	type edgeDeleteData struct {
		EdgeID idwrap.IDWrap
	}

	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentDeletes(ctx, t, config,
		func(i int) *edgeDeleteData {
			return &edgeDeleteData{
				EdgeID: edgeIDs[i],
			}
		},
		func(opCtx context.Context, data *edgeDeleteData) error {
			req := connect.NewRequest(&flowv1.EdgeDeleteRequest{
				Items: []*flowv1.EdgeDelete{
					{
						EdgeId: data.EdgeID.Bytes(),
					},
				},
			})
			_, err := svc.EdgeDelete(opCtx, req)
			return err
		},
	)

	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 50*time.Millisecond, "Operations should be fast")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all edges were deleted
	edges, err := edgeService.GetEdgesByFlowID(ctx, flowID)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(edges), "All edges should be deleted")
}
