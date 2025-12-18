package tpostmanv2

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"

	"github.com/stretchr/testify/require"
)

// TestIntegration_SimpleWorkflow tests a complete workflow from parsing to conversion
func TestIntegration_SimpleWorkflow(t *testing.T) {
	// Test with a realistic Postman collection
	collectionJSON := `{
		"info": {
			"name": "API Collection",
			"description": "Sample API collection for testing"
		},
		"item": [
			{
				"name": "Authentication",
				"item": [
					{
						"name": "Login",
						"request": {
							"method": "POST",
							"header": [
								{
									"key": "Content-Type",
									"value": "application/json"
								}
							],
							"body": {
								"mode": "raw",
								"raw": "{\"username\": \"test@example.com\", \"password\": \"secret123\"}"
							},
							"url": {
								"raw": "https://api.example.com/auth/login",
								"protocol": "https",
								"host": ["api", "example", "com"],
								"path": ["auth", "login"]
							}
						},
						"response": [
							{
								"name": "Success",
								"originalRequest": {
									"method": "POST",
									"url": {
										"raw": "https://api.example.com/auth/login"
									}
								},
								"status": "OK",
								"code": 200,
								"header": [
									{
										"key": "Content-Type",
										"value": "application/json"
									}
								],
								"body": "{\"token\": \"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...\"}"
							}
						]
					}
				]
			},
			{
				"name": "Users",
				"request": {
					"method": "GET",
					"header": [
						{
							"key": "Authorization",
							"value": "Bearer {{token}}"
						},
						{
							"key": "Accept",
							"value": "application/json",
							"description": "Specify JSON response format",
							"disabled": true
						}
					],
					"url": {
						"raw": "https://api.example.com/users?page=1&limit=20&sort=created_at",
						"protocol": "https",
						"host": ["api", "example", "com"],
						"path": ["users"],
						"query": [
							{
								"key": "page",
								"value": "1",
								"description": "Page number for pagination"
							},
							{
								"key": "limit",
								"value": "20",
								"description": "Number of items per page"
							},
							{
								"key": "sort",
								"value": "created_at",
								"description": "Sort order"
							}
						]
					}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "API Collection",
	}

	// Parse the collection
	parsed, err := ParsePostmanCollection([]byte(collectionJSON))
	require.NoError(t, err, "ParsePostmanCollection() error")

	require.Equal(t, "API Collection", parsed.Info.Name, "Expected collection name 'API Collection'")
	require.Len(t, parsed.Item, 2, "Expected 2 top-level items")

	// Convert to modern HTTP models
	resolved, err := ConvertPostmanCollection([]byte(collectionJSON), opts)
	require.NoError(t, err, "ConvertPostmanCollection() error")

	// Verify we got the right number of HTTP requests
	require.Len(t, resolved.HTTPRequests, 2, "Expected 2 HTTP requests")

	// Test first request (Login)
	loginReq := resolved.HTTPRequests[0]
	require.Equal(t, "Login", loginReq.Name, "Expected first request name 'Login'")
	require.Equal(t, "POST", loginReq.Method, "Expected login method 'POST'")
	require.Equal(t, "https://api.example.com/auth/login", loginReq.Url, "Expected login URL")

	// Verify login request has raw body
	loginBodyRaw := extractBodyRawForHTTP(loginReq.ID, resolved.BodyRaw)
	require.NotNil(t, loginBodyRaw, "Expected login request to have raw body")
	expectedLoginBody := `{"username": "test@example.com", "password": "secret123"}`
	require.Equal(t, expectedLoginBody, string(loginBodyRaw.RawData), "Expected login body")

	// Verify login request has content-type header
	loginHeaders := extractHeadersForHTTP(loginReq.ID, resolved.Headers)
	require.Len(t, loginHeaders, 1, "Expected 1 header for login request")
	require.Equal(t, "Content-Type", loginHeaders[0].Key, "Expected login header key 'Content-Type'")
	require.Equal(t, "application/json", loginHeaders[0].Value, "Expected login header value 'application/json'")

	// Test second request (Users)
	usersReq := resolved.HTTPRequests[1]
	require.Equal(t, "Users", usersReq.Name, "Expected second request name 'Users'")
	require.Equal(t, "GET", usersReq.Method, "Expected users method 'GET'")

	// Verify users request has search parameters
	usersSearchParams := extractSearchParamsForHTTP(usersReq.ID, resolved.SearchParams)
	require.Len(t, usersSearchParams, 3, "Expected 3 search parameters for users request")

	// Verify specific search parameters
	paramMap := make(map[string]string)
	for _, param := range usersSearchParams {
		paramMap[param.Key] = param.Value
	}

	require.Equal(t, "1", paramMap["page"], "Expected page parameter '1'")
	require.Equal(t, "20", paramMap["limit"], "Expected limit parameter '20'")
	require.Equal(t, "created_at", paramMap["sort"], "Expected sort parameter 'created_at'")

	// Verify users request has only enabled headers (disabled header should be filtered out)
	usersHeaders := extractHeadersForHTTP(usersReq.ID, resolved.Headers)
	require.Len(t, usersHeaders, 1, "Expected 1 enabled header for users request")
	require.Equal(t, "Authorization", usersHeaders[0].Key, "Expected users header key 'Authorization'")

	// Verify files were created for each HTTP request
	require.Len(t, resolved.Files, 2, "Expected 2 files created")

	// Verify file names match request names
	fileNames := make(map[string]bool)
	for _, file := range resolved.Files {
		fileNames[file.Name] = true
	}
	require.True(t, fileNames["Login"], "Expected file named 'Login'")
	require.True(t, fileNames["Users"], "Expected file named 'Users'")
}

// TestIntegration_RoundTrip tests that we can convert to HTTP models and back to Postman format
func TestIntegration_RoundTrip(t *testing.T) {
	// Original collection
	originalCollection := `{
		"info": {"name": "Round Trip Test"},
		"item": [
			{
				"name": "Test Request",
				"request": {
					"method": "POST",
					"header": [
						{
							"key": "Content-Type",
							"value": "application/json"
						}
					],
					"body": {
						"mode": "raw",
						"raw": "{\"test\": \"data\"}"
					},
					"url": {
						"raw": "https://api.example.com/test?param=value",
						"query": [
							{
								"key": "param",
								"value": "value"
							}
						]
					}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Round Trip Test",
	}

	// Convert to HTTP models
	resolved, err := ConvertPostmanCollection([]byte(originalCollection), opts)
	require.NoError(t, err, "ConvertPostmanCollection() error")

	// Convert back to Postman format
	generatedJSON, err := BuildPostmanCollection(resolved)
	require.NoError(t, err, "BuildPostmanCollection() error")

	// Parse the generated collection to verify it's valid
	generatedCollection, err := ParsePostmanCollection(generatedJSON)
	require.NoError(t, err, "Failed to parse generated collection")

	// Basic verification
	require.Equal(t, "Generated Collection", generatedCollection.Info.Name, "Expected generated collection name 'Generated Collection'")
	require.Len(t, generatedCollection.Item, 1, "Expected 1 item in generated collection")

	generatedItem := generatedCollection.Item[0]
	require.Equal(t, "Test Request", generatedItem.Name, "Expected generated item name 'Test Request'")
	require.Equal(t, "POST", generatedItem.Request.Method, "Expected generated request method 'POST'")
}
