package expression

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/secretresolver"
)

// mockSecretResolver is a test double for secretresolver.SecretResolver.
type mockSecretResolver struct {
	secrets map[string]string // "provider:ref#fragment" -> value
	err     error
}

var _ secretresolver.SecretResolver = (*mockSecretResolver)(nil)

func (m *mockSecretResolver) ResolveSecret(_ context.Context, provider, ref, fragment string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	key := provider + ":" + ref
	if fragment != "" {
		key += "#" + fragment
	}
	val, ok := m.secrets[key]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", key)
	}
	return val, nil
}

// =============================================================================
// Path Resolution Tests
// =============================================================================

func TestResolvePath_Simple(t *testing.T) {
	data := map[string]any{
		"name":  "John",
		"count": 42,
	}

	tests := []struct {
		name     string
		path     string
		expected any
		found    bool
	}{
		{"simple string", "name", "John", true},
		{"simple number", "count", 42, true},
		{"missing key", "missing", nil, false},
		{"empty path", "", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ResolvePath(data, tt.path)
			require.Equal(t, tt.found, ok)
			if tt.found {
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestResolvePath_Nested(t *testing.T) {
	data := map[string]any{
		"node": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"id":   123,
					"name": "test",
				},
				"status": 200,
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected any
		found    bool
	}{
		{"nested one level", "node", data["node"], true},
		{"nested two levels", "node.response", data["node"].(map[string]any)["response"], true},
		{"nested three levels", "node.response.status", 200, true},
		{"nested four levels", "node.response.body.id", 123, true},
		{"missing nested", "node.missing", nil, false},
		{"missing deep nested", "node.response.missing.field", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ResolvePath(data, tt.path)
			require.Equal(t, tt.found, ok)
			if tt.found {
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestResolvePath_ArrayIndex(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"id": 1, "name": "first"},
			map[string]any{"id": 2, "name": "second"},
			map[string]any{"id": 3, "name": "third"},
		},
		"numbers": []any{10, 20, 30},
	}

	tests := []struct {
		name     string
		path     string
		expected any
		found    bool
	}{
		{"array first element", "items[0]", data["items"].([]any)[0], true},
		{"array second element", "items[1]", data["items"].([]any)[1], true},
		{"array element property", "items[0].id", 1, true},
		{"array element property name", "items[1].name", "second", true},
		{"simple array element", "numbers[0]", 10, true},
		{"simple array last element", "numbers[2]", 30, true},
		{"out of bounds", "items[10]", nil, false},
		{"negative index", "items[-1]", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ResolvePath(data, tt.path)
			require.Equal(t, tt.found, ok)
			if tt.found {
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestResolvePath_Mixed(t *testing.T) {
	data := map[string]any{
		"nodes": []any{
			map[string]any{
				"response": map[string]any{
					"headers": map[string]any{
						"Content-Type": "application/json",
					},
					"body": []any{"item1", "item2"},
				},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected any
		found    bool
	}{
		{"mixed path", "nodes[0].response.headers.Content-Type", "application/json", true},
		{"nested array in object", "nodes[0].response.body[0]", "item1", true},
		{"nested array second item", "nodes[0].response.body[1]", "item2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ResolvePath(data, tt.path)
			require.Equal(t, tt.found, ok)
			if tt.found {
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSetPath(t *testing.T) {
	t.Run("set simple value", func(t *testing.T) {
		data := map[string]any{}
		err := SetPath(data, "name", "John")
		require.NoError(t, err)
		require.Equal(t, "John", data["name"])
	})

	t.Run("set nested value creates intermediate maps", func(t *testing.T) {
		data := map[string]any{}
		err := SetPath(data, "user.profile.name", "John")
		require.NoError(t, err)

		user := data["user"].(map[string]any)
		profile := user["profile"].(map[string]any)
		require.Equal(t, "John", profile["name"])
	})

	t.Run("set value in existing nested structure", func(t *testing.T) {
		data := map[string]any{
			"user": map[string]any{
				"profile": map[string]any{
					"name": "OldName",
				},
			},
		}
		err := SetPath(data, "user.profile.name", "NewName")
		require.NoError(t, err)

		value, ok := ResolvePath(data, "user.profile.name")
		require.True(t, ok)
		require.Equal(t, "NewName", value)
	})

	t.Run("set array element", func(t *testing.T) {
		data := map[string]any{
			"items": []any{"a", "b", "c"},
		}
		err := SetPath(data, "items[1]", "updated")
		require.NoError(t, err)
		require.Equal(t, "updated", data["items"].([]any)[1])
	})

	t.Run("error on nil map", func(t *testing.T) {
		err := SetPath(nil, "key", "value")
		require.Error(t, err)
	})

	t.Run("error on empty path", func(t *testing.T) {
		data := map[string]any{}
		err := SetPath(data, "", "value")
		require.Error(t, err)
	})
}

// =============================================================================
// UnifiedEnv Tests
// =============================================================================

func TestUnifiedEnv_NewAndClone(t *testing.T) {
	t.Run("create new env", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		env := NewUnifiedEnv(data)
		require.NotNil(t, env)
		require.Equal(t, "value", env.GetData()["key"])
	})

	t.Run("create from nil data", func(t *testing.T) {
		env := NewUnifiedEnv(nil)
		require.NotNil(t, env)
		require.NotNil(t, env.GetData())
	})

	t.Run("clone preserves data", func(t *testing.T) {
		env := NewUnifiedEnv(map[string]any{"key": "value"})
		clone := env.Clone()

		require.Equal(t, env.GetData()["key"], clone.GetData()["key"])

		// Modify clone shouldn't affect original
		clone.GetData()["key"] = "modified"
		require.Equal(t, "value", env.GetData()["key"])
	})
}

func TestUnifiedEnv_GetAndSet(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"name": "John",
		"user": map[string]any{
			"profile": map[string]any{
				"age": 30,
			},
		},
	})

	t.Run("get simple value", func(t *testing.T) {
		val, ok := env.Get("name")
		require.True(t, ok)
		require.Equal(t, "John", val)
	})

	t.Run("get nested value", func(t *testing.T) {
		val, ok := env.Get("user.profile.age")
		require.True(t, ok)
		require.Equal(t, 30, val)
	})

	t.Run("has returns true for existing", func(t *testing.T) {
		require.True(t, env.Has("name"))
		require.True(t, env.Has("user.profile.age"))
	})

	t.Run("has returns false for missing", func(t *testing.T) {
		require.False(t, env.Has("missing"))
		require.False(t, env.Has("user.missing.field"))
	})

	t.Run("set creates nested path", func(t *testing.T) {
		err := env.Set("new.nested.value", "created")
		require.NoError(t, err)

		val, ok := env.Get("new.nested.value")
		require.True(t, ok)
		require.Equal(t, "created", val)
	})
}

func TestUnifiedEnv_WithTracking(t *testing.T) {
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(map[string]any{
		"name": "John",
		"age":  30,
	}).WithTracking(tracker)

	// Access some variables
	_, _ = env.Get("name")
	_, _ = env.Get("age")

	// Check tracking
	readVars := tracker.GetReadVars()
	require.Len(t, readVars, 2)
	require.Equal(t, "John", readVars["name"])
	require.Equal(t, 30, readVars["age"])
}

func TestUnifiedEnv_WithFunc(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"value": 5,
	}).WithFunc("double", func(x int) int {
		return x * 2
	})

	ctx := context.Background()
	result, err := env.Eval(ctx, "double(value)")
	require.NoError(t, err)
	require.Equal(t, 10, result)
}

func TestUnifiedEnv_Merge(t *testing.T) {
	env1 := NewUnifiedEnv(map[string]any{
		"a": 1,
		"b": 2,
	})
	env2 := NewUnifiedEnv(map[string]any{
		"b": 20, // Override
		"c": 3,
	})

	merged := env1.Merge(env2)

	require.Equal(t, 1, merged.GetData()["a"])
	require.Equal(t, 20, merged.GetData()["b"]) // env2 takes precedence
	require.Equal(t, 3, merged.GetData()["c"])
}

// =============================================================================
// Interpolation Tests
// =============================================================================

func TestInterpolate_SingleVar(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"name": "World",
	})

	result, err := env.Interpolate("Hello, {{ name }}!")
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", result)
}

func TestInterpolate_MultipleVars(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"firstName": "John",
		"lastName":  "Doe",
		"age":       30,
	})

	result, err := env.Interpolate("Name: {{ firstName }} {{ lastName }}, Age: {{ age }}")
	require.NoError(t, err)
	require.Equal(t, "Name: John Doe, Age: 30", result)
}

func TestInterpolate_NestedPath(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"user": map[string]any{
			"profile": map[string]any{
				"name": "John",
			},
		},
	})

	result, err := env.Interpolate("User: {{ user.profile.name }}")
	require.NoError(t, err)
	require.Equal(t, "User: John", result)
}

func TestInterpolate_ArrayAccess(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"items": []any{
			map[string]any{"name": "first"},
			map[string]any{"name": "second"},
		},
	})

	result, err := env.Interpolate("First: {{ items[0].name }}, Second: {{ items[1].name }}")
	require.NoError(t, err)
	require.Equal(t, "First: first, Second: second", result)
}

func TestInterpolate_EnvVar(t *testing.T) {
	// Set an environment variable for testing
	t.Setenv("TEST_VAR", "test_value")

	env := NewUnifiedEnv(nil)
	result, err := env.Interpolate("Value: {{ #env:TEST_VAR }}")
	require.NoError(t, err)
	require.Equal(t, "Value: test_value", result)
}

func TestInterpolate_FileContent(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte("file content"), 0o644)
	require.NoError(t, err)

	env := NewUnifiedEnv(nil)
	result, err := env.Interpolate("Content: {{ #file:" + tmpFile + " }}")
	require.NoError(t, err)
	require.Equal(t, "Content: file content", result)
}

