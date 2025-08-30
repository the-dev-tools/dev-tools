package rimport_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
)

func TestCurlImportCollectionItemsOrdering(t *testing.T) {
	// Setup test context and database
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	// Initialize services
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)
	as := sassert.New(queries)
	cis := scollectionitem.New(queries, mockLogger)

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Test curl command
	curlStr := `curl 'http://example.com/api/users' \
  -H 'Accept: application/json' \
  -H 'Content-Type: application/json' \
  --data-raw '{"name":"John Doe"}'`

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// Create request
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr,
		Name:        "Test Collection",
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)

	// Assertions for successful import
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Find the created collection
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Test Collection")
	require.NoError(t, err)
	collectionID := collection.ID

	// Verify that endpoints were created
	apis, err := ias.GetApisWithCollectionID(ctx, collectionID)
	require.NoError(t, err)
	assert.Greater(t, len(apis), 0, "Should have created at least one endpoint")

	// Verify that examples were created
	examples, err := iaes.GetApiExampleByCollection(ctx, collectionID)
	require.NoError(t, err)
	assert.Greater(t, len(examples), 0, "Should have created at least one example")

	// NOW TEST THE MAIN ISSUE: CollectionItemList should return proper items with linked-list ordering
	
	// Create CollectionItemRPC service
	collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)

	// Test CollectionItemList API
	listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId: collectionID.Bytes(),
	})

	listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
	require.NoError(t, err, "CollectionItemList should not return an error")
	assert.NotNil(t, listResp, "CollectionItemList response should not be nil")

	// The main bug: CollectionItemList returns "{}" because no collection_items exist
	// This should return the endpoint as a collection item
	assert.NotEmpty(t, listResp.Msg.Items, "CollectionItemList should return items, not empty array")
	
	// Verify that the returned items are properly ordered (have linked-list structure)
	items := listResp.Msg.Items
	assert.Equal(t, 1, len(items), "Should have exactly one collection item (the endpoint)")
	
	// Verify that the item is an endpoint
	item := items[0]
	assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, item.Kind, "Item should be an endpoint")
	assert.NotNil(t, item.Endpoint, "Endpoint should not be nil")
	assert.NotNil(t, item.Example, "Example should not be nil")
	
	// Verify endpoint details match what we imported
	// Note: EndpointListItem doesn't include URL, but we can verify method and name
	assert.Equal(t, "POST", item.Endpoint.GetMethod()) // Should be POST due to --data-raw
	assert.Equal(t, "http://example.com/api/users", item.Endpoint.GetName()) // Name should be the URL
}

func TestMultipleCurlImportsCollectionItemsOrdering(t *testing.T) {
	// Setup test context and database  
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	// Initialize services
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)
	as := sassert.New(queries)
	cis := scollectionitem.New(queries, mockLogger)

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create ImportRPC
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	collectionName := "Multi-Endpoint Collection"

	// Import first curl command
	curlStr1 := `curl 'http://example.com/api/users' -H 'Content-Type: application/json'`
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr1,
		Name:        collectionName,
	})
	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err)
	assert.NotNil(t, resp1)

	// Import second curl command (should reuse collection)
	curlStr2 := `curl 'http://example.com/api/posts' -H 'Content-Type: application/json' --data '{"title":"Hello"}'`
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr2,
		Name:        collectionName, // Same collection name
	})
	resp2, err := importRPC.Import(authedCtx, req2)
	require.NoError(t, err)
	assert.NotNil(t, resp2)

	// Get collection
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, collectionName)
	require.NoError(t, err)
	collectionID := collection.ID

	// Verify endpoints were created
	apis, err := ias.GetApisWithCollectionID(ctx, collectionID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(apis), "Should have 2 endpoints")

	// Test CollectionItemList
	collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)
	listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId: collectionID.Bytes(),
	})

	listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
	require.NoError(t, err)
	assert.NotNil(t, listResp)

	// The main issue: should return 2 collection items, not empty
	assert.Equal(t, 2, len(listResp.Msg.Items), "Should have 2 collection items")
	
	// Verify proper ordering - items should be in the order they were imported
	items := listResp.Msg.Items
	for _, item := range items {
		assert.Equal(t, itemv1.ItemKind_ITEM_KIND_ENDPOINT, item.Kind)
		assert.NotNil(t, item.Endpoint)
		assert.NotNil(t, item.Example)
	}

	// Check endpoint names to verify ordering (names should be URLs in curl imports)
	names := []string{items[0].Endpoint.GetName(), items[1].Endpoint.GetName()}
	assert.Contains(t, names, "http://example.com/api/users", "Should contain users endpoint")
	assert.Contains(t, names, "http://example.com/api/posts", "Should contain posts endpoint")
}