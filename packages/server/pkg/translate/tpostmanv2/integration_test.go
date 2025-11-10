package tpostmanv2

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
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
	if err != nil {
		t.Fatalf("ParsePostmanCollection() error = %v", err)
	}

	if parsed.Info.Name != "API Collection" {
		t.Errorf("Expected collection name 'API Collection', got '%s'", parsed.Info.Name)
	}
	if len(parsed.Item) != 2 {
		t.Fatalf("Expected 2 top-level items, got %d", len(parsed.Item))
	}

	// Convert to modern HTTP models
	resolved, err := ConvertPostmanCollection([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertPostmanCollection() error = %v", err)
	}

	// Verify we got the right number of HTTP requests
	if len(resolved.HTTPRequests) != 2 {
		t.Fatalf("Expected 2 HTTP requests, got %d", len(resolved.HTTPRequests))
	}

	// Test first request (Login)
	loginReq := resolved.HTTPRequests[0]
	if loginReq.Name != "Login" {
		t.Errorf("Expected first request name 'Login', got '%s'", loginReq.Name)
	}
	if loginReq.Method != "POST" {
		t.Errorf("Expected login method 'POST', got '%s'", loginReq.Method)
	}
	if loginReq.Url != "https://api.example.com/auth/login" {
		t.Errorf("Expected login URL 'https://api.example.com/auth/login', got '%s'", loginReq.Url)
	}

	// Verify login request has raw body
	loginBodyRaw := extractBodyRawForHTTP(loginReq.ID, resolved.BodyRaw)
	if loginBodyRaw == nil {
		t.Fatal("Expected login request to have raw body")
	}
	expectedLoginBody := `{"username": "test@example.com", "password": "secret123"}`
	if string(loginBodyRaw.RawData) != expectedLoginBody {
		t.Errorf("Expected login body '%s', got '%s'", expectedLoginBody, string(loginBodyRaw.RawData))
	}

	// Verify login request has content-type header
	loginHeaders := extractHeadersForHTTP(loginReq.ID, resolved.Headers)
	if len(loginHeaders) != 1 {
		t.Fatalf("Expected 1 header for login request, got %d", len(loginHeaders))
	}
	if loginHeaders[0].Key != "Content-Type" {
		t.Errorf("Expected login header key 'Content-Type', got '%s'", loginHeaders[0].Key)
	}
	if loginHeaders[0].Value != "application/json" {
		t.Errorf("Expected login header value 'application/json', got '%s'", loginHeaders[0].Value)
	}

	// Test second request (Users)
	usersReq := resolved.HTTPRequests[1]
	if usersReq.Name != "Users" {
		t.Errorf("Expected second request name 'Users', got '%s'", usersReq.Name)
	}
	if usersReq.Method != "GET" {
		t.Errorf("Expected users method 'GET', got '%s'", usersReq.Method)
	}

	// Verify users request has search parameters
	usersSearchParams := extractSearchParamsForHTTP(usersReq.ID, resolved.SearchParams)
	if len(usersSearchParams) != 3 {
		t.Fatalf("Expected 3 search parameters for users request, got %d", len(usersSearchParams))
	}

	// Verify specific search parameters
	paramMap := make(map[string]string)
	for _, param := range usersSearchParams {
		paramMap[param.Key] = param.Value
	}

	if paramMap["page"] != "1" {
		t.Errorf("Expected page parameter '1', got '%s'", paramMap["page"])
	}
	if paramMap["limit"] != "20" {
		t.Errorf("Expected limit parameter '20', got '%s'", paramMap["limit"])
	}
	if paramMap["sort"] != "created_at" {
		t.Errorf("Expected sort parameter 'created_at', got '%s'", paramMap["sort"])
	}

	// Verify users request has only enabled headers (disabled header should be filtered out)
	usersHeaders := extractHeadersForHTTP(usersReq.ID, resolved.Headers)
	if len(usersHeaders) != 1 {
		t.Fatalf("Expected 1 enabled header for users request, got %d", len(usersHeaders))
	}
	if usersHeaders[0].Key != "Authorization" {
		t.Errorf("Expected users header key 'Authorization', got '%s'", usersHeaders[0].Key)
	}

	// Verify files were created for each HTTP request
	if len(resolved.Files) != 2 {
		t.Fatalf("Expected 2 files created, got %d", len(resolved.Files))
	}

	// Verify file names match request names
	fileNames := make(map[string]bool)
	for _, file := range resolved.Files {
		fileNames[file.Name] = true
	}
	if !fileNames["Login"] {
		t.Error("Expected file named 'Login'")
	}
	if !fileNames["Users"] {
		t.Error("Expected file named 'Users'")
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
	if err != nil {
		t.Fatalf("ConvertPostmanCollection() error = %v", err)
	}

	// Convert back to Postman format
	generatedJSON, err := BuildPostmanCollection(resolved)
	if err != nil {
		t.Fatalf("BuildPostmanCollection() error = %v", err)
	}

	// Parse the generated collection to verify it's valid
	generatedCollection, err := ParsePostmanCollection(generatedJSON)
	if err != nil {
		t.Fatalf("Failed to parse generated collection: %v", err)
	}

	// Basic verification
	if generatedCollection.Info.Name != "Generated Collection" {
		t.Errorf("Expected generated collection name 'Generated Collection', got '%s'", generatedCollection.Info.Name)
	}
	if len(generatedCollection.Item) != 1 {
		t.Fatalf("Expected 1 item in generated collection, got %d", len(generatedCollection.Item))
	}

	generatedItem := generatedCollection.Item[0]
	if generatedItem.Name != "Test Request" {
		t.Errorf("Expected generated item name 'Test Request', got '%s'", generatedItem.Name)
	}
	if generatedItem.Request.Method != "POST" {
		t.Errorf("Expected generated request method 'POST', got '%s'", generatedItem.Request.Method)
	}
}
