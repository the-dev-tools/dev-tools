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

func TestCreateVar(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test",
		Name:        "Something",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	varName := "testName"
	varValue := "testVal"
	varEnabled := true
	varDescription := "testDesc"

	req := connect.NewRequest(&variablev1.VariableCreateRequest{
		EnvironmentId: envID.Bytes(),
		Name:          varName,
		Value:         varValue,
		Enabled:       varEnabled,
		Description:   varDescription,
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcVar.VariableCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
		return
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	respVarID, err := idwrap.NewFromBytes(resp.Msg.VariableId)
	if err != nil {
		t.Fatal(err)
	}
	dbVar, err := vs.Get(ctx, respVarID)
	if err != nil {
		t.Fatal(err)
	}
	if dbVar == nil {
		t.Fatal("dbVar is nil")
		return
	}
	if dbVar.VarKey != varName {
		t.Error("name is not same")
	}
	if dbVar.Value != varValue {
		t.Error("value is not same")
	}
	if dbVar.Enabled != varEnabled {
		t.Error("enabled is not same")
	}
	if dbVar.Description != varDescription {
		t.Error("description is not same")
	}
}

func TestGetVar(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test",
		Name:        "Something",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	ID := idwrap.NewNow()
	varName := "testName"
	varValue := "testVal"
	varEnabled := true
	varDescription := "testDesc"
	variable := mvar.Var{
		ID:          ID,
		EnvID:       envID,
		VarKey:      varName,
		Value:       varValue,
		Enabled:     varEnabled,
		Description: varDescription,
	}
	err = vs.Create(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&variablev1.VariableGetRequest{
		VariableId: ID.Bytes(),
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcVar.VariableGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
		return
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	respVarID, err := idwrap.NewFromBytes(resp.Msg.VariableId)
	if err != nil {
		t.Fatal(err)
	}
	if ID.Compare(respVarID) != 0 {
		t.Error("ID is not same")
	}
	if resp.Msg.Name != varName {
		t.Error("name is not same")
	}
	if resp.Msg.Value != varValue {
		t.Error("value is not same")
	}
	if resp.Msg.Enabled != varEnabled {
		t.Error("enabled is not same")
	}
	if resp.Msg.Description != varDescription {
		t.Error("description is not same")
	}
}

func TestUpdateVar(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test",
		Name:        "Something",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	ID := idwrap.NewNow()
	varName := "testName"
	varValue := "testVal"
	varEnabled := true
	varDescription := "testDesc"
	variable := mvar.Var{
		ID:          ID,
		EnvID:       envID,
		VarKey:      varName,
		Value:       varValue,
		Enabled:     varEnabled,
		Description: varDescription,
	}
	err = vs.Create(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	updatedVarName := "testName2"
	updatedVarValue := "testVal2"
	updatedVarEnabled := false
	updatedVarDescription := "testDesc2"

	req := connect.NewRequest(&variablev1.VariableUpdateRequest{
		VariableId:  ID.Bytes(),
		Name:        &updatedVarName,
		Value:       &updatedVarValue,
		Enabled:     &updatedVarEnabled,
		Description: &updatedVarDescription,
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcVar.VariableUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	dbVar, err := vs.Get(ctx, ID)
	if err != nil {
		t.Fatal(err)
	}
	if dbVar == nil {
		t.Fatal("dbVar is nil")
	}
	if dbVar.VarKey != updatedVarName {
		t.Error("name is not same")
	}
	if dbVar.Value != updatedVarValue {
		t.Error("value is not same")
	}
	if dbVar.Enabled != updatedVarEnabled {
		t.Error("enabled is not same", dbVar.Enabled, updatedVarEnabled)
	}
	if dbVar.Description != updatedVarDescription {
		t.Error("description is not same")
	}
}

func TestDeleteVar(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test",
		Name:        "Something",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	ID := idwrap.NewNow()
	varName := "testName"
	varValue := "testVal"
	varEnabled := true
	varDescription := "testDesc"
	variable := mvar.Var{
		ID:          ID,
		EnvID:       envID,
		VarKey:      varName,
		Value:       varValue,
		Enabled:     varEnabled,
		Description: varDescription,
	}
	err = vs.Create(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&variablev1.VariableDeleteRequest{
		VariableId: ID.Bytes(),
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcVar.VariableDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	dbVar, err := vs.Get(ctx, ID)
	if err == nil {
		t.Error("should have error")
	}
	if svar.ErrNoVarFound != err {
		t.Error("error should be ErrNoVarFound")
	}
	if dbVar != nil {
		t.Error("dbVar is not nil")
	}
}

// TestVariableMoveAfter tests moving a variable after another variable in the same environment
func TestVariableMoveAfter(t *testing.T) {
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

	// Create two variables for testing move operation
	var1ID := idwrap.NewNow()
	var1 := mvar.Var{
		ID:          var1ID,
		EnvID:       envID,
		VarKey:      "VAR1",
		Value:       "value1",
		Enabled:     true,
		Description: "First variable",
	}
	err = vs.Create(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mvar.Var{
		ID:          var2ID,
		EnvID:       envID,
		VarKey:      "VAR2",
		Value:       "value2",
		Enabled:     true,
		Description: "Second variable",
	}
	err = vs.Create(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	// Test moving var1 after var2
	req := connect.NewRequest(&variablev1.VariableMoveRequest{
		EnvironmentId:    envID.Bytes(),
		VariableId:       var1ID.Bytes(),
		Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetVariableId: var2ID.Bytes(),
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := rpcVar.VariableMove(authedCtx, req)
	if err != nil {
		t.Fatal("Move operation failed:", err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	// Verify the order by getting the variables list
	variables, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
	if err != nil {
		t.Fatal("Failed to get ordered variables:", err)
	}

	if len(variables) != 2 {
		t.Fatal("Expected 2 variables, got:", len(variables))
	}

	// After moving var1 after var2, the order should be: var2, var1
	if variables[0].ID.Compare(var2ID) != 0 {
		t.Error("Expected var2 to be first, got:", variables[0].ID.String())
	}
	if variables[1].ID.Compare(var1ID) != 0 {
		t.Error("Expected var1 to be second, got:", variables[1].ID.String())
	}
}

// TestVariableMoveBefore tests moving a variable before another variable in the same environment
func TestVariableMoveBefore(t *testing.T) {
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

	// Create two variables for testing move operation
	var1ID := idwrap.NewNow()
	var1 := mvar.Var{
		ID:          var1ID,
		EnvID:       envID,
		VarKey:      "VAR1",
		Value:       "value1",
		Enabled:     true,
		Description: "First variable",
	}
	err = vs.Create(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mvar.Var{
		ID:          var2ID,
		EnvID:       envID,
		VarKey:      "VAR2",
		Value:       "value2",
		Enabled:     true,
		Description: "Second variable",
	}
	err = vs.Create(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	// Test moving var2 before var1
	req := connect.NewRequest(&variablev1.VariableMoveRequest{
		EnvironmentId:    envID.Bytes(),
		VariableId:       var2ID.Bytes(),
		Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		TargetVariableId: var1ID.Bytes(),
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := rpcVar.VariableMove(authedCtx, req)
	if err != nil {
		t.Fatal("Move operation failed:", err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	// Verify the order by getting the variables list
	variables, err := vs.GetVariablesByEnvIDOrdered(ctx, envID)
	if err != nil {
		t.Fatal("Failed to get ordered variables:", err)
	}

	if len(variables) != 2 {
		t.Fatal("Expected 2 variables, got:", len(variables))
	}

	// After moving var2 before var1, the order should be: var2, var1
	if variables[0].ID.Compare(var2ID) != 0 {
		t.Error("Expected var2 to be first, got:", variables[0].ID.String())
	}
	if variables[1].ID.Compare(var1ID) != 0 {
		t.Error("Expected var1 to be second, got:", variables[1].ID.String())
	}
}
