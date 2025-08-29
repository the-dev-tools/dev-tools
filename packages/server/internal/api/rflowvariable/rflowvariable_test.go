package rflowvariable_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

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
	/*
		if len(resp.Msg.Changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(resp.Msg.Changes))
		}
	*/
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
	/*
		if len(resp.Msg.Changes) != 2 {
			t.Fatalf("expected 2 changes, got %d", len(resp.Msg.Changes))
		}
	*/

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
	/*
		if len(resp.Msg.Changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(resp.Msg.Changes))
		}
	*/

	// Verify the variable has been deleted
	_, err = fvs.GetFlowVariable(ctx, varID)
	if err == nil {
		t.Fatal("variable still exists after deletion")
	}
}

func TestFlowVariableMove(t *testing.T) {
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

	// Create test variables in order: var1, var2, var3
	var1ID := idwrap.NewNow()
	var1 := mflowvariable.FlowVariable{
		ID:          var1ID,
		FlowID:      flowID,
		Name:        "var1",
		Value:       "value1",
		Enabled:     true,
		Description: "First variable",
	}

	var2ID := idwrap.NewNow()
	var2 := mflowvariable.FlowVariable{
		ID:          var2ID,
		FlowID:      flowID,
		Name:        "var2",
		Value:       "value2",
		Enabled:     true,
		Description: "Second variable",
	}

	var3ID := idwrap.NewNow()
	var3 := mflowvariable.FlowVariable{
		ID:          var3ID,
		FlowID:      flowID,
		Name:        "var3",
		Value:       "value3",
		Enabled:     true,
		Description: "Third variable",
	}

	// Create the variables in order
	err = fvs.CreateFlowVariable(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	err = fvs.CreateFlowVariable(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	err = fvs.CreateFlowVariable(ctx, var3)
	if err != nil {
		t.Fatal(err)
	}

	// Test moving var1 after var3 (should change order to: var2, var3, var1)
	t.Run("MoveAfter", func(t *testing.T) {
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       var1ID.Bytes(),
			TargetVariableId: var3ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		})

		rpcService := rflowvariable.New(db, fs, us, fvs)
		authedCtx := mwauth.CreateAuthedContext(ctx, userID)
		resp, err := rpcService.FlowVariableMove(authedCtx, req)
		if err != nil {
			t.Fatal(err)
		}

		if resp.Msg == nil {
			t.Fatal("response message is nil")
		}

		// Verify the new order by listing variables
		listReq := connect.NewRequest(&flowvariablev1.FlowVariableListRequest{
			FlowId: flowID.Bytes(),
		})
		listResp, err := rpcService.FlowVariableList(authedCtx, listReq)
		if err != nil {
			t.Fatal(err)
		}

		if len(listResp.Msg.Items) != 3 {
			t.Fatalf("expected 3 variables, got %d", len(listResp.Msg.Items))
		}

		// Expected order: var2, var3, var1
		expectedOrder := []idwrap.IDWrap{var2ID, var3ID, var1ID}
		for i, item := range listResp.Msg.Items {
			itemID, err := idwrap.NewFromBytes(item.VariableId)
			if err != nil {
				t.Fatal(err)
			}
			if itemID.Compare(expectedOrder[i]) != 0 {
				t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i].String(), itemID.String())
			}
		}
	})

	// Test moving var1 before var2 (should change order to: var1, var2, var3)
	t.Run("MoveBefore", func(t *testing.T) {
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       var1ID.Bytes(),
			TargetVariableId: var2ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		})

		rpcService := rflowvariable.New(db, fs, us, fvs)
		authedCtx := mwauth.CreateAuthedContext(ctx, userID)
		resp, err := rpcService.FlowVariableMove(authedCtx, req)
		if err != nil {
			t.Fatal(err)
		}

		if resp.Msg == nil {
			t.Fatal("response message is nil")
		}

		// Verify the new order by listing variables
		listReq := connect.NewRequest(&flowvariablev1.FlowVariableListRequest{
			FlowId: flowID.Bytes(),
		})
		listResp, err := rpcService.FlowVariableList(authedCtx, listReq)
		if err != nil {
			t.Fatal(err)
		}

		if len(listResp.Msg.Items) != 3 {
			t.Fatalf("expected 3 variables, got %d", len(listResp.Msg.Items))
		}

		// Expected order: var1, var2, var3
		expectedOrder := []idwrap.IDWrap{var1ID, var2ID, var3ID}
		for i, item := range listResp.Msg.Items {
			itemID, err := idwrap.NewFromBytes(item.VariableId)
			if err != nil {
				t.Fatal(err)
			}
			if itemID.Compare(expectedOrder[i]) != 0 {
				t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i].String(), itemID.String())
			}
		}
	})

	// Test error cases
	t.Run("ErrorCases", func(t *testing.T) {
		rpcService := rflowvariable.New(db, fs, us, fvs)
		authedCtx := mwauth.CreateAuthedContext(ctx, userID)

		// Test moving variable relative to itself
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       var1ID.Bytes(),
			TargetVariableId: var1ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("expected error when moving variable relative to itself")
		}

		// Test with unspecified position
		req = connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       var1ID.Bytes(),
			TargetVariableId: var2ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
		})

		_, err = rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("expected error with unspecified position")
		}

		// Test with non-existent variable
		nonExistentID := idwrap.NewNow()
		req = connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       nonExistentID.Bytes(),
			TargetVariableId: var2ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		})

		_, err = rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("expected error with non-existent variable")
		}
	})
}

