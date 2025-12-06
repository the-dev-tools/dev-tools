package yamlflowsimplev2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
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
      - noop:
          name: Start
          type: start
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

	require.NoError(t, err)

	// Verify basic structure
	require.Len(t, result.Flows, 1)
	require.Len(t, result.HTTPRequests, 1)

	// Expect 3 files: 1 flow file + 1 folder for HTTP requests + 1 HTTP file
	require.Len(t, result.Files, 3)

	// Verify we have a flow file
	var hasFlowFile bool
	for _, f := range result.Files {
		if f.ContentType == mfile.ContentTypeFlow {
			hasFlowFile = true
			break
		}
	}
	require.True(t, hasFlowFile, "Expected a flow file (ContentTypeFlow) but none found")

	// Verify flow
	flow := result.Flows[0]
	require.Equal(t, "Test Flow", flow.Name)
	require.Equal(t, 0, flow.WorkspaceID.Compare(workspaceID))

	// Verify Start Node exists
	require.Len(t, result.FlowNoopNodes, 1)
	require.Equal(t, "Start", result.FlowNodes[0].Name)

	// Verify HTTP request
	httpReq := result.HTTPRequests[0]
	require.Equal(t, "API Test", httpReq.Name)
	require.Equal(t, "GET", httpReq.Method)
	require.Equal(t, "https://api.example.com/test", httpReq.Url)
	require.Equal(t, 0, httpReq.WorkspaceID.Compare(workspaceID))

	// Verify headers
	require.Len(t, result.HTTPHeaders, 2)

	headerMap := make(map[string]string)
	for _, header := range result.HTTPHeaders {
		headerMap[header.Key] = header.Value
	}

	require.Equal(t, "Bearer token", headerMap["Authorization"])
	require.Equal(t, "application/json", headerMap["Content-Type"])

	// Verify query params
	require.Len(t, result.HTTPSearchParams, 2)

	paramMap := make(map[string]string)
	for _, param := range result.HTTPSearchParams {
		paramMap[param.Key] = param.Value
	}

	require.Equal(t, "value1", paramMap["param1"])
	require.Equal(t, "value2", paramMap["param2"])

	// Verify body
	require.Len(t, result.HTTPBodyRaw, 1)

	// Verify files - find folder and HTTP file
	var folderFile, httpFile *mfile.File
	for i := range result.Files {
		if result.Files[i].ContentType == mfile.ContentTypeFolder {
			folderFile = &result.Files[i]
		} else if result.Files[i].ContentType == mfile.ContentTypeHTTP {
			httpFile = &result.Files[i]
		}
	}

	require.NotNil(t, folderFile, "Expected folder file to be created")
	require.Equal(t, "Test Flow", folderFile.Name)

	require.NotNil(t, httpFile, "Expected HTTP file to be created")
	require.NotNil(t, httpFile.ContentID)
	require.Equal(t, 0, httpFile.ContentID.Compare(httpReq.ID), "HTTP file should reference HTTP request")
	require.NotNil(t, httpFile.ParentID)
	require.Equal(t, 0, httpFile.ParentID.Compare(folderFile.ID), "HTTP file should be inside the flow folder")

	// Verify flow variables
	require.Len(t, result.FlowVariables, 1)

	variable := result.FlowVariables[0]
	require.Equal(t, "timeout", variable.Name)
	require.Equal(t, "60", variable.Value)

	// Verify flow nodes (should have start node + request node)
	require.Len(t, result.FlowNodes, 2)

	// Verify request node
	require.Len(t, result.FlowRequestNodes, 1)

	requestNode := result.FlowRequestNodes[0]
	require.NotNil(t, requestNode.HttpID)
	require.Equal(t, 0, requestNode.HttpID.Compare(httpReq.ID), "Request node should reference HTTP request")
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
      - noop:
          name: Start
          type: start
      - request:
          name: Step 1
          use_request: "base_api"
          body:
            type: "json"
            json:
              action: "login"
          depends_on:
            - Start
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

	require.NoError(t, err)

	// Should have 2 HTTP requests (one for each step)
	require.Len(t, result.HTTPRequests, 2)

	// Both should have inherited method from template
	for i, httpReq := range result.HTTPRequests {
		require.Equal(t, "POST", httpReq.Method)
		if httpReq.Name != "" {
			// Name should be overridden by step
			expectedName := []string{"Step 1", "Step 2"}[i]
			require.Equal(t, expectedName, httpReq.Name)
		}
	}

	// Second request should have overridden URL
	require.Equal(t, "https://api.example.com/users", result.HTTPRequests[1].Url)
}

func TestConvertSimplifiedYAMLWithControlFlow(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Control Flow Test
flows:
  - name: Control Flow
    steps:
      - noop:
          name: Start
          type: start
      - request:
          name: Initial Request
          method: GET
          url: https://api.example.com/check
          depends_on:
            - Start
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

	require.NoError(t, err)

	// Should have 3 HTTP requests
	require.Len(t, result.HTTPRequests, 3)

	// Should have 5 flow nodes (start + 3 requests + 1 condition)
	require.Len(t, result.FlowNodes, 5)

	// Should have 1 condition node
	require.Len(t, result.FlowConditionNodes, 1)

	// Should have edges for control flow
	require.GreaterOrEqual(t, len(result.FlowEdges), 4)
}

