package rcollectionitem_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
	folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
)

// TestCollectionItemsEndToEndWorkflow tests the complete workflow from creation to movement
func TestCollectionItemsEndToEndWorkflow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	queries := base.Queries
	db := base.DB
	mockLogger := mocklogger.NewMockLogger()

	// Setup test infrastructure
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow() 
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Initialize services
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, mockLogger)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	// Create RPC services
	collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)
	folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
	apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Test 1: Create initial folder structure
	t.Run("CreateFolderStructure", func(t *testing.T) {
		// Create root folder "API v1"
		folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "API v1",
			ParentFolderId: nil,
		})

		folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
		require.NoError(t, err)
		require.NotNil(t, folderResp)

		rootFolderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

		// Create subfolder "Users"
		usersFolderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Users",
			ParentFolderId: rootFolderID.Bytes(),
		})

		usersFolderResp, err := folderRPC.FolderCreate(authedCtx, usersFolderReq)
		require.NoError(t, err)
		require.NotNil(t, usersFolderResp)

		usersFolderID := idwrap.NewFromBytesMust(usersFolderResp.Msg.FolderId)

		// Verify folder structure in database
		rootFolder, err := ifs.GetFolder(ctx, rootFolderID)
		require.NoError(t, err)
		assert.Equal(t, "API v1", rootFolder.Name)
		assert.Nil(t, rootFolder.ParentID)

		usersFolder, err := ifs.GetFolder(ctx, usersFolderID)
		require.NoError(t, err)
		assert.Equal(t, "Users", usersFolder.Name)
		require.NotNil(t, usersFolder.ParentID)
		assert.Equal(t, rootFolderID.Compare(*usersFolder.ParentID), 0)

		// Verify collection_items entries exist by getting all items and filtering
		allItems, err := queries.GetCollectionItemsByCollectionID(ctx, collectionID)
		require.NoError(t, err)
		require.Len(t, allItems, 2)

		// Find the collection items by matching folder_id
		var rootCollectionItem, usersCollectionItem *gen.CollectionItem
		for _, item := range allItems {
			if item.FolderID != nil {
				if item.FolderID.Compare(rootFolderID) == 0 {
					rootCollectionItem = &item
				} else if item.FolderID.Compare(usersFolderID) == 0 {
					usersCollectionItem = &item
				}
			}
		}

		require.NotNil(t, rootCollectionItem)
		assert.Equal(t, int8(0), rootCollectionItem.ItemType) // Folder type
		assert.Equal(t, "API v1", rootCollectionItem.Name)

		require.NotNil(t, usersCollectionItem)
		assert.Equal(t, int8(0), usersCollectionItem.ItemType)
		assert.Equal(t, "Users", usersCollectionItem.Name)
	})

	// Test 2: Create endpoints in different contexts
	t.Run("CreateEndpointsInDifferentContexts", func(t *testing.T) {
		// Get folder IDs from database
		folders, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
		require.NoError(t, err)
		require.Len(t, folders, 2)

		var rootFolderID, usersFolderID idwrap.IDWrap
		for _, folder := range folders {
			if folder.Name == "API v1" && folder.ParentID == nil {
				rootFolderID = folder.ID
			} else if folder.Name == "Users" && folder.ParentID != nil {
				usersFolderID = folder.ID
			}
		}

		// Create endpoint in root collection (no parent folder)
		rootEndpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Health Check",
			Method:         "GET",
			Url:            "/health",
			ParentFolderId: nil,
		})

		rootEndpointResp, err := apiRPC.EndpointCreate(authedCtx, rootEndpointReq)
		require.NoError(t, err)
		require.NotNil(t, rootEndpointResp)

		rootEndpointID := idwrap.NewFromBytesMust(rootEndpointResp.Msg.EndpointId)

		// Create endpoint in root folder "API v1"
		folderEndpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Get Version",
			Method:         "GET", 
			Url:            "/v1/version",
			ParentFolderId: rootFolderID.Bytes(),
		})

		folderEndpointResp, err := apiRPC.EndpointCreate(authedCtx, folderEndpointReq)
		require.NoError(t, err)
		require.NotNil(t, folderEndpointResp)

		folderEndpointID := idwrap.NewFromBytesMust(folderEndpointResp.Msg.EndpointId)

		// Create endpoints in "Users" subfolder
		usersEndpoints := []struct {
			name, method, url string
		}{
			{"Get Users", "GET", "/v1/users"},
			{"Create User", "POST", "/v1/users"},
			{"Update User", "PUT", "/v1/users/{id}"},
			{"Delete User", "DELETE", "/v1/users/{id}"},
		}

		var userEndpointIDs []idwrap.IDWrap
		for _, endpoint := range usersEndpoints {
			endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
				CollectionId:   collectionID.Bytes(),
				Name:           endpoint.name,
				Method:         endpoint.method,
				Url:            endpoint.url,
				ParentFolderId: usersFolderID.Bytes(),
			})

			endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
			require.NoError(t, err, "Failed to create endpoint: %s", endpoint.name)
			require.NotNil(t, endpointResp)

			endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)
			userEndpointIDs = append(userEndpointIDs, endpointID)
		}

		// Verify endpoints in database
		rootEndpoint, err := ias.GetItemApi(ctx, rootEndpointID)
		require.NoError(t, err)
		assert.Equal(t, "Health Check", rootEndpoint.Name)
		assert.Equal(t, "GET", rootEndpoint.Method)
		assert.Nil(t, rootEndpoint.FolderID)

		folderEndpoint, err := ias.GetItemApi(ctx, folderEndpointID)
		require.NoError(t, err)
		assert.Equal(t, "Get Version", folderEndpoint.Name)
		assert.Equal(t, "GET", folderEndpoint.Method)
		require.NotNil(t, folderEndpoint.FolderID)
		assert.Equal(t, rootFolderID.Compare(*folderEndpoint.FolderID), 0)

		// Verify user endpoints
		for i, endpointID := range userEndpointIDs {
			endpoint, err := ias.GetItemApi(ctx, endpointID)
			require.NoError(t, err)
			assert.Equal(t, usersEndpoints[i].name, endpoint.Name)
			assert.Equal(t, usersEndpoints[i].method, endpoint.Method)
			require.NotNil(t, endpoint.FolderID)
			assert.Equal(t, usersFolderID.Compare(*endpoint.FolderID), 0)
		}

		// Verify collection_items entries for endpoints by getting all items and filtering
		allItems, err := queries.GetCollectionItemsByCollectionID(ctx, collectionID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(allItems), 6) // 2 folders + 4+ endpoints

		// Find the collection items by matching endpoint_id
		var rootEndpointCollectionItem, folderEndpointCollectionItem *gen.CollectionItem
		for _, item := range allItems {
			if item.EndpointID != nil {
				if item.EndpointID.Compare(rootEndpointID) == 0 {
					rootEndpointCollectionItem = &item
				} else if item.EndpointID.Compare(folderEndpointID) == 0 {
					folderEndpointCollectionItem = &item
				}
			}
		}

		require.NotNil(t, rootEndpointCollectionItem)
		assert.Equal(t, int8(1), rootEndpointCollectionItem.ItemType) // Endpoint type
		assert.Equal(t, "Health Check", rootEndpointCollectionItem.Name)

		require.NotNil(t, folderEndpointCollectionItem)
		assert.Equal(t, int8(1), folderEndpointCollectionItem.ItemType)
		assert.Equal(t, "Get Version", folderEndpointCollectionItem.Name)
	})

	// Test 3: List items in mixed order and verify ordering
	t.Run("ListItemsWithMixedOrder", func(t *testing.T) {
		// List items in root collection
		listRootReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:    collectionID.Bytes(),
			ParentFolderId: nil,
		})

		listRootResp, err := collectionItemRPC.CollectionItemList(authedCtx, listRootReq)
		require.NoError(t, err)
		require.NotNil(t, listRootResp)

		rootItems := listRootResp.Msg.Items
		require.Len(t, rootItems, 2) // "API v1" folder + "Health Check" endpoint

		// Verify mixed order (folder first, then endpoint based on creation order)
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, rootItems[0].Kind)
		assert.Equal(t, "API v1", rootItems[0].Folder.Name)
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, rootItems[1].Kind)
		assert.Equal(t, "Health Check", rootItems[1].Endpoint.Name)

		// Get folder IDs for listing subfolders
		apiV1FolderID := idwrap.NewFromBytesMust(rootItems[0].Folder.FolderId)

		// List items in "API v1" folder
		listFolderReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:    collectionID.Bytes(),
			ParentFolderId: apiV1FolderID.Bytes(),
		})

		listFolderResp, err := collectionItemRPC.CollectionItemList(authedCtx, listFolderReq)
		require.NoError(t, err)
		require.NotNil(t, listFolderResp)

		folderItems := listFolderResp.Msg.Items
		require.Len(t, folderItems, 2) // "Users" subfolder + "Get Version" endpoint

		// Verify ordering within folder
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, folderItems[0].Kind)
		assert.Equal(t, "Users", folderItems[0].Folder.Name)
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, folderItems[1].Kind)
		assert.Equal(t, "Get Version", folderItems[1].Endpoint.Name)

		// List items in "Users" subfolder
		usersFolderID := idwrap.NewFromBytesMust(folderItems[0].Folder.FolderId)

		listUsersReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:    collectionID.Bytes(),
			ParentFolderId: usersFolderID.Bytes(),
		})

		listUsersResp, err := collectionItemRPC.CollectionItemList(authedCtx, listUsersReq)
		require.NoError(t, err)
		require.NotNil(t, listUsersResp)

		usersItems := listUsersResp.Msg.Items
		require.Len(t, usersItems, 4) // 4 user endpoints

		// Verify all are endpoints in correct order
		expectedNames := []string{"Get Users", "Create User", "Update User", "Delete User"}
		for i, item := range usersItems {
			assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, item.Kind)
			assert.Equal(t, expectedNames[i], item.Endpoint.Name)
		}
	})

	// Test 4: Complex move operations
	t.Run("ComplexMoveOperations", func(t *testing.T) {
		// Get current state for move operations
		listRootReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:    collectionID.Bytes(),
			ParentFolderId: nil,
		})

		listRootResp, err := collectionItemRPC.CollectionItemList(authedCtx, listRootReq)
		require.NoError(t, err)
		rootItems := listRootResp.Msg.Items

		apiV1FolderID := idwrap.NewFromBytesMust(rootItems[0].Folder.FolderId)
		healthCheckEndpointID := idwrap.NewFromBytesMust(rootItems[1].Endpoint.EndpointId)

		// Move "Health Check" before "API v1" folder (endpoint before folder)
		moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			CollectionId:  collectionID.Bytes(),
			ItemId:       healthCheckEndpointID.Bytes(),
			TargetItemId: apiV1FolderID.Bytes(),
			Position:     resourcesv1.MovePosition_MOVE_POSITION_BEFORE.Enum(),
		})

		moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
		require.NoError(t, err)
		require.NotNil(t, moveResp)

		// Verify new order
		listAfterMoveResp, err := collectionItemRPC.CollectionItemList(authedCtx, listRootReq)
		require.NoError(t, err)
		rootItemsAfterMove := listAfterMoveResp.Msg.Items

		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, rootItemsAfterMove[0].Kind)
		assert.Equal(t, "Health Check", rootItemsAfterMove[0].Endpoint.Name)
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, rootItemsAfterMove[1].Kind)
		assert.Equal(t, "API v1", rootItemsAfterMove[1].Folder.Name)

		// Test cross-folder move: Move "Get Version" from "API v1" to root
		apiV1Items, err := collectionItemRPC.CollectionItemList(authedCtx, &connect.Request[itemv1.CollectionItemListRequest]{
			Msg: &itemv1.CollectionItemListRequest{
				CollectionId:    collectionID.Bytes(),
				ParentFolderId: apiV1FolderID.Bytes(),
			},
		})
		require.NoError(t, err)

		getVersionEndpointID := idwrap.NewFromBytesMust(apiV1Items.Msg.Items[1].Endpoint.EndpointId)

		// Move "Get Version" after "Health Check" in root
		crossFolderMoveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			CollectionId:  collectionID.Bytes(),
			ItemId:       getVersionEndpointID.Bytes(),
			TargetItemId: healthCheckEndpointID.Bytes(),
			Position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		crossFolderMoveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, crossFolderMoveReq)
		require.NoError(t, err)
		require.NotNil(t, crossFolderMoveResp)

		// Verify cross-folder move result
		listAfterCrossMoveResp, err := collectionItemRPC.CollectionItemList(authedCtx, listRootReq)
		require.NoError(t, err)
		rootItemsAfterCrossMove := listAfterCrossMoveResp.Msg.Items

		require.Len(t, rootItemsAfterCrossMove, 3) // Health Check + Get Version + API v1
		assert.Equal(t, "Health Check", rootItemsAfterCrossMove[0].Endpoint.Name)
		assert.Equal(t, "Get Version", rootItemsAfterCrossMove[1].Endpoint.Name)
		assert.Equal(t, "API v1", rootItemsAfterCrossMove[2].Folder.Name)

		// Verify endpoint is now in root (no parent folder)
		movedEndpoint, err := ias.GetItemApi(ctx, getVersionEndpointID)
		require.NoError(t, err)
		assert.Nil(t, movedEndpoint.FolderID, "Moved endpoint should have no parent folder")
	})
}

