package referencecompletion_test

import (
	"reflect"
	"sort"
	"testing"

	"the-dev-tools/server/pkg/referencecompletion"
)

func TestAddPaths(t *testing.T) {
	creator := referencecompletion.NewReferenceCompletionCreator()

	// Test with a nested map
	testData := map[string]any{
		"key1": "value1",
		"key2": map[string]any{
			"nestedKey1": "nestedValue1",
			"nestedKey2": []any{"item1", "item2"},
		},
	}
	creator.Add(testData)

	expectedPaths := []string{
		"key1",
		"key2",
		"key2.nestedKey1",
		"key2.nestedKey2",
		"key2.nestedKey2[0]",
		"key2.nestedKey2[1]",
	}
	sort.Strings(expectedPaths)

	actualPaths := make([]string, 0, len(creator.PathMap))
	for path := range creator.PathMap {
		actualPaths = append(actualPaths, path)
	}
	sort.Strings(actualPaths)

	if !reflect.DeepEqual(expectedPaths, actualPaths) {
		t.Errorf("PathMap mismatch:\nExpected: %v\nActual:   %v", expectedPaths, actualPaths)
	}
}

func TestFindMatch(t *testing.T) {
	creator := referencecompletion.NewReferenceCompletionCreator()

	// Add some paths
	creator.Add(map[string]any{
		"users": map[string]any{
			"user1": "data1",
			"user2": "data2",
		},
		"settings": "config",
		"deep": map[string]any{
			"nested": map[string]any{
				"value": "found",
			},
			"other": "stuff",
		},
	})

	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "Simple match",
			query:    "user",
			expected: []string{"users", "users.user1", "users.user2"},
		},
		{
			name:     "Nested match",
			query:    "nested",
			expected: []string{"deep.nested", "deep.nested.value"},
		},
		{
			name:     "Full path match",
			query:    "deep.nested.value",
			expected: []string{"deep.nested.value"},
		},
		{
			name:     "No match",
			query:    "nomatch",
			expected: []string{},
		},
		{
			name:     "Partial nested match",
			query:    "deep.nes",
			expected: []string{"deep.nested", "deep.nested.value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := creator.FindMatch(tt.query)
			actualMatches := make([]string, len(matches))
			for i, match := range matches {
				actualMatches[i] = match.Target
			}
			sort.Strings(actualMatches)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(tt.expected, actualMatches) {
				t.Errorf("FindMatch(%q) mismatch:\nExpected: %v\nActual:   %v", tt.query, tt.expected, actualMatches)
			}
		})
	}
}

func TestFindMatchNoResults(t *testing.T) {
	creator := referencecompletion.NewReferenceCompletionCreator()

	// Add some paths
	creator.Add(map[string]any{
		"users": map[string]any{
			"user1": "data1",
			"user2": "data2",
		},
		"settings": "config",
		"deep": map[string]any{
			"nested": map[string]any{
				"value": "found",
			},
		},
	})

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "Empty query",
			query: "",
		},
		{
			name:  "Non-existent path",
			query: "nonexistentpath",
		},
		{
			name:  "Special characters",
			query: "!@#$%^&*()",
		},
		{
			name:  "Similar but not matching",
			query: "userX",
		},
		{
			name:  "Too long query",
			query: "deep.nested.value.that.does.not.exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := creator.FindMatch(tt.query)
			if len(matches) != 0 {
				t.Errorf("FindMatch(%q) expected empty array, but got: %v", tt.query, matches)
			}
		})
	}
}

func TestReferenceCompletionLookUp_Add(t *testing.T) {
	lookup := referencecompletion.NewReferenceCompletionLookup()

	// Test with a nested map
	testData := map[string]any{
		"key1": "value1",
		"key2": map[string]any{
			"nestedKey1": "nestedValue1",
			"nestedKey2": []any{"item1", "item2"},
		},
	}
	lookup.Add(testData)

	// Test if root data was stored
	rootData, err := lookup.GetValue("")
	if err != nil {
		t.Errorf("Failed to get root data: %v", err)
	}
	if !reflect.DeepEqual(rootData, testData) {
		t.Errorf("Root data mismatch:\nExpected: %v\nActual:   %v", testData, rootData)
	}

	// Test a few paths
	tests := []struct {
		path     string
		expected any
		hasError bool
	}{
		{"key1", "value1", false},
		{"key2.nestedKey1", "nestedValue1", false},
		{"key2.nestedKey2[0]", "item1", false},
		{"key2.nestedKey2[1]", "item2", false},
		{"nonexistent", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			value, err := lookup.GetValue(tt.path)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for path '%s', but got none", tt.path)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
				}
				if !reflect.DeepEqual(value, tt.expected) {
					t.Errorf("GetValue(%q) = %v, want %v", tt.path, value, tt.expected)
				}
			}
		})
	}
}

