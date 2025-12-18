package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"the-dev-tools/server/pkg/service/sflow"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rimportv2"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/shttp"

	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/testutil"
	importv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

// HARImportE2ETestSuite represents the complete e2e test suite for HAR import
type HARImportE2ETestSuite struct {
	t             *testing.T
	ctx           context.Context
	db            *sql.DB
	queries       *gen.Queries
	services      testutil.BaseTestServices
	importHandler *rimportv2.ImportV2RPC
	httpHandler   *rhttp.HttpServiceRPC // Optional, can be nil for import testing
	workspaceID   idwrap.IDWrap
	userID        idwrap.IDWrap
	baseDB        *testutil.BaseDBQueries
}

// setupHARImportE2ETest creates the complete test infrastructure
func setupHARImportE2ETest(t *testing.T) *HARImportE2ETestSuite {
	ctx := context.Background()

	// Create real database setup
	baseDB := testutil.CreateBaseDB(ctx, t)

	services := baseDB.GetBaseServices()

	// Create additional services needed for import RPC
	mockLogger := mocklogger.NewMockLogger()

	// Create HTTP service first
	httpService := shttp.New(baseDB.Queries, mockLogger)
	flowService := sflow.NewFlowService(baseDB.Queries)
	fileService := sfile.New(baseDB.Queries, mockLogger)

	// Create child entity services
	httpHeaderService := shttp.NewHttpHeaderService(baseDB.Queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(baseDB.Queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(baseDB.Queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(baseDB.Queries)
	bodyService := shttp.NewHttpBodyRawService(baseDB.Queries)
	httpAssertService := shttp.NewHttpAssertService(baseDB.Queries)

	// Create node services
	nodeService := sflow.NewNodeService(baseDB.Queries)
	nodeRequestService := sflow.NewNodeRequestService(baseDB.Queries)
	nodeNoopService := sflow.NewNodeNoopService(baseDB.Queries)
	edgeService := sflow.NewEdgeService(baseDB.Queries)

	// Create environment and variable services
	envService := senv.NewEnvironmentService(baseDB.Queries, mockLogger)
	varService := senv.NewVariableService(baseDB.Queries, mockLogger)

	// Create streamers using the same approach as integration tests
	flowStream := memory.NewInMemorySyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]()
	nodeStream := memory.NewInMemorySyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]()
	edgeStream := memory.NewInMemorySyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]()
	noopStream := memory.NewInMemorySyncStreamer[rflowv2.NoOpTopic, rflowv2.NoOpEvent]()
	stream := memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]()
	httpHeaderStream := memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]()
	httpSearchParamStream := memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]()
	httpBodyFormStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]()
	httpBodyUrlEncodedStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]()
	httpBodyRawStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]()
	httpAssertStream := memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]()
	fileStream := memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent]()

	// Create import handler
	importHandler := rimportv2.NewImportV2RPC(
		baseDB.DB,
		mockLogger,
		rimportv2.ImportServices{
			Workspace:          services.Ws,
			User:               services.Us,
			Http:               &httpService,
			Flow:               &flowService,
			File:               fileService,
			Env:                envService,
			Var:                varService,
			HttpHeader:         httpHeaderService,
			HttpSearchParam:    httpSearchParamService,
			HttpBodyForm:       httpBodyFormService,
			HttpBodyUrlEncoded: httpBodyUrlEncodedService,
			HttpBodyRaw:        bodyService,
			HttpAssert:         httpAssertService,
			Node:               &nodeService,
			NodeRequest:        &nodeRequestService,
			NodeNoop:           &nodeNoopService,
			Edge:               &edgeService,
		},
		rimportv2.ImportStreamers{
			Flow:               flowStream,
			Node:               nodeStream,
			Edge:               edgeStream,
			Noop:               noopStream,
			Http:               stream,
			HttpHeader:         httpHeaderStream,
			HttpSearchParam:    httpSearchParamStream,
			HttpBodyForm:       httpBodyFormStream,
			HttpBodyUrlEncoded: httpBodyUrlEncodedStream,
			HttpBodyRaw:        httpBodyRawStream,
			HttpAssert:         httpAssertStream,
			File:               fileStream,
		},
	)

	// Create resolver for delta resolution
	requestResolver := resolver.NewStandardResolver(
		&httpService,
		&httpHeaderService,
		httpSearchParamService,
		bodyService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
	)

	// Create HTTP handler
	httpHandler := rhttp.New(
		baseDB.DB,
		httpService.Reader(),
		httpService,
		services.Us,
		services.Ws,
		services.Wus,
		envService,
		varService,
		bodyService,
		httpHeaderService,
		httpSearchParamService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
		shttp.NewHttpResponseService(baseDB.Queries),
		requestResolver,
		&rhttp.HttpStreamers{
			Http:               stream,
			HttpHeader:         httpHeaderStream,
			HttpSearchParam:    httpSearchParamStream,
			HttpBodyForm:       httpBodyFormStream,
			HttpBodyUrlEncoded: httpBodyUrlEncodedStream,
			HttpAssert:         httpAssertStream,
			HttpVersion:        memory.NewInMemorySyncStreamer[rhttp.HttpVersionTopic, rhttp.HttpVersionEvent](),
			HttpResponse:       memory.NewInMemorySyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent](),
			HttpResponseHeader: memory.NewInMemorySyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent](),
			HttpResponseAssert: memory.NewInMemorySyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent](),
			HttpBodyRaw:        httpBodyRawStream,
			Log:                memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent](),
		},
	)

	// We'll call RPC methods directly instead of using Connect clients
	// This matches the approach used in the integration tests

	// Create test user first
	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	if err := services.Us.CreateUser(ctx, &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create authenticated context - this is the key fix!
	authCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Create test workspace
	workspaceID := idwrap.NewNow()

	// Create workspace in database using authenticated context
	err := services.Ws.Create(authCtx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "HAR Import Test Workspace",
	})
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Create workspace-user relationship using authenticated context
	err = services.Wus.CreateWorkspaceUser(authCtx, &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	})
	if err != nil {
		t.Fatalf("Failed to create workspace-user relationship: %v", err)
	}

	suite := &HARImportE2ETestSuite{
		t:             t,
		ctx:           authCtx, // Use authenticated context!
		db:            baseDB.DB,
		queries:       baseDB.Queries,
		services:      services,
		importHandler: importHandler,
		httpHandler:   &httpHandler,
		workspaceID:   workspaceID,
		userID:        userID,
		baseDB:        baseDB,
	}

	// Register cleanup function
	t.Cleanup(suite.Cleanup)

	return suite
}

