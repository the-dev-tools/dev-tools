package rcollectionitem_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/testutil"
)

// TestFreshDatabaseCollectionItemCreation validates that collection items can be created successfully
// in a completely fresh database without foreign key constraint failures.
//
// This test specifically addresses the issue that was occurring when users deleted their database
// and tried to create new collections/folders/endpoints, which was failing with:
// "SQLite failure: `FOREIGN KEY constraint failed`"
func TestFreshDatabaseCollectionItemCreation(t *testing.T) {
	// Create a completely fresh in-memory database to simulate user's scenario
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	db := base.DB
	mockLogger := mocklogger.NewMockLogger()

	// Setup test infrastructure like other tests
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Initialize services we need for the test
	cis := scollectionitem.New(base.Queries, mockLogger)

	// Test the core issue: Creating collection items in fresh database without foreign key constraint failures
	
	// Step 1: Create a folder directly using the service (this was failing with FK constraint)
	t.Log("Step 1: Creating folder using CollectionItemService (this was failing with FK constraints)...")
	
	folderID := idwrap.NewNow()
	folder := &mitemfolder.ItemFolder{
		ID:           folderID,
		CollectionID: collectionID,
		ParentID:     nil, // Root level
		Name:         "Authentication",
	}

	tx, err := db.Begin()
	require.NoError(t, err, "Transaction creation should succeed")
	defer tx.Rollback()

	// This is the critical test - CreateFolderTX was failing with foreign key constraints
	err = cis.CreateFolderTX(ctx, tx, folder)
	require.NoError(t, err, "Folder creation should succeed without foreign key constraint failures")
	
	err = tx.Commit()
	require.NoError(t, err, "Transaction commit should succeed")

	t.Log("✅ Step 1: Folder created successfully without foreign key constraint errors")

	// Step 2: Create an endpoint inside the folder (also was failing)
	t.Log("Step 2: Creating endpoint inside folder using CollectionItemService...")
	
	endpointID := idwrap.NewNow()
	endpoint := &mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: collectionID,
		FolderID:     &folderID, // Inside the folder we just created
		Name:         "Login",
		Url:          "/auth/login",
		Method:       "POST",
	}

	tx2, err := db.Begin()
	require.NoError(t, err, "Second transaction creation should succeed")
	defer tx2.Rollback()

	// This was also failing with foreign key constraints
	err = cis.CreateEndpointTX(ctx, tx2, endpoint)
	require.NoError(t, err, "Endpoint creation should succeed without foreign key constraint failures")
	
	err = tx2.Commit()
	require.NoError(t, err, "Second transaction commit should succeed")

	t.Log("✅ Step 2: Endpoint created successfully without foreign key constraint errors")

	// Step 3: Create a root-level endpoint
	t.Log("Step 3: Creating root-level endpoint...")
	
	rootEndpointID := idwrap.NewNow()
	rootEndpoint := &mitemapi.ItemApi{
		ID:           rootEndpointID,
		CollectionID: collectionID,
		FolderID:     nil, // Root level
		Name:         "Health Check",
		Url:          "/health",
		Method:       "GET",
	}

	tx3, err := db.Begin()
	require.NoError(t, err, "Third transaction creation should succeed")
	defer tx3.Rollback()

	err = cis.CreateEndpointTX(ctx, tx3, rootEndpoint)
	require.NoError(t, err, "Root endpoint creation should succeed")
	
	err = tx3.Commit()
	require.NoError(t, err, "Third transaction commit should succeed")

	t.Log("✅ Step 3: Root-level endpoint created successfully")

	// Step 4: Verify move operations work (testing cross-folder functionality)
	t.Log("Step 4: Testing move operations...")
	
	// Get collection item IDs for the move operation
	rootEndpointCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, rootEndpointID)
	require.NoError(t, err, "Should be able to get collection item ID for root endpoint")
	
	folderCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, folderID) 
	require.NoError(t, err, "Should be able to get collection item ID for folder")

	// Test move operation
	err = cis.MoveCollectionItem(ctx, rootEndpointCollectionItemID, &folderCollectionItemID, movable.MovePositionBefore)
	require.NoError(t, err, "Move operation should work without foreign key constraint issues")

	t.Log("✅ Step 4: Move operations work correctly")

	t.Log("🎉 Fresh Database E2E Test Passed!")
	t.Log("✅ All collection item creation operations work without foreign key constraint failures")
	t.Log("✅ Folder creation successful")
	t.Log("✅ Endpoint creation successful") 
	t.Log("✅ Move operations function properly")
	t.Log("✅ Reference-based architecture maintains data integrity")
}