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

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"

	"github.com/stretchr/testify/require"
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
				"auth.token": menv.Variable{VarKey: "auth.token", Value: "abc123"},
			},
			want:    "abc123",
			wantErr: false,
		},
		{
			name:        "bearer token",
			headerValue: "Bearer {{ auth.token }}",
			varMap: varsystem.VarMap{
				"auth.token": menv.Variable{VarKey: "auth.token", Value: "abc123"},
			},
			want:    "Bearer abc123",
			wantErr: false,
		},
		{
			name:        "multiple variables",
			headerValue: "{{ prefix }}/{{ version }}/{{ path }}",
			varMap: varsystem.VarMap{
				"prefix":  menv.Variable{VarKey: "prefix", Value: "api"},
				"version": menv.Variable{VarKey: "version", Value: "v1"},
				"path":    menv.Variable{VarKey: "path", Value: "users"},
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
				tt.varMap["#file:test.txt"] = menv.Variable{
					VarKey: "#file:test.txt",
					Value:  "#file:" + tmpFile.Name(),
				}
			}

			endpoint := mhttp.HTTP{
				Method:   "GET",
				Url:      "http://example.com",
				BodyKind: mhttp.HttpBodyKindRaw,
			}

			example := mhttp.HTTP{
				BodyKind: mhttp.HttpBodyKindRaw,
			}

			headers := []mhttp.HTTPHeader{
				{
					Key:     "Authorization",
					Value:   tt.headerValue,
					Enabled: true,
				},
			}

			req, err := PrepareRequest(endpoint, example, nil, headers, mhttp.HTTPBodyRaw{}, nil, nil, tt.varMap)
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
	require.NoError(t, err, "failed to create temp file 1")
	defer func() {
		if err := os.Remove(file1.Name()); err != nil {
			t.Errorf("failed to remove temp file 1: %v", err)
		}
	}()
	_, err = file1.WriteString("content of file 1")
	require.NoError(t, err, "failed to write to file 1")
	if err := file1.Close(); err != nil {
		t.Errorf("failed to close file 1: %v", err)
	}

	file2, err := os.CreateTemp("", "testfile2-*.txt")
	require.NoError(t, err, "failed to create temp file 2")
	defer func() {
		if err := os.Remove(file2.Name()); err != nil {
			t.Errorf("failed to remove temp file 2: %v", err)
		}
	}()
	_, err = file2.WriteString("content of file 2")
	require.NoError(t, err, "failed to write to file 2")
	if err := file2.Close(); err != nil {
		t.Errorf("failed to close file 2: %v", err)
	}

	// Prepare the request components using mhttp models
	endpoint := mhttp.HTTP{
		Method:   "POST",
		Url:      "http://example.com/upload",
		BodyKind: mhttp.HttpBodyKindFormData,
	}
	example := mhttp.HTTP{
		BodyKind: mhttp.HttpBodyKindFormData,
	}
	formBody := []mhttp.HTTPBodyForm{
		{
			Key:     "photos",
			Value:   fmt.Sprintf("{{#file:%s}},{{#file:%s}}", file1.Name(), file2.Name()),
			Enabled: true,
		},
	}
	varMap := varsystem.NewVarMap(nil) // No variables needed for direct file paths

	// Call PrepareRequest
	req, err := PrepareRequest(endpoint, example, nil, nil, mhttp.HTTPBodyRaw{}, formBody, nil, varMap)
	require.NoError(t, err, "PrepareRequest failed")

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
	require.NoError(t, err, "failed to parse Content-Type")
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

func TestMergeExamplesWithNilDeltaParentID(t *testing.T) {
	// This test verifies that MergeExamples can handle legacy delta examples
	// that have nil DeltaParentID without crashing

	baseExampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	baseExample := mhttp.HTTP{
		ID:   baseExampleID,
		Name: "Base Example",
	}

	deltaExample := mhttp.HTTP{
		ID:   deltaExampleID,
		Name: "Delta Example",
	}

	// Create base queries and headers
	baseQueryID := idwrap.NewNow()
	baseHeaderID := idwrap.NewNow()

	baseQueries := []mhttp.HTTPSearchParam{
		{
			ID:     baseQueryID,
			HttpID: baseExampleID,
			Key:    "page",
			Value:  "1",
		},
	}

	baseHeaders := []mhttp.HTTPHeader{
		{
			ID:     baseHeaderID,
			HttpID: baseExampleID,
			Key:    "Authorization",
			Value:  "Bearer token123",
		},
	}

	baseAsserts := []mhttp.HTTPAssert{
		{
			ID:      idwrap.NewNow(),
			HttpID:  baseExampleID,
			Value:   "response.status == 200",
			Enabled: true,
		},
	}

	// Create delta queries and headers with nil ParentSearchParamID/ParentHeaderID (legacy format)
	deltaQueries := []mhttp.HTTPSearchParam{
		{
			ID:                      idwrap.NewNow(),
			HttpID:                  deltaExampleID,
			Key:                     "page",
			Value:                   "2", // Changed value
			ParentHttpSearchParamID: nil, // This would cause a panic in the old code
		},
	}

	deltaHeaders := []mhttp.HTTPHeader{
		{
			ID:                 idwrap.NewNow(),
			HttpID:             deltaExampleID,
			Key:                "Authorization",
			Value:              "Bearer {{ token }}",
			ParentHttpHeaderID: nil, // This would cause a panic in the old code
		},
	}

	deltaAsserts := []mhttp.HTTPAssert{
		{
			ID:      idwrap.NewNow(),
			HttpID:  deltaExampleID,
			Value:   "response.status == 201",
			Enabled: true,
		},
	}

	// Create empty bodies for testing
	baseRawBody := mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  baseExampleID,
		RawData: []byte(`{"test": "base"}`),
	}

	deltaRawBody := mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  deltaExampleID,
		RawData: []byte(`{"test": "delta"}`),
	}

	input := MergeExamplesInput{
		Base:  baseExample,
		Delta: deltaExample,

		BaseQueries:  baseQueries,
		DeltaQueries: deltaQueries,

		BaseHeaders:  baseHeaders,
		DeltaHeaders: deltaHeaders,

		BaseRawBody:  baseRawBody,
		DeltaRawBody: deltaRawBody,

		BaseFormBody:        []mhttp.HTTPBodyForm{},
		DeltaFormBody:       []mhttp.HTTPBodyForm{},
		BaseUrlEncodedBody:  []mhttp.HTTPBodyUrlencoded{},
		DeltaUrlEncodedBody: []mhttp.HTTPBodyUrlencoded{},
		BaseAsserts:         baseAsserts,
		DeltaAsserts:        deltaAsserts,
	}

	// This should not panic even with nil ParentSearchParamID/ParentHeaderID
	output := MergeExamples(input)

	// Verify the merge worked
	if output.Merged.ID != baseExample.ID {
		t.Errorf("Expected merged ID to be %v, got %v", baseExample.ID, output.Merged.ID)
	}

	if len(output.MergeQueries) == 0 {
		t.Error("Expected at least one merged query")
	}

	if len(output.MergeHeaders) == 0 {
		t.Error("Expected at least one merged header")
	}

	// Verify that delta values override base values (key-based matching for legacy)
	foundDeltaQuery := false
	for _, query := range output.MergeQueries {
		if query.Key == "page" && query.Value == "2" {
			foundDeltaQuery = true
			break
		}
	}
	if !foundDeltaQuery {
		t.Error("Expected delta query value to override base query value")
	}

	foundDeltaHeader := false
	for _, header := range output.MergeHeaders {
		if header.Key == "Authorization" && header.Value == "Bearer {{ token }}" {
			foundDeltaHeader = true
			break
		}
	}
	if !foundDeltaHeader {
		t.Error("Expected delta header value to override base header value")
	}

	// Verify that we have exactly 1 query and 1 header (delta should override base)
	if len(output.MergeQueries) != 1 {
		t.Errorf("Expected exactly 1 merged query, got %d", len(output.MergeQueries))
	}

	if len(output.MergeHeaders) != 1 {
		t.Errorf("Expected exactly 1 merged header, got %d", len(output.MergeHeaders))
	}

	if len(output.MergeAsserts) != 2 {
		t.Fatalf("Expected merged asserts to include base and delta entries, got %d", len(output.MergeAsserts))
	}

	if output.MergeAsserts[1].Value != "response.status == 201" {
		t.Errorf("Expected delta assertion expression to be preserved, got %s", output.MergeAsserts[1].Value)
	}

	t.Logf("✅ MergeExamples handled nil ParentSearchParamID/ParentHeaderID successfully")
	t.Logf("📊 Merged %d queries and %d headers", len(output.MergeQueries), len(output.MergeHeaders))
}

