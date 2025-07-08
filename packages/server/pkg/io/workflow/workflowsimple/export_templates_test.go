package workflowsimple

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestExportWithRequestTemplates(t *testing.T) {
	// Test YAML with request templates
	yamlData := []byte(`
workspace_name: Template Test
request_templates:
  auth_template:
    method: POST
    url: https://api.example.com/auth/login
    headers:
      Content-Type: application/json
    body:
      username: admin
      password: secret123
  api_template:
    headers:
      Authorization: Bearer {{token}}
      Accept: application/json
flows:
  - name: Template Flow
    variables:
      - name: token
        value: test-token
      - name: user_id
        value: "123"
    steps:
      - request:
          name: Login
          use_request: auth_template
      - request:
          name: Get User
          use_request: api_template
          method: GET
          url: https://api.example.com/users/{{user_id}}
          headers:
            X-Custom: custom-value
`)

	// Import to get full workspace data
	workspaceData, err := ImportWorkflowYAML(yamlData)
	if err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	// Export back to YAML using clean export
	exportedYAML, err := ExportWorkflowYAML(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Parse exported YAML
	var exported WorkflowFormat
	if err := yaml.Unmarshal(exportedYAML, &exported); err != nil {
		t.Fatalf("failed to parse exported YAML: %v", err)
	}

	// Log the exported YAML
	t.Logf("Exported YAML:\n%s", string(exportedYAML))

	// Verify the export includes all data
	if len(exported.Flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(exported.Flows))
	}

	flow := exported.Flows[0]

	// Check variables
	if len(flow.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(flow.Variables))
	}

	// Check steps
	if len(flow.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(flow.Steps))
	}

	// Find Login step
	var loginStep map[string]any
	var getUserStep map[string]any

	for _, step := range flow.Steps {
		if stepMap, ok := step.(map[string]any); ok {
			if req, ok := stepMap["request"].(map[string]any); ok {
				name, _ := req["name"].(string)
				switch name {
				case "Login":
					loginStep = req
				case "Get User":
					getUserStep = req
				}
			}
		}
	}

	// Verify Login step references the request
	if loginStep == nil {
		t.Fatal("Login step not found")
	}
	if loginStep["use_request"] != "Login" {
		t.Errorf("Login: expected use_request 'Login', got %v", loginStep["use_request"])
	}
	
	// Check the requests section for Login data
	requests := exported.Requests
	if len(requests) == 0 {
		t.Fatal("No requests found in requests section")
	}
	
	var loginRequest map[string]any
	for _, req := range requests {
		if req["name"] == "Login" {
			loginRequest = req
			break
		}
	}
	
	if loginRequest == nil {
		t.Fatal("Login request not found in requests section")
	}
	
	// Verify Login request has auth_template data
	if loginRequest["method"] != "POST" {
		t.Errorf("Login request: expected method POST, got %v", loginRequest["method"])
	}
	if loginRequest["url"] != "https://api.example.com/auth/login" {
		t.Errorf("Login request: expected auth URL, got %v", loginRequest["url"])
	}
	
	// Check Login headers in request
	if headers, ok := loginRequest["headers"].(map[string]any); ok {
		if headers["Content-Type"] != "application/json" {
			t.Errorf("Login request: expected Content-Type header, got %v", headers["Content-Type"])
		}
	} else {
		t.Error("Login request: no headers found")
	}
	
	// Check Login body in request
	if body, ok := loginRequest["body"].(map[string]any); ok {
		if body["username"] != "admin" {
			t.Errorf("Login request: expected username 'admin', got %v", body["username"])
		}
		if body["password"] != "secret123" {
			t.Errorf("Login request: expected password 'secret123', got %v", body["password"])
		}
	} else {
		t.Error("Login request: no body found")
	}

	// Verify Get User step references the request and has overrides
	if getUserStep == nil {
		t.Fatal("Get User step not found")
	}
	if getUserStep["use_request"] != "Get User" {
		t.Errorf("Get User: expected use_request 'Get User', got %v", getUserStep["use_request"])
	}
	
	// The clean export merges everything into the request definition when using templates
	// So the step won't have method/url/headers - they'll all be in the request section
	
	// Find Get User in requests section
	var getUserRequest map[string]any
	for _, req := range requests {
		if req["name"] == "Get User" {
			getUserRequest = req
			break
		}
	}
	
	if getUserRequest == nil {
		t.Fatal("Get User request not found in requests section")
	}
	
	// Check headers in request (should have all headers merged)
	if headers, ok := getUserRequest["headers"].(map[string]any); ok {
		if headers["Authorization"] != "Bearer {{token}}" {
			t.Errorf("Get User request: expected Authorization header from template, got %v", headers["Authorization"])
		}
		if headers["Accept"] != "application/json" {
			t.Errorf("Get User request: expected Accept header from template, got %v", headers["Accept"])
		}
		// When using templates, all headers are merged into the request definition
		if headers["X-Custom"] != "custom-value" {
			t.Errorf("Get User request: expected X-Custom header, got %v", headers["X-Custom"])
		}
	} else {
		t.Error("Get User request: no headers found")
	}
}

func TestExportEmptyFieldsNotIncluded(t *testing.T) {
	// Test that empty arrays/maps are not included in export
	yamlData := []byte(`
workspace_name: Minimal Test
flows:
  - name: Minimal Flow
    steps:
      - request:
          name: Simple Request
          method: GET
          url: https://api.example.com/status
`)

	// Import and export
	workspaceData, err := ImportWorkflowYAML(yamlData)
	if err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	exportedYAML, err := ExportWorkflowYAML(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Convert to string and check it doesn't contain empty arrays
	exportedStr := string(exportedYAML)

	// These should not appear in minimal export
	if contains(exportedStr, "headers: []") {
		t.Error("exported YAML contains empty headers array")
	}
	if contains(exportedStr, "query_params: []") {
		t.Error("exported YAML contains empty query_params array")
	}
	if contains(exportedStr, "variables: []") {
		t.Error("exported YAML contains empty variables array")
	}

	t.Logf("Minimal export:\n%s", exportedStr)
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				len(s) > len(substr) && findSubstring(s[1:len(s)-1], substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
