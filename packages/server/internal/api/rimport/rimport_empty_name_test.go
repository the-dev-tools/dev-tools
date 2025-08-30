package rimport_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mcollection"
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

func TestImportCurl_EmptyName_GeneratesFromURL(t *testing.T) {
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

	// Create ImportRPC with actual services
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, as)

	tests := []struct {
		name            string
		curlStr         string
		reqName         string
		expectedPattern string
	}{
		{
			name:            "Empty name generates from httpbin.org",
			curlStr:         `curl 'https://httpbin.org/get'`,
			reqName:         "",
			expectedPattern: "Httpbin.Org API",
		},
		{
			name:            "Whitespace name generates from github API",
			curlStr:         `curl 'https://api.github.com/user' -H 'Authorization: token'`,
			reqName:         "   \t  ",
			expectedPattern: "Api.Github.Com API",
		},
		{
			name:            "Empty name with www prefix removes www",
			curlStr:         `curl 'https://www.google.com/search'`,
			reqName:         "",
			expectedPattern: "Google.Com API",
		},
		{
			name:            "Empty name with localhost",
			curlStr:         `curl 'http://localhost:3000/api'`,
			reqName:         "",
			expectedPattern: "Localhost API",
		},
		{
			name:            "Valid name is preserved",
			curlStr:         `curl 'https://jsonplaceholder.typicode.com/posts'`,
			reqName:         "My Custom Collection",
			expectedPattern: "My Custom Collection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := connect.NewRequest(&importv1.ImportRequest{
				WorkspaceId: workspaceID.Bytes(),
				TextData:    tt.curlStr,
				Name:        tt.reqName,
			})

			// Call Import method with authenticated context
			authedCtx := mwauth.CreateAuthedContext(ctx, userID)
			resp, err := importRPC.Import(authedCtx, req)

			// Assertions
			require.NoError(t, err)
			assert.NotNil(t, resp)

			// Verify collections were created with expected names
			collections, err := cs.GetCollectionsOrdered(ctx, workspaceID)
			require.NoError(t, err)

			// Find the collection with the expected name
			var foundCollection bool
			for _, col := range collections {
				if col.Name == tt.expectedPattern {
					foundCollection = true
					break
				}
			}

			// Verify the expected collection was found
			assert.True(t, foundCollection, "Expected collection with name '%s' not found. Available collections: %s", 
				tt.expectedPattern, getCollectionNames(collections))

			// Ensure no collection has an empty name
			for _, col := range collections {
				assert.NotEmpty(t, strings.TrimSpace(col.Name), "Collection should never have empty name: %+v", col)
			}
		})
	}
}

// Helper function to get collection names for better error messages
func getCollectionNames(collections []mcollection.Collection) string {
	names := make([]string, len(collections))
	for i, col := range collections {
		names[i] = "'" + col.Name + "'"
	}
	return strings.Join(names, ", ")
}