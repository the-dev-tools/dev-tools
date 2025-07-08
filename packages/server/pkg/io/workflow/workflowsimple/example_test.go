package workflowsimple_test

import (
	"os"
	"testing"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"

	"github.com/stretchr/testify/require"
)

func TestExampleWorkflowImport(t *testing.T) {
	// Read the example YAML file
	yamlData, err := os.ReadFile("../../../../../example-workflow.yaml")
	if err != nil {
		t.Skip("Example workflow file not found")
	}

	// Parse the YAML
	data, err := workflowsimple.Parse(yamlData)
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

	// Should have both template and override values for "include"
	require.Contains(t, queryValues["include"], "preferences", "Should have template value")
	require.Contains(t, queryValues["include"], "permissions,audit_log", "Should have override value")
	
	// Should have admin_view (addition)
	require.Contains(t, queryValues, "admin_view")
	require.Contains(t, queryValues["admin_view"], "true")
}