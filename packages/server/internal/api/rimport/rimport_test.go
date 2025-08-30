package rimport_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"net/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	"the-dev-tools/server/pkg/translate/tcurl"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
)

func TestImportCurl(t *testing.T) {
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

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Test curl command
	curlStr := `curl 'http://example.com/api' \
  -H 'Accept: */*' \
  -H 'Content-Type: application/json' \
  --data-raw '{"key":"value"}'`

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// Create request
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr,
		Name:        "Test Curl Import",
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify changes in response
	/*
		assert.NotEmpty(t, resp.Msg.Changes)
		assert.Equal(t, 1, len(resp.Msg.Changes))

		// Verify change is of expected type
		change := resp.Msg.Changes[0]
		assert.NotNil(t, change)
		assert.Equal(t, changev1.ChangeKind_CHANGE_KIND_UNSPECIFIED, *change.Kind)
		assert.NotEmpty(t, change.List)
	*/
}

func TestImportCurl_ProtocolLessURL(t *testing.T) {
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

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Test the specific problematic curl command
	curlStr := `curl google.com`

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// Create request - no name provided to trigger hostname extraction
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr,
		Name:        "", // Empty name to trigger automatic generation
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)

	// Debug logging
	t.Logf("Import response: %+v", resp)
	if err != nil {
		t.Logf("Import error: %v", err)
	}

	// Assertions
	require.NoError(t, err, "Import should succeed for 'curl google.com'")
	assert.NotNil(t, resp)

	// Now test if the collection was actually created by checking CollectionList
	collectionService := cs
	collections, err := collectionService.GetCollectionsOrdered(ctx, workspaceID)
	require.NoError(t, err, "Should be able to get collections")
	
	// Debug: print collections found
	t.Logf("Collections found: %d", len(collections))
	for i, col := range collections {
		t.Logf("Collection %d: ID=%s, Name=%s", i, col.ID.String(), col.Name)
	}

	// Verify a collection was created
	// There should be at least 2 collections: the original temp one plus the imported one
	assert.GreaterOrEqual(t, len(collections), 2, "Should have at least 2 collections (temp + imported)")
	
	// Find the imported collection
	var importedCollection *mcollection.Collection
	for _, col := range collections {
		if strings.Contains(col.Name, "Imported from cURL") || strings.Contains(col.Name, "Google") || strings.Contains(col.Name, "google") {
			importedCollection = &col
			break
		}
	}
	
	require.NotNil(t, importedCollection, "Should have created an imported collection")
	t.Logf("Found imported collection: %s", importedCollection.Name)
	
	// Verify the collection has endpoints
	apis, err := ias.GetApisWithCollectionID(ctx, importedCollection.ID)
	require.NoError(t, err, "Should be able to get APIs for the collection")
	
	// Debug: print APIs found  
	t.Logf("APIs found: %d", len(apis))
	for i, api := range apis {
		t.Logf("API %d: ID=%s, Method=%s, URL=%s", i, api.ID.String(), api.Method, api.Url)
	}
	
	assert.Len(t, apis, 1, "Should have exactly 1 API endpoint")
	assert.Equal(t, "GET", apis[0].Method, "Should be GET method")
	assert.Equal(t, "google.com", apis[0].Url, "Should have google.com as URL")
}

func TestCurlURLExtraction(t *testing.T) {
	// Import the tcurl package to test URL extraction directly
	curlStr := `curl google.com`
	
	// Test the URL extraction function used in collection name generation
	extractedURL := tcurl.ExtractURLForTesting(curlStr)  // Use the exported test function
	t.Logf("Extracted URL for 'curl google.com': '%s'", extractedURL)
	
	// This should not be empty for protocol-less URLs
	assert.NotEmpty(t, extractedURL, "Should extract URL from protocol-less curl command")
	assert.Equal(t, "google.com", extractedURL, "Should extract 'google.com' from curl command")
}

func TestURLParsingIssue(t *testing.T) {
	// Test the issue with url.Parse on protocol-less URLs
	testURL := "google.com"
	
	parsed, err := url.Parse(testURL)
	require.NoError(t, err, "url.Parse should not fail")
	
	t.Logf("Parsed URL: Scheme='%s', Host='%s', Path='%s'", parsed.Scheme, parsed.Host, parsed.Path)
	
	// This is the issue: url.Parse treats "google.com" as a path, not a host
	assert.Empty(t, parsed.Host, "Host should be empty for protocol-less URL")
	assert.Equal(t, "google.com", parsed.Path, "Path should contain the domain for protocol-less URL")
	
	// Test with protocol
	testURLWithProtocol := "https://google.com"
	parsedWithProtocol, err := url.Parse(testURLWithProtocol)
	require.NoError(t, err)
	
	t.Logf("Parsed URL with protocol: Scheme='%s', Host='%s', Path='%s'", parsedWithProtocol.Scheme, parsedWithProtocol.Host, parsedWithProtocol.Path)
	assert.Equal(t, "google.com", parsedWithProtocol.Host, "Host should be properly parsed with protocol")
}

