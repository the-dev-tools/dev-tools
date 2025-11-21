package rexportv2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
)

// mockExporter is a mock implementation of the Exporter interface
type mockExporter struct {
	ExportWorkspaceDataFunc func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error)
	ExportToYAMLFunc        func(ctx context.Context, data *WorkspaceExportData, simplified bool) ([]byte, error)
	ExportToCurlFunc        func(ctx context.Context, data *WorkspaceExportData, exampleIDs []idwrap.IDWrap) (string, error)
}

func (m *mockExporter) ExportWorkspaceData(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
	if m.ExportWorkspaceDataFunc != nil {
		return m.ExportWorkspaceDataFunc(ctx, workspaceID, filter)
	}
	return &WorkspaceExportData{}, nil
}

func (m *mockExporter) ExportToYAML(ctx context.Context, data *WorkspaceExportData, simplified bool) ([]byte, error) {
	if m.ExportToYAMLFunc != nil {
		return m.ExportToYAMLFunc(ctx, data, simplified)
	}
	return []byte("yaml data"), nil
}

func (m *mockExporter) ExportToCurl(ctx context.Context, data *WorkspaceExportData, exampleIDs []idwrap.IDWrap) (string, error) {
	if m.ExportToCurlFunc != nil {
		return m.ExportToCurlFunc(ctx, data, exampleIDs)
	}
	return "curl command", nil
}

// mockValidator is a mock implementation of the Validator interface
type mockValidator struct {
	ValidateExportRequestFunc  func(ctx context.Context, req *ExportRequest) error
	ValidateWorkspaceAccessFunc func(ctx context.Context, workspaceID idwrap.IDWrap) error
	ValidateExportFilterFunc    func(ctx context.Context, filter ExportFilter) error
}

func (m *mockValidator) ValidateExportRequest(ctx context.Context, req *ExportRequest) error {
	if m.ValidateExportRequestFunc != nil {
		return m.ValidateExportRequestFunc(ctx, req)
	}
	return nil
}

func (m *mockValidator) ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	if m.ValidateWorkspaceAccessFunc != nil {
		return m.ValidateWorkspaceAccessFunc(ctx, workspaceID)
	}
	return nil
}

func (m *mockValidator) ValidateExportFilter(ctx context.Context, filter ExportFilter) error {
	if m.ValidateExportFilterFunc != nil {
		return m.ValidateExportFilterFunc(ctx, filter)
	}
	return nil
}

// mockStorage is a mock implementation of the Storage interface
type mockStorage struct {
	GetWorkspaceFunc  func(ctx context.Context, workspaceID idwrap.IDWrap) (*WorkspaceInfo, error)
	GetFlowsFunc      func(ctx context.Context, workspaceID idwrap.IDWrap, flowIDs []idwrap.IDWrap) ([]*FlowData, error)
	GetHTTPRequestsFunc func(ctx context.Context, workspaceID idwrap.IDWrap, exampleIDs []idwrap.IDWrap) ([]*HTTPData, error)
	GetFilesFunc      func(ctx context.Context, workspaceID idwrap.IDWrap, fileIDs []idwrap.IDWrap) ([]*FileData, error)
}

func (m *mockStorage) GetWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) (*WorkspaceInfo, error) {
	if m.GetWorkspaceFunc != nil {
		return m.GetWorkspaceFunc(ctx, workspaceID)
	}
	return &WorkspaceInfo{ID: workspaceID, Name: "Test Workspace"}, nil
}

func (m *mockStorage) GetFlows(ctx context.Context, workspaceID idwrap.IDWrap, flowIDs []idwrap.IDWrap) ([]*FlowData, error) {
	if m.GetFlowsFunc != nil {
		return m.GetFlowsFunc(ctx, workspaceID, flowIDs)
	}
	return []*FlowData{}, nil
}

func (m *mockStorage) GetHTTPRequests(ctx context.Context, workspaceID idwrap.IDWrap, exampleIDs []idwrap.IDWrap) ([]*HTTPData, error) {
	if m.GetHTTPRequestsFunc != nil {
		return m.GetHTTPRequestsFunc(ctx, workspaceID, exampleIDs)
	}
	return []*HTTPData{}, nil
}

func (m *mockStorage) GetFiles(ctx context.Context, workspaceID idwrap.IDWrap, fileIDs []idwrap.IDWrap) ([]*FileData, error) {
	if m.GetFilesFunc != nil {
		return m.GetFilesFunc(ctx, workspaceID, fileIDs)
	}
	return []*FileData{}, nil
}

