package thar_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/translate/thar"
)

func TestUUIDTemplatingInURLs(t *testing.T) {
	t.Run("UUID_in_URL_path_should_be_templated", func(t *testing.T) {
		// Create a HAR with two requests - first creates a resource, second uses its UUID
		harData := `{
			"log": {
				"entries": [
					{
						"startedDateTime": "2024-01-01T10:00:00.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/users",
							"headers": []
						},
						"response": {
							"status": 201,
							"headers": [],
							"content": {
								"text": "{\"id\":\"550e8400-e29b-41d4-a716-446655440000\",\"name\":\"John\"}"
							}
						}
					},
					{
						"startedDateTime": "2024-01-01T10:00:01.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "GET",
							"url": "https://api.example.com/users/550e8400-e29b-41d4-a716-446655440000",
							"headers": []
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {"text": "{}"}
						}
					}
				]
			}
		}`

		collectionID := idwrap.NewNow()
		workspaceID := idwrap.NewNow()

		har, err := thar.ConvertRaw([]byte(harData))
		require.NoError(t, err)

		resolved, err := thar.ConvertHAR(har, collectionID, workspaceID)
		require.NoError(t, err)

		// Find the second API (GET request)
		var getAPI *mitemapi.ItemApi
		for i := range resolved.Apis {
			api := &resolved.Apis[i]
			if api.Method == "GET" && strings.Contains(api.Url, "users") && api.DeltaParentID == nil {
				getAPI = api
				break
			}
		}

		require.NotNil(t, getAPI, "Should find GET API")
		
		// The URL should have the UUID templated
		t.Logf("GET API URL: %s", getAPI.Url)
		require.Contains(t, getAPI.Url, "{{ request_0.response.body.id }}", 
			"UUID in URL should be replaced with template variable")
		
		// Check that an edge was created between the two nodes
		var foundEdge bool
		for _, edge := range resolved.Edges {
			if edge.TargetID == resolved.RequestNodes[1].FlowNodeID {
				foundEdge = true
				t.Logf("Found edge from node %s to node %s", edge.SourceID, edge.TargetID)
				break
			}
		}
		require.True(t, foundEdge, "Should have created edge showing dependency")
	})

	t.Run("Multiple_UUIDs_in_URL_should_be_templated", func(t *testing.T) {
		harData := `{
			"log": {
				"entries": [
					{
						"startedDateTime": "2024-01-01T10:00:00.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/orgs",
							"headers": []
						},
						"response": {
							"status": 201,
							"headers": [],
							"content": {
								"text": "{\"orgId\":\"11111111-1111-1111-1111-111111111111\",\"projectId\":\"22222222-2222-2222-2222-222222222222\"}"
							}
						}
					},
					{
						"startedDateTime": "2024-01-01T10:00:01.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "GET",
							"url": "https://api.example.com/orgs/11111111-1111-1111-1111-111111111111/projects/22222222-2222-2222-2222-222222222222",
							"headers": []
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {"text": "{}"}
						}
					}
				]
			}
		}`

		collectionID := idwrap.NewNow()
		workspaceID := idwrap.NewNow()

		har, err := thar.ConvertRaw([]byte(harData))
		require.NoError(t, err)

		resolved, err := thar.ConvertHAR(har, collectionID, workspaceID)
		require.NoError(t, err)

		// Find the second API
		var getAPI *mitemapi.ItemApi
		for i := range resolved.Apis {
			api := &resolved.Apis[i]
			if api.Method == "GET" && strings.Contains(api.Url, "projects") && api.DeltaParentID == nil {
				getAPI = api
				break
			}
		}

		require.NotNil(t, getAPI)
		t.Logf("GET API URL: %s", getAPI.Url)
		
		// Both UUIDs should be templated
		require.Contains(t, getAPI.Url, "{{ request_0.response.body.orgId }}")
		require.Contains(t, getAPI.Url, "{{ request_0.response.body.projectId }}")
	})

	t.Run("Bearer_token_in_header_should_be_templated", func(t *testing.T) {
		harData := `{
			"log": {
				"entries": [
					{
						"startedDateTime": "2024-01-01T10:00:00.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/auth/login",
							"headers": []
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {
								"text": "{\"token\":\"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U\"}"
							}
						}
					},
					{
						"startedDateTime": "2024-01-01T10:00:01.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "GET",
							"url": "https://api.example.com/profile",
							"headers": [
								{"name": "Authorization", "value": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"}
							]
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {"text": "{}"}
						}
					}
				]
			}
		}`

		collectionID := idwrap.NewNow()
		workspaceID := idwrap.NewNow()

		har, err := thar.ConvertRaw([]byte(harData))
		require.NoError(t, err)

		resolved, err := thar.ConvertHAR(har, collectionID, workspaceID)
		require.NoError(t, err)

		// Find delta headers for the second request
		var authHeader *mexampleheader.Header
		for _, header := range resolved.Headers {
			if header.HeaderKey == "Authorization" && header.DeltaParentID != nil {
				authHeader = &header
				break
			}
		}

		require.NotNil(t, authHeader, "Should find delta Authorization header")
		t.Logf("Authorization header value: %s", authHeader.Value)
		
		// The Bearer token should be templated
		require.Equal(t, "Bearer {{ request_0.response.body.token }}", authHeader.Value,
			"Bearer token should be replaced with template variable")
	})

	t.Run("Query_parameters_with_dependencies_should_be_templated", func(t *testing.T) {
		harData := `{
			"log": {
				"entries": [
					{
						"startedDateTime": "2024-01-01T10:00:00.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/session",
							"headers": []
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {
								"text": "{\"sessionId\":\"abc123def456\"}"
							}
						}
					},
					{
						"startedDateTime": "2024-01-01T10:00:01.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "GET",
							"url": "https://api.example.com/data",
							"queryString": [
								{"name": "session", "value": "abc123def456"},
								{"name": "format", "value": "json"}
							],
							"headers": []
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {"text": "{}"}
						}
					}
				]
			}
		}`

		collectionID := idwrap.NewNow()
		workspaceID := idwrap.NewNow()

		har, err := thar.ConvertRaw([]byte(harData))
		require.NoError(t, err)

		resolved, err := thar.ConvertHAR(har, collectionID, workspaceID)
		require.NoError(t, err)

		// Find delta queries for the second request
		var sessionQuery *mexamplequery.Query
		for _, query := range resolved.Queries {
			if query.QueryKey == "session" && query.DeltaParentID != nil {
				sessionQuery = &query
				break
			}
		}

		require.NotNil(t, sessionQuery, "Should find delta session query parameter")
		t.Logf("Session query value: %s", sessionQuery.Value)
		
		// The session ID should be templated
		require.Equal(t, "{{ request_0.response.body.sessionId }}", sessionQuery.Value,
			"Session ID in query should be replaced with template variable")
	})
}