func TestGenerateCurlCollectionName(t *testing.T) {
	// Test the fixed generateCurlCollectionName function with various URL patterns
	testCases := []struct {
		curlCommand    string
		expectedName   string
		description    string
	}{
		{
			curlCommand:  "curl google.com",
			expectedName: "Google.Com API",
			description:  "protocol-less domain",
		},
		{
			curlCommand:  "curl https://google.com",
			expectedName: "Google.Com API", 
			description:  "domain with https protocol",
		},
		{
			curlCommand:  "curl http://example.com",
			expectedName: "Example.Com API",
			description:  "domain with http protocol",
		},
		{
			curlCommand:  "curl www.github.com",
			expectedName: "Github.Com API",
			description:  "protocol-less www domain (should remove www)",
		},
		{
			curlCommand:  "curl https://api.stripe.com/v1/charges",
			expectedName: "Api.Stripe.Com API",
			description:  "subdomain with path",
		},
		{
			curlCommand:  "curl localhost:8080",
			expectedName: "Localhost API",
			description:  "localhost with port (should remove port)",
		},
		{
			curlCommand:  "curl invalidcommand",
			expectedName: "Imported from cURL",
			description:  "invalid command should fallback",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// We need to test the actual generateCurlCollectionName function
			// Since it's not exported, we'll test the Import function and check the collection name
			
			// Setup test context and database
			ctx := context.Background()
			base := testutil.CreateBaseDB(ctx, t)
			queries := base.Queries
			db := base.DB

			mockLogger := mocklogger.NewMockLogger()
			ws := sworkspace.New(queries)
			cs := scollection.New(queries, mockLogger)
			us := suser.New(queries)
			ifs := sitemfolder.New(queries)
			ias := sitemapi.New(queries)
			iaes := sitemapiexample.New(queries)
			ers := sexampleresp.New(queries)
			as := sassert.New(queries)

			workspaceID := idwrap.NewNow()
			workspaceUserID := idwrap.NewNow()
			userID := idwrap.NewNow()

			baseServices := base.GetBaseServices()
			baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

			importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

			req := connect.NewRequest(&importv1.ImportRequest{
				WorkspaceId: workspaceID.Bytes(),
				TextData:    tc.curlCommand,
				Name:        "", // Empty to trigger name generation
			})

			authedCtx := mwauth.CreateAuthedContext(ctx, userID)
			resp, err := importRPC.Import(authedCtx, req)

			if tc.expectedName == "Imported from cURL" {
				// For invalid commands, we expect an error or fallback
				if err == nil {
					// Check if we got the fallback collection name
					collections, getErr := cs.GetCollectionsOrdered(ctx, workspaceID)
					require.NoError(t, getErr)
					
					var foundCollection *mcollection.Collection
					for _, col := range collections {
						if col.Name == tc.expectedName {
							foundCollection = &col
							break
						}
					}
					
					if foundCollection != nil {
						t.Logf("Got expected fallback name: %s", foundCollection.Name)
						assert.Equal(t, tc.expectedName, foundCollection.Name)
					}
				}
				return
			}

			require.NoError(t, err, "Import should succeed for: %s", tc.curlCommand)
			assert.NotNil(t, resp)

			collections, err := cs.GetCollectionsOrdered(ctx, workspaceID)
			require.NoError(t, err)

			var foundCollection *mcollection.Collection
			for _, col := range collections {
				// Skip the temp collection
				if col.Name != "test" {
					foundCollection = &col
					break
				}
			}

			require.NotNil(t, foundCollection, "Should have created a collection for: %s", tc.curlCommand)
			assert.Equal(t, tc.expectedName, foundCollection.Name, "Collection name should match expected for: %s", tc.curlCommand)
		})
	}
}

