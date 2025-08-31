package thar_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/translate/thar"
)

func TestJSONBodyTemplating(t *testing.T) {
	t.Run("JSON_body_with_UUID_dependency_should_be_templated", func(t *testing.T) {
		// Scenario: Create a category, then create a product with that category ID
		harData := `{
			"log": {
				"entries": [
					{
						"startedDateTime": "2024-01-01T10:00:00.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/categories",
							"headers": [
								{"name": "Content-Type", "value": "application/json"}
							],
							"postData": {
								"mimeType": "application/json",
								"text": "{\"name\":\"Electronics\",\"description\":\"Electronic products\"}"
							}
						},
						"response": {
							"status": 201,
							"headers": [],
							"content": {
								"text": "{\"id\":\"550e8400-e29b-41d4-a716-446655440000\",\"name\":\"Electronics\",\"createdAt\":\"2024-01-01T10:00:00Z\"}"
							}
						}
					},
					{
						"startedDateTime": "2024-01-01T10:00:01.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/products",
							"headers": [
								{"name": "Content-Type", "value": "application/json"}
							],
							"postData": {
								"mimeType": "application/json",
								"text": "{\"name\":\"Laptop\",\"price\":999.99,\"categoryId\":\"550e8400-e29b-41d4-a716-446655440000\"}"
							}
						},
						"response": {
							"status": 201,
							"headers": [],
							"content": {
								"text": "{\"id\":\"660e8400-e29b-41d4-a716-446655440001\",\"name\":\"Laptop\",\"categoryId\":\"550e8400-e29b-41d4-a716-446655440000\"}"
							}
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

		// Find the delta raw body for the second request (product creation)
		var deltaBody *mbodyraw.ExampleBodyRaw
		for i := range resolved.RawBodies {
			body := &resolved.RawBodies[i]
			// Find the delta example for the second request
			for _, example := range resolved.Examples {
				if example.Name == "products (Delta)" && body.ExampleID == example.ID {
					deltaBody = body
					break
				}
			}
		}

		require.NotNil(t, deltaBody, "Should find delta body for product creation")

		// Decompress if needed
		var bodyData []byte
		if deltaBody.CompressType == compress.CompressTypeZstd {
			decompressed, err := compress.Decompress(deltaBody.Data, compress.CompressTypeZstd)
			require.NoError(t, err)
			bodyData = decompressed
		} else {
			bodyData = deltaBody.Data
		}

		t.Logf("Delta body content: %s", string(bodyData))

		// Parse the JSON to check if it's templated
		var bodyJSON map[string]interface{}
		err = json.Unmarshal(bodyData, &bodyJSON)
		require.NoError(t, err)

		// The categoryId should be templated
		categoryID, ok := bodyJSON["categoryId"].(string)
		require.True(t, ok, "Should have categoryId field")
		require.Equal(t, "{{ request_0.response.body.id }}", categoryID, 
			"Category ID in JSON body should be replaced with template variable")

		// Other fields should remain unchanged
		require.Equal(t, "Laptop", bodyJSON["name"])
		require.Equal(t, 999.99, bodyJSON["price"])
	})

	t.Run("Nested_JSON_values_should_be_templated", func(t *testing.T) {
		harData := `{
			"log": {
				"entries": [
					{
						"startedDateTime": "2024-01-01T10:00:00.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/auth/login",
							"headers": [
								{"name": "Content-Type", "value": "application/json"}
							],
							"postData": {
								"mimeType": "application/json",
								"text": "{\"username\":\"admin\",\"password\":\"secret\"}"
							}
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {
								"text": "{\"user\":{\"id\":\"user-123\",\"role\":\"admin\"},\"session\":{\"token\":\"abc-xyz-123\",\"expiresAt\":\"2024-01-02T10:00:00Z\"}}"
							}
						}
					},
					{
						"startedDateTime": "2024-01-01T10:00:01.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/actions",
							"headers": [
								{"name": "Content-Type", "value": "application/json"}
							],
							"postData": {
								"mimeType": "application/json",
								"text": "{\"action\":\"create\",\"context\":{\"userId\":\"user-123\",\"sessionToken\":\"abc-xyz-123\"},\"data\":{\"type\":\"report\"}}"
							}
						},
						"response": {
							"status": 201,
							"headers": [],
							"content": {
								"text": "{\"success\":true}"
							}
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

		// Find the delta raw body for the second request
		var deltaBody *mbodyraw.ExampleBodyRaw
		for i := range resolved.RawBodies {
			body := &resolved.RawBodies[i]
			// Find the delta example for the second request
			for _, example := range resolved.Examples {
				if example.Name == "actions (Delta)" && body.ExampleID == example.ID {
					deltaBody = body
					break
				}
			}
		}

		require.NotNil(t, deltaBody, "Should find delta body for action creation")

		// Decompress if needed
		var bodyData []byte
		if deltaBody.CompressType == compress.CompressTypeZstd {
			decompressed, err := compress.Decompress(deltaBody.Data, compress.CompressTypeZstd)
			require.NoError(t, err)
			bodyData = decompressed
		} else {
			bodyData = deltaBody.Data
		}

		t.Logf("Delta body content: %s", string(bodyData))

		// Parse the JSON to check if nested values are templated
		var bodyJSON map[string]interface{}
		err = json.Unmarshal(bodyData, &bodyJSON)
		require.NoError(t, err)

		// Check nested context object
		context, ok := bodyJSON["context"].(map[string]interface{})
		require.True(t, ok, "Should have context object")
		
		require.Equal(t, "{{ request_0.response.body.user.id }}", context["userId"],
			"Nested userId should be templated")
		require.Equal(t, "{{ request_0.response.body.session.token }}", context["sessionToken"],
			"Nested sessionToken should be templated")

		// Check that edges were created for dependencies
		var foundEdges int
		for _, edge := range resolved.Edges {
			if edge.TargetID == resolved.RequestNodes[1].FlowNodeID {
				foundEdges++
			}
		}
		require.Greater(t, foundEdges, 0, "Should have created edges for JSON body dependencies")
	})

	t.Run("Arrays_in_JSON_should_be_templated", func(t *testing.T) {
		harData := `{
			"log": {
				"entries": [
					{
						"startedDateTime": "2024-01-01T10:00:00.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "GET",
							"url": "https://api.example.com/users",
							"headers": []
						},
						"response": {
							"status": 200,
							"headers": [],
							"content": {
								"text": "{\"users\":[{\"id\":\"user-1\",\"name\":\"Alice\"},{\"id\":\"user-2\",\"name\":\"Bob\"}]}"
							}
						}
					},
					{
						"startedDateTime": "2024-01-01T10:00:01.000Z",
						"_resourceType": "xhr",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/groups",
							"headers": [
								{"name": "Content-Type", "value": "application/json"}
							],
							"postData": {
								"mimeType": "application/json",
								"text": "{\"name\":\"Team A\",\"members\":[\"user-1\",\"user-2\"]}"
							}
						},
						"response": {
							"status": 201,
							"headers": [],
							"content": {
								"text": "{\"id\":\"group-1\"}"
							}
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

		// Find the delta raw body for the second request
		var deltaBody *mbodyraw.ExampleBodyRaw
		for i := range resolved.RawBodies {
			body := &resolved.RawBodies[i]
			// Find the delta example for the second request
			for _, example := range resolved.Examples {
				if example.Name == "groups (Delta)" && body.ExampleID == example.ID {
					deltaBody = body
					break
				}
			}
		}

		require.NotNil(t, deltaBody, "Should find delta body for group creation")

		// Decompress if needed
		var bodyData []byte
		if deltaBody.CompressType == compress.CompressTypeZstd {
			decompressed, err := compress.Decompress(deltaBody.Data, compress.CompressTypeZstd)
			require.NoError(t, err)
			bodyData = decompressed
		} else {
			bodyData = deltaBody.Data
		}

		t.Logf("Delta body content: %s", string(bodyData))

		// Parse the JSON to check if array values are templated
		var bodyJSON map[string]interface{}
		err = json.Unmarshal(bodyData, &bodyJSON)
		require.NoError(t, err)

		members, ok := bodyJSON["members"].([]interface{})
		require.True(t, ok, "Should have members array")
		require.Len(t, members, 2, "Should have 2 members")
		
		// Array values should be templated
		require.Equal(t, "{{ request_0.response.body.users[0].id }}", members[0],
			"First member ID should be templated")
		require.Equal(t, "{{ request_0.response.body.users[1].id }}", members[1],
			"Second member ID should be templated")
	})
}