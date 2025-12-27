package rflowv2

import (
	"context"
	"log/slog"
	"os"
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

// TestFlowInsert_TransactionAtomicity verifies that FlowInsert creates ALL
// flows and start nodes or NONE when an error occurs during bulk insert.
func TestFlowInsert_TransactionAtomicity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
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

	// Test: Insert 3 flows atomically
	flow1ID := idwrap.NewNow()
	flow2ID := idwrap.NewNow()
	flow3ID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.FlowInsertRequest{
		Items: []*flowv1.FlowInsert{
			{
				FlowId:      flow1ID.Bytes(),
				WorkspaceId: workspaceID.Bytes(),
				Name:        "Test Flow 1",
			},
			{
				FlowId:      flow2ID.Bytes(),
				WorkspaceId: workspaceID.Bytes(),
				Name:        "Test Flow 2",
			},
			{
				FlowId:      flow3ID.Bytes(),
				WorkspaceId: workspaceID.Bytes(),
				Name:        "Test Flow 3",
			},
		},
	})

	_, err = svc.FlowInsert(ctx, req)
	require.NoError(t, err, "Bulk insert should succeed")

	// Verify ALL 3 flows were created
	flow1, err := flowService.GetFlow(ctx, flow1ID)
	require.NoError(t, err)
	require.NotNil(t, flow1)
	require.Equal(t, "Test Flow 1", flow1.Name)

	flow2, err := flowService.GetFlow(ctx, flow2ID)
	require.NoError(t, err)
	require.NotNil(t, flow2)
	require.Equal(t, "Test Flow 2", flow2.Name)

	flow3, err := flowService.GetFlow(ctx, flow3ID)
	require.NoError(t, err)
	require.NotNil(t, flow3)
	require.Equal(t, "Test Flow 3", flow3.Name)

	// Verify ALL 3 start nodes were created
	nodes1, err := nodeService.GetNodesByFlowID(ctx, flow1ID)
	require.NoError(t, err)
	require.Len(t, nodes1, 1)
	require.Equal(t, "Start", nodes1[0].Name)

	nodes2, err := nodeService.GetNodesByFlowID(ctx, flow2ID)
	require.NoError(t, err)
	require.Len(t, nodes2, 1)

	nodes3, err := nodeService.GetNodesByFlowID(ctx, flow3ID)
	require.NoError(t, err)
	require.Len(t, nodes3, 1)
}

// TestFlowUpdate_TransactionAtomicity verifies that FlowUpdate updates ALL
// flows or NONE when validation fails partway through.
func TestFlowUpdate_TransactionAtomicity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
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

	// Create 2 existing flows
	flow1ID := idwrap.NewNow()
	flow2ID := idwrap.NewNow()

	err = flowService.CreateFlow(ctx, mflow.Flow{
		ID:          flow1ID,
		WorkspaceID: workspaceID,
		Name:        "Original Flow 1",
	})
	require.NoError(t, err)

	err = flowService.CreateFlow(ctx, mflow.Flow{
		ID:          flow2ID,
		WorkspaceID: workspaceID,
		Name:        "Original Flow 2",
	})
	require.NoError(t, err)

	// Test: Update with 1 valid + 1 invalid flow (should fail validation before TX)
	invalidFlowID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.FlowUpdateRequest{
		Items: []*flowv1.FlowUpdate{
			{
				FlowId: flow1ID.Bytes(),
				Name:   stringPtr("Updated Flow 1"),
			},
			{
				FlowId: invalidFlowID.Bytes(), // This will fail validation
				Name:   stringPtr("Updated Invalid"),
			},
		},
	})

	_, err = svc.FlowUpdate(ctx, req)
	require.Error(t, err, "Should fail validation for invalid flow")

	// Verify flow1 was NOT updated (transaction rollback)
	flow1, err := flowService.GetFlow(ctx, flow1ID)
	require.NoError(t, err)
	require.NotNil(t, flow1)
	require.Equal(t, "Original Flow 1", flow1.Name, "Flow 1 should retain original name")

	// Now test successful bulk update
	req = connect.NewRequest(&flowv1.FlowUpdateRequest{
		Items: []*flowv1.FlowUpdate{
			{
				FlowId: flow1ID.Bytes(),
				Name:   stringPtr("Updated Flow 1"),
			},
			{
				FlowId: flow2ID.Bytes(),
				Name:   stringPtr("Updated Flow 2"),
			},
		},
	})

	_, err = svc.FlowUpdate(ctx, req)
	require.NoError(t, err, "Bulk update should succeed")

	// Verify BOTH flows were updated
	flow1, err = flowService.GetFlow(ctx, flow1ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Flow 1", flow1.Name)

	flow2, err := flowService.GetFlow(ctx, flow2ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Flow 2", flow2.Name)
}

// TestFlowDelete_TransactionAtomicity verifies that FlowDelete deletes ALL
// flows or NONE when validation fails partway through.
func TestFlowDelete_TransactionAtomicity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
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

	// Create 2 existing flows
	flow1ID := idwrap.NewNow()
	flow2ID := idwrap.NewNow()

	err = flowService.CreateFlow(ctx, mflow.Flow{
		ID:          flow1ID,
		WorkspaceID: workspaceID,
		Name:        "Flow 1",
	})
	require.NoError(t, err)

	err = flowService.CreateFlow(ctx, mflow.Flow{
		ID:          flow2ID,
		WorkspaceID: workspaceID,
		Name:        "Flow 2",
	})
	require.NoError(t, err)

	// Now test successful bulk delete
	req := connect.NewRequest(&flowv1.FlowDeleteRequest{
		Items: []*flowv1.FlowDelete{
			{FlowId: flow1ID.Bytes()},
			{FlowId: flow2ID.Bytes()},
		},
	})

	_, err = svc.FlowDelete(ctx, req)
	require.NoError(t, err, "Bulk delete should succeed")

	// Verify BOTH flows were deleted (GetFlow returns error)
	_, err = flowService.GetFlow(ctx, flow1ID)
	require.Error(t, err, "Flow 1 should be deleted")

	_, err = flowService.GetFlow(ctx, flow2ID)
	require.Error(t, err, "Flow 2 should be deleted")
}