// TestCollectionItemsErrorScenarios tests edge cases and error conditions
func TestCollectionItemsErrorScenarios(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	queries := base.Queries
	db := base.DB
	mockLogger := mocklogger.NewMockLogger()

	// Setup test infrastructure
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Initialize services
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, mockLogger)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("EmptyCollectionOperations", func(t *testing.T) {
		// List empty collection
		listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:    collectionID.Bytes(),
			ParentFolderId: nil,
		})

		listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
		require.NoError(t, err)
		require.NotNil(t, listResp)
		assert.Empty(t, listResp.Msg.Items)

		// Try to move non-existent item
		nonExistentID := idwrap.NewNow()
		moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			CollectionId: collectionID.Bytes(),
			ItemId:       nonExistentID.Bytes(),
			Position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
		assert.Error(t, err)
		assert.Nil(t, moveResp)
		
		connectErr := err.(*connect.Error)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("InvalidMoveOperations", func(t *testing.T) {
		// Create a single item for testing
		folderRPC := ritemfolder.New(db, ifs, us, cs, cis)

		folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Test Folder",
		})

		folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
		require.NoError(t, err)

		folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

		// Try to move item relative to itself
		moveToSelfReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			CollectionId:  collectionID.Bytes(),
			ItemId:       folderID.Bytes(),
			TargetItemId: folderID.Bytes(), // Same as item_id
			Position:     resourcesv1.MovePosition_MOVE_POSITION_BEFORE.Enum(),
		})

		moveToSelfResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveToSelfReq)
		assert.Error(t, err)
		assert.Nil(t, moveToSelfResp)
		
		connectErr := err.(*connect.Error)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
		assert.Contains(t, connectErr.Message(), "cannot move item relative to itself")

		// Try invalid position with target
		invalidPosReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			CollectionId:  collectionID.Bytes(),
			ItemId:       folderID.Bytes(),
			TargetItemId: folderID.Bytes(),
			Position:     resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
		})

		invalidPosResp, err := collectionItemRPC.CollectionItemMove(authedCtx, invalidPosReq)
		assert.Error(t, err)
		assert.Nil(t, invalidPosResp)
		
		connectErr = err.(*connect.Error)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("CrossCollectionMoveAttempt", func(t *testing.T) {
		// Create second collection
		collection2ID := idwrap.NewNow()
		baseServices.CreateTempCollection(t, ctx, workspaceID, idwrap.NewNow(), userID, collection2ID)

		folderRPC := ritemfolder.New(db, ifs, us, cs, cis)

		// Create folder in first collection
		folder1Req := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Folder 1",
		})

		folder1Resp, err := folderRPC.FolderCreate(authedCtx, folder1Req)
		require.NoError(t, err)

		// Create folder in second collection
		folder2Req := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId: collection2ID.Bytes(),
			Name:         "Folder 2",
		})

		folder2Resp, err := folderRPC.FolderCreate(authedCtx, folder2Req)
		require.NoError(t, err)

		folder1ID := idwrap.NewFromBytesMust(folder1Resp.Msg.FolderId)
		folder2ID := idwrap.NewFromBytesMust(folder2Resp.Msg.FolderId)

		// Try to move folder from collection1 relative to folder in collection2
		crossCollectionMoveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			CollectionId:  collectionID.Bytes(),
			ItemId:       folder1ID.Bytes(),
			TargetItemId: folder2ID.Bytes(),
			Position:     resourcesv1.MovePosition_MOVE_POSITION_BEFORE.Enum(),
		})

		crossCollectionMoveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, crossCollectionMoveReq)
		assert.Error(t, err)
		assert.Nil(t, crossCollectionMoveResp)
		
		connectErr := err.(*connect.Error)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

