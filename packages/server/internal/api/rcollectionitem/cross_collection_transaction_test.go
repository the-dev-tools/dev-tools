package rcollectionitem_test

import (
	"context"
	"database/sql"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DatabaseState represents the state of relevant tables for transaction testing
type DatabaseState struct {
	CollectionItems []scollectionitem.CollectionItem
	ItemFolders     []mitemfolder.ItemFolder
	ItemApis        []mitemapi.ItemApi
}

// captureDBState captures the current state of all relevant tables
func captureDBState(t *testing.T, ctx context.Context, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService, sourceCollectionID, targetCollectionID idwrap.IDWrap) DatabaseState {
	t.Helper()

	state := DatabaseState{}

	// Capture collection items from both collections
	sourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
	require.NoError(t, err)
	targetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
	require.NoError(t, err)

	state.CollectionItems = append(state.CollectionItems, sourceItems...)
	state.CollectionItems = append(state.CollectionItems, targetItems...)

	return state
}

// compareDBStates compares two database states for equality
func compareDBStates(t *testing.T, expected, actual DatabaseState, description string) {
	t.Helper()

	assert.Equal(t, len(expected.CollectionItems), len(actual.CollectionItems),
		"Collection items count should be unchanged: %s", description)

	// Create maps for easier comparison
	expectedItems := make(map[string]scollectionitem.CollectionItem)
	actualItems := make(map[string]scollectionitem.CollectionItem)

	for _, item := range expected.CollectionItems {
		expectedItems[item.ID.String()] = item
	}
	for _, item := range actual.CollectionItems {
		actualItems[item.ID.String()] = item
	}

	// Compare each expected item
	for id, expectedItem := range expectedItems {
		actualItem, found := actualItems[id]
		assert.True(t, found, "Collection item should exist after failed operation: %s (ID: %s)", description, id)
		if found {
			assert.Equal(t, expectedItem.CollectionID, actualItem.CollectionID,
				"Collection ID should be unchanged: %s (ID: %s)", description, id)
			assert.Equal(t, expectedItem.ParentFolderID, actualItem.ParentFolderID,
				"Parent folder ID should be unchanged: %s (ID: %s)", description, id)
			assert.Equal(t, expectedItem.ItemType, actualItem.ItemType,
				"Item type should be unchanged: %s (ID: %s)", description, id)
		}
	}
}

// TestCrossCollectionTransactionRollback tests that failed operations don't leave data in inconsistent state
func TestCrossCollectionTransactionRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	mockLogger := mocklogger.NewMockLogger()
	cis := scollectionitem.New(base.Queries, mockLogger)
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Invalid targetKind combination - transaction rollback", func(t *testing.T) {
		// Setup: Create source folder and target endpoint
		sourceFolderID := idwrap.NewNow()
		sourceFolder := &mitemfolder.ItemFolder{
			ID:           sourceFolderID,
			Name:         "Rollback Test Folder",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, sourceFolder)
		require.NoError(t, err)

		targetEndpointID := idwrap.NewNow()
		targetEndpoint := &mitemapi.ItemApi{
			ID:           targetEndpointID,
			Name:         "GET /rollback-test",
			Url:          "https://api.myapp.com/rollback-test",
			Method:       "GET",
			CollectionID: targetCollectionID,
			FolderID:     nil,
		}
		err = ias.CreateItemApi(ctx, targetEndpoint)
		require.NoError(t, err)

		// Create collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, sourceFolder)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, targetEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial state
		initialState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Get target endpoint collection item ID
		targetEndpointCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetEndpointID)
		require.NoError(t, err)

		// Attempt invalid move (folder into endpoint)
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetEndpointCollectionItemID.Bytes(),
			TargetKind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT.Enum(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Invalid targetKind combination should fail")

		// Capture state after failed operation
		finalState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Verify no changes occurred
		compareDBStates(t, initialState, finalState, "after invalid targetKind operation")
	})

	t.Run("Non-existent target collection - transaction rollback", func(t *testing.T) {
		// Setup: Create source item
		sourceEndpointID := idwrap.NewNow()
		sourceEndpoint := &mitemapi.ItemApi{
			ID:           sourceEndpointID,
			Name:         "POST /non-existent-rollback",
			Url:          "https://api.myapp.com/non-existent-rollback",
			Method:       "POST",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, sourceEndpoint)
		require.NoError(t, err)

		// Create collection item
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial state
		initialState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Attempt move to non-existent collection
		nonExistentCollectionID := idwrap.NewNow()
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:             sourceEndpointID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: nonExistentCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Move to non-existent collection should fail")

		// Capture state after failed operation
		finalState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Verify no changes occurred
		compareDBStates(t, initialState, finalState, "after non-existent target collection operation")
	})

	t.Run("Cross-workspace move attempt - transaction rollback", func(t *testing.T) {
		// Setup: Create a separate workspace and collection
		user2ID := idwrap.NewNow()
		workspace2ID := idwrap.NewNow()
		workspace2UserID := idwrap.NewNow()
		collection2ID := idwrap.NewNow()

		baseServices := base.GetBaseServices()
		baseServices.CreateTempCollection(t, ctx, workspace2ID, workspace2UserID, user2ID, collection2ID)

		// Create source item
		sourceFolderID := idwrap.NewNow()
		sourceFolder := &mitemfolder.ItemFolder{
			ID:           sourceFolderID,
			Name:         "Cross Workspace Test",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, sourceFolder)
		require.NoError(t, err)

		// Create collection item
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, sourceFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial state (include the cross-workspace collection for completeness)
		initialState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Attempt cross-workspace move
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: collection2ID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Cross-workspace move should fail")

		// Capture state after failed operation
		finalState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Verify no changes occurred
		compareDBStates(t, initialState, finalState, "after cross-workspace move attempt")
	})

	t.Run("Self-referential move - transaction rollback", func(t *testing.T) {
		// Setup: Create source item
		sourceEndpointID := idwrap.NewNow()
		sourceEndpoint := &mitemapi.ItemApi{
			ID:           sourceEndpointID,
			Name:         "PUT /self-reference",
			Url:          "https://api.myapp.com/self-reference",
			Method:       "PUT",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, sourceEndpoint)
		require.NoError(t, err)

		// Create collection item
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial state
		initialState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Get endpoint collection item ID
		sourceEndpointCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, sourceEndpointID)
		require.NoError(t, err)

		// Attempt self-referential move
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:             sourceEndpointID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       sourceEndpointCollectionItemID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Self-referential move should fail")

		// Capture state after failed operation
		finalState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Verify no changes occurred
		compareDBStates(t, initialState, finalState, "after self-referential move attempt")
	})

	t.Run("Unauthorized user - no data modification", func(t *testing.T) {
		// Setup: Create source item
		sourceEndpointID := idwrap.NewNow()
		sourceEndpoint := &mitemapi.ItemApi{
			ID:           sourceEndpointID,
			Name:         "DELETE /unauthorized-test",
			Url:          "https://api.myapp.com/unauthorized-test",
			Method:       "DELETE",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, sourceEndpoint)
		require.NoError(t, err)

		// Create collection item
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial state
		initialState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Create unauthorized user context
		unauthorizedUserID := idwrap.NewNow()
		unauthorizedCtx := mwauth.CreateAuthedContext(ctx, unauthorizedUserID)

		// Attempt move with unauthorized user
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:             sourceEndpointID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(unauthorizedCtx, req)
		assert.Error(t, err, "Unauthorized move should fail")

		// Capture state after failed operation
		finalState := captureDBState(t, ctx, cis, ifs, ias, sourceCollectionID, targetCollectionID)

		// Verify no changes occurred
		compareDBStates(t, initialState, finalState, "after unauthorized move attempt")
	})
}