// TestFlowVariableMoveValidationErrors tests various validation error scenarios
func TestFlowVariableMoveValidationErrors(t *testing.T) {
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

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create a variable for testing
	varID := idwrap.NewNow()
	variable := mflowvariable.FlowVariable{
		ID:          varID,
		FlowID:      flowID,
		Name:        "TEST_VAR",
		Value:       "test_value",
		Enabled:     true,
		Description: "Test variable",
	}
	err = fvs.CreateFlowVariable(ctx, variable)
	if err != nil {
		t.Fatal(err)
	}

	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test case 1: Invalid variable ID
	t.Run("InvalidVariableID", func(t *testing.T) {
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       []byte("invalid"),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varID.Bytes(),
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
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
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: []byte("invalid"),
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for invalid target variable ID")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
	})

	// Test case 3: Invalid flow ID
	t.Run("InvalidFlowID", func(t *testing.T) {
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           []byte("invalid"),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varID.Bytes(),
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for invalid flow ID")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
	})

	// Test case 4: Unspecified position (already tested in main TestFlowVariableMove, but more thorough here)
	t.Run("UnspecifiedPosition", func(t *testing.T) {
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
			TargetVariableId: varID.Bytes(),
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
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

	// Test case 5: Non-existent target variable
	t.Run("NonExistentTargetVariable", func(t *testing.T) {
		nonExistentVarID := idwrap.NewNow()
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: nonExistentVarID.Bytes(),
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for non-existent target variable")
		}

		connectErr := err.(*connect.Error)
		// When checking permissions for non-existent target variable,
		// we get CodeNotFound which is the correct behavior
		if connectErr.Code() != connect.CodeNotFound {
			t.Error("Expected CodeNotFound, got:", connectErr.Code())
		}
	})

	// Test case 6: Non-existent flow
	t.Run("NonExistentFlow", func(t *testing.T) {
		nonExistentFlowID := idwrap.NewNow()
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           nonExistentFlowID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varID.Bytes(),
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for non-existent flow")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInternal {
			t.Error("Expected CodeInternal for non-existent flow, got:", connectErr.Code())
		}
	})
}

// TestFlowVariableMoveCrossFlowValidation tests that variables cannot be moved across flows
func TestFlowVariableMoveCrossFlowValidation(t *testing.T) {
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

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create two flows in the same workspace
	flow1ID := idwrap.NewNow()
	flow1 := mflow.Flow{
		ID:          flow1ID,
		WorkspaceID: workspaceID,
		Name:        "Flow 1",
	}
	err := fs.CreateFlow(ctx, flow1)
	if err != nil {
		t.Fatal(err)
	}

	flow2ID := idwrap.NewNow()
	flow2 := mflow.Flow{
		ID:          flow2ID,
		WorkspaceID: workspaceID,
		Name:        "Flow 2",
	}
	err = fs.CreateFlow(ctx, flow2)
	if err != nil {
		t.Fatal(err)
	}

	// Create variables in different flows
	var1ID := idwrap.NewNow()
	var1 := mflowvariable.FlowVariable{
		ID:          var1ID,
		FlowID:      flow1ID,
		Name:        "VAR1",
		Value:       "value1",
		Enabled:     true,
		Description: "Variable in Flow 1",
	}
	err = fvs.CreateFlowVariable(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mflowvariable.FlowVariable{
		ID:          var2ID,
		FlowID:      flow2ID,
		Name:        "VAR2",
		Value:       "value2",
		Enabled:     true,
		Description: "Variable in Flow 2",
	}
	err = fvs.CreateFlowVariable(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	// Try to move variable from flow1 to be after variable in flow2
	req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
		FlowId:           flow1ID.Bytes(),
		VariableId:       var1ID.Bytes(),
		Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetVariableId: var2ID.Bytes(),
	})

	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	_, err = rpcService.FlowVariableMove(authedCtx, req)
	if err == nil {
		t.Error("Expected error for cross-flow move")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Error("Expected CodeInvalidArgument (variables must be in same flow), got:", connectErr.Code())
	}

	expectedMsg := "target flow variable does not belong to the specified flow"
	if connectErr.Message() != expectedMsg {
		t.Error("Expected message:", expectedMsg, "got:", connectErr.Message())
	}
}