func TestInterpolate_MissingVar(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{})

	_, err := env.Interpolate("Hello, {{ missing }}!")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing")
}

func TestInterpolate_Expression(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"a": 5,
		"b": 3,
	})

	// Expression inside {{ }} should be evaluated
	result, err := env.Interpolate("Result: {{ a + b }}")
	require.NoError(t, err)
	require.Equal(t, "Result: 8", result)
}

func TestInterpolate_BoolExpression(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"count": 10,
	})

	result, err := env.Interpolate("Is big: {{ count > 5 }}")
	require.NoError(t, err)
	require.Equal(t, "Is big: true", result)
}

func TestInterpolate_FunctionCall(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"items": []any{1, 2, 3},
	}).WithFunc("len", func(arr []any) int {
		return len(arr)
	})

	result, err := env.Interpolate("Count: {{ len(items) }}")
	require.NoError(t, err)
	require.Equal(t, "Count: 3", result)
}

// =============================================================================
// ResolveValue Tests - Returns typed values
// =============================================================================

func TestResolveValue_SingleExpr_ReturnsTyped(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"count":  10,
		"active": true,
		"name":   "John",
	})

	t.Run("returns int", func(t *testing.T) {
		val, err := env.ResolveValue("{{ count }}")
		require.NoError(t, err)
		require.Equal(t, 10, val)
	})

	t.Run("returns bool", func(t *testing.T) {
		val, err := env.ResolveValue("{{ active }}")
		require.NoError(t, err)
		require.Equal(t, true, val)
	})

	t.Run("returns string", func(t *testing.T) {
		val, err := env.ResolveValue("{{ name }}")
		require.NoError(t, err)
		require.Equal(t, "John", val)
	})

	t.Run("evaluates expression", func(t *testing.T) {
		val, err := env.ResolveValue("{{ count > 5 }}")
		require.NoError(t, err)
		require.Equal(t, true, val)
	})

	t.Run("evaluates math", func(t *testing.T) {
		val, err := env.ResolveValue("{{ count * 2 }}")
		require.NoError(t, err)
		require.Equal(t, 20, val)
	})
}