// Cleanup properly closes database connections
func (suite *HARImportE2ETestSuite) Cleanup() {
	if suite.baseDB != nil {
		suite.baseDB.Close()
	}
}

// TestHARImportE2E_Comprehensive suite tests the complete HAR import pipeline
func TestHARImportE2E_Comprehensive(t *testing.T) {
	if _, err := os.Stat("uuidhar.har"); os.IsNotExist(err) {
		t.Skip("uuidhar.har not found, skipping comprehensive E2E test")
	}

	// Note: t.Parallel() disabled to prevent database deadlocks between tests
	// Each test creates its own workspace and database to ensure isolation

	suite := setupHARImportE2ETest(t)

	t.Run("scenario_1_clean_import", func(t *testing.T) {
		// Note: t.Parallel() disabled to prevent database deadlocks
		suite.testCleanImport()
	})

	t.Run("scenario_2_duplicate_import_detection", func(t *testing.T) {
		// Note: t.Parallel() disabled to prevent database deadlocks
		suite.testDuplicateImportDetection()
	})

	t.Run("scenario_3_delta_child_entities", func(t *testing.T) {
		// Note: t.Parallel() disabled to prevent database deadlocks
		suite.testDeltaChildEntities()
	})

	t.Run("scenario_4_delta_header_structure", func(t *testing.T) {
		// Note: t.Parallel() disabled to prevent database deadlocks
		suite.testDeltaHeaderStructure()
	})

	t.Run("scenario_5_rpc_consistency", func(t *testing.T) {
		// Note: t.Parallel() disabled because it relies on the same suite with existing data?
		// Actually suite is recreated for each test in `TestHARImportE2E_Comprehensive` logic?
		// No, `suite` is created once outside the subtests loop (wait, line 213: `suite := setupHARImportE2ETest(t)`).
		// So `suite` is shared?
		// If suite is shared, parallel is dangerous.
		// However, `setupHARImportE2ETest` creates a NEW DB connection.
		// But `TestHARImportE2E_Comprehensive` calls it ONCE at line 213.
		// So all subtests share the SAME DB.
		// Thus `t.Parallel()` is indeed dangerous if they mutate state.

		// This test is read-only validation of previous state, or new import?
		// Better to run it sequentially.
		suite.testRPCConsistency()
	})
}

