package loadrun

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/common"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	gqlresolver "github.com/the-dev-tools/dev-tools/packages/server/pkg/graphql/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// twoStepFlowYAML is the load-test workhorse: two chained request steps, so a
// completed iteration must produce exactly two recorded requests.
func twoStepFlowYAML(baseURL string) string {
	return fmt.Sprintf(`
workspace_name: Load Test Workspace
flows:
  - name: LoadFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: StepOne
          depends_on: Start
          method: GET
          url: %s/one
      - request:
          name: StepTwo
          depends_on: StepOne
          method: GET
          url: %s/two
`, baseURL, baseURL)
}

// setupFlow imports a yamlflow document into a fresh in-memory workspace and
// returns the flow plus the services a load run needs. It mirrors the CLI's
// own setup in cmd/flow.go: the import happens exactly once, before any load
// iteration runs.
func setupFlow(t *testing.T, yamlDoc, flowName string) (*mflow.Flow, runner.RunnerServices) {
	t.Helper()

	ctx := t.Context()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("create in-memory db: %v", err)
	}
	t.Cleanup(cleanup)

	services, err := common.CreateServices(ctx, db, logger)
	if err != nil {
		t.Fatalf("create services: %v", err)
	}

	workspaceID := idwrap.NewNow()
	bundle, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlDoc), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("convert yaml: %v", err)
	}

	builder := flowbuilder.New(
		&services.Node, &services.NodeRequest, &services.NodeFor, &services.NodeForEach,
		&services.NodeIf, &services.NodeJS, &services.NodeAI, &services.NodeAiProvider,
		&services.NodeMemory, &services.NodeGraphQL, &services.NodeWsConnection,
		&services.NodeWsSend, &services.NodeWait, &services.NodeSubFlowTrigger,
		&services.NodeSubFlowReturn, &services.NodeRunSubFlow, &services.WebSocket,
		&services.WebSocketHeader, &services.GraphQL, &services.GraphQLHeader,
		&services.Workspace, &services.Variable, &services.FlowVariable,
		resolver.NewStandardResolver(
			&services.HTTP, &services.HTTPHeader, services.HTTPSearchParam,
			services.HTTPBodyRaw, services.HTTPBodyForm, services.HTTPBodyUrlEncoded,
			services.HTTPAssert,
		),
		gqlresolver.NewStandardResolver(
			services.GraphQL.Reader(), &services.GraphQLHeader, &services.GraphQLAssert,
		),
		services.Logger,
		scredential.NewLLMProviderFactory(&services.Credential),
	)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	bundle.Workspace.ID = workspaceID
	if err := services.Workspace.TX(tx).Create(ctx, &bundle.Workspace); err != nil {
		_ = tx.Rollback()
		t.Fatalf("create workspace: %v", err)
	}
	importOpts := ioworkspace.GetDefaultImportOptions(workspaceID)
	importOpts.PreserveIDs = true
	if _, err := ioworkspace.New(services.Queries, logger).Import(ctx, tx, bundle, importOpts); err != nil {
		_ = tx.Rollback()
		t.Fatalf("import bundle: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit import: %v", err)
	}

	flows, err := services.Flow.GetFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get flows: %v", err)
	}
	var flow *mflow.Flow
	for i := range flows {
		if flows[i].Name == flowName {
			flow = &flows[i]
			break
		}
	}
	if flow == nil {
		t.Fatalf("flow %q not found among %d imported flows", flowName, len(flows))
	}

	return flow, runner.RunnerServices{
		NodeService:         services.Node,
		EdgeService:         services.FlowEdge,
		FlowVariableService: services.FlowVariable,
		Builder:             builder,
	}
}

// countingServer is a deterministic target: fixed latency, fixed payload, and
// a request counter so tests can cross-check the aggregated Count.
type countingServer struct {
	*httptest.Server
	requests atomic.Int64
}

const testPayload = `{"ok":true,"payload":"deterministic"}`

func newCountingServer(t *testing.T, latency time.Duration, status int) *countingServer {
	t.Helper()

	cs := &countingServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.requests.Add(1)
		if latency > 0 {
			time.Sleep(latency)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(testPayload))
	}))
	t.Cleanup(cs.Close)
	return cs
}

