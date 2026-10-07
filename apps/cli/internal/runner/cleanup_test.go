package runner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/reporter"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// catalogAPI is a fake API for cleanup tests. It records every request as
// "METHOD /path" and answers from the routes table; unknown routes get 200 {}.
type catalogAPI struct {
	mu     sync.Mutex
	calls  []string
	routes map[string]struct {
		status int
		body   string
	}
}

func newCatalogAPI(t *testing.T) (*catalogAPI, *httptest.Server) {
	t.Helper()
	api := &catalogAPI{routes: map[string]struct {
		status int
		body   string
	}{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		api.mu.Lock()
		api.calls = append(api.calls, key)
		route, ok := api.routes[key]
		api.mu.Unlock()
		if !ok {
			route.status, route.body = http.StatusOK, `{}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(route.status)
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	return api, srv
}

func (a *catalogAPI) on(method, path string, status int, body string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.routes[method+" "+path] = struct {
		status int
		body   string
	}{status, body}
}

func (a *catalogAPI) callsWithMethod(method string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for _, c := range a.calls {
		if strings.HasPrefix(c, method+" ") {
			out = append(out, c)
		}
	}
	return out
}

// runCleanupFlow converts doc, imports it with its cleanup blocks the way the
// CLI does, and runs flowName with a JSON reporter. It returns the run result,
// the flow results the JSON report recorded, and the run error.
func runCleanupFlow(t *testing.T, doc, flowName string) (model.FlowRunResult, []model.FlowRunResult, error) {
	t.Helper()

	fixture := newFlowTestFixture(t)

	bundle, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(doc), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: fixture.workspaceID,
	})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	ios := ioworkspace.New(fixture.queries, fixture.services.Logger)
	opts := ioworkspace.GetDefaultImportOptions(fixture.workspaceID)
	opts.PreserveIDs = true
	opts.ImportFlowCleanups = true
	tx, err := fixture.db.BeginTx(fixture.ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := ios.Import(fixture.ctx, tx, bundle, opts); err != nil {
		_ = tx.Rollback()
		t.Fatalf("import: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var flow *mflow.Flow
	for i := range bundle.Flows {
		if bundle.Flows[i].Name == flowName {
			flow = &bundle.Flows[i]
		}
	}
	if flow == nil {
		t.Fatalf("flow %q not found", flowName)
	}

	reportPath := t.TempDir() + "/report.json"
	reporters := newJSONReporters(t, reportPath)

	services := fixture.getRunnerServices(nil)
	services.Cleanups = bundle.CleanupsByFlowID()

	ctx, cancel := context.WithTimeout(fixture.ctx, 20*time.Second)
	defer cancel()

	result, runErr := runner.RunFlow(ctx, flow, services, reporters)
	if err := reporters.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	return result, readJSONReport(t, reportPath), runErr
}

func nodeByName(t *testing.T, result model.FlowRunResult, name string) model.NodeRunResult {
	t.Helper()
	for _, n := range result.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("node %q not in result; have %v", name, nodeNames(result))
	return model.NodeRunResult{}
}

func nodeNames(result model.FlowRunResult) []string {
	names := make([]string, 0, len(result.Nodes))
	for _, n := range result.Nodes {
		names = append(names, n.Name)
	}
	return names
}

const catalogFlow = `
workspace_name: Cleanup Runner
flows:
  - name: Catalog
    steps:
      - manual_start:
          name: Start
      - request:
          name: PostCategory
          depends_on: Start
          method: POST
          url: %[1]s/categories
      - request:
          name: PostProducts
          depends_on: PostCategory
          method: POST
          url: %[1]s/products
          assertions:
            - response.status == 201
      - request:
          name: TagProduct
          depends_on: PostProducts
          method: POST
          url: %[1]s/tags
    cleanup:
      - request:
          name: UntagProduct
          method: DELETE
          url: "%[1]s/tags/{{ TagProduct.response.body.id }}"
      - request:
          name: ForgetTag
          method: DELETE
          url: %[1]s/tag-log
          depends_on: UntagProduct
      - request:
          name: DeleteProduct
          method: DELETE
          url: "%[1]s/products/{{ PostProducts.response.body.id }}"
      - request:
          name: DeleteCategory
          method: DELETE
          url: "%[1]s/categories/{{ PostCategory.response.body.id }}"
`

func TestRunFlowCleanup_RunsAfterSuccess(t *testing.T) {
	api, srv := newCatalogAPI(t)
	api.on("POST", "/categories", 201, `{"id":"c1"}`)
	api.on("POST", "/products", 201, `{"id":"p1"}`)
	api.on("POST", "/tags", 201, `{"id":"t1"}`)

	result, report, err := runCleanupFlow(t, fmt.Sprintf(catalogFlow, srv.URL), "Catalog")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q (%s)", result.Status, result.Error)
	}

	want := []string{"DELETE /tags/t1", "DELETE /tag-log", "DELETE /products/p1", "DELETE /categories/c1"}
	if got := api.callsWithMethod("DELETE"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("DELETE calls = %v, want %v", got, want)
	}

	// Cleanup steps are reported after the normal steps, marked as cleanup.
	names := nodeNames(result)
	if len(names) < 4 {
		t.Fatalf("nodes = %v", names)
	}
	tail := names[len(names)-4:]
	if strings.Join(tail, ",") != "UntagProduct,ForgetTag,DeleteProduct,DeleteCategory" {
		t.Fatalf("cleanup steps must be reported last in order, got %v", names)
	}
	for _, n := range result.Nodes {
		isCleanup := strings.HasPrefix(n.Name, "Untag") || strings.HasPrefix(n.Name, "Forget") || strings.HasPrefix(n.Name, "Delete")
		if n.Cleanup != isCleanup {
			t.Fatalf("node %s cleanup=%v", n.Name, n.Cleanup)
		}
	}

	if len(report) != 1 || len(report[0].Nodes) != len(result.Nodes) {
		t.Fatalf("JSON report must carry the cleanup steps: %+v", report)
	}
	if !nodeByName(t, report[0], "DeleteCategory").Cleanup {
		t.Fatal("JSON report must mark cleanup steps")
	}
}

func TestRunFlowCleanup_RunsAfterMiddleStepFails(t *testing.T) {
	api, srv := newCatalogAPI(t)
	api.on("POST", "/categories", 201, `{"id":"c1"}`)
	api.on("POST", "/products", 409, `{"id":"p-dup","error":"name already exists"}`)

	result, _, err := runCleanupFlow(t, fmt.Sprintf(catalogFlow, srv.URL), "Catalog")
	if err == nil {
		t.Fatal("expected the flow to fail")
	}
	if result.Status != "failed" {
		t.Fatalf("status = %q", result.Status)
	}
	if !strings.Contains(result.Error, "assertion failed") || strings.Contains(result.Error, "cleanup") {
		t.Fatalf("the original failure must stay the reported error, got %q", result.Error)
	}
	if err.Error() != result.Error {
		t.Fatalf("returned error %q differs from result error %q", err, result.Error)
	}

	// TagProduct never ran: UntagProduct is skipped, and so is ForgetTag,
	// which depends on it. The others still run.
	want := []string{"DELETE /products/p-dup", "DELETE /categories/c1"}
	if got := api.callsWithMethod("DELETE"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("DELETE calls = %v, want %v", got, want)
	}

	untag := nodeByName(t, result, "UntagProduct")
	if untag.State != model.NodeStateSkipped || !untag.Cleanup {
		t.Fatalf("UntagProduct = %+v, want skipped cleanup step", untag)
	}
	if !strings.Contains(untag.SkipReason, "TagProduct") {
		t.Fatalf("skip reason %q must name the step that never ran", untag.SkipReason)
	}
	if untag.Error != "" {
		t.Fatalf("a skipped step is not an error: %q", untag.Error)
	}
	forget := nodeByName(t, result, "ForgetTag")
	if forget.State != model.NodeStateSkipped || !strings.Contains(forget.SkipReason, "UntagProduct") {
		t.Fatalf("ForgetTag = %+v, want skipped because of UntagProduct", forget)
	}
	if nodeByName(t, result, "DeleteCategory").State != mflow.StringNodeState(mflow.NODE_STATE_SUCCESS) {
		t.Fatal("DeleteCategory must succeed")
	}
}

func TestRunFlowCleanup_CleanupFailureFailsPassingFlow(t *testing.T) {
	api, srv := newCatalogAPI(t)
	api.on("POST", "/categories", 201, `{"id":"c1"}`)
	api.on("POST", "/products", 201, `{"id":"p1"}`)
	api.on("POST", "/tags", 201, `{"id":"t1"}`)

	doc := fmt.Sprintf(`
workspace_name: Cleanup Failure
flows:
  - name: Catalog
    steps:
      - request:
          name: PostCategory
          method: POST
          url: %[1]s/categories
    cleanup:
      - request:
          name: DeleteCategory
          method: DELETE
          url: "%[1]s/categories/{{ PostCategory.response.body.id }}"
          assertions:
            - response.status == 204
      - request:
          name: DeleteAudit
          method: DELETE
          url: %[1]s/audit
`, srv.URL)

	result, _, err := runCleanupFlow(t, doc, "Catalog")
	if err == nil || result.Status != "failed" {
		t.Fatalf("a failing cleanup step must fail the flow, got status %q err %v", result.Status, err)
	}
	if !strings.Contains(result.Error, "cleanup step 'DeleteCategory' failed") {
		t.Fatalf("error = %q", result.Error)
	}
	// A failing cleanup step does not stop the remaining ones.
	want := []string{"DELETE /categories/c1", "DELETE /audit"}
	if got := api.callsWithMethod("DELETE"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("DELETE calls = %v, want %v", got, want)
	}
	if nodeByName(t, result, "PostCategory").State != mflow.StringNodeState(mflow.NODE_STATE_SUCCESS) {
		t.Fatal("PostCategory must still be reported as successful")
	}
}

func TestRunFlowCleanup_OriginalFailureWinsOverCleanupFailure(t *testing.T) {
	api, srv := newCatalogAPI(t)
	api.on("POST", "/categories", 201, `{"id":"c1"}`)
	api.on("POST", "/products", 500, `{}`)
	api.on("DELETE", "/categories/c1", 500, `{}`)

	doc := fmt.Sprintf(`
workspace_name: Both Fail
flows:
  - name: Catalog
    steps:
      - request:
          name: PostCategory
          method: POST
          url: %[1]s/categories
      - request:
          name: PostProducts
          depends_on: PostCategory
          method: POST
          url: %[1]s/products
          assertions:
            - response.status == 201
    cleanup:
      - request:
          name: DeleteCategory
          method: DELETE
          url: "%[1]s/categories/{{ PostCategory.response.body.id }}"
          assertions:
            - response.status == 204
`, srv.URL)

	result, _, err := runCleanupFlow(t, doc, "Catalog")
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(result.Error, "assertion failed: response.status == 201") {
		t.Fatalf("the original failure must be the reported error, got %q", result.Error)
	}
	if nodeByName(t, result, "DeleteCategory").State != mflow.StringNodeState(mflow.NODE_STATE_FAILURE) {
		t.Fatal("the cleanup failure must still be reported on its step")
	}
	if got := api.callsWithMethod("DELETE"); len(got) != 1 {
		t.Fatalf("DELETE calls = %v", got)
	}
}

func TestRunFlowCleanup_FlowWithoutCleanupUnchanged(t *testing.T) {
	api, srv := newCatalogAPI(t)
	api.on("POST", "/categories", 201, `{"id":"c1"}`)

	doc := fmt.Sprintf(`
workspace_name: No Cleanup
flows:
  - name: Catalog
    steps:
      - request:
          name: PostCategory
          method: POST
          url: %s/categories
`, srv.URL)

	result, _, err := runCleanupFlow(t, doc, "Catalog")
	if err != nil || result.Status != "success" {
		t.Fatalf("status %q err %v", result.Status, err)
	}
	for _, n := range result.Nodes {
		if n.Cleanup {
			t.Fatalf("unexpected cleanup node %s", n.Name)
		}
	}
	if got := api.callsWithMethod("DELETE"); len(got) != 0 {
		t.Fatalf("DELETE calls = %v", got)
	}
}

// readJSONReport decodes the bare-array JSON report.
func readJSONReport(t *testing.T, path string) []model.FlowRunResult {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var results []model.FlowRunResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("decode report: %v\n%s", err, data)
	}
	return results
}

func newJSONReporters(t *testing.T, path string) *reporter.ReporterGroup {
	t.Helper()
	specs, err := reporter.ParseReportSpecs([]string{"json:" + path})
	if err != nil {
		t.Fatalf("report specs: %v", err)
	}
	group, err := reporter.NewReporterGroup(specs, reporter.ReporterOptions{})
	if err != nil {
		t.Fatalf("reporters: %v", err)
	}
	return group
}