// TestFlowVariableMoveEdgeCases tests various edge cases for flow variable moves
func TestFlowVariableMoveEdgeCases(t *testing.T) {
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

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test case 1: Single variable flow (should handle gracefully)
	t.Run("SingleVariableFlow", func(t *testing.T) {
		// Create single variable
		varID := idwrap.NewNow()
		variable := mflowvariable.FlowVariable{
			ID:          varID,
			FlowID:      flowID,
			Name:        "SINGLE_VAR",
			Value:       "single_value",
			Enabled:     true,
			Description: "Single variable",
		}
		err := fvs.CreateFlowVariable(ctx, variable)
		if err != nil {
			t.Fatal(err)
		}

		// Try to move it relative to itself (should fail)
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varID.Bytes(),
		})

		_, err = rpcService.FlowVariableMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for self-referential move in single variable flow")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}

		// Clean up
		err = fvs.DeleteFlowVariable(ctx, varID)
		if err != nil {
			t.Fatal("Failed to clean up variable:", err)
		}
	})

	// Test case 2: Multiple variables with complex ordering
	t.Run("MultipleVariablesComplexOrdering", func(t *testing.T) {
		// Create 4 variables: A, B, C, D
		varAID := idwrap.NewNow()
		varA := mflowvariable.FlowVariable{
			ID:          varAID,
			FlowID:      flowID,
			Name:        "VAR_A",
			Value:       "value_a",
			Enabled:     true,
			Description: "Variable A",
		}
		err := fvs.CreateFlowVariable(ctx, varA)
		if err != nil {
			t.Fatal(err)
		}

		varBID := idwrap.NewNow()
		varB := mflowvariable.FlowVariable{
			ID:          varBID,
			FlowID:      flowID,
			Name:        "VAR_B",
			Value:       "value_b",
			Enabled:     true,
			Description: "Variable B",
		}
		err = fvs.CreateFlowVariable(ctx, varB)
		if err != nil {
			t.Fatal(err)
		}

		varCID := idwrap.NewNow()
		varC := mflowvariable.FlowVariable{
			ID:          varCID,
			FlowID:      flowID,
			Name:        "VAR_C",
			Value:       "value_c",
			Enabled:     true,
			Description: "Variable C",
		}
		err = fvs.CreateFlowVariable(ctx, varC)
		if err != nil {
			t.Fatal(err)
		}

		varDID := idwrap.NewNow()
		varD := mflowvariable.FlowVariable{
			ID:          varDID,
			FlowID:      flowID,
			Name:        "VAR_D",
			Value:       "value_d",
			Enabled:     true,
			Description: "Variable D",
		}
		err = fvs.CreateFlowVariable(ctx, varD)
		if err != nil {
			t.Fatal(err)
		}

		// Initial order should be: A, B, C, D
		variables, err := fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
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
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varDID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varAID.Bytes(),
		})

		_, err = rpcService.FlowVariableMove(authedCtx, req)
		if err != nil {
			t.Fatal("Move operation failed:", err)
		}

		// Verify new order: A, D, B, C
		variables, err = fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
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
		req2 := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varBID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetVariableId: varAID.Bytes(),
		})

		_, err = rpcService.FlowVariableMove(authedCtx, req2)
		if err != nil {
			t.Fatal("Second move operation failed:", err)
		}

		// Verify final order: B, A, D, C
		variables, err = fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
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
			err = fvs.DeleteFlowVariable(ctx, varID)
			if err != nil {
				t.Fatal("Failed to clean up variable:", err)
			}
		}
	})
}

// TestFlowVariableMovePermissionChecks tests permission validation for move operations
func TestFlowVariableMovePermissionChecks(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	fs := sflow.New(queries)
	fvs := sflowvariable.New(queries)

	// Create two different users and workspaces
	workspace1ID := idwrap.NewNow()
	workspaceUser1ID := idwrap.NewNow()
	collection1ID := idwrap.NewNow()
	user1ID := idwrap.NewNow()

	user2ID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspace1ID,
		workspaceUser1ID, user1ID, collection1ID)

	// Create flow for user1
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspace1ID,
		Name:        "User1's Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create variables owned by user1
	var1ID := idwrap.NewNow()
	var1 := mflowvariable.FlowVariable{
		ID:          var1ID,
		FlowID:      flowID,
		Name:        "USER1_VAR1",
		Value:       "value1",
		Enabled:     true,
		Description: "User1's first variable",
	}
	err = fvs.CreateFlowVariable(ctx, var1)
	if err != nil {
		t.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mflowvariable.FlowVariable{
		ID:          var2ID,
		FlowID:      flowID,
		Name:        "USER1_VAR2",
		Value:       "value2",
		Enabled:     true,
		Description: "User1's second variable",
	}
	err = fvs.CreateFlowVariable(ctx, var2)
	if err != nil {
		t.Fatal(err)
	}

	rpcService := rflowvariable.New(db, fs, us, fvs)

	// Test authorized access (user1 can move their own variables)
	t.Run("AuthorizedAccess", func(t *testing.T) {
		user1AuthCtx := mwauth.CreateAuthedContext(ctx, user1ID)
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       var1ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: var2ID.Bytes(),
		})

		resp, err := rpcService.FlowVariableMove(user1AuthCtx, req)
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
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       var1ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: var2ID.Bytes(),
		})

		_, err := rpcService.FlowVariableMove(user2AuthCtx, req)
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