// TestRunAggregatesAcrossVUs is the end-to-end proof: four concurrent VUs
// drive a two-step flow for a bounded iteration count, and the merged report
// accounts for every request exactly once, per step.
func TestRunAggregatesAcrossVUs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	const (
		vus        = 4
		iterations = 12
		steps      = 2
	)

	srv := newCountingServer(t, 5*time.Millisecond, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	result, err := Run(t.Context(), Config{
		ScenarioName:  "test-scenario",
		Flow:          flow,
		VUs:           vus,
		MaxIterations: iterations,
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.Summary.Iterations != iterations {
		t.Errorf("Summary.Iterations = %d, want %d", result.Summary.Iterations, iterations)
	}
	if result.Summary.Errors != 0 {
		t.Errorf("Summary.Errors = %d, want 0", result.Summary.Errors)
	}

	wantCount := int64(iterations * steps)
	if result.Report.Total.Count != wantCount {
		t.Errorf("Report.Total.Count = %d, want %d", result.Report.Total.Count, wantCount)
	}
	if got := srv.requests.Load(); got != wantCount {
		t.Errorf("server saw %d requests, report counted %d", got, result.Report.Total.Count)
	}
	if result.Report.Total.RPS <= 0 {
		t.Errorf("Report.Total.RPS = %v, want > 0", result.Report.Total.RPS)
	}
	if result.Report.Total.ErrorCount != 0 {
		t.Errorf("Report.Total.ErrorCount = %d, want 0", result.Report.Total.ErrorCount)
	}

	for _, step := range []string{"StepOne", "StepTwo"} {
		key := loadmetrics.Key{Step: step, StatusClass: loadmetrics.StatusClass2xx}
		stats, ok := result.Report.PerStep[key]
		if !ok {
			t.Fatalf("missing per-step key %+v; got keys %v", key, reportKeys(result.Report))
		}
		if stats.Count != iterations {
			t.Errorf("%s count = %d, want %d", step, stats.Count, iterations)
		}
		if stats.P50 <= 0 {
			t.Errorf("%s p50 = %v, want > 0", step, stats.P50)
		}
	}

	// Concurrency actually happened: four VUs each sleeping 5ms per step
	// cannot have run serially in the wall time we measured.
	serialFloor := time.Duration(iterations*steps) * 5 * time.Millisecond
	if result.Summary.Elapsed >= serialFloor {
		t.Errorf("elapsed %v >= serial floor %v: VUs did not overlap", result.Summary.Elapsed, serialFloor)
	}

	// The by-step fold that feeds the console table sees the same totals.
	byStep, ok := result.ByStep.PerStep[loadmetrics.Key{Step: "StepOne"}]
	if !ok {
		t.Fatalf("ByStep missing StepOne; got keys %v", reportKeys(result.ByStep))
	}
	if byStep.Count != iterations {
		t.Errorf("ByStep StepOne count = %d, want %d", byStep.Count, iterations)
	}
}

func reportKeys(r loadmetrics.Report) []loadmetrics.Key {
	keys := make([]loadmetrics.Key, 0, len(r.PerStep))
	for k := range r.PerStep {
		keys = append(keys, k)
	}
	return keys
}

// TestRunClassifiesServerErrors proves the status taxonomy survives the trip
// from the engine to the report: an all-500 target lands in the 5xx bucket
// and counts as an error, without failing the run.
func TestRunClassifiesServerErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newCountingServer(t, 0, http.StatusInternalServerError)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	const iterations = 6
	result, err := Run(t.Context(), Config{
		Flow:          flow,
		VUs:           2,
		MaxIterations: iterations,
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	key := loadmetrics.Key{Step: "StepOne", StatusClass: loadmetrics.StatusClass5xx}
	stats, ok := result.Report.PerStep[key]
	if !ok {
		t.Fatalf("missing 5xx key %+v; got keys %v", key, reportKeys(result.Report))
	}
	if stats.Count != iterations {
		t.Errorf("5xx count = %d, want %d", stats.Count, iterations)
	}
	if stats.ErrorCount != iterations {
		t.Errorf("5xx ErrorCount = %d, want %d (non-2xx/3xx counts as an error)", stats.ErrorCount, iterations)
	}
	if _, unexpected := result.Report.PerStep[loadmetrics.Key{Step: "StepOne", StatusClass: loadmetrics.StatusClass2xx}]; unexpected {
		t.Error("did not expect a 2xx bucket from an all-500 target")
	}
}

// TestRunRecordsResponseBytes guards the assumption that lets load mode
// account for bytes at all: the persistence side-channel hands the raw
// response to the drain before the node's status is emitted. If the engine
// ever reorders that, this test fails instead of silently zeroing bytes.
func TestRunRecordsResponseBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newCountingServer(t, 0, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	const iterations = 5
	result, err := Run(t.Context(), Config{
		Flow:          flow,
		VUs:           2,
		MaxIterations: iterations,
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	wantBytes := int64(iterations * 2 * len(testPayload))
	if result.Report.Total.Bytes != wantBytes {
		t.Errorf("Report.Total.Bytes = %d, want %d", result.Report.Total.Bytes, wantBytes)
	}
}

// TestRunUsesLeanMode proves lean mode reaches the request nodes end to end:
// StepTwo interpolates StepOne's response body into a header, and what the
// server receives is the lean placeholder rather than the decoded body.
func TestRunUsesLeanMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	var (
		mu       sync.Mutex
		echoed   []string
		observed = func(v string) {
			mu.Lock()
			defer mu.Unlock()
			echoed = append(echoed, v)
		}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Echo-Body"); v != "" {
			observed(v)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testPayload))
	}))
	t.Cleanup(srv.Close)

	yamlDoc := fmt.Sprintf(`
workspace_name: Lean Mode Workspace
flows:
  - name: LeanFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: StepOne
          depends_on: Start
          method: GET
          url: %s/one
      - request:
          name: StepTwo
          depends_on: StepOne
          method: GET
          url: %s/two
          headers:
            X-Echo-Body: '{{ StepOne.response.body }}'
`, srv.URL, srv.URL)

	flow, services := setupFlow(t, yamlDoc, "LeanFlow")

	if _, err := Run(t.Context(), Config{Flow: flow, VUs: 1, MaxIterations: 2}, services, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(echoed) == 0 {
		t.Fatal("StepTwo never sent the echo header")
	}
	for _, v := range echoed {
		if v != nrequest.LeanBodyPlaceholder {
			t.Errorf("echoed body = %q, want lean placeholder %q", v, nrequest.LeanBodyPlaceholder)
		}
	}
}

// TestRunDurationBoundedRunSucceeds pins contract addition #3: Duration is
// passed through RunProfile only. Wrapping the scenario context in a timeout
// would make this run - which completed exactly as configured - return
// context.DeadlineExceeded.
func TestRunDurationBoundedRunSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newCountingServer(t, 0, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	result, err := Run(t.Context(), Config{
		Flow:     flow,
		VUs:      2,
		Duration: 250 * time.Millisecond,
	}, services, nil)
	if err != nil {
		t.Fatalf("duration-bounded run returned error: %v", err)
	}
	if result.Summary.Iterations == 0 {
		t.Error("duration-bounded run completed zero iterations")
	}
	if result.Report.Total.Count == 0 {
		t.Error("duration-bounded run recorded no requests")
	}
}

// TestRunSetupFailureWhenEveryVUFailsFirstIteration pins contract addition
// #9's infra-failure case: an unreachable target on every VU's first
// iteration is a setup failure (exit 1), not a completed run with errors.
func TestRunSetupFailureWhenEveryVUFailsFirstIteration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newCountingServer(t, 0, http.StatusOK)
	baseURL := srv.URL
	srv.Close() // nothing is listening any more

	flow, services := setupFlow(t, twoStepFlowYAML(baseURL), "LoadFlow")

	_, err := Run(t.Context(), Config{
		Flow:          flow,
		VUs:           2,
		MaxIterations: 4,
	}, services, nil)
	if err == nil {
		t.Fatal("expected an unreachable target to be reported as a setup failure")
	}
	if !strings.Contains(err.Error(), "first iteration") {
		t.Errorf("error %q does not explain that every VU failed its first iteration", err)
	}
}

