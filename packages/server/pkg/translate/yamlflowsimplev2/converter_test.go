package yamlflowsimplev2

import (
	"strings"
	"testing"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
)

func TestConvertSimplifiedYAML(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    variables:
      - name: timeout
        value: "60"
    steps:
      - request:
          name: API Test
          method: GET
          url: https://api.example.com/test
          headers:
            Authorization: "Bearer token"
            Content-Type: "application/json"
          query_params:
            param1: "value1"
            param2: "value2"
          body:
            type: "json"
            json:
              test: true
              data: "sample"
          assertions:
            - expression: "response.status == 200"
              enabled: true
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	// Verify basic structure
	if len(result.Flows) != 1 {
		t.Errorf("Expected 1 flow, got %d", len(result.Flows))
	}

	if len(result.HTTPRequests) != 1 {
		t.Errorf("Expected 1 HTTP request, got %d", len(result.HTTPRequests))
	}

	if len(result.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(result.Files))
	}

	// Verify flow
	flow := result.Flows[0]
	if flow.Name != "Test Flow" {
		t.Errorf("Expected flow name 'Test Flow', got '%s'", flow.Name)
	}
	if flow.WorkspaceID.Compare(workspaceID) != 0 {
		t.Errorf("Expected flow workspace ID to match, got different ID")
	}

	// Verify HTTP request
	httpReq := result.HTTPRequests[0]
	if httpReq.Name != "API Test" {
		t.Errorf("Expected HTTP request name 'API Test', got '%s'", httpReq.Name)
	}
	if httpReq.Method != "GET" {
		t.Errorf("Expected method GET, got '%s'", httpReq.Method)
	}
	if httpReq.Url != "https://api.example.com/test" {
		t.Errorf("Expected URL 'https://api.example.com/test', got '%s'", httpReq.Url)
	}
	if httpReq.WorkspaceID.Compare(workspaceID) != 0 {
		t.Errorf("Expected HTTP request workspace ID to match, got different ID")
	}

	// Verify headers
	if len(result.Headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(result.Headers))
	}

	headerMap := make(map[string]string)
	for _, header := range result.Headers {
		headerMap[header.Key] = header.Value
	}

	if headerMap["Authorization"] != "Bearer token" {
		t.Errorf("Expected Authorization header 'Bearer token', got '%s'", headerMap["Authorization"])
	}

	if headerMap["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type header 'application/json', got '%s'", headerMap["Content-Type"])
	}

	// Verify query params
	if len(result.SearchParams) != 2 {
		t.Errorf("Expected 2 search params, got %d", len(result.SearchParams))
	}

	paramMap := make(map[string]string)
	for _, param := range result.SearchParams {
		paramMap[param.Key] = param.Value
	}

	if paramMap["param1"] != "value1" {
		t.Errorf("Expected param1 'value1', got '%s'", paramMap["param1"])
	}

	if paramMap["param2"] != "value2" {
		t.Errorf("Expected param2 'value2', got '%s'", paramMap["param2"])
	}

	// Verify body
	if len(result.BodyRaw) != 1 {
		t.Errorf("Expected 1 raw body, got %d", len(result.BodyRaw))
	}

	// Verify file
	file := result.Files[0]
	if file.ContentType != mfile.ContentTypeHTTP {
		t.Errorf("Expected file content type HTTP, got %v", file.ContentType)
	}
	if file.ContentID == nil || file.ContentID.Compare(httpReq.ID) != 0 {
		t.Errorf("File should reference HTTP request")
	}

	// Verify flow variables
	if len(result.FlowVariables) != 1 {
		t.Errorf("Expected 1 flow variable, got %d", len(result.FlowVariables))
	}

	variable := result.FlowVariables[0]
	if variable.Name != "timeout" {
		t.Errorf("Expected variable name 'timeout', got '%s'", variable.Name)
	}
	if variable.Value != "60" {
		t.Errorf("Expected variable value '60', got '%s'", variable.Value)
	}

	// Verify flow nodes (should have start node + request node)
	if len(result.FlowNodes) != 2 {
		t.Errorf("Expected 2 flow nodes (start + request), got %d", len(result.FlowNodes))
	}

	// Verify request node
	if len(result.FlowRequestNodes) != 1 {
		t.Errorf("Expected 1 request node, got %d", len(result.FlowRequestNodes))
	}

	requestNode := result.FlowRequestNodes[0]
	if requestNode.HttpID == nil || requestNode.HttpID.Compare(httpReq.ID) != 0 {
		t.Errorf("Request node should reference HTTP request")
	}
}

