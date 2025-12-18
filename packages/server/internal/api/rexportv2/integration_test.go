package rexportv2

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"the-dev-tools/server/pkg/service/sflow"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rimportv2"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/shttp"

	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	exportv1 "the-dev-tools/spec/dist/buf/go/api/export/v1"

	"connectrpc.com/connect"
)

// integrationTestFixture represents the complete test setup for integration testing
type integrationTestFixture struct {
	ctx         context.Context
	base        *testutil.BaseDBQueries
	services    BaseTestServices
	rpc         *ExportV2RPC
	userID      idwrap.IDWrap
	workspaceID idwrap.IDWrap
	logger      *slog.Logger
}

// BaseTestServices wraps the testutil services for easier access
type BaseTestServices struct {
	Us          suser.UserService
	Ws          sworkspace.WorkspaceService
	Wus         sworkspacesusers.WorkspaceUserService
	Hs          shttp.HTTPService
	Fs          sflow.FlowService
	FileService sfile.FileService

	// Child entity services
	HttpHeaderService         shttp.HttpHeaderService
	HttpSearchParamService    *shttp.HttpSearchParamService
	HttpBodyFormService       *shttp.HttpBodyFormService
	HttpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService
	BodyService               *shttp.HttpBodyRawService
	HttpAssertService         *shttp.HttpAssertService

	// Flow related services
	NodeService        *sflow.NodeService
	NodeRequestService *sflow.NodeRequestService
	NodeNoopService    *sflow.NodeNoopService
	EdgeService        *sflow.EdgeService
	EnvService         senv.EnvironmentService
	VarService         senv.VariableService
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

	// Create additional services needed for export
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
	nodeNoopService := sflow.NewNodeNoopService(base.Queries)
	edgeService := sflow.NewEdgeService(base.Queries)
	envService := senv.NewEnvironmentService(base.Queries, logger)
	varService := senv.NewVariableService(base.Queries, logger)

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
	workspace := &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Integration Test Workspace",
	}
	err = baseServices.Ws.Create(ctx, workspace)
	require.NoError(t, err)

	// Add user to workspace
	err = baseServices.Wus.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Role:        mworkspaceuser.RoleAdmin,
	})
	require.NoError(t, err)

	// Create authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Create RPC handler
	rpc := NewExportV2RPC(
		base.DB,
		base.Queries,
		baseServices.Ws,
		baseServices.Us,
		&httpService,
		&flowService,
		fileService,
		logger,
	)

	services := BaseTestServices{
		Us:                        baseServices.Us,
		Ws:                        baseServices.Ws,
		Wus:                       baseServices.Wus,
		Hs:                        httpService,
		Fs:                        flowService,
		FileService:               *fileService,
		HttpHeaderService:         httpHeaderService,
		HttpSearchParamService:    httpSearchParamService,
		HttpBodyFormService:       httpBodyFormService,
		HttpBodyUrlEncodedService: httpBodyUrlEncodedService,
		BodyService:               bodyService,
		HttpAssertService:         httpAssertService,
		NodeService:               &nodeService,
		NodeRequestService:        &nodeRequestService,
		NodeNoopService:           &nodeNoopService,
		EdgeService:               &edgeService,
		EnvService:                envService,
		VarService:                varService,
	}

	return &integrationTestFixture{
		ctx:         authedCtx,
		base:        base,
		services:    services,
		rpc:         rpc,
		userID:      userID,
		workspaceID: workspaceID,
		logger:      logger,
	}
}

// TestIntegration_ExportFullWorkflow tests the complete export workflow
func TestIntegration_ExportFullWorkflow(t *testing.T) {
	fixture := newIntegrationTestFixture(t)

	// Create test data
	_, _ = createComplexTestData(t, fixture)

	// Test full export
	resp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
	}))

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.Data)
	require.Contains(t, resp.Msg.Name, ".yaml")

	// Verify YAML structure (yamlflowsimplev2 format)
	var exportData map[string]interface{}
	err = yaml.Unmarshal(resp.Msg.Data, &exportData)
	require.NoError(t, err)

	// yamlflowsimplev2 format uses workspace_name and flows
	require.Contains(t, exportData, "workspace_name")
	require.Contains(t, exportData, "flows")
}

// TestIntegration_ExportCurlWorkflow tests the cURL export workflow
func TestIntegration_ExportCurlWorkflow(t *testing.T) {
	fixture := newIntegrationTestFixture(t)

	// Create test data
	_, exampleID := createComplexTestData(t, fixture)

	// Test cURL export
	resp, err := fixture.rpc.ExportCurl(fixture.ctx, connect.NewRequest(&exportv1.ExportCurlRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		HttpIds:     [][]byte{exampleID.Bytes()},
	}))

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.Data)

	// Verify cURL command structure
	curlData := resp.Msg.Data
	require.Contains(t, curlData, "curl")
	require.Contains(t, curlData, "https://api.example.com/test")
}

