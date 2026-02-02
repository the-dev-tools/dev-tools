package expression

import (
	"context"
	"strings"
	"testing"
)

func TestBuiltinAI_ErrorWhenNotFound(t *testing.T) {
	env := NewUnifiedEnv(nil)

	_, err := env.Eval(context.Background(), `ai("userId", "The user ID", "number")`)
	if err == nil {
		t.Fatal("expected error when variable not found, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestBuiltinAI_ErrorWhenNameEmpty(t *testing.T) {
	env := NewUnifiedEnv(nil)

	_, err := env.Eval(context.Background(), `ai("", "description", "string")`)
	if err == nil {
		t.Fatal("expected error when name is empty, got nil")
	}

	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}

func TestBuiltinAI_ResolvesWhenSet(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expr     string
		expected any
	}{
		{
			name: "ai returns string value",
			data: map[string]any{
				"userId": "12345",
			},
			expr:     `ai("userId", "The user ID to fetch", "number")`,
			expected: "12345",
		},
		{
			name: "ai returns numeric value",
			data: map[string]any{
				"count": 42,
			},
			expr:     `ai("count", "Number of items", "number")`,
			expected: 42,
		},
		{
			name: "ai returns nested value",
			data: map[string]any{
				"user": map[string]any{
					"id": "nested-id",
				},
			},
			expr:     `ai("user.id", "User ID", "string")`,
			expected: "nested-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewUnifiedEnv(tt.data)
			result, err := env.Eval(context.Background(), tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestBuiltinAI_InterpolationWithSetVar(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"userId":      "user-12345",
		"existingVar": "test-value",
	})

	input := `Create a user with ID {{ ai("userId", "user identifier", "string") }} and name {{ existingVar }}`

	result, err := env.Interpolate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "user-12345") {
		t.Errorf("result should contain resolved userId, got: %s", result)
	}

	if !strings.Contains(result, "test-value") {
		t.Errorf("result should contain resolved existingVar, got: %s", result)
	}
}

func TestBuiltinAI_InterpolationErrorWhenNotFound(t *testing.T) {
	env := NewUnifiedEnv(map[string]any{
		"existingVar": "test-value",
	})

	input := `Create a user with ID {{ ai("userId", "user identifier", "string") }}`

	_, err := env.Interpolate(input)
	if err == nil {
		t.Fatal("expected error when variable not found, got nil")
	}
}
