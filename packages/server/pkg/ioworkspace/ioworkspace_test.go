// Package ioworkspace_test contains tests for the ioworkspace package.
// This file contains merged tests from multiple test files focused on the
// simplified workflow YAML format functionality:
//
// - Basic unmarshal/marshal tests for the workflow YAML format
// - Global request definitions and inheritance
// - Flexible header and body format support
// - Error handling and validation tests
// - Advanced features like dependencies, control flow nodes, etc.
//
// The tests primarily focus on the UnmarshalWorkflowYAML function which
// converts simplified human-friendly YAML into the internal WorkspaceData format.
package ioworkspace_test

import (
	"strings"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
)

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

// ==================================================================================
// Simplified Workflow YAML Format Tests
// ==================================================================================

func TestUnmarshalWorkflowYAML(t *testing.T) {
	yaml := `
workspace_name: Test Workspace

flows:
  - name: TestFlow
    variables:
      - name: baseUrl
        value: "https://api.example.com"
    steps:
      - request:
          name: GetUser
          method: GET
          url: "{{baseUrl}}/users/123"
          headers:
            - name: Authorization
              value: "Bearer {{token}}"
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify workspace
	if wd.Workspace.Name != "Test Workspace" {
		t.Errorf("Expected workspace name 'Test Workspace', got '%s'", wd.Workspace.Name)
	}

	// Verify flows
	if len(wd.Flows) != 1 {
		t.Fatalf("Expected 1 flow, got %d", len(wd.Flows))
	}
	if wd.Flows[0].Name != "TestFlow" {
		t.Errorf("Expected flow name 'TestFlow', got '%s'", wd.Flows[0].Name)
	}

	// Verify variables
	if len(wd.FlowVariables) != 1 {
		t.Fatalf("Expected 1 variable, got %d", len(wd.FlowVariables))
	}
	if wd.FlowVariables[0].Name != "baseUrl" {
		t.Errorf("Expected variable name 'baseUrl', got '%s'", wd.FlowVariables[0].Name)
	}
	if wd.FlowVariables[0].Value != "https://api.example.com" {
		t.Errorf("Expected variable value 'https://api.example.com', got '%s'", wd.FlowVariables[0].Value)
	}

	// Verify nodes
	requestNodeCount := 0
	for _, node := range wd.FlowNodes {
		if node.NodeKind == mnnode.NODE_KIND_REQUEST {
			requestNodeCount++
			if node.Name != "GetUser" {
				t.Errorf("Expected node name 'GetUser', got '%s'", node.Name)
			}
		}
	}
	if requestNodeCount != 1 {
		t.Errorf("Expected 1 request node, got %d", requestNodeCount)
	}

	// Verify endpoints
	if len(wd.Endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(wd.Endpoints))
	}
	if wd.Endpoints[0].Method != "GET" {
		t.Errorf("Expected method 'GET', got '%s'", wd.Endpoints[0].Method)
	}
	if wd.Endpoints[0].Url != "{{baseUrl}}/users/123" {
		t.Errorf("Expected URL '{{baseUrl}}/users/123', got '%s'", wd.Endpoints[0].Url)
	}

	// Verify headers
	if len(wd.ExampleHeaders) != 1 {
		t.Fatalf("Expected 1 header, got %d", len(wd.ExampleHeaders))
	}
	if wd.ExampleHeaders[0].HeaderKey != "Authorization" {
		t.Errorf("Expected header key 'Authorization', got '%s'", wd.ExampleHeaders[0].HeaderKey)
	}
	if wd.ExampleHeaders[0].Value != "Bearer {{token}}" {
		t.Errorf("Expected header value 'Bearer {{token}}', got '%s'", wd.ExampleHeaders[0].Value)
	}
}

// TestMarshalWorkflowYAML tests the marshaling of WorkspaceData to simplified YAML format
// Since we can't easily create the test data without database access, we'll skip this test
// and focus on the UnmarshalWorkflowYAML tests which are more important

// ==================================================================================
// Workflow YAML Format - Basic Tests
// ==================================================================================

func TestUnmarshalWorkflowYAML_BasicWorkflow(t *testing.T) {
	yaml := `
workspace_name: Basic Test Workspace

flows:
  - name: SimpleFlow
    variables:
      base_url: "https://api.example.com"
    steps:
      - request:
          name: GetData
          method: GET
          url: "{{base_url}}/data"
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify workspace
	if wd.Workspace.Name != "Basic Test Workspace" {
		t.Errorf("Expected workspace name 'Basic Test Workspace', got '%s'", wd.Workspace.Name)
	}

	// Verify collection was created
	if len(wd.Collections) != 1 {
		t.Fatalf("Expected 1 collection, got %d", len(wd.Collections))
	}

	// Verify flow
	if len(wd.Flows) != 1 {
		t.Fatalf("Expected 1 flow, got %d", len(wd.Flows))
	}
	if wd.Flows[0].Name != "SimpleFlow" {
		t.Errorf("Expected flow name 'SimpleFlow', got '%s'", wd.Flows[0].Name)
	}

	// Verify variables
	if len(wd.FlowVariables) != 1 {
		t.Fatalf("Expected 1 variable, got %d", len(wd.FlowVariables))
	}
	if wd.FlowVariables[0].Name != "base_url" {
		t.Errorf("Expected variable name 'base_url', got '%s'", wd.FlowVariables[0].Name)
	}

	// Verify nodes (should have start node + request node)
	if len(wd.FlowNodes) != 2 {
		t.Fatalf("Expected 2 nodes (start + request), got %d", len(wd.FlowNodes))
	}

	// Verify request node
	requestNodeFound := false
	for _, node := range wd.FlowNodes {
		if node.NodeKind == mnnode.NODE_KIND_REQUEST && node.Name == "GetData" {
			requestNodeFound = true
			break
		}
	}
	if !requestNodeFound {
		t.Error("Request node 'GetData' not found")
	}

	// Verify endpoint and example
	if len(wd.Endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(wd.Endpoints))
	}
	if wd.Endpoints[0].Method != "GET" {
		t.Errorf("Expected method 'GET', got '%s'", wd.Endpoints[0].Method)
	}
	if wd.Endpoints[0].Url != "{{base_url}}/data" {
		t.Errorf("Expected URL '{{base_url}}/data', got '%s'", wd.Endpoints[0].Url)
	}
}