func TestResolveValue_WithText_ReturnsString(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"id": 123,
	})

	// When there's text around {{ }}, always returns string
	val, err := env.ResolveValue("user/{{ id }}/profile")
	require.NoError(t, err)
	require.Equal(t, "user/123/profile", val)
}

func TestResolveValue_NoVars_ReturnsAsIs(t *testing.T) {
	env := NewUnifiedEnv(nil)

	val, err := env.ResolveValue("plain text")
	require.NoError(t, err)
	require.Equal(t, "plain text", val)
}

func TestResolveValue_FunctionCall(t *testing.T) {
	env := NewUnifiedEnv(nil).WithFunc("now", func() string {
		return "2024-01-01"
	})

	val, err := env.ResolveValue("{{ now() }}")
	require.NoError(t, err)
	require.Equal(t, "2024-01-01", val)
}

func TestInterpolate_TracksReads(t *testing.T) {
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(map[string]any{
		"name": "John",
		"age":  30,
	}).WithTracking(tracker)

	result, err := env.InterpolateWithResult("{{ name }} is {{ age }}")
	require.NoError(t, err)
	require.Equal(t, "John is 30", result.Value)

	// Check that reads were tracked
	require.Len(t, result.ReadVars, 2)
	require.Equal(t, "John", result.ReadVars["name"])
	require.Equal(t, 30, result.ReadVars["age"])
}

func TestInterpolate_NoVars(t *testing.T) {
	env := NewUnifiedEnv(nil)
	result, err := env.Interpolate("Plain text without variables")
	require.NoError(t, err)
	require.Equal(t, "Plain text without variables", result)
}

