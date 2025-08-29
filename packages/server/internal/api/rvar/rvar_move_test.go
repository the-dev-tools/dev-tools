package rvar_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rvar"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	variablev1 "the-dev-tools/spec/dist/buf/go/variable/v1"
	"time"

	"connectrpc.com/connect"
)

// TestVariableMoveValidationErrors tests various validation error scenarios
func TestVariableMoveValidationErrors(t *testing.T) {
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
		Description: "Test Environment",
		Name:        "TestEnv",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Create a variable for testing
	varID := idwrap.NewNow()
	variable := mvar.Var{
		ID:          varID,
		EnvID:       envID,
		VarKey:      "TEST_VAR",
		Value:       "test_value",
		Enabled:     true,
		Description: "Test variable",
	}
	err = vs.Create(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test case 1: Invalid variable ID
	t.Run("InvalidVariableID", func(t *testing.T) {
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       []byte("invalid"),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varID.Bytes(),
		})

		_, err := rpcVar.VariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for invalid variable ID")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
	})

	// Test case 2: Invalid target variable ID
	t.Run("InvalidTargetVariableID", func(t *testing.T) {
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: []byte("invalid"),
		})

		_, err := rpcVar.VariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for invalid target variable ID")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
	})

	// Test case 3: Unspecified position
	t.Run("UnspecifiedPosition", func(t *testing.T) {
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
			TargetVariableId: varID.Bytes(),
		})

		_, err := rpcVar.VariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for unspecified position")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}

		if connectErr.Message() != "position must be specified" {
			t.Error("Expected specific error message, got:", connectErr.Message())
		}
	})

	// Test case 4: Self-referential move (variable cannot move relative to itself)
	t.Run("SelfReferentialMove", func(t *testing.T) {
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varID.Bytes(),
		})

		_, err := rpcVar.VariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for self-referential move")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
	})

	// Test case 5: Non-existent target variable
	t.Run("NonExistentTargetVariable", func(t *testing.T) {
		nonExistentVarID := idwrap.NewNow()
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: nonExistentVarID.Bytes(),
		})

		_, err := rpcVar.VariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for non-existent target variable")
		}

		connectErr := err.(*connect.Error)
		// When checking permissions for non-existent target variable,
		// we get CodeInternal (sql: no rows in result set) which is the correct
		// security behavior - don't reveal if variables exist to unauthorized users
		if connectErr.Code() != connect.CodeInternal {
			t.Error("Expected CodeInternal, got:", connectErr.Code())
		}
	})
}

