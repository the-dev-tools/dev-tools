package depfinder_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"the-dev-tools/backend/pkg/depfinder"
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
		t.Errorf("Templated JSON doesn't match expected result.\nGot: %v\nExpected: %v", result, expectedJSON)
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
}