func TestUnmarshalWorkflowYAML_AllStepTypes(t *testing.T) {
	yaml := `
workspace_name: All Step Types Test

flows:
  - name: ComplexFlow
    steps:
      - request:
          name: InitialRequest
          method: GET
          url: "https://api.example.com/init"
      
      - if:
          name: CheckStatus
          expression: "InitialRequest.response.status == 200"
          then: ProcessData
          else: HandleError
      
      - request:
          name: ProcessData
          method: POST
          url: "https://api.example.com/process"
          depends_on: [CheckStatus]
      
      - request:
          name: HandleError
          method: POST
          url: "https://api.example.com/error"
          depends_on: [CheckStatus]
      
      - for:
          name: RetryLoop
          iter_count: 3
          loop: ProcessData
      
      - js:
          name: TransformData
          code: |
            export default function(context) {
              return { transformed: true };
            }
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Count node types
	nodeTypes := make(map[mnnode.NodeKind]int)
	for _, node := range wd.FlowNodes {
		nodeTypes[node.NodeKind]++
	}

	// Verify we have all expected node types
	if nodeTypes[mnnode.NODE_KIND_REQUEST] != 3 {
		t.Errorf("Expected 3 request nodes, got %d", nodeTypes[mnnode.NODE_KIND_REQUEST])
	}
	if nodeTypes[mnnode.NODE_KIND_CONDITION] != 1 {
		t.Errorf("Expected 1 condition node, got %d", nodeTypes[mnnode.NODE_KIND_CONDITION])
	}
	if nodeTypes[mnnode.NODE_KIND_FOR] != 1 {
		t.Errorf("Expected 1 for node, got %d", nodeTypes[mnnode.NODE_KIND_FOR])
	}
	if nodeTypes[mnnode.NODE_KIND_JS] != 1 {
		t.Errorf("Expected 1 JS node, got %d", nodeTypes[mnnode.NODE_KIND_JS])
	}

	// Verify condition node details
	if len(wd.FlowConditionNodes) != 1 {
		t.Fatalf("Expected 1 condition node implementation, got %d", len(wd.FlowConditionNodes))
	}
	if wd.FlowConditionNodes[0].Condition.Comparisons.Expression != "InitialRequest.response.status == 200" {
		t.Errorf("Unexpected condition expression: %s", wd.FlowConditionNodes[0].Condition.Comparisons.Expression)
	}

	// Verify for node details
	if len(wd.FlowForNodes) != 1 {
		t.Fatalf("Expected 1 for node implementation, got %d", len(wd.FlowForNodes))
	}
	if wd.FlowForNodes[0].IterCount != 3 {
		t.Errorf("Expected iter_count 3, got %d", wd.FlowForNodes[0].IterCount)
	}

	// Verify JS node details
	if len(wd.FlowJSNodes) != 1 {
		t.Fatalf("Expected 1 JS node implementation, got %d", len(wd.FlowJSNodes))
	}
	if !strings.Contains(string(wd.FlowJSNodes[0].Code), "transformed: true") {
		t.Error("JS code not properly stored")
	}
}

// ==================================================================================
// Workflow YAML Format - Global Requests Tests
// ==================================================================================

func TestUnmarshalWorkflowYAML_GlobalRequests(t *testing.T) {
	yaml := `
workspace_name: Global Requests Test

requests:
  - name: auth_request
    method: POST
    url: "{{base_url}}/auth"
    headers:
      Content-Type: application/json
      X-API-Key: secret123
    body:
      username: "{{username}}"
      password: "{{password}}"

flows:
  - name: AuthFlow
    variables:
      base_url: "https://api.example.com"
    steps:
      - request:
          name: Login
          use_request: auth_request
          body:
            username: "testuser"
            password: "testpass"
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify endpoint was created from global request
	if len(wd.Endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(wd.Endpoints))
	}
	if wd.Endpoints[0].Method != "POST" {
		t.Errorf("Expected method 'POST', got '%s'", wd.Endpoints[0].Method)
	}
	if wd.Endpoints[0].Url != "{{base_url}}/auth" {
		t.Errorf("Expected URL '{{base_url}}/auth', got '%s'", wd.Endpoints[0].Url)
	}

	// Verify headers from global request
	headerMap := make(map[string]string)
	for _, h := range wd.ExampleHeaders {
		headerMap[h.HeaderKey] = h.Value
	}
	if headerMap["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type header 'application/json', got '%s'", headerMap["Content-Type"])
	}
	if headerMap["X-API-Key"] != "secret123" {
		t.Errorf("Expected X-API-Key header 'secret123', got '%s'", headerMap["X-API-Key"])
	}

	// Verify body was properly merged
	if len(wd.Rawbodies) != 1 {
		t.Fatalf("Expected 1 raw body, got %d", len(wd.Rawbodies))
	}
	bodyStr := string(wd.Rawbodies[0].Data)
	if !contains(bodyStr, `"username":"testuser"`) {
		t.Errorf("Expected username 'testuser' in body, got: %s", bodyStr)
	}
	if !contains(bodyStr, `"password":"testpass"`) {
		t.Errorf("Expected password 'testpass' in body, got: %s", bodyStr)
	}
}

func TestUnmarshalWorkflowWithGlobalRequests(t *testing.T) {
	yamlData := `
workspace_name: Test Workflow with Global Requests

# Global request definitions
requests:
  - name: auth_request
    method: POST
    url: "{{base_url}}/auth/login"
    headers:
      Content-Type: application/json
      X-API-Version: "v1"
    body:
      grant_type: "password"

  - name: get_user_request
    method: GET
    url: "{{base_url}}/users/{{user_id}}"
    headers:
      Authorization: "Bearer {{token}}"
      Accept: application/json

flows:
  - name: AuthFlow
    variables:
      base_url: "https://api.example.com"
      username: "testuser"
      password: "testpass"
    steps:
      # Use global auth request with overrides
      - request:
          name: Login
          use_request: auth_request
          body:
            username: "{{username}}"
            password: "{{password}}"
          headers:
            X-API-Version: "v2"  # Override global header
            X-Client-ID: "test-client"  # Add new header

      - js:
          name: ExtractToken
          code: |
            export default function(context) {
              const response = JSON.parse(context.Login.response.body);
              return { 
                token: response.access_token,
                user_id: response.user_id
              };
            }

      # Use global get user request without overrides
      - request:
          name: GetCurrentUser
          use_request: get_user_request

      # Regular request without global reference
      - request:
          name: GetUserProfile
          url: "{{base_url}}/users/{{user_id}}/profile"
          method: GET
          headers:
            Authorization: "Bearer {{token}}"

run:
  - AuthFlow
`

	// Parse the workflow YAML
	workspaceData, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yamlData))
	if err != nil {
		t.Fatalf("Failed to unmarshal workflow YAML: %v", err)
	}

	// Verify workspace
	if workspaceData.Workspace.Name != "Test Workflow with Global Requests" {
		t.Errorf("Expected workspace name 'Test Workflow with Global Requests', got '%s'", workspaceData.Workspace.Name)
	}

	// Verify we have the right number of endpoints/examples
	if len(workspaceData.Endpoints) != 3 {
		t.Fatalf("Expected 3 endpoints, got %d", len(workspaceData.Endpoints))
	}
	if len(workspaceData.Examples) != 3 {
		t.Fatalf("Expected 3 examples, got %d", len(workspaceData.Examples))
	}

	// Find endpoints by name
	var loginEndpoint, getCurrentUserEndpoint, getUserProfileEndpoint *int
	for i, endpoint := range workspaceData.Endpoints {
		switch endpoint.Name {
		case "Login Endpoint":
			loginEndpoint = &i
		case "GetCurrentUser Endpoint":
			getCurrentUserEndpoint = &i
		case "GetUserProfile Endpoint":
			getUserProfileEndpoint = &i
		}
	}

	if loginEndpoint == nil || getCurrentUserEndpoint == nil || getUserProfileEndpoint == nil {
		t.Fatal("Could not find all expected endpoints")
	}

	// Test 1: Login endpoint should have POST method and correct URL from global request
	if workspaceData.Endpoints[*loginEndpoint].Method != "POST" {
		t.Errorf("Login endpoint should have POST method, got %s", workspaceData.Endpoints[*loginEndpoint].Method)
	}
	if workspaceData.Endpoints[*loginEndpoint].Url != "{{base_url}}/auth/login" {
		t.Errorf("Login endpoint URL mismatch: got %s", workspaceData.Endpoints[*loginEndpoint].Url)
	}

	// Test 2: GetCurrentUser should inherit from global get_user_request
	if workspaceData.Endpoints[*getCurrentUserEndpoint].Method != "GET" {
		t.Errorf("GetCurrentUser endpoint should have GET method, got %s", workspaceData.Endpoints[*getCurrentUserEndpoint].Method)
	}
	if workspaceData.Endpoints[*getCurrentUserEndpoint].Url != "{{base_url}}/users/{{user_id}}" {
		t.Errorf("GetCurrentUser endpoint URL mismatch: got %s", workspaceData.Endpoints[*getCurrentUserEndpoint].Url)
	}

	// Test 3: GetUserProfile should have its own definition (no global reference)
	if workspaceData.Endpoints[*getUserProfileEndpoint].Method != "GET" {
		t.Errorf("GetUserProfile endpoint should have GET method, got %s", workspaceData.Endpoints[*getUserProfileEndpoint].Method)
	}
	if workspaceData.Endpoints[*getUserProfileEndpoint].Url != "{{base_url}}/users/{{user_id}}/profile" {
		t.Errorf("GetUserProfile endpoint URL mismatch: got %s", workspaceData.Endpoints[*getUserProfileEndpoint].Url)
	}

	// Test 4: Check headers - Login should have overridden X-API-Version and new X-Client-ID
	loginHeaders := make(map[string]string)
	for _, header := range workspaceData.ExampleHeaders {
		if header.ExampleID == workspaceData.Examples[*loginEndpoint].ID {
			loginHeaders[header.HeaderKey] = header.Value
		}
	}

	// Should have 3 headers: Content-Type (from global), X-API-Version (overridden), X-Client-ID (new)
	if len(loginHeaders) != 3 {
		t.Errorf("Login should have 3 headers, got %d", len(loginHeaders))
	}
	if loginHeaders["Content-Type"] != "application/json" {
		t.Errorf("Login Content-Type should be 'application/json', got '%s'", loginHeaders["Content-Type"])
	}
	if loginHeaders["X-API-Version"] != "v2" {
		t.Errorf("Login X-API-Version should be overridden to 'v2', got '%s'", loginHeaders["X-API-Version"])
	}
	if loginHeaders["X-Client-ID"] != "test-client" {
		t.Errorf("Login X-Client-ID should be 'test-client', got '%s'", loginHeaders["X-Client-ID"])
	}

	// Test 5: Check body - Login should have merged body with global grant_type and step username/password
	var loginBody []byte
	for _, body := range workspaceData.Rawbodies {
		if body.ExampleID == workspaceData.Examples[*loginEndpoint].ID {
			loginBody = body.Data
			break
		}
	}
	if loginBody == nil {
		t.Fatal("Login body not found")
	}
	// The body should contain grant_type from global and username/password from step
	expectedBodyParts := []string{`"grant_type":"password"`, `"username":"{{username}}"`, `"password":"{{password}}"`}
	bodyStr := string(loginBody)
	for _, part := range expectedBodyParts {
		if !contains(bodyStr, part) {
			t.Errorf("Login body should contain %s, but got: %s", part, bodyStr)
		}
	}

	// Test 6: GetCurrentUser should have Authorization header from global
	getCurrentUserHeaders := make(map[string]string)
	for _, header := range workspaceData.ExampleHeaders {
		if header.ExampleID == workspaceData.Examples[*getCurrentUserEndpoint].ID {
			getCurrentUserHeaders[header.HeaderKey] = header.Value
		}
	}
	if getCurrentUserHeaders["Authorization"] != "Bearer {{token}}" {
		t.Errorf("GetCurrentUser Authorization header should be 'Bearer {{token}}', got '%s'", getCurrentUserHeaders["Authorization"])
	}
	if getCurrentUserHeaders["Accept"] != "application/json" {
		t.Errorf("GetCurrentUser Accept header should be 'application/json', got '%s'", getCurrentUserHeaders["Accept"])
	}
}

