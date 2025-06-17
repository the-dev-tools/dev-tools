package rimport_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	fmt.Println(err)
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

	// Create request
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        "Test HAR Import with Folders",
		Filter:      []string{"api.example.com", "other.example.com"}, // Include both domains
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Collection)
	assert.NotNil(t, resp.Msg.Flow)

	// Verify the collection was created
	collectionID, err := idwrap.NewFromBytes(resp.Msg.Collection.CollectionId)
	require.NoError(t, err)

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

	// Create request
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        "Test HAR Import Simple URLs",
		Filter:      []string{"example.com"},
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Collection)

	// Verify the collection was created
	collectionID, err := idwrap.NewFromBytes(resp.Msg.Collection.CollectionId)
	require.NoError(t, err)

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
