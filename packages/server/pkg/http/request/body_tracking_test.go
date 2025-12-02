package request_test

import (
	"testing"

	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/varsystem"
)

func TestBodyTracking(t *testing.T) {
	// Setup
	varMap := varsystem.NewVarMap([]mvar.Var{
		{ID: idwrap.NewNow(), VarKey: "bodyVar", Value: "replacedBody"},
		{ID: idwrap.NewNow(), VarKey: "headerVar", Value: "replacedHeader"},
	})

	httpReq := mhttp.HTTP{
		Method:   "POST",
		Url:      "http://example.com",
		BodyKind: mhttp.HttpBodyKindRaw,
	}

	headers := []mhttp.HTTPHeader{
		{Key: "X-Test", Value: "{{headerVar}}", Enabled: true},
	}

	rawBody := &mhttp.HTTPBodyRaw{
		RawData: []byte(`{"data": "{{bodyVar}}"}`),
	}

	// Execute
	res, err := request.PrepareHTTPRequestWithTracking(
		httpReq,
		headers,
		nil,      // params
		rawBody,
		nil,      // form
		nil,      // urlEncoded
		varMap,
	)
	if err != nil {
		t.Fatalf("PrepareHTTPRequestWithTracking failed: %v", err)
	}

	// Verify Substitution
	expectedBody := `{"data": "replacedBody"}`
	if string(res.Request.Body) != expectedBody {
		t.Errorf("Body substitution failed. Got: %s, Want: %s", string(res.Request.Body), expectedBody)
	}

	// Verify Tracking
	tracked := res.ReadVars
	if len(tracked) == 0 {
		t.Fatal("No variables tracked!")
	}

	if _, ok := tracked["headerVar"]; !ok {
		t.Error("headerVar not tracked")
	}

	if _, ok := tracked["bodyVar"]; !ok {
		t.Error("bodyVar not tracked")
	}
}

// TestBodyOnlyVariableTracking verifies that variables used ONLY in the body
// (not in URL or headers) are properly tracked. This is a regression test for
// the issue where body variables were not being tracked while header variables were.
func TestBodyOnlyVariableTracking(t *testing.T) {
	// Setup: variable used ONLY in body, not in URL or headers
	// Simulates referencing another request's response body
	varMap := varsystem.NewVarMapFromAnyMap(map[string]any{
		"prev_request": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"id": "test-id-123",
				},
			},
		},
	})

	httpReq := mhttp.HTTP{
		Method:   "POST",
		Url:      "http://example.com/api", // NO variables in URL
		BodyKind: mhttp.HttpBodyKindRaw,
	}

	rawBody := &mhttp.HTTPBodyRaw{
		RawData: []byte(`{"categoryId": "{{prev_request.response.body.id}}"}`),
	}

	// Execute with NO headers (body is the only place with variables)
	res, err := request.PrepareHTTPRequestWithTracking(
		httpReq,
		nil, // no headers
		nil, // no params
		rawBody,
		nil, // no form
		nil, // no urlEncoded
		varMap,
	)
	if err != nil {
		t.Fatalf("PrepareHTTPRequestWithTracking failed: %v", err)
	}

	// Verify Substitution worked
	expectedBody := `{"categoryId": "test-id-123"}`
	if string(res.Request.Body) != expectedBody {
		t.Errorf("Body substitution failed. Got: %s, Want: %s", string(res.Request.Body), expectedBody)
	}

	// Verify Tracking - the body-only variable should be tracked
	tracked := res.ReadVars
	if len(tracked) == 0 {
		t.Fatal("No variables tracked! Body-only variable tracking failed.")
	}

	if _, ok := tracked["prev_request.response.body.id"]; !ok {
		t.Errorf("Body-only variable 'prev_request.response.body.id' not tracked. Got tracked vars: %v", tracked)
	}
}
