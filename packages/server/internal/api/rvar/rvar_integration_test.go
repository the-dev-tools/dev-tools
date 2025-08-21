package rvar_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rvar"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	variablev1 "the-dev-tools/spec/dist/buf/go/variable/v1"
	"time"

	"connectrpc.com/connect"
)

// TestVariableMoveIntegration tests the complete variable move flow from RPC to database
func TestVariableMoveIntegration(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create environment
	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Integration Test Environment",
		Name:        "IntegrationEnv",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test complete workflow: Create → Move → Verify → Move → Verify
	t.Run("CompleteWorkflow", func(t *testing.T) {
		// Step 1: Create variables using RPC API
		var1Resp, err := rpcVar.VariableCreate(authedCtx, connect.NewRequest(&variablev1.VariableCreateRequest{
			EnvironmentId: envID.Bytes(),
			Name:          "DATABASE_URL",
			Value:         "postgres://localhost:5432/app",
			Enabled:       true,
			Description:   "Database connection URL",
		}))
		if err != nil {
			t.Fatal("Failed to create variable 1:", err)
		}
		var1ID, _ := idwrap.NewFromBytes(var1Resp.Msg.VariableId)

		var2Resp, err := rpcVar.VariableCreate(authedCtx, connect.NewRequest(&variablev1.VariableCreateRequest{
			EnvironmentId: envID.Bytes(),
			Name:          "API_KEY",
			Value:         "secret123",
			Enabled:       true,
			Description:   "API authentication key",
		}))
		if err != nil {
			t.Fatal("Failed to create variable 2:", err)
		}
		var2ID, _ := idwrap.NewFromBytes(var2Resp.Msg.VariableId)

		var3Resp, err := rpcVar.VariableCreate(authedCtx, connect.NewRequest(&variablev1.VariableCreateRequest{
			EnvironmentId: envID.Bytes(),
			Name:          "REDIS_URL",
			Value:         "redis://localhost:6379",
			Enabled:       false,
			Description:   "Redis cache URL",
		}))
		if err != nil {
			t.Fatal("Failed to create variable 3:", err)
		}
		var3ID, _ := idwrap.NewFromBytes(var3Resp.Msg.VariableId)

		// Step 2: Verify initial order (creation order)
		initialVars, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get initial variables:", err)
		}
		if len(initialVars) != 3 {
			t.Fatalf("Expected 3 variables, got %d", len(initialVars))
		}

		// Initial order should be: var1 (DATABASE_URL), var2 (API_KEY), var3 (REDIS_URL)
		expectedOrder := []struct {
			id   idwrap.IDWrap
			name string
		}{
			{var1ID, "DATABASE_URL"},
			{var2ID, "API_KEY"},
			{var3ID, "REDIS_URL"},
		}

		for i, expected := range expectedOrder {
			if initialVars[i].ID.Compare(expected.id) != 0 {
				t.Errorf("Initial order incorrect at position %d: expected %s, got %s",
					i, expected.name, initialVars[i].VarKey)
			}
		}

		// Step 3: Move REDIS_URL to the beginning (before DATABASE_URL)
		_, err = rpcVar.VariableMove(authedCtx, connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       var3ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetVariableId: var1ID.Bytes(),
		}))
		if err != nil {
			t.Fatal("Failed to move REDIS_URL before DATABASE_URL:", err)
		}

		// Step 4: Verify order after first move
		afterFirstMove, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get variables after first move:", err)
		}

		// Order should now be: REDIS_URL, DATABASE_URL, API_KEY
		expectedAfterFirst := []struct {
			id   idwrap.IDWrap
			name string
		}{
			{var3ID, "REDIS_URL"},
			{var1ID, "DATABASE_URL"},
			{var2ID, "API_KEY"},
		}

		for i, expected := range expectedAfterFirst {
			if afterFirstMove[i].ID.Compare(expected.id) != 0 {
				t.Errorf("Order after first move incorrect at position %d: expected %s, got %s",
					i, expected.name, afterFirstMove[i].VarKey)
			}
		}

		// Step 5: Move API_KEY to middle (after REDIS_URL)
		_, err = rpcVar.VariableMove(authedCtx, connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       var2ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: var3ID.Bytes(),
		}))
		if err != nil {
			t.Fatal("Failed to move API_KEY after REDIS_URL:", err)
		}

		// Step 6: Verify final order
		finalVars, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get variables after final move:", err)
		}

		// Final order should be: REDIS_URL, API_KEY, DATABASE_URL
		expectedFinal := []struct {
			id   idwrap.IDWrap
			name string
		}{
			{var3ID, "REDIS_URL"},
			{var2ID, "API_KEY"},
			{var1ID, "DATABASE_URL"},
		}

		for i, expected := range expectedFinal {
			if finalVars[i].ID.Compare(expected.id) != 0 {
				t.Errorf("Final order incorrect at position %d: expected %s, got %s",
					i, expected.name, finalVars[i].VarKey)
			}
		}

		// Step 7: Verify variable properties are maintained
		for i, variable := range finalVars {
			expectedVar := expectedFinal[i]
			if variable.ID.Compare(expectedVar.id) != 0 {
				t.Errorf("Variable ID mismatch at position %d", i)
			}
			if variable.VarKey != expectedVar.name {
				t.Errorf("Variable name mismatch at position %d: expected %s, got %s",
					i, expectedVar.name, variable.VarKey)
			}
			// Verify other properties are preserved
			if variable.EnvID.Compare(envID) != 0 {
				t.Errorf("Variable environment ID changed for %s", variable.VarKey)
			}
		}

		t.Logf("Integration test completed successfully:")
		t.Logf("  - Created 3 variables via RPC")
		t.Logf("  - Performed 2 move operations")  
		t.Logf("  - Verified ordering at each step")
		t.Logf("  - Confirmed variable properties preserved")
		t.Logf("Final order: %s → %s → %s", 
			finalVars[0].VarKey, finalVars[1].VarKey, finalVars[2].VarKey)
	})

	// Test mixed operations (create, update, move)
	t.Run("MixedOperations", func(t *testing.T) {
		// Create a new environment for this test
		mixedEnvID := idwrap.NewNow()
		mixedEnv := menv.Env{
			ID:          mixedEnvID,
			WorkspaceID: workspaceID,
			Type:        menv.EnvNormal,
			Description: "Mixed Operations Environment",
			Name:        "MixedEnv",
			Updated:     time.Now(),
		}
		err := es.Create(ctx, mixedEnv)
		if err != nil {
			t.Fatal(err)
		}

		// Create variables
		createReq := connect.NewRequest(&variablev1.VariableCreateRequest{
			EnvironmentId: mixedEnvID.Bytes(),
			Name:          "TEST_VAR",
			Value:         "initial_value",
			Enabled:       true,
			Description:   "Test variable for mixed operations",
		})
		
		createResp, err := rpcVar.VariableCreate(authedCtx, createReq)
		if err != nil {
			t.Fatal("Failed to create test variable:", err)
		}
		testVarID, _ := idwrap.NewFromBytes(createResp.Msg.VariableId)

		// Create second variable
		createReq2 := connect.NewRequest(&variablev1.VariableCreateRequest{
			EnvironmentId: mixedEnvID.Bytes(),
			Name:          "SECOND_VAR",
			Value:         "second_value",
			Enabled:       false,
			Description:   "Second variable for mixed operations",
		})
		
		createResp2, err := rpcVar.VariableCreate(authedCtx, createReq2)
		if err != nil {
			t.Fatal("Failed to create second variable:", err)
		}
		secondVarID, _ := idwrap.NewFromBytes(createResp2.Msg.VariableId)

		// Update first variable
		newValue := "updated_value"
		updateReq := connect.NewRequest(&variablev1.VariableUpdateRequest{
			VariableId:  testVarID.Bytes(),
			Value:       &newValue,
		})
		
		_, err = rpcVar.VariableUpdate(authedCtx, updateReq)
		if err != nil {
			t.Fatal("Failed to update variable:", err)
		}

		// Move second variable before first
		_, err = rpcVar.VariableMove(authedCtx, connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    mixedEnvID.Bytes(),
			VariableId:       secondVarID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetVariableId: testVarID.Bytes(),
		}))
		if err != nil {
			t.Fatal("Failed to move variable in mixed operations:", err)
		}

		// Verify final state
		variables, err := vs.GetVariablesByEnvIDOrdered(ctx, mixedEnvID)
		if err != nil {
			t.Fatal("Failed to get variables after mixed operations:", err)
		}

		if len(variables) != 2 {
			t.Fatalf("Expected 2 variables after mixed operations, got %d", len(variables))
		}

		// Verify order: SECOND_VAR, TEST_VAR
		if variables[0].ID.Compare(secondVarID) != 0 {
			t.Error("Expected SECOND_VAR to be first after move")
		}
		if variables[1].ID.Compare(testVarID) != 0 {
			t.Error("Expected TEST_VAR to be second after move")
		}

		// Verify update was preserved
		if variables[1].Value != "updated_value" {
			t.Errorf("Expected updated value 'updated_value', got '%s'", variables[1].Value)
		}

		t.Logf("Mixed operations test completed successfully:")
		t.Logf("  - Created 2 variables")
		t.Logf("  - Updated 1 variable value")
		t.Logf("  - Moved variables")
		t.Logf("  - Verified all operations preserved correctly")
	})
}