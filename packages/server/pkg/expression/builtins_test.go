package expression

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

// =============================================================================
// UUID Built-in Tests
// =============================================================================

var (
	uuidV4Regex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	uuidV7Regex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func TestBuiltinUUID_DefaultIsV4(t *testing.T) {
	env := NewUnifiedEnv(nil)
	ctx := context.Background()

	result, err := env.Eval(ctx, "uuid()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}

	if !uuidV4Regex.MatchString(str) {
		t.Errorf("expected valid UUID v4, got: %s", str)
	}
}

func TestBuiltinUUID_ExplicitV4(t *testing.T) {
	env := NewUnifiedEnv(nil)
	ctx := context.Background()

	result, err := env.Eval(ctx, `uuid("v4")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}

	if !uuidV4Regex.MatchString(str) {
		t.Errorf("expected valid UUID v4, got: %s", str)
	}
}

func TestBuiltinUUID_V7(t *testing.T) {
	env := NewUnifiedEnv(nil)
	ctx := context.Background()

	result, err := env.Eval(ctx, `uuid("v7")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}

	if !uuidV7Regex.MatchString(str) {
		t.Errorf("expected valid UUID v7, got: %s", str)
	}
}

func TestBuiltinUUID_InvalidVersion(t *testing.T) {
	env := NewUnifiedEnv(nil)
	ctx := context.Background()

	_, err := env.Eval(ctx, `uuid("v5")`)
	if err == nil {
		t.Fatal("expected error for unsupported version, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported version") {
		t.Errorf("expected 'unsupported version' in error, got: %v", err)
	}
}

func TestBuiltinUUID_UniquePerCall(t *testing.T) {
	env := NewUnifiedEnv(nil)
	ctx := context.Background()

	result1, err := env.Eval(ctx, "uuid()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result2, err := env.Eval(ctx, "uuid()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1 == result2 {
		t.Errorf("expected unique UUIDs, got same value twice: %v", result1)
	}
}

func TestBuiltinUUID_Interpolation(t *testing.T) {
	env := NewUnifiedEnv(nil)

	result, err := env.Interpolate("id={{ uuid() }}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(result, "id=") {
		t.Errorf("expected 'id=' prefix, got: %s", result)
	}

	uuidPart := strings.TrimPrefix(result, "id=")
	if !uuidV4Regex.MatchString(uuidPart) {
		t.Errorf("expected valid UUID v4 after prefix, got: %s", uuidPart)
	}
}

func TestBuiltinUUID_V7Interpolation(t *testing.T) {
	env := NewUnifiedEnv(nil)

	result, err := env.Interpolate(`id={{ uuid("v7") }}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	uuidPart := strings.TrimPrefix(result, "id=")
	if !uuidV7Regex.MatchString(uuidPart) {
		t.Errorf("expected valid UUID v7 after prefix, got: %s", uuidPart)
	}
}

// =============================================================================
// ULID Built-in Tests
// =============================================================================

var ulidRegex = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func TestBuiltinULID_Eval(t *testing.T) {
	env := NewUnifiedEnv(nil)
	ctx := context.Background()

	result, err := env.Eval(ctx, "ulid()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}

	if !ulidRegex.MatchString(str) {
		t.Errorf("expected valid ULID, got: %s", str)
	}
}

func TestBuiltinULID_UniquePerCall(t *testing.T) {
	env := NewUnifiedEnv(nil)
	ctx := context.Background()

	result1, err := env.Eval(ctx, "ulid()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result2, err := env.Eval(ctx, "ulid()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result1 == result2 {
		t.Errorf("expected unique ULIDs, got same value twice: %v", result1)
	}
}

func TestBuiltinULID_Interpolation(t *testing.T) {
	env := NewUnifiedEnv(nil)

	result, err := env.Interpolate("id={{ ulid() }}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(result, "id=") {
		t.Errorf("expected 'id=' prefix, got: %s", result)
	}

	ulidPart := strings.TrimPrefix(result, "id=")
	if !ulidRegex.MatchString(ulidPart) {
		t.Errorf("expected valid ULID after prefix, got: %s", ulidPart)
	}
}

// =============================================================================
// AI Built-in Tests
// =============================================================================

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
