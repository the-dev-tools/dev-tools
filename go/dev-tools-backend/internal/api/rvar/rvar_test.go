package rvar_test

import (
	"context"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/rvar"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/model/mvar"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/svar"
	"dev-tools-backend/pkg/testutil"
	variablev1 "dev-tools-spec/dist/buf/go/variable/v1"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestCreateVar(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)

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
		Active:      true,
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
		t.Error(err)
	}
	if resp == nil {
		t.Error("resp is nil")
	}
	if resp.Msg == nil {
		t.Error("resp.Msg is nil")
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
	es := senv.New(queries)
	vs := svar.New(queries)

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
		Active:      true,
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
		t.Error(err)
	}
	if resp == nil {
		t.Error("resp is nil")
	}
	if resp.Msg == nil {
		t.Error("resp.Msg is nil")
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
	es := senv.New(queries)
	vs := svar.New(queries)

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
		Active:      true,
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
		Name:        updatedVarName,
		Value:       updatedVarValue,
		Enabled:     updatedVarEnabled,
		Description: updatedVarDescription,
	})

	rpcVar := rvar.New(db, us, es, vs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcVar.VariableUpdate(authedCtx, req)
	if err != nil {
		t.Error(err)
	}
	if resp == nil {
		t.Error("resp is nil")
	}
	if resp.Msg == nil {
		t.Error("resp.Msg is nil")
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
	es := senv.New(queries)
	vs := svar.New(queries)

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
		Active:      true,
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
		t.Error(err)
	}
	if resp == nil {
		t.Error("resp is nil")
	}
	if resp.Msg == nil {
		t.Error("resp.Msg is nil")
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