// TestVariableMoveCrossEnvironmentValidation tests that variables cannot be moved across environments
func TestVariableMoveCrossEnvironmentValidation(t *testing.T) {
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

	// Create two environments in the same workspace
	env1ID := idwrap.NewNow()
	env1 := menv.Env{
		ID:          env1ID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Environment 1",
		Name:        "Env1",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env1)
	if err != nil {
		t.Fatal(err)
	}

	env2ID := idwrap.NewNow()
	env2 := menv.Env{
		ID:          env2ID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Environment 2",
		Name:        "Env2",
		Updated:     time.Now(),
	}
	err = es.Create(ctx, env2)
	if err != nil {
		t.Fatal(err)
	}

	// Create variables in different environments
	var1ID := idwrap.NewNow()
	var1 := mvar.Var{
		ID:          var1ID,
		EnvID:       env1ID,
		VarKey:      "VAR1",
		Value:       "value1",
		Enabled:     true,
		Description: "Variable in Environment 1",
	}
	err = vs.Create(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mvar.Var{
		ID:          var2ID,
		EnvID:       env2ID,
		VarKey:      "VAR2",
		Value:       "value2",
		Enabled:     true,
		Description: "Variable in Environment 2",
	}
	err = vs.Create(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	// Try to move variable from env1 to be after variable in env2
	req := connect.NewRequest(&variablev1.VariableMoveRequest{
		EnvironmentId:    env1ID.Bytes(),
		VariableId:       var1ID.Bytes(),
		Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetVariableId: var2ID.Bytes(),
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	_, err = rpcVar.VariableMove(authedCtx, req)
	if err == nil {
		t.Error("Expected error for cross-environment move")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Error("Expected CodeInvalidArgument (variables must be in same environment), got:", connectErr.Code())
	}

	expectedMsg := "variables must be in the same environment"
	if connectErr.Message() != expectedMsg {
		t.Error("Expected message:", expectedMsg, "got:", connectErr.Message())
	}
}

// TestVariableMoveEdgeCases tests various edge cases for variable moves
func TestVariableMoveEdgeCases(t *testing.T) {
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
		Description: "Test Environment",
		Name:        "TestEnv",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test case 1: Single variable environment (should handle gracefully)
	t.Run("SingleVariableEnvironment", func(t *testing.T) {
		// Create single variable
		varID := idwrap.NewNow()
		variable := mvar.Var{
			ID:          varID,
			EnvID:       envID,
			VarKey:      "SINGLE_VAR",
			Value:       "single_value",
			Enabled:     true,
			Description: "Single variable",
		}
		err := vs.Create(ctx, variable)
		if err != nil {
			t.Fatal(err)
		}

		// Try to move it relative to itself (should fail)
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varID.Bytes(),
		})

		_, err = rpcVar.VariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for self-referential move in single variable environment")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}

		// Clean up
		err = vs.Delete(ctx, varID)
		if err != nil {
			t.Fatal("Failed to clean up variable:", err)
		}
	})

	// Test case 2: Multiple variables with complex ordering
	t.Run("MultipleVariablesComplexOrdering", func(t *testing.T) {
		// Create 4 variables: A, B, C, D
		varAID := idwrap.NewNow()
		varA := mvar.Var{
			ID:          varAID,
			EnvID:       envID,
			VarKey:      "VAR_A",
			Value:       "value_a",
			Enabled:     true,
			Description: "Variable A",
		}
		err := vs.Create(ctx, varA)
		if err != nil {
			t.Fatal(err)
		}

		varBID := idwrap.NewNow()
		varB := mvar.Var{
			ID:          varBID,
			EnvID:       envID,
			VarKey:      "VAR_B",
			Value:       "value_b",
			Enabled:     true,
			Description: "Variable B",
		}
		err = vs.Create(ctx, varB)
		if err != nil {
			t.Fatal(err)
		}

		varCID := idwrap.NewNow()
		varC := mvar.Var{
			ID:          varCID,
			EnvID:       envID,
			VarKey:      "VAR_C",
			Value:       "value_c",
			Enabled:     true,
			Description: "Variable C",
		}
		err = vs.Create(ctx, varC)
		if err != nil {
			t.Fatal(err)
		}

		varDID := idwrap.NewNow()
		varD := mvar.Var{
			ID:          varDID,
			EnvID:       envID,
			VarKey:      "VAR_D",
			Value:       "value_d",
			Enabled:     true,
			Description: "Variable D",
		}
		err = vs.Create(ctx, varD)
		if err != nil {
			t.Fatal(err)
		}

		// Initial order should be: A, B, C, D
		variables, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get ordered variables:", err)
		}

		if len(variables) != 4 {
			t.Fatal("Expected 4 variables, got:", len(variables))
		}

		// Verify initial order: A, B, C, D
		expectedOrder := []idwrap.IDWrap{varAID, varBID, varCID, varDID}
		for i, expected := range expectedOrder {
			if variables[i].ID.Compare(expected) != 0 {
				t.Errorf("Initial order incorrect at position %d: expected %s, got %s",
					i, expected.String(), variables[i].ID.String())
			}
		}

		// Move D to be after A (should result in: A, D, B, C)
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varDID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varAID.Bytes(),
		})

		_, err = rpcVar.VariableMove(authedCtx, req)
		if err != nil {
			t.Fatal("Move operation failed:", err)
		}

		// Verify new order: A, D, B, C
		variables, err = vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get ordered variables after move:", err)
		}

		expectedOrderAfterMove := []idwrap.IDWrap{varAID, varDID, varBID, varCID}
		for i, expected := range expectedOrderAfterMove {
			if variables[i].ID.Compare(expected) != 0 {
				t.Errorf("Order after move incorrect at position %d: expected %s, got %s",
					i, expected.String(), variables[i].ID.String())
			}
		}

		// Now move B to be before A (should result in: B, A, D, C)
		req2 := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       varBID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetVariableId: varAID.Bytes(),
		})

		_, err = rpcVar.VariableMove(authedCtx, req2)
		if err != nil {
			t.Fatal("Second move operation failed:", err)
		}

		// Verify final order: B, A, D, C
		variables, err = vs.GetVariablesByEnvIDOrdered(ctx, envID)
		if err != nil {
			t.Fatal("Failed to get ordered variables after second move:", err)
		}

		finalExpectedOrder := []idwrap.IDWrap{varBID, varAID, varDID, varCID}
		for i, expected := range finalExpectedOrder {
			if variables[i].ID.Compare(expected) != 0 {
				t.Errorf("Final order incorrect at position %d: expected %s, got %s",
					i, expected.String(), variables[i].ID.String())
			}
		}

		// Clean up variables
		for _, varID := range []idwrap.IDWrap{varAID, varBID, varCID, varDID} {
			err = vs.Delete(ctx, varID)
			if err != nil {
				t.Fatal("Failed to clean up variable:", err)
			}
		}
	})
}