// TestRunErrorsDoNotFailACompletedRun is #9's other half: once at least one
// VU gets going, request-level failures are data, not a run failure.
func TestRunErrorsDoNotFailACompletedRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First two requests succeed, everything after is a hard failure.
		if n.Add(1) > 2 {
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, err := hj.Hijack()
				if err == nil {
					_ = conn.Close()
					return
				}
			}
		}
		_, _ = w.Write([]byte(testPayload))
	}))
	t.Cleanup(srv.Close)

	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	result, err := Run(t.Context(), Config{
		Flow:          flow,
		VUs:           1,
		MaxIterations: 4,
	}, services, nil)
	if err != nil {
		t.Fatalf("a run whose first iteration succeeded must not fail: %v", err)
	}
	if result.Summary.Errors == 0 {
		t.Error("expected later iterations to be counted as errors")
	}
}

func TestRunValidatesConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantSub string
	}{
		{"no flow", Config{VUs: 1, MaxIterations: 1}, "flow"},
		{"zero vus", Config{Flow: &mflow.Flow{}, MaxIterations: 1}, "vus"},
		{"no stop condition", Config{Flow: &mflow.Flow{}, VUs: 1}, "duration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Run(context.Background(), tc.cfg, runner.RunnerServices{}, nil)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.wantSub) {
				t.Errorf("error %q does not mention %q", err, tc.wantSub)
			}
		})
	}
}

