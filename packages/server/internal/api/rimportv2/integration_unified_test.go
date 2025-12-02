package rimportv2

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
)

// TestIntegrationHARImport tests the complete HAR import workflow
func TestIntegrationHARImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require real service dependencies
	// For now, we'll test the format detection and basic workflow
	detector := NewFormatDetector()
	registry := NewTranslatorRegistry(nil)

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	// Sample HAR data
	harData := []byte(`{
		"log": {
			"version": "1.2",
			"creator": {"name": "test", "version": "1.0"},
			"entries": [
				{
					"startedDateTime": "2023-01-01T00:00:00.000Z",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Accept", "value": "application/json"},
							{"name": "User-Agent", "value": "test-agent"}
						]
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Content-Type", "value": "application/json"}
						],
						"content": {
							"size": 123,
							"mimeType": "application/json",
							"text": "{\"users\": []}"
						}
					},
					"time": 150,
					"_resourceType": "xhr"
				},
				{
					"startedDateTime": "2023-01-01T00:00:01.000Z",
					"request": {
						"method": "POST",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Content-Type", "value": "application/json"},
							{"name": "Accept", "value": "application/json"}
						],
						"postData": {
							"mimeType": "application/json",
							"text": "{\"name\": \"Test User\"}"
						}
					},
					"response": {
						"status": 201,
						"statusText": "Created",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Content-Type", "value": "application/json"}
						],
						"content": {
							"size": 45,
							"mimeType": "application/json",
							"text": "{\"id\": 1, \"name\": \"Test User\"}"
						}
					},
					"time": 200,
					"_resourceType": "xhr"
				}
			]
		}
	}`)

	// Test format detection
	detection := detector.DetectFormat(harData)
	if detection.Format != FormatHAR {
		t.Errorf("Expected HAR format, got %v", detection.Format)
	}
	if detection.Confidence < 0.8 {
		t.Errorf("Expected high confidence for HAR, got %.2f", detection.Confidence)
	}

	// Test validation
	if err := detector.ValidateFormat(harData, FormatHAR); err != nil {
		t.Errorf("HAR validation failed: %v", err)
	}

	// Test translation (this would require real services to complete)
	result, err := registry.DetectAndTranslate(ctx, harData, workspaceID)
	if err != nil {
		t.Errorf("Translation failed: %v", err)
	}
	if result == nil {
		t.Error("Expected translation result, got nil")
	}

	// Verify basic result structure
	if len(result.HTTPRequests) == 0 {
		t.Error("Expected HTTP requests in translation result")
	}
	if len(result.Files) == 0 {
		t.Error("Expected files in translation result")
	}
	if len(result.Flows) == 0 {
		t.Error("Expected flows in translation result")
	}
	if len(result.Headers) == 0 {
		t.Error("Expected headers in translation result")
	}
	if len(result.BodyRaw) == 0 {
		t.Error("Expected body raw in translation result")
	}

	t.Logf("HAR import test completed successfully:")
	t.Logf("  - Format: %v (confidence: %.2f)", detection.Format, detection.Confidence)
	t.Logf("  - HTTP Requests: %d", len(result.HTTPRequests))
	t.Logf("  - Files: %d", len(result.Files))
	t.Logf("  - Flows: %d", len(result.Flows))
	t.Logf("  - Domains: %v", result.Domains)
	t.Logf("  - Headers: %d", len(result.Headers))
	t.Logf("  - Body Raws: %d", len(result.BodyRaw))
}