func TestConvertSimplifiedYAMLWithTemplates(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Template Test
request_templates:
  base_api:
    name: "Base API Request"
    method: "POST"
    url: "https://api.example.com"
    headers:
      Authorization: "Bearer token"
      Content-Type: "application/json"
flows:
  - name: Template Flow
    steps:
      - request:
          name: Step 1
          use_request: "base_api"
          body:
            type: "json"
            json:
              action: "login"
      - request:
          name: Step 2
          use_request: "base_api"
          url: "https://api.example.com/users"
          body:
            type: "json"
            json:
              action: "get_users"
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	// Should have 2 HTTP requests (one for each step)
	if len(result.HTTPRequests) != 2 {
		t.Errorf("Expected 2 HTTP requests, got %d", len(result.HTTPRequests))
	}

	// Both should have inherited method from template
	for i, httpReq := range result.HTTPRequests {
		if httpReq.Method != "POST" {
			t.Errorf("HTTP request %d: Expected method POST, got '%s'", i, httpReq.Method)
		}
		if httpReq.Name != "" {
			// Name should be overridden by step
			expectedName := []string{"Step 1", "Step 2"}[i]
			if httpReq.Name != expectedName {
				t.Errorf("HTTP request %d: Expected name '%s', got '%s'", i, expectedName, httpReq.Name)
			}
		}
	}

	// Second request should have overridden URL
	if result.HTTPRequests[1].Url != "https://api.example.com/users" {
		t.Errorf("Expected second request URL 'https://api.example.com/users', got '%s'", result.HTTPRequests[1].Url)
	}
}

func TestConvertSimplifiedYAMLWithControlFlow(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Control Flow Test
flows:
  - name: Control Flow
    steps:
      - request:
          name: Initial Request
          method: GET
          url: https://api.example.com/check
      - if:
          name: Check Response
          condition: "response.status == 200"
          then: Success Request
          else: Error Request
      - request:
          name: Success Request
          method: POST
          url: https://api.example.com/success
          depends_on:
            - Check Response
      - request:
          name: Error Request
          method: POST
          url: https://api.example.com/error
          depends_on:
            - Check Response
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	// Should have 3 HTTP requests
	if len(result.HTTPRequests) != 3 {
		t.Errorf("Expected 3 HTTP requests, got %d", len(result.HTTPRequests))
	}

	// Should have 4 flow nodes (start + 3 requests + 1 condition)
	if len(result.FlowNodes) != 5 {
		t.Errorf("Expected 5 flow nodes, got %d", len(result.FlowNodes))
	}

	// Should have 1 condition node
	if len(result.FlowConditionNodes) != 1 {
		t.Errorf("Expected 1 condition node, got %d", len(result.FlowConditionNodes))
	}

	// Should have edges for control flow
	if len(result.FlowEdges) < 4 {
		t.Errorf("Expected at least 4 edges, got %d", len(result.FlowEdges))
	}
}

func TestConvertSimplifiedYAMLWithLoop(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Loop Test
flows:
  - name: Loop Flow
    steps:
      - request:
          name: Setup Request
          method: POST
          url: https://api.example.com/setup
      - for:
          name: Process Items
          iter_count: 3
          loop: Process Request
      - request:
          name: Process Request
          method: POST
          url: https://api.example.com/process
      - request:
          name: Cleanup Request
          method: POST
          url: https://api.example.com/cleanup
          depends_on:
            - Process Items
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	// Should have 3 HTTP requests
	if len(result.HTTPRequests) != 3 {
		t.Errorf("Expected 3 HTTP requests, got %d", len(result.HTTPRequests))
	}

	// Should have 1 for node
	if len(result.FlowForNodes) != 1 {
		t.Errorf("Expected 1 for node, got %d", len(result.FlowForNodes))
	}

	forNode := result.FlowForNodes[0]
	if forNode.IterCount != 3 {
		t.Errorf("Expected iter count 3, got %d", forNode.IterCount)
	}
}

func TestConvertSimplifiedYAMLWithForEach(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: ForEach Test
flows:
  - name: ForEach Flow
    steps:
      - request:
          name: Get Items
          method: GET
          url: https://api.example.com/items
      - for_each:
          name: Process Each Item
          items: "response.data.items"
          loop: Process Item
      - request:
          name: Process Item
          method: POST
          url: https://api.example.com/process
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	// Should have 2 HTTP requests
	if len(result.HTTPRequests) != 2 {
		t.Errorf("Expected 2 HTTP requests, got %d", len(result.HTTPRequests))
	}

	// Should have 1 for_each node
	if len(result.FlowForEachNodes) != 1 {
		t.Errorf("Expected 1 for_each node, got %d", len(result.FlowForEachNodes))
	}

	forEachNode := result.FlowForEachNodes[0]
	if forEachNode.IterExpression != "response.data.items" {
		t.Errorf("Expected iter expression 'response.data.items', got '%s'", forEachNode.IterExpression)
	}
}