// TestCollectionItemsPerformance tests performance with large collections
func TestCollectionItemsPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	queries := base.Queries
	db := base.DB
	mockLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Setup test infrastructure
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Initialize services
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, mockLogger)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)
	folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
	apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("CreateLargeCollection", func(t *testing.T) {
		const numFolders = 10
		const numEndpointsPerFolder = 10
		const numRootEndpoints = 20

		start := time.Now()

		// Create folders
		var folderIDs []idwrap.IDWrap
		for i := 0; i < numFolders; i++ {
			folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
				CollectionId: collectionID.Bytes(),
				Name:         fmt.Sprintf("Folder %d", i+1),
			})

			folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
			require.NoError(t, err)

			folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)
			folderIDs = append(folderIDs, folderID)
		}

		// Create endpoints in each folder
		for i, folderID := range folderIDs {
			for j := 0; j < numEndpointsPerFolder; j++ {
				endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
					CollectionId:   collectionID.Bytes(),
					Name:           fmt.Sprintf("Folder %d - Endpoint %d", i+1, j+1),
					Method:         "GET",
					Url:            fmt.Sprintf("/folder%d/endpoint%d", i+1, j+1),
					ParentFolderId: folderID.Bytes(),
				})

				_, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
				require.NoError(t, err)
			}
		}

		// Create root-level endpoints
		for i := 0; i < numRootEndpoints; i++ {
			endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
				CollectionId: collectionID.Bytes(),
				Name:         fmt.Sprintf("Root Endpoint %d", i+1),
				Method:       "GET",
				Url:          fmt.Sprintf("/root/endpoint%d", i+1),
			})

			_, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
			require.NoError(t, err)
		}

		creationTime := time.Since(start)
		t.Logf("Created large collection with %d folders and %d endpoints in %v",
			numFolders, numFolders*numEndpointsPerFolder+numRootEndpoints, creationTime)

		// Performance requirement: Total creation should be under 5 seconds
		assert.Less(t, creationTime, 5*time.Second, "Collection creation took too long")
	})

	t.Run("ListLargeCollectionPerformance", func(t *testing.T) {
		start := time.Now()

		listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:    collectionID.Bytes(),
			ParentFolderId: nil,
		})

		listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
		require.NoError(t, err)

		listTime := time.Since(start)
		t.Logf("Listed %d root items in %v", len(listResp.Msg.Items), listTime)

		// Performance requirement: Listing should be under 100ms
		assert.Less(t, listTime, 100*time.Millisecond, "Collection listing took too long")

		// Verify we have the expected items (10 folders + 20 root endpoints)
		assert.Len(t, listResp.Msg.Items, 30)
	})

	t.Run("MoveLargeCollectionPerformance", func(t *testing.T) {
		// Get items for move operation
		listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:    collectionID.Bytes(),
			ParentFolderId: nil,
		})

		listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
		require.NoError(t, err)
		require.Greater(t, len(listResp.Msg.Items), 2)

		// Move the last item to the beginning
		items := listResp.Msg.Items
		lastItem := items[len(items)-1]
		firstItem := items[0]

		var lastItemID, firstItemID []byte
		if lastItem.Kind == itemv1.ItemKind_ITEM_KIND_FOLDER {
			lastItemID = lastItem.Folder.FolderId
		} else {
			lastItemID = lastItem.Endpoint.EndpointId
		}

		if firstItem.Kind == itemv1.ItemKind_ITEM_KIND_FOLDER {
			firstItemID = firstItem.Folder.FolderId
		} else {
			firstItemID = firstItem.Endpoint.EndpointId
		}

		start := time.Now()

		moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			CollectionId:  collectionID.Bytes(),
			ItemId:       lastItemID,
			TargetItemId: firstItemID,
			Position:     resourcesv1.MovePosition_MOVE_POSITION_BEFORE.Enum(),
		})

		moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
		require.NoError(t, err)
		require.NotNil(t, moveResp)

		moveTime := time.Since(start)
		t.Logf("Moved item in large collection in %v", moveTime)

		// Performance requirement: Move should be under 100ms
		assert.Less(t, moveTime, 100*time.Millisecond, "Collection item move took too long")

		// Verify the move worked
		listAfterMoveResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
		require.NoError(t, err)
		
		afterMoveItems := listAfterMoveResp.Msg.Items
		require.Len(t, afterMoveItems, len(items))

		// The moved item should now be first
		movedItem := afterMoveItems[0]
		if lastItem.Kind == itemv1.ItemKind_ITEM_KIND_FOLDER {
			assert.Equal(t, lastItem.Folder.Name, movedItem.Folder.Name)
		} else {
			assert.Equal(t, lastItem.Endpoint.Name, movedItem.Endpoint.Name)
		}
	})
}

