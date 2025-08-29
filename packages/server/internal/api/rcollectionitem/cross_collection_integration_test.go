package rcollectionitem_test

import (
	"context"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCrossCollectionTestEnvironment creates a test environment with two collections in the same workspace
func setupCrossCollectionTestEnvironment(t *testing.T, ctx context.Context) (
	rpc rcollectionitem.CollectionItemRPC,
	services *testutil.BaseTestServices,
	userID idwrap.IDWrap,
	sourceCollectionID, targetCollectionID idwrap.IDWrap,
	cleanup func(),
) {
	t.Helper()

	base := testutil.CreateBaseDB(ctx, t)
	baseServices := base.GetBaseServices()
	services = &baseServices

	mockLogger := mocklogger.NewMockLogger()

	// Create all required services
	cs := services.Cs
	cis := scollectionitem.New(base.Queries, mockLogger)
	us := services.Us
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)
	iaes := sitemapiexample.New(base.Queries)
	res := sexampleresp.New(base.Queries)

	rpc = rcollectionitem.New(base.DB, cs, cis, us, ifs, ias, iaes, res)

	userID = idwrap.NewNow()

	// Create workspace and collections in the same workspace
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	sourceCollectionID = idwrap.NewNow()
	targetCollectionID = idwrap.NewNow()

	// Create source collection
	services.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, sourceCollectionID)
	// Create target collection in the same workspace
	services.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, targetCollectionID)

	cleanup = func() {
		base.Close()
	}

	return rpc, services, userID, sourceCollectionID, targetCollectionID, cleanup
}

// createTestItemsInCollections creates test folders and endpoints in both collections
func createTestItemsInCollections(t *testing.T, ctx context.Context, 
	sourceCollectionID, targetCollectionID idwrap.IDWrap,
	cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService,
	base *testutil.BaseDBQueries,
) (
	sourceFolderID, sourceEndpointID, targetFolderID, targetEndpointID idwrap.IDWrap,
) {
	t.Helper()

	sourceFolderID = idwrap.NewNow()
	sourceEndpointID = idwrap.NewNow()
	targetFolderID = idwrap.NewNow()
	targetEndpointID = idwrap.NewNow()

	// Create items in source collection
	sourceFolder := &mitemfolder.ItemFolder{
		ID:           sourceFolderID,
		Name:         "Source Folder",
		CollectionID: sourceCollectionID,
		ParentID:     nil,
	}
	err := ifs.CreateItemFolder(ctx, sourceFolder)
	require.NoError(t, err)

	sourceEndpoint := &mitemapi.ItemApi{
		ID:           sourceEndpointID,
		Name:         "Source Endpoint",
		Url:          "https://api.source.com/test",
		Method:       "GET",
		CollectionID: sourceCollectionID,
		FolderID:     nil,
	}
	err = ias.CreateItemApi(ctx, sourceEndpoint)
	require.NoError(t, err)

	// Create items in target collection
	targetFolder := &mitemfolder.ItemFolder{
		ID:           targetFolderID,
		Name:         "Target Folder",
		CollectionID: targetCollectionID,
		ParentID:     nil,
	}
	err = ifs.CreateItemFolder(ctx, targetFolder)
	require.NoError(t, err)

	targetEndpoint := &mitemapi.ItemApi{
		ID:           targetEndpointID,
		Name:         "Target Endpoint",
		Url:          "https://api.target.com/test",
		Method:       "POST",
		CollectionID: targetCollectionID,
		FolderID:     nil,
	}
	err = ias.CreateItemApi(ctx, targetEndpoint)
	require.NoError(t, err)

	// Create collection items entries for all items
	tx, err := base.DB.Begin()
	require.NoError(t, err)
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	err = cis.CreateFolderTX(ctx, tx, sourceFolder)
	require.NoError(t, err)
	err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
	require.NoError(t, err)
	err = cis.CreateFolderTX(ctx, tx, targetFolder)
	require.NoError(t, err)
	err = cis.CreateEndpointTX(ctx, tx, targetEndpoint)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)
	tx = nil

	return sourceFolderID, sourceEndpointID, targetFolderID, targetEndpointID
}

