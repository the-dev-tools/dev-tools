package loadrun

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
)

// TestRunRunsCleanupPerIteration: every iteration's cleanup steps run after
// its normal steps, read that iteration's outputs (so lean mode must keep
// the bodies they reference), and stay out of the load report.
func TestRunRunsCleanupPerIteration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	var (
		mu      sync.Mutex
		next    int
		deletes []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPost:
			next++
			_, _ = fmt.Fprintf(w, `{"id":"item-%d"}`, next)
		case http.MethodDelete:
			deletes = append(deletes, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(testPayload))
		}
	}))
	t.Cleanup(srv.Close)

	yamlDoc := fmt.Sprintf(`
workspace_name: Load Cleanup
flows:
  - name: CreateItem
    steps:
      - manual_start:
          name: Start
      - request:
          name: Create
          depends_on: Start
          method: POST
          url: %[1]s/items
    cleanup:
      - request:
          name: Remove
          method: DELETE
          url: "%[1]s/items/{{ Create.response.body.id }}"
`, srv.URL)

	flow, services := setupFlow(t, yamlDoc, "CreateItem")

	const iterations = 6
	result, err := Run(t.Context(), Config{Flow: flow, VUs: 2, MaxIterations: iterations}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Summary.Errors != 0 {
		t.Errorf("Summary.Errors = %d, want 0", result.Summary.Errors)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(deletes) != iterations {
		t.Fatalf("cleanup ran %d times, want %d: %v", len(deletes), iterations, deletes)
	}
	seen := make(map[string]bool, len(deletes))
	for _, d := range deletes {
		if d == "/items/" || seen[d] {
			t.Fatalf("each iteration must delete its own item, got %v", deletes)
		}
		seen[d] = true
	}

	if result.Report.Total.Count != iterations {
		t.Errorf("Report.Total.Count = %d, want %d (cleanup requests are not measured)", result.Report.Total.Count, iterations)
	}
	if _, ok := result.ByStep.PerStep[loadmetrics.Key{Step: "Remove"}]; ok {
		t.Error("cleanup step must not appear in the load report")
	}
}

// TestRunCleanupFailureCountsAsIterationError mirrors the functional rule: a
// cleanup failure fails an otherwise passing iteration.
func TestRunCleanupFailureCountsAsIterationError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	t.Cleanup(srv.Close)

	yamlDoc := fmt.Sprintf(`
workspace_name: Load Cleanup Failure
flows:
  - name: CreateItem
    steps:
      - request:
          name: Create
          method: POST
          url: %[1]s/items
    cleanup:
      - request:
          name: Remove
          method: DELETE
          url: "%[1]s/items/{{ Create.response.body.id }}"
          assertions:
            - response.status == 204
`, srv.URL)

	flow, services := setupFlow(t, yamlDoc, "CreateItem")
	result, _ := Run(t.Context(), Config{Flow: flow, VUs: 1, MaxIterations: 3}, services, nil)
	if result.Summary.Errors != 3 {
		t.Errorf("Summary.Errors = %d, want 3", result.Summary.Errors)
	}
}
