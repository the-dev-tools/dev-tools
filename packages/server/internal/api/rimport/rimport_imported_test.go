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

func TestImportHarUsesImportedCollection(t *testing.T) {
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
	res := sexampleresp.New(queries)
	as := sassert.New(queries)

	// Create ImportRPC instance
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, res, as)

	// Create test IDs
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create HAR data with simple URLs
	harData := `{
		"log": {
			"version": "1.2",
			"creator": {
				"name": "Test",
				"version": "1.0"
			},
			"entries": [
				{
					"startedDateTime": "2024-01-01T00:00:00.000Z",
					"time": 100,
					"request": {
						"method": "GET",
						"url": "https://example.com/api/users",
						"httpVersion": "HTTP/1.1",
						"cookies": [],
						"headers": [
							{
								"name": "User-Agent",
								"value": "Test/1.0"
							},
							{
								"name": "X-Requested-With",
								"value": "XMLHttpRequest"
							}
						],
						"queryString": [],
						"headersSize": 100,
						"bodySize": 0
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"cookies": [],
						"headers": [
							{
								"name": "Content-Type",
								"value": "application/json"
							}
						],
						"content": {
							"size": 50,
							"mimeType": "application/json",
							"text": "{\"users\":[]}"
						},
						"redirectURL": "",
						"headersSize": 100,
						"bodySize": 50
					},
					"cache": {},
					"timings": {
						"blocked": 0,
						"dns": 0,
						"connect": 0,
						"send": 0,
						"wait": 100,
						"receive": 0,
						"ssl": 0
					}
				}
			]
		}
	}`

	harJSON := []byte(harData)

	// First request to get filters
	req1 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        harJSON,
		Name:        "My Custom HAR Import Name", // This should be ignored for HAR imports
		Filter:      []string{},                  // Empty filter first
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
		Name:        "My Custom HAR Import Name", // This should be ignored for HAR imports
		Filter:      []string{"example.com"},
	})

	resp, err := importRPC.Import(authedCtx, req2)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify the collection was created with "Imported" name, not the custom name
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	require.NoError(t, err)
	assert.Equal(t, "Imported", collection.Name)

	// Verify that no collection was created with the custom name
	_, err = cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "My Custom HAR Import Name")
	assert.Error(t, err)
	assert.Equal(t, scollection.ErrNoCollectionFound, err)

	// Verify APIs were created in the "Imported" collection
	apis, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, len(apis), "Expected 1 API to be created")

	// Import another HAR file with a different name - should still go to "Imported"
	harData2 := `{
		"log": {
			"version": "1.2",
			"creator": {
				"name": "Test",
				"version": "1.0"
			},
			"entries": [
				{
					"startedDateTime": "2024-01-01T00:00:00.000Z",
					"time": 100,
					"request": {
						"method": "POST",
						"url": "https://example.com/api/posts",
						"httpVersion": "HTTP/1.1",
						"cookies": [],
						"headers": [
							{
								"name": "X-Requested-With",
								"value": "XMLHttpRequest"
							}
						],
						"queryString": [],
						"headersSize": 100,
						"bodySize": 0
					},
					"response": {
						"status": 201,
						"statusText": "Created",
						"httpVersion": "HTTP/1.1",
						"cookies": [],
						"headers": [],
						"content": {
							"size": 0,
							"mimeType": "text/plain"
						},
						"redirectURL": "",
						"headersSize": 100,
						"bodySize": 0
					},
					"cache": {},
					"timings": {
						"blocked": 0,
						"dns": 0,
						"connect": 0,
						"send": 0,
						"wait": 100,
						"receive": 0,
						"ssl": 0
					}
				}
			]
		}
	}`

	// Import with a different name
	req3 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(harData2),
		Name:        "Another HAR Import",
		Filter:      []string{},
	})

	resp3, err := importRPC.Import(authedCtx, req3)
	require.NoError(t, err)
	require.Equal(t, importv1.ImportKind_IMPORT_KIND_FILTER, resp3.Msg.Kind)

	req4 := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		Data:        []byte(harData2),
		Name:        "Another HAR Import",
		Filter:      []string{"example.com"},
	})

	resp4, err := importRPC.Import(authedCtx, req4)
	require.NoError(t, err)
	assert.NotNil(t, resp4)

	// Verify both imports went to the same "Imported" collection
	apis2, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(apis2), "Expected 2 APIs total in the Imported collection")

	// Verify no collection was created with the second name either
	_, err = cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Another HAR Import")
	assert.Error(t, err)
	assert.Equal(t, scollection.ErrNoCollectionFound, err)
}

func TestImportCurlStillUsesProvidedName(t *testing.T) {
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
	res := sexampleresp.New(queries)
	as := sassert.New(queries)

	// Create ImportRPC instance
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, res, as)

	// Create test IDs
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Create curl import request
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    "curl https://api.example.com/users -H 'Authorization: Bearer token'",
		Name:        "My Curl Collection",
		Filter:      []string{},
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify the collection was created with the provided name, NOT "Imported"
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "My Curl Collection")
	require.NoError(t, err)
	assert.Equal(t, "My Curl Collection", collection.Name)

	// Verify that no "Imported" collection was created
	_, err = cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Imported")
	assert.Error(t, err)
	assert.Equal(t, scollection.ErrNoCollectionFound, err)
}
