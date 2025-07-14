package yamlflowsimple_test

import (
	"strings"
	"testing"
	yamlflowsimple "the-dev-tools/server/pkg/io/yamlflow/yamlflowsimple"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
		checkOverrides func(t *testing.T, data *yamlflowsimple.YamlFlowData)
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
			checkOverrides: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
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
			checkOverrides: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
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
			checkOverrides: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
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
			checkOverrides: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
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
			checkOverrides: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
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
			data, err := yamlflowsimple.Parse([]byte(tt.yaml))
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

	data, err := yamlflowsimple.Parse([]byte(yaml))
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

// Tests from base_delta_separation_test.go

func TestBaseDeltaSeparation(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, data *yamlflowsimple.YamlFlowData)
	}{
		{
			name: "template_with_header_override",
			yaml: `
workspace_name: Test Base Delta Separation

requests:
  - name: auth_request
    method: POST
    url: https://api.example.com/auth/login
    headers:
      Content-Type: application/json
      Authorization: Bearer hardcoded-token-12345
    body:
      username: admin@example.com
      password: admin123

flows:
  - name: Test Flow
    steps:
      - request:
          name: login
          use_request: auth_request
      - request:
          name: get_data
          use_request: auth_request
          headers:
            Authorization: Bearer {{ login.response.body.token }}
          body:
            username: "{{ username }}"
            password: "{{ password }}"`,
			validate: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
				// Find base headers for auth_request template
				var baseAuthHeader string
				var deltaAuthHeader string

				// Get the base example ID for get_data
				var baseExampleID, deltaExampleID string
				for _, node := range data.Nodes {
					if node.Name == "get_data" {
						// Find the request node for this node
						for _, reqNode := range data.RequestNodes {
							if reqNode.FlowNodeID == node.ID {
								// The base example is referenced by ExampleID
								if reqNode.ExampleID != nil {
									baseExampleID = reqNode.ExampleID.String()
								}
								if reqNode.DeltaExampleID != nil {
									deltaExampleID = reqNode.DeltaExampleID.String()
								}
								break
							}
						}
						break
					}
				}

				// Check headers
				for _, h := range data.Headers {
					if h.HeaderKey == "Authorization" {
						if h.ExampleID.String() == baseExampleID {
							baseAuthHeader = h.Value
						} else if h.ExampleID.String() == deltaExampleID {
							deltaAuthHeader = h.Value
						}
					}
				}

				// Base should have the hardcoded token from template
				require.Equal(t, "Bearer hardcoded-token-12345", baseAuthHeader,
					"Base header should contain hardcoded token from template")

				// Delta should have the variable reference
				require.Equal(t, "Bearer {{ login.response.body.token }}", deltaAuthHeader,
					"Delta header should contain variable reference from step override")
			},
		},
		{
			name: "template_with_body_override",
			yaml: `
workspace_name: Test Body Override

requests:
  - name: create_user
    method: POST
    url: https://api.example.com/users
    body:
      name: Default User
      email: default@example.com
      role: user

flows:
  - name: Create Admin
    variables:
      - name: admin_name
        value: Admin User
      - name: admin_email
        value: admin@example.com
    steps:
      - request:
          name: create_admin
          use_request: create_user
          body:
            name: "{{ admin_name }}"
            email: "{{ admin_email }}"
            role: admin`,
			validate: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
				// Find base and delta bodies
				var baseBodyData, deltaBodyData []byte

				// Get example IDs
				var baseExampleID, deltaExampleID string
				for _, node := range data.Nodes {
					if node.Name == "create_admin" {
						// Find the request node for this node
						for _, reqNode := range data.RequestNodes {
							if reqNode.FlowNodeID == node.ID {
								if reqNode.ExampleID != nil {
									baseExampleID = reqNode.ExampleID.String()
								}
								if reqNode.DeltaExampleID != nil {
									deltaExampleID = reqNode.DeltaExampleID.String()
								}
								break
							}
						}
						break
					}
				}

				// Get body data
				for _, body := range data.RawBodies {
					if body.ExampleID.String() == baseExampleID {
						baseBodyData = body.Data
					} else if body.ExampleID.String() == deltaExampleID {
						deltaBodyData = body.Data
					}
				}

				// Check base body has template values
				baseBodyStr := string(baseBodyData)
				require.Contains(t, baseBodyStr, "Default User",
					"Base body should contain template values")
				require.Contains(t, baseBodyStr, "default@example.com",
					"Base body should contain template email")
				require.Contains(t, baseBodyStr, "user",
					"Base body should contain template role")

				// Check delta body has overrides
				deltaBodyStr := string(deltaBodyData)
				require.Contains(t, deltaBodyStr, "{{ admin_name }}",
					"Delta body should contain variable reference")
				require.Contains(t, deltaBodyStr, "{{ admin_email }}",
					"Delta body should contain email variable")
				require.Contains(t, deltaBodyStr, "admin",
					"Delta body should contain overridden role")
			},
		},
		{
			name: "no_template_with_variables",
			yaml: `
workspace_name: Test No Template

flows:
  - name: API Flow
    variables:
      - name: token
        value: test-token
    steps:
      - request:
          name: api_call
          method: GET
          url: https://api.example.com/data
          headers:
            Authorization: Bearer {{ token }}`,
			validate: func(t *testing.T, data *yamlflowsimple.YamlFlowData) {
				// When no template, base should have the variable reference
				var baseAuthHeader, deltaAuthHeader string

				// Get example IDs
				var baseExampleID, deltaExampleID string
				for _, example := range data.Examples {
					if example.Name == "api_call" && example.IsDefault {
						baseExampleID = example.ID.String()
					}
				}

				for _, reqNode := range data.RequestNodes {
					if reqNode.DeltaExampleID != nil {
						deltaExampleID = reqNode.DeltaExampleID.String()
					}
				}

				// Check headers
				for _, h := range data.Headers {
					if h.HeaderKey == "Authorization" {
						if h.ExampleID.String() == baseExampleID {
							baseAuthHeader = h.Value
						} else if h.ExampleID.String() == deltaExampleID {
							deltaAuthHeader = h.Value
						}
					}
				}

				// Both should have the variable reference since no template
				require.Equal(t, "Bearer {{ token }}", baseAuthHeader,
					"Base header should have variable when no template")
				require.Equal(t, "Bearer {{ token }}", deltaAuthHeader,
					"Delta header should have variable")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yamlflowsimple.Parse([]byte(tt.yaml))
			require.NoError(t, err)

			tt.validate(t, data)
		})
	}
}