// TestConfigFromScenario checks the mload.Scenario -> Config adapter, which
// is what `--scenario <name>` resolves through.
func TestConfigFromScenario(t *testing.T) {
	flow := &mflow.Flow{Name: "Checkout"}
	scenario := mload.Scenario{
		Name:          "checkout-baseline",
		FlowName:      "Checkout",
		Executor:      mload.ExecutorConstantVUs,
		VUs:           7,
		Duration:      45 * time.Second,
		MaxIterations: 900,
	}

	got := ConfigFromScenario(scenario, flow)
	want := Config{
		ScenarioName:  "checkout-baseline",
		Flow:          flow,
		VUs:           7,
		Duration:      45 * time.Second,
		MaxIterations: 900,
	}
	if got != want {
		t.Errorf("ConfigFromScenario() = %+v, want %+v", got, want)
	}
}

// TestVUWorkersAreIsolated pins the reason node graphs are built per VU
// rather than once: each virtual user is a distinct simulated user, so it
// needs its own HTTP client (and therefore its own cookie jar and connection
// pool) and its own metrics aggregator.
func TestVUWorkersAreIsolated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newCountingServer(t, 0, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	const vus = 3
	workers, release, err := newWorkers(t.Context(), Config{Flow: flow, VUs: vus, MaxIterations: 1}, services, nil)
	if err != nil {
		t.Fatalf("newWorkers failed: %v", err)
	}
	defer release()

	if len(workers) != vus {
		t.Fatalf("got %d workers, want %d", len(workers), vus)
	}

	seenClients := make(map[any]bool, vus)
	seenAggs := make(map[*loadmetrics.Aggregator]bool, vus)
	seenNodeMaps := make(map[any]bool, vus)
	for _, w := range workers {
		seenClients[w.httpClient] = true
		seenAggs[w.agg] = true
		for id := range w.flowNodeMap {
			seenNodeMaps[w.flowNodeMap[id]] = true
			break
		}
	}
	if len(seenClients) != vus {
		t.Errorf("%d distinct HTTP clients across %d VUs, want %d", len(seenClients), vus, vus)
	}
	if len(seenAggs) != vus {
		t.Errorf("%d distinct aggregators across %d VUs, want %d", len(seenAggs), vus, vus)
	}
	if len(seenNodeMaps) != vus {
		t.Errorf("%d distinct node instances across %d VUs, want %d", len(seenNodeMaps), vus, vus)
	}
}
