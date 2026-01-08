package rexportv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
)

// TestNewExporter tests the exporter constructor
func TestNewExporter(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	logger := base.Logger()
	services := base.GetBaseServices()

	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.NewFlowService(base.Queries)
	fileService := sfile.New(base.Queries, logger)
	workspaceService := services.WorkspaceService

	// Create IOWorkspaceService
	ioWorkspaceService := ioworkspace.New(base.Queries, logger)

	exporter := NewExporter(&httpService, &flowService, fileService, ioWorkspaceService)
	storage := NewStorage(&workspaceService, &httpService, &flowService, fileService)
	exporter.SetStorage(storage)

	require.NotNil(t, exporter)
	require.NotNil(t, exporter.httpService)
	require.NotNil(t, exporter.flowService)
	require.NotNil(t, exporter.fileService)
	require.NotNil(t, exporter.ioWorkspaceService)
	require.NotNil(t, exporter.storage)
}

// TestDefaultExporter_ExportWorkspaceData_Success tests successful workspace data export
func TestDefaultExporter_ExportWorkspaceData_Success(t *testing.T) {
	ctx := context.Background()
	exporter, workspaceID, flowID, exampleID, _ := setupExporterWithTestData(t, ctx)

	filter := ExportFilter{
		FileIDs:    []idwrap.IDWrap{flowID},    // Use flowID as fileID for now
		HTTPIDs:    []idwrap.IDWrap{exampleID}, // Use exampleID as httpID for now
		Format:     ExportFormat_YAML,
		Simplified: false,
	}

	data, err := exporter.ExportWorkspaceData(ctx, workspaceID, filter)

	require.NoError(t, err)
	require.NotNil(t, data)
	require.NotNil(t, data.Workspace)
	require.Equal(t, workspaceID, data.Workspace.ID)
	// Note: flows, HTTP requests, and files may be empty since we only created a workspace
	// The important thing is that the workspace is retrieved successfully
}

// TestDefaultExporter_ExportWorkspaceData_NoFilters tests export without filters
func TestDefaultExporter_ExportWorkspaceData_NoFilters(t *testing.T) {
	ctx := context.Background()
	exporter, workspaceID, _, _, _ := setupExporterWithTestData(t, ctx)

	filter := ExportFilter{
		Format:     ExportFormat_YAML,
		Simplified: false,
	}

	data, err := exporter.ExportWorkspaceData(ctx, workspaceID, filter)

	require.NoError(t, err)
	require.NotNil(t, data)
	require.NotNil(t, data.Workspace)
}

// TestDefaultExporter_ExportWorkspaceData_WorkspaceNotFound tests with non-existent workspace
func TestDefaultExporter_ExportWorkspaceData_WorkspaceNotFound(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	filter := ExportFilter{
		Format: ExportFormat_YAML,
	}

	nonExistentID := idwrap.NewNow()
	data, err := exporter.ExportWorkspaceData(ctx, nonExistentID, filter)

	require.Error(t, err)
	require.Nil(t, data)
	// Check that the error is properly wrapped and contains the expected message
	require.Contains(t, err.Error(), "failed to get workspace")
	require.Contains(t, err.Error(), "sql: no rows in result set")
}

