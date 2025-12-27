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

// TestFlowVariableInsert_TransactionRollback verifies that if inserting multiple flow variables fails,
// ALL variables are rolled back (not just the ones after the failure).
func TestFlowVariableInsert_TransactionRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
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
		fvs:      &flowVarService,
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

	// Try to insert 3 variables, but the 2nd one will fail due to invalid flow
	invalidFlowID := idwrap.NewNow()

	var1ID := idwrap.NewNow()
	var2ID := idwrap.NewNow()
	var3ID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.FlowVariableInsertRequest{
		Items: []*flowv1.FlowVariableInsert{
			{
				FlowVariableId: var1ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "var1",
				Value:          "value1",
				Enabled:        true,
			},
			{
				FlowVariableId: var2ID.Bytes(),
				FlowId:         invalidFlowID.Bytes(), // Invalid - user doesn't have access
				Key:            "var2",
				Value:          "value2",
				Enabled:        true,
			},
			{
				FlowVariableId: var3ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "var3",
				Value:          "value3",
				Enabled:        true,
			},
		},
	})

	_, err = svc.FlowVariableInsert(ctx, req)
	require.Error(t, err, "Insert should fail due to invalid flow access")

	// Verify NO variables were inserted (validation happens before transaction)
	vars, err := flowVarService.GetFlowVariablesByFlowID(ctx, flowID)
	require.NoError(t, err)
	require.Empty(t, vars, "No variables should be inserted when validation fails")
}

// TestFlowVariableInsert_AllOrNothing verifies successful batch insert
func TestFlowVariableInsert_AllOrNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
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
		fvs:      &flowVarService,
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

	// Insert 5 valid variables
	var1ID := idwrap.NewNow()
	var2ID := idwrap.NewNow()
	var3ID := idwrap.NewNow()
	var4ID := idwrap.NewNow()
	var5ID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.FlowVariableInsertRequest{
		Items: []*flowv1.FlowVariableInsert{
			{
				FlowVariableId: var1ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "apiKey",
				Value:          "secret123",
				Enabled:        true,
			},
			{
				FlowVariableId: var2ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "baseUrl",
				Value:          "https://api.example.com",
				Enabled:        true,
			},
			{
				FlowVariableId: var3ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "timeout",
				Value:          "30",
				Enabled:        false,
			},
			{
				FlowVariableId: var4ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "retries",
				Value:          "3",
				Enabled:        true,
			},
			{
				FlowVariableId: var5ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "debug",
				Value:          "true",
				Enabled:        false,
			},
		},
	})

	_, err = svc.FlowVariableInsert(ctx, req)
	require.NoError(t, err, "All valid variables should insert successfully")

	// Verify ALL 5 variables were inserted
	vars, err := flowVarService.GetFlowVariablesByFlowID(ctx, flowID)
	require.NoError(t, err)
	require.Len(t, vars, 5, "All 5 variables should be inserted")

	// Verify the variable IDs
	varIDs := make(map[string]bool)
	for _, v := range vars {
		varIDs[v.ID.String()] = true
	}

	require.True(t, varIDs[var1ID.String()], "Variable 1 should exist")
	require.True(t, varIDs[var2ID.String()], "Variable 2 should exist")
	require.True(t, varIDs[var3ID.String()], "Variable 3 should exist")
	require.True(t, varIDs[var4ID.String()], "Variable 4 should exist")
	require.True(t, varIDs[var5ID.String()], "Variable 5 should exist")
}

// TestFlowVariableUpdate_TransactionRollback verifies update rollback behavior
func TestFlowVariableUpdate_TransactionRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
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
		fvs:      &flowVarService,
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

	// Create 2 variables first
	var1ID := idwrap.NewNow()
	var2ID := idwrap.NewNow()

	insertReq := connect.NewRequest(&flowv1.FlowVariableInsertRequest{
		Items: []*flowv1.FlowVariableInsert{
			{
				FlowVariableId: var1ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "var1",
				Value:          "original1",
				Enabled:        true,
			},
			{
				FlowVariableId: var2ID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "var2",
				Value:          "original2",
				Enabled:        true,
			},
		},
	})

	_, err = svc.FlowVariableInsert(ctx, insertReq)
	require.NoError(t, err)

	// Try to update both variables, but use an invalid ID for the second one
	invalidVarID := idwrap.NewNow()

	newValue1 := "updated1"
	newValue2 := "updated2"

	updateReq := connect.NewRequest(&flowv1.FlowVariableUpdateRequest{
		Items: []*flowv1.FlowVariableUpdate{
			{
				FlowVariableId: var1ID.Bytes(),
				Value:          &newValue1,
			},
			{
				FlowVariableId: invalidVarID.Bytes(), // Invalid - doesn't exist
				Value:          &newValue2,
			},
		},
	})

	_, err = svc.FlowVariableUpdate(ctx, updateReq)
	require.Error(t, err, "Update should fail due to invalid variable ID")

	// Verify var1 was NOT updated (validation happens before transaction)
	var1, err := flowVarService.GetFlowVariable(ctx, var1ID)
	require.NoError(t, err)
	require.Equal(t, "original1", var1.Value, "Variable 1 should still have original value")
}

