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

// TestSimpleCollectionItemWorkflow tests the basic happy path:
// 1. Create folder
// 2. Create endpoint (same parent context)
// 3. List items (verify both appear)
// 4. Move endpoint after folder (same parent context)
// 5. Verify new order
func TestSimpleCollectionItemWorkflow(t *testing.T) {
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

	// Step 1: Create folder via RPC
	t.Log("Step 1: Creating folder...")
	folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
		CollectionId:   collectionID.Bytes(),
		Name:           "Test Folder",
		ParentFolderId: nil, // Root level
	})

	folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
	require.NoError(t, err, "Folder creation should succeed")
	require.NotNil(t, folderResp)
	t.Log("✅ Step 1 passed: Folder created successfully")

	folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

	// Step 2: Create endpoint via RPC (same parent context - root level)
	t.Log("Step 2: Creating endpoint...")
	endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
		CollectionId:   collectionID.Bytes(),
		Name:           "Test Endpoint",
		Method:         "GET",
		Url:            "/api/test",
		ParentFolderId: nil, // Same parent context as folder (root level)
	})

	endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
	require.NoError(t, err, "Endpoint creation should succeed")
	require.NotNil(t, endpointResp)
	t.Log("✅ Step 2 passed: Endpoint created successfully")

	endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)

	// Step 3: List items via RPC (verify both appear)
	t.Log("Step 3: Listing items to verify both appear...")
	listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId:    collectionID.Bytes(),
		ParentFolderId: nil, // List root level items
	})

	listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
	require.NoError(t, err, "Listing items should succeed")
	require.NotNil(t, listResp)

	items := listResp.Msg.Items
	require.Len(t, items, 2, "Should have exactly 2 items (folder + endpoint)")

	// Verify both items appear with correct details
	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, items[0].Kind, "First item should be a folder")
	assert.Equal(t, "Test Folder", items[0].Folder.Name, "Folder name should match")
	assert.Equal(t, folderID.Bytes(), items[0].Folder.FolderId, "Folder ID should match")

	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, items[1].Kind, "Second item should be an endpoint")
	assert.Equal(t, "Test Endpoint", items[1].Endpoint.Name, "Endpoint name should match")
	assert.Equal(t, endpointID.Bytes(), items[1].Endpoint.EndpointId, "Endpoint ID should match")
	t.Log("✅ Step 3 passed: Both items appear correctly in list")

	// Step 4: Move endpoint after folder (same parent context)
	t.Log("Step 4: Moving endpoint after folder...")
	moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
		CollectionId:  collectionID.Bytes(),
		ItemId:       endpointID.Bytes(),
		TargetItemId: folderID.Bytes(),
		Position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
	})

	moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
	require.NoError(t, err, "Move operation should succeed")
	require.NotNil(t, moveResp)
	t.Log("✅ Step 4 passed: Move operation completed successfully")

	// Step 5: List items again and verify new order
	t.Log("Step 5: Verifying new order after move...")
	listAfterMoveResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
	require.NoError(t, err, "Listing after move should succeed")
	require.NotNil(t, listAfterMoveResp)

	itemsAfterMove := listAfterMoveResp.Msg.Items
	require.Len(t, itemsAfterMove, 2, "Should still have exactly 2 items")

	// Verify order is now: folder first, endpoint second (endpoint moved after folder)
	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, itemsAfterMove[0].Kind, "First item should still be the folder")
	assert.Equal(t, "Test Folder", itemsAfterMove[0].Folder.Name, "Folder should be first")

	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, itemsAfterMove[1].Kind, "Second item should be the endpoint")
	assert.Equal(t, "Test Endpoint", itemsAfterMove[1].Endpoint.Name, "Endpoint should be second (after folder)")
	t.Log("✅ Step 5 passed: New order verified correctly")

	t.Log("🎉 All steps passed! Basic collection item workflow is working correctly.")
}

// TestSimpleCollectionItemWorkflowAlternativeOrder tests the same workflow but with endpoint before folder move
func TestSimpleCollectionItemWorkflowAlternativeOrder(t *testing.T) {
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

	// Create folder and endpoint in same order as first test
	folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
		CollectionId:   collectionID.Bytes(),
		Name:           "Another Folder",
		ParentFolderId: nil,
	})

	folderResp, err := folderRPC.FolderCreate(authedCtx, folderReq)
	require.NoError(t, err)
	folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

	endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
		CollectionId:   collectionID.Bytes(),
		Name:           "Another Endpoint",
		Method:         "POST",
		Url:            "/api/another",
		ParentFolderId: nil,
	})

	endpointResp, err := apiRPC.EndpointCreate(authedCtx, endpointReq)
	require.NoError(t, err)
	endpointID := idwrap.NewFromBytesMust(endpointResp.Msg.EndpointId)

	// Move endpoint BEFORE folder this time (opposite of first test)
	t.Log("Testing BEFORE position: Moving endpoint before folder...")
	moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
		CollectionId:  collectionID.Bytes(),
		ItemId:       endpointID.Bytes(),
		TargetItemId: folderID.Bytes(),
		Position:     resourcesv1.MovePosition_MOVE_POSITION_BEFORE.Enum(),
	})

	moveResp, err := collectionItemRPC.CollectionItemMove(authedCtx, moveReq)
	require.NoError(t, err, "Move BEFORE should succeed")
	require.NotNil(t, moveResp)

	// Verify new order: endpoint first, folder second
	listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId:    collectionID.Bytes(),
		ParentFolderId: nil,
	})

	listAfterMoveResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
	require.NoError(t, err)

	itemsAfterMove := listAfterMoveResp.Msg.Items
	require.Len(t, itemsAfterMove, 2)

	// Verify order is now: endpoint first, folder second (endpoint moved before folder)
	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, itemsAfterMove[0].Kind, "First item should be the endpoint")
	assert.Equal(t, "Another Endpoint", itemsAfterMove[0].Endpoint.Name, "Endpoint should be first")

	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_FOLDER, itemsAfterMove[1].Kind, "Second item should be the folder")
	assert.Equal(t, "Another Folder", itemsAfterMove[1].Folder.Name, "Folder should be second")
	t.Log("✅ BEFORE position test passed: Endpoint moved before folder correctly")
}