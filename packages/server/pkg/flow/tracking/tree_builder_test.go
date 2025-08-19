package tracking

import (
	"reflect"
	"testing"
)

func TestBuildTree_SimpleNesting(t *testing.T) {
	flatMap := map[string]any{
		"request_0.response.body.token": "abc123",
		"request_0.response.status":     200,
	}

	expected := map[string]any{
		"request_0": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"token": "abc123",
				},
				"status": 200,
			},
		},
	}

	result := BuildTree(flatMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestBuildTree_MultipleRoots(t *testing.T) {
	flatMap := map[string]any{
		"request_0.response.body.token": "token123",
		"request_1.response.body.data":  "data456",
		"config.timeout":                30,
	}

	expected := map[string]any{
		"request_0": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"token": "token123",
				},
			},
		},
		"request_1": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"data": "data456",
				},
			},
		},
		"config": map[string]any{
			"timeout": 30,
		},
	}

	result := BuildTree(flatMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestBuildTree_ComplexValues(t *testing.T) {
	complexValue := map[string]interface{}{
		"nested": "value",
		"array":  []int{1, 2, 3},
	}

	flatMap := map[string]any{
		"request_0.response.body":    complexValue,
		"request_0.response.headers": map[string]string{"Content-Type": "application/json"},
	}

	expected := map[string]any{
		"request_0": map[string]any{
			"response": map[string]any{
				"body":    complexValue,
				"headers": map[string]string{"Content-Type": "application/json"},
			},
		},
	}

	result := BuildTree(flatMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestBuildTree_WithSpaces(t *testing.T) {
	// Test handling of keys with spaces (current real-world issue)
	flatMap := map[string]any{
		" request_0.response.body.token ": "abc123",
		"request_0.response.status":       200,
	}

	expected := map[string]any{
		"request_0": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"token": "abc123",
				},
				"status": 200,
			},
		},
	}

	result := BuildTree(flatMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestBuildTree_EmptyInput(t *testing.T) {
	flatMap := map[string]any{}

	expected := map[string]any{}

	result := BuildTree(flatMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestBuildTree_SingleKey(t *testing.T) {
	flatMap := map[string]any{
		"simple": "value",
	}

	expected := map[string]any{
		"simple": "value",
	}

	result := BuildTree(flatMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestBuildTree_DeepNesting(t *testing.T) {
	flatMap := map[string]any{
		"a.b.c.d.e.f": "deep_value",
	}

	expected := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": map[string]any{
						"e": map[string]any{
							"f": "deep_value",
						},
					},
				},
			},
		},
	}

	result := BuildTree(flatMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestMergeTreesPreferFirst(t *testing.T) {
	first := map[string]any{
		"a": map[string]any{
			"b": "first_value",
			"c": "only_in_first",
		},
		"only_first": "value1",
	}

	second := map[string]any{
		"a": map[string]any{
			"b": "second_value", // Should be ignored (prefer first)
			"d": "only_in_second",
		},
		"only_second": "value2",
	}

	expected := map[string]any{
		"a": map[string]any{
			"b": "first_value",    // Kept from first
			"c": "only_in_first",  // Kept from first
			"d": "only_in_second", // Added from second
		},
		"only_first":  "value1", // Kept from first
		"only_second": "value2", // Added from second
	}

	result := MergeTreesPreferFirst(first, second)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestDeepCopyValue_PreventsMutation(t *testing.T) {
	original := map[string]any{
		"nested": map[string]any{
			"value": "original",
		},
	}

	copied := deepCopyValue(original)

	// Modify the copied value
	if copiedMap, ok := copied.(map[string]any); ok {
		if nestedMap, ok := copiedMap["nested"].(map[string]any); ok {
			nestedMap["value"] = "modified"
		}
	}

	// Original should be unchanged
	if nestedMap, ok := original["nested"].(map[string]any); ok {
		if nestedMap["value"] != "original" {
			t.Error("Original value was modified, deep copy failed")
		}
	}
}