func TestMergeExamplesWithProperDeltaParentID(t *testing.T) {
	// This test verifies that MergeExamples works correctly with proper ParentSearchParamID/ParentHeaderID
	// (the new format created by HAR conversion)

	baseExampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	baseExample := mhttp.HTTP{
		ID:   baseExampleID,
		Name: "Base Example",
	}

	deltaExample := mhttp.HTTP{
		ID:   deltaExampleID,
		Name: "Delta Example",
	}

	// Create base queries and headers
	baseQueryID := idwrap.NewNow()
	baseHeaderID := idwrap.NewNow()

	baseQueries := []mhttp.HTTPSearchParam{
		{
			ID:     baseQueryID,
			HttpID: baseExampleID,
			Key:    "page",
			Value:  "1",
		},
	}

	baseHeaders := []mhttp.HTTPHeader{
		{
			ID:     baseHeaderID,
			HttpID: baseExampleID,
			Key:    "Authorization",
			Value:  "Bearer token123",
		},
	}

	baseAssertIDWithParent := idwrap.NewNow()
	baseAssertsWithParent := []mhttp.HTTPAssert{
		{
			ID:      baseAssertIDWithParent,
			HttpID:  baseExampleID,
			Value:   "response.status == 200",
			Enabled: true,
		},
	}

	// Create delta queries and headers with proper ParentSearchParamID/ParentHeaderID (new format)
	deltaQueries := []mhttp.HTTPSearchParam{
		{
			ID:                      idwrap.NewNow(),
			HttpID:                  deltaExampleID,
			Key:                     "page",
			Value:                   "{{ request-1.response.page }}", // Templated value
			ParentHttpSearchParamID: &baseQueryID,                    // Proper reference to base query
		},
	}

	deltaHeaders := []mhttp.HTTPHeader{
		{
			ID:                 idwrap.NewNow(),
			HttpID:             deltaExampleID,
			Key:                "Authorization",
			Value:              "Bearer {{ request-1.response.body.token }}",
			ParentHttpHeaderID: &baseHeaderID, // Proper reference to base header
		},
	}

	deltaAsserts := []mhttp.HTTPAssert{
		{
			ID:                 idwrap.NewNow(),
			HttpID:             deltaExampleID,
			ParentHttpAssertID: &baseAssertIDWithParent,
			Value:              "response.status == 201",
			Enabled:            true,
		},
	}

	// Create empty bodies for testing
	baseRawBody := mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  baseExampleID,
		RawData: []byte(`{"test": "base"}`),
	}

	deltaRawBody := mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  deltaExampleID,
		RawData: []byte(`{"test": "delta"}`),
	}

	input := MergeExamplesInput{
		Base:  baseExample,
		Delta: deltaExample,

		BaseQueries:  baseQueries,
		DeltaQueries: deltaQueries,

		BaseHeaders:  baseHeaders,
		DeltaHeaders: deltaHeaders,

		BaseRawBody:  baseRawBody,
		DeltaRawBody: deltaRawBody,

		BaseFormBody:        []mhttp.HTTPBodyForm{},
		DeltaFormBody:       []mhttp.HTTPBodyForm{},
		BaseUrlEncodedBody:  []mhttp.HTTPBodyUrlencoded{},
		DeltaUrlEncodedBody: []mhttp.HTTPBodyUrlencoded{},
		BaseAsserts:         baseAssertsWithParent,
		DeltaAsserts:        deltaAsserts,
	}

	// This should work correctly with proper parent references
	output := MergeExamples(input)

	// Verify the merge worked
	if output.Merged.ID != baseExample.ID {
		t.Errorf("Expected merged ID to be %v, got %v", baseExample.ID, output.Merged.ID)
	}

	if len(output.MergeQueries) != 1 {
		t.Errorf("Expected exactly 1 merged query, got %d", len(output.MergeQueries))
	}

	if len(output.MergeHeaders) != 1 {
		t.Errorf("Expected exactly 1 merged header, got %d", len(output.MergeHeaders))
	}

	if len(output.MergeAsserts) != 1 {
		t.Fatalf("Expected merged asserts to reuse base slot and stay at 1 entry, got %d", len(output.MergeAsserts))
	}

	if output.MergeAsserts[0].Value != "response.status == 201" {
		t.Errorf("Expected merged assertion to reflect delta expression, got %s", output.MergeAsserts[0].Value)
	}

	// Verify that delta values replaced base values correctly
	mergedQuery := output.MergeQueries[0]
	if mergedQuery.Key != "page" || mergedQuery.Value != "{{ request-1.response.page }}" {
		t.Errorf("Expected delta query to replace base query, got Key: %s, Value: %s", mergedQuery.Key, mergedQuery.Value)
	}

	mergedHeader := output.MergeHeaders[0]
	if mergedHeader.Key != "Authorization" || mergedHeader.Value != "Bearer {{ request-1.response.body.token }}" {
		t.Errorf("Expected delta header to replace base header, got Key: %s, Value: %s", mergedHeader.Key, mergedHeader.Value)
	}

	t.Logf("✅ MergeExamples handled proper ParentSearchParamID/ParentHeaderID successfully")
	t.Logf("📊 Merged %d queries and %d headers with proper parent relationships", len(output.MergeQueries), len(output.MergeHeaders))
}