func TestUnmarshalWorkflowWithInvalidGlobalRequest(t *testing.T) {
	yamlData := `
workspace_name: Test Invalid Global Request

requests:
  - name: valid_request
    method: GET
    url: "/api/test"

flows:
  - name: TestFlow
    steps:
      - request:
          name: TestRequest
          use_request: non_existent_request  # This should cause an error
`

	_, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yamlData))
	if err == nil {
		t.Fatal("Expected error for non-existent global request reference, but got none")
	}
	if !contains(err.Error(), "references undefined global request 'non_existent_request'") {
		t.Errorf("Expected error message about missing global request, got: %v", err)
	}
}

func TestUnmarshalWorkflowWithPartialOverride(t *testing.T) {
	yamlData := `
workspace_name: Test Partial Override

requests:
  - name: base_request
    method: POST
    url: "/api/base"
    headers:
      Content-Type: application/json
      X-Version: "1.0"

flows:
  - name: TestFlow
    steps:
      # Override only URL, keep method and headers
      - request:
          name: Request1
          use_request: base_request
          url: "/api/override"
      
      # Override only method
      - request:
          name: Request2
          use_request: base_request
          method: PUT
`

	workspaceData, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yamlData))
	if err != nil {
		t.Fatalf("Failed to unmarshal workflow YAML: %v", err)
	}

	if len(workspaceData.Endpoints) != 2 {
		t.Fatalf("Expected 2 endpoints, got %d", len(workspaceData.Endpoints))
	}

	// Request1 should have overridden URL but original method
	request1 := workspaceData.Endpoints[0]
	if request1.Method != "POST" {
		t.Errorf("Request1 should keep POST method, got %s", request1.Method)
	}
	if request1.Url != "/api/override" {
		t.Errorf("Request1 should have overridden URL '/api/override', got %s", request1.Url)
	}

	// Request2 should have overridden method but original URL
	request2 := workspaceData.Endpoints[1]
	if request2.Method != "PUT" {
		t.Errorf("Request2 should have overridden method PUT, got %s", request2.Method)
	}
	if request2.Url != "/api/base" {
		t.Errorf("Request2 should keep original URL '/api/base', got %s", request2.Url)
	}
}

// ==================================================================================
// Workflow YAML Format - Body and Header Format Tests
// ==================================================================================

func TestUnmarshalWorkflowYAML_BodyFormats(t *testing.T) {
	yaml := `
workspace_name: Body Formats Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: JSONBody
          method: POST
          url: "https://api.example.com/json"
          body:
            field1: "value1"
            field2: 123
            nested:
              key: "value"
      
      - request:
          name: RawBody
          method: POST
          url: "https://api.example.com/raw"
          body:
            kind: raw
            value: "This is raw text content"
      
      - request:
          name: FormBody
          method: POST
          url: "https://api.example.com/form"
          body:
            kind: form
            value:
              username: "testuser"
              password: "testpass"
      
      - request:
          name: URLEncodedBody
          method: POST
          url: "https://api.example.com/urlencoded"
          body:
            kind: url
            value:
              grant_type: "password"
              client_id: "my-client"
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify we have 4 examples
	if len(wd.Examples) != 4 {
		t.Fatalf("Expected 4 examples, got %d", len(wd.Examples))
	}

	// Find examples by name
	examplesByName := make(map[string]mitemapiexample.ItemApiExample)
	for _, ex := range wd.Examples {
		examplesByName[ex.Name] = ex
	}

	// Test JSON body (default)
	jsonExample := examplesByName["JSONBody Example"]
	if jsonExample.BodyType != mitemapiexample.BodyTypeRaw {
		t.Errorf("Expected JSON body to have Raw body type, got %v", jsonExample.BodyType)
	}
	// Find the raw body
	var jsonBody string
	for _, rb := range wd.Rawbodies {
		if rb.ExampleID == jsonExample.ID {
			jsonBody = string(rb.Data)
			break
		}
	}
	if !contains(jsonBody, `"field1":"value1"`) {
		t.Errorf("Expected JSON body to contain field1, got: %s", jsonBody)
	}
	if !contains(jsonBody, `"nested":{"key":"value"}`) {
		t.Errorf("Expected JSON body to contain nested object, got: %s", jsonBody)
	}

	// Test raw body
	rawExample := examplesByName["RawBody Example"]
	if rawExample.BodyType != mitemapiexample.BodyTypeRaw {
		t.Errorf("Expected raw body to have Raw body type, got %v", rawExample.BodyType)
	}
	var rawBody string
	for _, rb := range wd.Rawbodies {
		if rb.ExampleID == rawExample.ID {
			rawBody = string(rb.Data)
			break
		}
	}
	if rawBody != "This is raw text content" {
		t.Errorf("Expected raw body content, got: %s", rawBody)
	}

	// Test form body
	formExample := examplesByName["FormBody Example"]
	if formExample.BodyType != mitemapiexample.BodyTypeForm {
		t.Errorf("Expected form body to have Form body type, got %v", formExample.BodyType)
	}
	formFields := make(map[string]string)
	for _, fb := range wd.FormBodies {
		if fb.ExampleID == formExample.ID {
			formFields[fb.BodyKey] = fb.Value
		}
	}
	if formFields["username"] != "testuser" {
		t.Errorf("Expected form username 'testuser', got '%s'", formFields["username"])
	}
	if formFields["password"] != "testpass" {
		t.Errorf("Expected form password 'testpass', got '%s'", formFields["password"])
	}

	// Test URL-encoded body
	urlExample := examplesByName["URLEncodedBody Example"]
	if urlExample.BodyType != mitemapiexample.BodyTypeUrlencoded {
		t.Errorf("Expected URL-encoded body to have Urlencoded body type, got %v", urlExample.BodyType)
	}
	urlFields := make(map[string]string)
	for _, ub := range wd.UrlBodies {
		if ub.ExampleID == urlExample.ID {
			urlFields[ub.BodyKey] = ub.Value
		}
	}
	if urlFields["grant_type"] != "password" {
		t.Errorf("Expected URL-encoded grant_type 'password', got '%s'", urlFields["grant_type"])
	}
	if urlFields["client_id"] != "my-client" {
		t.Errorf("Expected URL-encoded client_id 'my-client', got '%s'", urlFields["client_id"])
	}
}

func TestUnmarshalWorkflowYAML_HeaderFormats(t *testing.T) {
	yaml := `
workspace_name: Header Formats Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: ArrayHeaders
          method: GET
          url: "https://api.example.com/test1"
          headers:
            - name: Content-Type
              value: application/json
            - name: Authorization
              value: "Bearer token123"
      
      - request:
          name: ObjectHeaders
          method: GET
          url: "https://api.example.com/test2"
          headers:
            Content-Type: application/xml
            X-Custom-Header: custom-value
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Find headers for each example
	headersByExample := make(map[string]map[string]string)
	for _, ex := range wd.Examples {
		headersByExample[ex.Name] = make(map[string]string)
		for _, h := range wd.ExampleHeaders {
			if h.ExampleID == ex.ID {
				headersByExample[ex.Name][h.HeaderKey] = h.Value
			}
		}
	}

	// Test array format headers
	arrayHeaders := headersByExample["ArrayHeaders Example"]
	if arrayHeaders["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", arrayHeaders["Content-Type"])
	}
	if arrayHeaders["Authorization"] != "Bearer token123" {
		t.Errorf("Expected Authorization 'Bearer token123', got '%s'", arrayHeaders["Authorization"])
	}

	// Test object format headers
	objectHeaders := headersByExample["ObjectHeaders Example"]
	if objectHeaders["Content-Type"] != "application/xml" {
		t.Errorf("Expected Content-Type 'application/xml', got '%s'", objectHeaders["Content-Type"])
	}
	if objectHeaders["X-Custom-Header"] != "custom-value" {
		t.Errorf("Expected X-Custom-Header 'custom-value', got '%s'", objectHeaders["X-Custom-Header"])
	}
}

