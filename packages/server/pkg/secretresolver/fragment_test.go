package secretresolver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractFragment(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		fragment    string
		expected    string
		expectError bool
	}{
		{
			name:     "empty fragment returns raw value",
			value:    `{"key": "value"}`,
			fragment: "",
			expected: `{"key": "value"}`,
		},
		{
			name:     "extract string field",
			value:    `{"client_id": "abc", "client_secret": "xyz"}`,
			fragment: "client_secret",
			expected: "xyz",
		},
		{
			name:     "extract numeric field",
			value:    `{"port": 8080, "host": "localhost"}`,
			fragment: "port",
			expected: "8080",
		},
		{
			name:     "extract boolean field",
			value:    `{"enabled": true}`,
			fragment: "enabled",
			expected: "true",
		},
		{
			name:     "extract nested object field",
			value:    `{"config": {"nested": "value"}}`,
			fragment: "config",
			expected: `{"nested":"value"}`,
		},
		{
			name:     "extract array field",
			value:    `{"items": [1, 2, 3]}`,
			fragment: "items",
			expected: `[1,2,3]`,
		},
		{
			name:        "missing fragment key",
			value:       `{"key": "value"}`,
			fragment:    "missing",
			expectError: true,
		},
		{
			name:        "non-JSON value with fragment",
			value:       "plain-text-secret",
			fragment:    "key",
			expectError: true,
		},
		{
			name:     "plain text without fragment",
			value:    "plain-text-secret",
			fragment: "",
			expected: "plain-text-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractFragment(tt.value, tt.fragment)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