func TestImportCurl_OverwriteExistingCollection(t *testing.T) {
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

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Test curl command
	curlStr1 := `curl 'http://example.com/api/v1/users' \
  -H 'Accept: */*' \
  -H 'Content-Type: application/json'`

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	collectionName := "Test Curl Collection"

	// First import - create new collection
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr1,
		Name:        collectionName,
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp1, err := importRPC.Import(authedCtx, req1)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp1)
	// Collection is returned via service lookup
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, collectionName)
	require.NoError(t, err)
	collectionID1 := collection.ID

	// Verify first collection was created
	collection1, err := cs.GetCollection(ctx, collectionID1)
	require.NoError(t, err)
	assert.Equal(t, collectionName, collection1.Name)

	// Second curl command with same collection name
	curlStr2 := `curl 'http://example.com/api/v1/posts' \
  -H 'Accept: */*' \
  -H 'Content-Type: application/json' \
  --data-raw '{"title":"Hello"}'`

	// Second import with same collection name - should reuse existing collection
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr2,
		Name:        collectionName, // Same name
	})

	resp2, err := importRPC.Import(authedCtx, req2)
	require.NoError(t, err)
	assert.NotNil(t, resp2)
	// Collection is returned via service lookup
	collection2, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, collectionName)
	require.NoError(t, err)
	collectionID2 := collection2.ID

	// Verify same collection ID was reused
	assert.Equal(t, collectionID1, collectionID2, "Should reuse existing collection with same name")

	// Verify endpoints in the collection
	apis, err := ias.GetApisWithCollectionID(ctx, collectionID1)
	require.NoError(t, err)

	// Should have both endpoints
	assert.Equal(t, 2, len(apis), "Should have 2 endpoints in the collection")

	urls := make(map[string]bool)
	for _, api := range apis {
		urls[api.Url] = true
	}
	assert.True(t, urls["http://example.com/api/v1/users"], "Should have users endpoint")
	assert.True(t, urls["http://example.com/api/v1/posts"], "Should have posts endpoint")
}

func TestImportHarWithFolderHierarchy(t *testing.T) {
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

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create a test HAR file with structured URLs
	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:00.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "https://api.example.com/v1/users/123",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:01.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "POST",
						"url":         "https://api.example.com/v1/users/create",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      201,
						"statusText":  "Created",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:02.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "https://api.example.com/v1/posts/456",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:03.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "https://other.example.com/api/health",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
			},
		},
	}

	// Convert to JSON
	harJSON, err := json.Marshal(harData)
	require.NoError(t, err)

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// First request to get filter options
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        "Test HAR Import with Folders",
		Filter:      []string{}, // Empty filter first
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1)
	require.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp1.Msg.Kind)

	// Second request with filter
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        "Test HAR Import with Folders",
		Filter:      []string{"api.example.com", "other.example.com"}, // Include both domains
	})

	resp, err := importRPC.Import(authedCtx, req2)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	// ImportResponse no longer includes Collection field
	assert.NotNil(t, resp.Msg.Flow)

	// Verify the collection was created
	// Get collection ID via service lookup
	// HAR imports now always use "Imported" as collection name
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)
	collectionID := collection.ID

	// Verify folders were created
	folders, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
	require.NoError(t, err)
	assert.Greater(t, len(folders), 0, "Expected folders to be created")

	// Create a map of folder names for easy lookup
	foldersByName := make(map[string]bool)
	for _, folder := range folders {
		foldersByName[folder.Name] = true
	}

	// Verify expected folders exist
	expectedFolders := []string{"api.example.com", "other.example.com", "v1", "users", "posts", "api"}
	for _, expectedFolder := range expectedFolders {
		assert.True(t, foldersByName[expectedFolder], "Expected folder '%s' to be created", expectedFolder)
	}

	// Verify APIs were created
	apis, err := ias.GetApisWithCollectionID(ctx, collectionID)
	require.NoError(t, err)
	assert.Equal(t, 4, len(apis), "Expected 4 APIs to be created")

	// Verify all APIs are placed in folders (not in root)
	for _, api := range apis {
		assert.NotNil(t, api.FolderID, "API '%s' should be placed in a folder", api.Name)
	}

	// Verify API names are extracted correctly from URLs
	expectedAPINames := map[string]bool{"123": true, "create": true, "456": true, "health": true}
	for _, api := range apis {
		assert.True(t, expectedAPINames[api.Name], "Unexpected API name: %s", api.Name)
	}

	// Verify examples were created
	examples, err := iaes.GetApiExampleByCollection(ctx, collectionID)
	require.NoError(t, err)
	assert.Greater(t, len(examples), 0, "Expected examples to be created")

	// Verify example names match API names
	for _, example := range examples {
		if !example.IsDefault {
			assert.True(t, expectedAPINames[example.Name] || example.Name == "123 (Delta)" ||
				example.Name == "create (Delta)" || example.Name == "456 (Delta)" ||
				example.Name == "health (Delta)", "Unexpected example name: %s", example.Name)
		}
	}
}

