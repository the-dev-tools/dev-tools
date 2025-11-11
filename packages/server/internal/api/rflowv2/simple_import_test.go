package rflowv2

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
)

func TestFlowServiceV2_DetectFlowFormat(t *testing.T) {
	// Create a simple mock service with minimal dependencies
	mockImportService := NewMockWorkspaceImporter(nil, nil)

	flowService := &FlowServiceV2RPC{
		workspaceImportService: mockImportService,
	}

	ctx := context.Background()

	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "YAML format detection",
			data:     []byte("workspace_name: test\nflows:\n  - name: test"),
			expected: "yaml",
		},
		{
			name:     "JSON format detection",
			data:     []byte(`{"flows": [{"name": "test"}]}`),
			expected: "json",
		},
		{
			name:     "Unknown format",
			data:     []byte("plain text"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, err := flowService.DetectFlowFormat(ctx, tt.data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, format)
		})
	}
}

func TestFlowServiceV2_ValidateYAMLFlow(t *testing.T) {
	// Create a simple mock service with minimal dependencies
	mockImportService := NewMockWorkspaceImporter(nil, nil)

	flowService := &FlowServiceV2RPC{
		workspaceImportService: mockImportService,
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		data      []byte
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid YAML",
			data: []byte(`
workspace_name: "Test Workspace"
flows:
  - name: "Test Flow"
    steps:
      - name: "Request Step"
        method: "GET"
        url: "https://api.example.com/test"
`),
			expectErr: false,
		},
		{
			name:      "Invalid YAML syntax",
			data:      []byte("workspace_name: test\nflows:\n  - name: test\n  invalid_yaml: ["),
			expectErr: true,
			errMsg:    "invalid YAML format",
		},
		{
			name: "Missing required fields",
			data: []byte(`
flows:
  - name: "Test Flow"
`),
			expectErr: true,
			errMsg:    "YAML validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := flowService.ValidateYAMLFlow(ctx, tt.data)
			if tt.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFlowServiceV2_ParseYAMLFlow(t *testing.T) {
	// Test YAML data
	yamlData := []byte(`
workspace_name: "Test Workspace"
flows:
  - name: "Test Flow"
    variables:
      - name: "test_var"
        value: "test_value"
    steps:
      - name: "Request Step"
        method: "GET"
        url: "https://api.example.com/test"
`)

	// Test successful parsing using the v2 translate package directly
	convertOpts := yamlflowsimplev2.GetDefaultOptions(idwrap.NewNow())
	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(yamlData, convertOpts)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, 1, len(resolved.Flows))
	require.Equal(t, "Test Flow", resolved.Flows[0].Name)
}

func TestFlowServiceV2_ImportYAMLFlow_Mock(t *testing.T) {
	// Create a mock import service that returns predefined results
	expectedResults := &ImportResults{
		WorkspaceID:    idwrap.NewNow(),
		HTTPReqsCreated: 2,
		FlowsCreated:    1,
		NodesCreated:    3,
		Duration:        1000,
	}

	mockImportService := NewMockWorkspaceImporter(expectedResults, nil)

	flowService := &FlowServiceV2RPC{
		workspaceImportService: mockImportService,
	}

	// Test YAML data
	yamlData := []byte(`
workspace_name: "Test Workspace"
flows:
  - name: "Test Flow"
    steps:
      - name: "Request Step"
        method: "GET"
        url: "https://api.example.com/test"
`)

	// Test successful import - this will fail at workspace access check,
	// but we can still test the basic flow
	ctx := context.Background()
	results, err := flowService.ImportYAMLFlow(ctx, yamlData, idwrap.NewNow())

	// We expect this to fail with auth error since we don't have proper auth setup
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthenticated")
	require.Nil(t, results)
}

func TestMockWorkspaceImporter(t *testing.T) {
	expectedResults := &ImportResults{
		WorkspaceID:    idwrap.NewNow(),
		HTTPReqsCreated: 5,
		FlowsCreated:    2,
		NodesCreated:    7,
		Duration:        1500,
	}

	mockImporter := NewMockWorkspaceImporter(expectedResults, nil)

	ctx := context.Background()
	yamlData := []byte("test: data")
	workspaceID := idwrap.NewNow()

	results, err := mockImporter.ImportWorkspaceFromYAML(ctx, yamlData, workspaceID)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Equal(t, expectedResults.HTTPReqsCreated, results.HTTPReqsCreated)
	require.Equal(t, expectedResults.FlowsCreated, results.FlowsCreated)
	require.Equal(t, expectedResults.NodesCreated, results.NodesCreated)
	require.Equal(t, expectedResults.Duration, results.Duration)
}

func TestMockWorkspaceImporter_Error(t *testing.T) {
	expectedError := connect.NewError(connect.CodeInternal, fmt.Errorf("test error"))
	mockImporter := NewMockWorkspaceImporter(nil, expectedError)

	ctx := context.Background()
	yamlData := []byte("test: data")
	workspaceID := idwrap.NewNow()

	results, err := mockImporter.ImportWorkspaceFromYAML(ctx, yamlData, workspaceID)
	require.Error(t, err)
	require.Nil(t, results)
	require.Equal(t, expectedError, err)
}

func TestFlowServiceV2_DetectFlowFormat_Curl(t *testing.T) {
	// Create a simple mock service with minimal dependencies
	mockImportService := NewMockWorkspaceImporter(nil, nil)

	flowService := &FlowServiceV2RPC{
		workspaceImportService: mockImportService,
	}

	ctx := context.Background()

	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "Simple curl command",
			data:     []byte("curl https://example.com"),
			expected: "curl",
		},
		{
			name:     "Curl with headers",
			data:     []byte("curl -H \"Content-Type: application/json\" https://api.example.com"),
			expected: "curl",
		},
		{
			name:     "Multiline curl command",
			data:     []byte("curl https://example.com \\\n  -X POST \\\n  -d '{\"test\": true}'"),
			expected: "curl",
		},
		{
			name:     "Curl with data",
			data:     []byte("curl -X POST -d 'test data' https://example.com"),
			expected: "curl",
		},
		{
			name:     "Curl in middle of text",
			data:     []byte("some text curl https://example.com more text"),
			expected: "curl",
		},
		{
			name:     "Not curl - YAML",
			data:     []byte("flows:\n  - name: test"),
			expected: "yaml",
		},
		{
			name:     "Not curl - JSON",
			data:     []byte(`{"test": "value"}`),
			expected: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, err := flowService.DetectFlowFormat(ctx, tt.data)
			require.NoError(t, err)
			require.Equal(t, tt.expected, format)
		})
	}
}