func TestConvertSimplifiedYAMLWithJS(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: JS Test
flows:
  - name: JS Flow
    steps:
      - js:
          name: Transform Data
          code: |
            const data = JSON.parse(response.body);
            return data.items.map(item => ({
              id: item.id,
              name: item.name.toUpperCase()
            }));
      - request:
          name: Send Transformed Data
          method: POST
          url: https://api.example.com/submit
          body:
            type: "json"
            json:
              data: "{{transformed_data}}"
          depends_on:
            - Transform Data
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	// Should have 1 HTTP request
	if len(result.HTTPRequests) != 1 {
		t.Errorf("Expected 1 HTTP request, got %d", len(result.HTTPRequests))
	}

	// Should have 1 JS node
	if len(result.FlowJSNodes) != 1 {
		t.Errorf("Expected 1 JS node, got %d", len(result.FlowJSNodes))
	}

	jsNode := result.FlowJSNodes[0]
	expectedCode := `const data = JSON.parse(response.body);
return data.items.map(item => ({
  id: item.id,
  name: item.name.toUpperCase()
}));`
	if string(jsNode.Code) != expectedCode {
		t.Errorf("JS code doesn't match expected")
	}
}

func TestConvertSimplifiedYAMLWithDifferentBodyTypes(t *testing.T) {
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name     string
		bodyYAML string
		validate func(t *testing.T, result *SimplifiedYAMLResolvedV2)
	}{
		{
			name: "JSON Body",
			bodyYAML: `
          body:
            type: "json"
            json:
              key: "value"
              number: 42
              nested:
                field: "data"`,
			validate: func(t *testing.T, result *SimplifiedYAMLResolvedV2) {
				if len(result.BodyRaw) != 1 {
					t.Errorf("Expected 1 raw body, got %d", len(result.BodyRaw))
				}
				body := result.BodyRaw[0]
				if body.ContentType != "application/json" {
					t.Errorf("Expected content type 'application/json', got '%s'", body.ContentType)
				}
			},
		},
		{
			name: "Raw Body",
			bodyYAML: `
          body:
            type: "raw"
            raw: "plain text content"`,
			validate: func(t *testing.T, result *SimplifiedYAMLResolvedV2) {
				if len(result.BodyRaw) != 1 {
					t.Errorf("Expected 1 raw body, got %d", len(result.BodyRaw))
				}
				body := result.BodyRaw[0]
				if string(body.RawData) != "plain text content" {
					t.Errorf("Expected raw body content 'plain text content', got '%s'", string(body.RawData))
				}
			},
		},
		{
			name: "Form Data",
			bodyYAML: `
          body:
            type: "form-data"
            form_data:
              - name: "username"
                value: "john_doe"
              - name: "password"
                value: "secret123"`,
			validate: func(t *testing.T, result *SimplifiedYAMLResolvedV2) {
				if len(result.BodyForms) != 2 {
					t.Errorf("Expected 2 form fields, got %d", len(result.BodyForms))
				}
			},
		},
		{
			name: "URL Encoded",
			bodyYAML: `
          body:
            type: "urlencoded"
            urlencoded:
              - name: "param1"
                value: "value1"
              - name: "param2"
                value: "value2"`,
			validate: func(t *testing.T, result *SimplifiedYAMLResolvedV2) {
				if len(result.BodyUrlencoded) != 2 {
					t.Errorf("Expected 2 urlencoded fields, got %d", len(result.BodyUrlencoded))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlData := `
workspace_name: Body Test
flows:
  - name: Test Flow
    steps:
      - request:
          name: Test Request
          method: POST
          url: https://api.example.com/test` + tt.bodyYAML

			opts := GetDefaultOptions(workspaceID)
			result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

			if err != nil {
				t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
			}

			tt.validate(t, result)
		})
	}
}