// testCleanImport tests the initial import of HAR data
func (suite *HARImportE2ETestSuite) testCleanImport() {
	t := suite.t

	// Load the HAR file
	harPath := "uuidhar.har"
	harData, err := os.ReadFile(harPath)
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	t.Logf("📁 Loaded HAR file: %s (size: %d bytes)", harPath, len(harData))

	// Get initial state
	initialHTTPCount := suite.countHTTPEntries()
	initialDeltaCount := suite.countDeltaEntries()
	initialHeaderCount := suite.countHeaderEntries()
	initialParamCount := suite.countParamEntries()

	t.Logf("📊 Initial state - HTTP: %d, Deltas: %d, Headers: %d, Params: %d",
		initialHTTPCount, initialDeltaCount, initialHeaderCount, initialParamCount)

	// Perform import with domain data to prevent early return
	importReq := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "Clean HAR Import Test",
		Data:        harData,
		DomainData: []*importv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "ecommerce-admin-panel.fly.dev",
				Variable: "baseUrl",
			},
		},
	}

	startTime := time.Now()
	importResp, err := suite.importHandler.Import(suite.ctx, connect.NewRequest(importReq))
	importDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("❌ Import failed: %v", err)
	}

	t.Logf("✅ Import completed in %v", importDuration)
	t.Logf("📦 Import response - FlowID: %s, MissingData: %v, Domains: %d",
		importResp.Msg.FlowId, importResp.Msg.MissingData, len(importResp.Msg.Domains))

	// Verify state after import
	postHTTPCount := suite.countHTTPEntries()
	postDeltaCount := suite.countDeltaEntries()
	postHeaderCount := suite.countHeaderEntries()
	postParamCount := suite.countParamEntries()

	t.Logf("📊 Post-import state - HTTP: %d (+%d), Deltas: %d (+%d), Headers: %d (+%d), Params: %d (+%d)",
		postHTTPCount, postHTTPCount-initialHTTPCount,
		postDeltaCount, postDeltaCount-initialDeltaCount,
		postHeaderCount, postHeaderCount-initialHeaderCount,
		postParamCount, postParamCount-initialParamCount)

	// Verify HAR file entries count matches our expectations
	var harFileData map[string]interface{}
	err = json.Unmarshal(harData, &harFileData)
	if err != nil {
		t.Fatalf("Failed to parse HAR JSON: %v", err)
	}

	log, ok := harFileData["log"].(map[string]interface{})
	if !ok {
		t.Fatalf("Invalid HAR format: missing log")
	}

	entries, ok := log["entries"].([]interface{})
	if !ok {
		t.Fatalf("Invalid HAR format: missing entries")
	}

	expectedRequests := len(entries)
	t.Logf("🎯 HAR file contains %d HTTP entries", expectedRequests)

	// Verify we have the expected number of base and delta entries
	// Each HAR entry should create: 1 base HTTP + 1 delta HTTP + their child entities
	expectedBaseEntries := expectedRequests
	expectedDeltaEntries := expectedRequests

	if postHTTPCount-initialHTTPCount != expectedBaseEntries {
		t.Errorf("❌ Expected %d base HTTP entries, got %d",
			expectedBaseEntries, postHTTPCount-initialHTTPCount)
	}

	if postDeltaCount-initialDeltaCount != expectedDeltaEntries {
		t.Errorf("❌ Expected %d delta HTTP entries, got %d",
			expectedDeltaEntries, postDeltaCount-initialDeltaCount)
	}

	// Verify child entities were created
	if postHeaderCount-initialHeaderCount == 0 {
		t.Errorf("❌ Expected headers to be created, but count remained the same")
	}

	// Log detailed database state
	suite.logDetailedDatabaseState("After Clean Import")
}

