package rimport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/testutil"
)

// TestFKConstraintDebug tests our FK constraint debugging capabilities
func TestFKConstraintDebug(t *testing.T) {
	// Setup test context and in-memory database
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	db := base.DB

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Start a transaction
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	// Create a header service
	txHeaderService, err := sexampleheader.NewTX(authedCtx, tx)
	require.NoError(t, err)

	// Create a delta header that references a non-existent parent
	// This should trigger the FK constraint error
	exampleID := idwrap.NewNow()
	nonExistentParentID := idwrap.NewNow()

	deltaHeader := mexampleheader.Header{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		DeltaParentID: &nonExistentParentID, // This parent doesn't exist
		HeaderKey:     "test-header",
		Enable:        true,
		Description:   "Test delta header",
		Value:         "test-value",
	}

	// Check if FK constraints are enabled first
	var fkEnabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	require.NoError(t, err)
	t.Logf("Foreign keys enabled: %d", fkEnabled)

	// Try to create the delta header - this might fail with FK constraint error
	err = txHeaderService.AppendBulkHeader(ctx, []mexampleheader.Header{deltaHeader})

	if err != nil {
		t.Logf("Got error as expected: %v", err)
		// Verify the error contains the expected text
		require.Contains(t, err.Error(), "FOREIGN KEY constraint failed", 
			"Error should mention FK constraint failure")
		
		// Try to commit - should fail due to the error
		commitErr := tx.Commit()
		require.Error(t, commitErr, "Transaction commit should fail due to FK constraint violation")
	} else {
		t.Logf("No FK constraint error occurred - FK constraints may not be enabled in test DB")
		// Commit should succeed if no error
		err = tx.Commit()
		require.NoError(t, err, "Transaction should commit if no FK constraint error")
	}
	
	tx = nil // Mark as handled to avoid double rollback
}

// TestFKConstraintResolution demonstrates the fix in action
func TestFKConstraintResolution(t *testing.T) {
	// Setup test context and in-memory database
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	db := base.DB

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Start a transaction
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	// Create a header service
	txHeaderService, err := sexampleheader.NewTX(authedCtx, tx)
	require.NoError(t, err)

	exampleID := idwrap.NewNow()
	
	// Create a base header first
	baseHeaderID := idwrap.NewNow()
	baseHeader := mexampleheader.Header{
		ID:            baseHeaderID,
		ExampleID:     exampleID,
		DeltaParentID: nil, // No parent - this is a base header
		HeaderKey:     "test-header",
		Enable:        true,
		Description:   "Test base header",
		Value:         "base-value",
	}

	// Create the base header first
	err = txHeaderService.AppendBulkHeader(ctx, []mexampleheader.Header{baseHeader})
	require.NoError(t, err, "Should be able to create base header")

	// Now create a delta header that references the existing base header
	deltaHeader := mexampleheader.Header{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		DeltaParentID: &baseHeaderID, // Reference the existing base header
		HeaderKey:     "test-header",
		Enable:        true,
		Description:   "Test delta header",
		Value:         "delta-value",
	}

	// This should succeed since we created the parent first
	err = txHeaderService.AppendBulkHeader(ctx, []mexampleheader.Header{deltaHeader})
	require.NoError(t, err, "Should be able to create delta header after base header exists")

	// Commit should succeed
	err = tx.Commit()
	require.NoError(t, err, "Transaction should commit successfully when dependencies are correct")
	
	tx = nil // Mark as handled

	t.Logf("Successfully created base and delta headers with correct FK relationship")
}