// TestFlowVariableMovePerformance tests the performance of flow variable move operations
func TestFlowVariableMovePerformance(t *testing.T) {
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

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Performance Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test case 1: Single move operation performance (target: <10ms)
	t.Run("SingleMovePerformance", func(t *testing.T) {
		// Create variables
		var1ID := idwrap.NewNow()
		var1 := mflowvariable.FlowVariable{
			ID:          var1ID,
			FlowID:      flowID,
			Name:        "PERF_VAR1",
			Value:       "perf_value1",
			Enabled:     true,
			Description: "Performance test variable 1",
		}
		err := fvs.CreateFlowVariable(ctx, var1)
		if err != nil {
			t.Fatal(err)
		}

		var2ID := idwrap.NewNow()
		var2 := mflowvariable.FlowVariable{
			ID:          var2ID,
			FlowID:      flowID,
			Name:        "PERF_VAR2",
			Value:       "perf_value2",
			Enabled:     true,
			Description: "Performance test variable 2",
		}
		err = fvs.CreateFlowVariable(ctx, var2)
		if err != nil {
			t.Fatal(err)
		}

		// Measure move operation time
		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       var1ID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: var2ID.Bytes(),
		})

		start := time.Now()
		resp, err := rpcService.FlowVariableMove(authedCtx, req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatal("Move operation failed:", err)
		}
		if resp == nil {
			t.Fatal("Expected response")
		}

		// Performance requirement: <10ms
		if elapsed > 10*time.Millisecond {
			t.Errorf("Move operation took too long: %v (target: <10ms)", elapsed)
		} else {
			t.Logf("Move operation completed in %v (target: <10ms)", elapsed)
		}

		// Verify operation was successful
		variables, err := fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
		if err != nil {
			t.Fatal("Failed to get ordered variables:", err)
		}
		if len(variables) != 2 {
			t.Fatal("Expected 2 variables after move")
		}

		// Clean up
		err = fvs.DeleteFlowVariable(ctx, var1ID)
		if err != nil {
			t.Fatal("Failed to clean up var1:", err)
		}
		err = fvs.DeleteFlowVariable(ctx, var2ID)
		if err != nil {
			t.Fatal("Failed to clean up var2:", err)
		}
	})

	// Test case 2: Performance with realistic variable counts (10-50 variables)
	t.Run("RealisticVariableCountPerformance", func(t *testing.T) {
		variableCounts := []int{10, 25, 50}

		for _, count := range variableCounts {
			t.Run(fmt.Sprintf("%d_variables", count), func(t *testing.T) {
				// Create variables
				varIDs := make([]idwrap.IDWrap, count)
				for i := 0; i < count; i++ {
					varID := idwrap.NewNow()
					variable := mflowvariable.FlowVariable{
						ID:          varID,
						FlowID:      flowID,
						Name:        fmt.Sprintf("PERF_VAR_%d", i),
						Value:       fmt.Sprintf("perf_value_%d", i),
						Enabled:     true,
						Description: fmt.Sprintf("Performance test variable %d", i),
					}
					err := fvs.CreateFlowVariable(ctx, variable)
					if err != nil {
						t.Fatal("Failed to create variable:", err)
					}
					varIDs[i] = varID
				}

				// Measure move operation from middle to beginning
				middleIndex := count / 2
				req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
					FlowId:           flowID.Bytes(),
					VariableId:       varIDs[middleIndex].Bytes(),
					Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
					TargetVariableId: varIDs[0].Bytes(),
				})

				start := time.Now()
				resp, err := rpcService.FlowVariableMove(authedCtx, req)
				elapsed := time.Since(start)

				if err != nil {
					t.Fatal("Move operation failed:", err)
				}
				if resp == nil {
					t.Fatal("Expected response")
				}

				// Performance requirement: <10ms even with more variables
				if elapsed > 10*time.Millisecond {
					t.Errorf("Move operation with %d variables took too long: %v (target: <10ms)", count, elapsed)
				} else {
					t.Logf("Move operation with %d variables completed in %v (target: <10ms)", count, elapsed)
				}

				// Verify operation was successful
				variables, err := fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
				if err != nil {
					t.Fatal("Failed to get ordered variables:", err)
				}
				if len(variables) != count {
					t.Fatalf("Expected %d variables after move, got %d", count, len(variables))
				}

				// Clean up all variables
				for _, varID := range varIDs {
					err = fvs.DeleteFlowVariable(ctx, varID)
					if err != nil {
						t.Fatal("Failed to clean up variable:", err)
					}
				}
			})
		}
	})

	// Test case 3: Multiple consecutive move operations performance
	t.Run("ConsecutiveMovesPerformance", func(t *testing.T) {
		// Create 5 variables for multiple move operations
		varIDs := make([]idwrap.IDWrap, 5)
		for i := 0; i < 5; i++ {
			varID := idwrap.NewNow()
			variable := mflowvariable.FlowVariable{
				ID:          varID,
				FlowID:      flowID,
				Name:        fmt.Sprintf("CONSEC_VAR_%d", i),
				Value:       fmt.Sprintf("consec_value_%d", i),
				Enabled:     true,
				Description: fmt.Sprintf("Consecutive move test variable %d", i),
			}
			err := fvs.CreateFlowVariable(ctx, variable)
			if err != nil {
				t.Fatal("Failed to create variable:", err)
			}
			varIDs[i] = varID
		}

		// Perform multiple consecutive moves and measure total time
		start := time.Now()

		// Move var4 to beginning (before var0)
		req1 := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varIDs[4].Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetVariableId: varIDs[0].Bytes(),
		})
		_, err := rpcService.FlowVariableMove(authedCtx, req1)
		if err != nil {
			t.Fatal("First move failed:", err)
		}

		// Move var1 to end (after var2)
		req2 := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varIDs[1].Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varIDs[2].Bytes(),
		})
		_, err = rpcService.FlowVariableMove(authedCtx, req2)
		if err != nil {
			t.Fatal("Second move failed:", err)
		}

		// Move var3 to middle
		req3 := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       varIDs[3].Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: varIDs[0].Bytes(),
		})
		_, err = rpcService.FlowVariableMove(authedCtx, req3)
		if err != nil {
			t.Fatal("Third move failed:", err)
		}

		totalElapsed := time.Since(start)

		// Average per move should be <10ms
		avgPerMove := totalElapsed / 3
		if avgPerMove > 10*time.Millisecond {
			t.Errorf("Average move operation time too long: %v (target: <10ms)", avgPerMove)
		} else {
			t.Logf("Average move operation time: %v (target: <10ms)", avgPerMove)
		}

		// Verify final order is maintained correctly
		variables, err := fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
		if err != nil {
			t.Fatal("Failed to get ordered variables:", err)
		}
		if len(variables) != 5 {
			t.Fatal("Expected 5 variables after consecutive moves")
		}

		// Clean up all variables
		for _, varID := range varIDs {
			err = fvs.DeleteFlowVariable(ctx, varID)
			if err != nil {
				t.Fatal("Failed to clean up variable:", err)
			}
		}
	})
}

