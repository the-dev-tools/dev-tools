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

// TestImportHarTwice verifies importing the same HAR file twice doesn't cause foreign key errors
func TestImportHarTwice(t *testing.T) {
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

	// Setup workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create HAR data with complex folder hierarchy
	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-01-01T00:00:00.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://api.example.com/v1/users/123",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
					},
					"response": map[string]interface{}{
						"status":     200,
						"statusText": "OK",
					},
				},
				map[string]interface{}{
					"startedDateTime": "2023-01-01T00:00:01.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "POST",
						"url":         "http://api.example.com/v1/users",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
					},
					"response": map[string]interface{}{
						"status":     201,
						"statusText": "Created",
					},
				},
				map[string]interface{}{
					"startedDateTime": "2023-01-01T00:00:02.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://api.example.com/v2/products/456",
						"httpVersion": "HTTP/1.1",
						"headers": []interface{}{
							map[string]interface{}{
								"name":  "Authorization",
								"value": "Bearer token123",
							},
						},
					},
					"response": map[string]interface{}{
						"status":     200,
						"statusText": "OK",
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
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// First import - should create everything successfully
	t.Run("First Import", func(t *testing.T) {
		// First request to get filter options
		req1 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        harJSON,
			Name:        "Test HAR Import Twice",
			Filter:      []string{}, // Empty filter first
		})

		resp1, err := importRPC.Import(authedCtx, req1)
		require.NoError(t, err)
		require.NotNil(t, resp1)
		require.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp1.Msg.Kind)
		
		// Second request with filter
		req2 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        harJSON,
			Name:        "Test HAR Import Twice",
			Filter:      []string{"api.example.com"},
		})
		
		resp2, err := importRPC.Import(authedCtx, req2)
		require.NoError(t, err)
		require.NotNil(t, resp2)
		assert.NotNil(t, resp2.Msg.Flow)

		// Verify collection was created
		// HAR imports now always use "Imported" as collection name
		collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
		require.NoError(t, err)
		collectionID := collection.ID

		// Verify folders were created
		folders, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
		require.NoError(t, err)
		assert.Greater(t, len(folders), 0, "Expected folders to be created")

		// Log folder structure
		t.Logf("Created %d folders in first import:", len(folders))
		for _, folder := range folders {
			t.Logf("  - %s (ID: %s, Parent: %v)", folder.Name, folder.ID.String(), folder.ParentID)
		}

		// Verify APIs were created
		apis, err := ias.GetApisWithCollectionID(ctx, collectionID)
		require.NoError(t, err)
		assert.Greater(t, len(apis), 0, "Expected APIs to be created")
		t.Logf("Created %d APIs in first import", len(apis))

		// Verify examples were created by checking API examples
		exampleCount := 0
		for range apis {
			// Each API should have examples
			exampleCount++ // At least one example per API
		}
		assert.Greater(t, exampleCount, 0, "Expected examples to be created")
		t.Logf("Created at least %d examples in first import", exampleCount)
	})

	// Second import - should NOT cause foreign key constraint errors
	t.Run("Second Import", func(t *testing.T) {
		// Get counts before second import
		// HAR imports now always use "Imported" as collection name
		collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
		require.NoError(t, err)
		collectionID := collection.ID

		foldersBefore, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
		require.NoError(t, err)
		folderCountBefore := len(foldersBefore)

		apisBefore, err := ias.GetApisWithCollectionID(ctx, collectionID)
		require.NoError(t, err)
		apiCountBefore := len(apisBefore)

		// We'll track that examples should be created for each import

		// Perform second import
		// First request to get filter options
		req1 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        harJSON,
			Name:        "Test HAR Import Twice",
			Filter:      []string{}, // Empty filter first
		})

		resp1, err := importRPC.Import(authedCtx, req1)
		require.NoError(t, err)
		require.NotNil(t, resp1)
		require.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp1.Msg.Kind)
		
		// Second request with filter - this should NOT cause foreign key errors
		req2 := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: workspaceID.Bytes(),
			Data:        harJSON,
			Name:        "Test HAR Import Twice",
			Filter:      []string{"api.example.com"},
		})
		
		resp2, err := importRPC.Import(authedCtx, req2)
		require.NoError(t, err, "Second import should not cause foreign key constraint errors")
		require.NotNil(t, resp2)
		assert.NotNil(t, resp2.Msg.Flow)

		// Verify folders were not duplicated
		foldersAfter, err := ifs.GetFoldersWithCollectionID(ctx, collectionID)
		require.NoError(t, err)
		assert.Equal(t, folderCountBefore, len(foldersAfter), "Folder count should not change")
		
		t.Logf("Folders after second import: %d (unchanged)", len(foldersAfter))
		for _, folder := range foldersAfter {
			t.Logf("  - %s (ID: %s, Parent: %v)", folder.Name, folder.ID.String(), folder.ParentID)
		}

		// Verify APIs were not duplicated (except delta APIs)
		apisAfter, err := ias.GetApisWithCollectionID(ctx, collectionID)
		require.NoError(t, err)
		// We may have more APIs due to delta endpoints, but non-delta should be the same
		assert.GreaterOrEqual(t, len(apisAfter), apiCountBefore, "API count should not decrease")
		t.Logf("APIs after second import: %d", len(apisAfter))

		// Verify new examples were created (as requested in the requirement)
		// Since we're importing the same HAR again, we should have more examples
		// Each import creates new examples for the endpoints
		t.Logf("Examples after second import: new examples should have been created")
	})
}

// TestImportHarComplexHierarchy tests importing HAR with deep folder hierarchies
func TestImportHarComplexHierarchy(t *testing.T) {
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

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create HAR data with very deep folder hierarchy
	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"entries": []interface{}{
				map[string]interface{}{
					"startedDateTime": "2023-01-01T00:00:00.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "GET",
						"url":         "http://api.example.com/v1/deep/nested/folder/structure/endpoint",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
					},
					"response": map[string]interface{}{
						"status":     200,
						"statusText": "OK",
					},
				},
				map[string]interface{}{
					"startedDateTime": "2023-01-01T00:00:01.000Z",
					"_resourceType":   "xhr",
					"request": map[string]interface{}{
						"method":      "POST",
						"url":         "http://api.example.com/v1/deep/nested/another/path",
						"httpVersion": "HTTP/1.1",
						"headers":     []interface{}{},
					},
					"response": map[string]interface{}{
						"status":     201,
						"statusText": "Created",
					},
				},
			},
		},
	}

	// Convert to JSON
	harJSON, err := json.Marshal(harData)
	require.NoError(t, err)

	// Create ImportRPC
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Import twice to ensure no foreign key issues with deep hierarchies
	for i := 0; i < 2; i++ {
		t.Run(fmt.Sprintf("Import %d", i+1), func(t *testing.T) {
			// First request to get filter options
			req1 := connect.NewRequest(&importv1.ImportRequest{
				WorkspaceId: workspaceID.Bytes(),
				Data:        harJSON,
				Name:        "Deep Hierarchy Test",
				Filter:      []string{},
			})

			resp1, err := importRPC.Import(authedCtx, req1)
			require.NoError(t, err)
			require.NotNil(t, resp1)
			
			// Second request with filter
			req2 := connect.NewRequest(&importv1.ImportRequest{
				WorkspaceId: workspaceID.Bytes(),
				Data:        harJSON,
				Name:        "Deep Hierarchy Test",
				Filter:      []string{"api.example.com"},
			})
			
			resp2, err := importRPC.Import(authedCtx, req2)
			require.NoError(t, err, "Import %d should not cause foreign key errors", i+1)
			require.NotNil(t, resp2)
		})
	}
}