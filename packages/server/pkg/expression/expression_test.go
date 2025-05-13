package expression_test

import (
	"context"
	"fmt"
	"testing"

	"the-dev-tools/server/pkg/expression"
	"the-dev-tools/server/pkg/varsystem" // Import the actual varsystem package
)

func TestNormalizeExpression(t *testing.T) {
	// Setup actual varsystem.VarMap using NewVarMapFromAnyMap
	mockVarsAny := map[string]any{
		"var":     "value_of_var",
		"another": 42, // Use int for testing NewVarMapFromAnyMap
		"name":    "world",
		"a":       "123", // Example variable
	}
	// Use the actual varsystem function to create the VarMap
	varSystem := varsystem.NewVarMapFromAnyMap(mockVarsAny)

	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "Simple variable replacement",
			input:          "{{ var }}",
			expectedOutput: "value_of_var",
		},
		{
			name:           "Expression without variables",
			input:          "a == b",
			expectedOutput: "a == b",
		},
		{
			name:           "Expression with quoted variable",
			input:          `"{{ a }}" == a`,
			expectedOutput: `"123" == a`,
		},
		{
			name:           "String with variable",
			input:          "hello {{ name }}",
			expectedOutput: "hello world",
		},
		{
			name:           "Variable in expression (int converted to string)",
			input:          "{{ another }} + 1",
			expectedOutput: "42 + 1", // NewVarMapFromAnyMap converts int to string
		},
		{
			name:           "No variables",
			input:          "no vars here",
			expectedOutput: "no vars here",
		},
		{
			name:           "Multiple variables",
			input:          "{{ var }} and {{ another }}",
			expectedOutput: "value_of_var and 42",
		},
		{
			name:           "Variable not found",
			input:          "{{ unknown_var }}",
			expectedOutput: "{{ unknown_var }}", // varsystem.ReplaceVars leaves unknown vars as is
		},
		// Note: Testing the specific error handling of varsystem.ReplaceVars
		// (like {{error_var}} from the mock) would require mocking the file system
		// or other dependencies of varsystem.ReplaceVars. For this test,
		// we rely on the behavior of the actual varsystem.ReplaceVars.
		// The current varsystem.ReplaceVars returns an error on invalid format
		// or file read error, and NormalizeExpression returns the original string.
		// Let's add a case that triggers an invalid format error in varsystem.ReplaceVars.
		{
			name:           "Invalid variable format",
			input:          "invalid {{ var", // Missing closing suffix
			expectedOutput: "invalid {{ var", // varsystem.ReplaceVars returns error, NormalizeExpression returns original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function under test with the actual VarMap
			// Call the function under test with the actual VarMap
			actualOutput, err := expression.NormalizeExpression(context.Background(), tt.input, varSystem)

			// Assert the output matches the expected output
			// If an error occurred, we expect the original string to be returned.
			if err != nil {
				if actualOutput != tt.input {
					t.Errorf("NormalizeExpression(%q) returned error and %q; want original input %q on error", tt.input, actualOutput, tt.input)
				}
				// For the specific "Invalid variable format" case, we also check if the expected output matches the original input.
				// For other cases where no error is expected, the check below handles it.
				if tt.name == "Invalid variable format" && actualOutput != tt.expectedOutput {
					t.Errorf("NormalizeExpression(%q) returned error and %q; want %q on error", tt.input, actualOutput, tt.expectedOutput)
				}
			} else {
				// No error occurred, check if the output matches the expected output
				if actualOutput != tt.expectedOutput {
					t.Errorf("NormalizeExpression(%q) = %q; want %q", tt.input, actualOutput, tt.expectedOutput)
				}
			}
		})
	}
}

func TestExpressionEvaluteAsBool(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		envVars    map[string]any
		want       bool
		wantErr    bool
	}{
		{"Simple True", "a > 10", map[string]any{"a": 15}, true, false},
		{"Simple False", "a > 10", map[string]any{"a": 5}, false, false},
		{"String Equality True", "name == \"world\"", map[string]any{"name": "world"}, true, false},
		{"String Equality False", "name == \"universe\"", map[string]any{"name": "world"}, false, false},
		{"Logical AND True", "x > 5 && y < 10", map[string]any{"x": 6, "y": 9}, true, false},
		{"Logical AND False", "x > 5 && y < 10", map[string]any{"x": 4, "y": 9}, false, false},
		{"Variable Not Found", "z == 10", map[string]any{"a": 10}, false, true}, // Expect error
		{"Type Mismatch", "\"hello\" == 123", nil, false, true},                 // Expect error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new environment for each test case and populate it
			env := expression.NewEnv(tt.envVars)
			actualOutput, err := expression.ExpressionEvaluteAsBool(context.Background(), env, tt.expression)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExpressionEvaluteAsBool(\"%s\") returned error = %v, wantErr %v", tt.expression, err, tt.wantErr)
				return
			}
			if !tt.wantErr && actualOutput != tt.want {
				t.Errorf("ExpressionEvaluteAsBool(\"%s\") = %v; want %v", tt.expression, actualOutput, tt.want)
			}
		})
	}
}

