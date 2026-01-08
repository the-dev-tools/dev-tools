package httpclient_test

import (
	"encoding/json"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"net/http"
	"reflect"
	"testing"
)

func TestConvertResponseToVar(t *testing.T) {
	tests := []struct {
		name     string
		input    httpclient.Response
		expected httpclient.ResponseVar
	}{
		{
			name: "Valid JSON body",
			input: httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"key": "value", "number": 123}`),
				Headers: []httpclient.Header{
					{HeaderKey: "Content-Type", Value: "application/json"},
					{HeaderKey: "X-Request-Id", Value: "abc-123"},
				},
			},
			expected: httpclient.ResponseVar{
				StatusCode: http.StatusOK,
				Body: map[string]any{
					"key":    "value",
					"number": json.Number("123"), // Use json.Number for comparison
				},
				Headers: map[string]string{
					"Content-Type": "application/json",
					"X-Request-Id": "abc-123",
				},
				Duration: 0, // Duration is not set by this function
			},
		},
		{
			name: "Non-JSON body",
			input: httpclient.Response{
				StatusCode: http.StatusNotFound,
				Body:       []byte("This is plain text"),
				Headers: []httpclient.Header{
					{HeaderKey: "Content-Type", Value: "text/plain"},
				},
			},
			expected: httpclient.ResponseVar{
				StatusCode: http.StatusNotFound,
				Body:       "This is plain text",
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Duration: 0,
			},
		},
		{
			name: "Empty body and no headers",
			input: httpclient.Response{
				StatusCode: http.StatusNoContent,
				Body:       []byte{},
				Headers:    []httpclient.Header{},
			},
			expected: httpclient.ResponseVar{
				StatusCode: http.StatusNoContent,
				Body:       "",
				Headers:    map[string]string{},
				Duration:   0,
			},
		},
		// Add more test cases as needed, e.g., malformed JSON
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := httpclient.ConvertResponseToVar(tt.input)

			// Special handling for JSON body comparison due to potential type differences (e.g., float64 vs json.Number)
			if expectedBodyMap, ok := tt.expected.Body.(map[string]any); ok {
				if actualBodyMap, ok := actual.Body.(map[string]any); ok {
					// Marshal both to JSON strings for a robust comparison
					expectedJSON, _ := json.Marshal(expectedBodyMap)
					actualJSON, _ := json.Marshal(actualBodyMap)
					if string(expectedJSON) != string(actualJSON) {
						t.Errorf("ConvertResponseToVar() Body = %v, want %v", string(actualJSON), string(expectedJSON))
					}
					// Avoid comparing Body again in DeepEqual
					tt.expected.Body = nil
					actual.Body = nil
				} else {
					t.Errorf("ConvertResponseToVar() Body type mismatch: expected map[string]any, got %T", actual.Body)
				}
			}

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("ConvertResponseToVar() = %v, want %v", actual, tt.expected)
			}
		})
	}
}
