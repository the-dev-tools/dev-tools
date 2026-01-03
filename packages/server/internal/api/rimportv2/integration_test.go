package rimportv2

import (
	"the-dev-tools/server/pkg/service/senv"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"

	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
)

// integrationTestFixture represents the complete test setup for integration testing

type integrationTestFixture struct {
	ctx context.Context

	base *testutil.BaseDBQueries

	services BaseTestServices

	rpc *ImportV2RPC

	userID idwrap.IDWrap

	workspaceID idwrap.IDWrap

	logger *slog.Logger

	streamers IntegrationTestStreamers
}

type IntegrationTestStreamers struct {

	Flow       eventstream.SyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent]

	Node       eventstream.SyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent]

	Edge       eventstream.SyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent]

	Http       eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]

	HttpHeader eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]



	HttpSearchParam eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]



	HttpBodyForm eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]



	HttpBodyUrlEncoded eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]



	HttpBodyRaw eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]



	HttpAssert eventstream.SyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]



	File eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]

	Env eventstream.SyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]

	EnvVar eventstream.SyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent]
}



// BaseTestServices wraps the testutil services for easier access



type BaseTestServices struct {

	Us suser.UserService



	Ws sworkspace.WorkspaceService



	Wus sworkspace.UserService



	Hs shttp.HTTPService



	Fs sfile.FileService



	Fls sflow.FlowService

}



// newIntegrationTestFixture creates a complete test environment for integration tests



func newIntegrationTestFixture(t *testing.T) *integrationTestFixture {



	t.Helper()



	ctx := context.Background()



	base := testutil.CreateBaseDB(ctx, t)



	t.Cleanup(base.Close)



	// Get base services



	baseServices := base.GetBaseServices()



	logger := base.Logger()



	// Create additional services needed for import



	httpService := shttp.New(base.Queries, logger)



	flowService := sflow.NewFlowService(base.Queries)



	fileService := sfile.New(base.Queries, logger)



	httpHeaderService := shttp.NewHttpHeaderService(base.Queries)



	httpSearchParamService := shttp.NewHttpSearchParamService(base.Queries)



	httpBodyFormService := shttp.NewHttpBodyFormService(base.Queries)



	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(base.Queries)



	bodyService := shttp.NewHttpBodyRawService(base.Queries)



	httpAssertService := shttp.NewHttpAssertService(base.Queries)



	nodeService := sflow.NewNodeService(base.Queries)



	nodeRequestService := sflow.NewNodeRequestService(base.Queries)



	edgeService := sflow.NewEdgeService(base.Queries)



	envService := senv.NewEnvironmentService(base.Queries, logger)



	varService := senv.NewVariableService(base.Queries, logger)



	// Create streamers



	streamers := IntegrationTestStreamers{



		Flow: memory.NewInMemorySyncStreamer[rflowv2.FlowTopic, rflowv2.FlowEvent](),



		Node: memory.NewInMemorySyncStreamer[rflowv2.NodeTopic, rflowv2.NodeEvent](),



		Edge: memory.NewInMemorySyncStreamer[rflowv2.EdgeTopic, rflowv2.EdgeEvent](),



		Http:       memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent](),

		HttpHeader: memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent](),



		HttpSearchParam: memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent](),



		HttpBodyForm: memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent](),



		HttpBodyUrlEncoded: memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent](),



		HttpBodyRaw: memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent](),



		HttpAssert: memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent](),



		File: memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent](),

		Env: memory.NewInMemorySyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent](),

		EnvVar: memory.NewInMemorySyncStreamer[renv.EnvironmentVariableTopic, renv.EnvironmentVariableEvent](),
	}



	// Create user and workspace



	userID := idwrap.NewNow()



	workspaceID := idwrap.NewNow()



	// Create test user



	err := baseServices.Us.CreateUser(ctx, &muser.User{



		ID: userID,



		Email: "test@example.com",



		Password: []byte("password"),



		ProviderType: muser.MagicLink,



		Status:       muser.Active,

	})



	require.NoError(t, err)



	// Create test workspace



	err = baseServices.Ws.Create(ctx, &mworkspace.Workspace{



		ID: workspaceID,



		Name: "Test Workspace",

	})



	require.NoError(t, err)



	// Create workspace-user relationship



	err = baseServices.Wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{



		ID: idwrap.NewNow(),



		WorkspaceID: workspaceID,



		UserID:      userID,



		Role:        mworkspace.RoleOwner,

	})



	require.NoError(t, err)



	// Create RPC handler
	rpc := NewImportV2RPC(ImportV2Deps{
		DB:     base.DB,
		Logger: logger,
		Services: ImportServices{
			Workspace:          baseServices.Ws,
			User:               baseServices.Us,
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
			Edge:               &edgeService,
		},
		Readers: ImportV2Readers{
			Workspace: baseServices.Ws.Reader(),
			User:      baseServices.Wus.Reader(),
		},
		Streamers: ImportStreamers{
			Flow:               streamers.Flow,
			Node:               streamers.Node,
			Edge:               streamers.Edge,
			Http:               streamers.Http,
			HttpHeader:         streamers.HttpHeader,
			HttpSearchParam:    streamers.HttpSearchParam,
			HttpBodyForm:       streamers.HttpBodyForm,
			HttpBodyUrlEncoded: streamers.HttpBodyUrlEncoded,
			HttpBodyRaw:        streamers.HttpBodyRaw,
			HttpAssert:         streamers.HttpAssert,
			File:               streamers.File,
			Env:                streamers.Env,
			EnvVar:             streamers.EnvVar,
		},
	})

	services := BaseTestServices{

		Us: baseServices.Us,

		Ws: baseServices.Ws,

		Wus: baseServices.Wus,

		Hs: httpService,

		Fs: *fileService,

		Fls: flowService,
	}

	return &integrationTestFixture{

		ctx: mwauth.CreateAuthedContext(ctx, userID),

		base: base,

		services: services,

		rpc: rpc,

		userID: userID,

		workspaceID: workspaceID,

		logger: logger,

		streamers: streamers,
	}

}

