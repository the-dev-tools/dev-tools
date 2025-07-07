package workflowsimple

import (
	"testing"
	"gopkg.in/yaml.v3"
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
      - name: Content-Type
        value: application/json
    body:
      body_json:
        username: admin
        password: secret123
  api_template:
    headers:
      - name: Authorization
        value: Bearer {{token}}
      - name: Accept
        value: application/json
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
            - name: X-Custom
              value: custom-value
`)

	// Import to get full workspace data
	workspaceData, err := ImportWorkflowYAML(yamlData)
	if err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	// Export back to YAML
	exportedYAML, err := ExportWorkflowYAMLOld(workspaceData)
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
				if name == "Login" {
					loginStep = req
				} else if name == "Get User" {
					getUserStep = req
				}
			}
		}
	}

	// Verify Login step has auth_template data
	if loginStep == nil {
		t.Fatal("Login step not found")
	}
	if loginStep["method"] != "POST" {
		t.Errorf("Login: expected method POST, got %v", loginStep["method"])
	}
	if loginStep["url"] != "https://api.example.com/auth/login" {
		t.Errorf("Login: expected auth URL, got %v", loginStep["url"])
	}
	
	// Check Login headers
	if headers, ok := loginStep["headers"].([]any); ok {
		if len(headers) != 1 {
			t.Errorf("Login: expected 1 header, got %d", len(headers))
		}
	} else {
		t.Error("Login: no headers found")
	}

	// Check Login body
	if body, ok := loginStep["body"].(map[string]any); ok {
		if bodyJSON, ok := body["body_json"].(map[string]any); ok {
			if bodyJSON["username"] != "admin" {
				t.Errorf("Login: expected username 'admin', got %v", bodyJSON["username"])
			}
		} else {
			t.Error("Login: no body_json found")
		}
	} else {
		t.Error("Login: no body found")
	}

	// Verify Get User step has merged data
	if getUserStep == nil {
		t.Fatal("Get User step not found")
	}
	if getUserStep["method"] != "GET" {
		t.Errorf("Get User: expected method GET, got %v", getUserStep["method"])
	}
	if getUserStep["url"] != "https://api.example.com/users/{{user_id}}" {
		t.Errorf("Get User: expected user URL with variable, got %v", getUserStep["url"])
	}
	
	// Check Get User headers (should have both template and override headers)
	if headers, ok := getUserStep["headers"].([]any); ok {
		// Should have Authorization, Accept from template + X-Custom override
		if len(headers) < 3 {
			t.Errorf("Get User: expected at least 3 headers, got %d", len(headers))
		}
		
		// Check for specific headers
		hasAuth := false
		hasAccept := false
		hasCustom := false
		
		for _, h := range headers {
			if hMap, ok := h.(map[string]any); ok {
				name, _ := hMap["name"].(string)
				switch name {
				case "Authorization":
					hasAuth = true
					if hMap["value"] != "Bearer {{token}}" {
						t.Errorf("Authorization header has wrong value: %v", hMap["value"])
					}
				case "Accept":
					hasAccept = true
				case "X-Custom":
					hasCustom = true
					if hMap["value"] != "custom-value" {
						t.Errorf("X-Custom header has wrong value: %v", hMap["value"])
					}
				}
			}
		}
		
		if !hasAuth {
			t.Error("Get User: Authorization header not found")
		}
		if !hasAccept {
			t.Error("Get User: Accept header not found")
		}
		if !hasCustom {
			t.Error("Get User: X-Custom header not found")
		}
	} else {
		t.Error("Get User: no headers found")
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