func TestInterpolate_EmptyString(t *testing.T) {
	env := NewUnifiedEnv(nil)
	result, err := env.Interpolate("")
	require.NoError(t, err)
	require.Equal(t, "", result)
}

// =============================================================================
// Evaluation Tests
// =============================================================================

func TestEval_Basic(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"a": 10,
		"b": 5,
	})
	ctx := context.Background()

	tests := []struct {
		name     string
		expr     string
		expected any
	}{
		{"addition", "a + b", 15},
		{"subtraction", "a - b", 5},
		{"multiplication", "a * b", 50},
		{"division", "a / b", float64(2)}, // expr-lang returns float64 for division
		{"comparison", "a > b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := env.Eval(ctx, tt.expr)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalBool(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"active": true,
		"count":  10,
		"name":   "test",
	})
	ctx := context.Background()

	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{"simple true", "active", true},
		{"comparison", "count > 5", true},
		{"string comparison", "name == 'test'", true},
		{"logical and", "active && count > 5", true},
		{"logical or", "!active || count > 5", true},
		{"negation", "!active", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := env.EvalBool(ctx, tt.expr)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalString(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"name":   "John",
		"count":  42,
		"active": true,
	})
	ctx := context.Background()

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"string value", "name", "John"},
		{"number to string", "count", "42"},
		{"bool to string", "active", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := env.EvalString(ctx, tt.expr)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalIter_Array(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"items": []any{"a", "b", "c"},
	})
	ctx := context.Background()

	result, err := env.EvalIter(ctx, "items")
	require.NoError(t, err)

	seq, ok := result.(iter.Seq[any])
	require.True(t, ok, "expected iter.Seq[any]")

	var collected []any
	for item := range seq {
		collected = append(collected, item)
	}

	require.Equal(t, []any{"a", "b", "c"}, collected)
}

func TestEvalIter_Map(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"config": map[string]any{
			"a": 1,
			"b": 2,
		},
	})
	ctx := context.Background()

	result, err := env.EvalIter(ctx, "config")
	require.NoError(t, err)

	seq, ok := result.(iter.Seq2[string, any])
	require.True(t, ok, "expected iter.Seq2[string, any]")

	collected := make(map[string]any)
	for k, v := range seq {
		collected[k] = v
	}

	require.Equal(t, map[string]any{"a": 1, "b": 2}, collected)
}

func TestEvalIter_EmptyOnNil(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"value": nil,
	})
	ctx := context.Background()

	result, err := env.EvalIter(ctx, "value")
	require.NoError(t, err)

	seq, ok := result.(iter.Seq[any])
	require.True(t, ok)

	count := 0
	for range seq {
		count++
	}
	require.Equal(t, 0, count)
}

func TestEval_HelperFunctions(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"user": map[string]any{
			"name": "John",
		},
	})
	ctx := context.Background()

	t.Run("get helper", func(t *testing.T) {
		result, err := env.Eval(ctx, `get("user.name")`)
		require.NoError(t, err)
		require.Equal(t, "John", result)
	})

	t.Run("has helper true", func(t *testing.T) {
		result, err := env.EvalBool(ctx, `has("user.name")`)
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("has helper false", func(t *testing.T) {
		result, err := env.EvalBool(ctx, `has("user.missing")`)
		require.NoError(t, err)
		require.False(t, result)
	})
}

func TestEval_CustomFunction(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"items": []any{1, 2, 3, 4, 5},
	}).WithFunc("sum", func(arr []any) int {
		total := 0
		for _, v := range arr {
			if n, ok := v.(int); ok {
				total += n
			}
		}
		return total
	})
	ctx := context.Background()

	result, err := env.Eval(ctx, "sum(items)")
	require.NoError(t, err)
	require.Equal(t, 15, result)
}

func TestEval_WithInterpolation(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"threshold": 10,
		"value":     15,
	})
	ctx := context.Background()

	// Expression that uses both interpolation and evaluation
	result, err := env.EvalBool(ctx, "value > threshold")
	require.NoError(t, err)
	require.True(t, result)
}

func TestEval_SyntaxError(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{})
	ctx := context.Background()

	_, err := env.Eval(ctx, "invalid syntax +++")
	require.Error(t, err)
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestEdgeCases_NilEnv(t *testing.T) {
	var env *UnifiedEnv

	t.Run("Get on nil", func(t *testing.T) {
		val, ok := env.Get("key")
		require.False(t, ok)
		require.Nil(t, val)
	})

	t.Run("Has on nil", func(t *testing.T) {
		require.False(t, env.Has("key"))
	})

	t.Run("GetData on nil", func(t *testing.T) {
		data := env.GetData()
		require.NotNil(t, data)
		require.Len(t, data, 0)
	})

	t.Run("Clone on nil", func(t *testing.T) {
		clone := env.Clone()
		require.NotNil(t, clone)
	})
}

