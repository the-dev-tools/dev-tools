package workflowsimple_test

import (
	"testing"
	"the-dev-tools/server/pkg/io/workflow/workflowsimple"

	"github.com/stretchr/testify/require"
)

func TestDeltaCreationForOverrides(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantDeltas struct {
			headers int
			queries int
			bodies  int
		}
		checkOverrides func(t *testing.T, data *workflowsimple.WorkflowData)
	}{
		{
			name: "header override without variables",
			yaml: `
workspace_name: Test Delta Creation

request_templates:
  api_template:
    method: POST
    url: "/api/endpoint"
    headers:
      Content-Type: "application/json"
      X-API-Version: "1.0"
      Accept: "application/json"

flows:
  - name: TestFlow
    steps:
      - request:
          name: OverrideHeaders
          use_request: api_template
          headers:
            X-API-Version: "2.0"  # Override without variable
            X-Client-ID: "test-client"  # New header addition
`,
			wantDeltas: struct {
				headers int
				queries int
				bodies  int
			}{
				headers: 2, // X-API-Version override + X-Client-ID addition
				queries: 0,
				bodies:  1, // Always create delta body for flow execution
			},
			checkOverrides: func(t *testing.T, data *workflowsimple.WorkflowData) {
				// Find delta headers
				deltaHeaders := make(map[string]string)
				for _, h := range data.Headers {
					if h.DeltaParentID != nil {
						deltaHeaders[h.HeaderKey] = h.Value
					}
				}
				
				// Verify overrides
				require.Equal(t, "2.0", deltaHeaders["X-API-Version"], "X-API-Version should be overridden to 2.0")
				require.Equal(t, "test-client", deltaHeaders["X-Client-ID"], "X-Client-ID should be added")
			},
		},
		{
			name: "query parameter override and addition",
			yaml: `
workspace_name: Test Query Overrides

request_templates:
  search_template:
    method: GET
    url: "/api/search"
    query_params:
      limit: "10"
      format: "json"

flows:
  - name: TestFlow
    steps:
      - request:
          name: SearchOverride
          use_request: search_template
          query_params:
            limit: "50"  # Override
            sort: "desc"  # Addition
`,
			wantDeltas: struct {
				headers int
				queries int
				bodies  int
			}{
				headers: 0,
				queries: 2, // limit override + sort addition
				bodies:  1, // Always create delta body for flow execution
			},
			checkOverrides: func(t *testing.T, data *workflowsimple.WorkflowData) {
				// Find delta queries
				deltaQueries := make(map[string]string)
				for _, q := range data.Queries {
					if q.DeltaParentID != nil {
						deltaQueries[q.QueryKey] = q.Value
					}
				}
				
				require.Equal(t, "50", deltaQueries["limit"], "limit should be overridden to 50")
				require.Equal(t, "desc", deltaQueries["sort"], "sort should be added")
			},
		},
		{
			name: "body override without variables",
			yaml: `
workspace_name: Test Body Override

request_templates:
  create_user:
    method: POST
    url: "/api/users"
    body:
      body_json:
        role: "user"
        active: true

flows:
  - name: TestFlow
    steps:
      - request:
          name: CreateAdmin
          use_request: create_user
          body:
            body_json:
              role: "admin"  # Override
              active: true
              permissions: ["read", "write"]  # Addition
`,
			wantDeltas: struct {
				headers int
				queries int
				bodies  int
			}{
				headers: 0,
				queries: 0,
				bodies:  1, // Body content differs
			},
			checkOverrides: func(t *testing.T, data *workflowsimple.WorkflowData) {
				// Count delta bodies
				deltaBodies := 0
				for _, b := range data.RawBodies {
					// Check if this is a delta by looking at the example
					for _, ex := range data.Examples {
						if ex.ID == b.ExampleID && ex.VersionParentID != nil {
							deltaBodies++
							// Verify body contains admin role
							bodyStr := string(b.Data)
							require.Contains(t, bodyStr, `"role":"admin"`, "Body should have admin role")
							require.Contains(t, bodyStr, `"permissions"`, "Body should have permissions field")
						}
					}
				}
				require.Equal(t, 1, deltaBodies, "Should have one delta body")
			},
		},
		{
			name: "mixed overrides with variables",
			yaml: `
workspace_name: Test Mixed Overrides

request_templates:
  auth_template:
    method: POST
    url: "/api/auth"
    headers:
      X-API-Key: "default-key"
      Content-Type: "application/json"

flows:
  - name: TestFlow
    variables:
      - name: api_key
        value: "secret123"
    steps:
      - request:
          name: AuthRequest
          use_request: auth_template
          headers:
            X-API-Key: "{{ api_key }}"  # Override with variable
            X-Request-ID: "req-123"  # Addition without variable
`,
			wantDeltas: struct {
				headers int
				queries int
				bodies  int
			}{
				headers: 2, // Both should create deltas
				queries: 0,
				bodies:  1, // Always create delta body for flow execution
			},
			checkOverrides: func(t *testing.T, data *workflowsimple.WorkflowData) {
				deltaHeaders := make(map[string]string)
				for _, h := range data.Headers {
					if h.DeltaParentID != nil {
						deltaHeaders[h.HeaderKey] = h.Value
					}
				}
				
				require.Equal(t, "{{ api_key }}", deltaHeaders["X-API-Key"], "X-API-Key should have variable")
				require.Equal(t, "req-123", deltaHeaders["X-Request-ID"], "X-Request-ID should be added")
			},
		},
		{
			name: "no overrides should not create deltas",
			yaml: `
workspace_name: Test No Overrides

request_templates:
  get_template:
    method: GET
    url: "/api/data"
    headers:
      Accept: "application/json"

flows:
  - name: TestFlow
    steps:
      - request:
          name: GetData
          use_request: get_template
          # No overrides, using template as-is
`,
			wantDeltas: struct {
				headers int
				queries int
				bodies  int
			}{
				headers: 0, // No deltas expected
				queries: 0,
				bodies:  1, // Always create delta body for flow execution
			},
			checkOverrides: func(t *testing.T, data *workflowsimple.WorkflowData) {
				// Count delta items
				deltaCount := 0
				for _, h := range data.Headers {
					if h.DeltaParentID != nil {
						deltaCount++
					}
				}
				require.Equal(t, 0, deltaCount, "Should have no delta headers when no overrides")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := workflowsimple.Parse([]byte(tt.yaml))
			require.NoError(t, err)

			// Count actual deltas
			actualDeltas := struct {
				headers int
				queries int
				bodies  int
			}{}

			// Count delta headers
			for _, h := range data.Headers {
				if h.DeltaParentID != nil {
					actualDeltas.headers++
				}
			}

			// Count delta queries
			for _, q := range data.Queries {
				if q.DeltaParentID != nil {
					actualDeltas.queries++
				}
			}

			// Count delta bodies by checking examples
			for _, ex := range data.Examples {
				if ex.VersionParentID != nil {
					// Check if this delta example has a body
					for _, b := range data.RawBodies {
						if b.ExampleID == ex.ID {
							actualDeltas.bodies++
							break
						}
					}
				}
			}

			// Verify counts
			require.Equal(t, tt.wantDeltas.headers, actualDeltas.headers, 
				"Delta header count mismatch. Expected %d, got %d", tt.wantDeltas.headers, actualDeltas.headers)
			require.Equal(t, tt.wantDeltas.queries, actualDeltas.queries, 
				"Delta query count mismatch. Expected %d, got %d", tt.wantDeltas.queries, actualDeltas.queries)
			require.Equal(t, tt.wantDeltas.bodies, actualDeltas.bodies, 
				"Delta body count mismatch. Expected %d, got %d", tt.wantDeltas.bodies, actualDeltas.bodies)

			// Run custom checks
			if tt.checkOverrides != nil {
				tt.checkOverrides(t, data)
			}
		})
	}
}