// testDuplicateImportDetection tests that importing the same HAR creates deltas, not duplicates
func (suite *HARImportE2ETestSuite) testDuplicateImportDetection() {
	t := suite.t

	// First, perform initial import
	harPath := "uuidhar.har"
	harData, err := os.ReadFile(harPath)
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	// Initial import
	importReq1 := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "Duplicate Test - First Import",
		Data:        harData,
		DomainData: []*importv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "ecommerce-admin-panel.fly.dev",
				Variable: "baseUrl",
			},
		},
	}

	_, err = suite.importHandler.Import(suite.ctx, connect.NewRequest(importReq1))
	if err != nil {
		t.Fatalf("❌ First import failed: %v", err)
	}

	// Get state after first import
	firstImportHTTPCount := suite.countHTTPEntries()
	firstImportDeltaCount := suite.countDeltaEntries()
	firstImportHeaderCount := suite.countHeaderEntries()

	t.Logf("📊 After first import - HTTP: %d, Deltas: %d, Headers: %d",
		firstImportHTTPCount, firstImportDeltaCount, firstImportHeaderCount)

	// Wait a bit to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Second import of the same HAR
	importReq2 := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "Duplicate Test - Second Import",
		Data:        harData,
		DomainData: []*importv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "ecommerce-admin-panel.fly.dev",
				Variable: "baseUrl",
			},
		},
	}

	startTime := time.Now()
	secondImportResp, err := suite.importHandler.Import(suite.ctx, connect.NewRequest(importReq2))
	secondImportDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("❌ Second import failed: %v", err)
	}

	t.Logf("✅ Second import completed in %v", secondImportDuration)
	t.Logf("📦 Second import response - FlowID: %s, MissingData: %v",
		secondImportResp.Msg.FlowId, secondImportResp.Msg.MissingData)

	// Get state after second import
	secondImportHTTPCount := suite.countHTTPEntries()
	secondImportDeltaCount := suite.countDeltaEntries()
	secondImportHeaderCount := suite.countHeaderEntries()

	t.Logf("📊 After second import - HTTP: %d (+%d), Deltas: %d (+%d), Headers: %d (+%d)",
		secondImportHTTPCount, secondImportHTTPCount-firstImportHTTPCount,
		secondImportDeltaCount, secondImportDeltaCount-firstImportDeltaCount,
		secondImportHeaderCount, secondImportHeaderCount-firstImportHeaderCount)

	// The key test: we should get additional deltas, NOT duplicate base entries
	newHTTPEntries := secondImportHTTPCount - firstImportHTTPCount
	newDeltaEntries := secondImportDeltaCount - firstImportDeltaCount
	newHeaderEntries := secondImportHeaderCount - firstImportHeaderCount

	// We expect 0 new base HTTP entries (overwrite detection should prevent duplicates)
	// We expect some new delta entries (representing changes from the second import)
	// We expect some new header entries (delta child entities)

	if newHTTPEntries != 0 {
		t.Errorf("❌ Expected 0 new base HTTP entries (overwrite detection), got %d", newHTTPEntries)
	} else {
		t.Logf("✅ Overwrite detection working: no duplicate base entries created")
	}

	if newDeltaEntries == 0 {
		t.Logf("⚠️  No new delta entries created - this might indicate overwrite detection is too aggressive")
	} else {
		t.Logf("✅ Delta creation working: %d new delta entries created", newDeltaEntries)
	}

	if newHeaderEntries == 0 {
		t.Logf("⚠️  No new header entries created - delta child entities might be missing")
	} else {
		t.Logf("✅ Delta child entities working: %d new header entries created", newHeaderEntries)
	}

	// Log detailed analysis
	suite.analyzeDuplicateImportBehavior()
}

