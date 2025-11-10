package tcurlv2

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertCurl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("ConvertCurl() returned nil result for successful conversion")
					return
				}

				// Basic validation
				if result.HTTP.ID.Compare(idwrap.IDWrap{}) == 0 {
					t.Error("ConvertCurl() HTTP ID should not be empty")
				}
				if result.HTTP.WorkspaceID.Compare(workspace.ID) != 0 {
					t.Error("ConvertCurl() workspace ID mismatch")
				}
				if result.HTTP.Method == "" {
					t.Error("ConvertCurl() HTTP method should not be empty")
				}
				if result.HTTP.Url == "" {
					t.Error("ConvertCurl() HTTP URL should not be empty")
				}
			}
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
			if got != tt.want {
				t.Errorf("ExtractURLForTesting() = %v, want %v", got, tt.want)
			}
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
	if err != nil {
		t.Fatalf("ConvertCurl() error = %v", err)
	}

	// Build curl back
	built, err := BuildCurl(resolved)
	if err != nil {
		t.Fatalf("BuildCurl() error = %v", err)
	}

	if built == "" {
		t.Error("BuildCurl() returned empty string")
	}

	// Basic checks
	if !contains(built, "curl") {
		t.Error("BuildCurl() should contain 'curl'")
	}
	if !contains(built, "https://api.example.com/users") {
		t.Error("BuildCurl() should contain the URL")
	}
	if !contains(built, "POST") {
		t.Error("BuildCurl() should contain the POST method")
	}
	if !contains(built, "Content-Type: application/json") {
		t.Error("BuildCurl() should contain the Content-Type header")
	}
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