// TestNewService tests the service constructor
func TestNewService(t *testing.T) {
	exporter := &mockExporter{}
	validator := &mockValidator{}
	storage := &mockStorage{}

	tests := []struct {
		name     string
		exporter Exporter
		validator Validator
		storage   Storage
	}{
		{
			name:      "minimal dependencies",
			exporter:  exporter,
			validator: validator,
			storage:   storage,
		},
		{
			name:      "nil exporter should still create service",
			exporter:  nil,
			validator: validator,
			storage:   storage,
		},
		{
			name:      "nil validator should still create service",
			exporter:  exporter,
			validator: nil,
			storage:   storage,
		},
		{
			name:      "nil storage should still create service",
			exporter:  exporter,
			validator: validator,
			storage:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.exporter, tt.validator, tt.storage)

			require.NotNil(t, service)
			assert.Equal(t, tt.exporter, service.exporter)
			assert.Equal(t, tt.validator, service.validator)
			assert.Equal(t, tt.storage, service.storage)

			// Check default logger
			assert.NotNil(t, service.logger)
		})
	}
}

// TestService_Export_Success tests successful export operation
func TestService_Export_Success(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	// Create test data
	exportData := &WorkspaceExportData{
		Workspace: &WorkspaceInfo{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		Flows: []*FlowData{
			{
				ID:   flowID,
				Name: "Test Flow",
			},
		},
		HTTPRequests: []*HTTPData{
			{
				ID:  exampleID,
				Url: "https://api.example.com",
			},
		},
	}

	exporter := &mockExporter{
		ExportWorkspaceDataFunc: func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
			return exportData, nil
		},
		ExportToYAMLFunc: func(ctx context.Context, data *WorkspaceExportData, simplified bool) ([]byte, error) {
			return []byte("workspace_name: Test Workspace\nflows:\n  - name: Test Flow"), nil
		},
	}

	validator := &mockValidator{
		ValidateExportRequestFunc:  func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error { return nil },
		ValidateExportFilterFunc:    func(ctx context.Context, filter ExportFilter) error { return nil },
	}

	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: workspaceID,
		FileIDs:     []idwrap.IDWrap{flowID}, // Use flowID as fileID for now
		Format:      ExportFormat_YAML,
		Simplified:  false,
	}

	resp, err := service.Export(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Test Workspace.yaml", resp.Name)
	assert.NotEmpty(t, resp.Data)
}

// TestService_Export_Simplified tests simplified export
func TestService_Export_Simplified(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	exportData := &WorkspaceExportData{
		Workspace: &WorkspaceInfo{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
	}

	exporter := &mockExporter{
		ExportWorkspaceDataFunc: func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
			return exportData, nil
		},
		ExportToYAMLFunc: func(ctx context.Context, data *WorkspaceExportData, simplified bool) ([]byte, error) {
			return []byte("simplified: data"), nil
		},
	}

	validator := &mockValidator{
		ValidateExportRequestFunc:  func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error { return nil },
		ValidateExportFilterFunc:    func(ctx context.Context, filter ExportFilter) error { return nil },
	}

	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: workspaceID,
		Format:     ExportFormat_YAML,
		Simplified: true,
	}

	resp, err := service.Export(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Test Workspace_simplified.yaml", resp.Name)
}

// TestService_Export_CurlFormat tests export in cURL format
func TestService_Export_CurlFormat(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	exportData := &WorkspaceExportData{
		Workspace: &WorkspaceInfo{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		HTTPRequests: []*HTTPData{
			{
				ID:  exampleID,
				Url: "https://api.example.com",
			},
		},
	}

	exporter := &mockExporter{
		ExportWorkspaceDataFunc: func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
			return exportData, nil
		},
		ExportToCurlFunc: func(ctx context.Context, data *WorkspaceExportData, exampleIDs []idwrap.IDWrap) (string, error) {
			return "curl 'https://api.example.com'", nil
		},
	}

	validator := &mockValidator{
		ValidateExportRequestFunc:  func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error { return nil },
		ValidateExportFilterFunc:    func(ctx context.Context, filter ExportFilter) error { return nil },
	}

	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: workspaceID,
		FileIDs:     []idwrap.IDWrap{}, // Empty for cURL format tests
		Format:      ExportFormat_CURL,
	}

	resp, err := service.Export(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Test Workspace_curl.sh", resp.Name)
	assert.Contains(t, string(resp.Data), "curl '")
}

// TestService_Export_ValidationError tests export with validation errors
func TestService_Export_ValidationError(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	exporter := &mockExporter{}
	validator := &mockValidator{
		ValidateExportRequestFunc: func(ctx context.Context, req *ExportRequest) error {
			return NewValidationError("workspace_id", "invalid")
		},
	}
	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: workspaceID,
	}

	resp, err := service.Export(ctx, req)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
	require.Nil(t, resp)
}