// testDeltaHeaderStructure verifies that delta headers are created with the correct ID structure
func (suite *HARImportE2ETestSuite) testDeltaHeaderStructure() {
	t := suite.t

	// Import HAR
	harPath := "uuidhar.har"
	harData, err := os.ReadFile(harPath)
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	importReq := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "Delta Header Structure Test",
		Data:        harData,
		DomainData: []*importv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "ecommerce-admin-panel.fly.dev",
				Variable: "baseUrl",
			},
		},
	}

	_, err = suite.importHandler.Import(suite.ctx, connect.NewRequest(importReq))
	if err != nil {
		t.Fatalf("❌ Import failed: %v", err)
	}

	// Analyze HTTP structure
	t.Logf("🔍 === HTTP Structure Analysis ===")
	httpEntries, err := suite.queries.GetHTTPsByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		t.Fatalf("Failed to get HTTP entries: %v", err)
	}

	t.Logf("📊 Total HTTP entries: %d", len(httpEntries))

	baseCount := 0
	deltaCount := 0

	for _, http := range httpEntries {
		if http.IsDelta {
			deltaCount++
			t.Logf("🔄 Delta HTTP: %s %s | ID: %s | Parent: %s",
				http.Method, http.Url, http.ID,
				func() string {
					if http.ParentHttpID != nil {
						return http.ParentHttpID.String()
					}
					return "nil"
				}())
		} else {
			baseCount++
			t.Logf("📋 Base HTTP: %s %s | ID: %s", http.Method, http.Url, http.ID)
		}
	}

	t.Logf("📊 Summary: %d base entries, %d delta entries", baseCount, deltaCount)

	// Analyze header structure in detail
	t.Logf("🔍 === Header Structure Analysis ===")
	for _, http := range httpEntries {
		headers, err := suite.queries.GetHTTPHeaders(suite.ctx, http.ID)
		if err != nil {
			t.Logf("⚠️  Failed to get headers for HTTP %s: %v", http.ID, err)
			continue
		}

		if len(headers) > 0 {
			if http.IsDelta {
				t.Logf("🔄 Delta HTTP %s (%s %s) has %d headers:",
					http.ID, http.Method, http.Url, len(headers))

				// Show header structure
				for _, header := range headers {
					t.Logf("    📄 Header: %s=%s | HttpID: %s | ParentHeaderID: %s | IsDelta: %v",
						header.HeaderKey, header.HeaderValue, header.HttpID,
						func() string {
							if header.ParentHeaderID != nil {
								return header.ParentHeaderID.String()
							}
							return "nil"
						}(), header.IsDelta)

					// Show delta fields if available
					if header.IsDelta {
						t.Logf("        🔄 Delta fields - Key: %v, Value: %v, Description: %v, Enabled: %v",
							header.DeltaHeaderKey, header.DeltaHeaderValue, header.DeltaDescription, header.DeltaEnabled)
					}
				}
			} else {
				t.Logf("📋 Base HTTP %s (%s %s) has %d headers",
					http.ID, http.Method, http.Url, len(headers))
			}
		}
	}
}

