package request_test

import (
	"testing"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/varsystem"
)

func TestPrepareRequestWithTracking_URL(t *testing.T) {
	// Setup variables
	vars := []mvar.Var{
		{VarKey: "baseUrl", Value: "https://api.example.com"},
		{VarKey: "version", Value: "v1"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Setup endpoint with variables
	endpoint := mitemapi.ItemApi{
		Method: "GET",
		Url:    "{{baseUrl}}/{{version}}/users",
	}

	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Call PrepareRequestWithTracking
	result, err := request.PrepareRequestWithTracking(
		endpoint, example, []mexamplequery.Query{}, []mexampleheader.Header{},
		mbodyraw.ExampleBodyRaw{}, []mbodyform.BodyForm{}, []mbodyurl.BodyURLEncoded{},
		varMap,
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the prepared request
	expectedURL := "https://api.example.com/v1/users"
	if result.Request.URL != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, result.Request.URL)
	}

	// Check tracked variables
	if len(result.ReadVars) != 2 {
		t.Errorf("Expected 2 tracked variables, got %d", len(result.ReadVars))
	}

	expectedVars := map[string]string{
		"baseUrl": "https://api.example.com",
		"version": "v1",
	}

	for key, expectedValue := range expectedVars {
		if result.ReadVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%s', got '%s'", key, expectedValue, result.ReadVars[key])
		}
	}
}

func TestPrepareRequestWithTracking_TrimsVariableKeys(t *testing.T) {
	varMap := varsystem.NewVarMapFromAnyMap(map[string]any{
		"baseUrl": "https://api.example.com",
		"foreach_4": map[string]any{
			"item": map[string]any{
				"id": "abc123",
			},
		},
	})

	endpoint := mitemapi.ItemApi{
		Method: "GET",
		Url:    "{{ baseUrl }}/api/categories/{{ foreach_4.item.id }}",
	}

	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	result, err := request.PrepareRequestWithTracking(
		endpoint,
		example,
		nil,
		nil,
		mbodyraw.ExampleBodyRaw{},
		nil,
		nil,
		varMap,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"baseUrl":           "https://api.example.com",
		"foreach_4.item.id": "abc123",
	}

	if len(result.ReadVars) != len(expected) {
		t.Fatalf("expected %d tracked vars, got %d (%v)", len(expected), len(result.ReadVars), result.ReadVars)
	}

	for key, value := range expected {
		if got := result.ReadVars[key]; got != value {
			t.Fatalf("expected %s=%s, got %s", key, value, got)
		}
	}
}

