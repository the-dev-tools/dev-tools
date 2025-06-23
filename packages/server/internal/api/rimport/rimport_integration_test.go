package rimport_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
)

// TestImportWithWorkspaceUpdateInTransaction verifies workspace updates happen atomically
func TestImportWithWorkspaceUpdateInTransaction(t *testing.T) {
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

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Get initial workspace state
	initialWS, err := ws.Get(ctx, workspaceID)
	require.NoError(t, err)
	initialCollectionCount := initialWS.CollectionCount

	// Create ImportRPC
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// Test 1: Successful curl import should update workspace atomically
	curlStr := `curl 'http://example.com/api/v1/users' -H 'Content-Type: application/json' --data '{"name": "test"}'`
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr,
		Name:        "New Collection",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify workspace was updated
	updatedWS, err := ws.Get(ctx, workspaceID)
	require.NoError(t, err)
	assert.Equal(t, initialCollectionCount+1, updatedWS.CollectionCount, "Collection count should increase by 1")

	// Test 2: Import to existing collection should not increase count
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    `curl 'http://example.com/api/v1/posts' -H 'Content-Type: application/json'`,
		Name:        "New Collection", // Same name
	})

	resp2, err := importRPC.Import(authedCtx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	// Verify workspace collection count didn't change
	finalWS, err := ws.Get(ctx, workspaceID)
	require.NoError(t, err)
	assert.Equal(t, updatedWS.CollectionCount, finalWS.CollectionCount, "Collection count should not change when using existing collection")
}

// TestImportHarEmptyData verifies HAR import falls back to Postman for empty/invalid HAR
func TestImportHarEmptyData(t *testing.T) {
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

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create ImportRPC
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// Create HAR data with no valid entries - this is also valid as empty Postman collection
	emptyHAR := []byte(`{
		"log": {
			"version": "1.2",
			"entries": []
		}
	}`)

	// First request to parse HAR (should work even with empty data)
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        emptyHAR,
		Name:        "Test Import",
		Filter:      []string{}, // No filter for first request
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err) // Should succeed and return empty filter options
	require.NotNil(t, resp1)
	require.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp1.Msg.Kind)
	require.Empty(t, resp1.Msg.Filter) // No domains found
	
	// Second request with filter - HAR will fail but should fall back to Postman
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        emptyHAR,
		Name:        "Test Import",
		Filter:      []string{"example.com"},
	})
	
	resp2, err := importRPC.Import(authedCtx, req2)
	
	// This should succeed because it falls back to Postman import
	require.NoError(t, err, "Should succeed via Postman fallback")
	require.NotNil(t, resp2)
	
	// Verify collection was created
	// HAR imports now always use "Imported" as collection name
	collection, collErr := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, collErr)
	require.NotNil(t, collection)
	
	// Verify no flow was created (Postman imports don't create flows)
	assert.Nil(t, resp2.Msg.Flow, "Postman imports should not create flows")
}

// TestImportTransactionRollback verifies proper rollback on errors
func TestImportTransactionRollback(t *testing.T) {
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

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Get initial state
	initialWS, err := ws.Get(ctx, workspaceID)
	require.NoError(t, err)
	initialCount := initialWS.CollectionCount

	// Create ImportRPC
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// Test with very large curl data that might cause issues
	// Create a curl with invalid JSON body that will fail during parsing
	curlStr := `curl 'http://example.com/api' -H 'Content-Type: application/json' --data '{"invalid json: }'`
	
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr,
		Name:        "Test Collection",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	_, err = importRPC.Import(authedCtx, req)
	
	// This might succeed or fail depending on curl parsing, but workspace should be consistent
	
	// Verify workspace state is consistent
	finalWS, err2 := ws.Get(ctx, workspaceID)
	require.NoError(t, err2)
	
	if err != nil {
		// If import failed, count should not change
		assert.Equal(t, initialCount, finalWS.CollectionCount, "Collection count should not change on error")
	} else {
		// If import succeeded, count should increase by at most 1
		assert.LessOrEqual(t, finalWS.CollectionCount, initialCount+1, "Collection count should increase by at most 1")
	}
}

// TestImportHarWithExistingCollection tests HAR import into existing collection
func TestImportHarWithExistingCollection(t *testing.T) {
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

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create ImportRPC
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// First, create a collection with curl
	curlStr := `curl 'http://example.com/api/v1/users' -H 'Content-Type: application/json'`
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr,
		Name:        "API Collection",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1)

	// Get workspace state after first import
	ws1, err := ws.Get(ctx, workspaceID)
	require.NoError(t, err)
	collectionCount1 := ws1.CollectionCount
	flowCount1 := ws1.FlowCount

	// Now import HAR with same collection name
	harData := createValidHARDataForTest()
	
	// First request to get filter options
	req2a := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harData,
		Name:        "API Collection", // Same name
		Filter:      []string{}, // Empty filter to get options
	})

	resp2a, err := importRPC.Import(authedCtx, req2a)
	require.NoError(t, err)
	require.NotNil(t, resp2a)
	require.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp2a.Msg.Kind)
	
	// Second request with filter
	req2b := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harData,
		Name:        "API Collection", // Same name
		Filter:      []string{"example.com"},
	})

	resp2b, err := importRPC.Import(authedCtx, req2b)
	require.NoError(t, err)
	require.NotNil(t, resp2b)

	// Verify workspace counts
	ws2, err := ws.Get(ctx, workspaceID)
	require.NoError(t, err)
	
	// Collection count should increase by 1 (HAR imports now use "Imported" collection)
	assert.Equal(t, collectionCount1+1, ws2.CollectionCount, "Collection count should increase by 1 for new 'Imported' collection")
	
	// Flow count should increase by 1 (HAR creates flows)
	assert.Equal(t, flowCount1+1, ws2.FlowCount, "Flow count should increase by 1")
}

func createValidHARDataForTest() []byte {
	return []byte(`{
		"log": {
			"version": "1.2",
			"creator": {
				"name": "Test",
				"version": "1.0"
			},
			"entries": [{
				"startedDateTime": "2023-01-01T00:00:00.000Z",
				"_resourceType": "xhr",
				"request": {
					"method": "GET",
					"url": "http://example.com/api/test",
					"httpVersion": "HTTP/1.1",
					"headers": [],
					"queryString": [],
					"cookies": [],
					"headersSize": -1,
					"bodySize": -1
				},
				"response": {
					"status": 200,
					"statusText": "OK",
					"httpVersion": "HTTP/1.1",
					"headers": [],
					"cookies": [],
					"content": {
						"size": 0,
						"mimeType": "application/json"
					},
					"redirectURL": "",
					"headersSize": -1,
					"bodySize": -1
				},
				"cache": {},
				"timings": {
					"send": 0,
					"wait": 0,
					"receive": 0
				}
			}]
		}
	}`)
}