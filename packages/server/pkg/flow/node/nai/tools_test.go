package nai

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
)

func TestVariableTools(t *testing.T) {
	ctx := context.Background()
	tracker := tracking.NewVariableTracker()
	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"existing": "value",
		},
		ReadWriteLock:   &sync.RWMutex{},
		VariableTracker: tracker,
	}

	t.Run("get_variable", func(t *testing.T) {
		res, err := handleGetVariable(ctx, req, `{"key": "existing"}`)
		assert.NoError(t, err)
		assert.Equal(t, `"value"`, res)

		_, err = handleGetVariable(ctx, req, `{"key": "missing"}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("set_variable", func(t *testing.T) {
		res, err := handleSetVariable(ctx, req, `{"key": "new", "value": 123}`)
		assert.NoError(t, err)
		assert.Contains(t, res, "Successfully")

		val, err := node.ReadVarRaw(req, "new")
		assert.NoError(t, err)
		assert.Equal(t, float64(123), val) // json.Unmarshal defaults to float64 for numbers
	})

	t.Run("set_and_get_nested_path", func(t *testing.T) {
		// Test nested path access - this was the original bug
		// AI node sets "ai_1.id" and other nodes need to access it via {{ ai_1.id }}

		// Set a nested value
		res, err := handleSetVariable(ctx, req, `{"key": "ai_1.id", "value": 42}`)
		assert.NoError(t, err)
		assert.Contains(t, res, "Successfully")

		// Verify it's stored in a nested structure (not as flat key "ai_1.id")
		ai1, ok := req.VarMap["ai_1"]
		assert.True(t, ok, "ai_1 should exist as top-level key")
		ai1Map, ok := ai1.(map[string]any)
		assert.True(t, ok, "ai_1 should be a map")
		assert.Equal(t, float64(42), ai1Map["id"])

		// Get the nested value using dotted path
		res, err = handleGetVariable(ctx, req, `{"key": "ai_1.id"}`)
		assert.NoError(t, err)
		assert.Equal(t, "42", res)

		// Set another nested value on the same node
		_, err = handleSetVariable(ctx, req, `{"key": "ai_1.name", "value": "test"}`)
		assert.NoError(t, err)

		// Both values should exist
		res, err = handleGetVariable(ctx, req, `{"key": "ai_1.id"}`)
		assert.NoError(t, err)
		assert.Equal(t, "42", res)

		res, err = handleGetVariable(ctx, req, `{"key": "ai_1.name"}`)
		assert.NoError(t, err)
		assert.Equal(t, `"test"`, res)
	})

	t.Run("get_deeply_nested_path", func(t *testing.T) {
		// Test accessing deeply nested values like node outputs
		req.VarMap["http_1"] = map[string]any{
			"response": map[string]any{
				"status": float64(200),
				"body": map[string]any{
					"id":   "abc123",
					"name": "Test",
				},
			},
		}

		res, err := handleGetVariable(ctx, req, `{"key": "http_1.response.status"}`)
		assert.NoError(t, err)
		assert.Equal(t, "200", res)

		res, err = handleGetVariable(ctx, req, `{"key": "http_1.response.body.id"}`)
		assert.NoError(t, err)
		assert.Equal(t, `"abc123"`, res)
	})
}

func TestSanitizeToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain", "my_tool", "my_tool"},
		{"with spaces", "my tool", "my_tool"},
		{"special chars", "my-tool! @#$", "my-tool_____"},
		{"leading/trailing", "  tool  ", "__tool__"},
		{"alphanumeric", "Tool123", "Tool123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeToolName(tt.input))
		})
	}
}
