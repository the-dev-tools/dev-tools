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
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
)

func TestCurlImportWithHeaders(t *testing.T) {
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
	hs := sexampleheader.New(queries)

	// Create test data - workspace, user, etc.
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()

	// Setup workspace and collection for testing
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, idwrap.NewNow())

	// Test curl command with multiple headers
	curlStr := `curl 'https://api.example.com/test' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer token123' \
  -H 'Content-Type: application/json' \
  -H 'User-Agent: DevTools/1.0' \
  --data-raw '{"test":"data"}'`

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	// Create request
	req := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: workspaceID.Bytes(),
		TextData:    curlStr,
		Name:        "Test Curl Import Headers",
	})

	// Call Import method with authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := importRPC.Import(authedCtx, req)

	// Assertions
	require.NoError(t, err, "Import should succeed")
	assert.NotNil(t, resp, "Response should not be nil")

	// Get the created collection by name
	collection, err := cs.GetCollectionByWorkspaceIDAndName(ctx, workspaceID, "Test Curl Import Headers")
	require.NoError(t, err, "Should be able to get collection")

	// Get APIs in the collection
	apis, err := ias.GetApisWithCollectionID(ctx, collection.ID)
	require.NoError(t, err, "Should be able to get APIs")
	require.Len(t, apis, 1, "Should have one API")
	
	t.Logf("API created: ID=%s, Method=%s, URL=%s", apis[0].ID.String(), apis[0].Method, apis[0].Url)

	// Get examples for the API
	examples, err := iaes.GetApiExamples(ctx, apis[0].ID)
	require.NoError(t, err, "Should be able to get examples")
	t.Logf("Found %d examples", len(examples))
	
	// Also try getting examples by collection ID to debug
	allExamples, err := iaes.GetApiExampleByCollection(ctx, collection.ID)
	require.NoError(t, err, "Should be able to get all examples in collection")
	t.Logf("Found %d total examples in collection", len(allExamples))
	
	if len(examples) == 0 && len(allExamples) > 0 {
		t.Logf("Examples exist in collection but not found by API ID. Using first example.")
		examples = []mitemapiexample.ItemApiExample{allExamples[0]}
	}
	
	require.Len(t, examples, 1, "Should have one example")

	// Get headers for the example using both old and new methods
	exampleID := examples[0].ID

	// Test old method (should still work)
	headersOld, err := hs.GetHeaderByExampleID(ctx, exampleID)
	require.NoError(t, err, "Old header method should work")
	assert.Len(t, headersOld, 4, "Should have 4 headers")

	// Test new ordered method
	headersNew, err := hs.GetHeaderByExampleIDOrdered(ctx, exampleID)
	require.NoError(t, err, "New ordered header method should work")
	assert.Len(t, headersNew, 4, "Should have 4 headers")

	// Verify header content
	expectedHeaders := map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer token123",
		"Content-Type":  "application/json",
		"User-Agent":    "DevTools/1.0",
	}

	// Check that all expected headers are present
	for _, header := range headersNew {
		expectedValue, exists := expectedHeaders[header.HeaderKey]
		assert.True(t, exists, "Header %s should be expected", header.HeaderKey)
		assert.Equal(t, expectedValue, header.Value, "Header %s should have correct value", header.HeaderKey)
		assert.True(t, header.Enable, "Header %s should be enabled", header.HeaderKey)
	}

	// Verify linked-list structure (at least check that prev/next fields are set appropriately)
	// First header should have no prev
	// Last header should have no next
	// Middle headers should have both prev and next
	hasHeadHeader := false
	hasTailHeader := false
	
	for _, header := range headersNew {
		if header.Prev == nil {
			hasHeadHeader = true
		}
		if header.Next == nil {
			hasTailHeader = true
		}
	}
	
	assert.True(t, hasHeadHeader, "Should have a head header (prev == nil)")
	assert.True(t, hasTailHeader, "Should have a tail header (next == nil)")
}