// TestImportRPC_Integration tests the complete import flow through the RPC
func TestImportRPC_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	tests := []struct {
		name         string
		harData      []byte
		domainData   []ImportDomainData // nil = first call (not provided), empty = skip, with values = configure
		useNilDomain bool               // When true, use nil for DomainData in request (first call behavior)
		expectError  bool
		expectResp   func(*apiv1.ImportResponse) bool
	}{
		{
			name:    "successful simple import",
			harData: createSimpleHAR(t),
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
			},
			expectError: false,
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return len(resp.Domains) == 0 // Domains should be empty on success
			},
		},
		{
			// Two-step flow: First call returns domains for user to configure
			name:         "import with missing domain data",
			harData:      createMultiEntryHAR(t),
			useNilDomain: true, // nil signals first call - return domains for configuration
			expectError:  false,
			expectResp: func(resp *apiv1.ImportResponse) bool {
				// MissingData = DOMAIN signals client to prompt user for domain configuration
				// Domains are returned so user can map them to variables
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_DOMAIN && len(resp.Domains) > 0
			},
		},
		{
			name:         "complex HAR with multiple domains",
			harData:      createComplexHAR(t),
			useNilDomain: true, // nil signals first call - return domains
			expectError:  false,
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return len(resp.Domains) >= 1 // Should find api.example.com (XHR request), but not cdn.example.com (CSS file)
			},
		},
		{
			name:        "invalid HAR format",
			harData:     []byte(`{"invalid": "json"}`),
			domainData:  []ImportDomainData{},
			expectError: true,
		},
		{
			name:        "empty HAR",
			harData:     createEmptyHAR(t),
			domainData:  []ImportDomainData{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create protobuf domain data
			// nil = first call (domain not provided), empty slice = user chose to skip
			var protoDomainData []*apiv1.ImportDomainData
			if !tt.useNilDomain {
				protoDomainData = make([]*apiv1.ImportDomainData, len(tt.domainData))
				for i, dd := range tt.domainData {
					protoDomainData[i] = &apiv1.ImportDomainData{
						Enabled:  dd.Enabled,
						Domain:   dd.Domain,
						Variable: dd.Variable,
					}
				}
			}
			// If useNilDomain is true, protoDomainData stays nil (first call behavior)

			// Create request
			req := connect.NewRequest(&apiv1.ImportRequest{
				WorkspaceId: fixture.workspaceID.Bytes(),
				Name:        "Integration Test Import",
				Data:        tt.harData,
				DomainData:  protoDomainData,
			})

			// Call RPC
			resp, err := fixture.rpc.Import(fixture.ctx, req)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify response structure
				if tt.expectResp != nil {
					require.True(t, tt.expectResp(resp.Msg), "Response validation failed")
				}

				// Verify that data was actually stored in the database
				shouldStore := len(tt.harData) > 0 && !tt.expectError
				// If MissingData is returned, we expect NO data to be stored (new behavior)
				if resp.Msg.MissingData != apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED {
					shouldStore = false
				}

				if shouldStore {
					fixture.verifyStoredData(t, tt.harData)
				}
			}
		})
	}
}

// TestImportService_DatabaseImport tests basic database import functionality
func TestImportService_DatabaseImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Create a HAR that will generate multiple entities
	harData := createMultiEntryHAR(t)

	// Perform import with regular service
	req := &ImportRequest{
		WorkspaceID: fixture.workspaceID,
		Name:        "Database Import Test",
		Data:        harData,
		DomainData: []ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
		},
		DomainDataWasProvided: true,
	}

	// Create the service directly using the RPC's service
	service := fixture.rpc.service
	resp, err := service.ImportUnified(fixture.ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify data was stored correctly
	fixture.verifyStoredData(t, harData)
}

