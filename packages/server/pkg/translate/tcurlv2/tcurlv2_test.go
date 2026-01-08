package tcurlv2

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"

	"github.com/stretchr/testify/require"
)

func TestConvertCurl(t *testing.T) {
	workspace := mworkspace.Workspace{
		ID:        idwrap.NewNow(),
		Name:      "Test Workspace",
		ActiveEnv: idwrap.NewNow(),
		GlobalEnv: idwrap.NewNow(),
	}

	tests := []struct {
		name    string
		curl    string
		wantErr bool
	}{
		{
			name:    "simple GET request",
			curl:    "curl https://api.example.com/users",
			wantErr: false,
		},
		{
			name:    "POST request with headers",
			curl:    "curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{\"name\":\"John\"}'",
			wantErr: false,
		},
		{
			name:    "form data",
			curl:    "curl -X POST https://api.example.com/upload -F 'file=@test.txt' -F 'description=test'",
			wantErr: false,
		},
		{
			name:    "URL encoded data",
			curl:    "curl -X POST https://api.example.com/search --data-urlencode 'query=golang' --data-urlencode 'limit=10'",
			wantErr: false,
		},
		{
			name:    "cookies",
			curl:    "curl -b 'session=abc123; user=john' https://api.example.com/profile",
			wantErr: false,
		},
		{
			name:    "invalid command",
			curl:    "not a curl command",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ConvertCurlOptions{
				WorkspaceID: workspace.ID,
			}

			result, err := ConvertCurl(tt.curl, opts)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result, "ConvertCurl() returned nil result for successful conversion")

			// Basic validation
			require.NotEqual(t, idwrap.IDWrap{}, result.HTTP.ID, "ConvertCurl() HTTP ID should not be empty")
			require.Equal(t, workspace.ID, result.HTTP.WorkspaceID, "ConvertCurl() workspace ID mismatch")
			require.NotEmpty(t, result.HTTP.Method, "ConvertCurl() HTTP method should not be empty")
			require.NotEmpty(t, result.HTTP.Url, "ConvertCurl() HTTP URL should not be empty")
		})
	}
}

func TestExtractURLForTesting(t *testing.T) {
	tests := []struct {
		name string
		curl string
		want string
	}{
		{
			name: "simple URL",
			curl: "curl https://example.com",
			want: "https://example.com",
		},
		{
			name: "URL with path",
			curl: "curl https://api.example.com/v1/users",
			want: "https://api.example.com/v1/users",
		},
		{
			name: "URL with options",
			curl: "curl -X GET -H 'Accept: application/json' https://api.example.com/data",
			want: "https://api.example.com/data",
		},
		{
			name: "no URL",
			curl: "curl -X POST",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractURLForTesting(tt.curl)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildCurl(t *testing.T) {
	workspace := mworkspace.Workspace{
		ID:        idwrap.NewNow(),
		Name:      "Test Workspace",
		ActiveEnv: idwrap.NewNow(),
		GlobalEnv: idwrap.NewNow(),
	}

	// Create a simple HTTP request
	curl := "curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{\"name\":\"John\"}'"
	opts := ConvertCurlOptions{
		WorkspaceID: workspace.ID,
	}

	resolved, err := ConvertCurl(curl, opts)
	require.NoError(t, err)

	// Build curl back
	built, err := BuildCurl(resolved)
	require.NoError(t, err)
	require.NotEmpty(t, built, "BuildCurl() returned empty string")

	// Basic checks
	require.Contains(t, built, "curl", "BuildCurl() should contain 'curl'")
	require.Contains(t, built, "https://api.example.com/users", "BuildCurl() should contain the URL")
	require.Contains(t, built, "POST", "BuildCurl() should contain the POST method")
	require.Contains(t, built, "Content-Type: application/json", "BuildCurl() should contain the Content-Type header")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
