package workflowsimple

import (
	"testing"
	"github.com/stretchr/testify/require"
)

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
			data, err := Parse([]byte(tt.yaml))
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

