package svar

import (
	"context"
	"log/slog"
	"testing"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
)

// TestVarServiceStructure tests that the VarService can be instantiated with the new structure
func TestVarServiceStructure(t *testing.T) {
	// Mock queries - in a real test you'd use a test database
	var queries *gen.Queries = nil
	logger := slog.Default()
	
	// Test service creation
	service := New(queries, logger)
	
	// Verify service has expected fields
	if service.queries != queries {
		t.Errorf("Expected queries to be set")
	}
	if service.logger != logger {
		t.Errorf("Expected logger to be set")
	}
	if service.linkedListManager == nil {
		t.Errorf("Expected linkedListManager to be initialized")
	}
	if service.movableRepository == nil {
		t.Errorf("Expected movableRepository to be initialized")
	}
}

// TestErrorDefinitions tests that error constants are properly defined
func TestErrorDefinitions(t *testing.T) {
	// Test that all error constants are defined and not nil
	if ErrNoVarFound == nil {
		t.Errorf("Expected ErrNoVarFound to be defined")
	}
	if ErrInvalidMoveOperation == nil {
		t.Errorf("Expected ErrInvalidMoveOperation to be defined")  
	}
	if ErrEnvironmentBoundaryViolation == nil {
		t.Errorf("Expected ErrEnvironmentBoundaryViolation to be defined")
	}
	if ErrSelfReferentialMove == nil {
		t.Errorf("Expected ErrSelfReferentialMove to be defined")
	}
}

// TestValidateMoveOperation tests the move validation logic
func TestValidateMoveOperation(t *testing.T) {
	logger := slog.Default()
	service := New(nil, logger)
	ctx := context.Background()
	
	// Create test IDs
	varID1 := idwrap.NewTextMust("01HPQR2S3T4U5V6W7X8Y9Z0ABC")
	varID2 := idwrap.NewTextMust("01HPQR2S3T4U5V6W7X8Y9Z0DEF")
	
	// Test valid move operation
	err := service.validateMoveOperation(ctx, varID1, varID2)
	if err != nil {
		t.Errorf("Expected valid move operation to pass, got: %v", err)
	}
	
	// Test self-referential move (should fail)
	err = service.validateMoveOperation(ctx, varID1, varID1)
	if err != ErrSelfReferentialMove {
		t.Errorf("Expected ErrSelfReferentialMove, got: %v", err)
	}
}

// TestConversionFunctions tests the model conversion functions
func TestConversionFunctions(t *testing.T) {
	// Create test variable model
	testID := idwrap.NewTextMust("01HPQR2S3T4U5V6W7X8Y9Z0ABC")
	testEnvID := idwrap.NewTextMust("01HPQR2S3T4U5V6W7X8Y9Z0DEF")
	
	testVar := mvar.Var{
		ID:          testID,
		EnvID:       testEnvID,
		VarKey:      "TEST_KEY",
		Value:       "test_value",
		Enabled:     true,
		Description: "Test variable",
	}
	
	// Test model to DB conversion
	dbVar := ConvertToDBVar(testVar)
	if dbVar.ID.Compare(testVar.ID) != 0 {
		t.Errorf("ID conversion failed")
	}
	if dbVar.EnvID.Compare(testVar.EnvID) != 0 {
		t.Errorf("EnvID conversion failed")
	}
	if dbVar.VarKey != testVar.VarKey {
		t.Errorf("VarKey conversion failed")
	}
	if dbVar.Value != testVar.Value {
		t.Errorf("Value conversion failed")
	}
	if dbVar.Enabled != testVar.Enabled {
		t.Errorf("Enabled conversion failed")
	}
	if dbVar.Description != testVar.Description {
		t.Errorf("Description conversion failed")
	}
	
	// Test DB to model conversion
	modelVar := ConvertToModelVar(dbVar)
	if modelVar.ID.Compare(testVar.ID) != 0 {
		t.Errorf("ID back-conversion failed")
	}
	if modelVar.EnvID.Compare(testVar.EnvID) != 0 {
		t.Errorf("EnvID back-conversion failed")
	}
	if modelVar.VarKey != testVar.VarKey {
		t.Errorf("VarKey back-conversion failed")
	}
	if modelVar.Value != testVar.Value {
		t.Errorf("Value back-conversion failed")
	}
	if modelVar.Enabled != testVar.Enabled {
		t.Errorf("Enabled back-conversion failed")
	}
	if modelVar.Description != testVar.Description {
		t.Errorf("Description back-conversion failed")
	}
}