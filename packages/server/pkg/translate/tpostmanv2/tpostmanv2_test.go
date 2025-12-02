package tpostmanv2

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
)

func TestConvertPostmanCollection_SimpleRequest(t *testing.T) {
	// Simple Postman collection with one GET request
	collectionJSON := `{
		"info": {
			"name": "Test Collection",
			"description": "A simple test collection"
		},
		"item": [
			{
				"name": "Get Users",
				"request": {
					"method": "GET",
					"header": [
						{
							"key": "Accept",
							"value": "application/json"
						}
					],
					"url": {
						"raw": "https://api.example.com/users?page=1&limit=10",
						"protocol": "https",
						"host": ["api", "example", "com"],
						"path": ["users"],
						"query": [
							{
								"key": "page",
								"value": "1"
							},
							{
								"key": "limit",
								"value": "10"
							}
						]
					}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Test Collection",
	}

	resolved, err := ConvertPostmanCollection([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertPostmanCollection() error = %v", err)
	}

	if len(resolved.HTTPRequests) != 1 {
		t.Fatalf("Expected 1 HTTP request, got %d", len(resolved.HTTPRequests))
	}

	httpReq := resolved.HTTPRequests[0]
	if httpReq.Name != "Get Users" {
		t.Errorf("Expected name 'Get Users', got '%s'", httpReq.Name)
	}
	if httpReq.Method != "GET" {
		t.Errorf("Expected method 'GET', got '%s'", httpReq.Method)
	}
	if httpReq.Url != "https://api.example.com/users" {
		t.Errorf("Expected URL 'https://api.example.com/users', got '%s'", httpReq.Url)
	}

	// Check search parameters
	if len(resolved.SearchParams) != 2 {
		t.Fatalf("Expected 2 search parameters, got %d", len(resolved.SearchParams))
	}

	// Check headers
	if len(resolved.Headers) != 1 {
		t.Fatalf("Expected 1 header, got %d", len(resolved.Headers))
	}
	header := resolved.Headers[0]
	if header.Key != "Accept" {
		t.Errorf("Expected header key 'Accept', got '%s'", header.Key)
	}
	if header.Value != "application/json" {
		t.Errorf("Expected header value 'application/json', got '%s'", header.Value)
	}

	// Check files
	if len(resolved.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(resolved.Files))
	}
	file := resolved.Files[0]
	if file.Name != "Get Users" {
		t.Errorf("Expected file name 'Get Users', got '%s'", file.Name)
	}
	if file.ContentType != mfile.ContentTypeHTTP {
		t.Errorf("Expected content type %d, got %d", mfile.ContentTypeHTTP, file.ContentType)
	}
}

func TestConvertPostmanCollection_RequestBodyModes(t *testing.T) {
	// Collection with different body modes
	collectionJSON := `{
		"info": {
			"name": "Body Test Collection"
		},
		"item": [
			{
				"name": "Raw Body Request",
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
						"raw": "{\"name\": \"John\", \"age\": 30}"
					},
					"url": {
						"raw": "https://api.example.com/users"
					}
				}
			},
			{
				"name": "Form Data Request",
				"request": {
					"method": "POST",
					"body": {
						"mode": "formdata",
						"formdata": [
							{
								"key": "name",
								"value": "John Doe"
							},
							{
								"key": "email",
								"value": "john@example.com"
							}
						]
					},
					"url": {
						"raw": "https://api.example.com/submit"
					}
				}
			},
			{
				"name": "URL Encoded Request",
				"request": {
					"method": "POST",
					"body": {
						"mode": "urlencoded",
						"urlencoded": [
							{
								"key": "username",
								"value": "johndoe"
							},
							{
								"key": "password",
								"value": "secret123"
							}
						]
					},
					"url": {
						"raw": "https://api.example.com/login"
					}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Body Test Collection",
	}

	resolved, err := ConvertPostmanCollection([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertPostmanCollection() error = %v", err)
	}

	if len(resolved.HTTPRequests) != 3 {
		t.Fatalf("Expected 3 HTTP requests, got %d", len(resolved.HTTPRequests))
	}

	// Test raw body
	rawBodyReq := resolved.HTTPRequests[0]
	if rawBodyReq.Method != "POST" {
		t.Errorf("Expected POST method for raw body request, got %s", rawBodyReq.Method)
	}
	if len(resolved.BodyRaw) != 1 {
		t.Fatalf("Expected 1 raw body, got %d", len(resolved.BodyRaw))
	}
	rawBody := resolved.BodyRaw[0]
	expectedRawData := []byte(`{"name": "John", "age": 30}`)
	if string(rawBody.RawData) != string(expectedRawData) {
		t.Errorf("Expected raw body data '%s', got '%s'", string(expectedRawData), string(rawBody.RawData))
	}

	// Test form data
	if len(resolved.BodyForms) != 2 {
		t.Fatalf("Expected 2 form fields, got %d", len(resolved.BodyForms))
	}

	// Test URL encoded
	if len(resolved.BodyUrlencoded) != 2 {
		t.Fatalf("Expected 2 URL encoded fields, got %d", len(resolved.BodyUrlencoded))
	}
}

func TestConvertPostmanCollection_FoldersAndNesting(t *testing.T) {
	// Collection with nested folders
	collectionJSON := `{
		"info": {
			"name": "Nested Collection"
		},
		"item": [
			{
				"name": "Users",
				"item": [
					{
						"name": "Get All Users",
						"request": {
							"method": "GET",
							"url": {
								"raw": "https://api.example.com/users"
							}
						}
					},
					{
						"name": "User Management",
						"item": [
							{
								"name": "Create User",
								"request": {
									"method": "POST",
									"url": {
										"raw": "https://api.example.com/users"
									}
								}
							}
						]
					}
				]
			},
			{
				"name": "Posts",
				"item": [
					{
						"name": "Get Posts",
						"request": {
							"method": "GET",
							"url": {
								"raw": "https://api.example.com/posts"
							}
						}
					}
				]
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Nested Collection",
	}

	resolved, err := ConvertPostmanCollection([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertPostmanCollection() error = %v", err)
	}

	// Should extract all HTTP requests regardless of nesting level
	if len(resolved.HTTPRequests) != 3 {
		t.Fatalf("Expected 3 HTTP requests, got %d", len(resolved.HTTPRequests))
	}

	expectedNames := []string{"Get All Users", "Create User", "Get Posts"}
	for i, expectedName := range expectedNames {
		if resolved.HTTPRequests[i].Name != expectedName {
			t.Errorf("Expected request name '%s' at index %d, got '%s'", expectedName, i, resolved.HTTPRequests[i].Name)
		}
	}
}

func TestConvertPostmanCollection_DisabledItems(t *testing.T) {
	// Collection with disabled headers and parameters
	collectionJSON := `{
		"info": {
			"name": "Disabled Items Test"
		},
		"item": [
			{
				"name": "Request with Disabled Items",
				"request": {
					"method": "GET",
					"header": [
						{
							"key": "Enabled Header",
							"value": "enabled-value"
						},
						{
							"key": "Disabled Header",
							"value": "disabled-value",
							"disabled": true
						}
					],
					"url": {
						"raw": "https://api.example.com/data?enabled=true&disabled=false",
						"query": [
							{
								"key": "enabled",
								"value": "true"
							},
							{
								"key": "disabled",
								"value": "false",
								"disabled": true
							}
						]
					}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Disabled Items Test",
	}

	resolved, err := ConvertPostmanCollection([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertPostmanCollection() error = %v", err)
	}

	// Should only have enabled headers
	if len(resolved.Headers) != 1 {
		t.Fatalf("Expected 1 enabled header, got %d", len(resolved.Headers))
	}
	if resolved.Headers[0].Key != "Enabled Header" {
		t.Errorf("Expected enabled header key 'Enabled Header', got '%s'", resolved.Headers[0].Key)
	}
	if !resolved.Headers[0].Enabled {
		t.Error("Expected header to be enabled")
	}

	// Should have only enabled search parameters (disabled ones filtered out)
	if len(resolved.SearchParams) != 1 {
		t.Errorf("Expected 1 search parameter (disabled filtered out), got %d", len(resolved.SearchParams))
	}
}

func TestConvertPostmanCollection_EmptyCollection(t *testing.T) {
	tests := []struct {
		name        string
		collection  string
		expectError bool
	}{
		{
			name:        "empty data",
			collection:  "",
			expectError: true,
		},
		{
			name:        "empty collection",
			collection:  `{"info": {"name": "Empty"}, "item": []}`,
			expectError: false,
		},
		{
			name:        "invalid JSON",
			collection:  `{"info": {"name": "Invalid"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ConvertOptions{
				WorkspaceID:    idwrap.NewNow(),
				CollectionName: "Test",
			}

			resolved, err := ConvertPostmanCollection([]byte(tt.collection), opts)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !tt.expectError && len(resolved.HTTPRequests) != 0 {
				t.Errorf("Expected 0 HTTP requests for empty collection, got %d", len(resolved.HTTPRequests))
			}
		})
	}
}

func TestConvertToFiles(t *testing.T) {
	collectionJSON := `{
		"info": {"name": "Files Test"},
		"item": [
			{
				"name": "Test Request",
				"request": {
					"method": "GET",
					"url": {"raw": "https://example.com"}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Files Test",
	}

	files, err := ConvertToFiles([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertToFiles() error = %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	file := files[0]
	if file.Name != "Test Request" {
		t.Errorf("Expected file name 'Test Request', got '%s'", file.Name)
	}
	if file.ContentType != mfile.ContentTypeHTTP {
		t.Errorf("Expected content type %d, got %d", mfile.ContentTypeHTTP, file.ContentType)
	}
}

func TestConvertToHTTPRequests(t *testing.T) {
	collectionJSON := `{
		"info": {"name": "HTTP Test"},
		"item": [
			{
				"name": "Test Request",
				"request": {
					"method": "POST",
					"url": {"raw": "https://example.com/api"}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "HTTP Test",
	}

	httpReqs, err := ConvertToHTTPRequests([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertToHTTPRequests() error = %v", err)
	}

	if len(httpReqs) != 1 {
		t.Fatalf("Expected 1 HTTP request, got %d", len(httpReqs))
	}

	httpReq := httpReqs[0]
	if httpReq.Method != "POST" {
		t.Errorf("Expected method 'POST', got '%s'", httpReq.Method)
	}
	if httpReq.Url != "https://example.com/api" {
		t.Errorf("Expected URL 'https://example.com/api', got '%s'", httpReq.Url)
	}
}

func TestBuildPostmanCollection(t *testing.T) {
	// Create test HTTP data
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()
	now := time.Now().UnixMilli()

	httpReqs := []mhttp.HTTP{
		{
			ID:          httpID1,
			WorkspaceID: idwrap.NewNow(),
			Name:        "Get Users",
			Method:      "GET",
			Url:         "https://api.example.com/users",
			Description: "Retrieve all users",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          httpID2,
			WorkspaceID: idwrap.NewNow(),
			Name:        "Create User",
			Method:      "POST",
			Url:         "https://api.example.com/users",
			Description: "Create a new user",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	headers := []mhttp.HTTPHeader{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID1,
			Key:   "Accept",
			Value: "application/json",
			Enabled:     true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	searchParams := []mhttp.HTTPSearchParam{
		{
			ID:         idwrap.NewNow(),
			HttpID:     httpID1,
			Key:   "page",
			Value: "1",
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}

	rawData := []byte(`{"name": "John Doe"}`)
	bodyRaw := &mhttp.HTTPBodyRaw{
		ID:          idwrap.NewNow(),
		HttpID:      httpID2,
		RawData:     rawData,
		ContentType: "application/json",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	resolved := &PostmanResolved{
		HTTPRequests: httpReqs,
		Headers:      headers,
		SearchParams: searchParams,
		BodyRaw:      []*mhttp.HTTPBodyRaw{bodyRaw},
	}

	// Build Postman collection
	collectionJSON, err := BuildPostmanCollection(resolved)
	if err != nil {
		t.Fatalf("BuildPostmanCollection() error = %v", err)
	}

	// Parse the result to verify structure
	var collection PostmanCollection
	if err := json.Unmarshal(collectionJSON, &collection); err != nil {
		t.Fatalf("Failed to parse generated collection: %v", err)
	}

	if collection.Info.Name != "Generated Collection" {
		t.Errorf("Expected collection name 'Generated Collection', got '%s'", collection.Info.Name)
	}

	if len(collection.Item) != 2 {
		t.Fatalf("Expected 2 items in collection, got %d", len(collection.Item))
	}

	// Verify first request
	firstItem := collection.Item[0]
	if firstItem.Name != "Get Users" {
		t.Errorf("Expected first item name 'Get Users', got '%s'", firstItem.Name)
	}
	if firstItem.Request.Method != "GET" {
		t.Errorf("Expected first item method 'GET', got '%s'", firstItem.Request.Method)
	}

	// Verify second request has raw body
	secondItem := collection.Item[1]
	if secondItem.Request.Body == nil {
		t.Error("Expected second item to have body")
	} else if secondItem.Request.Body.Raw != string(rawData) {
		t.Errorf("Expected body raw '%s', got '%s'", string(rawData), secondItem.Request.Body.Raw)
	}
}

func TestConvertPostmanCollection_DeltaSystem(t *testing.T) {
	collectionJSON := `{
		"info": {"name": "Delta Test"},
		"item": [
			{
				"name": "Base Request",
				"request": {
					"method": "GET",
					"url": {"raw": "https://api.example.com/data"}
				}
			}
		]
	}`

	parentID := idwrap.NewNow()
	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		ParentHttpID:   &parentID,
		IsDelta:        true,
		DeltaName:      stringPtr("Variation A"),
		CollectionName: "Delta Test",
	}

	resolved, err := ConvertPostmanCollection([]byte(collectionJSON), opts)
	if err != nil {
		t.Fatalf("ConvertPostmanCollection() error = %v", err)
	}

	if len(resolved.HTTPRequests) != 1 {
		t.Fatalf("Expected 1 HTTP request, got %d", len(resolved.HTTPRequests))
	}

	httpReq := resolved.HTTPRequests[0]
	if !httpReq.IsDelta {
		t.Error("Expected HTTP request to be marked as delta")
	}
	if httpReq.ParentHttpID.Compare(parentID) != 0 {
		t.Error("Expected ParentHttpID to match parent ID")
	}
	if httpReq.DeltaName == nil {
		t.Error("Expected DeltaName to be set")
	} else if *httpReq.DeltaName != "Variation A" {
		t.Errorf("Expected DeltaName 'Variation A', got '%s'", *httpReq.DeltaName)
	}
}

func TestConvertPostmanCollection_Authentication(t *testing.T) {
	tests := []struct {
		name                string
		collection          string
		expectedHeaders     int
		expectedAuthHeaders map[string]string
	}{
		{
			name: "API Key authentication",
			collection: `{
				"info": {"name": "Auth Test"},
				"item": [
					{
						"name": "API Key Request",
						"request": {
							"method": "GET",
							"auth": {
								"type": "apikey",
								"apikey": [
									{"key": "key", "value": "X-API-Key"},
									{"key": "value", "value": "secret123"}
								]
							},
							"url": {"raw": "https://api.example.com/data"}
						}
					}
				]
			}`,
			expectedHeaders: 1,
			expectedAuthHeaders: map[string]string{
				"X-API-Key": "secret123",
			},
		},
		{
			name: "Bearer token authentication",
			collection: `{
				"info": {"name": "Auth Test"},
				"item": [
					{
						"name": "Bearer Request",
						"request": {
							"method": "GET",
							"auth": {
								"type": "bearer",
								"bearer": [
									{"key": "token", "value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"}
								]
							},
							"url": {"raw": "https://api.example.com/data"}
						}
					}
				]
			}`,
			expectedHeaders: 1,
			expectedAuthHeaders: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			},
		},
		{
			name: "Basic authentication",
			collection: `{
				"info": {"name": "Auth Test"},
				"item": [
					{
						"name": "Basic Request",
						"request": {
							"method": "GET",
							"auth": {
								"type": "basic",
								"basic": [
									{"key": "username", "value": "testuser"},
									{"key": "password", "value": "testpass"}
								]
							},
							"url": {"raw": "https://api.example.com/data"}
						}
					}
				]
			}`,
			expectedHeaders: 1,
			expectedAuthHeaders: map[string]string{
				"Authorization": "Basic dGVzdHVzZXI6dGVzdHBhc3M=",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ConvertOptions{
				WorkspaceID:    idwrap.NewNow(),
				CollectionName: "Auth Test",
			}

			resolved, err := ConvertPostmanCollection([]byte(tt.collection), opts)
			if err != nil {
				t.Fatalf("ConvertPostmanCollection() error = %v", err)
			}

			if len(resolved.Headers) != tt.expectedHeaders {
				t.Errorf("Expected %d headers, got %d", tt.expectedHeaders, len(resolved.Headers))
			}

			// Verify auth headers
			for expectedKey, expectedValue := range tt.expectedAuthHeaders {
				found := false
				for _, header := range resolved.Headers {
					if header.Key == expectedKey && header.Value == expectedValue {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected header %s: %s not found", expectedKey, expectedValue)
				}
			}
		})
	}
}

func TestParsePostmanCollection(t *testing.T) {
	collectionJSON := `{
		"info": {
			"name": "Parse Test",
			"description": "Test parsing functionality"
		},
		"variable": [
			{
				"key": "baseUrl",
				"value": "https://api.example.com"
			}
		],
		"item": []
	}`

	collection, err := ParsePostmanCollection([]byte(collectionJSON))
	if err != nil {
		t.Fatalf("ParsePostmanCollection() error = %v", err)
	}

	if collection.Info.Name != "Parse Test" {
		t.Errorf("Expected collection name 'Parse Test', got '%s'", collection.Info.Name)
	}
	if collection.Info.Description != "Test parsing functionality" {
		t.Errorf("Expected description 'Test parsing functionality', got '%s'", collection.Info.Description)
	}
	if len(collection.Variable) != 1 {
		t.Fatalf("Expected 1 variable, got %d", len(collection.Variable))
	}
	if collection.Variable[0].Key != "baseUrl" {
		t.Errorf("Expected variable key 'baseUrl', got '%s'", collection.Variable[0].Key)
	}
}

// Helper function to create string pointers for tests
func stringPtr(s string) *string {
	return &s
}

// Benchmark tests
func BenchmarkConvertPostmanCollection_Simple(b *testing.B) {
	collectionJSON := `{
		"info": {"name": "Benchmark Collection"},
		"item": [
			{
				"name": "Simple Request",
				"request": {
					"method": "GET",
					"header": [{"key": "Accept", "value": "application/json"}],
					"url": {"raw": "https://api.example.com/test"}
				}
			}
		]
	}`

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Benchmark Collection",
	}

	data := []byte(collectionJSON)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ConvertPostmanCollection(data, opts)
		if err != nil {
			b.Fatalf("ConvertPostmanCollection() error = %v", err)
		}
	}
}

func BenchmarkConvertPostmanCollection_Large(b *testing.B) {
	// Generate a larger collection for benchmarking
	items := make([]string, 100)
	for i := 0; i < 100; i++ {
		items[i] = fmt.Sprintf(`{
			"name": "Request %d",
			"request": {
				"method": "GET",
				"header": [
					{"key": "Accept", "value": "application/json"},
					{"key": "X-Request-ID", "value": "req-%d"}
				],
				"url": {
					"raw": "https://api.example.com/items/%d",
					"query": [
						{"key": "page", "value": "%d"},
						{"key": "limit", "value": "10"}
					]
				}
			}
		}`, i+1, i+1, i+1, i+1)
	}

	collectionJSON := fmt.Sprintf(`{
		"info": {"name": "Large Benchmark Collection"},
		"item": [%s]
	}`, strings.Join(items, ","))

	opts := ConvertOptions{
		WorkspaceID:    idwrap.NewNow(),
		CollectionName: "Large Benchmark Collection",
	}

	data := []byte(collectionJSON)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ConvertPostmanCollection(data, opts)
		if err != nil {
			b.Fatalf("ConvertPostmanCollection() error = %v", err)
		}
	}
}