func TestEdgeCases_TypeConversions(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"int":     42,
		"float":   3.14,
		"bool":    true,
		"string":  "hello",
		"nil":     nil,
		"intflt":  float64(100), // JSON numbers come as float64
		"bytes":   []byte("binary"),
		"complex": map[string]any{"nested": "value"},
	})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"int to string", "{{ int }}", "42"},
		{"float to string", "{{ float }}", "3.14"},
		{"bool to string", "{{ bool }}", "true"},
		{"string stays string", "{{ string }}", "hello"},
		{"nil to empty", "{{ nil }}", ""},
		{"int as float64", "{{ intflt }}", "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := env.Interpolate(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractVarRefs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single var", "{{ name }}", []string{"name"}},
		{"multiple vars", "{{ first }} {{ second }}", []string{"first", "second"}},
		{"duplicate vars", "{{ name }} and {{ name }}", []string{"name"}},
		{"nested path", "{{ user.profile.name }}", []string{"user.profile.name"}},
		{"env var", "{{ #env:VAR }}", []string{"#env:VAR"}},
		{"file ref", "{{ #file:/path }}", []string{"#file:/path"}},
		{"no vars", "plain text", nil},
		{"empty string", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractVarRefs(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestHasVars(t *testing.T) {
	require.True(t, HasVars("{{ var }}"))
	require.True(t, HasVars("prefix {{ var }} suffix"))
	require.False(t, HasVars("no variables"))
	require.False(t, HasVars("{{ incomplete"))
	require.False(t, HasVars("incomplete }}"))
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkUnifiedEnv_EvalBool_PureExpr(b *testing.B) {
	// Pure expression without {{ }} - measures baseline expr-lang performance
	env := NewUnifiedEnv(map[string]any{
		"count": 10,
		"limit": 5,
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.EvalBool(ctx, "count > limit")
	}
}

func BenchmarkUnifiedEnv_EvalBool_WithInterpolation(b *testing.B) {
	// Expression with {{ }} interpolation - measures interpolation overhead
	env := NewUnifiedEnv(map[string]any{
		"config": map[string]any{
			"threshold": 5,
		},
		"value": 10,
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.EvalBool(ctx, "value > {{ config.threshold }}")
	}
}

func BenchmarkUnifiedEnv_Interpolate_SingleVar(b *testing.B) {
	env := NewUnifiedEnv(map[string]any{
		"name": "John",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.Interpolate("Hello, {{ name }}!")
	}
}

func BenchmarkUnifiedEnv_Interpolate_MultipleVars(b *testing.B) {
	env := NewUnifiedEnv(map[string]any{
		"first":  "John",
		"last":   "Doe",
		"age":    30,
		"city":   "NYC",
		"active": true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.Interpolate("{{ first }} {{ last }}, age {{ age }}, lives in {{ city }}, active: {{ active }}")
	}
}

func BenchmarkUnifiedEnv_Interpolate_NestedPath(b *testing.B) {
	env := NewUnifiedEnv(map[string]any{
		"user": map[string]any{
			"profile": map[string]any{
				"settings": map[string]any{
					"theme": "dark",
				},
			},
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.Interpolate("Theme: {{ user.profile.settings.theme }}")
	}
}

func BenchmarkUnifiedEnv_Interpolate_NoVars(b *testing.B) {
	// Measures overhead of scanning for {{ }} when there are none
	env := NewUnifiedEnv(map[string]any{
		"name": "John",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.Interpolate("Plain text without any variables at all")
	}
}

func BenchmarkResolvePath_Simple(b *testing.B) {
	data := map[string]any{
		"name": "John",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ResolvePath(data, "name")
	}
}

func BenchmarkResolvePath_Nested(b *testing.B) {
	data := map[string]any{
		"user": map[string]any{
			"profile": map[string]any{
				"name": "John",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ResolvePath(data, "user.profile.name")
	}
}

func BenchmarkResolvePath_ArrayAccess(b *testing.B) {
	data := map[string]any{
		"items": []any{
			map[string]any{"id": 1},
			map[string]any{"id": 2},
			map[string]any{"id": 3},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ResolvePath(data, "items[1].id")
	}
}

func BenchmarkHasVars(b *testing.B) {
	b.Run("no_vars", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = HasVars("Plain text without variables")
		}
	})
	b.Run("has_vars", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = HasVars("Hello {{ name }}, welcome!")
		}
	})
}

// =============================================================================
// ExtractExprPaths Tests
// =============================================================================

func TestExtractExprPaths(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected []string
	}{
		{
			name:     "simple identifier",
			expr:     "flag",
			expected: []string{"flag"},
		},
		{
			name:     "nested path",
			expr:     "node.response.status",
			expected: []string{"node.response.status"},
		},
		{
			name:     "comparison expression",
			expr:     "node.response.status == 200",
			expected: []string{"node.response.status"},
		},
		{
			name:     "multiple paths",
			expr:     "nodeA.result && nodeB.value > 30",
			expected: []string{"nodeA.result", "nodeB.value"},
		},
		{
			name:     "complex expression",
			expr:     "node.response.body.items[0].id == config.expected",
			expected: []string{"node.response.body.items", "config.expected"},
		},
		{
			name:     "skip keywords",
			expr:     "true && false || nil",
			expected: []string{}, // All are keywords - empty result
		},
		{
			name:     "skip built-in functions",
			expr:     "len(items) > 0",
			expected: []string{"items"},
		},
		{
			name:     "string comparison",
			expr:     `status == "success"`,
			expected: []string{"status"},
		},
		{
			name:     "empty string",
			expr:     "",
			expected: nil,
		},
		{
			name:     "invalid expression",
			expr:     "invalid +++",
			expected: nil,
		},
		{
			name:     "bracket notation",
			expr:     `env["api.key"]`,
			expected: []string{"env.api.key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractExprPaths(tt.expr)
			// Convert to sets for comparison since order is not guaranteed
			resultSet := make(map[string]bool)
			for _, p := range result {
				resultSet[p] = true
			}
			expectedSet := make(map[string]bool)
			for _, p := range tt.expected {
				expectedSet[p] = true
			}
			require.Equal(t, expectedSet, resultSet)
		})
	}
}

// =============================================================================
// Variable Tracking in Eval Functions Tests
// =============================================================================

func TestUnifiedEnv_EvalBool_TracksVariables(t *testing.T) {
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(map[string]any{
		"node": map[string]any{
			"response": map[string]any{
				"status": 200,
			},
		},
	}).WithTracking(tracker)
	ctx := context.Background()

	result, err := env.EvalBool(ctx, "node.response.status == 200")
	require.NoError(t, err)
	require.True(t, result)

	// Verify tracking - the full path should be tracked
	readVars := tracker.GetReadVars()
	require.NotEmpty(t, readVars, "expected variables to be tracked")
	require.Contains(t, readVars, "node.response.status")
}

func TestUnifiedEnv_EvalBool_TracksMultiplePaths(t *testing.T) {
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(map[string]any{
		"nodeA": map[string]any{
			"result": "success",
		},
		"nodeB": map[string]any{
			"value": 42,
		},
		"config": map[string]any{
			"enabled": true,
		},
	}).WithTracking(tracker)
	ctx := context.Background()

	result, err := env.EvalBool(ctx, `nodeA.result == "success" && nodeB.value > 30 && config.enabled`)
	require.NoError(t, err)
	require.True(t, result)

	// Verify all paths were tracked
	readVars := tracker.GetReadVars()
	require.Contains(t, readVars, "nodeA.result")
	require.Contains(t, readVars, "nodeB.value")
	require.Contains(t, readVars, "config.enabled")
}

func TestUnifiedEnv_Eval_TracksVariables(t *testing.T) {
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(map[string]any{
		"a": 10,
		"b": 5,
	}).WithTracking(tracker)
	ctx := context.Background()

	result, err := env.Eval(ctx, "a + b")
	require.NoError(t, err)
	require.Equal(t, 15, result)

	// Verify tracking
	readVars := tracker.GetReadVars()
	require.Contains(t, readVars, "a")
	require.Contains(t, readVars, "b")
}

func TestUnifiedEnv_EvalIter_TracksVariables(t *testing.T) {
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(map[string]any{
		"data": map[string]any{
			"items": []any{1, 2, 3},
		},
	}).WithTracking(tracker)
	ctx := context.Background()

	result, err := env.EvalIter(ctx, "data.items")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify tracking
	readVars := tracker.GetReadVars()
	require.Contains(t, readVars, "data.items")
}

func TestUnifiedEnv_EvalString_TracksVariables(t *testing.T) {
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(map[string]any{
		"user": map[string]any{
			"name": "John",
		},
	}).WithTracking(tracker)
	ctx := context.Background()

	result, err := env.EvalString(ctx, "user.name")
	require.NoError(t, err)
	require.Equal(t, "John", result)

	// Verify tracking (EvalString uses Eval internally)
	readVars := tracker.GetReadVars()
	require.Contains(t, readVars, "user.name")
}

func TestUnifiedEnv_EvalBool_NoTrackingWhenNilTracker(t *testing.T) {
	// Test that eval works without a tracker (no panic)
	env := NewUnifiedEnv(map[string]any{
		"flag": true,
	})
	ctx := context.Background()

	result, err := env.EvalBool(ctx, "flag")
	require.NoError(t, err)
	require.True(t, result)
}

// =============================================================================
// Benchmarks for Tracking Overhead
// =============================================================================

func BenchmarkUnifiedEnv_EvalBool_WithTracking(b *testing.B) {
	env := NewUnifiedEnv(map[string]any{
		"node": map[string]any{
			"response": map[string]any{
				"status": 200,
			},
		},
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker := tracking.NewVariableTracker()
		envWithTracking := env.WithTracking(tracker)
		_, _ = envWithTracking.EvalBool(ctx, "node.response.status == 200")
	}
}

func BenchmarkUnifiedEnv_EvalBool_WithoutTracking(b *testing.B) {
	env := NewUnifiedEnv(map[string]any{
		"node": map[string]any{
			"response": map[string]any{
				"status": 200,
			},
		},
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = env.EvalBool(ctx, "node.response.status == 200")
	}
}

func BenchmarkExtractExprPaths(b *testing.B) {
	b.Run("simple", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ExtractExprPaths("flag")
		}
	})
	b.Run("complex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ExtractExprPaths(`nodeA.result == "success" && nodeB.value > 30 && config.enabled`)
		}
	})
}

// =============================================================================
// Cloud Secret Reference Tests
// =============================================================================

func TestInterpolate_GCPSecret_SimpleValue(t *testing.T) {
	resolver := &mockSecretResolver{
		secrets: map[string]string{
			"gcp:projects/p/secrets/s/versions/latest": "my-secret-value",
		},
	}
	env := NewUnifiedEnv(nil).WithSecretResolver(resolver)

	result, err := env.Interpolate("Secret: {{ #gcp:projects/p/secrets/s/versions/latest }}")
	require.NoError(t, err)
	require.Equal(t, "Secret: my-secret-value", result)
}

func TestInterpolate_GCPSecret_WithFragment(t *testing.T) {
	resolver := &mockSecretResolver{
		secrets: map[string]string{
			"gcp:projects/p/secrets/oauth/versions/latest#client_secret": "xyz-secret",
		},
	}
	env := NewUnifiedEnv(nil).WithSecretResolver(resolver)

	result, err := env.Interpolate("{{ #gcp:projects/p/secrets/oauth/versions/latest#client_secret }}")
	require.NoError(t, err)
	require.Equal(t, "xyz-secret", result)
}

func TestInterpolate_GCPSecret_NoResolver(t *testing.T) {
	env := NewUnifiedEnv(nil) // No secret resolver configured

	_, err := env.Interpolate("{{ #gcp:projects/p/secrets/s/versions/latest }}")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no secret resolver configured")

	var secretErr *SecretReferenceError
	require.ErrorAs(t, err, &secretErr)
}

func TestInterpolate_GCPSecret_EmptyPath(t *testing.T) {
	resolver := &mockSecretResolver{secrets: map[string]string{}}
	env := NewUnifiedEnv(nil).WithSecretResolver(resolver)

	_, err := env.Interpolate("{{ #gcp: }}")
	require.Error(t, err)
}

func TestInterpolate_GCPSecret_MixedReferences(t *testing.T) {
	t.Setenv("TEST_MIX_VAR", "env-value")

	resolver := &mockSecretResolver{
		secrets: map[string]string{
			"gcp:projects/p/secrets/s/versions/1": "secret-value",
		},
	}
	env := NewUnifiedEnv(map[string]any{
		"name": "John",
	}).WithSecretResolver(resolver)

	result, err := env.Interpolate("{{ name }} {{ #env:TEST_MIX_VAR }} {{ #gcp:projects/p/secrets/s/versions/1 }}")
	require.NoError(t, err)
	require.Equal(t, "John env-value secret-value", result)
}

func TestInterpolate_GCPSecret_TrackedAsMasked(t *testing.T) {
	resolver := &mockSecretResolver{
		secrets: map[string]string{
			"gcp:projects/p/secrets/s/versions/latest": "super-secret",
		},
	}
	tracker := tracking.NewVariableTracker()
	env := NewUnifiedEnv(nil).WithSecretResolver(resolver).WithTracking(tracker)

	result, err := env.Interpolate("{{ #gcp:projects/p/secrets/s/versions/latest }}")
	require.NoError(t, err)
	require.Equal(t, "super-secret", result)

	// Tracker should record masked value, not the actual secret
	readVars := tracker.GetReadVars()
	require.Contains(t, readVars, "#gcp:projects/p/secrets/s/versions/latest")
	require.Equal(t, "***", readVars["#gcp:projects/p/secrets/s/versions/latest"])
}

func TestInterpolate_GCPSecret_ResolverError(t *testing.T) {
	resolver := &mockSecretResolver{
		err: fmt.Errorf("permission denied"),
	}
	env := NewUnifiedEnv(nil).WithSecretResolver(resolver)

	_, err := env.Interpolate("{{ #gcp:projects/p/secrets/s/versions/latest }}")
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

func TestResolveValue_GCPSecret(t *testing.T) {
	resolver := &mockSecretResolver{
		secrets: map[string]string{
			"gcp:projects/p/secrets/s/versions/latest": "typed-secret",
		},
	}
	env := NewUnifiedEnv(nil).WithSecretResolver(resolver)

	val, err := env.ResolveValue("{{ #gcp:projects/p/secrets/s/versions/latest }}")
	require.NoError(t, err)
	require.Equal(t, "typed-secret", val)
}

func TestInterpolate_AWSSecret_NoResolver(t *testing.T) {
	// AWS prefix is recognized but no resolver means clear error
	env := NewUnifiedEnv(nil)

	_, err := env.Interpolate("{{ #aws:my-secret#key }}")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no secret resolver configured")
}

func TestInterpolate_AzureSecret_NoResolver(t *testing.T) {
	// Azure prefix is recognized but no resolver means clear error
	env := NewUnifiedEnv(nil)

	_, err := env.Interpolate("{{ #azure:vault/secret#key }}")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no secret resolver configured")
}

// =============================================================================
// Secret Reference Helper Tests
// =============================================================================

func TestIsSecretReference(t *testing.T) {
	require.True(t, IsSecretReference("#gcp:projects/p/secrets/s"))
	require.True(t, IsSecretReference("#aws:my-secret"))
	require.True(t, IsSecretReference("#azure:vault/secret"))
	require.False(t, IsSecretReference("#env:VAR"))
	require.False(t, IsSecretReference("#file:/path"))
	require.False(t, IsSecretReference("plain"))
}

func TestParseSecretReference(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedProvider string
		expectedRef      string
		expectedFragment string
	}{
		{
			name:             "gcp with fragment",
			input:            "#gcp:projects/p/secrets/s/versions/latest#client_secret",
			expectedProvider: "gcp",
			expectedRef:      "projects/p/secrets/s/versions/latest",
			expectedFragment: "client_secret",
		},
		{
			name:             "gcp without fragment",
			input:            "#gcp:projects/p/secrets/s/versions/latest",
			expectedProvider: "gcp",
			expectedRef:      "projects/p/secrets/s/versions/latest",
			expectedFragment: "",
		},
		{
			name:             "aws with fragment",
			input:            "#aws:secret-name#key",
			expectedProvider: "aws",
			expectedRef:      "secret-name",
			expectedFragment: "key",
		},
		{
			name:             "azure with fragment",
			input:            "#azure:vault/secret-name#field",
			expectedProvider: "azure",
			expectedRef:      "vault/secret-name",
			expectedFragment: "field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, ref, fragment := ParseSecretReference(tt.input)
			require.Equal(t, tt.expectedProvider, provider)
			require.Equal(t, tt.expectedRef, ref)
			require.Equal(t, tt.expectedFragment, fragment)
		})
	}
}

func TestWithSecretResolver_ClonePreservesResolver(t *testing.T) {
	resolver := &mockSecretResolver{
		secrets: map[string]string{
			"gcp:projects/p/secrets/s/versions/latest": "value",
		},
	}
	env := NewUnifiedEnv(nil).WithSecretResolver(resolver)

	// Clone should preserve the resolver
	clone := env.Clone()
	result, err := clone.Interpolate("{{ #gcp:projects/p/secrets/s/versions/latest }}")
	require.NoError(t, err)
	require.Equal(t, "value", result)
}

func TestExtractVarRefs_IncludesSecretRefs(t *testing.T) {
	refs := ExtractVarRefs("{{ #gcp:projects/p/secrets/s/versions/latest#key }} and {{ name }}")
	require.Len(t, refs, 2)
	require.Contains(t, refs, "#gcp:projects/p/secrets/s/versions/latest#key")
	require.Contains(t, refs, "name")
}