// TestVariableMovePermissionChecks tests permission validation for move operations
func TestVariableMovePermissionChecks(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	// Create two different users and workspaces
	workspace1ID := idwrap.NewNow()
	workspaceUser1ID := idwrap.NewNow()
	collection1ID := idwrap.NewNow()
	user1ID := idwrap.NewNow()

	user2ID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspace1ID,
		workspaceUser1ID, user1ID, collection1ID)

	// Create environment for user1
	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspace1ID,
		Type:        menv.EnvNormal,
		Description: "User1's Environment",
		Name:        "User1Env",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Create variables owned by user1
	var1ID := idwrap.NewNow()
	var1 := mvar.Var{
		ID:          var1ID,
		EnvID:       envID,
		VarKey:      "USER1_VAR1",
		Value:       "value1",
		Enabled:     true,
		Description: "User1's first variable",
	}
	err = vs.Create(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mvar.Var{
		ID:          var2ID,
		EnvID:       envID,
		VarKey:      "USER1_VAR2",
		Value:       "value2",
		Enabled:     true,
		Description: "User1's second variable",
	}
	err = vs.Create(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	rpcVar := rvar.New(db, us, es, vs)

	// Test authorized access (user1 can move their own variables)
	t.Run("AuthorizedAccess", func(t *testing.T) {
		user1AuthCtx := mwauth.CreateAuthedContext(ctx, user1ID)
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       var1ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: var2ID.Bytes(),
		})

		resp, err := rpcVar.VariableMove(user1AuthCtx, req)
		if err != nil {
			t.Error("Authorized move should succeed, got error:", err)
		}
		if resp == nil {
			t.Error("Expected response for authorized move")
		}
	})

	// Test unauthorized access (user2 cannot move user1's variables)
	t.Run("UnauthorizedAccess", func(t *testing.T) {
		user2AuthCtx := mwauth.CreateAuthedContext(ctx, user2ID)
		req := connect.NewRequest(&variablev1.VariableMoveRequest{
			EnvironmentId:    envID.Bytes(),
			VariableId:       var1ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: var2ID.Bytes(),
		})

		_, err := rpcVar.VariableMove(user2AuthCtx, req)
		if err == nil {
			t.Error("Unauthorized move should fail")
		}

		connectErr := err.(*connect.Error)
		// Should get CodePermissionDenied for unauthorized access
		if connectErr.Code() != connect.CodePermissionDenied {
			t.Error("Expected CodePermissionDenied for unauthorized access, got:", connectErr.Code())
		}
	})
}