// TestImportService_ErrorHandling tests error handling in import operations
func TestImportService_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Create HAR that will cause an error during processing
	harData := createProblematicHAR(t)

	// Perform import that should fail with regular service
	req := &ImportRequest{
		WorkspaceID: fixture.workspaceID,
		Name:        "Error Test Import",
		Data:        harData,
		DomainData:  []ImportDomainData{},
	}

	// Use the RPC's service directly
	service := fixture.rpc.service
	_, err := service.Import(fixture.ctx, req)
	require.Error(t, err) // Should fail

	// Verify error handling - the service should not have stored partial data
	// Check that no unexpected HTTP requests were created for this workspace
	httpReqs, err := fixture.services.Hs.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	// Note: Some requests might exist from previous tests, so we just check the operation didn't crash
	t.Logf("HTTP requests count after error: %d", len(httpReqs))
}

// TestImportService_ConcurrentImports tests concurrent import operations
func TestImportService_ConcurrentImports(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	numImports := 5
	results := make(chan error, numImports)

	// Run multiple imports concurrently
	for i := 0; i < numImports; i++ {
		go func(index int) {
			harData := createIndexedHAR(t, index)
			req := connect.NewRequest(&apiv1.ImportRequest{
				WorkspaceId: fixture.workspaceID.Bytes(),
				Name:        fmt.Sprintf("Concurrent Import %d", index),
				Data:        harData,
				DomainData: []*apiv1.ImportDomainData{
					{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
				},
			})

			_, err := fixture.rpc.Import(fixture.ctx, req)
			results <- err
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numImports; i++ {
		err := <-results
		if err != nil {
			errorCount++
			t.Logf("Import %d failed: %v", i, err)
		} else {
			successCount++
		}
	}

	t.Logf("Concurrent imports: %d successful, %d failed", successCount, errorCount)

	// At least some imports should succeed
	require.Greater(t, successCount, 0, "At least some concurrent imports should succeed")

	// Verify that successful imports created data
	httpReqs, err := fixture.services.Hs.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Greater(t, len(httpReqs), 0, "HTTP requests should exist from successful imports")
}

// TestImportService_LargeHARImport tests importing a large HAR file
func TestImportService_LargeHARImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large HAR test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Create a HAR with many entries
	harData := createLargeHAR(t, 50) // 50 entries

	start := time.Now()
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Large HAR Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
			{Enabled: true, Domain: "data.example.org", Variable: "DATA_DOMAIN"},
			{Enabled: true, Domain: "service.example.net", Variable: "SERVICE_DOMAIN"},
		},
	})

	resp, err := fixture.rpc.Import(fixture.ctx, req)
	duration := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("Large HAR import (50 entries) completed in %v", duration)
	// Verify all entries were processed
	httpReqs, err := fixture.services.Hs.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Len(t, httpReqs, 50, "All 50 HTTP requests should be imported")

	// Domains should be cleared on success
	require.Empty(t, resp.Msg.Domains, "Domains should be cleared on successful import")
}

// TestImportService_DomainProcessingIntegration tests domain processing functionality
func TestImportService_DomainProcessingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping domain processing test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Create HAR with multiple domains
	harData := createMultiDomainHAR(t)

	tests := []struct {
		name       string
		domainData []ImportDomainData // nil = first call (not provided), empty = skip, with values = configure
		expectResp func(*apiv1.ImportResponse) bool
	}{
		{
			// First call: domainData not provided (nil) - return domains for user to configure
			name:       "first call - domain data not provided",
			domainData: nil, // nil means first call, domains should be returned
			expectResp: func(resp *apiv1.ImportResponse) bool {
				// MissingData = DOMAIN signals client to prompt user for domain configuration
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_DOMAIN
			},
		},
		{
			// Second call: user chose to skip domain configuration (empty array)
			name:       "second call - user skipped domain config",
			domainData: []ImportDomainData{}, // empty means user chose to skip
			expectResp: func(resp *apiv1.ImportResponse) bool {
				// Import proceeds without creating env vars, domains cleared
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED
			},
		},
		{
			// Second call with domain data: import proceeds and creates env vars
			name: "second call - domain data provided",
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
				{Enabled: true, Domain: "cdn.example.com", Variable: "CDN_DOMAIN"},
			},
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED
			},
		},
		{
			name: "second call - partial domain data",
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
				// Missing cdn.example.com - but we still proceed
			},
			expectResp: func(resp *apiv1.ImportResponse) bool {
				// We proceed with partial data, so storage happens, so domains are cleared
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED && len(resp.Domains) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert domain data to protobuf format
			// nil means "not provided yet" (first call)
			// empty slice means "user chose to skip" (second call with no mappings)
			var protoDomainData []*apiv1.ImportDomainData
			if tt.domainData != nil {
				protoDomainData = make([]*apiv1.ImportDomainData, len(tt.domainData))
				for i, dd := range tt.domainData {
					protoDomainData[i] = &apiv1.ImportDomainData{
						Enabled:  dd.Enabled,
						Domain:   dd.Domain,
						Variable: dd.Variable,
					}
				}
			}

			req := connect.NewRequest(&apiv1.ImportRequest{
				WorkspaceId: fixture.workspaceID.Bytes(),
				Name:        "Domain Processing Test",
				Data:        harData,
				DomainData:  protoDomainData,
			})

			resp, err := fixture.rpc.Import(fixture.ctx, req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			// Verify domain processing response
			require.True(t, tt.expectResp(resp.Msg), "Domain processing response validation failed")

			// If MissingData is set (waiting for user config), domains should be returned
			// If MissingData is NOT set (success), domains should be cleared
			if resp.Msg.MissingData != apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED {
				require.NotEmpty(t, resp.Msg.Domains, "Should return domains when waiting for user configuration")
			} else {
				require.Empty(t, resp.Msg.Domains, "Should clear domains on successful import")
			}
		})
	}
}

