//nolint:revive // test file
package rgraphql

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

func TestCreateAssertionEvalContext(t *testing.T) {
	t.Parallel()

	srv := &GraphQLServiceRPC{}

	tests := []struct {
		name     string
		response GraphQLResponseData
		validate func(t *testing.T, ctx map[string]any)
	}{
		{
			name: "basic JSON response",
			response: GraphQLResponseData{
				StatusCode: 200,
				Body:       []byte(`{"data": {"user": {"name": "Alice"}}}`),
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			},
			validate: func(t *testing.T, ctx map[string]any) {
				// Check status code
				if status, ok := ctx["status"].(int); !ok || status != 200 {
					t.Errorf("expected status 200, got %v", ctx["status"])
				}

				// Check success flag
				if success, ok := ctx["success"].(bool); !ok || !success {
					t.Errorf("expected success=true for 2xx status, got %v", ctx["success"])
				}

				// Check is_json flag
				if isJSON, ok := ctx["is_json"].(bool); !ok || !isJSON {
					t.Errorf("expected is_json=true for JSON content-type, got %v", ctx["is_json"])
				}

				// Check body parsing
				if body, ok := ctx["body"].(map[string]any); !ok {
					t.Errorf("expected body to be parsed as map, got %T", ctx["body"])
				} else {
					if data, ok := body["data"].(map[string]any); !ok {
						t.Errorf("expected body.data to exist")
					} else {
						if user, ok := data["user"].(map[string]any); !ok {
							t.Errorf("expected body.data.user to exist")
						} else {
							if name, ok := user["name"].(string); !ok || name != "Alice" {
								t.Errorf("expected body.data.user.name='Alice', got %v", name)
							}
						}
					}
				}

				// Check JSON path helpers
				if jsonHelpers, ok := ctx["json"].(map[string]any); !ok {
					t.Errorf("expected json helpers to exist")
				} else {
					// Test path helper
					if pathFn, ok := jsonHelpers["path"].(func(string) any); ok {
						result := pathFn("data.user.name")
						if name, ok := result.(string); !ok || name != "Alice" {
							t.Errorf("json.path('data.user.name') expected 'Alice', got %v", result)
						}
					} else {
						t.Errorf("expected json.path function to exist")
					}

					// Test has helper
					if hasFn, ok := jsonHelpers["has"].(func(string) bool); ok {
						if !hasFn("data.user.name") {
							t.Errorf("json.has('data.user.name') should return true")
						}
						if hasFn("data.missing") {
							t.Errorf("json.has('data.missing') should return false")
						}
					} else {
						t.Errorf("expected json.has function to exist")
					}
				}
			},
		},
		{
			name: "client error response",
			response: GraphQLResponseData{
				StatusCode: 404,
				Body:       []byte(`{"error": "Not found"}`),
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			},
			validate: func(t *testing.T, ctx map[string]any) {
				if status, ok := ctx["status"].(int); !ok || status != 404 {
					t.Errorf("expected status 404, got %v", ctx["status"])
				}

				if success, ok := ctx["success"].(bool); !ok || success {
					t.Errorf("expected success=false for 4xx status, got %v", ctx["success"])
				}

				if clientError, ok := ctx["client_error"].(bool); !ok || !clientError {
					t.Errorf("expected client_error=true for 4xx status, got %v", ctx["client_error"])
				}

				if serverError, ok := ctx["server_error"].(bool); !ok || serverError {
					t.Errorf("expected server_error=false for 4xx status, got %v", ctx["server_error"])
				}
			},
		},
		{
			name: "server error response",
			response: GraphQLResponseData{
				StatusCode: 500,
				Body:       []byte(`Internal Server Error`),
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
			},
			validate: func(t *testing.T, ctx map[string]any) {
				if status, ok := ctx["status"].(int); !ok || status != 500 {
					t.Errorf("expected status 500, got %v", ctx["status"])
				}

				if success, ok := ctx["success"].(bool); !ok || success {
					t.Errorf("expected success=false for 5xx status, got %v", ctx["success"])
				}

				if serverError, ok := ctx["server_error"].(bool); !ok || !serverError {
					t.Errorf("expected server_error=true for 5xx status, got %v", ctx["server_error"])
				}

				if isJSON, ok := ctx["is_json"].(bool); !ok || isJSON {
					t.Errorf("expected is_json=false for text/plain, got %v", ctx["is_json"])
				}

				// Body should be string since JSON parsing fails
				if bodyStr, ok := ctx["body_string"].(string); !ok || bodyStr != "Internal Server Error" {
					t.Errorf("expected body_string='Internal Server Error', got %v", ctx["body_string"])
				}
			},
		},
		{
			name: "empty response",
			response: GraphQLResponseData{
				StatusCode: 204,
				Body:       []byte{},
				Headers:    map[string]string{},
			},
			validate: func(t *testing.T, ctx map[string]any) {
				if hasBody, ok := ctx["has_body"].(bool); !ok || hasBody {
					t.Errorf("expected has_body=false for empty body, got %v", ctx["has_body"])
				}

				if success, ok := ctx["success"].(bool); !ok || !success {
					t.Errorf("expected success=true for 204 status, got %v", ctx["success"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := srv.createAssertionEvalContext(tt.response)
			tt.validate(t, ctx)
		})
	}
}

func TestEvaluateAssertionsParallel(t *testing.T) {
	t.Parallel()

	srv := &GraphQLServiceRPC{}
	ctx := context.Background()

	tests := []struct {
		name          string
		asserts       []mgraphql.GraphQLAssert
		evalContext   map[string]any
		validateCount int
		checkResults  func(t *testing.T, results []AssertionResult)
	}{
		{
			name:          "empty assertions list",
			asserts:       []mgraphql.GraphQLAssert{},
			evalContext:   map[string]any{},
			validateCount: 0,
			checkResults: func(t *testing.T, results []AssertionResult) {
				if len(results) != 0 {
					t.Errorf("expected 0 results for empty assertions, got %d", len(results))
				}
			},
		},
		{
			name: "single successful assertion",
			asserts: []mgraphql.GraphQLAssert{
				{
					ID:      idwrap.NewNow(),
					Value:   "status == 200",
					Enabled: true,
				},
			},
			evalContext: map[string]any{
				"status": 200,
			},
			validateCount: 1,
			checkResults: func(t *testing.T, results []AssertionResult) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				if results[0].Error != nil {
					t.Errorf("expected no error, got %v", results[0].Error)
				}
				if !results[0].Success {
					t.Errorf("expected success=true for status == 200")
				}
			},
		},
		{
			name: "single failing assertion",
			asserts: []mgraphql.GraphQLAssert{
				{
					ID:      idwrap.NewNow(),
					Value:   "status == 404",
					Enabled: true,
				},
			},
			evalContext: map[string]any{
				"status": 200,
			},
			validateCount: 1,
			checkResults: func(t *testing.T, results []AssertionResult) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				if results[0].Error != nil {
					t.Errorf("expected no error, got %v", results[0].Error)
				}
				if results[0].Success {
					t.Errorf("expected success=false for status == 404 when status is 200")
				}
			},
		},
		{
			name: "multiple assertions",
			asserts: []mgraphql.GraphQLAssert{
				{
					ID:      idwrap.NewNow(),
					Value:   "status == 200",
					Enabled: true,
				},
				{
					ID:      idwrap.NewNow(),
					Value:   "success == true",
					Enabled: true,
				},
				{
					ID:      idwrap.NewNow(),
					Value:   "is_json == true",
					Enabled: true,
				},
			},
			evalContext: map[string]any{
				"status":  200,
				"success": true,
				"is_json": true,
			},
			validateCount: 3,
			checkResults: func(t *testing.T, results []AssertionResult) {
				if len(results) != 3 {
					t.Fatalf("expected 3 results, got %d", len(results))
				}
				for i, result := range results {
					if result.Error != nil {
						t.Errorf("result[%d]: expected no error, got %v", i, result.Error)
					}
					if !result.Success {
						t.Errorf("result[%d]: expected success=true, expression=%s", i, result.Expression)
					}
				}
			},
		},
		{
			name: "invalid expression",
			asserts: []mgraphql.GraphQLAssert{
				{
					ID:      idwrap.NewNow(),
					Value:   "invalid syntax %%%",
					Enabled: true,
				},
			},
			evalContext: map[string]any{
				"status": 200,
			},
			validateCount: 1,
			checkResults: func(t *testing.T, results []AssertionResult) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				// Should have an error for invalid syntax
				if results[0].Error == nil {
					t.Errorf("expected error for invalid expression syntax")
				}
				if results[0].Success {
					t.Errorf("expected success=false for invalid expression")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := srv.evaluateAssertionsParallel(ctx, tt.asserts, tt.evalContext)

			if len(results) != tt.validateCount {
				t.Fatalf("expected %d results, got %d", tt.validateCount, len(results))
			}

			tt.checkResults(t, results)

			// Verify all results have timestamps
			for i, result := range results {
				if result.EvaluatedAt.IsZero() {
					t.Errorf("result[%d]: expected non-zero EvaluatedAt timestamp", i)
				}
			}
		})
	}
}

func TestCreateJSONPathHelpers(t *testing.T) {
	t.Parallel()

	srv := &GraphQLServiceRPC{}

	tests := []struct {
		name    string
		bodyMap map[string]any
		checks  func(t *testing.T, helpers map[string]any)
	}{
		{
			name:    "nil body map",
			bodyMap: nil,
			checks: func(t *testing.T, helpers map[string]any) {
				if helpers == nil {
					t.Errorf("expected non-nil helpers map")
				}
				if len(helpers) != 0 {
					t.Errorf("expected empty helpers for nil body, got %d", len(helpers))
				}
			},
		},
		{
			name: "simple nested object",
			bodyMap: map[string]any{
				"data": map[string]any{
					"user": map[string]any{
						"name": "Bob",
						"age":  30,
					},
				},
			},
			checks: func(t *testing.T, helpers map[string]any) {
				// Test path function
				if pathFn, ok := helpers["path"].(func(string) any); ok {
					// Test valid path
					if result := pathFn("data.user.name"); result != "Bob" {
						t.Errorf("path('data.user.name') expected 'Bob', got %v", result)
					}

					// Test nested path
					if result := pathFn("data.user.age"); result != 30 {
						t.Errorf("path('data.user.age') expected 30, got %v", result)
					}

					// Test invalid path
					if result := pathFn("data.missing"); result != nil {
						t.Errorf("path('data.missing') expected nil, got %v", result)
					}
				} else {
					t.Errorf("expected path function to exist")
				}

				// Test has function
				if hasFn, ok := helpers["has"].(func(string) bool); ok {
					if !hasFn("data.user.name") {
						t.Errorf("has('data.user.name') should return true")
					}
					if hasFn("data.missing") {
						t.Errorf("has('data.missing') should return false")
					}
				} else {
					t.Errorf("expected has function to exist")
				}

				// Test string function
				if strFn, ok := helpers["string"].(func(string) string); ok {
					if result := strFn("data.user.name"); result != "Bob" {
						t.Errorf("string('data.user.name') expected 'Bob', got %v", result)
					}
					// Non-string value should be converted
					if result := strFn("data.user.age"); result != "30" {
						t.Errorf("string('data.user.age') expected '30', got %v", result)
					}
				} else {
					t.Errorf("expected string function to exist")
				}

				// Test number function
				if numFn, ok := helpers["number"].(func(string) float64); ok {
					if result := numFn("data.user.age"); result != 30.0 {
						t.Errorf("number('data.user.age') expected 30.0, got %v", result)
					}
					// Missing path should return 0
					if result := numFn("data.missing"); result != 0 {
						t.Errorf("number('data.missing') expected 0, got %v", result)
					}
				} else {
					t.Errorf("expected number function to exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			helpers := srv.createJSONPathHelpers(tt.bodyMap)
			tt.checks(t, helpers)
		})
	}
}