func TestReferenceCompletionLookUp_AddWithKey(t *testing.T) {
	lookup := referencecompletion.NewReferenceCompletionLookup()

	// Add data with specific key
	userData := map[string]any{
		"name":  "John",
		"email": "john@example.com",
	}

	lookup.AddWithKey("user", userData)

	// Test if data was stored with the correct key
	value, err := lookup.GetValue("user")
	if err != nil {
		t.Errorf("Failed to get value for 'user': %v", err)
	}
	if !reflect.DeepEqual(value, userData) {
		t.Errorf("User data mismatch:\nExpected: %v\nActual:   %v", userData, value)
	}

	// Test nested values
	email, err := lookup.GetValue("user.email")
	if err != nil {
		t.Errorf("Failed to get value for 'user.email': %v", err)
	}
	if email != "john@example.com" {
		t.Errorf("Email mismatch: expected 'john@example.com', got '%v'", email)
	}
}

func TestReferenceCompletionLookUp_GetValue(t *testing.T) {
	lookup := referencecompletion.NewReferenceCompletionLookup()

	// Test data with various nested structures
	testData := map[string]any{
		"string": "value",
		"number": 42,
		"bool":   true,
		"nested": map[string]any{
			"key":  "nestedValue",
			"nums": []int{10, 20, 30},
			"deep": map[string]any{
				"level3": "deep value",
			},
		},
		"array": []any{
			"first",
			map[string]any{"key": "valueInArray"},
			[]int{1, 2, 3},
		},
	}

	lookup.Add(testData)

	tests := []struct {
		name     string
		path     string
		expected any
		hasError bool
	}{
		{
			name:     "Empty path",
			path:     "",
			expected: testData,
			hasError: false,
		},
		{
			name:     "Simple property",
			path:     "string",
			expected: "value",
			hasError: false,
		},
		{
			name:     "Numeric property",
			path:     "number",
			expected: 42,
			hasError: false,
		},
		{
			name:     "Boolean property",
			path:     "bool",
			expected: true,
			hasError: false,
		},
		{
			name:     "Nested property",
			path:     "nested.key",
			expected: "nestedValue",
			hasError: false,
		},
		{
			name:     "Array element",
			path:     "nested.nums[1]",
			expected: 20,
			hasError: false,
		},
		{
			name:     "Deep nested property",
			path:     "nested.deep.level3",
			expected: "deep value",
			hasError: false,
		},
		{
			name:     "Array access",
			path:     "array[0]",
			expected: "first",
			hasError: false,
		},
		{
			name:     "Map in array",
			path:     "array[1].key",
			expected: "valueInArray",
			hasError: false,
		},
		{
			name:     "Array in array",
			path:     "array[2][0]",
			expected: 1,
			hasError: false,
		},
		{
			name:     "Invalid property",
			path:     "nonexistent",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Index out of bounds",
			path:     "array[10]",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Invalid array index",
			path:     "array[notanumber]",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Array index on non-array",
			path:     "string[0]",
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := lookup.GetValue(tt.path)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for path '%s', but got none", tt.path)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for path '%s': %v", tt.path, err)
				}
				if !reflect.DeepEqual(value, tt.expected) {
					t.Errorf("GetValue(%q) = %v, want %v", tt.path, value, tt.expected)
				}
			}
		})
	}
}

func TestParsePath(t *testing.T) {
	// Since parsePath is a private function, we need to test it indirectly through GetValue
	// We'll use some test cases to verify its behavior

	lookup := referencecompletion.NewReferenceCompletionLookup()
	testData := map[string]any{"a": map[string]any{"b": []any{1, 2, 3}}}
	lookup.Add(testData)

	tests := []struct {
		path     string
		expected any
		valid    bool
	}{
		{"a.b[0]", 1, true},
		{"a.b[1]", 2, true},
		{"a.b[2]", 3, true},
		{"a.b", []any{1, 2, 3}, true},
		// Test complex paths
		{"a.b[0].c", nil, false}, // Invalid path
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			value, err := lookup.GetValue(tt.path)
			if tt.valid && err != nil {
				t.Errorf("Error parsing valid path '%s': %v", tt.path, err)
			}
			if tt.valid && !reflect.DeepEqual(value, tt.expected) {
				t.Errorf("Path '%s' returned %v, want %v", tt.path, value, tt.expected)
			}
			if !tt.valid && err == nil {
				t.Errorf("Expected error for invalid path '%s', but got none", tt.path)
			}
		})
	}
}