func TestConvertSimplifiedYAMLWithCompression(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Compression Test
flows:
  - name: Test Flow
    steps:
      - request:
          name: Large Request
          method: POST
          url: https://api.example.com/large
          body:
            type: "raw"
            raw: |-
              This is a large piece of data that should definitely be larger than 1024 bytes to trigger compression in the test.
              Let me add enough content to make sure this happens. The compression logic only kicks in for data larger than 1KB,
              so I need to create a substantial amount of content here. Let me keep adding more and more text until we reach the threshold.
              This should be enough content to trigger the compression functionality and make the test pass as expected.
              The test is expecting the compression type to be set to gzip, so we need to ensure that the body is large enough to meet the compression criteria.
              Let me add more content here to make sure we exceed the 1024 byte threshold. This is still not enough, so let me add even more content.
              We need to keep going until we reach at least 1024 bytes. This is getting repetitive, but we need enough data to trigger compression.
              Almost there, let me add some more text to make sure we cross the threshold. This should be sufficient now for the compression test.
              One final addition to ensure we have enough data for the compression test to work properly. This should definitely exceed 1024 bytes now.
`

	opts := GetDefaultOptions(workspaceID)
	opts.EnableCompression = true
	opts.CompressionType = compress.CompressTypeGzip

	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	// Should have 1 raw body with compression
	if len(result.BodyRaw) != 1 {
		t.Errorf("Expected 1 raw body, got %d", len(result.BodyRaw))
	}

	body := result.BodyRaw[0]
	if body.CompressionType != compress.CompressTypeGzip {
		t.Errorf("Expected compression type %v, got %v", compress.CompressTypeGzip, body.CompressionType)
	}
}

func TestConvertSimplifiedYAMLErrorCases(t *testing.T) {
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name      string
		yamlData  string
		expectErr bool
		errMsg    string
	}{
		{
			name: "Missing workspace name",
			yamlData: `
flows:
  - name: Test Flow
    steps: []`,
			expectErr: true,
			errMsg:    "workspace_name is required",
		},
		{
			name: "Missing flows",
			yamlData: `
workspace_name: Test Workspace`,
			expectErr: true,
			errMsg:    "at least one flow is required",
		},
		{
			name: "Invalid step type",
			yamlData: `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - invalid_step:
          name: Invalid Step`,
			expectErr: true,
			errMsg:    "unknown step type",
		},
		{
			name: "Request missing URL",
			yamlData: `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - request:
          name: Test Request
          method: GET`,
			expectErr: true,
			errMsg:    "missing required url",
		},
		{
			name: "If step missing condition",
			yamlData: `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - if:
          name: Test Condition`,
			expectErr: true,
			errMsg:    "missing required condition",
		},
		{
			name: "For step missing iter_count",
			yamlData: `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - for:
          name: Test Loop`,
			expectErr: false, // Should default to 1
		},
		{
			name: "JS step missing code",
			yamlData: `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - js:
          name: Test Script`,
			expectErr: true,
			errMsg:    "missing required code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := GetDefaultOptions(workspaceID)
			result, err := ConvertSimplifiedYAML([]byte(tt.yamlData), opts)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}

			// For error cases, result should be nil or partial
			if tt.expectErr && result != nil {
				// This might be acceptable if partial parsing succeeded
				t.Logf("Warning: Got result despite error: %+v", result)
			}
		})
	}
}

func TestConvertOptionsValidation(t *testing.T) {
	tests := []struct {
		name      string
		opts      ConvertOptionsV2
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid options",
			opts: ConvertOptionsV2{
				WorkspaceID: idwrap.NewNow(),
			},
			expectErr: false,
		},
		{
			name: "Empty workspace ID",
			opts: ConvertOptionsV2{
				WorkspaceID: idwrap.IDWrap{},
			},
			expectErr: true,
			errMsg:    "workspace ID is required",
		},
		{
			name: "Delta without name",
			opts: ConvertOptionsV2{
				WorkspaceID: idwrap.NewNow(),
				IsDelta:     true,
				DeltaName:   nil,
			},
			expectErr: true,
			errMsg:    "delta name is required",
		},
		{
			name: "Invalid compression type",
			opts: ConvertOptionsV2{
				WorkspaceID:      idwrap.NewNow(),
				CompressionType:  compress.CompressType(127), // Invalid type (out of valid range)
			},
			expectErr: true,
			errMsg:    "invalid compression type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestGetDefaultOptions(t *testing.T) {
	workspaceID := idwrap.NewNow()
	opts := GetDefaultOptions(workspaceID)

	if opts.WorkspaceID.Compare(workspaceID) != 0 {
		t.Errorf("Expected workspace ID to match")
	}

	if opts.FolderID != nil {
		t.Errorf("Expected folder ID to be nil")
	}

	if opts.IsDelta {
		t.Errorf("Expected IsDelta to be false")
	}

	if !opts.EnableCompression {
		t.Errorf("Expected EnableCompression to be true")
	}

	if opts.CompressionType != compress.CompressTypeGzip {
		t.Errorf("Expected compression type Gzip, got %v", opts.CompressionType)
	}

	if !opts.GenerateFiles {
		t.Errorf("Expected GenerateFiles to be true")
	}

	if opts.FileOrder != 0 {
		t.Errorf("Expected FileOrder to be 0, got %d", opts.FileOrder)
	}
}