// testDeltaChildEntities tests that delta child entities (headers, params, body) are created correctly
func (suite *HARImportE2ETestSuite) testDeltaChildEntities() {
	t := suite.t

	// Import HAR
	harPath := "uuidhar.har"
	harData, err := os.ReadFile(harPath)
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	importReq := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "Delta Child Entities Test",
		Data:        harData,
		DomainData: []*importv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "ecommerce-admin-panel.fly.dev",
				Variable: "baseUrl",
			},
		},
	}

	_, err = suite.importHandler.Import(suite.ctx, connect.NewRequest(importReq))
	if err != nil {
		t.Fatalf("❌ Import failed: %v", err)
	}

	// Get all HTTP entries
	httpEntries, err := suite.queries.GetHTTPsByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		t.Fatalf("Failed to get HTTP entries: %v", err)
	}

	if len(httpEntries) == 0 {
		t.Fatal("No HTTP entries found after import")
	}

	t.Logf("🔍 Analyzing %d HTTP entries for delta child entities", len(httpEntries))

	// Find base-delta pairs
	baseDeltaPairs := make(map[string][]gen.Http) // key: URL+Method, value: HTTP entries
	for _, entry := range httpEntries {
		key := fmt.Sprintf("%s:%s", entry.Method, entry.Url)
		baseDeltaPairs[key] = append(baseDeltaPairs[key], entry)
	}

	for key, entries := range baseDeltaPairs {
		t.Logf("📋 Analyzing HTTP pair: %s", key)

		var baseEntry, deltaEntry *gen.Http
		for _, entry := range entries {
			if entry.IsDelta {
				deltaEntry = &entry
			} else {
				baseEntry = &entry
			}
		}

		if baseEntry == nil {
			t.Errorf("❌ Missing base entry for %s", key)
			continue
		}

		if deltaEntry == nil {
			t.Errorf("❌ Missing delta entry for %s", key)
			continue
		}

		t.Logf("  🏷️  Base ID: %s, Delta ID: %s", baseEntry.ID, deltaEntry.ID)
		t.Logf("  🔗 Delta ParentID: %s", *deltaEntry.ParentHttpID)

		// Verify parent-child relationship
		if deltaEntry.ParentHttpID == nil {
			t.Errorf("❌ Delta entry missing ParentHttpID for %s", key)
		} else if *deltaEntry.ParentHttpID != baseEntry.ID {
			t.Errorf("❌ Delta ParentHttpID mismatch. Expected: %s, Got: %s",
				baseEntry.ID, *deltaEntry.ParentHttpID)
		} else {
			t.Logf("  ✅ Parent-child relationship correct")
		}

		// Analyze child entities
		suite.analyzeChildEntities(baseEntry.ID.String(), deltaEntry.ID.String(), key)
	}
}

// testRPCConsistency verifies that the RPC collection endpoints return data consistent with the database state
// This ensures that what is imported is correctly exposed to the frontend.
func (suite *HARImportE2ETestSuite) testRPCConsistency() {
	t := suite.t

	// Ensure we have data
	if suite.countHTTPEntries() == 0 {
		harPath := "uuidhar.har"
		harData, err := os.ReadFile(harPath)
		if err != nil {
			t.Fatalf("Failed to read HAR file: %v", err)
		}

		importReq := &importv1.ImportRequest{
			WorkspaceId: suite.workspaceID.Bytes(),
			Name:        "RPC Consistency Import",
			Data:        harData,
		}
		_, err = suite.importHandler.Import(suite.ctx, connect.NewRequest(importReq))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}
	}

	// 1. Verify HttpBodyRawDeltaCollection
	bodyRawReq := connect.NewRequest(&emptypb.Empty{})
	bodyRawResp, err := suite.httpHandler.HttpBodyRawDeltaCollection(suite.ctx, bodyRawReq)
	if err != nil {
		t.Fatalf("HttpBodyRawDeltaCollection failed: %v", err)
	}

	bodyDeltas := bodyRawResp.Msg.Items
	t.Logf("📦 HttpBodyRawDeltaCollection returned %d items", len(bodyDeltas))

	if len(bodyDeltas) == 0 {
		t.Error("❌ Expected body deltas, got 0. HAR file definitely has bodies.")
	}

	for _, d := range bodyDeltas {
		// Parity Check 1: DeltaHttpId is populated
		if len(d.DeltaHttpId) == 0 {
			t.Errorf("❌ Missing DeltaHttpId for body delta with HttpId: %x", d.HttpId)
		} else {
			// Parity Check 2: DeltaHttpId matches HttpId (since it's keyed by Delta HTTP ID in Collection)
			if string(d.DeltaHttpId) != string(d.HttpId) {
				t.Errorf("❌ DeltaHttpId (%x) does not match HttpId (%x)", d.DeltaHttpId, d.HttpId)
			}
		}

		// Check data presence
		if d.Data == nil {
			t.Errorf("❌ Missing Data for body delta %x", d.HttpId)
		}
	}

	// 2. Verify HttpHeaderDeltaCollection
	headerReq := connect.NewRequest(&emptypb.Empty{})
	headerResp, err := suite.httpHandler.HttpHeaderDeltaCollection(suite.ctx, headerReq)
	if err != nil {
		t.Fatalf("HttpHeaderDeltaCollection failed: %v", err)
	}

	headerDeltas := headerResp.Msg.Items
	t.Logf("📦 HttpHeaderDeltaCollection returned %d items", len(headerDeltas))

	// We expect headers. HAR usually has them.
	if len(headerDeltas) > 0 {
		for _, h := range headerDeltas {
			// Check IDs
			if len(h.DeltaHttpHeaderId) == 0 {
				t.Errorf("❌ Missing DeltaHttpHeaderId for header delta")
			}
		}
	} else {
		t.Log("⚠️  No header deltas found (might be expected if HAR has no header overrides?)")
	}

	t.Log("✅ RPC Consistency Checks Passed")
}

