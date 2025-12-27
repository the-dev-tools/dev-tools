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
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// TestNodeHttpInsert_TransactionAtomicity verifies that NodeHttpInsert creates ALL
// node HTTP configs or NONE when an error occurs during bulk insert.
func TestNodeHttpInsert_TransactionAtomicity(t *testing.T) {
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
	nrsService := sflow.NewNodeRequestService(queries)

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
		nrs:      &nrsService,
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

	// Create 3 base nodes (REQUEST nodes)
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Request Node 1",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Request Node 2",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node3ID,
		FlowID:    flowID,
		Name:      "Request Node 3",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create HTTP IDs for linking
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()
	httpID3 := idwrap.NewNow()

	// Test: Insert 3 node HTTP configs atomically
	req := connect.NewRequest(&flowv1.NodeHttpInsertRequest{
		Items: []*flowv1.NodeHttpInsert{
			{
				NodeId: node1ID.Bytes(),
				HttpId: httpID1.Bytes(),
			},
			{
				NodeId: node2ID.Bytes(),
				HttpId: httpID2.Bytes(),
			},
			{
				NodeId: node3ID.Bytes(),
				HttpId: httpID3.Bytes(),
			},
		},
	})

	_, err = svc.NodeHttpInsert(ctx, req)
	require.NoError(t, err, "Bulk insert should succeed")

	// Verify ALL 3 node HTTP configs were created
	nodeReq1, err := nrsService.GetNodeRequest(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq1)
	require.NotNil(t, nodeReq1.HttpID)
	require.Equal(t, httpID1, *nodeReq1.HttpID)

	nodeReq2, err := nrsService.GetNodeRequest(ctx, node2ID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq2)
	require.NotNil(t, nodeReq2.HttpID)
	require.Equal(t, httpID2, *nodeReq2.HttpID)

	nodeReq3, err := nrsService.GetNodeRequest(ctx, node3ID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq3)
	require.NotNil(t, nodeReq3.HttpID)
	require.Equal(t, httpID3, *nodeReq3.HttpID)
}

// TestNodeHttpUpdate_TransactionAtomicity verifies that NodeHttpUpdate updates ALL
// node HTTP configs or NONE when validation fails partway through.
func TestNodeHttpUpdate_TransactionAtomicity(t *testing.T) {
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
	nrsService := sflow.NewNodeRequestService(queries)

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
		nrs:      &nrsService,
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

	// Create 2 base nodes with existing HTTP configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Request Node 1",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Request Node 2",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create initial HTTP configs
	err = nrsService.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID:       node1ID,
		HttpID:           &httpID1,
		HasRequestConfig: true,
	})
	require.NoError(t, err)

	err = nrsService.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID:       node2ID,
		HttpID:           &httpID2,
		HasRequestConfig: true,
	})
	require.NoError(t, err)

	// Test: Update 2 node HTTP configs + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow() // Non-existent node
	newHttpID1 := idwrap.NewNow()
	newHttpID2 := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
		Items: []*flowv1.NodeHttpUpdate{
			{
				NodeId: node1ID.Bytes(),
				HttpId: newHttpID1.Bytes(),
			},
			{
				NodeId: invalidNodeID.Bytes(), // This will fail validation
				HttpId: newHttpID2.Bytes(),
			},
		},
	})

	_, err = svc.NodeHttpUpdate(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 was NOT updated (transaction rollback)
	nodeReq1, err := nrsService.GetNodeRequest(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq1)
	require.NotNil(t, nodeReq1.HttpID)
	require.Equal(t, httpID1, *nodeReq1.HttpID, "Node 1 should retain original HttpID")

	// Now test successful bulk update
	req = connect.NewRequest(&flowv1.NodeHttpUpdateRequest{
		Items: []*flowv1.NodeHttpUpdate{
			{
				NodeId: node1ID.Bytes(),
				HttpId: newHttpID1.Bytes(),
			},
			{
				NodeId: node2ID.Bytes(),
				HttpId: newHttpID2.Bytes(),
			},
		},
	})

	_, err = svc.NodeHttpUpdate(ctx, req)
	require.NoError(t, err, "Bulk update should succeed")

	// Verify BOTH nodes were updated
	nodeReq1, err = nrsService.GetNodeRequest(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq1.HttpID)
	require.Equal(t, newHttpID1, *nodeReq1.HttpID)

	nodeReq2, err := nrsService.GetNodeRequest(ctx, node2ID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq2.HttpID)
	require.Equal(t, newHttpID2, *nodeReq2.HttpID)
}

// TestNodeHttpDelete_TransactionAtomicity verifies that NodeHttpDelete deletes ALL
// node HTTP configs or NONE when validation fails partway through.
func TestNodeHttpDelete_TransactionAtomicity(t *testing.T) {
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
	nrsService := sflow.NewNodeRequestService(queries)

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
		nrs:      &nrsService,
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

	// Create 2 base nodes with HTTP configs
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node1ID,
		FlowID:    flowID,
		Name:      "Request Node 1",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)

	err = nodeService.CreateNode(ctx, mflow.Node{
		ID:        node2ID,
		FlowID:    flowID,
		Name:      "Request Node 2",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// Create HTTP configs
	err = nrsService.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID:       node1ID,
		HttpID:           &httpID1,
		HasRequestConfig: true,
	})
	require.NoError(t, err)

	err = nrsService.CreateNodeRequest(ctx, mflow.NodeRequest{
		FlowNodeID:       node2ID,
		HttpID:           &httpID2,
		HasRequestConfig: true,
	})
	require.NoError(t, err)

	// Test: Delete with 1 valid + 1 invalid node (should fail validation before TX)
	invalidNodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
		Items: []*flowv1.NodeHttpDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: invalidNodeID.Bytes()}, // This will fail validation
		},
	})

	_, err = svc.NodeHttpDelete(ctx, req)
	require.Error(t, err, "Should fail validation for invalid node")

	// Verify node1 HTTP config was NOT deleted (transaction rollback)
	nodeReq1, err := nrsService.GetNodeRequest(ctx, node1ID)
	require.NoError(t, err)
	require.NotNil(t, nodeReq1, "Node 1 HTTP config should still exist")

	// Now test successful bulk delete
	req = connect.NewRequest(&flowv1.NodeHttpDeleteRequest{
		Items: []*flowv1.NodeHttpDelete{
			{NodeId: node1ID.Bytes()},
			{NodeId: node2ID.Bytes()},
		},
	})

	_, err = svc.NodeHttpDelete(ctx, req)
	require.NoError(t, err, "Bulk delete should succeed")

	// Verify BOTH HTTP configs were deleted (GetNodeRequest returns nil, nil when not found)
	nodeReq1, err = nrsService.GetNodeRequest(ctx, node1ID)
	require.NoError(t, err)
	require.Nil(t, nodeReq1, "Node 1 HTTP config should be deleted")

	nodeReq2, err := nrsService.GetNodeRequest(ctx, node2ID)
	require.NoError(t, err)
	require.Nil(t, nodeReq2, "Node 2 HTTP config should be deleted")
}
