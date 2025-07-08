package workflowsimple

import (
	"testing"
	"strings"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBaseDeltaImportExportRoundtrip(t *testing.T) {
	originalYAML := `workspace_name: Test Roundtrip

requests:
  - name: api_request
    method: POST
    url: https://api.example.com/auth
    headers:
      Content-Type: application/json
      Authorization: Bearer static-token-12345
    body:
      username: admin
      password: secret

flows:
  - name: Auth Flow
    variables:
      - name: dynamic_token
        value: test-token
    steps:
      - request:
          name: login
          use_request: api_request
      - request:
          name: get_data
          use_request: api_request
          headers:
            Authorization: Bearer {{ login.response.body.token }}
          body:
            username: "{{ username }}"
            password: "{{ password }}"`

	// Convert to ioworkspace format
	workspaceData, err := ImportWorkflowYAML([]byte(originalYAML))
	require.NoError(t, err)
	
	// Export back to YAML
	exportedYAML, err := ExportWorkflowClean(workspaceData)
	require.NoError(t, err)
	
	t.Logf("Exported YAML:\n%s", string(exportedYAML))
	
	// Parse the exported YAML to verify structure
	var exported map[string]any
	err = yaml.Unmarshal(exportedYAML, &exported)
	require.NoError(t, err)
	
	// Check that requests section has static values
	requests := exported["requests"].([]any)
	require.Greater(t, len(requests), 0, "Should have requests")
	
	// Find a request with the static token (either login or get_data)
	var apiRequest map[string]any
	for _, r := range requests {
		req := r.(map[string]any)
		if headers, ok := req["headers"].(map[string]any); ok {
			if auth, ok := headers["Authorization"].(string); ok && strings.Contains(auth, "static-token-12345") {
				apiRequest = req
				break
			}
		}
	}
	require.NotNil(t, apiRequest, "Should find a request with static token in exported YAML")
	
	// Check headers in the request template
	headers := apiRequest["headers"].(map[string]any)
	authHeader := headers["Authorization"].(string)
	require.Equal(t, "Bearer static-token-12345", authHeader,
		"Request template should have static token, not variable reference")
	require.NotContains(t, authHeader, "{{",
		"Request template should not contain variable references")
	
	// Check that flow steps have the overrides
	flows := exported["flows"].([]any)
	flow := flows[0].(map[string]any)
	steps := flow["steps"].([]any)
	
	// Find get_data step
	var getDataStep map[string]any
	for _, s := range steps {
		step := s.(map[string]any)
		if req, ok := step["request"].(map[string]any); ok {
			if req["name"] == "get_data" {
				getDataStep = req
				break
			}
		}
	}
	require.NotNil(t, getDataStep, "Should find get_data step")
	
	// Check that the step has the override
	stepHeaders := getDataStep["headers"].(map[string]any)
	stepAuthHeader := stepHeaders["Authorization"].(string)
	require.Equal(t, "Bearer {{ login.response.body.token }}", stepAuthHeader,
		"Step should have variable reference override")
	
	// Verify the step body has overrides too
	stepBody := getDataStep["body"].(map[string]any)
	require.Equal(t, "{{ username }}", stepBody["username"],
		"Step body should have username variable")
	require.Equal(t, "{{ password }}", stepBody["password"],
		"Step body should have password variable")
}

func TestBaseDeltaPreservesHardcodedTokens(t *testing.T) {
	// This test uses the exact YAML format from the user's example
	yamlWithHardcodedTokens := `workspace_name: E-commerce Admin

requests:
  - name: login_request
    method: POST
    url: https://ecommerce-admin-panel.fly.dev/api/auth/login
    headers:
      Content-Type: application/json
      Accept: '*/*'
    body:
      email: admin@example.com
      password: admin123
      
  - name: get_categories
    method: GET
    url: https://ecommerce-admin-panel.fly.dev/api/categories
    headers:
      Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjU5MmFiNzc0LWU3ODMtNDQ5NS05Yjc3LWFkODU2ODlmODRkNyIsImlhdCI6MTc0ODQzODkyMSwiZXhwIjoxNzQ4NTI1MzIxfQ.nRJ8x6ItgC8aOXj8P8jonmjwwOgs2lVTCOd7-KbYlxQ
      Content-Type: application/json

flows:
  - name: Category Management
    steps:
      - request:
          name: login
          use_request: login_request
      - request:
          name: list_categories
          use_request: get_categories
          headers:
            Authorization: Bearer {{ login.response.body.token }}`
            
	// Parse and convert
	workspaceData, err := ImportWorkflowYAML([]byte(yamlWithHardcodedTokens))
	require.NoError(t, err)
	
	// Export back
	exportedYAML, err := ExportWorkflowClean(workspaceData)
	require.NoError(t, err)
	
	// Verify the hardcoded token is preserved in the template
	require.Contains(t, string(exportedYAML), "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"Exported YAML should contain the hardcoded JWT token in the request template")
	
	// Verify the override uses variable
	require.Contains(t, string(exportedYAML), "{{ login.response.body.token }}",
		"Exported YAML should contain the variable reference in the flow step")
	
	// Parse to check structure
	var exported map[string]any
	err = yaml.Unmarshal(exportedYAML, &exported)
	require.NoError(t, err)
	
	// Verify the get_categories template has the hardcoded token
	requests := exported["requests"].([]any)
	for _, r := range requests {
		req := r.(map[string]any)
		if strings.Contains(req["name"].(string), "categories") {
			headers := req["headers"].(map[string]any)
			authHeader := headers["Authorization"].(string)
			require.Contains(t, authHeader, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				"Categories request template should have hardcoded JWT token")
			break
		}
	}
}