// TestFlowVariableCreateMoveListE2E tests the complete Create → Move → List workflow end-to-end
func TestFlowVariableCreateMoveListE2E(t *testing.T) {
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
		Name:        "E2E Test Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// === PHASE 1: CREATE VARIABLES ===
	t.Log("Phase 1: Creating variables")

	// Create 4 variables with distinct names
	var1CreateReq := connect.NewRequest(&flowvariablev1.FlowVariableCreateRequest{
		FlowId:      flowID.Bytes(),
		Name:        "var1",
		Value:       "value1",
		Enabled:     true,
		Description: "First test variable",
	})

	var1CreateResp, err := rpcService.FlowVariableCreate(authedCtx, var1CreateReq)
	if err != nil {
		t.Fatal("Failed to create var1:", err)
	}
	if var1CreateResp.Msg == nil || var1CreateResp.Msg.VariableId == nil {
		t.Fatal("var1 create response invalid")
	}
	var1ID, err := idwrap.NewFromBytes(var1CreateResp.Msg.VariableId)
	if err != nil {
		t.Fatal("Failed to parse var1 ID:", err)
	}

	var2CreateReq := connect.NewRequest(&flowvariablev1.FlowVariableCreateRequest{
		FlowId:      flowID.Bytes(),
		Name:        "var2",
		Value:       "value2",
		Enabled:     true,
		Description: "Second test variable",
	})

	var2CreateResp, err := rpcService.FlowVariableCreate(authedCtx, var2CreateReq)
	if err != nil {
		t.Fatal("Failed to create var2:", err)
	}
	if var2CreateResp.Msg == nil || var2CreateResp.Msg.VariableId == nil {
		t.Fatal("var2 create response invalid")
	}
	var2ID, err := idwrap.NewFromBytes(var2CreateResp.Msg.VariableId)
	if err != nil {
		t.Fatal("Failed to parse var2 ID:", err)
	}

	var3CreateReq := connect.NewRequest(&flowvariablev1.FlowVariableCreateRequest{
		FlowId:      flowID.Bytes(),
		Name:        "var3",
		Value:       "value3",
		Enabled:     true,
		Description: "Third test variable",
	})

	var3CreateResp, err := rpcService.FlowVariableCreate(authedCtx, var3CreateReq)
	if err != nil {
		t.Fatal("Failed to create var3:", err)
	}
	if var3CreateResp.Msg == nil || var3CreateResp.Msg.VariableId == nil {
		t.Fatal("var3 create response invalid")
	}
	var3ID, err := idwrap.NewFromBytes(var3CreateResp.Msg.VariableId)
	if err != nil {
		t.Fatal("Failed to parse var3 ID:", err)
	}

	var4CreateReq := connect.NewRequest(&flowvariablev1.FlowVariableCreateRequest{
		FlowId:      flowID.Bytes(),
		Name:        "var4",
		Value:       "value4",
		Enabled:     true,
		Description: "Fourth test variable",
	})

	var4CreateResp, err := rpcService.FlowVariableCreate(authedCtx, var4CreateReq)
	if err != nil {
		t.Fatal("Failed to create var4:", err)
	}
	if var4CreateResp.Msg == nil || var4CreateResp.Msg.VariableId == nil {
		t.Fatal("var4 create response invalid")
	}
	var4ID, err := idwrap.NewFromBytes(var4CreateResp.Msg.VariableId)
	if err != nil {
		t.Fatal("Failed to parse var4 ID:", err)
	}

	t.Log("Created all 4 variables successfully")

	// === PHASE 2: INITIAL LIST VERIFICATION ===
	t.Log("Phase 2: Verifying initial list order")

	initialListReq := connect.NewRequest(&flowvariablev1.FlowVariableListRequest{
		FlowId: flowID.Bytes(),
	})

	initialListResp, err := rpcService.FlowVariableList(authedCtx, initialListReq)
	if err != nil {
		t.Fatal("Failed to list variables initially:", err)
	}

	if initialListResp.Msg == nil {
		t.Fatal("Initial list response is nil")
	}

	if len(initialListResp.Msg.Items) != 4 {
		t.Fatalf("Expected 4 variables in initial list, got %d", len(initialListResp.Msg.Items))
	}

	// Verify initial order should be creation order: var1, var2, var3, var4
	expectedInitialOrder := []idwrap.IDWrap{var1ID, var2ID, var3ID, var4ID}
	expectedInitialNames := []string{"var1", "var2", "var3", "var4"}
	
	for i, item := range initialListResp.Msg.Items {
		itemID, err := idwrap.NewFromBytes(item.VariableId)
		if err != nil {
			t.Fatal("Failed to parse item ID:", err)
		}
		if itemID.Compare(expectedInitialOrder[i]) != 0 {
			t.Errorf("Initial order incorrect at position %d: expected %s, got %s", 
				i, expectedInitialOrder[i].String(), itemID.String())
		}
		if item.Name != expectedInitialNames[i] {
			t.Errorf("Initial name incorrect at position %d: expected %s, got %s", 
				i, expectedInitialNames[i], item.Name)
		}
		// Verify other variable data is preserved
		expectedValue := fmt.Sprintf("value%d", i+1)
		if item.Value != expectedValue {
			t.Errorf("Initial value incorrect at position %d: expected %s, got %s", 
				i, expectedValue, item.Value)
		}
		if !item.Enabled {
			t.Errorf("Initial enabled incorrect at position %d: expected true, got false", i)
		}
		expectedDesc := fmt.Sprintf("%s test variable", []string{"First", "Second", "Third", "Fourth"}[i])
		if item.Description != expectedDesc {
			t.Errorf("Initial description incorrect at position %d: expected %s, got %s", 
				i, expectedDesc, item.Description)
		}
	}

	t.Log("Initial list order verified successfully")

	// === PHASE 3: MOVE OPERATIONS ===
	t.Log("Phase 3: Performing move operations")

	// Move var1 to AFTER var3 (should reorder to: var2, var3, var1, var4)
	move1Req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
		FlowId:           flowID.Bytes(),
		VariableId:       var1ID.Bytes(),
		TargetVariableId: var3ID.Bytes(),
		Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
	})

	move1Resp, err := rpcService.FlowVariableMove(authedCtx, move1Req)
	if err != nil {
		t.Fatal("Failed to move var1 after var3:", err)
	}
	if move1Resp.Msg == nil {
		t.Fatal("Move1 response is nil")
	}

	t.Log("Moved var1 after var3 successfully")

	// Move var4 to BEFORE var2 (should reorder to: var4, var2, var3, var1)
	move2Req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
		FlowId:           flowID.Bytes(),
		VariableId:       var4ID.Bytes(),
		TargetVariableId: var2ID.Bytes(),
		Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
	})

	move2Resp, err := rpcService.FlowVariableMove(authedCtx, move2Req)
	if err != nil {
		t.Fatal("Failed to move var4 before var2:", err)
	}
	if move2Resp.Msg == nil {
		t.Fatal("Move2 response is nil")
	}

	t.Log("Moved var4 before var2 successfully")

	// === PHASE 4: FINAL LIST VERIFICATION ===
	t.Log("Phase 4: Verifying final list order after moves")

	finalListReq := connect.NewRequest(&flowvariablev1.FlowVariableListRequest{
		FlowId: flowID.Bytes(),
	})

	finalListResp, err := rpcService.FlowVariableList(authedCtx, finalListReq)
	if err != nil {
		t.Fatal("Failed to list variables after moves:", err)
	}

	if finalListResp.Msg == nil {
		t.Fatal("Final list response is nil")
	}

	if len(finalListResp.Msg.Items) != 4 {
		t.Fatalf("Expected 4 variables in final list, got %d", len(finalListResp.Msg.Items))
	}

	// Verify final order: var4, var2, var3, var1
	expectedFinalOrder := []idwrap.IDWrap{var4ID, var2ID, var3ID, var1ID}
	expectedFinalNames := []string{"var4", "var2", "var3", "var1"}
	expectedFinalValues := []string{"value4", "value2", "value3", "value1"}
	expectedFinalDescs := []string{"Fourth test variable", "Second test variable", "Third test variable", "First test variable"}

	for i, item := range finalListResp.Msg.Items {
		itemID, err := idwrap.NewFromBytes(item.VariableId)
		if err != nil {
			t.Fatal("Failed to parse final item ID:", err)
		}
		if itemID.Compare(expectedFinalOrder[i]) != 0 {
			t.Errorf("Final order incorrect at position %d: expected %s, got %s", 
				i, expectedFinalOrder[i].String(), itemID.String())
		}
		if item.Name != expectedFinalNames[i] {
			t.Errorf("Final name incorrect at position %d: expected %s, got %s", 
				i, expectedFinalNames[i], item.Name)
		}
		if item.Value != expectedFinalValues[i] {
			t.Errorf("Final value incorrect at position %d: expected %s, got %s", 
				i, expectedFinalValues[i], item.Value)
		}
		if !item.Enabled {
			t.Errorf("Final enabled incorrect at position %d: expected true, got false", i)
		}
		if item.Description != expectedFinalDescs[i] {
			t.Errorf("Final description incorrect at position %d: expected %s, got %s", 
				i, expectedFinalDescs[i], item.Description)
		}
	}

	t.Log("Final list order verified successfully")

	// === PHASE 5: COMPREHENSIVE VALIDATION ===
	t.Log("Phase 5: Performing comprehensive validation")

	// Verify no variables were lost or duplicated
	variableIDs := make(map[string]bool)
	variableNames := make(map[string]bool)
	
	for _, item := range finalListResp.Msg.Items {
		itemID, err := idwrap.NewFromBytes(item.VariableId)
		if err != nil {
			t.Fatal("Failed to parse validation item ID:", err)
		}
		
		idStr := itemID.String()
		if variableIDs[idStr] {
			t.Errorf("Duplicate variable ID found: %s", idStr)
		}
		variableIDs[idStr] = true
		
		if variableNames[item.Name] {
			t.Errorf("Duplicate variable name found: %s", item.Name)
		}
		variableNames[item.Name] = true
	}

	// Verify all original variables are still present
	originalIDs := []idwrap.IDWrap{var1ID, var2ID, var3ID, var4ID}
	for _, originalID := range originalIDs {
		if !variableIDs[originalID.String()] {
			t.Errorf("Original variable missing after moves: %s", originalID.String())
		}
	}

	// Verify all original names are still present
	originalNames := []string{"var1", "var2", "var3", "var4"}
	for _, originalName := range originalNames {
		if !variableNames[originalName] {
			t.Errorf("Original variable name missing after moves: %s", originalName)
		}
	}

	t.Log("Comprehensive validation passed")

	// === PHASE 6: ADDITIONAL MOVE AND RE-VERIFICATION ===
	t.Log("Phase 6: Additional move to test linked-list integrity")

	// Move var3 to the beginning (before var4) - final order should be: var3, var4, var2, var1
	move3Req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
		FlowId:           flowID.Bytes(),
		VariableId:       var3ID.Bytes(),
		TargetVariableId: var4ID.Bytes(),
		Position:         resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
	})

	move3Resp, err := rpcService.FlowVariableMove(authedCtx, move3Req)
	if err != nil {
		t.Fatal("Failed to move var3 before var4:", err)
	}
	if move3Resp.Msg == nil {
		t.Fatal("Move3 response is nil")
	}

	// Verify the final final order
	finalFinalListReq := connect.NewRequest(&flowvariablev1.FlowVariableListRequest{
		FlowId: flowID.Bytes(),
	})

	finalFinalListResp, err := rpcService.FlowVariableList(authedCtx, finalFinalListReq)
	if err != nil {
		t.Fatal("Failed to list variables after final move:", err)
	}

	if len(finalFinalListResp.Msg.Items) != 4 {
		t.Fatalf("Expected 4 variables in final final list, got %d", len(finalFinalListResp.Msg.Items))
	}

	// Verify final final order: var3, var4, var2, var1
	expectedFinalFinalOrder := []idwrap.IDWrap{var3ID, var4ID, var2ID, var1ID}
	expectedFinalFinalNames := []string{"var3", "var4", "var2", "var1"}

	for i, item := range finalFinalListResp.Msg.Items {
		itemID, err := idwrap.NewFromBytes(item.VariableId)
		if err != nil {
			t.Fatal("Failed to parse final final item ID:", err)
		}
		if itemID.Compare(expectedFinalFinalOrder[i]) != 0 {
			t.Errorf("Final final order incorrect at position %d: expected %s, got %s", 
				i, expectedFinalFinalOrder[i].String(), itemID.String())
		}
		if item.Name != expectedFinalFinalNames[i] {
			t.Errorf("Final final name incorrect at position %d: expected %s, got %s", 
				i, expectedFinalFinalNames[i], item.Name)
		}
	}

	t.Log("Additional move and re-verification completed successfully")

	t.Log("=== E2E TEST COMPLETED SUCCESSFULLY ===")
	t.Log("✅ Create: Created 4 variables with distinct data")
	t.Log("✅ List Initial: Verified creation order and data integrity")  
	t.Log("✅ Move Operations: Performed 3 different move operations")
	t.Log("✅ List Final: Verified move effects on ordering")
	t.Log("✅ Data Preservation: All variable data preserved through moves")
	t.Log("✅ Linked-List Integrity: Order maintained consistently")
	t.Log("✅ No Duplication/Loss: All variables accounted for")
}

