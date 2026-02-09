package secretresolver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSecretRef(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedPath string
		expectedFrag string
	}{
		{
			name:         "path with fragment",
			input:        "projects/p/secrets/s/versions/latest#client_secret",
			expectedPath: "projects/p/secrets/s/versions/latest",
			expectedFrag: "client_secret",
		},
		{
			name:         "path without fragment",
			input:        "projects/p/secrets/s/versions/latest",
			expectedPath: "projects/p/secrets/s/versions/latest",
			expectedFrag: "",
		},
		{
			name:         "empty string",
			input:        "",
			expectedPath: "",
			expectedFrag: "",
		},
		{
			name:         "fragment only",
			input:        "#key",
			expectedPath: "",
			expectedFrag: "key",
		},
		{
			name:         "multiple hash signs uses last",
			input:        "path#middle#last",
			expectedPath: "path#middle",
			expectedFrag: "last",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, fragment := ParseSecretRef(tt.input)
			require.Equal(t, tt.expectedPath, path)
			require.Equal(t, tt.expectedFrag, fragment)
		})
	}
}
