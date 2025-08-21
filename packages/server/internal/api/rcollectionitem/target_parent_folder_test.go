package rcollectionitem_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestCollectionItemMoveWithTargetParentFolderId tests the targetParentFolderId functionality
func TestCollectionItemMoveWithTargetParentFolderId(t *testing.T) {
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

	t.Run("MoveEndpointFromRootToFolder", func(t *testing.T) {
		// Create folder and endpoint at root level
		folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Target Folder",
			ParentFolderId: nil,
		})
		folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
		require.NoError(t, err)
		folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

		endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Test Endpoint",
			Method:         "GET",
			Url:            "/api/test",
			ParentFolderId: nil,
		})
		endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
		require.NoError(t, err)
		endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)

		// Move endpoint to folder using targetParentFolderId
		moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:               endpointID.Bytes(),
			CollectionId:         collectionID.Bytes(),
			TargetParentFolderId: folderID.Bytes(),
			Position:             resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
		})

		moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
		require.NoError(t, err)
		assert.NotNil(t, moveResp)

		// Verify the endpoint is now inside the folder
		listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:     collectionID.Bytes(),
			ParentFolderId:   folderID.Bytes(),
		})
		listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
		require.NoError(t, err)
		
		// Should have one item (the endpoint) inside the folder
		assert.Len(t, listResp.Msg.Items, 1)
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, listResp.Msg.Items[0].Kind)
		assert.Equal(t, "Test Endpoint", listResp.Msg.Items[0].Endpoint.Name)
	})

	t.Run("MoveFolderToInsideAnotherFolder", func(t *testing.T) {
		// Create two folders at root level
		folder1Req := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Folder 1",
			ParentFolderId: nil,
		})
		folder1Resp, err := folderRPC.FolderCreate(authedCtx, folder1Req)
		require.NoError(t, err)
		folder1ID := idwrap.NewFromBytesMust(folder1Resp.Msg.FolderId)

		folder2Req := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Folder 2",
			ParentFolderId: nil,
		})
		folder2Resp, err := folderRPC.FolderCreate(authedCtx, folder2Req)
		require.NoError(t, err)
		folder2ID := idwrap.NewFromBytesMust(folder2Resp.Msg.FolderId)

		// Move folder1 inside folder2 using targetParentFolderId
		moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:               folder1ID.Bytes(),
			CollectionId:         collectionID.Bytes(),
			TargetParentFolderId: folder2ID.Bytes(),
			Position:             resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
		})

		moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
		require.NoError(t, err)
		assert.NotNil(t, moveResp)

		// Verify folder1 is now inside folder2
		listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:     collectionID.Bytes(),
			ParentFolderId:   folder2ID.Bytes(),
		})
		listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
		require.NoError(t, err)
		
		// Should have one item (folder1) inside folder2
		assert.Len(t, listResp.Msg.Items, 1)
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, listResp.Msg.Items[0].Kind)
		assert.Equal(t, "Folder 1", listResp.Msg.Items[0].Folder.Name)
	})

	t.Run("MoveEndpointFromFolderToRoot", func(t *testing.T) {
		// Create folder at root level
		folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Source Folder",
			ParentFolderId: nil,
		})
		folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
		require.NoError(t, err)
		folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

		// Create endpoint inside the folder
		endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Nested Endpoint",
			Method:         "POST",
			Url:            "/api/nested",
			ParentFolderId: folderID.Bytes(),
		})
		endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
		require.NoError(t, err)
		endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)

		// Move endpoint to root using empty targetParentFolderId
		moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:               endpointID.Bytes(),
			CollectionId:         collectionID.Bytes(),
			TargetParentFolderId: []byte{}, // Explicitly empty bytes to move to root
			Position:             resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
		})

		moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
		require.NoError(t, err)
		assert.NotNil(t, moveResp)

		// Verify the endpoint is now at root level
		listRootReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId: collectionID.Bytes(),
			// ParentFolderId left empty/nil for root level
		})
		listRootResp, err := collectionItemRPC.CollectionItemList(authedCtx, listRootReq)
		require.NoError(t, err)
		
		// Find the endpoint in root items
		endpointFound := false
		for _, item := range listRootResp.Msg.Items {
			if item.Kind == itemv1.ItemKind_ITEM_KIND_ENDPOINT && item.Endpoint.Name == "Nested Endpoint" {
				endpointFound = true
				break
			}
		}
		assert.True(t, endpointFound, "Endpoint should be found at root level")
	})

	t.Run("ErrorNonExistentTargetParentFolder", func(t *testing.T) {
		// Create endpoint at root level
		endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId:   collectionID.Bytes(),
			Name:           "Error Test Endpoint",
			Method:         "GET",
			Url:            "/api/error-test",
			ParentFolderId: nil,
		})
		endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
		require.NoError(t, err)
		endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)

		// Try to move to non-existent folder
		nonExistentFolderID := idwrap.NewNow()
		moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:               endpointID.Bytes(),
			CollectionId:         collectionID.Bytes(),
			TargetParentFolderId: nonExistentFolderID.Bytes(),
			Position:             resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
		})

		_, err = collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
		require.Error(t, err)
		connectErr := err.(*connect.Error)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

// TestCollectionItemMoveExactFailingPayload tests the exact payload that was failing
func TestCollectionItemMoveExactFailingPayload(t *testing.T) {
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

	// Create test data exactly as in the failing scenario
	folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
		CollectionId:   collectionID.Bytes(),
		Name:           "Test Folder",
		ParentFolderId: nil,
	})
	folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
	require.NoError(t, err)
	folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

	endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
		CollectionId:   collectionID.Bytes(),
		Name:           "Test Endpoint",
		Method:         "GET",
		Url:            "/api/test",
		ParentFolderId: nil,
	})
	endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
	require.NoError(t, err)
	endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)

	// Test the exact failing payload structure
	req := &itemv1.CollectionItemMoveRequest{
		Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
		ItemId:               endpointID.Bytes(),
		CollectionId:         collectionID.Bytes(),
		TargetParentFolderId: folderID.Bytes(),
		TargetCollectionId:   collectionID.Bytes(),
		Position:             resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
	}

	t.Logf("Testing exact failing payload:")
	t.Logf("  ItemId: %s", endpointID.String())
	t.Logf("  CollectionId: %s", collectionID.String())
	t.Logf("  TargetParentFolderId: %s", folderID.String())
	t.Logf("  TargetCollectionId: %s", collectionID.String())

	resp, err := collectionItemRPC.CollectionItemMove(authedCtx, connect.NewRequest(req))
	require.NoError(t, err, "The exact failing payload should now work")
	assert.NotNil(t, resp)

	// Validate the move was successful by checking the endpoint is now in the folder
	listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId:   collectionID.Bytes(),
		ParentFolderId: folderID.Bytes(),
	})
	listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
	require.NoError(t, err)
	
	assert.Len(t, listResp.Msg.Items, 1, "Folder should contain exactly one item")
	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, listResp.Msg.Items[0].Kind)
	assert.Equal(t, "Test Endpoint", listResp.Msg.Items[0].Endpoint.Name)

	t.Log("✅ Exact failing payload now works correctly!")
}