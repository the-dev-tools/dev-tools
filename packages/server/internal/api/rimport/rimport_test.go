package rimport_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	changev1 "the-dev-tools/spec/dist/buf/go/change/v1"
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
	importRPC := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers)

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
	assert.NotEmpty(t, resp.Msg.Changes)
	assert.Equal(t, 1, len(resp.Msg.Changes))

	// Verify change is of expected type
	change := resp.Msg.Changes[0]
	assert.NotNil(t, change)
	assert.Equal(t, changev1.ChangeKind_CHANGE_KIND_UNSPECIFIED, *change.Kind)
	assert.NotEmpty(t, change.List)
}
