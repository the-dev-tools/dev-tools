package openyaml

import (
	"os"
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

func TestReadDirectory(t *testing.T) {
	dirPath := testdataPath("collection")
	opts := ReadOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	bundle, err := ReadDirectory(dirPath, opts)
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}

	// Verify workspace name
	if bundle.Workspace.Name != "collection" {
		t.Errorf("expected workspace name 'collection', got %q", bundle.Workspace.Name)
	}

	// Verify HTTP requests: Get Users, Create User = 2
	if len(bundle.HTTPRequests) != 2 {
		t.Fatalf("expected 2 HTTP requests, got %d", len(bundle.HTTPRequests))
	}

	reqByName := make(map[string]mhttp.HTTP)
	for _, r := range bundle.HTTPRequests {
		reqByName[r.Name] = r
	}

	getUsers, ok := reqByName["Get Users"]
	if !ok {
		t.Fatal("missing 'Get Users' request")
	}
	if getUsers.Method != "GET" {
		t.Errorf("Get Users method: expected GET, got %s", getUsers.Method)
	}

	createUser, ok := reqByName["Create User"]
	if !ok {
		t.Fatal("missing 'Create User' request")
	}
	if createUser.Method != "POST" {
		t.Errorf("Create User method: expected POST, got %s", createUser.Method)
	}

	// Verify headers
	if len(bundle.HTTPHeaders) == 0 {
		t.Error("expected some headers, got none")
	}

	// Verify query params for Get Users
	var getUsersParams int
	for _, p := range bundle.HTTPSearchParams {
		if p.HttpID == getUsers.ID {
			getUsersParams++
		}
	}
	if getUsersParams != 2 {
		t.Errorf("expected 2 search params for Get Users, got %d", getUsersParams)
	}

	// Verify body raw for Create User
	var createUserBodyRaw int
	for _, b := range bundle.HTTPBodyRaw {
		if b.HttpID == createUser.ID {
			createUserBodyRaw++
		}
	}
	if createUserBodyRaw != 1 {
		t.Errorf("expected 1 body raw for Create User, got %d", createUserBodyRaw)
	}

	// Verify assertions
	if len(bundle.HTTPAsserts) == 0 {
		t.Error("expected some assertions, got none")
	}

	// Verify files: should have "users" folder + 2 requests + 1 flow file
	var folderCount int
	for _, f := range bundle.Files {
		if f.ContentType == mfile.ContentTypeFolder {
			folderCount++
		}
	}
	if folderCount != 1 {
		t.Errorf("expected 1 folder (users), got %d", folderCount)
	}

	// Verify environments
	if len(bundle.Environments) != 1 {
		t.Errorf("expected 1 environment, got %d", len(bundle.Environments))
	}

	// Verify flows
	if len(bundle.Flows) != 1 {
		t.Errorf("expected 1 flow, got %d", len(bundle.Flows))
	}
	if len(bundle.Flows) > 0 && bundle.Flows[0].Name != "Smoke Test" {
		t.Errorf("expected flow name 'Smoke Test', got %q", bundle.Flows[0].Name)
	}
}

func TestRoundTrip(t *testing.T) {
	// Read a directory
	srcPath := testdataPath("collection")
	opts := ReadOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	bundle, err := ReadDirectory(srcPath, opts)
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}

	// Write to a temp directory
	outDir := filepath.Join(t.TempDir(), "output")
	if err := WriteDirectory(outDir, bundle); err != nil {
		t.Fatalf("WriteDirectory failed: %v", err)
	}

	// Re-read the written directory
	opts2 := ReadOptions{
		WorkspaceID: idwrap.NewNow(),
	}

	bundle2, err := ReadDirectory(outDir, opts2)
	if err != nil {
		t.Fatalf("ReadDirectory (round-trip) failed: %v", err)
	}

	// Compare counts
	if len(bundle.HTTPRequests) != len(bundle2.HTTPRequests) {
		t.Errorf("request count mismatch: %d vs %d", len(bundle.HTTPRequests), len(bundle2.HTTPRequests))
	}
	if len(bundle.Environments) != len(bundle2.Environments) {
		t.Errorf("environment count mismatch: %d vs %d", len(bundle.Environments), len(bundle2.Environments))
	}
	if len(bundle.Flows) != len(bundle2.Flows) {
		t.Errorf("flow count mismatch: %d vs %d", len(bundle.Flows), len(bundle2.Flows))
	}

	// Verify the written directory has correct structure
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}

	var hasFolders, hasEnvs, hasFlows bool
	for _, e := range entries {
		switch e.Name() {
		case "environments":
			hasEnvs = true
		case "flows":
			hasFlows = true
		case "users":
			hasFolders = true
		}
	}

	if !hasEnvs {
		t.Error("missing environments/ directory in output")
	}
	if !hasFlows {
		t.Error("missing flows/ directory in output")
	}
	if !hasFolders {
		t.Error("missing users/ directory in output")
	}
}

func TestReadWriteSingleRequest(t *testing.T) {
	yamlData := []byte(`
name: Test Request
method: POST
url: "https://api.example.com/test"
order: 5
headers:
  Content-Type: application/json
body:
  type: raw
  raw: '{"key": "value"}'
assertions:
  - "res.status eq 200"
`)

	req, err := ReadSingleRequest(yamlData)
	if err != nil {
		t.Fatalf("ReadSingleRequest failed: %v", err)
	}

	if req.Name != "Test Request" {
		t.Errorf("expected name 'Test Request', got %q", req.Name)
	}
	if req.Method != "POST" {
		t.Errorf("expected method POST, got %s", req.Method)
	}
	if req.Order != 5 {
		t.Errorf("expected order 5, got %f", req.Order)
	}

	// Round-trip
	data, err := WriteSingleRequest(*req)
	if err != nil {
		t.Fatalf("WriteSingleRequest failed: %v", err)
	}

	req2, err := ReadSingleRequest(data)
	if err != nil {
		t.Fatalf("ReadSingleRequest (round-trip) failed: %v", err)
	}

	if req2.Name != req.Name || req2.Method != req.Method || req2.URL != req.URL {
		t.Error("round-trip mismatch")
	}
}

func TestReadWriteSingleFlow(t *testing.T) {
	yamlData := []byte(`
name: Test Flow
variables:
  - name: token
    value: ""
steps:
  - request:
      name: Login
      method: POST
      url: "https://api.example.com/login"
`)

	flow, err := ReadSingleFlow(yamlData)
	if err != nil {
		t.Fatalf("ReadSingleFlow failed: %v", err)
	}

	if flow.Name != "Test Flow" {
		t.Errorf("expected name 'Test Flow', got %q", flow.Name)
	}
	if len(flow.Variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(flow.Variables))
	}

	// Round-trip
	data, err := WriteSingleFlow(*flow)
	if err != nil {
		t.Fatalf("WriteSingleFlow failed: %v", err)
	}

	flow2, err := ReadSingleFlow(data)
	if err != nil {
		t.Fatalf("ReadSingleFlow (round-trip) failed: %v", err)
	}

	if flow2.Name != flow.Name {
		t.Error("round-trip mismatch")
	}
}
