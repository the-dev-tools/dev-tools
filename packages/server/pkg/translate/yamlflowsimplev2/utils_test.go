package yamlflowsimplev2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		expectedURL string
		expectErr   bool
	}{
		{
			name:        "Valid HTTPS URL",
			rawURL:      "https://example.com/api",
			expectedURL: "https://example.com/api",
			expectErr:   false,
		},
		{
			name:        "Valid HTTP URL",
			rawURL:      "http://example.com/api",
			expectedURL: "http://example.com/api",
			expectErr:   false,
		},
		{
			name:        "URL without scheme (should add HTTPS)",
			rawURL:      "example.com/api",
			expectedURL: "https://example.com/api",
			expectErr:   false,
		},
		{
			name:        "URL with path and query",
			rawURL:      "api.example.com/v1/users?active=true",
			expectedURL: "https://api.example.com/v1/users?active=true",
			expectErr:   false,
		},
		{
			name:      "Empty URL",
			rawURL:    "",
			expectErr: true,
		},
		{
			name:      "Invalid URL",
			rawURL:    "not a url",
			expectErr: true,
		},
		{
			name:      "URL with no host",
			rawURL:    "https://",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateURL(tt.rawURL)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedURL, result)
			}
		})
	}
}

func TestValidateHTTPMethod(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{"GET uppercase", "GET", "GET"},
		{"get lowercase", "get", "GET"},
		{"Post mixed case", "Post", "POST"},
		{"PUT with spaces", "  PUT  ", "PUT"},
		{"Invalid method", "INVALID", "GET"},
		{"Empty method", "", "GET"},
		{"DELETE", "DELETE", "DELETE"},
		{"PATCH", "PATCH", "PATCH"},
		{"HEAD", "HEAD", "HEAD"},
		{"OPTIONS", "OPTIONS", "OPTIONS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateHTTPMethod(tt.method)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Normal filename", "test-request", "test-request"},
		{"Spaces and dots", "  test.file.txt  ", "test.file.txt"},
		{"Invalid characters", "test<file>name", "test_file_name"},
		{"All invalid chars", "<>:\"/\\|?*", "_"},
		{"Empty string", "", "untitled"},
		{"Only invalid chars", "<>", "untitled"},
		{"Long filename", strings.Repeat("a", 300), strings.Repeat("a", 255)},
		{"Mixed case", "Test-Request_Name", "Test-Request_Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFileName(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractQueryParamsFromURL(t *testing.T) {
	tests := []struct {
		name           string
		rawURL         string
		expectedParams []NameValuePair
		expectedBase   string
		expectErr      bool
	}{
		{
			name:   "URL with query params",
			rawURL: "https://api.example.com/users?active=true&limit=10",
			expectedParams: []NameValuePair{
				{Name: "active", Value: "true"},
				{Name: "limit", Value: "10"},
			},
			expectedBase: "https://api.example.com/users",
			expectErr:    false,
		},
		{
			name:           "URL without query params",
			rawURL:         "https://api.example.com/users",
			expectedParams: []NameValuePair{},
			expectedBase:   "https://api.example.com/users",
			expectErr:      false,
		},
		{
			name:   "URL with multiple values for same param",
			rawURL: "https://api.example.com/users?tag=a&tag=b",
			expectedParams: []NameValuePair{
				{Name: "tag", Value: "a"}, // Should take first value
			},
			expectedBase: "https://api.example.com/users",
			expectErr:    false,
		},
		{
			name:      "Invalid URL",
			rawURL:    "not a url",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, baseURL, err := ExtractQueryParamsFromURL(tt.rawURL)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				require.Len(t, params, len(tt.expectedParams))

				for i, expected := range tt.expectedParams {
					require.Less(t, i, len(params), "Missing param at index %d", i)
					require.Equal(t, expected.Name, params[i].Name, "Param %d name mismatch", i)
					require.Equal(t, expected.Value, params[i].Value, "Param %d value mismatch", i)
				}

				require.Equal(t, tt.expectedBase, baseURL)
			}
		})
	}
}

