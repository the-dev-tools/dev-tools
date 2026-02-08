package topencollection

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", name)
}

func TestConvertOpenCollection_BasicCollection(t *testing.T) {
	collectionPath := testdataPath("basic-collection")
	opts := ConvertOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	result, err := ConvertOpenCollection(collectionPath, opts)
	if err != nil {
		t.Fatalf("ConvertOpenCollection failed: %v", err)
	}

	// Verify collection name
	if result.CollectionName != "Test API Collection" {
		t.Errorf("expected collection name 'Test API Collection', got %q", result.CollectionName)
	}

	// Verify HTTP requests: Get Users, Create User, Login = 3
	if len(result.HTTPRequests) != 3 {
		t.Fatalf("expected 3 HTTP requests, got %d", len(result.HTTPRequests))
	}

	// Build lookup by name
	reqByName := make(map[string]mhttp.HTTP)
	for _, r := range result.HTTPRequests {
		reqByName[r.Name] = r
	}

	// Verify Get Users
	getUsers, ok := reqByName["Get Users"]
	if !ok {
		t.Fatal("missing 'Get Users' request")
	}
	if getUsers.Method != "GET" {
		t.Errorf("Get Users method: expected GET, got %s", getUsers.Method)
	}
	if getUsers.Url != "{{base_url}}/users" {
		t.Errorf("Get Users URL: expected {{base_url}}/users, got %s", getUsers.Url)
	}
	if getUsers.Description != "Fetch all users with pagination" {
		t.Errorf("Get Users description mismatch: got %q", getUsers.Description)
	}

	// Verify Create User
	createUser, ok := reqByName["Create User"]
	if !ok {
		t.Fatal("missing 'Create User' request")
	}
	if createUser.Method != "POST" {
		t.Errorf("Create User method: expected POST, got %s", createUser.Method)
	}
	if createUser.BodyKind != mhttp.HttpBodyKindRaw {
		t.Errorf("Create User body kind: expected Raw (%d), got %d", mhttp.HttpBodyKindRaw, createUser.BodyKind)
	}

	// Verify Login
	login, ok := reqByName["Login"]
	if !ok {
		t.Fatal("missing 'Login' request")
	}
	if login.Method != "POST" {
		t.Errorf("Login method: expected POST, got %s", login.Method)
	}

	// Verify headers exist
	if len(result.HTTPHeaders) == 0 {
		t.Error("expected some headers, got none")
	}

	// Check for bearer auth header on Create User
	var foundBearerAuth bool
	for _, h := range result.HTTPHeaders {
		if h.HttpID == createUser.ID && h.Key == "Authorization" {
			foundBearerAuth = true
			if h.Value != "Bearer {{token}}" {
				t.Errorf("expected 'Bearer {{token}}', got %q", h.Value)
			}
		}
	}
	if !foundBearerAuth {
		t.Error("missing bearer auth header for Create User")
	}

	// Verify search params (Get Users has page + limit)
	var getUsersParams int
	for _, p := range result.HTTPSearchParams {
		if p.HttpID == getUsers.ID {
			getUsersParams++
		}
	}
	if getUsersParams != 2 {
		t.Errorf("expected 2 search params for Get Users, got %d", getUsersParams)
	}

	// Verify body raw exists for Create User
	var createUserBodyRaw int
	for _, b := range result.HTTPBodyRaw {
		if b.HttpID == createUser.ID {
			createUserBodyRaw++
		}
	}
	if createUserBodyRaw != 1 {
		t.Errorf("expected 1 body raw for Create User, got %d", createUserBodyRaw)
	}

	// Verify assertions
	if len(result.HTTPAsserts) == 0 {
		t.Error("expected some assertions, got none")
	}

	// Verify files
	if len(result.Files) == 0 {
		t.Error("expected some files, got none")
	}

	// Verify folder structure: should have "users" and "auth" folders
	var folderCount int
	for _, f := range result.Files {
		if f.ContentType == mfile.ContentTypeFolder {
			folderCount++
		}
	}
	if folderCount != 2 {
		t.Errorf("expected 2 folders (users, auth), got %d", folderCount)
	}

	// Verify environments
	if len(result.Environments) != 2 {
		t.Errorf("expected 2 environments, got %d", len(result.Environments))
	}

	// Verify environment variables
	if len(result.EnvironmentVars) == 0 {
		t.Error("expected some environment variables, got none")
	}
}

