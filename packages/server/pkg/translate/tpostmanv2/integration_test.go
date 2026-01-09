package tpostmanv2

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"

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

	// Verify we got the right number of HTTP requests (Base + Delta for each)
	require.Len(t, resolved.HTTPRequests, 4, "Expected 4 HTTP requests (2 items * 2)")

	// Test first request (Login)
	loginReq := resolved.HTTPRequests[0]
	require.Equal(t, "Login", loginReq.Name, "Expected first request name 'Login'")
	require.Equal(t, "POST", loginReq.Method, "Expected login method 'POST'")
	require.Equal(t, "https://api.example.com/auth/login", loginReq.Url, "Expected login URL")

	// Test second request (Users)
	usersReq := resolved.HTTPRequests[2]
	require.Equal(t, "Users", usersReq.Name, "Expected second request name 'Users'")
	require.Equal(t, "GET", usersReq.Method, "Expected users method 'GET'")

	// Verify files were created for each HTTP request, folders (including deltas), URL folders, and flow file
	// Now includes: 1 Postman folder + 4 request files + URL-based folders + 1 flow file
	require.GreaterOrEqual(t, len(resolved.Files), 6, "Expected at least 6 files (folders + requests + flow)")

	// Verify file names match request names and folder names
	fileNames := make(map[string]bool)
	var authFolderID idwrap.IDWrap
	for _, file := range resolved.Files {
		fileNames[file.Name] = true
		if file.Name == "Authentication" {
			authFolderID = file.ID
		}
	}
	require.True(t, fileNames["Login"], "Expected file named 'Login'")
	require.True(t, fileNames["Users"], "Expected file named 'Users'")
	require.True(t, fileNames["Authentication"], "Expected file named 'Authentication'")

	// Verify hierarchy
	for _, file := range resolved.Files {
		if file.Name == "Login" {
			require.NotNil(t, file.ParentID, "Login request should be in a folder")
			require.Equal(t, 0, file.ParentID.Compare(authFolderID), "Login request should be in Authentication folder")
		}
		if file.Name == "Users" && file.ContentType == mfile.ContentTypeHTTP {
			// Users request should now be in a URL-based folder (not at root)
			require.NotNil(t, file.ParentID, "Users request should be in URL-based folder")
		}
	}
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
