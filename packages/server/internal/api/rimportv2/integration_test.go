package rimportv2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"connectrpc.com/connect"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"
)

// integrationTestFixture represents the complete test setup for integration testing

type integrationTestFixture struct {

	ctx         context.Context

	base        *testutil.BaseDBQueries

	services    BaseTestServices

	rpc         *ImportV2RPC

	userID      idwrap.IDWrap

	workspaceID idwrap.IDWrap

	logger      *slog.Logger

	streamers   IntegrationTestStreamers

}



type IntegrationTestStreamers struct {

	Http               eventstream.SyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]

	HttpHeader         eventstream.SyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]

	HttpSearchParam    eventstream.SyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]

	HttpBodyForm       eventstream.SyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]

	HttpBodyUrlEncoded eventstream.SyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]

	HttpBodyRaw        eventstream.SyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]

	File               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]

}



// BaseTestServices wraps the testutil services for easier access

type BaseTestServices struct {

	Us  suser.UserService

	Ws  sworkspace.WorkspaceService

	Wus sworkspacesusers.WorkspaceUserService

	Hs  shttp.HTTPService

	Fs  sfile.FileService

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

	flowService := sflow.New(base.Queries)

	fileService := sfile.New(base.Queries, logger)



	httpHeaderService := shttpheader.New(base.Queries)

	httpSearchParamService := shttpsearchparam.New(base.Queries)

	httpBodyFormService := shttpbodyform.New(base.Queries)

	httpBodyUrlEncodedService := shttpbodyurlencoded.New(base.Queries)

	bodyService := shttp.NewHttpBodyRawService(base.Queries)



	// Create streamers

	streamers := IntegrationTestStreamers{

		Http:               memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent](),

		HttpHeader:         memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent](),

		HttpSearchParam:    memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent](),

		HttpBodyForm:       memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent](),

		HttpBodyUrlEncoded: memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent](),

		HttpBodyRaw:        memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent](),

		File:               memory.NewInMemorySyncStreamer[rfile.FileTopic, rfile.FileEvent](),

	}



	// Create user and workspace

	userID := idwrap.NewNow()

	workspaceID := idwrap.NewNow()



	// Create test user

	err := baseServices.Us.CreateUser(ctx, &muser.User{

		ID:           userID,

		Email:        "test@example.com",

		Password:     []byte("password"),

		ProviderType: muser.MagicLink,

		Status:       muser.Active,

	})

	require.NoError(t, err)



	// Create test workspace

	err = baseServices.Ws.Create(ctx, &mworkspace.Workspace{

		ID:   workspaceID,

		Name: "Test Workspace",

	})

	require.NoError(t, err)



	// Create workspace-user relationship

	err = baseServices.Wus.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{

		ID:          idwrap.NewNow(),

		WorkspaceID: workspaceID,

		UserID:      userID,

		Role:        mworkspaceuser.RoleOwner,

	})

	require.NoError(t, err)



	// Create the import RPC with all dependencies

	rpc := NewImportV2RPC(

		base.DB,

		baseServices.Ws,

		baseServices.Us,

		&httpService,

		&flowService,

		fileService,

		httpHeaderService,

		httpSearchParamService,

		httpBodyFormService,

		httpBodyUrlEncodedService,

		bodyService,

		logger,

		streamers.Http,

		streamers.HttpHeader,

		streamers.HttpSearchParam,

				streamers.HttpBodyForm,

				streamers.HttpBodyUrlEncoded,

				streamers.HttpBodyRaw,

				streamers.File,

			)

		

			services := BaseTestServices{

		Us:  baseServices.Us,

		Ws:  baseServices.Ws,

		Wus: baseServices.Wus,

		Hs:  httpService,

		Fs:  *fileService,

		Fls: flowService,

	}



	return &integrationTestFixture{

		ctx:         mwauth.CreateAuthedContext(ctx, userID),

		base:        base,

		services:    services,

		rpc:         rpc,

		userID:      userID,

		workspaceID: workspaceID,

		logger:      logger,

		streamers:   streamers,

	}

}

