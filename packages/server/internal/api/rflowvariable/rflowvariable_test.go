package rflowvariable_test

import (
	"context"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rflowvariable"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	flowvariablev1 "the-dev-tools/spec/dist/buf/go/flowvariable/v1"

	"connectrpc.com/connect"
)

func TestFlowVariableList(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	fs := sflow.New(queries)
	fvs := sflowvariable.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create a test flow
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create test variables
	var1ID := idwrap.NewNow()
	var1 := mflowvariable.FlowVariable{
		ID:          var1ID,
		FlowID:      flowID,
		Name:        "var1",
		Value:       "value1",
		Enabled:     true,
		Description: "description1",
	}

	var2ID := idwrap.NewNow()
	var2 := mflowvariable.FlowVariable{
		ID:          var2ID,
		FlowID:      flowID,
		Name:        "var2",
		Value:       "value2",
		Enabled:     true,
		Description: "description2",
	}

	// Create the variables in the database
	err = fvs.CreateFlowVariable(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	err = fvs.CreateFlowVariable(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	// Create the request
	req := connect.NewRequest(&flowvariablev1.FlowVariableListRequest{
		FlowId: flowID.Bytes(),
	})

	// Create the service and call the RPC method
	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := rpcService.FlowVariableList(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the response
	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	// Should have 2 variables
	if len(resp.Msg.Items) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(resp.Msg.Items))
	}

	// Verify both variables are in the response
	foundVar1 := false
	foundVar2 := false
	for _, item := range resp.Msg.Items {
		varID, err := idwrap.NewFromBytes(item.VariableId)
		if err != nil {
			t.Fatal(err)
		}

		if varID.Compare(var1ID) == 0 {
			foundVar1 = true
			if item.Name != var1.Name {
				t.Errorf("expected name %s, got %s", var1.Name, item.Name)
			}
			if item.Value != var1.Value {
				t.Errorf("expected value %s, got %s", var1.Value, item.Value)
			}
		} else if varID.Compare(var2ID) == 0 {
			foundVar2 = true
			if item.Name != var2.Name {
				t.Errorf("expected name %s, got %s", var2.Name, item.Name)
			}
			if item.Value != var2.Value {
				t.Errorf("expected value %s, got %s", var2.Value, item.Value)
			}
		}
	}

	if !foundVar1 {
		t.Error("var1 not found in response")
	}
	if !foundVar2 {
		t.Error("var2 not found in response")
	}
}

func TestFlowVariableGet(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	fs := sflow.New(queries)
	fvs := sflowvariable.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create a test flow
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create test variable
	varID := idwrap.NewNow()
	variable := mflowvariable.FlowVariable{
		ID:          varID,
		FlowID:      flowID,
		Name:        "testVar",
		Value:       "testValue",
		Enabled:     true,
		Description: "Test description",
	}

	// Create the variable in the database
	err = fvs.CreateFlowVariable(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	// Create the request
	req := connect.NewRequest(&flowvariablev1.FlowVariableGetRequest{
		VariableId: varID.Bytes(),
	})

	// Create the service and call the RPC method
	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := rpcService.FlowVariableGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the response
	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	respVarID, err := idwrap.NewFromBytes(resp.Msg.VariableId)
	if err != nil {
		t.Fatal(err)
	}

	if varID.Compare(respVarID) != 0 {
		t.Error("variable ID mismatch")
	}

	if resp.Msg.Name != variable.Name {
		t.Errorf("expected name %s, got %s", variable.Name, resp.Msg.Name)
	}

	if resp.Msg.Value != variable.Value {
		t.Errorf("expected value %s, got %s", variable.Value, resp.Msg.Value)
	}

	if resp.Msg.Enabled != variable.Enabled {
		t.Errorf("expected enabled %v, got %v", variable.Enabled, resp.Msg.Enabled)
	}

	if resp.Msg.Description != variable.Description {
		t.Errorf("expected description %s, got %s", variable.Description, resp.Msg.Description)
	}
}

func TestFlowVariableCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	fs := sflow.New(queries)
	fvs := sflowvariable.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create a test flow
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Define variable details
	varName := "testVar"
	varValue := "testValue"
	varEnabled := true
	varDesc := "Test description"

	// Create the request
	req := connect.NewRequest(&flowvariablev1.FlowVariableCreateRequest{
		FlowId:      flowID.Bytes(),
		Name:        varName,
		Value:       varValue,
		Enabled:     varEnabled,
		Description: varDesc,
	})

	// Create the service and call the RPC method
	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := rpcService.FlowVariableCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the response
	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	if resp.Msg.VariableId == nil {
		t.Fatal("variable ID is nil")
	}

	// Get the created variable ID
	createdVarID, err := idwrap.NewFromBytes(resp.Msg.VariableId)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the variable exists in the database
	createdVar, err := fvs.GetFlowVariable(ctx, createdVarID)
	if err != nil {
		t.Fatalf("variable not found in database: %v", err)
	}

	// Verify variable fields
	if createdVar.FlowID.Compare(flowID) != 0 {
		t.Error("flow ID mismatch")
	}

	if createdVar.Name != varName {
		t.Errorf("expected name %s, got %s", varName, createdVar.Name)
	}

	if createdVar.Value != varValue {
		t.Errorf("expected value %s, got %s", varValue, createdVar.Value)
	}

	if createdVar.Enabled != varEnabled {
		t.Errorf("expected enabled %v, got %v", varEnabled, createdVar.Enabled)
	}

	if createdVar.Description != varDesc {
		t.Errorf("expected description %s, got %s", varDesc, createdVar.Description)
	}

	// Verify the changes in response
	if len(resp.Msg.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(resp.Msg.Changes))
	}
}