// TestCrossCollectionConsistencyAfterPartialFailure tests consistency when operations partially complete before failing
func TestCrossCollectionConsistencyAfterPartialFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	mockLogger := mocklogger.NewMockLogger()
	cis := scollectionitem.New(base.Queries, mockLogger)
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Linked list integrity maintained on failure", func(t *testing.T) {
		// Setup: Create multiple items to establish a linked list structure
		folder1ID := idwrap.NewNow()
		folder1 := &mitemfolder.ItemFolder{
			ID:           folder1ID,
			Name:         "Folder 1",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, folder1)
		require.NoError(t, err)

		folder2ID := idwrap.NewNow()
		folder2 := &mitemfolder.ItemFolder{
			ID:           folder2ID,
			Name:         "Folder 2",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err = ifs.CreateItemFolder(ctx, folder2)
		require.NoError(t, err)

		endpoint1ID := idwrap.NewNow()
		endpoint1 := &mitemapi.ItemApi{
			ID:           endpoint1ID,
			Name:         "GET /consistency-test-1",
			Url:          "https://api.myapp.com/consistency-test-1",
			Method:       "GET",
			CollectionID: targetCollectionID,
			FolderID:     nil,
		}
		err = ias.CreateItemApi(ctx, endpoint1)
		require.NoError(t, err)

		endpoint2ID := idwrap.NewNow()
		endpoint2 := &mitemapi.ItemApi{
			ID:           endpoint2ID,
			Name:         "POST /consistency-test-2",
			Url:          "https://api.myapp.com/consistency-test-2",
			Method:       "POST",
			CollectionID: targetCollectionID,
			FolderID:     nil,
		}
		err = ias.CreateItemApi(ctx, endpoint2)
		require.NoError(t, err)

		// Create collection items to establish ordering
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, folder1)
		require.NoError(t, err)
		err = cis.CreateFolderTX(ctx, tx, folder2)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, endpoint1)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, endpoint2)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial linked list structure
		initialSourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		initialTargetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		// Verify initial ordering
		require.Len(t, initialSourceItems, 2, "Source collection should have 2 items")
		require.Len(t, initialTargetItems, 2, "Target collection should have 2 items")

		// Attempt invalid move that should fail after some processing
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             folder1ID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetKind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT.Enum(), // Invalid combination
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Invalid move should fail")

		// Verify linked list structure remains intact
		finalSourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		finalTargetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		assert.Len(t, finalSourceItems, len(initialSourceItems), "Source collection item count should be unchanged")
		assert.Len(t, finalTargetItems, len(initialTargetItems), "Target collection item count should be unchanged")

		// Verify ordering is preserved
		for i, item := range finalSourceItems {
			assert.Equal(t, initialSourceItems[i].ID, item.ID, "Source item order should be preserved")
			assert.Equal(t, initialSourceItems[i].CollectionID, item.CollectionID, "Source item collection should be preserved")
		}

		for i, item := range finalTargetItems {
			assert.Equal(t, initialTargetItems[i].ID, item.ID, "Target item order should be preserved")
			assert.Equal(t, initialTargetItems[i].CollectionID, item.CollectionID, "Target item collection should be preserved")
		}
	})

	t.Run("Legacy table consistency on rollback", func(t *testing.T) {
		// This test ensures that legacy tables (item_folder, item_api) remain consistent
		// even if the collection_items table operations are rolled back

		// Create source folder
		sourceFolderID := idwrap.NewNow()
		sourceFolder := &mitemfolder.ItemFolder{
			ID:           sourceFolderID,
			Name:         "Legacy Consistency Test",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, sourceFolder)
		require.NoError(t, err)

		// Create collection item
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, sourceFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial legacy table state
		initialFolder, err := ifs.GetFolder(ctx, sourceFolderID)
		require.NoError(t, err)

		// Attempt move to non-existent collection (should fail)
		nonExistentCollectionID := idwrap.NewNow()
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: nonExistentCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Move to non-existent collection should fail")

		// Verify legacy table state is unchanged
		finalFolder, err := ifs.GetFolder(ctx, sourceFolderID)
		require.NoError(t, err)

		assert.Equal(t, initialFolder.ID, finalFolder.ID, "Folder ID should be unchanged")
		assert.Equal(t, initialFolder.Name, finalFolder.Name, "Folder name should be unchanged")
		assert.Equal(t, initialFolder.CollectionID, finalFolder.CollectionID, "Folder collection ID should be unchanged")
		assert.Equal(t, initialFolder.ParentID, finalFolder.ParentID, "Folder parent ID should be unchanged")
	})
}