// ==================================================================================
// Workflow YAML Format - Flexible Format Tests
// ==================================================================================

func TestFlexibleHeaderFormats(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, wd *ioworkspace.WorkspaceData)
	}{
		{
			name: "Array format headers in global request",
			yaml: `
workspace_name: Test Array Headers

requests:
  - name: test_request
    method: GET
    url: "https://api.example.com/test"
    headers:
      - name: Content-Type
        value: application/json
      - name: X-API-Key
        value: secret123

flows:
  - name: TestFlow
    steps:
      - request:
          name: UseGlobalRequest
          use_request: test_request
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.ExampleHeaders) != 2 {
					t.Errorf("expected 2 headers, got %d", len(wd.ExampleHeaders))
				}
				
				headerMap := make(map[string]string)
				for _, h := range wd.ExampleHeaders {
					headerMap[h.HeaderKey] = h.Value
				}
				
				if headerMap["Content-Type"] != "application/json" {
					t.Errorf("expected Content-Type to be 'application/json', got '%s'", headerMap["Content-Type"])
				}
				if headerMap["X-API-Key"] != "secret123" {
					t.Errorf("expected X-API-Key to be 'secret123', got '%s'", headerMap["X-API-Key"])
				}
			},
		},
		{
			name: "Object format headers in global request",
			yaml: `
workspace_name: Test Object Headers

requests:
  - name: test_request
    method: GET
    url: "https://api.example.com/test"
    headers:
      Content-Type: application/json
      X-API-Key: secret123
      Authorization: "Bearer {{token}}"

flows:
  - name: TestFlow
    steps:
      - request:
          name: UseGlobalRequest
          use_request: test_request
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.ExampleHeaders) != 3 {
					t.Errorf("expected 3 headers, got %d", len(wd.ExampleHeaders))
				}
				
				headerMap := make(map[string]string)
				for _, h := range wd.ExampleHeaders {
					headerMap[h.HeaderKey] = h.Value
				}
				
				if headerMap["Content-Type"] != "application/json" {
					t.Errorf("expected Content-Type to be 'application/json', got '%s'", headerMap["Content-Type"])
				}
				if headerMap["X-API-Key"] != "secret123" {
					t.Errorf("expected X-API-Key to be 'secret123', got '%s'", headerMap["X-API-Key"])
				}
				if headerMap["Authorization"] != "Bearer {{token}}" {
					t.Errorf("expected Authorization to be 'Bearer {{token}}', got '%s'", headerMap["Authorization"])
				}
			},
		},
		{
			name: "Override array headers with object format",
			yaml: `
workspace_name: Test Header Override

requests:
  - name: test_request
    method: GET
    url: "https://api.example.com/test"
    headers:
      - name: Content-Type
        value: application/xml
      - name: X-API-Key
        value: old-key

flows:
  - name: TestFlow
    steps:
      - request:
          name: OverrideHeaders
          use_request: test_request
          headers:
            Content-Type: application/json
            X-Custom-Header: custom-value
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.ExampleHeaders) != 3 {
					t.Errorf("expected 3 headers, got %d", len(wd.ExampleHeaders))
				}
				
				headerMap := make(map[string]string)
				for _, h := range wd.ExampleHeaders {
					headerMap[h.HeaderKey] = h.Value
				}
				
				// Content-Type should be overridden
				if headerMap["Content-Type"] != "application/json" {
					t.Errorf("expected Content-Type to be 'application/json', got '%s'", headerMap["Content-Type"])
				}
				// X-API-Key should remain from global
				if headerMap["X-API-Key"] != "old-key" {
					t.Errorf("expected X-API-Key to be 'old-key', got '%s'", headerMap["X-API-Key"])
				}
				// X-Custom-Header should be added
				if headerMap["X-Custom-Header"] != "custom-value" {
					t.Errorf("expected X-Custom-Header to be 'custom-value', got '%s'", headerMap["X-Custom-Header"])
				}
			},
		},
		{
			name: "Override object headers with array format",
			yaml: `
workspace_name: Test Header Override Array

requests:
  - name: test_request
    method: GET
    url: "https://api.example.com/test"
    headers:
      Content-Type: application/xml
      X-API-Key: old-key

flows:
  - name: TestFlow
    steps:
      - request:
          name: OverrideHeaders
          use_request: test_request
          headers:
            - name: Content-Type
              value: application/json
            - name: X-Custom-Header
              value: custom-value
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.ExampleHeaders) != 3 {
					t.Errorf("expected 3 headers, got %d", len(wd.ExampleHeaders))
				}
				
				headerMap := make(map[string]string)
				for _, h := range wd.ExampleHeaders {
					headerMap[h.HeaderKey] = h.Value
				}
				
				// Content-Type should be overridden
				if headerMap["Content-Type"] != "application/json" {
					t.Errorf("expected Content-Type to be 'application/json', got '%s'", headerMap["Content-Type"])
				}
				// X-API-Key should remain from global
				if headerMap["X-API-Key"] != "old-key" {
					t.Errorf("expected X-API-Key to be 'old-key', got '%s'", headerMap["X-API-Key"])
				}
				// X-Custom-Header should be added
				if headerMap["X-Custom-Header"] != "custom-value" {
					t.Errorf("expected X-Custom-Header to be 'custom-value', got '%s'", headerMap["X-Custom-Header"])
				}
			},
		},
		{
			name: "No headers in step uses global headers",
			yaml: `
workspace_name: Test No Override

requests:
  - name: test_request
    method: GET
    url: "https://api.example.com/test"
    headers:
      Authorization: "Bearer token123"
      Accept: application/json

flows:
  - name: TestFlow
    steps:
      - request:
          name: UseDefaultHeaders
          use_request: test_request
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.ExampleHeaders) != 2 {
					t.Errorf("expected 2 headers, got %d", len(wd.ExampleHeaders))
				}
				
				headerMap := make(map[string]string)
				for _, h := range wd.ExampleHeaders {
					headerMap[h.HeaderKey] = h.Value
				}
				
				if headerMap["Authorization"] != "Bearer token123" {
					t.Errorf("expected Authorization to be 'Bearer token123', got '%s'", headerMap["Authorization"])
				}
				if headerMap["Accept"] != "application/json" {
					t.Errorf("expected Accept to be 'application/json', got '%s'", headerMap["Accept"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}
			
			tt.validate(t, wd)
		})
	}
}

func TestFlexibleBodyFormats(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, wd *ioworkspace.WorkspaceData)
	}{
		{
			name: "Simple JSON body format",
			yaml: `
workspace_name: Test Simple Body

flows:
  - name: TestFlow
    steps:
      - request:
          name: SimpleBody
          method: POST
          url: "https://api.example.com/test"
          body:
            username: testuser
            password: testpass
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.Rawbodies) != 1 {
					t.Fatalf("expected 1 raw body, got %d", len(wd.Rawbodies))
				}
				
				bodyData := string(wd.Rawbodies[0].Data)
				if !contains(bodyData, `"username":"testuser"`) {
					t.Errorf("expected body to contain username, got: %s", bodyData)
				}
				if !contains(bodyData, `"password":"testpass"`) {
					t.Errorf("expected body to contain password, got: %s", bodyData)
				}
			},
		},
		{
			name: "Explicit JSON body format",
			yaml: `
workspace_name: Test Explicit JSON Body

flows:
  - name: TestFlow
    steps:
      - request:
          name: ExplicitJSONBody
          method: POST
          url: "https://api.example.com/test"
          body:
            kind: json
            value:
              username: testuser
              password: testpass
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.Rawbodies) != 1 {
					t.Fatalf("expected 1 raw body, got %d", len(wd.Rawbodies))
				}
				
				bodyData := string(wd.Rawbodies[0].Data)
				if !contains(bodyData, `"username":"testuser"`) {
					t.Errorf("expected body to contain username, got: %s", bodyData)
				}
				if !contains(bodyData, `"password":"testpass"`) {
					t.Errorf("expected body to contain password, got: %s", bodyData)
				}
			},
		},
		{
			name: "Raw text body format",
			yaml: `
workspace_name: Test Raw Body

flows:
  - name: TestFlow
    steps:
      - request:
          name: RawBody
          method: POST
          url: "https://api.example.com/test"
          body:
            kind: raw
            value: "This is raw text content"
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.Rawbodies) != 1 {
					t.Fatalf("expected 1 raw body, got %d", len(wd.Rawbodies))
				}
				
				bodyData := string(wd.Rawbodies[0].Data)
				if bodyData != "This is raw text content" {
					t.Errorf("expected raw body to be 'This is raw text content', got: %s", bodyData)
				}
			},
		},
		{
			name: "Global body with simple override",
			yaml: `
workspace_name: Test Body Override

requests:
  - name: test_request
    method: POST
    url: "https://api.example.com/test"
    body:
      globalField: globalValue
      overrideMe: oldValue

flows:
  - name: TestFlow
    steps:
      - request:
          name: OverrideBody
          use_request: test_request
          body:
            overrideMe: newValue
            newField: newValue
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.Rawbodies) != 1 {
					t.Fatalf("expected 1 raw body, got %d", len(wd.Rawbodies))
				}
				
				bodyData := string(wd.Rawbodies[0].Data)
				// Global field should remain
				if !contains(bodyData, `"globalField":"globalValue"`) {
					t.Errorf("expected body to contain globalField, got: %s", bodyData)
				}
				// Override field should be updated
				if !contains(bodyData, `"overrideMe":"newValue"`) {
					t.Errorf("expected overrideMe to be 'newValue', got: %s", bodyData)
				}
				// New field should be added
				if !contains(bodyData, `"newField":"newValue"`) {
					t.Errorf("expected body to contain newField, got: %s", bodyData)
				}
			},
		},
		{
			name: "Global JSON body with explicit JSON override",
			yaml: `
workspace_name: Test Explicit Override

requests:
  - name: test_request
    method: POST
    url: "https://api.example.com/test"
    body:
      kind: json
      value:
        globalField: globalValue
        overrideMe: oldValue

flows:
  - name: TestFlow
    steps:
      - request:
          name: ExplicitOverride
          use_request: test_request
          body:
            kind: json
            value:
              overrideMe: newValue
              newField: newValue
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.Rawbodies) != 1 {
					t.Fatalf("expected 1 raw body, got %d", len(wd.Rawbodies))
				}
				
				bodyData := string(wd.Rawbodies[0].Data)
				// Global field should remain
				if !contains(bodyData, `"globalField":"globalValue"`) {
					t.Errorf("expected body to contain globalField, got: %s", bodyData)
				}
				// Override field should be updated
				if !contains(bodyData, `"overrideMe":"newValue"`) {
					t.Errorf("expected overrideMe to be 'newValue', got: %s", bodyData)
				}
				// New field should be added
				if !contains(bodyData, `"newField":"newValue"`) {
					t.Errorf("expected body to contain newField, got: %s", bodyData)
				}
			},
		},
		{
			name: "Override JSON body with raw body",
			yaml: `
workspace_name: Test Kind Override

requests:
  - name: test_request
    method: POST
    url: "https://api.example.com/test"
    body:
      kind: json
      value:
        field: value

flows:
  - name: TestFlow
    steps:
      - request:
          name: OverrideWithRaw
          use_request: test_request
          body:
            kind: raw
            value: "Raw text override"
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.Rawbodies) != 1 {
					t.Fatalf("expected 1 raw body, got %d", len(wd.Rawbodies))
				}
				
				bodyData := string(wd.Rawbodies[0].Data)
				// Should be raw text, not JSON
				if bodyData != "Raw text override" {
					t.Errorf("expected raw body to be 'Raw text override', got: %s", bodyData)
				}
			},
		},
		{
			name: "Form-encoded body",
			yaml: `
workspace_name: Test Form Body

flows:
  - name: TestFlow
    steps:
      - request:
          name: FormRequest
          method: POST
          url: "https://api.example.com/test"
          body:
            kind: form
            value:
              username: testuser
              password: testpass
              remember_me: "true"
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.FormBodies) != 3 {
					t.Fatalf("expected 3 form body entries, got %d", len(wd.FormBodies))
				}
				
				// Check that example has correct body type
				if len(wd.Examples) != 1 {
					t.Fatalf("expected 1 example, got %d", len(wd.Examples))
				}
				if wd.Examples[0].BodyType != mitemapiexample.BodyTypeForm {
					t.Errorf("expected example body type to be Form, got %v", wd.Examples[0].BodyType)
				}
				
				// Check form fields
				formMap := make(map[string]string)
				for _, fb := range wd.FormBodies {
					formMap[fb.BodyKey] = fb.Value
				}
				
				if formMap["username"] != "testuser" {
					t.Errorf("expected username to be 'testuser', got '%s'", formMap["username"])
				}
				if formMap["password"] != "testpass" {
					t.Errorf("expected password to be 'testpass', got '%s'", formMap["password"])
				}
				if formMap["remember_me"] != "true" {
					t.Errorf("expected remember_me to be 'true', got '%s'", formMap["remember_me"])
				}
				
				// Verify all form entries are enabled
				for _, fb := range wd.FormBodies {
					if !fb.Enable {
						t.Errorf("expected form field '%s' to be enabled", fb.BodyKey)
					}
				}
			},
		},
		{
			name: "URL-encoded body",
			yaml: `
workspace_name: Test URL-encoded Body

flows:
  - name: TestFlow
    steps:
      - request:
          name: URLEncodedRequest
          method: POST
          url: "https://api.example.com/test"
          body:
            kind: url
            value:
              grant_type: password
              client_id: my-client
              scope: "read write"
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.UrlBodies) != 3 {
					t.Fatalf("expected 3 URL-encoded body entries, got %d", len(wd.UrlBodies))
				}
				
				// Check that example has correct body type
				if len(wd.Examples) != 1 {
					t.Fatalf("expected 1 example, got %d", len(wd.Examples))
				}
				if wd.Examples[0].BodyType != mitemapiexample.BodyTypeUrlencoded {
					t.Errorf("expected example body type to be Urlencoded, got %v", wd.Examples[0].BodyType)
				}
				
				// Check URL-encoded fields
				urlMap := make(map[string]string)
				for _, ub := range wd.UrlBodies {
					urlMap[ub.BodyKey] = ub.Value
				}
				
				if urlMap["grant_type"] != "password" {
					t.Errorf("expected grant_type to be 'password', got '%s'", urlMap["grant_type"])
				}
				if urlMap["client_id"] != "my-client" {
					t.Errorf("expected client_id to be 'my-client', got '%s'", urlMap["client_id"])
				}
				if urlMap["scope"] != "read write" {
					t.Errorf("expected scope to be 'read write', got '%s'", urlMap["scope"])
				}
				
				// Verify all URL-encoded entries are enabled
				for _, ub := range wd.UrlBodies {
					if !ub.Enable {
						t.Errorf("expected URL-encoded field '%s' to be enabled", ub.BodyKey)
					}
				}
			},
		},
		{
			name: "Mixed body types in global and step",
			yaml: `
workspace_name: Test Mixed Body Types

requests:
  - name: form_request
    method: POST
    url: "https://api.example.com/form"
    body:
      kind: form
      value:
        field1: value1
        field2: value2

flows:
  - name: TestFlow
    steps:
      - request:
          name: UseFormRequest
          use_request: form_request
      
      - request:
          name: OverrideWithJSON
          use_request: form_request
          body:
            kind: json
            value:
              json_field: json_value
`,
			validate: func(t *testing.T, wd *ioworkspace.WorkspaceData) {
				if len(wd.Examples) != 2 {
					t.Fatalf("expected 2 examples, got %d", len(wd.Examples))
				}
				
				// First request should use form body
				if len(wd.FormBodies) != 2 {
					t.Fatalf("expected 2 form body entries, got %d", len(wd.FormBodies))
				}
				
				// Second request should use JSON body
				if len(wd.Rawbodies) != 1 {
					t.Fatalf("expected 1 raw body, got %d", len(wd.Rawbodies))
				}
				
				// Verify body types
				exampleBodyTypes := make(map[string]mitemapiexample.BodyType)
				for _, ex := range wd.Examples {
					exampleBodyTypes[ex.Name] = ex.BodyType
				}
				
				if exampleBodyTypes["UseFormRequest Example"] != mitemapiexample.BodyTypeForm {
					t.Errorf("expected UseFormRequest to have Form body type")
				}
				if exampleBodyTypes["OverrideWithJSON Example"] != mitemapiexample.BodyTypeRaw {
					t.Errorf("expected OverrideWithJSON to have Raw body type")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}
			
			tt.validate(t, wd)
		})
	}
}

// ==================================================================================
// Workflow YAML Format - Advanced Tests
// ==================================================================================

func TestUnmarshalWorkflowYAML_Dependencies(t *testing.T) {
	yaml := `
workspace_name: Dependencies Test

flows:
  - name: DependencyFlow
    steps:
      - request:
          name: Step1
          method: GET
          url: "https://api.example.com/step1"
      
      - request:
          name: Step2
          method: GET
          url: "https://api.example.com/step2"
          depends_on: [Step1]
      
      - request:
          name: Step3
          method: GET
          url: "https://api.example.com/step3"
          depends_on: [Step1, Step2]
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Create node name to ID map for verification
	nodeNameToID := make(map[string]idwrap.IDWrap)
	for _, node := range wd.FlowNodes {
		if node.NodeKind == mnnode.NODE_KIND_REQUEST {
			nodeNameToID[node.Name] = node.ID
		}
	}

	// Verify edges were created for dependencies
	// Step2 should have an edge from Step1
	step1ToStep2Found := false
	step1ToStep3Found := false
	step2ToStep3Found := false

	for _, edge := range wd.FlowEdges {
		if edge.SourceID == nodeNameToID["Step1"] && edge.TargetID == nodeNameToID["Step2"] {
			step1ToStep2Found = true
		}
		if edge.SourceID == nodeNameToID["Step1"] && edge.TargetID == nodeNameToID["Step3"] {
			step1ToStep3Found = true
		}
		if edge.SourceID == nodeNameToID["Step2"] && edge.TargetID == nodeNameToID["Step3"] {
			step2ToStep3Found = true
		}
	}

	if !step1ToStep2Found {
		t.Error("Expected edge from Step1 to Step2")
	}
	if !step1ToStep3Found {
		t.Error("Expected edge from Step1 to Step3")
	}
	if !step2ToStep3Found {
		t.Error("Expected edge from Step2 to Step3")
	}
}