// TestFlowVariableDelete_TransactionRollback verifies delete rollback behavior
func TestFlowVariableDelete_TransactionRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
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
		fvs:      &flowVarService,
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

	// Create two flows - one the user has access to, one from another user
	flowID1 := idwrap.NewNow()
	flow1 := mflow.Flow{
		ID:          flowID1,
		WorkspaceID: workspaceID,
		Name:        "Test Flow 1",
	}
	err = flowService.CreateFlow(ctx, flow1)
	require.NoError(t, err)

	// Create another workspace for another user
	otherUserID := idwrap.NewNow()
	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    otherUserID,
		Email: "other@example.com",
	})
	require.NoError(t, err)

	otherWorkspaceID := idwrap.NewNow()
	otherWorkspace := mworkspace.Workspace{
		ID:              otherWorkspaceID,
		Name:            "Other Workspace",
		Updated:         dbtime.DBNow(),
		CollectionCount: 0,
		FlowCount:       0,
	}
	otherCtx := mwauth.CreateAuthedContext(context.Background(), otherUserID)
	err = wsService.Create(otherCtx, &otherWorkspace)
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(otherCtx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: otherWorkspaceID,
		UserID:      otherUserID,
		Role:        1,
	})
	require.NoError(t, err)

	flowID2 := idwrap.NewNow()
	flow2 := mflow.Flow{
		ID:          flowID2,
		WorkspaceID: otherWorkspaceID,
		Name:        "Test Flow 2",
	}
	err = flowService.CreateFlow(otherCtx, flow2)
	require.NoError(t, err)

	// Create variables in both flows
	var1ID := idwrap.NewNow()
	var2ID := idwrap.NewNow()

	insertReq1 := connect.NewRequest(&flowv1.FlowVariableInsertRequest{
		Items: []*flowv1.FlowVariableInsert{
			{
				FlowVariableId: var1ID.Bytes(),
				FlowId:         flowID1.Bytes(),
				Key:            "var1",
				Value:          "value1",
				Enabled:        true,
			},
		},
	})

	_, err = svc.FlowVariableInsert(ctx, insertReq1)
	require.NoError(t, err)

	// Create variable in other user's flow
	otherSvc := &FlowServiceV2RPC{
		DB:       db,
		wsReader: wsReader,
		fsReader: fsReader,
		nsReader: nsReader,
		ws:       &wsService,
		fs:       &flowService,
		fvs:      &flowVarService,
		logger:   logger,
	}

	insertReq2 := connect.NewRequest(&flowv1.FlowVariableInsertRequest{
		Items: []*flowv1.FlowVariableInsert{
			{
				FlowVariableId: var2ID.Bytes(),
				FlowId:         flowID2.Bytes(),
				Key:            "var2",
				Value:          "value2",
				Enabled:        true,
			},
		},
	})

	_, err = otherSvc.FlowVariableInsert(otherCtx, insertReq2)
	require.NoError(t, err)

	// Try to delete both variables as the first user - should fail on var2 due to access control
	deleteReq := connect.NewRequest(&flowv1.FlowVariableDeleteRequest{
		Items: []*flowv1.FlowVariableDelete{
			{
				FlowVariableId: var1ID.Bytes(),
			},
			{
				FlowVariableId: var2ID.Bytes(), // User doesn't have access to this flow
			},
		},
	})

	_, err = svc.FlowVariableDelete(ctx, deleteReq)
	require.Error(t, err, "Delete should fail due to access control")

	// Verify var1 was NOT deleted (validation happens before transaction)
	var1, err := flowVarService.GetFlowVariable(ctx, var1ID)
	require.NoError(t, err)
	require.Equal(t, "var1", var1.Name, "Variable 1 should still exist")

	// Verify var2 still exists
	var2, err := flowVarService.GetFlowVariable(otherCtx, var2ID)
	require.NoError(t, err)
	require.Equal(t, "var2", var2.Name, "Variable 2 should still exist")
}