func TestDepFinderIntegration(t *testing.T) {
	t.Run("DepFinder_should_track_response_values", func(t *testing.T) {
		df := depfinder.NewDepFinder()
		nodeID := idwrap.NewNow()
		
		// Simulate adding response data from first request
		responseData := `{
			"id": "550e8400-e29b-41d4-a716-446655440000",
			"token": "secret-token-123",
			"nested": {
				"value": "nested-value"
			}
		}`
		
		err := df.AddJsonBytes([]byte(responseData), depfinder.VarCouple{
			Path:   "request_0.response.body",
			NodeID: nodeID,
		})
		require.NoError(t, err)
		
		// Test that values can be found
		couple, err := df.FindVar("550e8400-e29b-41d4-a716-446655440000")
		require.NoError(t, err)
		require.Equal(t, "request_0.response.body.id", couple.Path)
		
		couple, err = df.FindVar("secret-token-123")
		require.NoError(t, err)
		require.Equal(t, "request_0.response.body.token", couple.Path)
		
		couple, err = df.FindVar("nested-value")
		require.NoError(t, err)
		require.Equal(t, "request_0.response.body.nested.value", couple.Path)
	})

	t.Run("URL_templating_should_replace_UUIDs", func(t *testing.T) {
		df := depfinder.NewDepFinder()
		nodeID := idwrap.NewNow()
		
		// Add a UUID to the depfinder
		uuid := "550e8400-e29b-41d4-a716-446655440000"
		df.AddVar(uuid, depfinder.VarCouple{
			Path:   "request_0.response.body.id",
			NodeID: nodeID,
		})
		
		// Test URL templating
		url := "https://api.example.com/users/550e8400-e29b-41d4-a716-446655440000/profile"
		templatedURL, found, couples := df.ReplaceURLPathParams(url)
		
		require.True(t, found, "Should find UUID in URL")
		require.Equal(t, "https://api.example.com/users/{{ request_0.response.body.id }}/profile", templatedURL)
		require.Len(t, couples, 1)
		require.Equal(t, "request_0.response.body.id", couples[0].Path)
	})
}