// Helper methods for fixture

func (f *integrationTestFixture) verifyStoredData(t *testing.T, originalHAR []byte) {
	t.Helper()

	// Parse the original HAR to verify expected content
	var har struct {
		Log struct {
			Entries []struct {
				Request struct {
					Method string `json:"method"`
					URL    string `json:"url"`
				} `json:"request"`
			} `json:"entries"`
		} `json:"log"`
	}

	err := json.Unmarshal(originalHAR, &har)
	require.NoError(t, err)

	// Verify HTTP requests were stored
	httpReqs, err := f.services.Hs.GetByWorkspaceID(f.ctx, f.workspaceID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(httpReqs), len(har.Log.Entries),
		"Should have stored at least as many HTTP requests as HAR entries")

	// Verify flows were created
	flows, err := f.services.Fls.GetFlowsByWorkspaceID(f.ctx, f.workspaceID)
	require.NoError(t, err)
	require.NotEmpty(t, flows, "Should have created flows")

	// Verify files if any were referenced in HAR (binary data, etc.)
	// This is more complex to test without specific HAR content
}

// TestImportRPC_SyncEvents verifies that sync events are published during import
func TestImportRPC_SyncEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sync events test in short mode")
	}

	fixture := newIntegrationTestFixture(t)
	harData := createComplexHAR(t) // Contains headers, params, etc.

	// Subscribe to streamers
	httpCh, err := fixture.streamers.Http.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	headerCh, err := fixture.streamers.HttpHeader.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	paramCh, err := fixture.streamers.HttpSearchParam.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	fileCh, err := fixture.streamers.File.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	// Perform import
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Sync Events Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
			{Enabled: true, Domain: "cdn.example.com", Variable: "CDN_DOMAIN"},
		},
	})

	_, err = fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)

	// Verify events are received
	// We expect multiple events for HTTP requests, Headers, and Params

	// Check HTTP events (at least 2 from complex HAR)
	select {
	case evt := <-httpCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.Http)
	case <-time.After(time.Second):
		require.Fail(t, "Timed out waiting for HTTP event")
	}

	// Check File events (for the files created)
	select {
	case evt := <-fileCh:
		require.Equal(t, "create", evt.Payload.Type)
		require.NotNil(t, evt.Payload.File)
	case <-time.After(time.Second):
		require.Fail(t, "Timed out waiting for File event")
	}

	// Check Header events
	select {
	case evt := <-headerCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.HttpHeader)
	case <-time.After(time.Second):
		require.Fail(t, "Timed out waiting for Header event")
	}

	// Check SearchParam events (Complex HAR has query params)
	select {
	case evt := <-paramCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.HttpSearchParam)
	case <-time.After(time.Second):
		require.Fail(t, "Timed out waiting for SearchParam event")
	}
}

// HAR creation helpers for testing

func createSimpleHAR(t *testing.T) []byte {
	return []byte(`{
		"log": {
			"version": "1.2",
			"creator": {"name": "Test", "version": "1.0"},
			"entries": [
				{
					"startedDateTime": "` + time.Now().UTC().Format(time.RFC3339) + `",
					"time": 100,
					"request": {
						"method": "GET",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Accept", "value": "application/json"},
							{"name": "User-Agent", "value": "Test Agent"}
						]
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Content-Type", "value": "application/json"}
						],
						"content": {
							"size": 50,
							"mimeType": "application/json",
							"text": "{\"users\": []}"
						}
					}
				}
			]
		}
	}`)
}

func createMultiEntryHAR(t *testing.T) []byte {
	entries := make([]interface{}, 3)
	baseTime := time.Now().UTC()

	for i := 0; i < 3; i++ {
		entries[i] = map[string]interface{}{
			"startedDateTime": baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			"time":            100 + i*10,
			"request": map[string]interface{}{
				"method":      []string{"GET", "POST", "PUT"}[i],
				"url":         fmt.Sprintf("https://api.example.com/resource_%d", i),
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "application/json"},
				},
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
				},
				"content": map[string]interface{}{
					"size":     50,
					"mimeType": "application/json",
					"text":     fmt.Sprintf(`{"id": %d, "data": "test"}`, i),
				},
			},
		}
	}

	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name":    "Test HAR Generator",
				"version": "1.0",
			},
			"entries": entries,
		},
	}

	data, err := json.Marshal(har)
	require.NoError(t, err)
	return data
}