// TestFlowVariableInsert_Concurrency verifies that concurrent FlowVariableInsert operations
// complete successfully without SQLite deadlocks.
func TestFlowVariableInsert_Concurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
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
		fvs:      &flowVarService,
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

	// Create flow BEFORE concurrency test
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Define test data structure
	type varInsertData struct {
		VarID idwrap.IDWrap
		Key   string
		Value string
	}

	// Run concurrent flow variable inserts
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentInserts(ctx, t, config,
		func(i int) *varInsertData {
			return &varInsertData{
				VarID: idwrap.NewNow(),
				Key:   fmt.Sprintf("var%d", i),
				Value: fmt.Sprintf("value%d", i),
			}
		},
		func(opCtx context.Context, data *varInsertData) error {
			req := connect.NewRequest(&flowv1.FlowVariableInsertRequest{
				Items: []*flowv1.FlowVariableInsert{
					{
						FlowVariableId: data.VarID.Bytes(),
						FlowId:         flowID.Bytes(),
						Key:            data.Key,
						Value:          data.Value,
						Enabled:        true,
					},
				},
			})
			_, err := svc.FlowVariableInsert(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 200*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all variables were created
	vars, err := flowVarService.GetFlowVariablesByFlowID(ctx, flowID)
	assert.NoError(t, err)
	assert.Equal(t, 20, len(vars), "All 20 variables should be created")
}

// TestFlowVariableUpdate_Concurrency verifies that concurrent FlowVariableUpdate operations
// complete successfully without SQLite deadlocks.
func TestFlowVariableUpdate_Concurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
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
		fvs:      &flowVarService,
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

	// Create flow BEFORE concurrency test
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Pre-create 20 variables BEFORE concurrency test
	varIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		varIDs[i] = idwrap.NewNow()
		err = flowVarService.CreateFlowVariable(ctx, mflow.FlowVariable{
			ID:      varIDs[i],
			FlowID:  flowID,
			Name:    fmt.Sprintf("var%d", i),
			Value:   fmt.Sprintf("old_value%d", i),
			Enabled: true,
		})
		require.NoError(t, err)
	}

	// Define test data structure
	type varUpdateData struct {
		VarID idwrap.IDWrap
		Key   string
		Value string
	}

	// Run concurrent flow variable updates
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentUpdates(ctx, t, config,
		func(i int) *varUpdateData {
			return &varUpdateData{
				VarID: varIDs[i],
				Key:   fmt.Sprintf("updated_var%d", i),
				Value: fmt.Sprintf("updated_value%d", i),
			}
		},
		func(opCtx context.Context, data *varUpdateData) error {
			req := connect.NewRequest(&flowv1.FlowVariableUpdateRequest{
				Items: []*flowv1.FlowVariableUpdate{
					{
						FlowVariableId: data.VarID.Bytes(),
						Key:            &data.Key,
						Value:          &data.Value,
						Enabled:        boolPtr(true),
					},
				},
			})
			_, err := svc.FlowVariableUpdate(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 200*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all variables were updated
	for i, varID := range varIDs {
		v, err := flowVarService.GetFlowVariable(ctx, varID)
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("updated_var%d", i), v.Name)
		assert.Equal(t, fmt.Sprintf("updated_value%d", i), v.Value)
	}
}

// TestFlowVariableDelete_Concurrency verifies that concurrent FlowVariableDelete operations
// complete successfully without SQLite deadlocks.
func TestFlowVariableDelete_Concurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
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
		fvs:      &flowVarService,
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

	// Create flow BEFORE concurrency test
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err = flowService.CreateFlow(ctx, flow)
	require.NoError(t, err)

	// Pre-create 20 variables BEFORE concurrency test
	varIDs := make([]idwrap.IDWrap, 20)
	for i := 0; i < 20; i++ {
		varIDs[i] = idwrap.NewNow()
		err = flowVarService.CreateFlowVariable(ctx, mflow.FlowVariable{
			ID:      varIDs[i],
			FlowID:  flowID,
			Name:    fmt.Sprintf("var%d", i),
			Value:   fmt.Sprintf("value%d", i),
			Enabled: true,
		})
		require.NoError(t, err)
	}

	// Define test data structure
	type varDeleteData struct {
		VarID idwrap.IDWrap
	}

	// Run concurrent flow variable deletes
	config := testutil.ConcurrencyTestConfig{
		NumGoroutines: 20,
		Timeout:       3 * time.Second,
	}

	result := testutil.RunConcurrentDeletes(ctx, t, config,
		func(i int) *varDeleteData {
			return &varDeleteData{
				VarID: varIDs[i],
			}
		},
		func(opCtx context.Context, data *varDeleteData) error {
			req := connect.NewRequest(&flowv1.FlowVariableDeleteRequest{
				Items: []*flowv1.FlowVariableDelete{
					{
						FlowVariableId: data.VarID.Bytes(),
					},
				},
			})
			_, err := svc.FlowVariableDelete(opCtx, req)
			return err
		},
	)

	// Assertions
	assert.Equal(t, 20, result.SuccessCount, "All operations should succeed")
	assert.Equal(t, 0, result.ErrorCount, "No operations should fail")
	assert.Equal(t, 0, result.TimeoutCount, "No SQLite deadlocks expected")
	assert.Less(t, result.AverageDuration, 200*time.Millisecond, "Operations should complete quickly")

	t.Logf("✅ Concurrency test passed: %d ops, avg: %v, max: %v",
		result.SuccessCount, result.AverageDuration, result.MaxDuration)

	// Verify all variables were deleted
	vars, err := flowVarService.GetFlowVariablesByFlowID(ctx, flowID)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(vars), "All variables should be deleted")
}

func boolPtr(b bool) *bool {
	return &b
}