// Helper methods for database queries and analysis

func (suite *HARImportE2ETestSuite) countHTTPEntries() int {
	// Use GetHTTPsByWorkspaceID and count the results
	entries, err := suite.queries.GetHTTPsByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		suite.t.Fatalf("Failed to count HTTP entries: %v", err)
	}
	return len(entries)
}

func (suite *HARImportE2ETestSuite) countDeltaEntries() int {
	// Use GetHTTPDeltasByWorkspaceID and count the results
	entries, err := suite.queries.GetHTTPDeltasByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		suite.t.Fatalf("Failed to count delta entries: %v", err)
	}
	return len(entries)
}

func (suite *HARImportE2ETestSuite) countHeaderEntries() int {
	// Get all HTTP entries and count their headers
	httpEntries, err := suite.queries.GetHTTPsByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		suite.t.Fatalf("Failed to get HTTP entries for header count: %v", err)
	}

	totalHeaders := 0
	for _, entry := range httpEntries {
		headers, err := suite.queries.GetHTTPHeaders(suite.ctx, entry.ID)
		if err != nil {
			suite.t.Logf("Warning: Failed to get headers for HTTP %s: %v", entry.ID, err)
			continue
		}
		totalHeaders += len(headers)
	}
	return totalHeaders
}

func (suite *HARImportE2ETestSuite) countParamEntries() int {
	// Get all HTTP entries and count their search params
	httpEntries, err := suite.queries.GetHTTPsByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		suite.t.Fatalf("Failed to get HTTP entries for param count: %v", err)
	}

	totalParams := 0
	for _, entry := range httpEntries {
		params, err := suite.queries.GetHTTPSearchParams(suite.ctx, entry.ID)
		if err != nil {
			suite.t.Logf("Warning: Failed to get params for HTTP %s: %v", entry.ID, err)
			continue
		}
		totalParams += len(params)
	}
	return totalParams
}