func TestDetectBodyType(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"JSON object", `{"key": "value"}`, "json"},
		{"JSON array", `[1, 2, 3]`, "json"},
		{"JSON with whitespace", "\n  {\"key\": \"value\"}\n", "json"},
		{"URL encoded", "param1=value1&param2=value2", "urlencoded"},
		{"Form data mention", "multipart/form-data boundary", "form-data"},
		{"Plain text", "plain text content", "raw"},
		{"Empty string", "", "raw"},
		{"XML", "<root><item>test</item></root>", "raw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBodyType(tt.content)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateHTTPKey(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		url      string
		expected string
	}{
		{"GET request", "GET", "https://api.example.com/users", "GET|https://api.example.com/users"},
		{"POST request", "POST", "https://api.example.com/users", "POST|https://api.example.com/users"},
		{"Lowercase method", "get", "https://api.example.com/users", "GET|https://api.example.com/users"},
		{"Mixed case method", "Post", "https://api.example.com/users", "POST|https://api.example.com/users"},
		{"URL with query", "GET", "https://api.example.com/users?active=true", "GET|https://api.example.com/users?active=true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateHTTPKey(tt.method, tt.url)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateFileOrder(t *testing.T) {
	tests := []struct {
		name          string
		existingFiles []mfile.File
		expected      float64
	}{
		{"No existing files", []mfile.File{}, 1},
		{"One file with order 0", []mfile.File{{Order: 0}}, 1},
		{"One file with order 5", []mfile.File{{Order: 5}}, 6},
		{"Multiple files", []mfile.File{{Order: 2}, {Order: 5}, {Order: 1}}, 6},
		{"Unordered files", []mfile.File{{Order: 10}, {Order: 5}, {Order: 15}}, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateFileOrder(tt.existingFiles)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateSummary(t *testing.T) {
	data := &ioworkspace.WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: idwrap.NewNow(), Name: "Test Flow 1"},
			{ID: idwrap.NewNow(), Name: "Test Flow 2"},
		},
		HTTPRequests: []mhttp.HTTP{
			{ID: idwrap.NewNow(), Name: "Request 1", Method: "GET"},
			{ID: idwrap.NewNow(), Name: "Request 2", Method: "POST"},
			{ID: idwrap.NewNow(), Name: "Request 3", Method: "PUT"},
		},
		Files: []mfile.File{
			{ID: idwrap.NewNow(), Name: "File 1"},
			{ID: idwrap.NewNow(), Name: "File 2"},
		},
	}

	summary := CreateSummary(data)

	// Check basic counts
	require.Equal(t, 2, summary["total_flows"].(int))

	require.Equal(t, 3, summary["total_requests"].(int))

	require.Equal(t, 2, summary["total_files"].(int))

	// Check flow details
	flowDetails := summary["flow_details"].([]map[string]interface{})
	require.Len(t, flowDetails, 2)

	// Check request summary
	requestSummary := summary["request_summary"].(map[string]interface{})
	methods := requestSummary["methods"].(map[string]int)
	require.Equal(t, 1, methods["GET"])
	require.Equal(t, 1, methods["POST"])
	require.Equal(t, 1, methods["PUT"])
}

func TestValidateReferences(t *testing.T) {
	tests := []struct {
		name      string
		data      *ioworkspace.WorkspaceBundle
		expectErr bool
	}{
		{
			name: "Valid data",
			data: &ioworkspace.WorkspaceBundle{
				Flows: []mflow.Flow{{ID: idwrap.NewNow()}},
				FlowNodes: []mflow.Node{
					{ID: idwrap.NewNow(), FlowID: idwrap.NewNow()}, // This will be invalid
				},
			},
			expectErr: true, // Node references non-existent flow
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReferences(tt.data)

			if tt.expectErr {
				require.Error(t, err)
			}

			if !tt.expectErr {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateStats(t *testing.T) {
	data := &ioworkspace.WorkspaceBundle{
		HTTPRequests: []mhttp.HTTP{
			{Method: "GET"},
			{Method: "POST"},
			{Method: "GET"},
		},
		FlowNodes: []mflow.Node{
			{NodeKind: mflow.NODE_KIND_REQUEST},
			{NodeKind: mflow.NODE_KIND_CONDITION},
			{NodeKind: mflow.NODE_KIND_REQUEST},
		},
		Flows: []mflow.Flow{{}, {}},
		Files: []mfile.File{{}, {}, {}},
	}

	stats := GenerateStats(data)

	// Check basic counts
	require.Equal(t, 3, stats["http_requests"].(int))

	require.Equal(t, 3, stats["flow_nodes"].(int))

	require.Equal(t, 2, stats["flows"].(int))

	// Check method breakdown
	methods := stats["http_methods"].(map[string]int)
	require.Equal(t, 2, methods["GET"])
	require.Equal(t, 1, methods["POST"])

	// Check node type breakdown
	nodeTypes := stats["flow_node_types"].(map[string]int)
	require.Equal(t, 2, nodeTypes[string(mflow.NODE_KIND_REQUEST)])
	require.Equal(t, 1, nodeTypes[string(mflow.NODE_KIND_CONDITION)])

	// Check averages
	require.Equal(t, 1.5, stats["avg_nodes_per_flow"].(float64))
}

func TestOptimizeYAMLData(t *testing.T) {
	data := &ioworkspace.WorkspaceBundle{
		HTTPHeaders: []mhttp.HTTPHeader{
			{Key: "B", Value: "value2"},
			{Key: "A", Value: "value1"},
			{Key: "B", Value: "value2"}, // Duplicate
		},
		HTTPSearchParams: []mhttp.HTTPSearchParam{
			{Key: "param2", Value: "value2"},
			{Key: "param1", Value: "value1"},
		},
	}

	OptimizeYAMLData(data)

	// Headers should be sorted and deduplicated
	require.Len(t, data.HTTPHeaders, 2)

	// Should be sorted by key
	require.Equal(t, "A", data.HTTPHeaders[0].Key)
	require.Equal(t, "B", data.HTTPHeaders[1].Key)

	// Search params should be sorted
	require.Len(t, data.HTTPSearchParams, 2)

	require.Equal(t, "param1", data.HTTPSearchParams[0].Key)
	require.Equal(t, "param2", data.HTTPSearchParams[1].Key)
}
