package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
)

// TestFlowRunCommand_CleanupRemovesCreatedData runs `flow run` end to end
// (YAML file -> import -> run -> JSON report) against a fake API that rejects
// a duplicate name the way a real one would, and counts DELETE calls. The
// first run fails in the middle; its cleanup must still remove what it
// created, so the second run starts from the same state and passes.
func TestFlowRunCommand_CleanupRemovesCreatedData(t *testing.T) {
	var (
		mu      sync.Mutex
		names   = map[string]bool{} // the API's "database" of category names
		deletes int
		failTag = true
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/categories":
			if names["golden"] {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":"name already exists"}`))
				return
			}
			names["golden"] = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"golden"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/tags":
			if failTag {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"t1"}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/categories/"):
			deletes++
			delete(names, strings.TrimPrefix(r.URL.Path, "/categories/"))
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete:
			deletes++
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	flowFile := filepath.Join(dir, "catalog.yaml")
	doc := fmt.Sprintf(`
workspace_name: Catalog
flows:
  - name: Catalog
    steps:
      - request:
          name: PostCategory
          method: POST
          url: %[1]s/categories
          assertions:
            - response.status == 201
      - request:
          name: PostTag
          depends_on: PostCategory
          method: POST
          url: %[1]s/tags
          assertions:
            - response.status == 201
    cleanup:
      - request:
          name: DeleteTag
          method: DELETE
          url: "%[1]s/tags/{{ PostTag.response.body.id }}"
          assertions:
            - response.status == 204
      - request:
          name: DeleteCategory
          method: DELETE
          url: "%[1]s/categories/{{ PostCategory.response.body.id }}"
          assertions:
            - response.status == 204
`, srv.URL)
	if err := os.WriteFile(flowFile, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	// Run 1: PostTag fails. DeleteTag reads PostTag's (error) response and
	// still runs; DeleteCategory removes the category.
	report1 := filepath.Join(dir, "run1.json")
	err := runFlowCommand(t, flowFile, report1)
	if err == nil || !strings.Contains(err.Error(), "assertion failed: response.status == 201") {
		t.Fatalf("run 1 must fail with the original assertion, got %v", err)
	}
	mu.Lock()
	if deletes != 2 || names["golden"] {
		mu.Unlock()
		t.Fatalf("run 1 cleanup: deletes=%d, leftover category=%v", deletes, names["golden"])
	}
	failTag = false
	mu.Unlock()

	results := readReport(t, report1)
	if len(results) != 1 {
		t.Fatalf("report has %d flows, want 1 (the hidden cleanup flow must not run on its own)", len(results))
	}
	var cleanupSteps []string
	for _, n := range results[0].Nodes {
		if n.Cleanup {
			cleanupSteps = append(cleanupSteps, n.Name+"="+n.State)
		}
	}
	if strings.Join(cleanupSteps, ",") != "DeleteTag=Success,DeleteCategory=Success" {
		t.Fatalf("cleanup steps in report = %v", cleanupSteps)
	}

	// Run 2 starts from the same state: no "name already exists".
	if err := runFlowCommand(t, flowFile, filepath.Join(dir, "run2.json")); err != nil {
		t.Fatalf("run 2 must pass after run 1's cleanup, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if deletes != 4 || names["golden"] {
		t.Fatalf("run 2 cleanup: deletes=%d, leftover category=%v", deletes, names["golden"])
	}
}

func runFlowCommand(t *testing.T, flowFile, reportPath string) error {
	t.Helper()

	prevFormats, prevQuiet := reportFormats, quietMode
	t.Cleanup(func() { reportFormats, quietMode = prevFormats, prevQuiet })
	reportFormats = []string{"json:" + reportPath}
	quietMode = true

	yamlflowRunCmd.SetContext(context.Background())
	return yamlflowRunCmd.RunE(yamlflowRunCmd, []string{flowFile, "Catalog"})
}

func readReport(t *testing.T, path string) []model.FlowRunResult {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var results []model.FlowRunResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	return results
}