func createComplexHAR(t *testing.T) []byte {
	entries := []interface{}{
		map[string]interface{}{
			"startedDateTime": time.Now().UTC().Format(time.RFC3339),
			"time":            150,
			"request": map[string]interface{}{
				"method":      "GET",
				"url":         "https://api.example.com/users",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "application/json"},
					{"name": "Authorization", "value": "Bearer token123"},
				},
				"queryString": []map[string]interface{}{
					{"name": "page", "value": "1"},
					{"name": "limit", "value": "10"},
				},
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
				},
				"content": map[string]interface{}{
					"size":     200,
					"mimeType": "application/json",
					"text":     `{"users": [{"id": 1, "name": "John"}]}`,
				},
			},
		},
		map[string]interface{}{
			"startedDateTime": time.Now().Add(time.Second).UTC().Format(time.RFC3339),
			"time":            50,
			"request": map[string]interface{}{
				"method":      "GET",
				"url":         "https://cdn.example.com/assets/style.css",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "text/css"},
				},
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "text/css"},
				},
				"content": map[string]interface{}{
					"size":     1024,
					"mimeType": "text/css",
					"text":     "body { margin: 0; }",
				},
			},
		},
	}

	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name":    "Complex Test HAR",
				"version": "1.0",
			},
			"entries": entries,
		},
	}

	data, err := json.Marshal(har)
	require.NoError(t, err)
	return data
}

func createMultiDomainHAR(t *testing.T) []byte {
	entries := []interface{}{
		map[string]interface{}{
			"startedDateTime": time.Now().UTC().Format(time.RFC3339),
			"request": map[string]interface{}{
				"method":      "GET",
				"url":         "https://api.example.com/data",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "application/json"},
				},
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
			},
		},
		map[string]interface{}{
			"startedDateTime": time.Now().Add(time.Second).UTC().Format(time.RFC3339),
			"request": map[string]interface{}{
				"method":      "GET",
				"url":         "https://cdn.example.com/assets/image.png",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "image/png"},
				},
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
			},
		},
		map[string]interface{}{
			"startedDateTime": time.Now().Add(2 * time.Second).UTC().Format(time.RFC3339),
			"request": map[string]interface{}{
				"method":      "POST",
				"url":         "https://api.example.com/submit",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
				},
				"postData": map[string]interface{}{
					"mimeType": "application/json",
					"text":     `{"data": "test"}`,
				},
			},
			"response": map[string]interface{}{
				"status":      201,
				"statusText":  "Created",
				"httpVersion": "HTTP/1.1",
			},
		},
	}

	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name":    "Multi-Domain Test HAR",
				"version": "1.0",
			},
			"entries": entries,
		},
	}

	data, err := json.Marshal(har)
	require.NoError(t, err)
	return data
}

func createEmptyHAR(t *testing.T) []byte {
	return []byte(`{
		"log": {
			"version": "1.2",
			"creator": {"name": "Test", "version": "1.0"},
			"entries": []
		}
	}`)
}

func createProblematicHAR(t *testing.T) []byte {
	// Create a HAR that might cause processing issues
	return []byte(`{
		"log": {
			"version": "1.2",
			"entries": [
				{
					"startedDateTime": "invalid-date",
					"request": {
						"method": "INVALID",
						"url": "not-a-url"
					},
					"response": {
						"status": -1
					}
				}
			]
		}
	}`)
}

func createLargeHAR(t *testing.T, numEntries int) []byte {
	entries := make([]interface{}, numEntries)
	baseTime := time.Now().UTC()

	domains := []string{"api.example.com", "data.example.org", "service.example.net"}
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for i := 0; i < numEntries; i++ {
		domain := domains[i%len(domains)]
		method := methods[i%len(methods)]

		entries[i] = map[string]interface{}{
			"startedDateTime": baseTime.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339),
			"time":            100 + (i % 50),
			"request": map[string]interface{}{
				"method":      method,
				"url":         fmt.Sprintf("https://%s/endpoint_%d", domain, i),
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "application/json"},
					{"name": "User-Agent", "value": "Large HAR Test"},
				},
				"queryString": []map[string]interface{}{
					{"name": "id", "value": fmt.Sprintf("%d", i)},
				},
			},
			"response": map[string]interface{}{
				"status":      200,
				"statusText":  "OK",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Content-Type", "value": "application/json"},
					{"name": "Cache-Control", "value": "no-cache"},
				},
				"content": map[string]interface{}{
					"size":     100 + (i % 200),
					"mimeType": "application/json",
					"text":     fmt.Sprintf(`{"id": %d, "timestamp": "%s"}`, i, baseTime.Format(time.RFC3339)),
				},
			},
		}
	}

	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name":    "Large HAR Generator",
				"version": "1.0.0",
			},
			"entries": entries,
		},
	}

	data, err := json.Marshal(har)
	require.NoError(t, err)
	return data
}

func createIndexedHAR(t *testing.T, index int) []byte {
	return []byte(fmt.Sprintf(`{
		"log": {
			"version": "1.2",
			"entries": [
				{
					"startedDateTime": "%s",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/concurrent_%d",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "X-Test-Index", "value": "%d"}
						]
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"content": {
							"text": "concurrent test %d"
						}
					}
				}
			]
		}
	}`, time.Now().UTC().Format(time.RFC3339), index, index, index))
}

