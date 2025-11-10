package yamlflowsimplev2

import (
	"strings"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
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
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expectedURL {
					t.Errorf("Expected URL '%s', got '%s'", tt.expectedURL, result)
				}
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
			if result != tt.expected {
				t.Errorf("Expected method '%s', got '%s'", tt.expected, result)
			}
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
			if result != tt.expected {
				t.Errorf("Expected filename '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestExtractQueryParamsFromURL(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
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
			name:          "URL without query params",
			rawURL:        "https://api.example.com/users",
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
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}

				if len(params) != len(tt.expectedParams) {
					t.Errorf("Expected %d params, got %d", len(tt.expectedParams), len(params))
				}

				for i, expected := range tt.expectedParams {
					if i >= len(params) {
						t.Errorf("Missing param at index %d", i)
						continue
					}
					if params[i].Name != expected.Name || params[i].Value != expected.Value {
						t.Errorf("Param %d: expected {%s: %s}, got {%s: %s}",
							i, expected.Name, expected.Value, params[i].Name, params[i].Value)
					}
				}

				if baseURL != tt.expectedBase {
					t.Errorf("Expected base URL '%s', got '%s'", tt.expectedBase, baseURL)
				}
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
			if result != tt.expected {
				t.Errorf("Expected body type '%s', got '%s'", tt.expected, result)
			}
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
			if result != tt.expected {
				t.Errorf("Expected key '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGenerateFileOrder(t *testing.T) {
	tests := []struct {
		name          string
		existingFiles []mfile.File
		expected      int
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
			if result != tt.expected {
				t.Errorf("Expected order %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestCreateSummary(t *testing.T) {
	data := &SimplifiedYAMLResolvedV2{
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
	if summary["total_flows"].(int) != 2 {
		t.Errorf("Expected total_flows 2, got %v", summary["total_flows"])
	}

	if summary["total_requests"].(int) != 3 {
		t.Errorf("Expected total_requests 3, got %v", summary["total_requests"])
	}

	if summary["total_files"].(int) != 2 {
		t.Errorf("Expected total_files 2, got %v", summary["total_files"])
	}

	// Check flow details
	flowDetails := summary["flow_details"].([]map[string]interface{})
	if len(flowDetails) != 2 {
		t.Errorf("Expected 2 flow details, got %d", len(flowDetails))
	}

	// Check request summary
	requestSummary := summary["request_summary"].(map[string]interface{})
	methods := requestSummary["methods"].(map[string]int)
	if methods["GET"] != 1 || methods["POST"] != 1 || methods["PUT"] != 1 {
		t.Errorf("Expected methods {GET:1, POST:1, PUT:1}, got %v", methods)
	}
}

func TestValidateReferences(t *testing.T) {
	tests := []struct {
		name      string
		data      *SimplifiedYAMLResolvedV2
		expectErr bool
	}{
		{
			name: "Valid data",
			data: &SimplifiedYAMLResolvedV2{
				Flows: []mflow.Flow{{ID: idwrap.NewNow()}},
				FlowNodes: []mnnode.MNode{
					{ID: idwrap.NewNow(), FlowID: idwrap.NewNow()}, // This will be invalid
				},
			},
			expectErr: true, // Node references non-existent flow
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up valid relationships
			if len(tt.data.Flows) > 0 && len(tt.data.FlowNodes) > 0 {
				// Make the first node reference the first flow
				tt.data.FlowNodes[0].FlowID = tt.data.Flows[0].ID
			}

			err := ValidateReferences(tt.data)

			if tt.expectErr && err == nil {
				t.Errorf("Expected error, but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}
		})
	}
}

func TestGenerateStats(t *testing.T) {
	data := &SimplifiedYAMLResolvedV2{
		HTTPRequests: []mhttp.HTTP{
			{Method: "GET"},
			{Method: "POST"},
			{Method: "GET"},
		},
		FlowNodes: []mnnode.MNode{
			{NodeKind: mnnode.NODE_KIND_REQUEST},
			{NodeKind: mnnode.NODE_KIND_CONDITION},
			{NodeKind: mnnode.NODE_KIND_REQUEST},
		},
		Flows: []mflow.Flow{{}, {}},
		Files: []mfile.File{{}, {}, {}},
	}

	stats := GenerateStats(data)

	// Check basic counts
	if stats["http_requests"].(int) != 3 {
		t.Errorf("Expected http_requests 3, got %v", stats["http_requests"])
	}

	if stats["flow_nodes"].(int) != 3 {
		t.Errorf("Expected flow_nodes 3, got %v", stats["flow_nodes"])
	}

	if stats["flows"].(int) != 2 {
		t.Errorf("Expected flows 2, got %v", stats["flows"])
	}

	// Check method breakdown
	methods := stats["http_methods"].(map[string]int)
	if methods["GET"] != 2 || methods["POST"] != 1 {
		t.Errorf("Expected methods {GET:2, POST:1}, got %v", methods)
	}

	// Check node type breakdown
	nodeTypes := stats["flow_node_types"].(map[string]int)
	if nodeTypes[string(mnnode.NODE_KIND_REQUEST)] != 2 || nodeTypes[string(mnnode.NODE_KIND_CONDITION)] != 1 {
		t.Errorf("Expected node types {REQUEST:2, CONDITION:1}, got %v", nodeTypes)
	}

	// Check averages
	if stats["avg_nodes_per_flow"].(float64) != 1.5 {
		t.Errorf("Expected avg_nodes_per_flow 1.5, got %v", stats["avg_nodes_per_flow"])
	}
}

func TestOptimizeYAMLData(t *testing.T) {
	data := &SimplifiedYAMLResolvedV2{
		Headers: []mhttp.HTTPHeader{
			{HeaderKey: "B", HeaderValue: "value2"},
			{HeaderKey: "A", HeaderValue: "value1"},
			{HeaderKey: "B", HeaderValue: "value2"}, // Duplicate
		},
		SearchParams: []mhttp.HTTPSearchParam{
			{ParamKey: "param2", ParamValue: "value2"},
			{ParamKey: "param1", ParamValue: "value1"},
		},
	}

	OptimizeYAMLData(data)

	// Headers should be sorted and deduplicated
	if len(data.Headers) != 2 {
		t.Errorf("Expected 2 headers after optimization, got %d", len(data.Headers))
	}

	// Should be sorted by key
	if data.Headers[0].HeaderKey != "A" || data.Headers[1].HeaderKey != "B" {
		t.Errorf("Headers not sorted correctly: %v", data.Headers)
	}

	// Search params should be sorted
	if len(data.SearchParams) != 2 {
		t.Errorf("Expected 2 search params, got %d", len(data.SearchParams))
	}

	if data.SearchParams[0].ParamKey != "param1" || data.SearchParams[1].ParamKey != "param2" {
		t.Errorf("Search params not sorted correctly: %v", data.SearchParams)
	}
}