package topenapiv2

import (
	"encoding/json"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func TestConvertOperation_DeterministicStatusCode(t *testing.T) {
	// Bug 3 regression test: when multiple 2xx response codes exist,
	// the lowest one should always be selected (deterministic).
	op := operation{
		Summary: "Multi-status endpoint",
		Responses: map[string]response{
			"201": {Description: "Created"},
			"200": {Description: "OK"},
			"204": {Description: "No Content"},
		},
	}

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}

	// Run multiple times to catch non-determinism
	for i := 0; i < 50; i++ {
		_, _, _, _, assert := convertOperation("POST", "/test", "https://api.example.com", op, opts)

		if assert == nil {
			t.Fatal("expected an assert to be created from 2xx responses")
		}
		if assert.Value != "response.status == 200" {
			t.Errorf("iteration %d: expected assert for status 200 (lowest 2xx), got %q", i, assert.Value)
		}
	}
}

func TestConvertOperation_SingleStatusCode(t *testing.T) {
	op := operation{
		Summary: "Single status endpoint",
		Responses: map[string]response{
			"201": {Description: "Created"},
		},
	}

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	_, _, _, _, assert := convertOperation("POST", "/test", "https://api.example.com", op, opts)

	if assert == nil {
		t.Fatal("expected an assert to be created")
	}
	if assert.Value != "response.status == 201" {
		t.Errorf("expected assert for status 201, got %q", assert.Value)
	}
}

func TestConvertOperation_No2xxResponse(t *testing.T) {
	op := operation{
		Summary: "Error-only endpoint",
		Responses: map[string]response{
			"400": {Description: "Bad Request"},
			"500": {Description: "Internal Server Error"},
		},
	}

	opts := ConvertOptions{WorkspaceID: idwrap.NewNow()}
	_, _, _, _, assert := convertOperation("GET", "/test", "https://api.example.com", op, opts)

	if assert != nil {
		t.Errorf("expected no assert for non-2xx responses, got %q", assert.Value)
	}
}

func TestParseRequestBody_DeterministicContentType(t *testing.T) {
	// Nit 7 regression test: when application/json is not present,
	// parseRequestBody should pick a deterministic content type (sorted).
	rbMap := map[string]interface{}{
		"content": map[string]interface{}{
			"text/xml": map[string]interface{}{
				"schema": map[string]interface{}{"type": "string"},
			},
			"application/x-www-form-urlencoded": map[string]interface{}{
				"schema": map[string]interface{}{"type": "object"},
			},
			"multipart/form-data": map[string]interface{}{
				"schema": map[string]interface{}{"type": "object"},
			},
		},
	}

	// Run multiple times to catch non-determinism
	for i := 0; i < 50; i++ {
		rb := parseRequestBody(rbMap)
		// Sorted order: application/x-www-form-urlencoded, multipart/form-data, text/xml
		// Since none is application/json, the first in sorted order is used.
		if rb.ContentType != "application/x-www-form-urlencoded" {
			t.Errorf("iteration %d: expected content type 'application/x-www-form-urlencoded' (first in sorted order), got %q", i, rb.ContentType)
		}
	}
}

func TestParseRequestBody_PrefersApplicationJSON(t *testing.T) {
	// When application/json exists, it should always be selected regardless of other types.
	rbMap := map[string]interface{}{
		"content": map[string]interface{}{
			"text/xml": map[string]interface{}{
				"schema": map[string]interface{}{"type": "string"},
			},
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{"type": "string"},
					},
				},
			},
			"multipart/form-data": map[string]interface{}{
				"schema": map[string]interface{}{"type": "object"},
			},
		},
	}

	for i := 0; i < 50; i++ {
		rb := parseRequestBody(rbMap)
		if rb.ContentType != "application/json" {
			t.Errorf("iteration %d: expected content type 'application/json', got %q", i, rb.ContentType)
		}
	}
}

func TestParseRequestBody_JSONSchemaPreserved(t *testing.T) {
	// Verify that when application/json is selected, its schema is used (not from another type).
	rbMap := map[string]interface{}{
		"content": map[string]interface{}{
			"text/xml": map[string]interface{}{
				"schema": map[string]interface{}{"type": "string"},
			},
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{"type": "string", "example": "John"},
					},
				},
				"example": map[string]interface{}{"name": "John"},
			},
		},
	}

	rb := parseRequestBody(rbMap)
	if rb.ContentType != "application/json" {
		t.Fatalf("expected content type 'application/json', got %q", rb.ContentType)
	}
	if rb.Schema == nil {
		t.Fatal("expected schema to be set")
	}
	if rb.Schema.Type != "object" {
		t.Errorf("expected schema type 'object', got %q", rb.Schema.Type)
	}

	var example map[string]interface{}
	if err := json.Unmarshal([]byte(rb.Example), &example); err != nil {
		t.Fatalf("failed to parse example: %v", err)
	}
	if example["name"] != "John" {
		t.Errorf("expected example name 'John', got %v", example["name"])
	}
}

func TestMergeParameters_Deterministic(t *testing.T) {
	pathParams := []parameter{
		{Name: "id", In: "path"},
		{Name: "version", In: "path"},
	}
	opParams := []parameter{
		{Name: "limit", In: "query"},
		{Name: "offset", In: "query"},
		{Name: "id", In: "path"}, // overrides path-level
	}

	var first []parameter
	for i := 0; i < 50; i++ {
		result := mergeParameters(pathParams, opParams)
		if first == nil {
			first = result
			continue
		}
		if len(result) != len(first) {
			t.Fatalf("iteration %d: length mismatch %d vs %d", i, len(result), len(first))
		}
		for j := range result {
			if result[j].Name != first[j].Name || result[j].In != first[j].In {
				t.Errorf("iteration %d: param[%d] mismatch: got %s:%s, want %s:%s",
					i, j, result[j].In, result[j].Name, first[j].In, first[j].Name)
			}
		}
	}
}