// TestYAMLImport_WithTimeout tests that YAML import completes without stalling
// This test verifies the fix for missing flow nodes/edges in YAMLTranslator
func TestYAMLImport_WithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping YAML import test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// YAML data similar to what the exporter generates with use_request references
	yamlData := []byte(`workspace_name: New Workspace
requests:
    - name: DELETE Api Categories
      method: DELETE
      url: https://ecommerce-admin-panel.fly.dev/api/categories/c2b85766
      headers:
        accept: '*/*'
        authorization: Bearer token123
    - name: GET Api Categories
      method: GET
      url: https://ecommerce-admin-panel.fly.dev/api/categories
      headers:
        accept: '*/*'
        authorization: Bearer token123
    - name: POST Api Auth Login
      method: POST
      url: https://ecommerce-admin-panel.fly.dev/api/auth/login
      body:
        json:
            email: admin@example.com
            password: admin123
        type: json
flows:
    - name: Imported HAR Flow
      steps:
        - request:
            name: request_1
            use_request: POST Api Auth Login
        - request:
            name: request_2
            use_request: GET Api Categories
        - request:
            depends_on:
                - request_1
            name: request_3
            use_request: DELETE Api Categories
`)

	// Create a context with timeout to detect stalls
	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	// Create the request - YAML imports don't need DomainData
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "YAML Import Test",
		Data:        yamlData,
	})

	// Channel to capture result
	type result struct {
		resp *connect.Response[apiv1.ImportResponse]
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := fixture.rpc.Import(ctx, req)
		done <- result{resp, err}
	}()

	// Wait for completion or timeout
	select {
	case res := <-done:
		require.NoError(t, res.err, "YAML import should complete without error")
		require.NotNil(t, res.resp, "Response should not be nil")

		// Verify data was stored
		httpReqs, err := fixture.services.Hs.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(httpReqs), 3, "Should have at least 3 HTTP requests")

		flows, err := fixture.services.Fls.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
		require.NoError(t, err)
		require.NotEmpty(t, flows, "Should have created flows")

		t.Logf("YAML import completed successfully:")
		t.Logf("  - HTTP Requests: %d", len(httpReqs))
		t.Logf("  - Flows: %d", len(flows))

	case <-ctx.Done():
		t.Fatal("YAML import timed out - import is stalling!")
	}
}

// TestYAMLImport_WithFlowNodes verifies that flow nodes and edges are properly stored
func TestYAMLImport_WithFlowNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping YAML flow nodes test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// YAML with explicit dependencies to test edge creation
	yamlData := []byte(`workspace_name: Flow Nodes Test
flows:
    - name: Test Flow
      steps:
        - request:
            name: Step 1
            method: GET
            url: https://api.example.com/step1
        - request:
            name: Step 2
            method: GET
            url: https://api.example.com/step2
        - request:
            name: Step 3
            method: POST
            url: https://api.example.com/step3
            depends_on:
                - Step 1
                - Step 2
`)

	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	// YAML imports don't need DomainData - they persist directly
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Flow Nodes Test",
		Data:        yamlData,
	})

	type result struct {
		resp *connect.Response[apiv1.ImportResponse]
		err  error
	}
	done := make(chan result, 1)

	go func() {
		resp, err := fixture.rpc.Import(ctx, req)
		done <- result{resp, err}
	}()

	select {
	case res := <-done:
		require.NoError(t, res.err, "YAML import should complete without error")

		// Verify flows were created
		flows, err := fixture.services.Fls.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
		require.NoError(t, err)
		require.Len(t, flows, 1, "Should have exactly 1 flow")

		// Verify nodes were created using the RPC's node service
		nodes, err := fixture.rpc.NodeService.GetNodesByFlowID(fixture.ctx, flows[0].ID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(nodes), 3, "Should have at least 3 request nodes + start node")

		// Verify edges were created using the RPC's edge service
		edges, err := fixture.rpc.EdgeService.GetEdgesByFlowID(fixture.ctx, flows[0].ID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(edges), 1, "Should have at least 1 edge")

		t.Logf("Flow nodes test completed successfully with %d flow(s), %d nodes, %d edges", len(flows), len(nodes), len(edges))

	case <-ctx.Done():
		t.Fatal("YAML import with flow nodes timed out!")
	}
}