// TestCrossCollectionEndpointMove tests moving an endpoint from one collection to another
func TestCrossCollectionEndpointMove(t *testing.T) {
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

	_, sourceEndpointID, targetFolderID, _ := createTestItemsInCollections(
		t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, base)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Move endpoint to different collection", func(t *testing.T) {
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:             sourceEndpointID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		assert.NoError(t, err, "Cross-collection endpoint move should succeed")
		assert.NotNil(t, resp, "Response should not be nil")
		assert.NotNil(t, resp.Msg, "Response message should not be nil")

		// Verify endpoint is now in target collection
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		
		found := false
		for _, item := range items {
			if item.EndpointID != nil && item.EndpointID.Compare(sourceEndpointID) == 0 {
				found = true
				assert.Equal(t, targetCollectionID, item.CollectionID, "Endpoint should be in target collection")
				break
			}
		}
		assert.True(t, found, "Moved endpoint should be found in target collection")

		// Verify endpoint is no longer in source collection
		sourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		
		for _, item := range sourceItems {
			if item.EndpointID != nil {
				assert.NotEqual(t, sourceEndpointID, *item.EndpointID, "Endpoint should not be in source collection")
			}
		}
	})

	t.Run("Move endpoint to specific folder in different collection", func(t *testing.T) {
		// Create another endpoint to move
		anotherEndpointID := idwrap.NewNow()
		anotherEndpoint := &mitemapi.ItemApi{
			ID:           anotherEndpointID,
			Name:         "Another Endpoint",
			Url:          "https://api.another.com/test",
			Method:       "PUT",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, anotherEndpoint)
		require.NoError(t, err)

		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, anotherEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:               anotherEndpointID.Bytes(),
			CollectionId:         sourceCollectionID.Bytes(),
			TargetCollectionId:   targetCollectionID.Bytes(),
			TargetParentFolderId: targetFolderID.Bytes(),
			Position:             resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		assert.NoError(t, err, "Cross-collection endpoint move to folder should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Verify endpoint is in target collection under the folder
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, &targetFolderID)
		require.NoError(t, err)
		
		found := false
		for _, item := range items {
			if item.EndpointID != nil && item.EndpointID.Compare(anotherEndpointID) == 0 {
				found = true
				assert.Equal(t, targetCollectionID, item.CollectionID, "Endpoint should be in target collection")
				assert.Equal(t, targetFolderID, *item.ParentFolderID, "Endpoint should be in target folder")
				break
			}
		}
		assert.True(t, found, "Moved endpoint should be found in target folder")
	})
}

// TestCrossCollectionFolderMove tests moving a folder from one collection to another
func TestCrossCollectionFolderMove(t *testing.T) {
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

	sourceFolderID, _, targetFolderID, _ := createTestItemsInCollections(
		t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, base)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Move folder to different collection", func(t *testing.T) {
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		assert.NoError(t, err, "Cross-collection folder move should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Verify folder is now in target collection
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		
		found := false
		for _, item := range items {
			if item.FolderID != nil && item.FolderID.Compare(sourceFolderID) == 0 {
				found = true
				assert.Equal(t, targetCollectionID, item.CollectionID, "Folder should be in target collection")
				break
			}
		}
		assert.True(t, found, "Moved folder should be found in target collection")

		// Verify folder is no longer in source collection
		sourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		
		for _, item := range sourceItems {
			if item.FolderID != nil {
				assert.NotEqual(t, sourceFolderID, *item.FolderID, "Folder should not be in source collection")
			}
		}
	})

	t.Run("Move folder to nested location in different collection", func(t *testing.T) {
		// Create another folder to move
		anotherFolderID := idwrap.NewNow()
		anotherFolder := &mitemfolder.ItemFolder{
			ID:           anotherFolderID,
			Name:         "Another Folder",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, anotherFolder)
		require.NoError(t, err)

		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, anotherFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:               anotherFolderID.Bytes(),
			CollectionId:         sourceCollectionID.Bytes(),
			TargetCollectionId:   targetCollectionID.Bytes(),
			TargetParentFolderId: targetFolderID.Bytes(),
			Position:             resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		assert.NoError(t, err, "Cross-collection folder move to nested location should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Verify folder is in target collection under the parent folder
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, &targetFolderID)
		require.NoError(t, err)
		
		found := false
		for _, item := range items {
			if item.FolderID != nil && item.FolderID.Compare(anotherFolderID) == 0 {
				found = true
				assert.Equal(t, targetCollectionID, item.CollectionID, "Folder should be in target collection")
				assert.Equal(t, targetFolderID, *item.ParentFolderID, "Folder should be nested in target parent folder")
				break
			}
		}
		assert.True(t, found, "Moved folder should be found nested in target collection")
	})
}