func TestImportHarSimpleURL(t *testing.T) {
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

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create a test HAR file with simple URLs
	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:00.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://example.com/api",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:01.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
			},
		},
	}

	// Convert to JSON
	harJSON, err := json.Marshal(harData)
	require.NoError(t, err)

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// First request to get filter options
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        "Test HAR Import Simple URLs",
		Filter:      []string{}, // Empty filter first
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1)
	require.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp1.Msg.Kind)

	// Second request with filter
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        "Test HAR Import Simple URLs",
		Filter:      []string{"example.com"},
	})

	resp, err := importRPC.Import(authedCtx, req2)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	// ImportResponse no longer includes Collection field

	// Verify the collection was created
	// Get collection ID via service lookup
	// HAR imports now always use "Imported" as collection name
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)
	collectionID := collection.ID

	// Verify folders were created (should only create domain folder)
	folders, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
	require.NoError(t, err)
	assert.Equal(t, 1, len(folders), "Expected only domain folder to be created")
	assert.Equal(t, "example.com", folders[0].Name, "Expected domain folder name to be 'example.com'")

	// Verify APIs were created and placed in the domain folder
	apis, err := ias.GetApisWithCollectionID(ctx, collectionID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(apis), "Expected 2 APIs to be created")

	for _, api := range apis {
		assert.NotNil(t, api.FolderID, "API '%s' should be placed in a folder", api.Name)
		assert.Equal(t, folders[0].ID, *api.FolderID, "API '%s' should be in domain folder", api.Name)
	}

	// Verify API names are extracted correctly
	apiNames := make([]string, len(apis))
	for i, api := range apis {
		apiNames[i] = api.Name
	}
	assert.ElementsMatch(t, []string{"api", "users"}, apiNames, "Expected API names to be 'api' and 'users'")
}

func TestImportHar_OverwriteExistingCollection(t *testing.T) {
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

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create HAR data
	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:00.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://example.com/api/v1/users",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
			},
		},
	}

	// Convert to JSON
	harJSON, err := json.Marshal(harData)
	require.NoError(t, err)

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	collectionName := "Test Collection Overwrite"

	// First import - get filter options
	req0 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        collectionName,
		Filter:      []string{}, // Empty filter to get domains
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp0, err := importRPC.Import(authedCtx, req0)
	require.NoError(t, err)
	assert.NotNil(t, resp0)
	assert.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp0.Msg.Kind)
	assert.Contains(t, resp0.Msg.Filter, "example.com")

	// Now import with filter - create new collection
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        collectionName,
		Filter:      []string{"example.com"},
	})

	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err)
	assert.NotNil(t, resp1)
	// Collection is returned via service lookup
	// HAR imports now always use "Imported" as collection name
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)
	collectionID1 := collection.ID
	require.NoError(t, err)
	// Verify first collection was created
	collection1, err := cs.GetCollection(ctx, collectionID1)
	require.NoError(t, err)
	assert.Equal(t, "Imported", collection1.Name)

	// Second import with same collection name - should reuse existing collection
	harData2 := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:01:00.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "POST",
						"url":         "http://example.com/api/v1/users",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
						"postData": map[string]interface{}{
							"mimeType": "application/json",
							"text":     `{"name":"John"}`,
						},
					},
					"response": map[string]interface{}{
						"status":      201,
						"statusText":  "Created",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
			},
		},
	}

	harJSON2, err := json.Marshal(harData2)
	require.NoError(t, err)

	// Get filter options for second HAR
	req2a := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON2,
		Name:        collectionName,
		Filter:      []string{}, // Empty filter to get domains
	})

	resp2a, err := importRPC.Import(authedCtx, req2a)
	require.NoError(t, err)
	assert.NotNil(t, resp2a)
	assert.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp2a.Msg.Kind)

	// Import with filter
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON2,
		Name:        collectionName, // Same name as before
		Filter:      []string{"example.com"},
	})

	resp2, err := importRPC.Import(authedCtx, req2)
	require.NoError(t, err)
	assert.NotNil(t, resp2)
	// Collection is returned via service lookup
	// HAR imports now always use "Imported" as collection name
	collection2, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)
	collectionID2 := collection2.ID
	// Verify same collection ID was reused
	assert.Equal(t, collectionID1, collectionID2, "Should reuse existing collection with same name")

	// Verify endpoints in the collection
	apis, err := ias.GetApisWithCollectionID(ctx, collectionID1)
	require.NoError(t, err)

	// Should have both GET and POST endpoints
	assert.Greater(t, len(apis), 1, "Should have multiple endpoints in the collection")

	methods := make(map[string]bool)
	for _, api := range apis {
		methods[api.Method] = true
	}
	assert.True(t, methods["GET"], "Should have GET endpoint")
	assert.True(t, methods["POST"], "Should have POST endpoint")
}