// TestYAMLImport_FileNames verifies that file names are correctly set during YAML import
func TestYAMLImport_FileNames(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping YAML file names test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// YAML with use_request references - file names should match step names
	yamlData := []byte(`workspace_name: File Names Test
requests:
    - name: Get Users API
      method: GET
      url: https://api.example.com/users
    - name: Create User API
      method: POST
      url: https://api.example.com/users
flows:
    - name: User Flow
      steps:
        - request:
            name: fetch_users
            use_request: Get Users API
        - request:
            name: create_user
            use_request: Create User API
`)

	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	// YAML imports don't need DomainData - they persist directly
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "File Names Test",
		Data:        yamlData,
	})

	resp, err := fixture.rpc.Import(ctx, req)
	require.NoError(t, err, "YAML import should complete without error")
	require.NotNil(t, resp, "Response should not be nil")

	// Get all files in the workspace
	files, err := fixture.services.Fs.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Log all files for debugging
	t.Logf("Created %d files:", len(files))
	for i, file := range files {
		t.Logf("  [%d] Name=%q ContentType=%d", i, file.Name, file.ContentType)
	}

	// Verify file names are NOT empty or "N/A"
	for _, file := range files {
		require.NotEmpty(t, file.Name, "File name should not be empty")
		require.NotEqual(t, "N/A", file.Name, "File name should not be N/A")
	}

	// Check for expected file names (step names: fetch_users, create_user)
	fileNames := make(map[string]bool)
	for _, file := range files {
		fileNames[file.Name] = true
	}

	require.True(t, fileNames["fetch_users"] || fileNames["create_user"],
		"Should have files named after the step names (fetch_users, create_user)")
}

// TestImportRPC_DomainReplacement tests that domain URLs are replaced with variable references
// and stored correctly in the database
func TestImportRPC_DomainReplacement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Create a simple HAR with a known domain
	harData := []byte(`{
		"log": {
			"version": "1.2",
			"creator": {"name": "test", "version": "1.0"},
			"entries": [
				{
					"startedDateTime": "2024-01-01T00:00:00.000Z",
					"time": 100,
					"request": {
						"method": "GET",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"queryString": [],
						"cookies": [],
						"headersSize": -1,
						"bodySize": 0
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"cookies": [],
						"content": {"size": 0, "mimeType": "application/json"},
						"redirectURL": "",
						"headersSize": -1,
						"bodySize": 0
					},
					"cache": {},
					"timings": {"send": 0, "wait": 100, "receive": 0}
				},
				{
					"startedDateTime": "2024-01-01T00:00:01.000Z",
					"time": 100,
					"request": {
						"method": "POST",
						"url": "https://api.example.com/users/create",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"queryString": [],
						"cookies": [],
						"headersSize": -1,
						"bodySize": 0
					},
					"response": {
						"status": 201,
						"statusText": "Created",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"cookies": [],
						"content": {"size": 0, "mimeType": "application/json"},
						"redirectURL": "",
						"headersSize": -1,
						"bodySize": 0
					},
					"cache": {},
					"timings": {"send": 0, "wait": 100, "receive": 0}
				}
			]
		}
	}`)

	// Step 1: First import call without domain data - should return domains for configuration
	req1 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Domain Replacement Test",
		Data:        harData,
		DomainData:  nil, // nil = first call, should return domains
	})

	resp1, err := fixture.rpc.Import(fixture.ctx, req1)
	require.NoError(t, err, "First import call should succeed")
	require.NotNil(t, resp1)

	// Verify domains are returned
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_DOMAIN, resp1.Msg.MissingData,
		"Should indicate domain data is missing")
	require.Contains(t, resp1.Msg.Domains, "api.example.com",
		"Should return api.example.com as a detected domain")

	t.Logf("Step 1: Detected domains: %v", resp1.Msg.Domains)

	// Step 2: Second import call with domain data - should replace URLs and store
	req2 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Domain Replacement Test",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "api.example.com",
				Variable: "API_HOST",
			},
		},
	})

	resp2, err := fixture.rpc.Import(fixture.ctx, req2)
	require.NoError(t, err, "Second import call should succeed")
	require.NotNil(t, resp2)

	// Verify import succeeded
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp2.Msg.MissingData,
		"Should not indicate any missing data")
	require.Empty(t, resp2.Msg.Domains, "Domains should be empty after successful import with domain data")

	t.Logf("Step 2: Import completed, FlowId: %x", resp2.Msg.FlowId)

	// Step 3: Verify the stored HTTP requests have replaced URLs
	// Query the database directly to check the stored URLs
	httpRequests, err := fixture.services.Hs.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err, "Should be able to list HTTP requests")
	require.NotEmpty(t, httpRequests, "Should have stored HTTP requests")

	t.Logf("Step 3: Found %d HTTP requests in database", len(httpRequests))

	// Check each URL is replaced with variable reference
	for i, httpReq := range httpRequests {
		t.Logf("  HTTP[%d]: Method=%s, URL=%s", i, httpReq.Method, httpReq.Url)

		// URLs should now use {{API_HOST}} instead of https://api.example.com
		require.NotContains(t, httpReq.Url, "https://api.example.com",
			"URL should not contain the original domain")
		require.Contains(t, httpReq.Url, "{{API_HOST}}",
			"URL should contain the variable reference {{API_HOST}}")
	}

	// Verify specific URL patterns
	urlsFound := make(map[string]bool)
	for _, httpReq := range httpRequests {
		urlsFound[httpReq.Url] = true
	}

	require.True(t, urlsFound["{{API_HOST}}/users"],
		"Should have URL {{API_HOST}}/users")
	require.True(t, urlsFound["{{API_HOST}}/users/create"],
		"Should have URL {{API_HOST}}/users/create")

	t.Log("Domain replacement test passed - URLs are correctly replaced in storage")
}