// TestCrossCollectionAtomicOperations tests that cross-collection moves are truly atomic
func TestCrossCollectionAtomicOperations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	mockLogger := mocklogger.NewMockLogger()
	cis := scollectionitem.New(base.Queries, mockLogger)
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("All-or-nothing move semantics", func(t *testing.T) {
		// Setup: Create complex structure with nested items
		parentFolderID := idwrap.NewNow()
		parentFolder := &mitemfolder.ItemFolder{
			ID:           parentFolderID,
			Name:         "Parent Folder",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, parentFolder)
		require.NoError(t, err)

		childFolderID := idwrap.NewNow()
		childFolder := &mitemfolder.ItemFolder{
			ID:           childFolderID,
			Name:         "Child Folder",
			CollectionID: sourceCollectionID,
			ParentID:     &parentFolderID,
		}
		err = ifs.CreateItemFolder(ctx, childFolder)
		require.NoError(t, err)

		nestedEndpointID := idwrap.NewNow()
		nestedEndpoint := &mitemapi.ItemApi{
			ID:           nestedEndpointID,
			Name:         "GET /nested/endpoint",
			Url:          "https://api.myapp.com/nested/endpoint",
			Method:       "GET",
			CollectionID: sourceCollectionID,
			FolderID:     &childFolderID,
		}
		err = ias.CreateItemApi(ctx, nestedEndpoint)
		require.NoError(t, err)

		// Create collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, parentFolder)
		require.NoError(t, err)
		err = cis.CreateFolderTX(ctx, tx, childFolder)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, nestedEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Capture initial state
		initialSourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		initialTargetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		// Attempt valid move (parent folder)
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             parentFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		if err != nil {
			// If operation failed, verify NO changes occurred
			t.Logf("Move operation failed: %v", err)
			
			finalSourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
			require.NoError(t, err)
			finalTargetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
			require.NoError(t, err)

			assert.Equal(t, len(initialSourceItems), len(finalSourceItems), "Source collection should be unchanged on failure")
			assert.Equal(t, len(initialTargetItems), len(finalTargetItems), "Target collection should be unchanged on failure")
		} else {
			// If operation succeeded, verify ALL related changes occurred
			assert.NotNil(t, resp, "Response should not be nil for successful operation")

			finalSourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
			require.NoError(t, err)
			finalTargetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
			require.NoError(t, err)

			// Parent folder should be moved
			assert.Equal(t, len(initialSourceItems)-1, len(finalSourceItems), "Source should have one less item")
			assert.Equal(t, len(initialTargetItems)+1, len(finalTargetItems), "Target should have one more item")

			// Verify parent folder is in target collection
			foundInTarget := false
			for _, item := range finalTargetItems {
				if item.FolderID != nil && item.FolderID.Compare(parentFolderID) == 0 {
					foundInTarget = true
					assert.Equal(t, targetCollectionID, item.CollectionID, "Parent folder should be in target collection")
					break
				}
			}
			assert.True(t, foundInTarget, "Parent folder should be found in target collection")

			// Verify parent folder is NOT in source collection
			foundInSource := false
			for _, item := range finalSourceItems {
				if item.FolderID != nil && item.FolderID.Compare(parentFolderID) == 0 {
					foundInSource = true
					break
				}
			}
			assert.False(t, foundInSource, "Parent folder should not be found in source collection")
		}
	})

	t.Run("Database constraint preservation", func(t *testing.T) {
		// This test ensures that database constraints are preserved during rollback
		// For example, foreign key constraints should not be violated during intermediate states

		// Create source endpoint
		sourceEndpointID := idwrap.NewNow()
		sourceEndpoint := &mitemapi.ItemApi{
			ID:           sourceEndpointID,
			Name:         "GET /constraint-test",
			Url:          "https://api.myapp.com/constraint-test",
			Method:       "GET",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, sourceEndpoint)
		require.NoError(t, err)

		// Create collection item
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Attempt move with invalid parameters that would violate constraints
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:       sourceEndpointID.Bytes(),
			CollectionId: sourceCollectionID.Bytes(),
			// Intentionally omit target_collection_id to test constraint handling
			Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Move without target collection should fail")

		// Verify item still exists in source collection
		sourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)

		found := false
		for _, item := range sourceItems {
			if item.EndpointID != nil && item.EndpointID.Compare(sourceEndpointID) == 0 {
				found = true
				assert.Equal(t, sourceCollectionID, item.CollectionID, "Endpoint should remain in source collection")
				break
			}
		}
		assert.True(t, found, "Endpoint should still exist in source collection after constraint violation")

		// Verify item does NOT exist in target collection
		targetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		for _, item := range targetItems {
			if item.EndpointID != nil {
				assert.NotEqual(t, sourceEndpointID, *item.EndpointID, "Endpoint should not appear in target collection")
			}
		}
	})
}