// Tests from base_delta_roundtrip_test.go

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
	workspaceData, err := yamlflowsimple.ImportYamlFlowYAML([]byte(originalYAML))
	require.NoError(t, err)

	// Export back to YAML
	exportedYAML, err := yamlflowsimple.ExportYamlFlowYAML(workspaceData)
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
	workspaceData, err := yamlflowsimple.ImportYamlFlowYAML([]byte(yamlWithHardcodedTokens))
	require.NoError(t, err)

	// Export back
	exportedYAML, err := yamlflowsimple.ExportYamlFlowYAML(workspaceData)
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

// Tests from delta_rawbody_test.go

func TestDeltaRawBodyAlwaysCreated(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		desc string
	}{
		{
			name: "request_with_body",
			yaml: `
workspace_name: Test Delta Raw Bodies

flows:
  - name: Test Flow
    steps:
      - request:
          name: create_user
          method: POST
          url: https://api.example.com/users
          body:
            name: John Doe
            email: john@example.com`,
			desc: "Request with body should create delta raw body",
		},
		{
			name: "request_without_body",
			yaml: `
workspace_name: Test Delta Raw Bodies

flows:
  - name: Test Flow
    steps:
      - request:
          name: get_users
          method: GET
          url: https://api.example.com/users`,
			desc: "Request without body should create empty delta raw body",
		},
		{
			name: "request_with_template_override",
			yaml: `
workspace_name: Test Delta Raw Bodies

requests:
  - name: user_template
    method: POST
    url: https://api.example.com/users
    body:
      name: Default User
      email: default@example.com

flows:
  - name: Test Flow
    steps:
      - request:
          name: create_user
          use_request: user_template
          body:
            name: "{{ username }}"
            email: "{{ email }}"`,
			desc: "Request with template override should create delta raw body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yamlflowsimple.Parse([]byte(tt.yaml))
			require.NoError(t, err)

			// Find all delta examples by checking request nodes
			deltaExampleIDs := []string{}
			for _, reqNode := range data.RequestNodes {
				if reqNode.DeltaExampleID != nil {
					deltaExampleIDs = append(deltaExampleIDs, reqNode.DeltaExampleID.String())
				}
			}

			require.Greater(t, len(deltaExampleIDs), 0, "Should have delta examples")

			// Verify each delta example has a corresponding raw body
			for _, deltaID := range deltaExampleIDs {
				foundDeltaBody := false
				for _, rawBody := range data.RawBodies {
					if rawBody.ExampleID.String() == deltaID {
						foundDeltaBody = true
						break
					}
				}
				require.True(t, foundDeltaBody, "Delta example %s should have a corresponding raw body", deltaID)
			}

			// Verify we have exactly 3 examples per request (base, default, delta)
			require.Equal(t, 3, len(data.Examples), "Should have exactly 3 examples (base, default, delta)")

			// Verify we have exactly 3 raw bodies (one for each example)
			require.Equal(t, 3, len(data.RawBodies), "Should have exactly 3 raw bodies (one for each example)")

			// Also verify request nodes have delta endpoint and example IDs
			for _, reqNode := range data.RequestNodes {
				require.NotNil(t, reqNode.DeltaEndpointID, "Request node should have delta endpoint ID")
				require.NotNil(t, reqNode.DeltaExampleID, "Request node should have delta example ID")
			}
		})
	}
}