// TestCollectionItemsDataConsistency tests data consistency between collection_items and legacy tables
func TestCollectionItemsDataConsistency(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	queries := base.Queries
	db := base.DB
	mockLogger := mocklogger.NewMockLogger()

	// Setup test infrastructure
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Initialize services
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, mockLogger)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
	apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("FolderDataConsistency", func(t *testing.T) {
		// Create folder via RPC
		folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Consistency Test Folder",
		})

		folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
		require.NoError(t, err)

		folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

		// Verify folder in legacy item_folder table
		folder, err := ifs.GetFolder(ctx, folderID)
		require.NoError(t, err)
		assert.Equal(t, "Consistency Test Folder", folder.Name)
		assert.Equal(t, collectionID.Compare(folder.CollectionID), 0)

		// Verify folder in collection_items table
		allItems, err := queries.GetCollectionItemsByCollectionID(ctx, collectionID)
		require.NoError(t, err)

		// Find the collection item by matching folder_id
		var collectionItem *gen.CollectionItem
		for _, item := range allItems {
			if item.FolderID != nil {
				if item.FolderID.Compare(folderID) == 0 {
					collectionItem = &item
					break
				}
			}
		}

		require.NotNil(t, collectionItem)
		assert.Equal(t, int8(0), collectionItem.ItemType) // Folder type
		assert.Equal(t, "Consistency Test Folder", collectionItem.Name)
		assert.Equal(t, collectionID.Compare(collectionItem.CollectionID), 0)

		// Verify FK relationship
		assert.NotNil(t, collectionItem.FolderID)
		assert.Equal(t, folderID.Compare(*collectionItem.FolderID), 0)
	})

	t.Run("EndpointDataConsistency", func(t *testing.T) {
		// Create endpoint via RPC
		endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Consistency Test Endpoint",
			Method:       "POST",
			Url:          "/test/consistency",
		})

		endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
		require.NoError(t, err)

		endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)

		// Verify endpoint in legacy item_api table
		endpoint, err := ias.GetItemApi(ctx, endpointID)
		require.NoError(t, err)
		assert.Equal(t, "Consistency Test Endpoint", endpoint.Name)
		assert.Equal(t, "POST", endpoint.Method)
		assert.Equal(t, "/test/consistency", endpoint.Url)
		assert.Equal(t, collectionID.Compare(endpoint.CollectionID), 0)

		// Verify endpoint in collection_items table
		allItems, err := queries.GetCollectionItemsByCollectionID(ctx, collectionID)
		require.NoError(t, err)

		// Find the collection item by matching endpoint_id
		var collectionItem *gen.CollectionItem
		for _, item := range allItems {
			if item.EndpointID != nil {
				if item.EndpointID.Compare(endpointID) == 0 {
					collectionItem = &item
					break
				}
			}
		}

		require.NotNil(t, collectionItem)
		assert.Equal(t, int8(1), collectionItem.ItemType) // Endpoint type
		assert.Equal(t, "Consistency Test Endpoint", collectionItem.Name)
		assert.Equal(t, collectionID.Compare(collectionItem.CollectionID), 0)

		// Verify FK relationship
		assert.NotNil(t, collectionItem.EndpointID)
		assert.Equal(t, endpointID.Compare(*collectionItem.EndpointID), 0)
	})

	t.Run("LinkedListConsistency", func(t *testing.T) {
		// Create multiple items to test linked list consistency
		var itemIDs []idwrap.IDWrap

		// Create 5 folders
		for i := 0; i < 5; i++ {
			folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
				CollectionId: collectionID.Bytes(),
				Name:         fmt.Sprintf("Link Test Folder %d", i+1),
			})

			folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
			require.NoError(t, err)

			folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)
			itemIDs = append(itemIDs, folderID)
		}

		// Get collection items and verify linked list structure
		collectionItems, err := cis.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)
		require.Greater(t, len(collectionItems), 5) // Should have at least our 5 new folders

		// Verify linked list structure
		var prevID *idwrap.IDWrap = nil
		for i, item := range collectionItems {
			if i == 0 {
				assert.Nil(t, item.PrevID, "First item should have no prev")
			} else {
				assert.NotNil(t, item.PrevID, "Non-first item should have prev")
				if prevID != nil {
					assert.Equal(t, prevID.Compare(*item.PrevID), 0, "Prev ID should match previous item")
				}
			}

			if i == len(collectionItems)-1 {
				assert.Nil(t, item.NextID, "Last item should have no next")
			} else {
				assert.NotNil(t, item.NextID, "Non-last item should have next")
			}

			prevID = &item.ID
		}

		// Verify symmetry: if A.NextID = B.ID, then B.PrevID = A.ID
		for i := 0; i < len(collectionItems)-1; i++ {
			currentItem := collectionItems[i]
			nextItem := collectionItems[i+1]

			assert.NotNil(t, currentItem.NextID, "Current item should have next ID")
			assert.Equal(t, currentItem.NextID.Compare(nextItem.ID), 0, "Current.NextID should equal Next.ID")
			
			assert.NotNil(t, nextItem.PrevID, "Next item should have prev ID")
			assert.Equal(t, nextItem.PrevID.Compare(currentItem.ID), 0, "Next.PrevID should equal Current.ID")
		}
	})

	t.Run("WorkspacePermissionConsistency", func(t *testing.T) {
		// Create folder
		folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Permission Test Folder",
		})

		folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
		require.NoError(t, err)

		folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

		// Get collection item ID
		allItems, err := queries.GetCollectionItemsByCollectionID(ctx, collectionID)
		require.NoError(t, err)

		// Find the collection item by matching folder_id
		var collectionItem *gen.CollectionItem
		for _, item := range allItems {
			if item.FolderID != nil {
				if item.FolderID.Compare(folderID) == 0 {
					collectionItem = &item
					break
				}
			}
		}
		require.NotNil(t, collectionItem)

		collectionItemID := collectionItem.ID

		// Verify workspace ID consistency through collection item service
		itemWorkspaceID, err := cis.GetWorkspaceID(ctx, collectionItemID)
		require.NoError(t, err)
		assert.Equal(t, workspaceID.Compare(itemWorkspaceID), 0)

		// Verify workspace check
		belongsToWorkspace, err := cis.CheckWorkspaceID(ctx, collectionItemID, workspaceID)
		require.NoError(t, err)
		assert.True(t, belongsToWorkspace)

		// Verify it doesn't belong to different workspace
		differentWorkspaceID := idwrap.NewNow()
		belongsToDifferent, err := cis.CheckWorkspaceID(ctx, collectionItemID, differentWorkspaceID)
		require.NoError(t, err)
		assert.False(t, belongsToDifferent)
	})
}