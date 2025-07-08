package workflowsimple

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestBaseDeltaSeparation(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, data *WorkflowData)
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
			validate: func(t *testing.T, data *WorkflowData) {
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
			validate: func(t *testing.T, data *WorkflowData) {
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
			validate: func(t *testing.T, data *WorkflowData) {
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
			data, err := Parse([]byte(tt.yaml))
			require.NoError(t, err)
			
			tt.validate(t, data)
		})
	}
}