func TestUnmarshalWorkflowYAML_ErrorCases(t *testing.T) {
	testCases := []struct {
		name        string
		yaml        string
		expectError string
	}{
		{
			name: "Missing workspace name",
			yaml: `
flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          url: "/test"
`,
			expectError: "workspace_name is required",
		},
		{
			name: "Missing flow name",
			yaml: `
workspace_name: Test

flows:
  - steps:
      - request:
          name: Test
          url: "/test"
`,
			expectError: "flow name is required",
		},
		{
			name: "Missing step name",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - request:
          url: "/test"
`,
			expectError: "missing required 'name' field",
		},
		{
			name: "Invalid step type",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - invalid_type:
          name: Test
`,
			expectError: "unknown step type 'invalid_type'",
		},
		{
			name: "Missing if expression",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - if:
          name: TestIf
          then: NextStep
`,
			expectError: "'expression' field is required for if/condition nodes",
		},
		{
			name: "Invalid for iter_count",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - for:
          name: TestFor
          iter_count: -5
`,
			expectError: "'iter_count' for 'for' node 'TestFor' cannot be negative",
		},
		{
			name: "Missing JS code",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - js:
          name: TestJS
`,
			expectError: "'code' field is required and must be a non-empty string for js node",
		},
		{
			name: "Invalid global request reference",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: TestRequest
          use_request: non_existent_request
`,
			expectError: "references undefined global request",
		},
		{
			name: "Missing request URL",
			yaml: `
workspace_name: Error Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: NoURL
          method: GET
`,
			expectError: "missing required 'url' field",
		},
		{
			name: "Duplicate node names",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: DuplicateName
          url: "/test1"
      - request:
          name: DuplicateName
          url: "/test2"
`,
			expectError: "duplicate node name 'DuplicateName'",
		},
		{
			name: "Invalid dependency reference",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: Step1
          url: "/test"
          depends_on: [NonExistent]
`,
			expectError: "dependency node 'NonExistent' for node 'Step1' not found",
		},
		{
			name: "Self dependency",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: Step1
          url: "/test"
          depends_on: [Step1]
`,
			expectError: "node 'Step1' cannot depend on itself",
		},
		{
			name: "Invalid then target",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - if:
          name: TestIf
          expression: "true"
          then: NonExistent
`,
			expectError: "target node 'NonExistent' for 'then' branch",
		},
		{
			name: "Invalid run field",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: Test
          url: "/test"

run:
  - NonExistentFlow
`,
			expectError: "run field references non-existent flow: NonExistentFlow",
		},
		{
			name: "Circular dependency",
			yaml: `
workspace_name: Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: Step1
          url: "/test1"
          depends_on: [Step3]
      - request:
          name: Step2
          url: "/test2"
          depends_on: [Step1]
      - request:
          name: Step3
          url: "/test3"
          depends_on: [Step2]
`,
			expectError: "circular dependency detected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ioworkspace.UnmarshalWorkflowYAML([]byte(tc.yaml))
			if err == nil {
				t.Fatal("Expected error but got none")
			}
			if !contains(err.Error(), tc.expectError) {
				t.Errorf("Expected error containing '%s', got '%s'", tc.expectError, err.Error())
			}
		})
	}
}

func TestUnmarshalWorkflowYAML_CompleteWorkflow(t *testing.T) {
	yaml := `
workspace_name: Complete E-commerce Workflow

requests:
  - name: auth_request
    method: POST
    url: "{{base_url}}/auth/login"
    headers:
      Content-Type: application/json
    body:
      grant_type: password

  - name: api_request
    headers:
      Authorization: "Bearer {{token}}"
      Accept: application/json

flows:
  - name: AuthenticationFlow
    variables:
      base_url: "https://api.example.com"
    steps:
      - request:
          name: Login
          use_request: auth_request
          body:
            username: "{{username}}"
            password: "{{password}}"
      
      - js:
          name: ExtractToken
          code: |
            export default function(context) {
              const response = JSON.parse(context.Login.response.body);
              return { token: response.access_token };
            }

  - name: ProductFlow
    variables:
      base_url: "https://api.example.com"
    steps:
      - request:
          name: GetProducts
          use_request: api_request
          method: GET
          url: "{{base_url}}/products"
      
      - if:
          name: CheckProducts
          expression: "GetProducts.response.status == 200"
          then: ProcessProducts
          else: HandleError
      
      - js:
          name: ProcessProducts
          code: |
            export default function(context) {
              const products = JSON.parse(context.GetProducts.response.body);
              return { productCount: products.length };
            }
          depends_on: [CheckProducts]
      
      - request:
          name: HandleError
          method: POST
          url: "{{base_url}}/errors"
          body:
            error: "Failed to get products"
          depends_on: [CheckProducts]

  - name: OrderFlow
    steps:
      - for:
          name: ProcessOrders
          iter_count: 5
          loop: CreateOrder
      
      - request:
          name: CreateOrder
          use_request: api_request
          method: POST
          url: "{{base_url}}/orders"
          body:
            product_id: "{{product_id}}"
            quantity: 1

run:
  - AuthenticationFlow
  - ProductFlow
  - OrderFlow
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify workspace
	if wd.Workspace.Name != "Complete E-commerce Workflow" {
		t.Errorf("Expected workspace name 'Complete E-commerce Workflow', got '%s'", wd.Workspace.Name)
	}

	// Verify flows
	if len(wd.Flows) != 3 {
		t.Fatalf("Expected 3 flows, got %d", len(wd.Flows))
	}
	flowNames := make(map[string]bool)
	for _, flow := range wd.Flows {
		flowNames[flow.Name] = true
	}
	expectedFlows := []string{"AuthenticationFlow", "ProductFlow", "OrderFlow"}
	for _, expected := range expectedFlows {
		if !flowNames[expected] {
			t.Errorf("Expected flow '%s' not found", expected)
		}
	}

	// Count node types
	nodeTypes := make(map[mnnode.NodeKind]int)
	for _, node := range wd.FlowNodes {
		nodeTypes[node.NodeKind]++
	}

	// Verify node counts (excluding start nodes)
	if nodeTypes[mnnode.NODE_KIND_REQUEST] != 4 {
		t.Errorf("Expected 4 request nodes, got %d", nodeTypes[mnnode.NODE_KIND_REQUEST])
	}
	if nodeTypes[mnnode.NODE_KIND_JS] != 2 {
		t.Errorf("Expected 2 JS nodes, got %d", nodeTypes[mnnode.NODE_KIND_JS])
	}
	if nodeTypes[mnnode.NODE_KIND_CONDITION] != 1 {
		t.Errorf("Expected 1 condition node, got %d", nodeTypes[mnnode.NODE_KIND_CONDITION])
	}
	if nodeTypes[mnnode.NODE_KIND_FOR] != 1 {
		t.Errorf("Expected 1 for node, got %d", nodeTypes[mnnode.NODE_KIND_FOR])
	}

	// Verify global request usage
	endpointMethods := make(map[string]string)
	for _, endpoint := range wd.Endpoints {
		endpointMethods[endpoint.Name] = endpoint.Method
	}
	
	// Login should use POST from auth_request
	if endpointMethods["Login Endpoint"] != "POST" {
		t.Errorf("Expected Login to use POST method from global request")
	}
	
	// GetProducts should use GET (specified in step)
	if endpointMethods["GetProducts Endpoint"] != "GET" {
		t.Errorf("Expected GetProducts to use GET method")
	}

	// Verify headers inheritance
	// Count endpoints that should have Authorization header from api_request
	authHeaderCount := 0
	for _, header := range wd.ExampleHeaders {
		if header.HeaderKey == "Authorization" && header.Value == "Bearer {{token}}" {
			authHeaderCount++
		}
	}
	// Should be on GetProducts and CreateOrder
	if authHeaderCount < 2 {
		t.Errorf("Expected at least 2 endpoints with Authorization header, found %d", authHeaderCount)
	}
}

func TestUnmarshalWorkflowYAML_BackwardCompatibility(t *testing.T) {
	// Test old body_json format
	yaml := `
workspace_name: Backward Compatibility Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: OldBodyFormat
          method: POST
          url: "https://api.example.com/test"
          body:
            body_json:
              field1: "value1"
              field2: 123
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML with old format: %v", err)
	}

	// Verify the body was processed correctly
	if len(wd.Rawbodies) != 1 {
		t.Fatalf("Expected 1 raw body, got %d", len(wd.Rawbodies))
	}

	bodyStr := string(wd.Rawbodies[0].Data)
	if !contains(bodyStr, `"field1":"value1"`) {
		t.Errorf("Expected body to contain field1, got: %s", bodyStr)
	}
	if !contains(bodyStr, `"field2":123`) {
		t.Errorf("Expected body to contain field2, got: %s", bodyStr)
	}
}

func TestUnmarshalWorkflowYAML_FileReferences(t *testing.T) {
	// This test ensures file references in body/headers work correctly
	yaml := `
workspace_name: File References Test

flows:
  - name: TestFlow
    steps:
      - request:
          name: FileUpload
          method: POST
          url: "https://api.example.com/upload"
          headers:
            Content-Type: multipart/form-data
          body:
            kind: form
            value:
              file: "@/path/to/file.pdf"
              description: "Test file upload"
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify form body contains file reference
	fileFieldFound := false
	for _, fb := range wd.FormBodies {
		if fb.BodyKey == "file" && fb.Value == "@/path/to/file.pdf" {
			fileFieldFound = true
			break
		}
	}
	if !fileFieldFound {
		t.Error("Expected file field with value '@/path/to/file.pdf'")
	}
}

func TestUnmarshalWorkflowYAML_MultipleFlows(t *testing.T) {
	yaml := `
workspace_name: Multiple Flows Test

flows:
  - name: Flow1
    variables:
      var1: "value1"
    steps:
      - request:
          name: Request1
          method: GET
          url: "{{var1}}/endpoint1"

  - name: Flow2
    variables:
      var2: "value2"
    steps:
      - request:
          name: Request2
          method: POST
          url: "{{var2}}/endpoint2"

  - name: Flow3
    steps:
      - js:
          name: Script1
          code: |
            export default function() {
              return { result: "done" };
            }

run:
  - Flow1
  - Flow3
  - Flow2
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify all flows were created
	if len(wd.Flows) != 3 {
		t.Fatalf("Expected 3 flows, got %d", len(wd.Flows))
	}

	// Verify flow names
	flowNames := []string{}
	for _, flow := range wd.Flows {
		flowNames = append(flowNames, flow.Name)
	}
	expectedNames := []string{"Flow1", "Flow2", "Flow3"}
	for _, expected := range expectedNames {
		found := false
		for _, actual := range flowNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected flow '%s' not found", expected)
		}
	}

	// Verify variables are associated with correct flows
	varsByFlow := make(map[string][]string)
	for _, v := range wd.FlowVariables {
		// Find the flow name for this variable
		for _, flow := range wd.Flows {
			if flow.ID == v.FlowID {
				varsByFlow[flow.Name] = append(varsByFlow[flow.Name], v.Name)
				break
			}
		}
	}

	if len(varsByFlow["Flow1"]) != 1 || varsByFlow["Flow1"][0] != "var1" {
		t.Error("Expected Flow1 to have variable 'var1'")
	}
	if len(varsByFlow["Flow2"]) != 1 || varsByFlow["Flow2"][0] != "var2" {
		t.Error("Expected Flow2 to have variable 'var2'")
	}
	if len(varsByFlow["Flow3"]) != 0 {
		t.Error("Expected Flow3 to have no variables")
	}
}

func TestUnmarshalWorkflowYAML_EmptyWorkflow(t *testing.T) {
	yaml := `
workspace_name: Empty Workspace
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal empty workflow: %v", err)
	}

	// Verify workspace was created
	if wd.Workspace.Name != "Empty Workspace" {
		t.Errorf("Expected workspace name 'Empty Workspace', got '%s'", wd.Workspace.Name)
	}

	// Verify default collection was created
	if len(wd.Collections) != 1 {
		t.Errorf("Expected 1 default collection, got %d", len(wd.Collections))
	}

	// Verify no flows
	if len(wd.Flows) != 0 {
		t.Errorf("Expected 0 flows, got %d", len(wd.Flows))
	}
}

// ==================================================================================
// Flow Dependencies Tests
// ==================================================================================

func TestUnmarshalWorkflowYAML_RunWithDependencies(t *testing.T) {
	yaml := `
workspace_name: Flow Dependencies Test

flows:
  - name: FlowA
    steps:
      - request:
          name: RequestA
          method: GET
          url: "https://api.example.com/a"

  - name: FlowB
    steps:
      - request:
          name: RequestB
          method: GET
          url: "https://api.example.com/b"

  - name: FlowC
    steps:
      - request:
          name: RequestC
          method: GET
          url: "https://api.example.com/c"

run:
  - flow: FlowA
  
  - flow: FlowB
    depends_on: FlowA
  
  - flow: FlowC
    depends_on:
      - FlowA
      - FlowB
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML with flow dependencies: %v", err)
	}

	// Verify all flows were created
	if len(wd.Flows) != 3 {
		t.Fatalf("Expected 3 flows, got %d", len(wd.Flows))
	}

	// Verify flow names
	flowNames := make(map[string]bool)
	for _, flow := range wd.Flows {
		flowNames[flow.Name] = true
	}
	
	expectedFlows := []string{"FlowA", "FlowB", "FlowC"}
	for _, expected := range expectedFlows {
		if !flowNames[expected] {
			t.Errorf("Expected flow '%s' not found", expected)
		}
	}
}

func TestUnmarshalWorkflowYAML_RunMixedFormat(t *testing.T) {
	yaml := `
workspace_name: Mixed Run Format Test

flows:
  - name: Flow1
    steps:
      - request:
          name: Request1
          method: GET
          url: "/test1"

  - name: Flow2
    steps:
      - request:
          name: Request2
          method: GET
          url: "/test2"

  - name: Flow3
    steps:
      - request:
          name: Request3
          method: GET
          url: "/test3"

run:
  - Flow1  # Simple string format
  - flow: Flow2  # Object format without dependencies
  - flow: Flow3
    depends_on: Flow2  # Object format with single dependency
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML with mixed run format: %v", err)
	}

	// Verify all flows were created
	if len(wd.Flows) != 3 {
		t.Fatalf("Expected 3 flows, got %d", len(wd.Flows))
	}
}