func TestExpressionEvaluteAsArray(t *testing.T) {
	tests := []struct {
		name           string
		expression     string
		envVars        map[string]any
		expectedOutput []any
		expectedError  bool // true if an error is expected
	}{
		{
			name:           "Simple array literal (integers)",
			expression:     "[1, 2, 3]",
			envVars:        nil,
			expectedOutput: []any{1, 2, 3},
			expectedError:  false,
		},
		{
			name:           "Simple array literal (mixed types)",
			expression:     `[1, "hello", true, 3.14]`,
			envVars:        nil,
			expectedOutput: []any{1, "hello", true, 3.14},
			expectedError:  false,
		},
		{
			name:           "Empty array literal",
			expression:     "[]",
			envVars:        nil,
			expectedOutput: []any{},
			expectedError:  false,
		},
		{
			name:           "Array literal with variables",
			expression:     "[a, b, c]",
			envVars:        map[string]any{"a": 10, "b": "world", "c": false},
			expectedOutput: []any{10, "world", false},
			expectedError:  false,
		},
		{
			name:           "Array literal with missing variable (causes compile error)",
			expression:     "[a, missing_var]",
			envVars:        map[string]any{"a": 10},
			expectedOutput: nil,  // Output is nil on error
			expectedError:  true, // Should cause a compile error due to strict env check
		},
		{
			name:           "Using filter function",
			expression:     "filter([1, 2, 3, 4, 5], {# > 3})",
			envVars:        nil,
			expectedOutput: []any{4, 5},
			expectedError:  false,
		},
		{
			name:           "Using map function",
			expression:     "map([1, 2, 3], {# * 2})",
			envVars:        nil,
			expectedOutput: []any{2, 4, 6},
			expectedError:  false,
		},
		{
			name:           "Using map with variable",
			expression:     "map(numbers, {# + offset})",
			envVars:        map[string]any{"numbers": []any{1, 2, 3}, "offset": 10},
			expectedOutput: []any{11, 12, 13},
			expectedError:  false,
		},
		{
			name:           "Expression evaluates to non-array (string)",
			expression:     `"hello"`,
			envVars:        nil,
			expectedOutput: nil,  // Output is nil on error
			expectedError:  true, // Should cause a type assertion error in ExpressionEvaluteAsArray
		},
		{
			name:           "Expression evaluates to non-array (number)",
			expression:     `123`,
			envVars:        nil,
			expectedOutput: nil,  // Output is nil on error
			expectedError:  true, // Should cause a type assertion error
		},
		{
			name:           "Expression evaluates to non-array (boolean)",
			expression:     `true`,
			envVars:        nil,
			expectedOutput: nil,  // Output is nil on error
			expectedError:  true, // Should cause a type assertion error
		},
		{
			name:           "Invalid syntax",
			expression:     "[1, 2,", // Incomplete array
			envVars:        nil,
			expectedOutput: nil,  // Output is nil on error
			expectedError:  true, // Should cause a compile error
		},
		{
			name:           "Runtime error (index out of bounds)",
			expression:     "[1, 2, 3][5]",
			envVars:        nil,
			expectedOutput: nil,  // Output is nil on error
			expectedError:  true, // Should cause a runtime error
		},
		{
			name:           "Runtime error (accessing field on non-object)",
			expression:     `[1, 2, 3].field`,
			envVars:        nil,
			expectedOutput: nil,  // Output is nil on error
			expectedError:  true, // Should cause a runtime error
		},
		{
			name:       "Access nested field with hyphen and dot",
			expression: `data.request_8.response.body`,
			envVars: map[string]any{
				"data": map[string]any{
					"request_8": map[string]any{
						"response": map[string]any{
							"body": []any{"some body content"},
						},
					},
				},
			},
			expectedOutput: []any{"some body content"},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pass the raw map directly
			env := expression.NewEnv(tt.envVars)
			actualOutput, err := expression.ExpressionEvaluteAsArray(context.Background(), env, tt.expression)

			if tt.expectedError {
				if err == nil {
					t.Errorf("ExpressionEvaluteAsArray(%q) expected an error, but got none. Output: %v", tt.expression, actualOutput)
				}
				// We don't check actualOutput if an error is expected, as the return value is nil.
			} else {
				if err != nil {
					t.Errorf("ExpressionEvaluteAsArray(%q) returned an unexpected error: %v", tt.expression, err)
				}

				// Compare slices element by element
				if len(actualOutput) != len(tt.expectedOutput) {
					t.Errorf("ExpressionEvaluteAsArray(%q) returned array of length %d (%v); want length %d (%v)", tt.expression, len(actualOutput), actualOutput, len(tt.expectedOutput), tt.expectedOutput)
				} else {
					for i := range actualOutput {
						// Use fmt.Sprintf for comparison to handle different types gracefully
						if fmt.Sprintf("%v", actualOutput[i]) != fmt.Sprintf("%v", tt.expectedOutput[i]) {
							t.Errorf("ExpressionEvaluteAsArray(%q) element at index %d is %v (type %T); want %v (type %T)", tt.expression, i, actualOutput[i], actualOutput[i], tt.expectedOutput[i], tt.expectedOutput[i])
							break // Stop checking after the first mismatch
						}
					}
				}
			}
		})
	}
}