func TestPrepareRequestWithTracking_Headers(t *testing.T) {
	// Setup variables
	vars := []mvar.Var{
		{VarKey: "token", Value: "abc123"},
		{VarKey: "contentType", Value: "application/json"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Setup endpoint and headers with variables
	endpoint := mitemapi.ItemApi{
		Method: "POST",
		Url:    "https://api.example.com/users",
	}

	headers := []mexampleheader.Header{
		{HeaderKey: "Authorization", Value: "Bearer {{token}}", Enable: true},
		{HeaderKey: "Content-Type", Value: "{{contentType}}", Enable: true},
		{HeaderKey: "X-Static", Value: "static-value", Enable: true},
	}

	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Call PrepareRequestWithTracking
	result, err := request.PrepareRequestWithTracking(
		endpoint, example, []mexamplequery.Query{}, headers,
		mbodyraw.ExampleBodyRaw{}, []mbodyform.BodyForm{}, []mbodyurl.BodyURLEncoded{},
		varMap,
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check tracked variables (should not include static values)
	expectedVarCount := 2
	if len(result.ReadVars) != expectedVarCount {
		t.Errorf("Expected %d tracked variables, got %d", expectedVarCount, len(result.ReadVars))
	}

	expectedVars := map[string]string{
		"token":       "abc123",
		"contentType": "application/json",
	}

	for key, expectedValue := range expectedVars {
		if result.ReadVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%s', got '%s'", key, expectedValue, result.ReadVars[key])
		}
	}

	// Check that headers were properly resolved
	foundAuth := false
	foundContentType := false
	for _, header := range result.Request.Headers {
		if header.HeaderKey == "Authorization" && header.Value == "Bearer abc123" {
			foundAuth = true
		}
		if header.HeaderKey == "Content-Type" && header.Value == "application/json" {
			foundContentType = true
		}
	}

	if !foundAuth {
		t.Error("Authorization header was not properly resolved")
	}
	if !foundContentType {
		t.Error("Content-Type header was not properly resolved")
	}
}

func TestPrepareRequestWithTracking_Queries(t *testing.T) {
	// Setup variables
	vars := []mvar.Var{
		{VarKey: "limit", Value: "10"},
		{VarKey: "sortBy", Value: "name"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Setup endpoint and queries with variables
	endpoint := mitemapi.ItemApi{
		Method: "GET",
		Url:    "https://api.example.com/users",
	}

	queries := []mexamplequery.Query{
		{QueryKey: "limit", Value: "{{limit}}", Enable: true},
		{QueryKey: "sort", Value: "{{sortBy}}", Enable: true},
		{QueryKey: "active", Value: "true", Enable: true},
	}

	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Call PrepareRequestWithTracking
	result, err := request.PrepareRequestWithTracking(
		endpoint, example, queries, []mexampleheader.Header{},
		mbodyraw.ExampleBodyRaw{}, []mbodyform.BodyForm{}, []mbodyurl.BodyURLEncoded{},
		varMap,
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check tracked variables
	expectedVarCount := 2
	if len(result.ReadVars) != expectedVarCount {
		t.Errorf("Expected %d tracked variables, got %d", expectedVarCount, len(result.ReadVars))
	}

	expectedVars := map[string]string{
		"limit":  "10",
		"sortBy": "name",
	}

	for key, expectedValue := range expectedVars {
		if result.ReadVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%s', got '%s'", key, expectedValue, result.ReadVars[key])
		}
	}
}

func TestPrepareRequestWithTracking_Body(t *testing.T) {
	// Setup variables
	vars := []mvar.Var{
		{VarKey: "userName", Value: "john_doe"},
		{VarKey: "userEmail", Value: "john@example.com"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Setup endpoint with body containing variables
	endpoint := mitemapi.ItemApi{
		Method: "POST",
		Url:    "https://api.example.com/users",
	}

	bodyData := `{"name": "{{userName}}", "email": "{{userEmail}}"}`
	rawBody := mbodyraw.ExampleBodyRaw{
		Data: []byte(bodyData),
	}

	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Call PrepareRequestWithTracking
	result, err := request.PrepareRequestWithTracking(
		endpoint, example, []mexamplequery.Query{}, []mexampleheader.Header{},
		rawBody, []mbodyform.BodyForm{}, []mbodyurl.BodyURLEncoded{},
		varMap,
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check tracked variables
	expectedVarCount := 2
	if len(result.ReadVars) != expectedVarCount {
		t.Errorf("Expected %d tracked variables, got %d", expectedVarCount, len(result.ReadVars))
	}

	expectedVars := map[string]string{
		"userName":  "john_doe",
		"userEmail": "john@example.com",
	}

	for key, expectedValue := range expectedVars {
		if result.ReadVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%s', got '%s'", key, expectedValue, result.ReadVars[key])
		}
	}

	// Check that body was properly resolved
	expectedBody := `{"name": "john_doe", "email": "john@example.com"}`
	actualBody := string(result.Request.Body)
	if actualBody != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, actualBody)
	}
}

func TestPrepareRequestWithTracking_NoVariables(t *testing.T) {
	// Setup without variables
	varMap := varsystem.NewVarMap([]mvar.Var{})

	// Setup static endpoint
	endpoint := mitemapi.ItemApi{
		Method: "GET",
		Url:    "https://api.example.com/users",
	}

	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Call PrepareRequestWithTracking
	result, err := request.PrepareRequestWithTracking(
		endpoint, example, []mexamplequery.Query{}, []mexampleheader.Header{},
		mbodyraw.ExampleBodyRaw{}, []mbodyform.BodyForm{}, []mbodyurl.BodyURLEncoded{},
		varMap,
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that no variables were tracked
	if len(result.ReadVars) != 0 {
		t.Errorf("Expected 0 tracked variables, got %d", len(result.ReadVars))
	}

	// Check that URL is unchanged
	if result.Request.URL != endpoint.Url {
		t.Errorf("Expected URL '%s', got '%s'", endpoint.Url, result.Request.URL)
	}
}

func TestPrepareRequestWithTracking_ComplexScenario(t *testing.T) {
	// Setup variables
	vars := []mvar.Var{
		{VarKey: "baseUrl", Value: "https://api.example.com"},
		{VarKey: "version", Value: "v2"},
		{VarKey: "token", Value: "xyz789"},
		{VarKey: "userId", Value: "123"},
		{VarKey: "format", Value: "json"},
	}
	varMap := varsystem.NewVarMap(vars)

	// Setup complex endpoint with variables in multiple places
	endpoint := mitemapi.ItemApi{
		Method: "PUT",
		Url:    "{{baseUrl}}/{{version}}/users/{{userId}}",
	}

	headers := []mexampleheader.Header{
		{HeaderKey: "Authorization", Value: "Bearer {{token}}", Enable: true},
	}

	queries := []mexamplequery.Query{
		{QueryKey: "format", Value: "{{format}}", Enable: true},
	}

	bodyData := `{"id": {{userId}}}`
	rawBody := mbodyraw.ExampleBodyRaw{
		Data: []byte(bodyData),
	}

	example := mitemapiexample.ItemApiExample{
		BodyType: mitemapiexample.BodyTypeRaw,
	}

	// Call PrepareRequestWithTracking
	result, err := request.PrepareRequestWithTracking(
		endpoint, example, queries, headers,
		rawBody, []mbodyform.BodyForm{}, []mbodyurl.BodyURLEncoded{},
		varMap,
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check tracked variables - userId appears twice but should be tracked only once
	expectedVarCount := 5
	if len(result.ReadVars) != expectedVarCount {
		t.Errorf("Expected %d tracked variables, got %d", expectedVarCount, len(result.ReadVars))
		t.Logf("Tracked variables: %v", result.ReadVars)
	}

	expectedVars := map[string]string{
		"baseUrl": "https://api.example.com",
		"version": "v2",
		"token":   "xyz789",
		"userId":  "123",
		"format":  "json",
	}

	for key, expectedValue := range expectedVars {
		if result.ReadVars[key] != expectedValue {
			t.Errorf("Expected tracked %s value '%s', got '%s'", key, expectedValue, result.ReadVars[key])
		}
	}
}