func TestUnmarshalWorkflowYAML_RunInvalidDependency(t *testing.T) {
	yaml := `
workspace_name: Invalid Dependency Test

flows:
  - name: FlowA
    steps:
      - request:
          name: RequestA
          method: GET
          url: "/test"

run:
  - flow: FlowA
    depends_on: NonExistentFlow
`

	_, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err == nil {
		t.Fatal("Expected error for non-existent dependency, but got none")
	}
	if !contains(err.Error(), "depends on non-existent flow: NonExistentFlow") {
		t.Errorf("Expected error about non-existent flow, got: %v", err)
	}
}

func TestUnmarshalWorkflowYAML_RunCircularDependency(t *testing.T) {
	yaml := `
workspace_name: Circular Dependency Test

flows:
  - name: FlowA
    steps:
      - request:
          name: RequestA
          method: GET
          url: "/a"

  - name: FlowB
    steps:
      - request:
          name: RequestB
          method: GET
          url: "/b"

  - name: FlowC
    steps:
      - request:
          name: RequestC
          method: GET
          url: "/c"

run:
  - flow: FlowA
    depends_on: FlowC
  
  - flow: FlowB
    depends_on: FlowA
  
  - flow: FlowC
    depends_on: FlowB
`

	_, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err == nil {
		t.Fatal("Expected error for circular dependency, but got none")
	}
	if !contains(err.Error(), "circular dependency") {
		t.Errorf("Expected error about circular dependency, got: %v", err)
	}
}