func (suite *HARImportE2ETestSuite) logDetailedDatabaseState(label string) {
	t := suite.t

	t.Logf("🔍 === %s - Detailed Database State ===", label)

	// Get all HTTP entries
	httpEntries, err := suite.queries.GetHTTPsByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		t.Logf("❌ Failed to get HTTP entries: %v", err)
		return
	}

	for i, entry := range httpEntries {
		t.Logf("  %d. HTTP: %s %s | ID: %s | Delta: %t | Parent: %s",
			i+1, entry.Method, entry.Url, entry.ID, entry.IsDelta,
			func() string {
				if entry.ParentHttpID != nil {
					return entry.ParentHttpID.String()
				}
				return "nil"
			}())

		// Count headers for this HTTP entry
		headers, err := suite.queries.GetHTTPHeaders(suite.ctx, entry.ID)
		if err == nil {
			t.Logf("     Headers: %d", len(headers))
		}

		// Count params for this HTTP entry
		params, err := suite.queries.GetHTTPSearchParams(suite.ctx, entry.ID)
		if err == nil {
			t.Logf("     Params: %d", len(params))
		}
	}

	t.Logf("🔍 === End Database State ===")
}

func (suite *HARImportE2ETestSuite) analyzeDuplicateImportBehavior() {
	t := suite.t

	t.Logf("🔍 === Duplicate Import Analysis ===")

	// Analyze HTTP entries for potential duplicates
	httpEntries, err := suite.queries.GetHTTPsByWorkspaceID(suite.ctx, suite.workspaceID)
	if err != nil {
		t.Logf("❌ Failed to get HTTP entries for analysis: %v", err)
		return
	}

	// Group by URL+Method to identify potential duplicates
	requestGroups := make(map[string][]gen.Http)
	for _, entry := range httpEntries {
		key := fmt.Sprintf("%s:%s", entry.Method, entry.Url)
		requestGroups[key] = append(requestGroups[key], entry)
	}

	for key, entries := range requestGroups {
		if len(entries) > 2 {
			t.Logf("⚠️  Potential issue: %s has %d entries (expected max 2)", key, len(entries))
		} else if len(entries) == 2 {
			t.Logf("✅ Normal base-delta pair: %s", key)
		} else {
			t.Logf("ℹ️  Single entry: %s", key)
		}

		// Check if we have the expected base-delta structure
		var baseCount, deltaCount int
		for _, entry := range entries {
			if entry.IsDelta {
				deltaCount++
			} else {
				baseCount++
			}
		}

		if baseCount > 1 {
			t.Logf("❌ Multiple base entries found for %s - overwrite detection may be failing", key)
		}
		if deltaCount > 1 {
			t.Logf("ℹ️  Multiple delta entries found for %s - multiple imports detected", key)
		}
	}

	t.Logf("🔍 === End Duplicate Import Analysis ===")
}

func (suite *HARImportE2ETestSuite) analyzeChildEntities(baseID, deltaID, key string) {
	t := suite.t

	// Get headers for base and delta
	baseHeaders, err := suite.queries.GetHTTPHeaders(suite.ctx, idwrap.NewTextMust(baseID))
	if err != nil {
		t.Logf("❌ Failed to get base headers for %s: %v", key, err)
		return
	}

	deltaHeaders, err := suite.queries.GetHTTPHeaders(suite.ctx, idwrap.NewTextMust(deltaID))
	if err != nil {
		t.Logf("❌ Failed to get delta headers for %s: %v", key, err)
		return
	}

	t.Logf("  📋 Base headers: %d, Delta headers: %d", len(baseHeaders), len(deltaHeaders))

	if len(deltaHeaders) == 0 {
		t.Logf("  ⚠️  No delta headers found - child entities may be missing")
	} else {
		t.Logf("  ✅ Delta headers created successfully")
		// Show a few sample headers
		for i, header := range deltaHeaders {
			if i < 3 { // Show first 3
				t.Logf("    📝 %s: %s", header.HeaderKey, header.HeaderValue)
			}
		}
		if len(deltaHeaders) > 3 {
			t.Logf("    ... and %d more", len(deltaHeaders)-3)
		}
	}
}

// func (suite *HARImportE2ETestSuite) analyzeRPCvsDatabaseEntries(httpItems []*apiv1.Http, deltaItems []*apiv1.HttpDelta) {
// 	t := suite.t

// 	t.Logf("🔍 === RPC vs Database Entry Analysis ===")

// 	// TODO: Implement this when HTTP client functionality is available
// 	t.Skip("RPC vs Database analysis not yet implemented")
// }