// TestService_Export_WorkspaceAccessError tests export with workspace access error
func TestService_Export_WorkspaceAccessError(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	exporter := &mockExporter{}
	validator := &mockValidator{
		ValidateExportRequestFunc: func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error {
			return ErrPermissionDenied
		},
	}
	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: workspaceID,
	}

	resp, err := service.Export(ctx, req)

	require.Error(t, err)
	assert.Equal(t, ErrPermissionDenied, err)
	require.Nil(t, resp)
}

// TestService_Export_ExporterError tests export with exporter errors
func TestService_Export_ExporterError(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	exporter := &mockExporter{
		ExportWorkspaceDataFunc: func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
			return nil, errors.New("exporter failed")
		},
	}
	validator := &mockValidator{
		ValidateExportRequestFunc:  func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error { return nil },
		ValidateExportFilterFunc:    func(ctx context.Context, filter ExportFilter) error { return nil },
	}
	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: workspaceID,
		Format:     ExportFormat_YAML,
	}

	resp, err := service.Export(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace data export failed: exporter failed")
	require.Nil(t, resp)
}

// TestService_Export_UnsupportedFormat tests export with unsupported format
func TestService_Export_UnsupportedFormat(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	exporter := &mockExporter{
		ExportWorkspaceDataFunc: func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
			return &WorkspaceExportData{}, nil
		},
	}
	validator := &mockValidator{
		ValidateExportRequestFunc:  func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error { return nil },
		ValidateExportFilterFunc:    func(ctx context.Context, filter ExportFilter) error { return nil },
	}
	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: workspaceID,
		Format:     ExportFormat("UNSUPPORTED"), // Unsupported format
	}

	resp, err := service.Export(ctx, req)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
	assert.Contains(t, err.Error(), "unsupported export format")
	require.Nil(t, resp)
}

// TestService_Export_ContextCancellation tests export with context cancellation
func TestService_Export_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to be cancelled
	time.Sleep(1 * time.Millisecond)

	exporter := &mockExporter{}
	validator := &mockValidator{
		ValidateExportRequestFunc: func(ctx context.Context, req *ExportRequest) error {
			return ctx.Err() // Return context error
		},
	}
	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportRequest{
		WorkspaceID: idwrap.NewNow(),
	}

	// Call Export with cancelled context
	resp, err := service.Export(ctx, req)

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestService_ExportCurl_Success tests successful cURL export
func TestService_ExportCurl_Success(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	exportData := &WorkspaceExportData{
		Workspace: &WorkspaceInfo{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		HTTPRequests: []*HTTPData{
			{
				ID:  exampleID,
				Url: "https://api.example.com",
			},
		},
	}

	exporter := &mockExporter{
		ExportWorkspaceDataFunc: func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
			return exportData, nil
		},
		ExportToCurlFunc: func(ctx context.Context, data *WorkspaceExportData, exampleIDs []idwrap.IDWrap) (string, error) {
			return "curl 'https://api.example.com'", nil
		},
	}

	validator := &mockValidator{
		ValidateExportRequestFunc:  func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error { return nil },
	}

	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportCurlRequest{
		WorkspaceID: workspaceID,
		HTTPIDs:     []idwrap.IDWrap{exampleID}, // Use exampleID as httpID for now
	}

	resp, err := service.ExportCurl(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.Data, "curl '")
}

// TestService_ExportCurl_ValidationError tests cURL export with validation errors
func TestService_ExportCurl_ValidationError(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	exporter := &mockExporter{}
	validator := &mockValidator{
		ValidateExportRequestFunc: func(ctx context.Context, req *ExportRequest) error {
			return NewValidationError("workspace_id", "invalid")
		},
	}
	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportCurlRequest{
		WorkspaceID: workspaceID,
	}

	resp, err := service.ExportCurl(ctx, req)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
	require.Nil(t, resp)
}

// TestService_ExportCurl_ExporterError tests cURL export with exporter errors
func TestService_ExportCurl_ExporterError(t *testing.T) {
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	exporter := &mockExporter{
		ExportWorkspaceDataFunc: func(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
			return nil, errors.New("exporter failed")
		},
	}
	validator := &mockValidator{
		ValidateExportRequestFunc:  func(ctx context.Context, req *ExportRequest) error { return nil },
		ValidateWorkspaceAccessFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error { return nil },
	}
	storage := &mockStorage{}

	service := NewService(exporter, validator, storage)

	req := &ExportCurlRequest{
		WorkspaceID: workspaceID,
	}

	// Call ExportCurl with mock exporter error
	resp, err := service.ExportCurl(ctx, req)

	// Verify error
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "workspace data export failed: exporter failed")
}