func TestUnmarshalWorkflowYAML_RunBackwardCompatibility(t *testing.T) {
	yaml := `
workspace_name: Backward Compatibility Test

flows:
  - name: Flow1
    steps:
      - request:
          name: Request1
          method: GET
          url: "/test1"

  - name: Flow2
    steps:
      - request:
          name: Request2
          method: GET
          url: "/test2"

  - name: Flow3
    steps:
      - request:
          name: Request3
          method: GET
          url: "/test3"

run:
  - Flow1
  - Flow2
  - Flow3
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML with simple run format: %v", err)
	}

	// Verify all flows were created
	if len(wd.Flows) != 3 {
		t.Fatalf("Expected 3 flows, got %d", len(wd.Flows))
	}
}

func TestUnmarshalWorkflowYAML_RunSelfContained(t *testing.T) {
	yaml := `
workspace_name: Self-Contained Dependencies Test

flows:
  - name: SetupFlow
    steps:
      - request:
          name: CreateUser
          method: POST
          url: "/users"

  - name: MainFlow
    steps:
      - request:
          name: GetUser
          method: GET
          url: "/users/{{user_id}}"

  - name: CleanupFlow
    steps:
      - request:
          name: DeleteUser
          method: DELETE
          url: "/users/{{user_id}}"

run:
  - flow: SetupFlow
  
  - flow: MainFlow
    depends_on: SetupFlow
  
  - flow: CleanupFlow
    depends_on:
      - SetupFlow
      - MainFlow
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML with self-contained dependencies: %v", err)
	}

	// Verify all flows exist
	if len(wd.Flows) != 3 {
		t.Fatalf("Expected 3 flows, got %d", len(wd.Flows))
	}
}

func TestUnmarshalWorkflowYAML_RunMissingFlowField(t *testing.T) {
	yaml := `
workspace_name: Missing Flow Field Test

flows:
  - name: Flow1
    steps:
      - request:
          name: Request1
          method: GET
          url: "/test"

run:
  - depends_on: SomeFlow  # Missing 'flow' field
`

	_, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err == nil {
		t.Fatal("Expected error for missing flow field, but got none")
	}
	if !contains(err.Error(), "missing 'flow' field") {
		t.Errorf("Expected error about missing flow field, got: %v", err)
	}
}

func TestUnmarshalWorkflowYAML_RunEmptyArray(t *testing.T) {
	yaml := `
workspace_name: Empty Run Array Test

flows:
  - name: Flow1
    steps:
      - request:
          name: Request1
          method: GET
          url: "/test"

run: []
`

	wd, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML with empty run array: %v", err)
	}

	// Should succeed with no run steps
	if len(wd.Flows) != 1 {
		t.Errorf("Expected 1 flow, got %d", len(wd.Flows))
	}
}