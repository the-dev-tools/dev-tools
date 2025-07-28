package rimport_test

import (
	"context"
	"testing"
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

	"connectrpc.com/connect"
)

func TestSimplifiedYAMLImport(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB
	defer base.Close()

	// Create services
	ws := sworkspace.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)
	eas := sassert.New(queries)

	// Create test workspace
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsuserID, userID, baseCollectionID)

	// Create import service
	importService := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, eas)

	// Test 1: Basic YAML import
	t.Run("BasicImport", func(t *testing.T) {
		yamlData := []byte(`
workspace_name: Test Workspace
flows:
  - name: Simple Flow
    steps:
      - request:
          name: Get Users
          url: https://api.example.com/users
          method: GET
`)

		importReq := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        "Test Import",
			Data:        yamlData,
		})

		authedCtx := mwauth.CreateAuthedContext(ctx, userID)
		importResp, err := importService.Import(authedCtx, importReq)
		testutil.AssertFatal(t, nil, err)
		testutil.AssertNotFatal(t, nil, importResp.Msg)

		if importResp.Msg.Flow == nil {
			t.Fatal("No flow was imported")
		}
	})

	// Test 2: Import with variables
	t.Run("ImportWithVariables", func(t *testing.T) {
		yamlData := []byte(`
workspace_name: Variable Workspace
flows:
  - name: Variable Flow
    variables:
      - name: BASE_URL
        value: https://api.example.com
      - name: VERSION
        value: v2
    steps:
      - request:
          name: Get Resource
          url: "{{BASE_URL}}/{{VERSION}}/resource"
          method: GET
`)

		importReq := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        "Variable Import",
			Data:        yamlData,
		})

		authedCtx := mwauth.CreateAuthedContext(ctx, userID)
		importResp, err := importService.Import(authedCtx, importReq)
		testutil.AssertFatal(t, nil, err)
		testutil.AssertNotFatal(t, nil, importResp.Msg)

		if importResp.Msg.Flow == nil {
			t.Fatal("No flow was imported")
		}
	})

	// Test 3: Import with headers and query params
	t.Run("ImportWithHeadersAndQuery", func(t *testing.T) {
		yamlData := []byte(`
workspace_name: Headers Workspace
flows:
  - name: Headers Flow
    steps:
      - request:
          name: API Call
          url: https://api.example.com/data
          method: POST
          headers:
            - name: Authorization
              value: Bearer token123
            - name: Content-Type
              value: application/json
          query_params:
            - name: page
              value: "1"
            - name: limit
              value: "10"
`)

		importReq := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        "Headers Import",
			Data:        yamlData,
		})

		authedCtx := mwauth.CreateAuthedContext(ctx, userID)
		importResp, err := importService.Import(authedCtx, importReq)
		testutil.AssertFatal(t, nil, err)
		testutil.AssertNotFatal(t, nil, importResp.Msg)

		if importResp.Msg.Flow == nil {
			t.Fatal("No flow was imported")
		}
	})

	// Test 4: Import with control flow
	t.Run("ImportWithControlFlow", func(t *testing.T) {
		yamlData := []byte(`
workspace_name: Control Flow Workspace
flows:
  - name: Control Flow
    steps:
      - request:
          name: Initial Request
          url: https://api.example.com/check
          method: GET
      - if:
          name: Check Status
          condition: response.status == 200
          then: success_step
          else: error_step
      - request:
          name: success_step
          url: https://api.example.com/success
          method: POST
          depends_on: [Check Status]
      - request:
          name: error_step
          url: https://api.example.com/error
          method: POST
          depends_on: [Check Status]
`)

		importReq := connect.NewRequest(&importv1.ImportRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        "Control Flow Import",
			Data:        yamlData,
		})

		authedCtx := mwauth.CreateAuthedContext(ctx, userID)
		importResp, err := importService.Import(authedCtx, importReq)
		testutil.AssertFatal(t, nil, err)
		testutil.AssertNotFatal(t, nil, importResp.Msg)

		if importResp.Msg.Flow == nil {
			t.Fatal("No flow was imported")
		}
	})
}
