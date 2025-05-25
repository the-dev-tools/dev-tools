package request

import (
	"os"
	"testing"

	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/sort/sortenabled"
	"the-dev-tools/server/pkg/varsystem"
)

func TestPrepareRequest_HeaderVariableReplacement(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		varMap      varsystem.VarMap
		want        string
		wantErr     bool
	}{
		{
			name:        "simple variable",
			headerValue: "{{ auth.token }}",
			varMap: varsystem.VarMap{
				"auth.token": mvar.Var{VarKey: "auth.token", Value: "abc123"},
			},
			want:    "abc123",
			wantErr: false,
		},
		{
			name:        "bearer token",
			headerValue: "Bearer {{ auth.token }}",
			varMap: varsystem.VarMap{
				"auth.token": mvar.Var{VarKey: "auth.token", Value: "abc123"},
			},
			want:    "Bearer abc123",
			wantErr: false,
		},
		{
			name:        "multiple variables",
			headerValue: "{{ prefix }}/{{ version }}/{{ path }}",
			varMap: varsystem.VarMap{
				"prefix":  mvar.Var{VarKey: "prefix", Value: "api"},
				"version": mvar.Var{VarKey: "version", Value: "v1"},
				"path":    mvar.Var{VarKey: "path", Value: "users"},
			},
			want:    "api/v1/users",
			wantErr: false,
		},
		{
			name:        "variable not found",
			headerValue: "Bearer {{ auth.token }}",
			varMap:      varsystem.VarMap{},
			want:        "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file for file reference test
			if tt.name == "file reference" {
				tmpFile, err := os.CreateTemp("", "test-*.txt")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpFile.Name())

				if _, err := tmpFile.WriteString("file content"); err != nil {
					t.Fatal(err)
				}
				tmpFile.Close()

				// Update the varMap with the actual file path
				tt.varMap["#file:test.txt"] = mvar.Var{
					VarKey: "#file:test.txt",
					Value:  "#file:" + tmpFile.Name(),
				}
			}

			endpoint := mitemapi.ItemApi{
				Method: "GET",
				Url:    "http://example.com",
			}

			example := mitemapiexample.ItemApiExample{
				BodyType: mitemapiexample.BodyTypeRaw,
			}

			headers := []mexampleheader.Header{
				{
					HeaderKey: "Authorization",
					Value:     tt.headerValue,
					Enable:    true,
				},
			}

			sortenabled.GetAllWithState(&headers, true)

			req, err := PrepareRequest(endpoint, example, nil, headers, mbodyraw.ExampleBodyRaw{}, nil, nil, tt.varMap)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Find the Authorization header
			var found bool
			for _, h := range req.Headers {
				if h.HeaderKey == "Authorization" {
					found = true
					if h.Value != tt.want {
						t.Errorf("got %q, want %q", h.Value, tt.want)
					}
					break
				}
			}
			if !found && !tt.wantErr {
				t.Error("Authorization header not found in request")
			}
		})
	}
}