// TestIntegrationYAMLImport tests the complete YAML flow import workflow
func TestIntegrationYAMLImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	detector := NewFormatDetector()
	registry := NewTranslatorRegistry(nil)

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	// Sample YAML flow data
	yamlData := []byte(`workspace_name: Test Workspace
flows:
  - name: User API Flow
    variables:
      - name: api_host
        value: "https://api.example.com"
      - name: user_id
        value: "123"
    steps:
      - request:
          name: Get User
          method: GET
          url: "{{api_host}}/users/{{user_id}}"
          headers:
            Accept: application/json
      - request:
          name: Update User
          method: PUT
          url: "{{api_host}}/users/{{user_id}}"
          headers:
            Content-Type: application/json
            Accept: application/json
          body:
            type: json
            json:
              name: "Updated Name"
              email: "updated@example.com"
      - if:
          name: Check Response
          condition: "response.status == 200"
          then: Success Step
          else: Error Step
      - request:
          name: Success Step
          method: GET
          url: "{{api_host}}/users/{{user_id}}"
          depends_on:
            - Check Response
      - request:
          name: Error Step
          method: GET
          url: "{{api_host}}/error"
          depends_on:
            - Check Response

requests:
  - name: get_user
    method: GET
    url: "{{api_host}}/users/{{user_id}}"
    headers:
      Accept: application/json
  - name: update_user
    method: PUT
    url: "{{api_host}}/users/{{user_id}}"
    headers:
      Content-Type: application/json
    body:
      type: json
      json:
        name: "User Name"`)

	// Test format detection
	detection := detector.DetectFormat(yamlData)
	if detection.Format != FormatYAML {
		t.Errorf("Expected YAML format, got %v", detection.Format)
	}
	if detection.Confidence < 0.5 {
		t.Errorf("Expected reasonable confidence for YAML, got %.2f", detection.Confidence)
	}

	// Test validation
	if err := detector.ValidateFormat(yamlData, FormatYAML); err != nil {
		t.Errorf("YAML validation failed: %v", err)
	}

	// Test translation
	result, err := registry.DetectAndTranslate(ctx, yamlData, workspaceID)
	if err != nil {
		t.Errorf("Translation failed: %v", err)
		return // Return early to avoid nil pointer access
	}
	if result == nil {
		t.Error("Expected translation result, got nil")
		return // Return early to avoid nil pointer access
	}

	// Verify basic result structure
	if len(result.HTTPRequests) == 0 {
		t.Error("Expected HTTP requests in translation result")
	}
	if len(result.Flows) == 0 {
		t.Error("Expected flows in translation result")
	}

	t.Logf("YAML import test completed successfully:")
	t.Logf("  - Format: %v (confidence: %.2f)", detection.Format, detection.Confidence)
	t.Logf("  - HTTP Requests: %d", len(result.HTTPRequests))
	t.Logf("  - Files: %d", len(result.Files))
	t.Logf("  - Flows: %d", len(result.Flows))
	t.Logf("  - Headers: %d", len(result.Headers))
	t.Logf("  - Search Params: %d", len(result.SearchParams))
}

// TestIntegrationCURLImport tests the complete CURL import workflow
func TestIntegrationCURLImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	detector := NewFormatDetector()
	registry := NewTranslatorRegistry(nil)

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	// Sample CURL command
	curlData := []byte(`curl -X POST "https://api.example.com/users" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer token123" \
  -d '{
    "name": "John Doe",
    "email": "john@example.com",
    "active": true
  }' \
  --verbose`)

	// Test format detection
	detection := detector.DetectFormat(curlData)
	if detection.Format != FormatCURL {
		t.Errorf("Expected CURL format, got %v", detection.Format)
	}
	if detection.Confidence < 0.7 {
		t.Errorf("Expected high confidence for CURL, got %.2f", detection.Confidence)
	}

	// Test validation
	if err := detector.ValidateFormat(curlData, FormatCURL); err != nil {
		t.Errorf("CURL validation failed: %v", err)
	}

	// Test translation
	result, err := registry.DetectAndTranslate(ctx, curlData, workspaceID)
	if err != nil {
		t.Errorf("Translation failed: %v", err)
	}
	if result == nil {
		t.Error("Expected translation result, got nil")
	}

	// Verify basic result structure
	if len(result.HTTPRequests) == 0 {
		t.Error("Expected HTTP requests in translation result")
	}
	if len(result.HTTPRequests) > 1 {
		t.Errorf("Expected 1 HTTP request from CURL, got %d", len(result.HTTPRequests))
	}

	httpReq := result.HTTPRequests[0]
	if httpReq.Method != "POST" {
		t.Errorf("Expected POST method, got %s", httpReq.Method)
	}
	if httpReq.Url != "https://api.example.com/users" {
		t.Errorf("Expected URL, got %s", httpReq.Url)
	}

	t.Logf("CURL import test completed successfully:")
	t.Logf("  - Format: %v (confidence: %.2f)", detection.Format, detection.Confidence)
	t.Logf("  - HTTP Requests: %d", len(result.HTTPRequests))
	t.Logf("  - Files: %d", len(result.Files))
	t.Logf("  - Headers: %d", len(result.Headers))
	t.Logf("  - Body Raw: %v", result.BodyRaw != nil)
}

