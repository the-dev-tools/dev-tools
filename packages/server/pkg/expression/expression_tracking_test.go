package expression

import (
	"context"
	"testing"

	"iter"
	"the-dev-tools/server/pkg/flow/tracking"

	"github.com/stretchr/testify/require"
)

func TestExpressionEvaluteAsBool_WithTracking(t *testing.T) {
	env := NewEnv(map[string]any{
		"flag":   true,
		"count":  5,
		"unused": "not accessed",
	})

	tracker := tracking.NewVariableTracker()

	// Test expression that should evaluate to true
	result, err := ExpressionEvaluteAsBoolWithTracking(context.Background(), env, "flag && count > 3", tracker)
	require.NoError(t, err, "Expression evaluation failed")
	if !result {
		t.Errorf("Expected true, got %v", result)
	}

	// Verify variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 3 {
		t.Errorf("Expected 3 tracked variables, got %d", len(readVars))
	}

	if readVars["flag"] != true {
		t.Errorf("Expected flag=true, got %v", readVars["flag"])
	}
	if readVars["count"] != 5 {
		t.Errorf("Expected count=5, got %v", readVars["count"])
	}
	if readVars["unused"] != "not accessed" {
		t.Errorf("Expected unused='not accessed', got %v", readVars["unused"])
	}
}

func TestExpressionEvaluteAsBool_WithoutTracking(t *testing.T) {
	env := NewEnv(map[string]any{
		"flag": true,
	})

	// Test with nil tracker should use regular function
	result, err := ExpressionEvaluteAsBoolWithTracking(context.Background(), env, "flag", nil)
	require.NoError(t, err, "Expression evaluation failed")
	if !result {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestExpressionEvaluteAsArray_WithTracking(t *testing.T) {
	env := NewEnv(map[string]any{
		"items":      []any{1, 2, 3},
		"multiplier": 2,
	})

	tracker := tracking.NewVariableTracker()

	// Test array expression
	result, err := ExpressionEvaluteAsArrayWithTracking(context.Background(), env, "items", tracker)
	require.NoError(t, err, "Expression evaluation failed")
	if len(result) != 3 {
		t.Errorf("Expected array length 3, got %d", len(result))
	}

	// Verify variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 2 {
		t.Errorf("Expected 2 tracked variables, got %d", len(readVars))
	}

	if trackedItems, ok := readVars["items"].([]any); ok {
		if len(trackedItems) != 3 {
			t.Errorf("Expected tracked items length 3, got %d", len(trackedItems))
		}
	} else {
		t.Errorf("Expected items to be []any, got %T", readVars["items"])
	}
}

func TestExpressionEvaluateAsIter_WithTracking(t *testing.T) {
	env := NewEnv(map[string]any{
		"data": map[string]any{
			"a": 1,
			"b": 2,
		},
		"otherVar": "test",
	})

	tracker := tracking.NewVariableTracker()

	// Test iterator expression
	result, err := ExpressionEvaluateAsIterWithTracking(context.Background(), env, "data", tracker)
	require.NoError(t, err, "Expression evaluation failed")

	// Should return an iterator
	if result == nil {
		t.Error("Expected non-nil iterator result")
	}

	// Verify variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 2 {
		t.Errorf("Expected 2 tracked variables, got %d", len(readVars))
	}

	if trackedData, ok := readVars["data"].(map[string]any); ok {
		if len(trackedData) != 2 {
			t.Errorf("Expected tracked data length 2, got %d", len(trackedData))
		}
		if trackedData["a"] != 1 {
			t.Errorf("Expected data.a=1, got %v", trackedData["a"])
		}
		if trackedData["b"] != 2 {
			t.Errorf("Expected data.b=2, got %v", trackedData["b"])
		}
	} else {
		t.Errorf("Expected data to be map[string]any, got %T", readVars["data"])
	}
}