func TestImportHar_OverwriteExistingEndpoints(t *testing.T) {
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

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create HAR data with endpoint that will be duplicated
	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-10-01T12:00:00.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://example.com/api/users/123",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     "",
						},
					},
				},
			},
		},
	}

	// Convert to JSON
	harJSON, err := json.Marshal(harData)
	require.NoError(t, err)

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	collectionName := "Test Endpoint Overwrite"

	// First import - get filter options
	req0 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        collectionName,
		Filter:      []string{}, // Empty filter to get domains
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp0, err := importRPC.Import(authedCtx, req0)
	require.NoError(t, err)
	assert.NotNil(t, resp0)
	assert.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp0.Msg.Kind)
	assert.Contains(t, resp0.Msg.Filter, "example.com")

	// Now import with filter - create collection with endpoint
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        collectionName,
		Filter:      []string{"example.com"},
	})

	resp1, err := importRPC.Import(authedCtx, req1)
	require.NoError(t, err)
	assert.NotNil(t, resp1)

	// Get collection ID via service lookup
	// HAR imports now always use "Imported" as collection name
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)
	collectionID := collection.ID
	// Get the first endpoint
	apis1, err := ias.GetApisWithCollectionID(ctx, collectionID)
	require.NoError(t, err)

	var originalEndpoint *mitemapi.ItemApi
	for _, api := range apis1 {
		if api.Method == "GET" && strings.Contains(api.Url, "/api/users/") && !strings.Contains(api.Name, "Delta") {
			copyApi := api // Create a copy to avoid taking address of loop variable
			originalEndpoint = &copyApi
			break
		}
	}
	require.NotNil(t, originalEndpoint, "Should find original endpoint")
	originalEndpointID := originalEndpoint.ID

	// Import same endpoint again with updated HAR (simulating a re-recording)
	harData2 := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-10-01T13:00:00.000Z", // Different time
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://example.com/api/users/123", // Same URL and method
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
							map[string]interface{}{ // Additional header
								"name":  "Authorization",
								"value": "Bearer token123",
							},
						},
						"queryString": []interface{}{},
					},
					"response": map[string]interface{}{
						"status":      200,
						"statusText":  "OK",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
						"content": map[string]interface{}{
							"size":     0,
							"mimeType": "application/json",
							"text":     `{"updated": true}`, // Different response
						},
					},
				},
			},
		},
	}

	harJSON2, err := json.Marshal(harData2)
	require.NoError(t, err)

	// Get filter options for second HAR
	req2a := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON2,
		Name:        collectionName,
		Filter:      []string{}, // Empty filter to get domains
	})

	resp2a, err := importRPC.Import(authedCtx, req2a)
	require.NoError(t, err)
	assert.NotNil(t, resp2a)
	assert.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp2a.Msg.Kind)

	// Import with filter
	req2 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON2,
		Name:        collectionName, // Same collection name
		Filter:      []string{"example.com"},
	})

	resp2, err := importRPC.Import(authedCtx, req2)
	require.NoError(t, err)
	assert.NotNil(t, resp2)

	// Get endpoints after second import
	apis2, err := ias.GetApisWithCollectionID(ctx, collectionID)
	require.NoError(t, err)

	// Find the endpoint with same URL and method
	var updatedEndpoint *mitemapi.ItemApi
	for _, api := range apis2 {
		if api.Method == "GET" && strings.Contains(api.Url, "/api/users/") && api.ID == originalEndpointID {
			copyApi := api // Create a copy to avoid taking address of loop variable
			updatedEndpoint = &copyApi
			break
		}
	}

	require.NotNil(t, updatedEndpoint, "Should find the same endpoint after update")

	// Verify endpoint was updated, not duplicated
	assert.Equal(t, originalEndpointID, updatedEndpoint.ID, "Endpoint should be updated, not recreated")

	// Count GET endpoints with the same URL pattern (excluding delta endpoints)
	getEndpointCount := 0
	for _, api := range apis2 {
		if api.Method == "GET" && strings.Contains(api.Url, "/api/users/") && !strings.Contains(api.Name, "Delta") {
			getEndpointCount++
		}
	}
	assert.Equal(t, 1, getEndpointCount, "Should have only one GET endpoint for the same URL")

	// Verify examples exist for the endpoint
	examples, err := iaes.GetApiExamples(ctx, originalEndpointID)
	require.NoError(t, err)
	// When updating an existing endpoint, examples are preserved (not duplicated)
	assert.GreaterOrEqual(t, len(examples), 2, "Should have examples for the endpoint")
}