func TestFlowVariableUpdate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	fs := sflow.New(queries)
	fvs := sflowvariable.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create a test flow
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create test variable
	varID := idwrap.NewNow()
	variable := mflowvariable.FlowVariable{
		ID:          varID,
		FlowID:      flowID,
		Name:        "originalName",
		Value:       "originalValue",
		Enabled:     true,
		Description: "Original description",
	}

	// Create the variable in the database
	err = fvs.CreateFlowVariable(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	// Define update values
	newName := "updatedName"
	newValue := "updatedValue"
	newEnabled := false
	newDesc := "Updated description"

	// Create the update request
	req := connect.NewRequest(&flowvariablev1.FlowVariableUpdateRequest{
		VariableId:  varID.Bytes(),
		Name:        &newName,
		Value:       &newValue,
		Enabled:     &newEnabled,
		Description: &newDesc,
	})

	// Create the service and call the RPC method
	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := rpcService.FlowVariableUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the response
	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	// Verify changes in response
	if len(resp.Msg.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(resp.Msg.Changes))
	}

	// Get the updated variable
	updatedVar, err := fvs.GetFlowVariable(ctx, varID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify updates were applied
	if updatedVar.Name != newName {
		t.Errorf("expected name %s, got %s", newName, updatedVar.Name)
	}

	if updatedVar.Value != newValue {
		t.Errorf("expected value %s, got %s", newValue, updatedVar.Value)
	}

	if updatedVar.Enabled != newEnabled {
		t.Errorf("expected enabled %v, got %v", newEnabled, updatedVar.Enabled)
	}

	if updatedVar.Description != newDesc {
		t.Errorf("expected description %s, got %s", newDesc, updatedVar.Description)
	}
}

func TestFlowVariableDelete(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	fs := sflow.New(queries)
	fvs := sflowvariable.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create a test flow
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create test variable
	varID := idwrap.NewNow()
	variable := mflowvariable.FlowVariable{
		ID:          varID,
		FlowID:      flowID,
		Name:        "testVar",
		Value:       "testValue",
		Enabled:     true,
		Description: "Test description",
	}

	// Create the variable in the database
	err = fvs.CreateFlowVariable(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	// Create the delete request
	req := connect.NewRequest(&flowvariablev1.FlowVariableDeleteRequest{
		VariableId: varID.Bytes(),
	})

	// Create the service and call the RPC method
	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := rpcService.FlowVariableDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the response
	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	// Verify changes in response
	if len(resp.Msg.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(resp.Msg.Changes))
	}

	// Verify the variable has been deleted
	_, err = fvs.GetFlowVariable(ctx, varID)
	if err == nil {
		t.Fatal("variable still exists after deletion")
	}
}
