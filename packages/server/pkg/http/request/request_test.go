package request

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"the-dev-tools/server/pkg/model/mbodyform"
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
				defer func() {
					if err := os.Remove(tmpFile.Name()); err != nil {
						t.Errorf("failed to remove temporary file: %v", err)
					}
				}()

				if _, err := tmpFile.WriteString("file content"); err != nil {
					t.Fatal(err)
				}
				if err := tmpFile.Close(); err != nil {
					t.Errorf("failed to close temporary file: %v", err)
				}

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

func TestPrepareRequest_MultiFileUpload(t *testing.T) {
	// Create temporary files
	file1, err := os.CreateTemp("", "testfile1-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file 1: %v", err)
	}
	defer func() {
		if err := os.Remove(file1.Name()); err != nil {
			t.Errorf("failed to remove temp file 1: %v", err)
		}
	}()
	_, err = file1.WriteString("content of file 1")
	if err != nil {
		t.Fatalf("failed to write to file 1: %v", err)
	}
	if err := file1.Close(); err != nil {
		t.Errorf("failed to close file 1: %v", err)
	}

	file2, err := os.CreateTemp("", "testfile2-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file 2: %v", err)
	}
	defer func() {
		if err := os.Remove(file2.Name()); err != nil {
			t.Errorf("failed to remove temp file 2: %v", err)
		}
	}()
	_, err = file2.WriteString("content of file 2")
	if err != nil {
		t.Fatalf("failed to write to file 2: %v", err)
	}
	if err := file2.Close(); err != nil {
		t.Errorf("failed to close file 2: %v", err)
	}

	// Prepare the request components
	endpoint := mitemapi.ItemApi{
		Method: "POST",
		Url:    "http://example.com/upload",
	}
	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeForm,
	}
	formBody := []mbodyform.BodyForm{
		{
			BodyKey: "photos",
			Value:   fmt.Sprintf("{{#file:%s}},{{#file:%s}}", file1.Name(), file2.Name()),
			Enable:  true,
		},
	}
	varMap := varsystem.NewVarMap(nil) // No variables needed for direct file paths

	// Call PrepareRequest
	req, err := PrepareRequest(endpoint, example, nil, nil, mbodyraw.ExampleBodyRaw{}, formBody, nil, varMap)
	if err != nil {
		t.Fatalf("PrepareRequest failed: %v", err)
	}

	// Verify the request body
	if req.Body == nil {
		t.Fatal("request body is nil")
	}

	// Determine the boundary from the Content-Type header
	contentType := ""
	for _, h := range req.Headers {
		if h.HeaderKey == "Content-Type" {
			contentType = h.Value
			break
		}
	}
	if contentType == "" {
		t.Fatal("Content-Type header not found")
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse Content-Type: %v", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatal("multipart boundary not found")
	}

	reader := multipart.NewReader(bytes.NewReader(req.Body), boundary)

	expectedFiles := map[string]string{
		filepath.Base(file1.Name()): "content of file 1",
		filepath.Base(file2.Name()): "content of file 2",
	}

	foundFiles := make(map[string]bool)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read multipart part: %v", err)
		}

		if part.FormName() == "photos" {
			fileName := part.FileName()
			if fileName == "" {
				t.Errorf("expected filename for part, got empty")
				continue
			}

			contentBytes, err := io.ReadAll(part)
			if err != nil {
				t.Errorf("failed to read part content: %v", err)
				continue
			}
			actualContent := string(contentBytes)

			expectedContent, ok := expectedFiles[fileName]
			if !ok {
				t.Errorf("unexpected file uploaded: %s", fileName)
				continue
			}

			if actualContent != expectedContent {
				t.Errorf("file %s: got content %q, want %q", fileName, actualContent, expectedContent)
			}
			foundFiles[fileName] = true
		}
	}

	// Check if all expected files were found
	for fileName := range expectedFiles {
		if !foundFiles[fileName] {
			t.Errorf("expected file %s not found in multipart body", fileName)
		}
	}
}