// TestIntegrationPostmanImport tests the complete Postman import workflow
func TestIntegrationPostmanImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	detector := NewFormatDetector()
	registry := NewTranslatorRegistry(nil)

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	// Sample Postman collection
	postmanData := []byte(`{
		"info": {
			"_postman_id": "test-id",
			"name": "User API Collection",
			"description": "A collection for testing user API endpoints",
			"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
		},
		"item": [
			{
				"name": "Get Users",
				"request": {
					"method": "GET",
					"header": [
						{
							"key": "Accept",
							"value": "application/json",
							"type": "text"
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
								"value": "1",
								"description": "Page number"
							},
							{
								"key": "limit",
								"value": "10",
								"description": "Number of items per page"
							}
						]
					},
					"description": "Retrieve a list of users"
				},
				"response": []
			},
			{
				"name": "Create User",
				"request": {
					"method": "POST",
					"header": [
						{
							"key": "Content-Type",
							"value": "application/json",
							"type": "text"
						},
						{
							"key": "Accept",
							"value": "application/json",
							"type": "text"
						}
					],
					"body": {
						"mode": "raw",
						"raw": "{\n  \"name\": \"John Doe\",\n  \"email\": \"john@example.com\"\n}",
						"options": {
							"raw": {
								"language": "json"
							}
						}
					},
					"url": {
						"raw": "https://api.example.com/users",
						"protocol": "https",
						"host": ["api", "example", "com"],
						"path": ["users"]
					},
					"description": "Create a new user"
				},
				"response": []
			}
		]
	}`)

	// Test format detection
	detection := detector.DetectFormat(postmanData)
	if detection.Format != FormatPostman {
		t.Errorf("Expected Postman format, got %v", detection.Format)
	}
	if detection.Confidence < 0.8 {
		t.Errorf("Expected high confidence for Postman, got %.2f", detection.Confidence)
	}

	// Test validation
	if err := detector.ValidateFormat(postmanData, FormatPostman); err != nil {
		t.Errorf("Postman validation failed: %v", err)
	}

	// Test translation
	result, err := registry.DetectAndTranslate(ctx, postmanData, workspaceID)
	if err != nil {
		t.Errorf("Translation failed: %v", err)
	}
	if result == nil {
		t.Error("Expected translation result, got nil")
	}

	// Verify basic result structure
	if len(result.HTTPRequests) == 0 {
		t.Error("Expected HTTP requests in translation result")
	}
	if len(result.HTTPRequests) < 2 {
		t.Errorf("Expected at least 2 HTTP requests from Postman, got %d", len(result.HTTPRequests))
	}

	t.Logf("Postman import test completed successfully:")
	t.Logf("  - Format: %v (confidence: %.2f)", detection.Format, detection.Confidence)
	t.Logf("  - HTTP Requests: %d", len(result.HTTPRequests))
	t.Logf("  - Files: %d", len(result.Files))
	t.Logf("  - Headers: %d", len(result.Headers))
	t.Logf("  - Search Params: %d", len(result.SearchParams))
}