func TestDeltaCreationComplexScenario(t *testing.T) {
	yaml := `
workspace_name: Complex Delta Test

request_templates:
  base_api:
    method: POST
    url: "/api/v1/resource"
    headers:
      Content-Type: "application/json"
      X-API-Version: "1.0"
      Accept: "application/json"
    query_params:
      format: "json"
      include: "basic"
    body:
      body_json:
        action: "create"
        metadata:
          source: "api"

flows:
  - name: ComplexFlow
    variables:
      - name: api_version
        value: "2.0"
      - name: token
        value: "bearer-secret"
    steps:
      - request:
          name: ComplexRequest
          use_request: base_api
          headers:
            X-API-Version: "{{ api_version }}"  # Override with variable
            Accept: "application/xml"  # Override without variable
            Authorization: "Bearer {{ token }}"  # New with variable
            X-Trace-ID: "trace-123"  # New without variable
          query_params:
            include: "full,audit"  # Override
            debug: "true"  # New
          body:
            body_json:
              action: "update"  # Override
              metadata:
                source: "api"
                user: "admin"  # New field
              timestamp: "{{ now }}"  # New field with variable
`

	data, err := workflowsimple.Parse([]byte(yaml))
	require.NoError(t, err)

	// Collect delta headers
	deltaHeaders := make(map[string]string)
	for _, h := range data.Headers {
		if h.DeltaParentID != nil {
			deltaHeaders[h.HeaderKey] = h.Value
		}
	}

	// Should have 4 delta headers (2 overrides + 2 additions)
	require.Len(t, deltaHeaders, 4, "Should have 4 delta headers")
	require.Equal(t, "{{ api_version }}", deltaHeaders["X-API-Version"])
	require.Equal(t, "application/xml", deltaHeaders["Accept"])
	require.Equal(t, "Bearer {{ token }}", deltaHeaders["Authorization"])
	require.Equal(t, "trace-123", deltaHeaders["X-Trace-ID"])

	// Collect delta queries
	deltaQueries := make(map[string]string)
	for _, q := range data.Queries {
		if q.DeltaParentID != nil {
			deltaQueries[q.QueryKey] = q.Value
		}
	}

	// Should have 2 delta queries (1 override + 1 addition)
	require.Len(t, deltaQueries, 2, "Should have 2 delta queries")
	require.Equal(t, "full,audit", deltaQueries["include"])
	require.Equal(t, "true", deltaQueries["debug"])

	// Check for delta body
	deltaBodies := 0
	for _, ex := range data.Examples {
		if ex.VersionParentID != nil {
			for _, b := range data.RawBodies {
				if b.ExampleID == ex.ID {
					deltaBodies++
					// Verify body content
					bodyStr := string(b.Data)
					require.Contains(t, bodyStr, `"action":"update"`, "Body action should be overridden")
					require.Contains(t, bodyStr, `"user":"admin"`, "Body should have new user field")
					require.Contains(t, bodyStr, `"timestamp":"{{ now }}"`, "Body should have timestamp with variable")
				}
			}
		}
	}
	require.Equal(t, 1, deltaBodies, "Should have 1 delta body")
}