// TestIntegration_ExportWithFilters tests export with various filters
func TestIntegration_ExportWithFilters(t *testing.T) {
	fixture := newIntegrationTestFixture(t)

	// Create multiple flows and requests for filtering tests
	flowID1, _ := createComplexTestData(t, fixture)
	flowID2, _ := createIntegrationTestData(t, fixture)

	tests := []struct {
		name           string
		fileIDs        [][]byte // Use file IDs instead of flow IDs
		expectFlows    int
		expectRequests int
	}{
		{
			name:        "filter by single flow",
			fileIDs:     [][]byte{flowID1.Bytes()}, // Treat flow IDs as file IDs
			expectFlows: 1,
		},
		{
			name:        "filter by multiple flows",
			fileIDs:     [][]byte{flowID1.Bytes(), flowID2.Bytes()}, // Treat flow IDs as file IDs
			expectFlows: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
				WorkspaceId: fixture.workspaceID.Bytes(),
				FileIds:     tt.fileIDs,
			}))

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NotEmpty(t, resp.Msg.Data)

			var exportData map[string]interface{}
			err = yaml.Unmarshal(resp.Msg.Data, &exportData)
			require.NoError(t, err)

			// yamlflowsimplev2 format has flows array
			if flows, ok := exportData["flows"].([]interface{}); ok && tt.expectFlows > 0 {
				require.GreaterOrEqual(t, len(flows), tt.expectFlows)
			}
		})
	}
}

// TestIntegration_ImportExportRoundTrip tests import followed by export
func TestIntegration_ImportExportRoundTrip(t *testing.T) {
	fixture := newIntegrationTestFixture(t)

	// First, import some data using rimportv2
	importReq := &rimportv2.ImportRequest{
		WorkspaceID: fixture.workspaceID,
		Name:        "Round Trip Test",
		Data:        createTestHARData(t),
		DomainData:  []rimportv2.ImportDomainData{},
	}

	// Create import service (simplified for this test)
	importService := createImportService(t, fixture)
	importResp, err := importService.Import(fixture.ctx, importReq)
	require.NoError(t, err)
	require.NotNil(t, importResp)

	// Now export the imported data
	exportResp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
	}))

	require.NoError(t, err)
	require.NotNil(t, exportResp)
	require.NotEmpty(t, exportResp.Msg.Data)

	// Verify the exported data contains what was imported
	var exportData map[string]interface{}
	err = yaml.Unmarshal(exportResp.Msg.Data, &exportData)
	require.NoError(t, err)

	// yamlflowsimplev2 format uses workspace_name and flows
	require.Contains(t, exportData, "workspace_name")
	require.Contains(t, exportData, "flows")
}

// TestIntegration_ErrorHandling tests error handling in realistic scenarios
func TestIntegration_ErrorHandling(t *testing.T) {
	fixture := newIntegrationTestFixture(t)

	t.Run("non-existent workspace", func(t *testing.T) {
		nonExistentID := idwrap.NewNow()
		resp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
			WorkspaceId: nonExistentID.Bytes(),
		}))

		require.Error(t, err)
		require.Nil(t, resp)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok)
		require.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("invalid workspace ID", func(t *testing.T) {
		resp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
			WorkspaceId: []byte{}, // Invalid empty bytes
		}))

		require.Error(t, err)
		require.Nil(t, resp)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok)
		require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("filtering non-existent flows", func(t *testing.T) {
		nonExistentFlowID := idwrap.NewNow()
		resp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
			WorkspaceId: fixture.workspaceID.Bytes(),
			FileIds:     [][]byte{nonExistentFlowID.Bytes()},
		}))

		// When filtering by specific flow IDs that don't exist, expect an error
		require.Error(t, err)
		require.Nil(t, resp)

		connectErr, ok := err.(*connect.Error)
		require.True(t, ok)
		require.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

// TestIntegration_PerformanceWithLargeDataset tests performance with larger datasets
func TestIntegration_PerformanceWithLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Create a larger dataset
	const numFlows = 10
	const numRequests = 50

	for i := 0; i < numFlows; i++ {
		flowID := idwrap.NewNow()
		flow := mflow.Flow{
			ID:          flowID,
			WorkspaceID: fixture.workspaceID,
			Name:        "Performance Test Flow",
		}
		err := fixture.services.Fs.CreateFlow(fixture.ctx, flow)
		require.NoError(t, err)
	}

	for i := 0; i < numRequests; i++ {
		exampleID := idwrap.NewNow()
		httpRequest := &mhttp.HTTP{
			ID:          exampleID,
			WorkspaceID: fixture.workspaceID,
			Url:         "https://api.example.com/performance-test",
			Method:      "POST",
			Name:        "Performance Test Request",
		}
		err := fixture.services.Hs.Create(fixture.ctx, httpRequest)
		require.NoError(t, err)
	}

	// Test export performance
	resp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
	}))

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.Data)

	// Verify the data is comprehensive
	var exportData map[string]interface{}
	err = yaml.Unmarshal(resp.Msg.Data, &exportData)
	require.NoError(t, err)

	// yamlflowsimplev2 format uses workspace_name and flows
	require.Contains(t, exportData, "workspace_name")
	require.Contains(t, exportData, "flows")

	flows := exportData["flows"].([]interface{})
	require.Len(t, flows, numFlows)
}