// TestCrossCollectionWithTargetPosition tests positioning items relative to existing items in target collection
func TestCrossCollectionWithTargetPosition(t *testing.T) {
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

	sourceFolderID, _, _, targetEndpointID := createTestItemsInCollections(
		t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, base)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Move folder before existing endpoint in target collection", func(t *testing.T) {
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetEndpointID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_BEFORE.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		assert.NoError(t, err, "Cross-collection move with positioning should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Verify ordering in target collection
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		require.Len(t, items, 2, "Target collection should have 2 items")

		// Find positions
		var folderPos, endpointPos int = -1, -1
		for i, item := range items {
			if item.FolderID != nil && item.FolderID.Compare(sourceFolderID) == 0 {
				folderPos = i
			}
			if item.EndpointID != nil && item.EndpointID.Compare(targetEndpointID) == 0 {
				endpointPos = i
			}
		}

		require.NotEqual(t, -1, folderPos, "Moved folder should be found in target collection")
		require.NotEqual(t, -1, endpointPos, "Target endpoint should be found in target collection")
		assert.Less(t, folderPos, endpointPos, "Moved folder should be positioned before target endpoint")
	})

	t.Run("Move folder after existing endpoint in target collection", func(t *testing.T) {
		// Create another folder to move
		anotherFolderID := idwrap.NewNow()
		anotherFolder := &mitemfolder.ItemFolder{
			ID:           anotherFolderID,
			Name:         "Position Test Folder",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, anotherFolder)
		require.NoError(t, err)

		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, anotherFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             anotherFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetEndpointID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		assert.NoError(t, err, "Cross-collection move after target should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Verify ordering in target collection
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		require.Len(t, items, 3, "Target collection should have 3 items now")

		// Find positions
		var newFolderPos, endpointPos int = -1, -1
		for i, item := range items {
			if item.FolderID != nil && item.FolderID.Compare(anotherFolderID) == 0 {
				newFolderPos = i
			}
			if item.EndpointID != nil && item.EndpointID.Compare(targetEndpointID) == 0 {
				endpointPos = i
			}
		}

		require.NotEqual(t, -1, newFolderPos, "Moved folder should be found in target collection")
		require.NotEqual(t, -1, endpointPos, "Target endpoint should be found in target collection")
		assert.Greater(t, newFolderPos, endpointPos, "Moved folder should be positioned after target endpoint")
	})
}

