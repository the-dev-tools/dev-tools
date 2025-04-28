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