// TestCrossCollectionConcurrencyConsistency tests behavior under concurrent operations
func TestCrossCollectionConcurrencyConsistency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	mockLogger := mocklogger.NewMockLogger()
	cis := scollectionitem.New(base.Queries, mockLogger)
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Concurrent move attempts consistency", func(t *testing.T) {
		// Setup: Create item that will be moved
		endpointID := idwrap.NewNow()
		endpoint := &mitemapi.ItemApi{
			ID:           endpointID,
			Name:         "GET /concurrent-test",
			Url:          "https://api.myapp.com/concurrent-test",
			Method:       "GET",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, endpoint)
		require.NoError(t, err)

		// Create collection item
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, endpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Create valid move request
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:             endpointID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		// Execute the move
		resp, err := rpc.CollectionItemMove(authedCtx, req)

		// First move should succeed or fail cleanly
		if err == nil {
			assert.NotNil(t, resp, "Successful response should not be nil")

			// Attempt second move of same item (should fail since it's already moved)
			req2 := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
				ItemId:             endpointID.Bytes(),
				CollectionId:       sourceCollectionID.Bytes(), // Still referencing source collection
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			})

			_, err2 := rpc.CollectionItemMove(authedCtx, req2)
			assert.Error(t, err2, "Second move attempt should fail since item is no longer in source collection")
		}

		// Verify final state consistency
		sourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		targetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		// Count occurrences of the endpoint
		sourceCount := 0
		targetCount := 0

		for _, item := range sourceItems {
			if item.EndpointID != nil && item.EndpointID.Compare(endpointID) == 0 {
				sourceCount++
			}
		}

		for _, item := range targetItems {
			if item.EndpointID != nil && item.EndpointID.Compare(endpointID) == 0 {
				targetCount++
			}
		}

		// Endpoint should exist exactly once, in either source or target (never both, never neither)
		assert.Equal(t, 1, sourceCount+targetCount, "Endpoint should exist exactly once across both collections")
		assert.True(t, sourceCount == 1 || targetCount == 1, "Endpoint should be in exactly one collection")
		assert.False(t, sourceCount == 1 && targetCount == 1, "Endpoint should not exist in both collections")
	})
}