// TestWorkspaceBoundaryEnforcement tests that items cannot be moved between different workspaces
func TestWorkspaceBoundaryEnforcement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	services := base.GetBaseServices()

	mockLogger := mocklogger.NewMockLogger()
	cs := services.Cs
	cis := scollectionitem.New(base.Queries, mockLogger)
	us := services.Us
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)
	iaes := sitemapiexample.New(base.Queries)
	res := sexampleresp.New(base.Queries)

	rpc := rcollectionitem.New(base.DB, cs, cis, us, ifs, ias, iaes, res)

	user1ID := idwrap.NewNow()
	user2ID := idwrap.NewNow()

	// Create collections in different workspaces
	workspace1ID := idwrap.NewNow()
	workspace1UserID := idwrap.NewNow()
	collection1ID := idwrap.NewNow()

	workspace2ID := idwrap.NewNow()
	workspace2UserID := idwrap.NewNow()
	collection2ID := idwrap.NewNow()

	// Setup first workspace/collection
	services.CreateTempCollection(t, ctx, workspace1ID, workspace1UserID, user1ID, collection1ID)
	// Setup second workspace/collection  
	services.CreateTempCollection(t, ctx, workspace2ID, workspace2UserID, user2ID, collection2ID)

	// Create item in first collection
	folderID := idwrap.NewNow()
	folder := &mitemfolder.ItemFolder{
		ID:           folderID,
		Name:         "Cross Workspace Test Folder",
		CollectionID: collection1ID,
		ParentID:     nil,
	}
	err := ifs.CreateItemFolder(ctx, folder)
	require.NoError(t, err)

	// Create collection item
	tx, err := base.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	err = cis.CreateFolderTX(ctx, tx, folder)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	authedCtx1 := mwauth.CreateAuthedContext(ctx, user1ID)

	t.Run("User cannot move item to collection in different workspace", func(t *testing.T) {
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             folderID.Bytes(),
			CollectionId:       collection1ID.Bytes(),
			TargetCollectionId: collection2ID.Bytes(), // Different workspace
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err := rpc.CollectionItemMove(authedCtx1, req)

		assert.Error(t, err, "Cross-workspace move should fail")
		if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
			assert.Contains(t, 
				[]connect.Code{connect.CodePermissionDenied, connect.CodeNotFound}, 
				connectErr.Code(),
				"Should reject cross-workspace move with PermissionDenied or NotFound")
			assert.Contains(t, connectErr.Message(), "workspace", 
				"Error message should mention workspace restriction")
		}

		// Verify item remains in original collection
		items, err := cis.ListCollectionItems(ctx, collection1ID, nil)
		require.NoError(t, err)
		
		found := false
		for _, item := range items {
			if item.FolderID != nil && item.FolderID.Compare(folderID) == 0 {
				found = true
				assert.Equal(t, collection1ID, item.CollectionID, "Item should remain in original collection")
				break
			}
		}
		assert.True(t, found, "Item should remain in original collection")
	})
}

// TestPermissionValidation tests that users can only move items in collections they have access to
func TestPermissionValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, _, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	mockLogger := mocklogger.NewMockLogger()
	cis := scollectionitem.New(base.Queries, mockLogger)
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)

	sourceFolderID, _, _, _ := createTestItemsInCollections(
		t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, base)

	unauthorizedUserID := idwrap.NewNow()
	unauthorizedCtx := mwauth.CreateAuthedContext(ctx, unauthorizedUserID)

	t.Run("Unauthorized user cannot move items", func(t *testing.T) {
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err := rpc.CollectionItemMove(unauthorizedCtx, req)

		assert.Error(t, err, "Unauthorized user move should fail")
		if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
			assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(),
				"Should reject unauthorized move with PermissionDenied")
		}
	})

	t.Run("User with source access but no target access cannot move", func(t *testing.T) {
		// Create another user with access to source but not target
		partialUserID := idwrap.NewNow()
		partialWorkspaceID := idwrap.NewNow()
		partialWorkspaceUserID := idwrap.NewNow()
		partialCollectionID := idwrap.NewNow()

		// Give user access to source collection by creating them as owner
		// This is a simplified test - in real scenarios this would be more complex
		baseServices := base.GetBaseServices()
		baseServices.CreateTempCollection(t, ctx, partialWorkspaceID, partialWorkspaceUserID, partialUserID, partialCollectionID)

		// Create item in their collection
		partialFolderID := idwrap.NewNow()
		partialFolder := &mitemfolder.ItemFolder{
			ID:           partialFolderID,
			Name:         "Partial Access Folder",
			CollectionID: partialCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, partialFolder)
		require.NoError(t, err)

		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, partialFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		partialCtx := mwauth.CreateAuthedContext(ctx, partialUserID)

		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             partialFolderID.Bytes(),
			CollectionId:       partialCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(), // User doesn't have access to this
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(partialCtx, req)

		assert.Error(t, err, "Move without target collection access should fail")
		if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
			assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(),
				"Should reject move without target access with PermissionDenied")
		}
	})
}