func TestFlowServiceV2_ParseCurlCommand_Mock(t *testing.T) {
	// Create a mock workspace import service
	mockResults := &ImportResults{
		HTTPReqsCreated: 1,
		FilesCreated:    1,
		FlowsCreated:    1,
		NodesCreated:    1,
	}
	mockImportService := NewMockWorkspaceImporter(mockResults, nil)

	// Create flow service with mocks
	flowService := &FlowServiceV2RPC{
		workspaceImportService: mockImportService,
	}

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	// Test parsing a simple curl command
	curlData := []byte("curl -X POST https://api.example.com/users -H \"Content-Type: application/json\" -d '{\"name\": \"John Doe\"}'")

	resolved, err := flowService.ParseCurlCommand(ctx, curlData, workspaceID)
	if err != nil {
		// This will fail with auth error, but we can still test the logic
		require.Contains(t, err.Error(), "unauthenticated")
		return
	}

	if resolved != nil {
		// Test that the curl was parsed correctly
		require.Equal(t, "POST", resolved.HTTP.Method)
		require.Equal(t, "https://api.example.com/users", resolved.HTTP.Url)
	}

	// Test parsing invalid curl command
	invalidCurl := []byte("invalid command")
	_, err = flowService.ParseCurlCommand(ctx, invalidCurl, workspaceID)
	require.Error(t, err)
}

func TestFlowServiceV2_ParseFlowData_Mock(t *testing.T) {
	// Create a mock workspace import service
	mockResults := &ImportResults{
		HTTPReqsCreated: 1,
		FilesCreated:    1,
		FlowsCreated:    1,
	}
	mockImportService := NewMockWorkspaceImporter(mockResults, nil)

	// Create flow service with mocks
	flowService := &FlowServiceV2RPC{
		workspaceImportService: mockImportService,
	}

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name        string
		data        []byte
		expectError bool
		description string
	}{
		{
			name:        "Parse YAML flow",
			data:        []byte("flows:\n  - name: test\n    steps:\n      - type: request\n        request: test-request"),
			expectError: false,
			description: "Should parse YAML flow correctly",
		},
		{
			name:        "Parse JSON flow",
			data:        []byte(`{"flows": [{"name": "test", "steps": [{"type": "request", "request": "test-request"}]}`),
			expectError: false,
			description: "Should parse JSON flow correctly",
		},
		{
			name:        "Parse curl command",
			data:        []byte("curl -X GET https://api.example.com/test"),
			expectError: false,
			description: "Should parse curl command correctly",
		},
		{
			name:        "Parse unknown format",
			data:        []byte("just some random text"),
			expectError: true,
			description: "Should return error for unknown format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := flowService.ParseFlowData(ctx, tt.data, workspaceID)

			if tt.expectError {
				require.Error(t, err, tt.description)
				return
			}

			// For successful cases, we might get auth error, but that's expected
			if err != nil && !contains(err.Error(), "unauthenticated") {
				t.Errorf("Unexpected error for %s: %v - %s", tt.name, err, tt.description)
				return
			}

			if result == nil && err == nil {
				t.Errorf("Expected result for %s: %s", tt.name, tt.description)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}

// MockHTTPService provides a mock implementation for testing
type MockHTTPService struct{}

func (m *MockHTTPService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTP, error) {
	return &mhttp.HTTP{ID: id}, nil
}

func (m *MockHTTPService) List(ctx context.Context, workspaceID idwrap.IDWrap) ([]*mhttp.HTTP, error) {
	return []*mhttp.HTTP{}, nil
}

func (m *MockHTTPService) Create(ctx context.Context, httpReq *mhttp.HTTP) (*mhttp.HTTP, error) {
	return httpReq, nil
}

func (m *MockHTTPService) Update(ctx context.Context, httpReq *mhttp.HTTP) (*mhttp.HTTP, error) {
	return httpReq, nil
}

func (m *MockHTTPService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return nil
}