func TestConvertSimplifiedYAMLWithLoop(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Loop Test
flows:
  - name: Loop Flow
    steps:
      - noop:
          name: Start
          type: start
      - request:
          name: Setup Request
          method: POST
          url: https://api.example.com/setup
          depends_on:
            - Start
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

	require.NoError(t, err)

	// Should have 3 HTTP requests
	require.Len(t, result.HTTPRequests, 3)

	// Should have 1 for node
	require.Len(t, result.FlowForNodes, 1)

	forNode := result.FlowForNodes[0]
	require.Equal(t, int64(3), forNode.IterCount)
}

func TestConvertSimplifiedYAMLWithForEach(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: ForEach Test
flows:
  - name: ForEach Flow
    steps:
      - noop:
          name: Start
          type: start
      - request:
          name: Get Items
          method: GET
          url: https://api.example.com/items
          depends_on:
            - Start
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

	require.NoError(t, err)

	// Should have 2 HTTP requests
	require.Len(t, result.HTTPRequests, 2)

	// Should have 1 for_each node
	require.Len(t, result.FlowForEachNodes, 1)

	forEachNode := result.FlowForEachNodes[0]
	require.Equal(t, "response.data.items", forEachNode.IterExpression)
}

func TestConvertSimplifiedYAMLWithJS(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: JS Test
flows:
  - name: JS Flow
    steps:
      - noop:
          name: Start
          type: start
      - js:
          name: Transform Data
          code: |
            const data = JSON.parse(response.body);
            return data.items.map(item => ({
              id: item.id,
              name: item.name.toUpperCase()
            }));
          depends_on:
            - Start
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

	require.NoError(t, err)

	// Should have 1 HTTP request
	require.Len(t, result.HTTPRequests, 1)

	// Should have 1 JS node
	require.Len(t, result.FlowJSNodes, 1)

	jsNode := result.FlowJSNodes[0]
	expectedCode := `const data = JSON.parse(response.body);
return data.items.map(item => ({
  id: item.id,
  name: item.name.toUpperCase()
}));`
	require.Equal(t, expectedCode, string(jsNode.Code))
}

func TestConvertSimplifiedYAMLWithDifferentBodyTypes(t *testing.T) {
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name     string
		bodyYAML string
		validate func(t *testing.T, result *ioworkspace.WorkspaceBundle)
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
			validate: func(t *testing.T, result *ioworkspace.WorkspaceBundle) {
				require.Len(t, result.HTTPBodyRaw, 1)
				body := result.HTTPBodyRaw[0]
				require.Equal(t, "application/json", body.ContentType)
				// Verify BodyKind is set correctly on HTTP request
				require.Len(t, result.HTTPRequests, 1)
				require.Equal(t, mhttp.HttpBodyKindRaw, result.HTTPRequests[0].BodyKind)
			},
		},
		{
			name: "Raw Body",
			bodyYAML: `
          body:
            type: "raw"
            raw: "plain text content"`,
			validate: func(t *testing.T, result *ioworkspace.WorkspaceBundle) {
				require.Len(t, result.HTTPBodyRaw, 1)
				body := result.HTTPBodyRaw[0]
				require.Equal(t, "plain text content", string(body.RawData))
				// Verify BodyKind is set correctly on HTTP request
				require.Len(t, result.HTTPRequests, 1)
				require.Equal(t, mhttp.HttpBodyKindRaw, result.HTTPRequests[0].BodyKind)
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
			validate: func(t *testing.T, result *ioworkspace.WorkspaceBundle) {
				require.Len(t, result.HTTPBodyForms, 2)
				// Verify BodyKind is set correctly on HTTP request
				require.Len(t, result.HTTPRequests, 1)
				require.Equal(t, mhttp.HttpBodyKindFormData, result.HTTPRequests[0].BodyKind)
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
			validate: func(t *testing.T, result *ioworkspace.WorkspaceBundle) {
				require.Len(t, result.HTTPBodyUrlencoded, 2)
				// Verify BodyKind is set correctly on HTTP request
				require.Len(t, result.HTTPRequests, 1)
				require.Equal(t, mhttp.HttpBodyKindUrlEncoded, result.HTTPRequests[0].BodyKind)
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
      - noop:
          name: Start
          type: start
      - request:
          name: Test Request
          method: POST
          url: https://api.example.com/test
          depends_on:
            - Start` + tt.bodyYAML

			opts := GetDefaultOptions(workspaceID)
			result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

			require.NoError(t, err)

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
      - noop:
          name: Start
          type: start
      - request:
          name: Large Request
          method: POST
          url: https://api.example.com/large
          depends_on:
            - Start
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

	require.NoError(t, err)

	// Should have 1 raw body with compression
	require.Len(t, result.HTTPBodyRaw, 1)

	body := result.HTTPBodyRaw[0]
	require.Equal(t, compress.CompressTypeGzip, body.CompressionType)
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
      - noop:
          name: Start
          type: start
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
      - noop:
          name: Start
          type: start
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
      - noop:
          name: Start
          type: start
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
      - noop:
          name: Start
          type: start
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
      - noop:
          name: Start
          type: start
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
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
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
				WorkspaceID:     idwrap.NewNow(),
				CompressionType: compress.CompressType(127), // Invalid type (out of valid range)
			},
			expectErr: true,
			errMsg:    "invalid compression type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()

			if tt.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetDefaultOptions(t *testing.T) {
	workspaceID := idwrap.NewNow()
	opts := GetDefaultOptions(workspaceID)

	require.Equal(t, 0, opts.WorkspaceID.Compare(workspaceID))
	require.Nil(t, opts.FolderID)
	require.False(t, opts.IsDelta)
	require.True(t, opts.EnableCompression)
	require.Equal(t, compress.CompressTypeGzip, opts.CompressionType)
	require.True(t, opts.GenerateFiles)
	require.Equal(t, 0, opts.FileOrder)
}