// TestDefaultExporter_ExportToYAML_WithEnvironments tests YAML export with environments
func TestDefaultExporter_ExportToYAML_WithEnvironments(t *testing.T) {
	ctx := context.Background()

	// Manual setup to access DB
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	logger := base.Logger()
	services := base.GetBaseServices()

	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.NewFlowService(base.Queries)
	fileService := sfile.New(base.Queries, logger)
	workspaceService := services.WorkspaceService
	ioWorkspaceService := ioworkspace.New(base.Queries, logger)

	exporter := NewExporter(&httpService, &flowService, fileService, ioWorkspaceService)
	storage := NewStorage(&workspaceService, &httpService, &flowService, fileService)
	exporter.SetStorage(storage)

	// Create test data
	workspaceID := idwrap.NewNow()
	envID := idwrap.NewNow()

	// Create workspace
	workspace := &mworkspace.Workspace{
		ID:        workspaceID,
		Name:      "Test Workspace",
		Updated:   dbtime.DBNow(),
		ActiveEnv: envID,
		GlobalEnv: envID,
	}
	err := services.WorkspaceService.Create(ctx, workspace)
	require.NoError(t, err)

	// Create environment
	envService := senv.NewEnvironmentService(base.Queries, logger)
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	err = envService.CreateEnvironment(ctx, &env)
	require.NoError(t, err)

	// Create variable
	varService := senv.NewVariableService(base.Queries, logger)
	err = varService.Create(ctx, menv.Variable{
		ID:      idwrap.NewNow(),
		EnvID:   envID,
		VarKey:  "exported_var",
		Value:   "exported_value",
		Enabled: true,
	})
	require.NoError(t, err)

	data := &WorkspaceExportData{
		Workspace: &WorkspaceInfo{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
	}

	yamlData, err := exporter.ExportToYAML(ctx, data, false, nil)
	require.NoError(t, err)
	require.NotEmpty(t, yamlData)

	var parsed map[string]interface{}
	err = yaml.Unmarshal(yamlData, &parsed)
	require.NoError(t, err)

	// Check environments
	require.Contains(t, parsed, "environments")
	envsRaw, ok := parsed["environments"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, envsRaw)

	envMap, ok := envsRaw[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "default", envMap["name"])

	varsMap, ok := envMap["variables"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "exported_value", varsMap["exported_var"])
}

// TestDefaultExporter_ExportToYAML_Success tests successful YAML export
func TestDefaultExporter_ExportToYAML_Success(t *testing.T) {
	ctx := context.Background()
	exporter, workspaceID, _, _, _ := setupExporterWithTestData(t, ctx)

	data := &WorkspaceExportData{
		Workspace: &WorkspaceInfo{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
	}

	tests := []struct {
		name       string
		simplified bool
	}{
		{
			name:       "full export",
			simplified: false,
		},
		{
			name:       "simplified export",
			simplified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlData, err := exporter.ExportToYAML(ctx, data, tt.simplified, nil)

			require.NoError(t, err)
			require.NotEmpty(t, yamlData)

			// Verify YAML is valid
			var parsed map[string]interface{}
			err = yaml.Unmarshal(yamlData, &parsed)
			require.NoError(t, err)

			// Check that we have workspace_name (from yamlflowsimplev2 format)
			require.Contains(t, parsed, "workspace_name")
			require.Equal(t, "Test Workspace", parsed["workspace_name"])
		})
	}
}

// TestDefaultExporter_ExportToYAML_EmptyData tests YAML export with empty workspace (no HTTP/flows)
func TestDefaultExporter_ExportToYAML_EmptyData(t *testing.T) {
	ctx := context.Background()
	exporter, workspaceID, _, _, _ := setupExporterWithTestData(t, ctx)

	data := &WorkspaceExportData{
		Workspace: &WorkspaceInfo{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		Flows:        []*FlowData{},
		HTTPRequests: []*HTTPData{},
		Files:        []*FileData{},
	}

	yamlData, err := exporter.ExportToYAML(ctx, data, false, nil)

	require.NoError(t, err)
	require.NotEmpty(t, yamlData)

	var parsed map[string]interface{}
	err = yaml.Unmarshal(yamlData, &parsed)
	require.NoError(t, err)

	// Check workspace name from yamlflowsimplev2 format
	require.Equal(t, "Test Workspace", parsed["workspace_name"])
}

// TestDefaultExporter_ExportToYAML_NilWorkspace tests YAML export with nil workspace returns error
func TestDefaultExporter_ExportToYAML_NilWorkspace(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	data := &WorkspaceExportData{
		Workspace:    nil,
		Flows:        []*FlowData{},
		HTTPRequests: []*HTTPData{},
		Files:        []*FileData{},
	}

	_, err := exporter.ExportToYAML(ctx, data, false, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "workspace data is required")
}

// TestDefaultExporter_ExportToCurl_Success tests successful cURL export
func TestDefaultExporter_ExportToCurl_Success(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	exampleID := idwrap.NewNow()
	data := &WorkspaceExportData{
		HTTPRequests: []*HTTPData{
			{
				ID:     exampleID,
				Url:    "https://api.example.com/test",
				Method: "POST",
				Name:   "Test Request",
				Headers: map[string][]string{
					"Content-Type":  {"application/json"},
					"Authorization": {"Bearer token123"},
				},
				Body: `{"test": "data"}`,
			},
		},
	}

	exampleIDs := []idwrap.IDWrap{exampleID}
	curlData, err := exporter.ExportToCurl(ctx, data, exampleIDs)

	require.NoError(t, err)
	require.NotEmpty(t, curlData)

	// Verify cURL command structure
	require.Contains(t, curlData, "curl")
	require.Contains(t, curlData, "https://api.example.com/test")
	require.Contains(t, curlData, "POST")
	require.Contains(t, curlData, "Content-Type: application/json")
	require.Contains(t, curlData, "Authorization: Bearer token123")
	require.Contains(t, curlData, `{"test": "data"}`)
}

// TestDefaultExporter_ExportToCurl_MultipleRequests tests cURL export with multiple requests
func TestDefaultExporter_ExportToCurl_MultipleRequests(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	exampleID1 := idwrap.NewNow()
	exampleID2 := idwrap.NewNow()
	data := &WorkspaceExportData{
		HTTPRequests: []*HTTPData{
			{
				ID:     exampleID1,
				Url:    "https://api.example.com/test1",
				Method: "GET",
				Name:   "Test Request 1",
			},
			{
				ID:     exampleID2,
				Url:    "https://api.example.com/test2",
				Method: "POST",
				Name:   "Test Request 2",
				Body:   "test data",
			},
		},
	}

	exampleIDs := []idwrap.IDWrap{exampleID1, exampleID2}
	curlData, err := exporter.ExportToCurl(ctx, data, exampleIDs)

	require.NoError(t, err)
	require.NotEmpty(t, curlData)

	// Should contain both requests
	require.Contains(t, curlData, "https://api.example.com/test1")
	require.Contains(t, curlData, "https://api.example.com/test2")
	require.Contains(t, curlData, "GET")
	require.Contains(t, curlData, "POST")
}

// TestDefaultExporter_ExportToCurl_FilteredRequests tests cURL export with filtered requests
func TestDefaultExporter_ExportToCurl_FilteredRequests(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	exampleID1 := idwrap.NewNow()
	exampleID2 := idwrap.NewNow()
	exampleID3 := idwrap.NewNow()
	data := &WorkspaceExportData{
		HTTPRequests: []*HTTPData{
			{
				ID:     exampleID1,
				Url:    "https://api.example.com/test1",
				Method: "GET",
				Name:   "Test Request 1",
			},
			{
				ID:     exampleID2,
				Url:    "https://api.example.com/test2",
				Method: "POST",
				Name:   "Test Request 2",
			},
			{
				ID:     exampleID3,
				Url:    "https://api.example.com/test3",
				Method: "PUT",
				Name:   "Test Request 3",
			},
		},
	}

	// Only export requests 1 and 3
	exampleIDs := []idwrap.IDWrap{exampleID1, exampleID3}
	curlData, err := exporter.ExportToCurl(ctx, data, exampleIDs)

	require.NoError(t, err)
	require.NotEmpty(t, curlData)

	// Should contain only the requested requests
	require.Contains(t, curlData, "https://api.example.com/test1")
	require.Contains(t, curlData, "https://api.example.com/test3")
	require.NotContains(t, curlData, "https://api.example.com/test2")
	require.Contains(t, curlData, "GET")
	require.Contains(t, curlData, "PUT")
	require.NotContains(t, curlData, "POST")
}

// TestDefaultExporter_ExportToCurl_EmptyData tests cURL export with no requests
func TestDefaultExporter_ExportToCurl_EmptyData(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	data := &WorkspaceExportData{
		HTTPRequests: []*HTTPData{},
	}

	exampleIDs := []idwrap.IDWrap{}
	curlData, err := exporter.ExportToCurl(ctx, data, exampleIDs)

	require.NoError(t, err)
	require.Equal(t, "", curlData) // Empty string when no requests
}

// TestDefaultExporter_ExportToCurl_NoMatchingRequests tests cURL export with no matching requests
func TestDefaultExporter_ExportToCurl_NoMatchingRequests(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	exampleID := idwrap.NewNow()
	data := &WorkspaceExportData{
		HTTPRequests: []*HTTPData{
			{
				ID:     exampleID,
				Url:    "https://api.example.com/test",
				Method: "GET",
				Name:   "Test Request",
			},
		},
	}

	// Request a non-existent example ID
	nonExistentID := idwrap.NewNow()
	exampleIDs := []idwrap.IDWrap{nonExistentID}
	curlData, err := exporter.ExportToCurl(ctx, data, exampleIDs)

	require.NoError(t, err)
	require.Equal(t, "", curlData) // Empty string when no matching requests
}

// TestDefaultExporter_ContextCancellation tests exporter with context cancellation
func TestDefaultExporter_ContextCancellation(t *testing.T) {
	// Use background context for setup
	exporter := setupExporterWithoutData(t, context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	workspaceID := idwrap.NewNow()
	filter := ExportFilter{Format: ExportFormat_YAML}

	data, err := exporter.ExportWorkspaceData(ctx, workspaceID, filter)

	require.Error(t, err)
	require.Nil(t, data)
	require.Contains(t, err.Error(), "context")
}

// TestDefaultExporter_CurlCommandGeneration tests cURL command generation details
func TestDefaultExporter_CurlCommandGeneration(t *testing.T) {
	ctx := context.Background()
	exporter := setupExporterWithoutData(t, ctx)

	exampleID := idwrap.NewNow()
	data := &WorkspaceExportData{
		HTTPRequests: []*HTTPData{
			{
				ID:     exampleID,
				Url:    "https://api.example.com/test?param=value&other=123",
				Method: "POST",
				Name:   "Complex Request",
				Headers: map[string][]string{
					"Content-Type":    {"application/json"},
					"X-Custom-Header": {"custom-value"},
					"User-Agent":      {"TestAgent/1.0"},
				},
				Body: `{"name": "test", "data": [1, 2, 3]}`,
			},
		},
	}

	exampleIDs := []idwrap.IDWrap{exampleID}
	curlData, err := exporter.ExportToCurl(ctx, data, exampleIDs)

	require.NoError(t, err)
	require.NotEmpty(t, curlData)

	// Verify all components are present
	require.Contains(t, curlData, "curl")
	require.Contains(t, curlData, "-X POST")
	require.Contains(t, curlData, "https://api.example.com/test?param=value&other=123")
	require.Contains(t, curlData, `-H "Content-Type: application/json"`)
	require.Contains(t, curlData, `-H "X-Custom-Header: custom-value"`)
	require.Contains(t, curlData, `-H "User-Agent: TestAgent/1.0"`)
	require.Contains(t, curlData, `--data-raw '{"name": "test", "data": [1, 2, 3]}'`)
}

// setupExporterWithoutData creates an exporter without test data
func setupExporterWithoutData(t *testing.T, ctx context.Context) *SimpleExporter {
	t.Helper()

	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	logger := base.Logger()
	services := base.GetBaseServices()

	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.NewFlowService(base.Queries)
	fileService := sfile.New(base.Queries, logger)
	workspaceService := services.WorkspaceService

	// Create IOWorkspaceService
	ioWorkspaceService := ioworkspace.New(base.Queries, logger)

	exporter := NewExporter(&httpService, &flowService, fileService, ioWorkspaceService)
	storage := NewStorage(&workspaceService, &httpService, &flowService, fileService)
	exporter.SetStorage(storage)

	return exporter
}

// setupExporterWithTestData creates an exporter with comprehensive test data
func setupExporterWithTestData(t *testing.T, ctx context.Context) (*SimpleExporter, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap) {
	t.Helper()

	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	logger := base.Logger()
	services := base.GetBaseServices()

	httpService := shttp.New(base.Queries, logger)
	flowService := sflow.NewFlowService(base.Queries)
	fileService := sfile.New(base.Queries, logger)
	workspaceService := services.WorkspaceService

	// Create IOWorkspaceService
	ioWorkspaceService := ioworkspace.New(base.Queries, logger)

	exporter := NewExporter(&httpService, &flowService, fileService, ioWorkspaceService)
	storage := NewStorage(&workspaceService, &httpService, &flowService, fileService)
	exporter.SetStorage(storage)

	// Create test data
	workspaceID, flowID, exampleID, fileID := createExporterTestData(t, ctx, base)

	return exporter, workspaceID, flowID, exampleID, fileID
}

// createExporterTestData creates basic test data for exporter testing
func createExporterTestData(t *testing.T, ctx context.Context, base *testutil.BaseDBQueries) (idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap) {
	t.Helper()

	// Create IDs for test data
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	fileID := idwrap.NewNow()
	envID := idwrap.NewNow()

	// Create workspace in database
	services := base.GetBaseServices()

	workspace := &mworkspace.Workspace{
		ID:        workspaceID,
		Name:      "Test Workspace",
		Updated:   dbtime.DBNow(),
		ActiveEnv: envID,
		GlobalEnv: envID,
	}
	if err := services.WorkspaceService.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Create environment
	envService := senv.NewEnvironmentService(base.Queries, base.Logger())
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	if err := envService.CreateEnvironment(ctx, &env); err != nil {
		t.Fatalf("create environment: %v", err)
	}

	return workspaceID, flowID, exampleID, fileID
}
