package request_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/request"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

// TestPrepareHTTPRequest_JSONNumberSubstitution ensures that
// variables containing large integers or numbers are preserved exactly
// as they appear in the JSON response when used in subsequent requests.
func TestPrepareHTTPRequest_JSONNumberSubstitution(t *testing.T) {
	// 1. Simulate a previous response with a large integer
	// "1234567890123456789" is large enough to lose precision if converted to float64
	jsonResponse := []byte(`{
		"large_id": 1234567890123456789,
		"normal_id": 123,
		"string_id": "abc-123"
	}`)

	// Convert using httpclient logic (which now uses UseNumber)
	resp := httpclient.Response{
		StatusCode: 200,
		Body:       jsonResponse,
	}
	respVar := httpclient.ConvertResponseToVar(resp)

	// Verify that large_id is preserved as json.Number (string)
	bodyMap, ok := respVar.Body.(map[string]any)
	if !ok {
		t.Fatalf("Expected body to be map, got %T", respVar.Body)
	}

	largeID, ok := bodyMap["large_id"]
	require.True(t, ok, "large_id missing from response body")

	// Check type and value
	if _, isNumber := largeID.(json.Number); !isNumber {
		t.Logf("Note: large_id is %T, not json.Number (might be string if UseNumber is active/inactive depending on context, but essentially we want string-preservation)", largeID)
	}

	largeIDStr := fmt.Sprintf("%v", largeID)
	if largeIDStr != "1234567890123456789" {
		t.Errorf("Large integer lost precision/formatting. Expected '1234567890123456789', got '%s'", largeIDStr)
	}

	// 2. Setup Variable Map for Substitution (hierarchical map)
	varMap := map[string]any{
		"prevNode": map[string]any{
			"response": map[string]any{
				"body": bodyMap,
			},
		},
	}

	// 3. Test Substitution into a Request Body
	// Case A: Unquoted substitution of the number (valid JSON for numbers)
	// Template: {"id": {{prevNode.response.body.large_id}}}
	rawBodyUnquoted := `{"id": {{prevNode.response.body.large_id}}}`
	httpReq := mhttp.HTTP{Method: "POST", Url: "http://test.com", BodyKind: mhttp.HttpBodyKindRaw}

	resUnquoted, err := request.PrepareHTTPRequestWithTracking(
		httpReq, nil, nil, &mhttp.HTTPBodyRaw{RawData: []byte(rawBodyUnquoted)}, nil, nil, varMap,
	)
	require.NoError(t, err, "Unquoted substitution failed")

	expectedBodyUnquoted := `{"id": 1234567890123456789}`
	if string(resUnquoted.Request.Body) != expectedBodyUnquoted {
		t.Errorf("Unquoted substitution mismatch.\nWant: %s\nGot:  %s", expectedBodyUnquoted, string(resUnquoted.Request.Body))
	}

	// Case B: Quoted substitution of the number (result is a string "123...")
	// Template: {"id": "{{prevNode.response.body.large_id}}"}
	rawBodyQuoted := `{"id": "{{prevNode.response.body.large_id}}"}`

	resQuoted, err := request.PrepareHTTPRequestWithTracking(
		httpReq, nil, nil, &mhttp.HTTPBodyRaw{RawData: []byte(rawBodyQuoted)}, nil, nil, varMap,
	)
	require.NoError(t, err, "Quoted substitution failed")

	expectedBodyQuoted := `{"id": "1234567890123456789"}`
	if string(resQuoted.Request.Body) != expectedBodyQuoted {
		t.Errorf("Quoted substitution mismatch.\nWant: %s\nGot:  %s", expectedBodyQuoted, string(resQuoted.Request.Body))
	}
}

// TestPrepareHTTPRequest_UUIDSubstitution ensures that string variables (UUIDs)
// are substituted literally. Users must quote them in JSON templates.
func TestPrepareHTTPRequest_UUIDSubstitution(t *testing.T) {
	uuidVal := "8d98027f-8570-45cb-90ae-e8fa1d87dbf5"

	// Hierarchical variable map
	varMap := map[string]any{
		"prevNode": map[string]any{
			"id": uuidVal,
		},
	}

	// Case A: User puts quotes around variable (Correct for JSON strings)
	templateQuoted := `{"id": "{{prevNode.id}}"}`

	resQuoted, err := request.PrepareHTTPRequestWithTracking(
		mhttp.HTTP{Url: "http://test.com", BodyKind: mhttp.HttpBodyKindRaw}, nil, nil, &mhttp.HTTPBodyRaw{RawData: []byte(templateQuoted)}, nil, nil, varMap,
	)
	require.NoError(t, err)

	expectedQuoted := fmt.Sprintf(`{"id": "%s"}`, uuidVal)
	if string(resQuoted.Request.Body) != expectedQuoted {
		t.Errorf("Quoted substitution failed.\nWant: %s\nGot:  %s", expectedQuoted, string(resQuoted.Request.Body))
	}

	// Case B: User forgets quotes (Invalid JSON result, but correct substitution behavior)
	templateUnquoted := `{"id": {{prevNode.id}}}`

	resUnquoted, err := request.PrepareHTTPRequestWithTracking(
		mhttp.HTTP{Url: "http://test.com", BodyKind: mhttp.HttpBodyKindRaw}, nil, nil, &mhttp.HTTPBodyRaw{RawData: []byte(templateUnquoted)}, nil, nil, varMap,
	)
	require.NoError(t, err)

	// The engine simply replaces the text. It does not auto-quote.
	// Result: {"id": 8d98027f-8570-45cb-90ae-e8fa1d87dbf5} -> Invalid JSON
	expectedUnquoted := fmt.Sprintf(`{"id": %s}`, uuidVal)
	if string(resUnquoted.Request.Body) != expectedUnquoted {
		t.Errorf("Unquoted substitution failed.\nWant: %s\nGot:  %s", expectedUnquoted, string(resUnquoted.Request.Body))
	}
}