// TestErrorHandlingIntegration tests error handling across the unified import system
func TestErrorHandlingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	detector := NewFormatDetector()
	registry := NewTranslatorRegistry(nil)

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name           string
		data           []byte
		expectedError  string
		expectedFormat Format
	}{
		{
			name:           "Empty data",
			data:           []byte(""),
			expectedError:  "empty",
			expectedFormat: FormatUnknown,
		},
		{
			name:           "Invalid JSON",
			data:           []byte(`{invalid json that cannot be parsed`),
			expectedError:  "invalid",
			expectedFormat: FormatUnknown,
		},
		{
			name: "Invalid HAR structure",
			data: []byte(`{
				"log": {
					"version": "1.2",
					"entries": []
				}
			}`),
			expectedError:  "entries",
			expectedFormat: FormatHAR, // Will detect as HAR but fail validation due to no entries
		},
		{
			name:           "Invalid YAML",
			data:           []byte(`invalid: yaml: content: [missing colon`),
			expectedError:  "invalid",
			expectedFormat: FormatUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test format detection
			detection := detector.DetectFormat(tt.data)
			if detection.Format != tt.expectedFormat {
				t.Errorf("Expected format %v, got %v", tt.expectedFormat, detection.Format)
			}

			// Test validation if format is detected
			if detection.Format != FormatUnknown {
				err := detector.ValidateFormat(tt.data, detection.Format)
				if err == nil && tt.expectedError != "" {
					t.Errorf("Expected validation error containing '%s', got none", tt.expectedError)
				} else if err != nil && tt.expectedError == "" {
					t.Errorf("Unexpected validation error: %v", err)
				} else if err != nil {
					if !contains(err.Error(), tt.expectedError) {
						t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
					}
				}
			}

			// Test translation (should fail for invalid data)
			_, err := registry.DetectAndTranslate(ctx, tt.data, workspaceID)
			if err == nil && tt.expectedError != "" {
				t.Errorf("Expected translation error containing '%s', got none", tt.expectedError)
			} else if err != nil && tt.expectedError == "" {
				t.Logf("Unexpected translation error (might be acceptable): %v", err)
			} else if err != nil {
				if !contains(err.Error(), tt.expectedError) {
					t.Logf("Translation error: %s (expected to contain '%s')", err.Error(), tt.expectedError)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestPerformanceIntegration tests performance characteristics of the unified system
func TestPerformanceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	detector := NewFormatDetector()

	// Test data of various sizes
	smallHAR := []byte(`{"log": {"entries": [{"request": {"method": "GET", "url": "https://api.example.com"}}]}}`)
	mediumHAR := []byte(`{"log": {"entries": [`)
	for i := 0; i < 100; i++ {
		mediumHAR = append(mediumHAR, []byte(`{"request": {"method": "GET", "url": "https://api.example.com/item`+string(rune(i%10))+`"}},`)...)
	}
	mediumHAR = append(mediumHAR[:len(mediumHAR)-1], []byte(`]}`)...)

	tests := []struct {
		name      string
		data      []byte
		maxTime   time.Duration
		testCount int
	}{
		{
			name:      "Small HAR Detection",
			data:      smallHAR,
			maxTime:   10 * time.Millisecond,
			testCount: 1000,
		},
		{
			name:      "Medium HAR Detection",
			data:      mediumHAR,
			maxTime:   50 * time.Millisecond,
			testCount: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			for i := 0; i < tt.testCount; i++ {
				_ = detector.DetectFormat(tt.data)
			}

			duration := time.Since(start)
			avgDuration := duration / time.Duration(tt.testCount)

			t.Logf("Performance results for %s:", tt.name)
			t.Logf("  - Total time: %v", duration)
			t.Logf("  - Average time per operation: %v", avgDuration)
			t.Logf("  - Operations per second: %.0f", float64(time.Second)/float64(avgDuration))

			if avgDuration > tt.maxTime {
				t.Errorf("Average operation time %v exceeds maximum %v", avgDuration, tt.maxTime)
			}
		})
	}
}