func TestExpressionEvaluateAsIterWithTracking_EmptyString(t *testing.T) {
	env := NewEnv(map[string]any{"value": ""})

	tracker := tracking.NewVariableTracker()

	seqAny, err := ExpressionEvaluateAsIterWithTracking(context.Background(), env, "value", tracker)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	seq, ok := seqAny.(iter.Seq[any])
	if !ok {
		t.Fatalf("expected iter.Seq[any], got %T", seqAny)
	}

	for range seq {
		t.Fatalf("expected empty sequence, but iterator yielded elements")
	}

	readVars := tracker.GetReadVars()
	val, ok := readVars["value"]
	if !ok {
		t.Fatalf("expected tracker to record read for value")
	}
	if val != "" {
		t.Fatalf("expected tracker to record empty string, got %v", val)
	}
}

func TestExpression_VariableAccess_Tracking(t *testing.T) {
	env := NewEnv(map[string]any{
		"nodeA": map[string]interface{}{
			"result": "success",
			"code":   200,
		},
		"nodeB": map[string]interface{}{
			"value": 42,
		},
		"config": map[string]interface{}{
			"enabled": true,
		},
	})

	tracker := tracking.NewVariableTracker()

	// Test complex expression accessing nested values
	result, err := ExpressionEvaluteAsBoolWithTracking(context.Background(), env, "nodeA.result == \"success\" && nodeB.value > 30 && config.enabled", tracker)
	require.NoError(t, err, "Expression evaluation failed")
	if !result {
		t.Errorf("Expected true, got %v", result)
	}

	// Verify all variables were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 3 {
		t.Errorf("Expected 3 tracked variables, got %d", len(readVars))
	}

	// Check nodeA was tracked
	if nodeA, ok := readVars["nodeA"].(map[string]interface{}); ok {
		if nodeA["result"] != "success" {
			t.Errorf("Expected nodeA.result='success', got %v", nodeA["result"])
		}
		if nodeA["code"] != 200 {
			t.Errorf("Expected nodeA.code=200, got %v", nodeA["code"])
		}
	} else {
		t.Errorf("Expected nodeA to be tracked as map, got %T", readVars["nodeA"])
	}

	// Check nodeB was tracked
	if nodeB, ok := readVars["nodeB"].(map[string]interface{}); ok {
		if nodeB["value"] != 42 {
			t.Errorf("Expected nodeB.value=42, got %v", nodeB["value"])
		}
	} else {
		t.Errorf("Expected nodeB to be tracked as map, got %T", readVars["nodeB"])
	}

	// Check config was tracked
	if config, ok := readVars["config"].(map[string]interface{}); ok {
		if config["enabled"] != true {
			t.Errorf("Expected config.enabled=true, got %v", config["enabled"])
		}
	} else {
		t.Errorf("Expected config to be tracked as map, got %T", readVars["config"])
	}
}

func TestExpression_TrackingWithError(t *testing.T) {
	env := NewEnv(map[string]any{
		"validVar": "test",
	})

	tracker := tracking.NewVariableTracker()

	// Test expression that should fail
	_, err := ExpressionEvaluteAsBoolWithTracking(context.Background(), env, "invalidVar == true", tracker)
	if err == nil {
		t.Error("Expected error for invalid expression, got nil")
	}

	// Even though expression failed, all variables should still be tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 1 {
		t.Errorf("Expected 1 tracked variable even on error, got %d", len(readVars))
	}
}

func BenchmarkExpressionEvaluteAsBool_WithTracking(b *testing.B) {
	env := NewEnv(map[string]any{
		"flag":   true,
		"count":  5,
		"result": "success",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker := tracking.NewVariableTracker()
		_, err := ExpressionEvaluteAsBoolWithTracking(context.Background(), env, "flag && count > 3 && result == \"success\"", tracker)
		if err != nil {
			b.Fatalf("Expression evaluation failed: %v", err)
		}
	}
}

func BenchmarkExpressionEvaluteAsBool_WithoutTracking(b *testing.B) {
	env := NewEnv(map[string]any{
		"flag":   true,
		"count":  5,
		"result": "success",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ExpressionEvaluteAsBool(context.Background(), env, "flag && count > 3 && result == \"success\"")
		if err != nil {
			b.Fatalf("Expression evaluation failed: %v", err)
		}
	}
}