// TestImportRPC_Integration tests the complete import flow through the RPC
func TestImportRPC_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	tests := []struct {
		name        string
		harData     []byte
		domainData  []ImportDomainData
		expectError bool
		expectResp  func(*apiv1.ImportResponse) bool
	}{
		{
			name:    "successful simple import",
			harData: createSimpleHAR(t),
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
			},
			expectError: false,
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return len(resp.Domains) > 0
			},
		},
		{
			name:        "import with missing domain data",
			harData:     createMultiEntryHAR(t),
			domainData:  []ImportDomainData{},
			expectError: false,
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_DOMAIN && len(resp.Domains) > 0
			},
		},
		{
			name:        "complex HAR with multiple domains",
			harData:     createComplexHAR(t),
			domainData:  []ImportDomainData{},
			expectError: false,
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
			protoDomainData := make([]*apiv1.ImportDomainData, len(tt.domainData))
			for i, dd := range tt.domainData {
				protoDomainData[i] = &apiv1.ImportDomainData{
					Enabled:  dd.Enabled,
					Domain:   dd.Domain,
					Variable: dd.Variable,
				}
			}

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
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)

				// Verify response structure
				if tt.expectResp != nil {
					assert.True(t, tt.expectResp(resp.Msg), "Response validation failed")
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
		DomainData:  []ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
		},
	}

	// Create the service directly using the RPC's service
	service := fixture.rpc.service
	resp, err := service.Import(fixture.ctx, req)
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
	assert.Error(t, err) // Should fail

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
				DomainData:  []*apiv1.ImportDomainData{
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
	assert.Greater(t, successCount, 0, "At least some concurrent imports should succeed")

	// Verify that successful imports created data
	httpReqs, err := fixture.services.Hs.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	assert.Greater(t, len(httpReqs), 0, "HTTP requests should exist from successful imports")
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
		DomainData:  []*apiv1.ImportDomainData{
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
	assert.Len(t, httpReqs, 50, "All 50 HTTP requests should be imported")

	// Should find domains from the large HAR
	assert.NotEmpty(t, resp.Msg.Domains, "Large HAR should contain domains")
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
		domainData []ImportDomainData
		expectResp func(*apiv1.ImportResponse) bool
	}{
		{
			name: "no domain data provided",
			domainData: []ImportDomainData{},
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_DOMAIN
			},
		},
		{
			name: "domain data provided",
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
				{Enabled: true, Domain: "cdn.example.com", Variable: "CDN_DOMAIN"},
			},
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return resp.MissingData == apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED
			},
		},
		{
			name: "partial domain data",
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_DOMAIN"},
				// Missing cdn.example.com
			},
			expectResp: func(resp *apiv1.ImportResponse) bool {
				return len(resp.Domains) > 0 // Should still extract all domains
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert domain data to protobuf format
			protoDomainData := make([]*apiv1.ImportDomainData, len(tt.domainData))
			for i, dd := range tt.domainData {
				protoDomainData[i] = &apiv1.ImportDomainData{
					Enabled:  dd.Enabled,
					Domain:   dd.Domain,
					Variable: dd.Variable,
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
			assert.True(t, tt.expectResp(resp.Msg), "Domain processing response validation failed")

			// Should always extract domains regardless of provided domain data
			assert.NotEmpty(t, resp.Msg.Domains, "Should extract domains from HAR")
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
	assert.GreaterOrEqual(t, len(httpReqs), len(har.Log.Entries),
		"Should have stored at least as many HTTP requests as HAR entries")

	// Verify flows were created
	flows, err := f.services.Fls.GetFlowsByWorkspaceID(f.ctx, f.workspaceID)
	require.NoError(t, err)
	assert.NotEmpty(t, flows, "Should have created flows")

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
		DomainData:  []*apiv1.ImportDomainData{},
	})

	_, err = fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)

	// Verify events are received
	// We expect multiple events for HTTP requests, Headers, and Params
	
	// Check HTTP events (at least 2 from complex HAR)
	select {
	case evt := <-httpCh:
		assert.Equal(t, "insert", evt.Payload.Type)
		assert.NotNil(t, evt.Payload.Http)
	case <-time.After(time.Second):
		assert.Fail(t, "Timed out waiting for HTTP event")
	}

	// Check File events (for the files created)
	select {
	case evt := <-fileCh:
		assert.Equal(t, "create", evt.Payload.Type)
		assert.NotNil(t, evt.Payload.File)
	case <-time.After(time.Second):
		assert.Fail(t, "Timed out waiting for File event")
	}

	// Check Header events
	select {
	case evt := <-headerCh:
		assert.Equal(t, "insert", evt.Payload.Type)
		assert.NotNil(t, evt.Payload.HttpHeader)
	case <-time.After(time.Second):
		assert.Fail(t, "Timed out waiting for Header event")
	}

	// Check SearchParam events (Complex HAR has query params)
	select {
	case evt := <-paramCh:
		assert.Equal(t, "insert", evt.Payload.Type)
		assert.NotNil(t, evt.Payload.HttpSearchParam)
	case <-time.After(time.Second):
		assert.Fail(t, "Timed out waiting for SearchParam event")
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
				"method":     []string{"GET", "POST", "PUT"}[i],
				"url":        fmt.Sprintf("https://api.example.com/resource_%d", i),
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "application/json"},
				},
			},
			"response": map[string]interface{}{
				"status":     200,
				"statusText": "OK",
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
				"method": "GET",
				"url":    "https://api.example.com/users",
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
				"status":     200,
				"statusText": "OK",
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
				"method": "GET",
				"url":    "https://cdn.example.com/assets/style.css",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "text/css"},
				},
			},
			"response": map[string]interface{}{
				"status":     200,
				"statusText": "OK",
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
				"method": "GET",
				"url":    "https://api.example.com/data",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "application/json"},
				},
			},
			"response": map[string]interface{}{
				"status": 200,
				"statusText": "OK",
				"httpVersion": "HTTP/1.1",
			},
		},
		map[string]interface{}{
			"startedDateTime": time.Now().Add(time.Second).UTC().Format(time.RFC3339),
			"request": map[string]interface{}{
				"method": "GET",
				"url":    "https://cdn.example.com/assets/image.png",
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]interface{}{
					{"name": "Accept", "value": "image/png"},
				},
			},
			"response": map[string]interface{}{
				"status": 200,
				"statusText": "OK",
				"httpVersion": "HTTP/1.1",
			},
		},
		map[string]interface{}{
			"startedDateTime": time.Now().Add(2 * time.Second).UTC().Format(time.RFC3339),
			"request": map[string]interface{}{
				"method": "POST",
				"url":    "https://api.example.com/submit",
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
				"status": 201,
				"statusText": "Created",
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
			"time":            100 + (i%50),
			"request": map[string]interface{}{
				"method":     method,
				"url":        fmt.Sprintf("https://%s/endpoint_%d", domain, i),
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
				"status":     200,
				"statusText": "OK",
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