func TestConvertOpenCollection_SkipsNonHTTP(t *testing.T) {
	// Create a temp directory with a graphql request
	dir := t.TempDir()

	// Write opencollection.yml
	writeYAML(t, filepath.Join(dir, "opencollection.yml"), map[string]interface{}{
		"opencollection": "1.0.0",
		"info": map[string]interface{}{
			"name": "Test",
		},
	})

	// Write a graphql request
	writeYAML(t, filepath.Join(dir, "graphql-query.yml"), map[string]interface{}{
		"info": map[string]interface{}{
			"name": "GraphQL Query",
			"type": "graphql",
		},
	})

	// Write an HTTP request
	writeYAML(t, filepath.Join(dir, "http-request.yml"), map[string]interface{}{
		"info": map[string]interface{}{
			"name": "HTTP Request",
			"type": "http",
		},
		"http": map[string]interface{}{
			"method": "GET",
			"url":    "https://example.com",
		},
	})

	opts := ConvertOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	result, err := ConvertOpenCollection(dir, opts)
	if err != nil {
		t.Fatalf("ConvertOpenCollection failed: %v", err)
	}

	// Should only have 1 HTTP request (graphql skipped)
	if len(result.HTTPRequests) != 1 {
		t.Errorf("expected 1 HTTP request (graphql skipped), got %d", len(result.HTTPRequests))
	}

	if result.HTTPRequests[0].Name != "HTTP Request" {
		t.Errorf("expected 'HTTP Request', got %q", result.HTTPRequests[0].Name)
	}
}

func TestConvertAuth_Bearer(t *testing.T) {
	httpID := idwrap.NewNow()
	auth := &OCAuth{Type: "bearer", Token: "my-token"}

	headers, params := convertAuth(auth, httpID)

	if len(headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(headers))
	}
	if headers[0].Key != "Authorization" || headers[0].Value != "Bearer my-token" {
		t.Errorf("unexpected header: %s: %s", headers[0].Key, headers[0].Value)
	}
	if len(params) != 0 {
		t.Errorf("expected 0 params, got %d", len(params))
	}
}

func TestConvertAuth_APIKey_Query(t *testing.T) {
	httpID := idwrap.NewNow()
	auth := &OCAuth{Type: "apikey", Key: "api_key", Value: "secret123", Placement: "query"}

	headers, params := convertAuth(auth, httpID)

	if len(headers) != 0 {
		t.Errorf("expected 0 headers, got %d", len(headers))
	}
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if params[0].Key != "api_key" || params[0].Value != "secret123" {
		t.Errorf("unexpected param: %s=%s", params[0].Key, params[0].Value)
	}
}

func TestConvertBody_JSON(t *testing.T) {
	httpID := idwrap.NewNow()
	body := &OCBody{Type: "json", Data: `{"key": "value"}`}

	kind, raw, forms, urlencoded := convertBody(body, httpID)

	if kind != mhttp.HttpBodyKindRaw {
		t.Errorf("expected Raw body kind, got %d", kind)
	}
	if raw == nil {
		t.Fatal("expected non-nil raw body")
	}
	if string(raw.RawData) != `{"key": "value"}` {
		t.Errorf("unexpected raw data: %s", string(raw.RawData))
	}
	if len(forms) != 0 {
		t.Errorf("expected 0 forms, got %d", len(forms))
	}
	if len(urlencoded) != 0 {
		t.Errorf("expected 0 urlencoded, got %d", len(urlencoded))
	}
}

func TestConvertBody_None(t *testing.T) {
	httpID := idwrap.NewNow()
	kind, raw, forms, urlencoded := convertBody(nil, httpID)

	if kind != mhttp.HttpBodyKindNone {
		t.Errorf("expected None body kind, got %d", kind)
	}
	if raw != nil {
		t.Error("expected nil raw body")
	}
	if len(forms) != 0 {
		t.Errorf("expected 0 forms, got %d", len(forms))
	}
	if len(urlencoded) != 0 {
		t.Errorf("expected 0 urlencoded, got %d", len(urlencoded))
	}
}

// writeYAML is a test helper to write YAML files.
func writeYAML(t *testing.T, path string, data interface{}) {
	t.Helper()
	yamlData, err := yamlMarshal(data)
	if err != nil {
		t.Fatalf("failed to marshal yaml: %v", err)
	}
	if err := writeFile(path, yamlData); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

func yamlMarshal(v interface{}) ([]byte, error) {
	return yamlMarshalImpl(v)
}