// BenchmarkFlowVariableMove benchmarks the flow variable move operation
func BenchmarkFlowVariableMove(b *testing.B) {
	ctx := context.Background()
	// Use a dummy *testing.T for setup since testutil.CreateBaseDB expects *testing.T
	dummyT := &testing.T{}
	base := testutil.CreateBaseDB(ctx, dummyT)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	fs := sflow.New(queries)
	fvs := sflowvariable.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(dummyT, ctx, workspaceID,
		workspaceUserID, userID, collectionID)

	// Create flow
	flowID := idwrap.NewNow()
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Benchmark Flow",
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		b.Fatal(err)
	}

	// Create variables for benchmarking
	var1ID := idwrap.NewNow()
	var1 := mflowvariable.FlowVariable{
		ID:          var1ID,
		FlowID:      flowID,
		Name:        "BENCH_VAR1",
		Value:       "bench_value1",
		Enabled:     true,
		Description: "Benchmark variable 1",
	}
	err = fvs.CreateFlowVariable(ctx, var1)
	if err != nil {
		b.Fatal(err)
	}

	var2ID := idwrap.NewNow()
	var2 := mflowvariable.FlowVariable{
		ID:          var2ID,
		FlowID:      flowID,
		Name:        "BENCH_VAR2",
		Value:       "bench_value2",
		Enabled:     true,
		Description: "Benchmark variable 2",
	}
	err = fvs.CreateFlowVariable(ctx, var2)
	if err != nil {
		b.Fatal(err)
	}

	rpcService := rflowvariable.New(db, fs, us, fvs)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Alternate between moving var1 after var2 and var2 after var1
		var sourceID, targetID idwrap.IDWrap
		if i%2 == 0 {
			sourceID, targetID = var1ID, var2ID
		} else {
			sourceID, targetID = var2ID, var1ID
		}

		req := connect.NewRequest(&flowvariablev1.FlowVariableMoveRequest{
			FlowId:           flowID.Bytes(),
			VariableId:       sourceID.Bytes(),
			Position:         resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetVariableId: targetID.Bytes(),
		})

		_, err := rpcService.FlowVariableMove(authedCtx, req)
		if err != nil {
			b.Fatal("Move operation failed:", err)
		}
	}
}