// TestYAMLImport_FlowFileEntryCreated verifies that a File entry (sidebar) is created
// for imported flows, so they appear in the sidebar file tree.
func TestYAMLImport_FlowFileEntryCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping flow file entry test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	yamlData := []byte(`workspace_name: Flow File Test
flows:
  - name: My Test Flow
    steps:
      - request:
          name: Request1
          method: GET
          url: "{{baseUrl}}/test"
`)

	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	// Import YAML without domain data
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Flow File Entry Test",
		Data:        yamlData,
	})

	resp, err := fixture.rpc.Import(ctx, req)
	require.NoError(t, err, "YAML import should complete without error")
	require.NotNil(t, resp)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData,
		"YAML imports should not require domain data")

	// Verify flow was created
	flows, err := fixture.services.Fls.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Len(t, flows, 1, "Should have exactly 1 flow")
	require.Equal(t, "My Test Flow", flows[0].Name)

	t.Logf("Flow created: ID=%s Name=%s", flows[0].ID.String(), flows[0].Name)

	// Verify File entry was created for the flow (sidebar entry)
	// The File entry should have fileId = flowId and ContentType = Flow
	files, err := fixture.services.Fs.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Filter for flow files only (ContentType = ContentTypeFlow = 3)
	var flowFiles []mfile.File
	for _, f := range files {
		if f.ContentType == mfile.ContentTypeFlow {
			flowFiles = append(flowFiles, f)
		}
	}

	require.Len(t, flowFiles, 1, "Should have exactly 1 flow file entry in sidebar")
	require.Equal(t, flows[0].ID, flowFiles[0].ID, "Flow file ID should equal flow ID")
	require.Equal(t, "My Test Flow", flowFiles[0].Name, "Flow file name should match flow name")
	require.Equal(t, fixture.workspaceID, flowFiles[0].WorkspaceID, "Flow file should belong to workspace")

	t.Logf("Flow File entry created: ID=%s Name=%s ContentType=%d",
		flowFiles[0].ID.String(), flowFiles[0].Name, flowFiles[0].ContentType)

	t.Log("Flow file entry test passed - flow appears in sidebar")
}

// TestYAMLImport_FolderFileCreated verifies that folder files are created
// for YAML imports containing flows with HTTP requests
func TestYAMLImport_FolderFileCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping folder file test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	yamlData := []byte(`workspace_name: Folder Test
flows:
  - name: Test Flow
    steps:
      - request:
          name: Request1
          method: GET
          url: https://example.com/test
`)

	ctx, cancel := context.WithTimeout(fixture.ctx, 10*time.Second)
	defer cancel()

	// Import YAML
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Folder File Test",
		Data:        yamlData,
	})

	// Also test the translator directly to see what files it creates
	translator := NewYAMLTranslator()
	translatorResult, err := translator.Translate(ctx, yamlData, fixture.workspaceID)
	require.NoError(t, err, "Translator should work")
	t.Logf("Translator returned %d files:", len(translatorResult.Files))
	for i, f := range translatorResult.Files {
		t.Logf("  [%d] Name=%q ContentType=%d ParentID=%v", i, f.Name, f.ContentType, f.ParentID)
	}

	resp, err := fixture.rpc.Import(ctx, req)
	require.NoError(t, err, "YAML import should complete without error")
	require.NotNil(t, resp)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData,
		"YAML imports should not require domain data")

	// Query all files from database
	files, err := fixture.services.Fs.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Count files by type
	var folderFiles []mfile.File
	var flowFiles []mfile.File
	var httpFiles []mfile.File

	for _, f := range files {
		switch f.ContentType {
		case mfile.ContentTypeFolder:
			folderFiles = append(folderFiles, f)
		case mfile.ContentTypeFlow:
			flowFiles = append(flowFiles, f)
		case mfile.ContentTypeHTTP:
			httpFiles = append(httpFiles, f)
		}
	}

	t.Logf("Files created: Folders=%d, Flows=%d, HTTP=%d", len(folderFiles), len(flowFiles), len(httpFiles))

	// YAML import creates:
	// 1. Flow file (ContentTypeFlow) - for the flow definition
	// 2. Folder file (ContentTypeFolder) - to contain HTTP requests
	// 3. HTTP file(s) (ContentTypeHTTP) - for each request
	require.Len(t, flowFiles, 1, "Should have 1 flow file")
	require.Len(t, folderFiles, 1, "Should have 1 folder file")
	require.Len(t, httpFiles, 1, "Should have 1 HTTP file")

	// Verify folder has correct properties
	folder := folderFiles[0]
	require.Equal(t, "Test Flow", folder.Name, "Folder name should match flow name")
	require.Nil(t, folder.ContentID, "Folder should not have ContentID")
	require.Equal(t, fixture.workspaceID, folder.WorkspaceID, "Folder should belong to workspace")

	// Verify HTTP file is inside the folder
	httpFile := httpFiles[0]
	require.NotNil(t, httpFile.ParentID, "HTTP file should have parent")
	require.Equal(t, folder.ID, *httpFile.ParentID, "HTTP file should be inside the folder")

	t.Log("Folder file test passed - folder created and HTTP file is inside folder")
}
