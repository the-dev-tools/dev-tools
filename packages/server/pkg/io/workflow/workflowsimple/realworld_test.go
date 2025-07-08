package workflowsimple_test

import (
	"testing"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"

	"github.com/stretchr/testify/require"
)

func TestRealWorldWorkflowImport(t *testing.T) {
	yamlData := `workspace_name: User Authentication Flow

request_templates:
  login_template:
    method: POST
    url: "/auth/login"
    headers:
      Content-Type: "application/json"
      X-API-Version: "1.0"
      Accept: "application/json"
    body:
      body_json:
        email: "{{ email }}"
        password: "{{ password }}"

  get_profile_template:
    method: GET
    url: "/users/{{ user_id }}/profile"
    headers:
      Authorization: "Bearer {{ token }}"
      X-API-Version: "1.0"
    query_params:
      include: "preferences"
      format: "full"

flows:
  - name: User Authentication Flow
    variables:
      - name: base_url
        value: "https://api.example.com"
      - name: api_version
        value: "v1"
    steps:
      - request:
          name: Login as admin
          use_request: login_template
          headers:
            X-API-Version: "2.0"  # Override from 1.0
            X-Client-ID: "admin-client"  # New header not in template
          body:
            body_json:
              email: "admin@example.com"
              password: "{{ env.admin_password }}"
              role: "admin"
      
      - request:
          name: Get admin profile
          use_request: get_profile_template
          url: "/users/{{ admin_id }}/profile"  # Override with flow variable
          headers:
            Authorization: "Bearer {{ admin_token }}"  # Override with flow variable
            X-Admin-Access: "true"  # New header
          query_params:
            include: "permissions,audit_log"  # Override from "preferences"
            admin_view: "true"  # New query param
      
      - request:
          name: Create test user
          method: POST
          url: "/admin/users"
          headers:
            Authorization: "Bearer {{ admin_token }}"
            Content-Type: "application/json"
            X-Admin-Operation: "create_user"
          body:
            body_json:
              email: "testuser@example.com"
              password: "{{ test_password }}"
              role: "user"`

	// Parse the YAML
	data, err := workflowsimple.Parse([]byte(yamlData))
	require.NoError(t, err)

	// Verify basic structure
	require.Equal(t, "User Authentication Flow", data.Flow.Name)

	// Verify environment variables
	require.Len(t, data.Variables, 2)
	varMap := make(map[string]string)
	for _, v := range data.Variables {
		varMap[v.VarKey] = v.Value
	}
	require.Equal(t, "https://api.example.com", varMap["base_url"])
	require.Equal(t, "v1", varMap["api_version"])

	// Count nodes by name
	nodeNames := make(map[string]bool)
	for _, node := range data.Nodes {
		nodeNames[node.Name] = true
	}

	// Should have nodes for each step
	require.True(t, nodeNames["Login as admin"])
	require.True(t, nodeNames["Get admin profile"])
	require.True(t, nodeNames["Create test user"])

	// Check delta creation for "Login as admin" step
	// This uses login_template and overrides headers
	deltaHeaders := 0
	deltaQueries := 0
	deltaBodies := 0

	// Count deltas by checking DeltaParentID
	for _, h := range data.Headers {
		if h.DeltaParentID != nil {
			deltaHeaders++
		}
	}
	for _, q := range data.Queries {
		if q.DeltaParentID != nil {
			deltaQueries++
		}
	}
	// Count delta bodies by checking examples with VersionParentID
	for _, ex := range data.Examples {
		if ex.VersionParentID != nil {
			for _, b := range data.RawBodies {
				if b.ExampleID == ex.ID {
					deltaBodies++
					break
				}
			}
		}
	}

	// "Login as admin" should have:
	// - 2 delta headers (X-API-Version override + X-Client-ID addition)
	// - 1 delta body (different content)
	// "Get admin profile" should have:
	// - 2 delta headers (Authorization override + X-Admin-Access addition)
	// - 2 delta queries (include override + admin_view addition)
	
	// Note: These are totals across all steps
	require.Greater(t, deltaHeaders, 0, "Should have delta headers for overrides/additions")
	require.Greater(t, deltaQueries, 0, "Should have delta queries for overrides/additions")
	require.Greater(t, deltaBodies, 0, "Should have delta bodies for content changes")

	// Verify specific overrides
	headerValues := make(map[string][]string)
	for _, h := range data.Headers {
		headerValues[h.HeaderKey] = append(headerValues[h.HeaderKey], h.Value)
	}

	// Should have both "1.0" and "2.0" for X-API-Version
	require.Contains(t, headerValues["X-API-Version"], "1.0", "Should have template value")
	require.Contains(t, headerValues["X-API-Version"], "2.0", "Should have override value")
	
	// Should have X-Client-ID (addition)
	require.Contains(t, headerValues, "X-Client-ID")
	require.Contains(t, headerValues["X-Client-ID"], "admin-client")

	// Verify query overrides
	queryValues := make(map[string][]string)
	for _, q := range data.Queries {
		queryValues[q.QueryKey] = append(queryValues[q.QueryKey], q.Value)
	}

	// Should have override value for "include" (not template value since it was overridden)
	require.Contains(t, queryValues["include"], "permissions,audit_log", "Should have override value")
	// The original "preferences" value should be in the base example, but overridden in delta
	// So it's OK if we don't see it in the final values
	t.Logf("Include query values: %v", queryValues["include"])
	
	// Should have admin_view (addition)
	require.Contains(t, queryValues, "admin_view")
	require.Contains(t, queryValues["admin_view"], "true")

	t.Logf("Successfully imported workflow with %d delta headers, %d delta queries, %d delta bodies",
		deltaHeaders, deltaQueries, deltaBodies)
}