// TestIntegration_ConcurrentExports tests concurrent export operations
func TestIntegration_ConcurrentExports(t *testing.T) {
	fixture := newIntegrationTestFixture(t)

	// Create test data
	_, _ = createComplexTestData(t, fixture)

	const numGoroutines = 5
	done := make(chan bool, numGoroutines)

	// Launch concurrent exports
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer func() { done <- true }()

			resp, err := fixture.rpc.Export(fixture.ctx, connect.NewRequest(&exportv1.ExportRequest{
				WorkspaceId: fixture.workspaceID.Bytes(),
			}))

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NotEmpty(t, resp.Msg.Data)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// OK
		case <-fixture.ctx.Done():
			t.Fatal("timeout waiting for goroutines")
		}
	}
}

// createComplexTestData creates comprehensive test data for integration testing
func createComplexTestData(t *testing.T, fixture *integrationTestFixture) (idwrap.IDWrap, idwrap.IDWrap) {
	t.Helper()

	flowID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	fileID := idwrap.NewNow()

	// Create flow
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: fixture.workspaceID,
		Name:        "Integration Test Flow",
	}
	err := fixture.services.Fs.CreateFlow(fixture.ctx, flow)
	require.NoError(t, err)

	// Create HTTP request
	httpRequest := mhttp.HTTP{
		ID:          exampleID,
		WorkspaceID: fixture.workspaceID,
		Url:         "https://api.example.com/test",
		Method:      "POST",
		Name:        "Integration Test Request",
	}
	err = fixture.services.Hs.Create(fixture.ctx, &httpRequest)
	require.NoError(t, err)

	// Create file
	file := mfile.File{
		ID:          fileID,
		WorkspaceID: fixture.workspaceID,
		Name:        "integration-test.txt",
		ContentType: mfile.ContentTypeFolder,
		Order:       0,
		UpdatedAt:   time.Now(),
	}
	err = fixture.services.FileService.CreateFile(fixture.ctx, &file)
	require.NoError(t, err)

	return flowID, exampleID
}

// createIntegrationTestData creates additional test data for filtering tests
func createIntegrationTestData(t *testing.T, fixture *integrationTestFixture) (idwrap.IDWrap, idwrap.IDWrap) {
	t.Helper()

	flowID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	// Create additional flow
	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: fixture.workspaceID,
		Name:        "Additional Test Flow",
	}
	err := fixture.services.Fs.CreateFlow(fixture.ctx, flow)
	require.NoError(t, err)

	// Create additional HTTP request
	httpRequest := mhttp.HTTP{
		ID:          exampleID,
		WorkspaceID: fixture.workspaceID,
		Url:         "https://api.example.com/additional",
		Method:      "GET",
		Name:        "Additional Test Request",
	}
	err = fixture.services.Hs.Create(fixture.ctx, &httpRequest)
	require.NoError(t, err)

	return flowID, exampleID
}

// createTestHARData creates sample HAR data for import testing
func createTestHARData(t *testing.T) []byte {
	t.Helper()

	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name":    "test-creator",
				"version": "1.0",
			},
			"entries": []map[string]interface{}{
				{
					"request": map[string]interface{}{
						"method":      "POST",
						"url":         "https://api.example.com/har-test",
						"httpVersion": "HTTP/1.1",
						"headers": []map[string]interface{}{
							{
								"name":  "Content-Type",
								"value": "application/json",
							},
						},
						"postData": map[string]interface{}{
							"mimeType": "application/json",
							"text":     `{"test": "har-data"}`,
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

	data, err := json.Marshal(harData)
	require.NoError(t, err)
	return data
}

// createImportService creates a mock import service for round-trip testing
func createImportService(t *testing.T, fixture *integrationTestFixture) *rimportv2.Service {
	t.Helper()

	importer := rimportv2.NewImporter(
		fixture.base.DB,
		&fixture.services.Hs,
		&fixture.services.Fs,
		&fixture.services.FileService,
		fixture.services.HttpHeaderService,
		fixture.services.HttpSearchParamService,
		fixture.services.HttpBodyFormService,
		fixture.services.HttpBodyUrlEncodedService,
		fixture.services.BodyService,
		fixture.services.HttpAssertService,
		fixture.services.NodeService,
		fixture.services.NodeRequestService,
		fixture.services.NodeNoopService,
		fixture.services.EdgeService,
		fixture.services.EnvService,
		fixture.services.VarService,
	)
	validator := rimportv2.NewValidator(&fixture.services.Us)

	return rimportv2.NewService(importer, validator, rimportv2.WithLogger(fixture.logger))
}
