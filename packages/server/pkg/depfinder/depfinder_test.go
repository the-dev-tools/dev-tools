package depfinder_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
)

func TestNewDepFinder(t *testing.T) {
	df := depfinder.NewDepFinder()

	// Test that FindVar returns ErrNotFound on a new instance
	_, err := df.FindVar("non-existent")
	if err != depfinder.ErrNotFound {
		t.Errorf("Expected ErrNotFound on a new DepFinder, got %v", err)
	}
}

func TestAddAndFindVar(t *testing.T) {
	df := depfinder.NewDepFinder()

	// Add and find a string
	df.AddVar("value", depfinder.VarCouple{Path: "test.path"})
	couple, err := df.FindVar("value")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if couple.Path != "test.path" {
		t.Errorf("Expected path 'test.path', got '%s'", couple.Path)
	}

	// Add and find a number
	df.AddVar(42, depfinder.VarCouple{Path: "answer"})
	couple, err = df.FindVar(42)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if couple.Path != "answer" {
		t.Errorf("Expected path 'answer', got '%s'", couple.Path)
	}

	// Try to find a value that doesn't exist
	_, err = df.FindVar("non-existent")
	if err != depfinder.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestAddJsonBytes(t *testing.T) {
	jsonData := []byte(`{
		"name": "test",
		"properties": {
			"id": 123,
			"active": true
		},
		"tags": ["tag1", "tag2"]
	}`)

	df := depfinder.NewDepFinder()
	err := df.AddJsonBytes(jsonData, depfinder.VarCouple{})
	if err != nil {
		t.Errorf("Failed to add JSON bytes: %v", err)
	}

	testCases := []struct {
		value        any
		expectedPath string
	}{
		{"test", "name"},
		{123.0, "properties.id"},
		{true, "properties.active"},
		{"tag1", "tags[0]"},
		{"tag2", "tags[1]"},
	}

	for _, tc := range testCases {
		couple, err := df.FindVar(tc.value)
		if err != nil {
			t.Errorf("Failed to find value %v: %v", tc.value, err)
		}
		if couple.Path != tc.expectedPath {
			t.Errorf("For value %v, expected path '%s', got '%s'", tc.value, tc.expectedPath, couple.Path)
		}
	}
}

func TestFindInJsonBytes(t *testing.T) {
	jsonData := []byte(`{
		"name": "test",
		"properties": {
			"id": 123,
			"active": true
		},
		"tags": ["tag1", "tag2"],
		"nested": {
			"deep": {
				"value": "found me"
			}
		}
	}`)

	df := depfinder.NewDepFinder()

	testCases := []struct {
		value        any
		expectedPath string
	}{
		{"test", "name"},
		{123.0, "properties.id"},
		{true, "properties.active"},
		{"tag1", "tags[0]"},
		{"tag2", "tags[1]"},
		{"found me", "nested.deep.value"},
	}

	for _, tc := range testCases {
		path, err := df.FindInJsonBytes(jsonData, tc.value)
		if err != nil {
			t.Errorf("Failed to find value %v: %v", tc.value, err)
		}
		if path != tc.expectedPath {
			t.Errorf("For value %v, expected path '%s', got '%s'", tc.value, tc.expectedPath, path)
		}
	}

	// Test for value that doesn't exist
	_, err := df.FindInJsonBytes(jsonData, "non-existent")
	if err != depfinder.ErrNotFound {
		t.Errorf("Expected ErrNotFound for non-existent value, got %v", err)
	}

	// Test with invalid JSON
	_, err = df.FindInJsonBytes([]byte(`invalid json`), "test")
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestReplaceValueWithPath(t *testing.T) {
	df := depfinder.NewDepFinder()

	// Add some test variables
	df.AddVar("test-value", depfinder.VarCouple{Path: "config.name"})
	df.AddVar(42.0, depfinder.VarCouple{Path: "config.answer"})
	df.AddVar(true, depfinder.VarCouple{Path: "config.enabled"})

	testCases := []struct {
		value    any
		expected string
	}{
		{"test-value", "{{ config.name }}"},
		{42.0, "{{ config.answer }}"},
		{true, "{{ config.enabled }}"},
		{"unknown", "unknown"}, // Should return the original value for unknown values
	}

	for _, tc := range testCases {
		value, _, _ := df.ReplaceWithPaths(tc.value)
		if value != tc.expected {
			t.Errorf("For value %v, expected '%s', got '%s'", tc.value, tc.expected, value)
		}
	}
}

func TestTemplateJSON(t *testing.T) {
	// Create JSON with values we'll recognize
	jsonData := []byte(`{
		"name": "service-name.abc",
		"config": {
			"port": 8080,
			"debug": true
		},
		"tags": ["production", "api"],
		"nested": {
			"value": "secret-key"
		}
	}`)

	df := depfinder.NewDepFinder()

	// Add some known values
	df.AddVar("service-name", depfinder.VarCouple{Path: "app.name"})
	df.AddVar(8080.0, depfinder.VarCouple{Path: "app.port"})
	df.AddVar(true, depfinder.VarCouple{Path: "app.debug"})
	df.AddVar("production", depfinder.VarCouple{Path: "app.environment"})
	df.AddVar("secret-key", depfinder.VarCouple{Path: "app.credentials.key"})

	// Template the JSON
	result := df.TemplateJSON(jsonData)
	if result.Err != nil {
		t.Fatalf("Failed to template JSON: %v", result.Err)
	}

	// Parse the templated JSON to verify the values
	var resultMap map[string]any
	if err := json.Unmarshal(result.NewJson, &resultMap); err != nil {
		t.Fatalf("Failed to parse templated JSON: %v", err)
	}

	// Check the templated values
	expected := map[string]interface{}{
		"name": "service-name.abc",
		"config": map[string]interface{}{
			"port":  "{{ app.port }}",
			"debug": "{{ app.debug }}",
		},
		"tags": []any{
			"{{ app.environment }}",
			"api",
		},
		"nested": map[string]any{
			"value": "{{ app.credentials.key }}",
		},
	}

	// We need to convert to JSON and back to make sure the comparison is accurate
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(result.NewJson, expectedJSON) {
		t.Errorf("Templated JSON doesn't match expected result.\nGot: %s\nExpected: %s", result.NewJson, expectedJSON)
	}
}

func TestTemplateJSONWithSubstringValues(t *testing.T) {
	// Create JSON with values that contain substrings of our known variables
	jsonData := []byte(`{
		"service": "service-name-extended",
		"description": "This contains service-name somewhere",
		"config": {
			"setting": "prefix-secret-key-suffix"
		},
		"nested": {
			"properties": {
				"id": "app-123-production-env"
			}
		},
		"exact": {
			"service": "service-name",
			"key": "secret-key",
			"env": "production"
		}
	}`)

	df := depfinder.NewDepFinder()

	// Add variables that are substrings of values in our JSON
	df.AddVar("service-name", depfinder.VarCouple{Path: "app.name"})
	df.AddVar("secret-key", depfinder.VarCouple{Path: "app.credentials.key"})
	df.AddVar("production", depfinder.VarCouple{Path: "app.environment"})

	// Template the JSON
	result := df.TemplateJSON(jsonData)

	if result.Err != nil {
		t.Fatalf("Failed to template JSON: %v", result.Err)
	}

	// Parse the templated JSON to verify the values
	var resultMap map[string]any
	if err := json.Unmarshal(result.NewJson, &resultMap); err != nil {
		t.Fatalf("Failed to parse templated JSON: %v", err)
	}

	// Verify that strings containing our variables as substrings weren't replaced
	if resultMap["service"] != "service-name-extended" {
		t.Errorf("Expected 'service' to remain unchanged, got %v", resultMap["service"])
	}

	if resultMap["description"] != "This contains service-name somewhere" {
		t.Errorf("Expected 'description' to remain unchanged, got %v", resultMap["description"])
	}

	configMap := resultMap["config"].(map[string]any)
	if configMap["setting"] != "prefix-secret-key-suffix" {
		t.Errorf("Expected 'setting' to remain unchanged, got %v", configMap["setting"])
	}

	nestedMap := resultMap["nested"].(map[string]any)
	propertiesMap := nestedMap["properties"].(map[string]any)
	if propertiesMap["id"] != "app-123-production-env" {
		t.Errorf("Expected 'id' to remain unchanged, got %v", propertiesMap["id"])
	}

	// Verify that exact matches were replaced
	exactMap := resultMap["exact"].(map[string]any)
	if exactMap["service"] != "{{ app.name }}" {
		t.Errorf("Expected 'exact.service' to be templated, got %v", exactMap["service"])
	}
	if exactMap["key"] != "{{ app.credentials.key }}" {
		t.Errorf("Expected 'exact.key' to be templated, got %v", exactMap["key"])
	}
	if exactMap["env"] != "{{ app.environment }}" {
		t.Errorf("Expected 'exact.env' to be templated, got %v", exactMap["env"])
	}
}

func TestDepFinderPartialTokenAndRecursiveJSON(t *testing.T) {
	df := depfinder.NewDepFinder()

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiaWF0IjoxNzQ4MTg1MDkwLCJleHAiOjE3NDgyNzE0OTB9.TG4reOVX09bjGnB04xuYH0HrdfMcKn9vq03mG2aGa7Q"
	path := "auth.token"
	df.AddVar(token, depfinder.VarCouple{Path: path})

	// 1. Test Bearer header replacement
	header := "Bearer " + token
	templated, _, _ := df.ReplaceWithPathsSubstring(header)
	if templated != "Bearer {{ auth.token }}" {
		t.Errorf("Expected Bearer token to be templated, got: %v", templated)
	}

	// 2. Test query parameter replacement
	query := "?token=" + token
	templated, _, _ = df.ReplaceWithPathsSubstring(query)
	if templated != "?token={{ auth.token }}" {
		t.Errorf("Expected query token to be templated, got: %v", templated)
	}

	// 3. Test nested JSON replacement
	jsonData := []byte(`{"user": {"auth": {"token": "` + token + `"}}}`)
	result := df.TemplateJSON(jsonData)
	if result.Err != nil {
		t.Fatalf("Failed to template JSON: %v", result.Err)
	}
	var resultMap map[string]any
	if err := json.Unmarshal(result.NewJson, &resultMap); err != nil {
		t.Fatalf("Failed to parse templated JSON: %v", err)
	}
	userMap := resultMap["user"].(map[string]any)
	authMap := userMap["auth"].(map[string]any)
	tokenVal := authMap["token"]
	if tokenVal != "{{ auth.token }}" {
		t.Errorf("Expected nested JSON token to be templated, got: %v", tokenVal)
	}

	// 4. Test that unrelated values are not replaced
	unrelated := "no-token-here"
	templated, _, _ = df.ReplaceWithPaths(unrelated)
	if templated != unrelated {
		t.Errorf("Expected unrelated value to remain unchanged, got: %v", templated)
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"6d316d59-4cb4-451e-b5b1-673ecbdd5609", true},
		{"ef2574d1-1781-4ca9-bfcd-c571e124be02", true},
		{"c2b85766-9fb6-4a3b-a032-9628552cbdf2", true},
		{"not-a-uuid", false},
		{"", false},
		{"6d316d59-4cb4-451e-b5b1-673ecbdd560", false},   // too short
		{"6d316d59-4cb4-451e-b5b1-673ecbdd5609a", false}, // too long
		{"6d316d59X4cb4-451e-b5b1-673ecbdd5609", false},  // wrong separator
		{"6d316d59-4cb4-451e-b5b1-673ecbdd560g", false},  // invalid hex
	}

	for _, test := range tests {
		result := depfinder.IsUUID(test.input)
		if result != test.expected {
			t.Errorf("IsUUID(%s) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestReplaceURLPathParams(t *testing.T) {
	depFinder := depfinder.NewDepFinder()

	// Add some UUIDs to the depfinder
	nodeID1 := idwrap.NewNow()
	nodeID2 := idwrap.NewNow()

	depFinder.AddVar("6d316d59-4cb4-451e-b5b1-673ecbdd5609", depfinder.VarCouple{
		Path:   "request_1.response.body.id",
		NodeID: nodeID1,
	})

	depFinder.AddVar("ef2574d1-1781-4ca9-bfcd-c571e124be02", depfinder.VarCouple{
		Path:   "request_2.response.body.id",
		NodeID: nodeID2,
	})

	tests := []struct {
		name            string
		url             string
		expectedURL     string
		expectedFound   bool
		expectedCouples int
	}{
		{
			name:            "URL with UUID in path",
			url:             "https://example.com/api/products/6d316d59-4cb4-451e-b5b1-673ecbdd5609",
			expectedURL:     "https://example.com/api/products/{{ request_1.response.body.id }}",
			expectedFound:   true,
			expectedCouples: 1,
		},
		{
			name:            "URL with multiple UUIDs",
			url:             "https://example.com/api/products/6d316d59-4cb4-451e-b5b1-673ecbdd5609/tags/ef2574d1-1781-4ca9-bfcd-c571e124be02",
			expectedURL:     "https://example.com/api/products/{{ request_1.response.body.id }}/tags/{{ request_2.response.body.id }}",
			expectedFound:   true,
			expectedCouples: 2},
		{
			name:            "URL with unknown UUID",
			url:             "https://example.com/api/products/unknown-uuid-not-in-depfinder",
			expectedURL:     "https://example.com/api/products/unknown-uuid-not-in-depfinder",
			expectedFound:   false,
			expectedCouples: 0,
		},
		{
			name:            "URL without UUIDs",
			url:             "https://example.com/api/products",
			expectedURL:     "https://example.com/api/products",
			expectedFound:   false,
			expectedCouples: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resultURL, found, couples := depFinder.ReplaceURLPathParams(test.url)

			if resultURL != test.expectedURL {
				t.Errorf("Expected URL: %s, got: %s", test.expectedURL, resultURL)
			}

			if found != test.expectedFound {
				t.Errorf("Expected found: %v, got: %v", test.expectedFound, found)
			}

			if len(couples) != test.expectedCouples {
				t.Errorf("Expected %d couples, got: %d", test.expectedCouples, len(couples))
			}
		})
	}
}
