package tpostmanv2

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "ConvertPostmanCollection() error")

	require.Len(t, resolved.HTTPRequests, 2, "Expected 2 HTTP requests (Base + Delta)")

	httpReq := resolved.HTTPRequests[0]
	require.Equal(t, "Get Users", httpReq.Name, "Expected name 'Get Users'")
	require.Equal(t, "GET", httpReq.Method, "Expected method 'GET'")
	require.Equal(t, "https://api.example.com/users", httpReq.Url, "Expected URL")

	// Check search parameters (2 Base, Delta will have 0 because they are identical)
	require.Len(t, resolved.SearchParams, 2, "Expected 2 search parameters")

	// Check headers (1 Base, Delta will have 0)
	require.Len(t, resolved.Headers, 1, "Expected 1 header")
	header := resolved.Headers[0]
	require.Equal(t, "Accept", header.Key, "Expected header key 'Accept'")
	require.Equal(t, "application/json", header.Value, "Expected header value 'application/json'")

	// Check files - now includes URL-based folders and flow file
	// For https://api.example.com/users: 4 folders (com, example, api, users) + 1 base + 1 delta + 1 flow = 7
	require.GreaterOrEqual(t, len(resolved.Files), 3, "Expected at least 3 files (Base + Delta + Flow)")

	// Find the HTTP file
	var httpFile *mfile.File
	for i := range resolved.Files {
		if resolved.Files[i].ContentType == mfile.ContentTypeHTTP {
			httpFile = &resolved.Files[i]
			break
		}
	}
	require.NotNil(t, httpFile, "Expected to find HTTP file")
	require.Equal(t, "Get Users", httpFile.Name, "Expected file name 'Get Users'")
	require.Equal(t, mfile.ContentTypeHTTP, httpFile.ContentType, "Expected content type")
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
	require.NoError(t, err, "ConvertPostmanCollection() error")

	require.Len(t, resolved.HTTPRequests, 6, "Expected 6 HTTP requests (3 items * 2)")

	// Test raw body
	rawBodyReq := resolved.HTTPRequests[0]
	require.Equal(t, "POST", rawBodyReq.Method, "Expected POST method for raw body request")
	require.Len(t, resolved.BodyRaw, 2, "Expected 2 raw bodies (Base + Delta)")
	rawBody := resolved.BodyRaw[0]
	expectedRawData := []byte(`{"name": "John", "age": 30}`)
	require.Equal(t, string(expectedRawData), string(rawBody.RawData), "Expected raw body data")

	// Test form data
	require.Len(t, resolved.BodyForms, 2, "Expected 2 form fields")

	// Test URL encoded
	require.Len(t, resolved.BodyUrlencoded, 2, "Expected 2 URL encoded fields")
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
	require.NoError(t, err, "ConvertPostmanCollection() error")

	// Should extract all HTTP requests regardless of nesting level (Base + Delta for each)
	require.Len(t, resolved.HTTPRequests, 6, "Expected 6 HTTP requests (3 items * 2)")

	expectedNames := []string{"Get All Users", "Get All Users (Delta)", "Create User", "Create User (Delta)", "Get Posts", "Get Posts (Delta)"}
	for i, expectedName := range expectedNames {
		require.Equal(t, expectedName, resolved.HTTPRequests[i].Name, "Expected request name at index %d", i)
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
	require.NoError(t, err, "ConvertPostmanCollection() error")

	// Should only have enabled headers
	require.Len(t, resolved.Headers, 1, "Expected 1 enabled header")
	require.Equal(t, "Enabled Header", resolved.Headers[0].Key, "Expected enabled header key")
	require.True(t, resolved.Headers[0].Enabled, "Expected header to be enabled")

	// Should have only enabled search parameters (disabled ones filtered out)
	require.Len(t, resolved.SearchParams, 1, "Expected 1 search parameter (disabled filtered out)")
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
				require.Error(t, err, "Expected error but got none")
				return
			}

			require.NoError(t, err, "Unexpected error")

			if !tt.expectError {
				require.Empty(t, resolved.HTTPRequests, "Expected 0 HTTP requests for empty collection")
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
	require.NoError(t, err, "ConvertToFiles() error")

	// Now includes URL-based folders and flow file
	require.GreaterOrEqual(t, len(files), 3, "Expected at least 3 files (Base + Delta + Flow)")

	// Find the HTTP file
	var httpFile *mfile.File
	for i := range files {
		if files[i].ContentType == mfile.ContentTypeHTTP {
			httpFile = &files[i]
			break
		}
	}
	require.NotNil(t, httpFile, "Expected to find HTTP file")
	require.Equal(t, "Test Request", httpFile.Name, "Expected file name 'Test Request'")
	require.Equal(t, mfile.ContentTypeHTTP, httpFile.ContentType, "Expected content type")
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
	require.NoError(t, err, "ConvertToHTTPRequests() error")

	require.Len(t, httpReqs, 2, "Expected 2 HTTP requests (Base + Delta)")

	httpReq := httpReqs[0]
	require.Equal(t, "POST", httpReq.Method, "Expected method 'POST'")
	require.Equal(t, "https://example.com/api", httpReq.Url, "Expected URL")
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
			ID:        idwrap.NewNow(),
			HttpID:    httpID1,
			Key:       "Accept",
			Value:     "application/json",
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	searchParams := []mhttp.HTTPSearchParam{
		{
			ID:        idwrap.NewNow(),
			HttpID:    httpID1,
			Key:       "page",
			Value:     "1",
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	rawData := []byte(`{"name": "John Doe"}`)
	bodyRaw := &mhttp.HTTPBodyRaw{
		ID:        idwrap.NewNow(),
		HttpID:    httpID2,
		RawData:   rawData,
		CreatedAt: now,
		UpdatedAt: now,
	}

	resolved := &PostmanResolved{
		HTTPRequests: httpReqs,
		Headers:      headers,
		SearchParams: searchParams,
		BodyRaw:      []mhttp.HTTPBodyRaw{*bodyRaw},
	}

	// Build Postman collection
	collectionJSON, err := BuildPostmanCollection(resolved)
	require.NoError(t, err, "BuildPostmanCollection() error")

	// Parse the result to verify structure
	var collection PostmanCollection
	err = json.Unmarshal(collectionJSON, &collection)
	require.NoError(t, err, "Failed to parse generated collection")

	require.Equal(t, "Generated Collection", collection.Info.Name, "Expected collection name 'Generated Collection'")

	require.Len(t, collection.Item, 2, "Expected 2 items in collection")

	// Verify first request
	firstItem := collection.Item[0]
	require.Equal(t, "Get Users", firstItem.Name, "Expected first item name 'Get Users'")
	require.Equal(t, "GET", firstItem.Request.Method, "Expected first item method 'GET'")

	// Verify second request has raw body
	secondItem := collection.Item[1]
	require.NotNil(t, secondItem.Request.Body, "Expected second item to have body")
	require.Equal(t, string(rawData), secondItem.Request.Body.Raw, "Expected body raw")
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
	require.NoError(t, err, "ConvertPostmanCollection() error")

	require.Len(t, resolved.HTTPRequests, 2, "Expected 2 HTTP requests (Base + Delta)")

	httpReq := resolved.HTTPRequests[0]
	require.False(t, httpReq.IsDelta, "Expected first HTTP request to be base")
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
			require.NoError(t, err, "ConvertPostmanCollection() error")

			require.Len(t, resolved.Headers, tt.expectedHeaders, "Expected %d headers", tt.expectedHeaders)

			// Verify auth headers
			for expectedKey, expectedValue := range tt.expectedAuthHeaders {
				found := false
				for _, header := range resolved.Headers {
					if header.Key == expectedKey && header.Value == expectedValue {
						found = true
						break
					}
				}
				require.True(t, found, "Expected header %s: %s not found", expectedKey, expectedValue)
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
	require.NoError(t, err, "ParsePostmanCollection() error")

	require.Equal(t, "Parse Test", collection.Info.Name, "Expected collection name 'Parse Test'")
	require.Equal(t, "Test parsing functionality", collection.Info.Description, "Expected description")
	require.Len(t, collection.Variable, 1, "Expected 1 variable")
	require.Equal(t, "baseUrl", collection.Variable[0].Key, "Expected variable key 'baseUrl'")
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
