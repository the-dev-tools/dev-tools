package loadrun

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// inflightServer is a target that records its request count and the
// high-water mark of concurrent requests. Its handler honours the request's
// context, so a canceled request ends promptly.
type inflightServer struct {
	*httptest.Server
	requests atomic.Int64
	current  atomic.Int64
	highest  atomic.Int64
}

func newInflightServer(t *testing.T, latency time.Duration, status int) *inflightServer {
	t.Helper()
	s := &inflightServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		now := s.current.Add(1)
		defer s.current.Add(-1)
		for {
			high := s.highest.Load()
			if now <= high || s.highest.CompareAndSwap(high, now) {
				break
			}
		}
		if latency > 0 {
			select {
			case <-time.After(latency):
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(testPayload))
	}))
	t.Cleanup(s.Close)
	return s
}

// scenarioRun imports a two-step flow plus loadBlock and returns the named
// scenario as a runnable Config - the same path `flow run --scenario` takes.
func scenarioRun(t *testing.T, baseURL, loadBlock, scenario string) (Config, func(Config) (Result, error)) {
	t.Helper()
	doc := twoStepFlowYAML(baseURL) + loadBlock
	flow, services := setupFlow(t, doc, "LoadFlow")

	bundle, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(doc), yamlflowsimplev2.GetDefaultOptions(idwrap.NewNow()))
	if err != nil {
		t.Fatalf("convert load block: %v", err)
	}
	for _, s := range bundle.LoadScenarios {
		if s.Name == scenario {
			return ConfigFromScenario(s, flow), func(cfg Config) (Result, error) {
				return Run(t.Context(), cfg, services, nil)
			}
		}
	}
	t.Fatalf("scenario %q not found", scenario)
	return Config{}, nil
}

func assertRequestsAccountedFor(t *testing.T, result Result, srv *inflightServer) {
	t.Helper()
	if got := srv.requests.Load(); got != result.Report.Total.Count {
		t.Errorf("server saw %d requests, report counted %d", got, result.Report.Total.Count)
	}
	if want := result.Summary.Iterations * 2; result.Report.Total.Count != want {
		t.Errorf("report counted %d requests for %d iterations, want %d", result.Report.Total.Count, result.Summary.Iterations, want)
	}
}

func TestRunRampingVUsEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 5*time.Millisecond, http.StatusOK)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: ramp
    flow: LoadFlow
    executor: ramping-vus
    start_vus: 0
    stages:
      - { duration: 200ms, target: 3 }
      - { duration: 200ms, target: 3 }
      - { duration: 100ms, target: 0 }
`, "ramp")

	result, err := run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Summary.Iterations == 0 {
		t.Fatal("no iterations ran")
	}
	if result.Summary.Interrupted != 0 {
		t.Errorf("Interrupted = %d, want 0: 10ms iterations fit any graceful ramp-down", result.Summary.Interrupted)
	}
	assertRequestsAccountedFor(t, result, srv)
	if high := srv.highest.Load(); high > 3 {
		t.Errorf("target saw %d concurrent requests, want <= 3 VUs", high)
	}
	if result.Summary.Elapsed < 450*time.Millisecond || result.Summary.Elapsed > 2*time.Second {
		t.Errorf("Elapsed = %v, want about the 500ms of stages", result.Summary.Elapsed)
	}
}

func TestRunConstantArrivalRateEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	// Each iteration (two 40ms requests) takes over three arrival intervals,
	// so only an open model keeps the schedule: VUs must overlap.
	srv := newInflightServer(t, 40*time.Millisecond, http.StatusOK)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: steady
    flow: LoadFlow
    executor: constant-arrival-rate
    rate: 40
    time_unit: 1s
    duration: 500ms
    pre_allocated_vus: 2
    max_vus: 10
`, "steady")

	result, err := run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Summary.Iterations != 20 {
		t.Errorf("Iterations = %d, want 20 (40/s for 500ms)", result.Summary.Iterations)
	}
	if result.Summary.Dropped != 0 {
		t.Errorf("Dropped = %d, want 0", result.Summary.Dropped)
	}
	assertRequestsAccountedFor(t, result, srv)
	if high := srv.highest.Load(); high < 2 {
		t.Errorf("target saw at most %d concurrent requests: iterations did not overlap", high)
	}
}

func TestRunConstantArrivalRateCountsDroppedIterations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 100*time.Millisecond, http.StatusOK)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: overloaded
    flow: LoadFlow
    executor: constant-arrival-rate
    rate: 50
    duration: 400ms
    pre_allocated_vus: 1
`, "overloaded")

	result, err := run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if total := result.Summary.Iterations + result.Summary.Dropped; total != 20 {
		t.Errorf("Iterations + Dropped = %d + %d, want the 20 scheduled starts", result.Summary.Iterations, result.Summary.Dropped)
	}
	if result.Summary.Dropped < 15 {
		t.Errorf("Dropped = %d, want most starts dropped with a single 200ms VU", result.Summary.Dropped)
	}
	assertRequestsAccountedFor(t, result, srv)
}

func TestRunRampingArrivalRateEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 5*time.Millisecond, http.StatusOK)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: spike
    flow: LoadFlow
    executor: ramping-arrival-rate
    start_rate: 0
    stages:
      - { duration: 300ms, target: 60 }
    pre_allocated_vus: 2
    max_vus: 10
`, "spike")

	result, err := run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// The area under a 0 -> 60/s ramp over 300ms is 9 iterations.
	if result.Summary.Iterations != 9 {
		t.Errorf("Iterations = %d, want 9", result.Summary.Iterations)
	}
	assertRequestsAccountedFor(t, result, srv)
}

// TestRunCountsRequestStepsOnly pins what the summary counts: requests, not
// flow nodes. The manual_start node runs every iteration but is not a
// request, so it never appears in the report.
func TestRunCountsRequestStepsOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 0, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")
	result, err := Run(t.Context(), Config{Flow: flow, VUs: 2, MaxIterations: 6}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	for key := range result.ByStep.PerStep {
		if key.Step == "Start" {
			t.Errorf("the manual_start node was counted as a request step")
		}
	}
	if len(result.ByStep.PerStep) != 2 {
		t.Errorf("report has %d steps, want the 2 request steps", len(result.ByStep.PerStep))
	}
	if result.Report.Total.Count != 12 {
		t.Errorf("Total.Count = %d, want 12 (6 iterations x 2 requests)", result.Report.Total.Count)
	}
}

func TestRunEvaluatesThresholds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 0, http.StatusOK)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: gated
    flow: LoadFlow
    vus: 2
    iterations: 10
    thresholds:
      p95: <10s
      errors: <1%
      steps:
        StepOne:
          rps: '>0'
        Missing:
          p95: <1s
`, "gated")

	result, err := run(cfg)
	if !errors.Is(err, ErrThresholdsFailed) {
		t.Fatalf("error = %v, want ErrThresholdsFailed", err)
	}
	if errors.Is(err, ErrAborted) {
		t.Error("a threshold failure is not an abort")
	}
	if !strings.Contains(err.Error(), "p95(Missing)<1s (observed no data)") {
		t.Errorf("error %q does not name the failing threshold", err)
	}

	verdicts := map[string]ThresholdResult{}
	for _, r := range result.Thresholds {
		verdicts[r.Condition.String()] = r
	}
	if len(verdicts) != 4 {
		t.Fatalf("got %d verdicts, want 4: %+v", len(verdicts), result.Thresholds)
	}
	for _, passing := range []string{"p95<10s", "errors<1%", "rps(StepOne)>0"} {
		if !verdicts[passing].Passed {
			t.Errorf("%s failed (observed %s), want pass", passing, verdicts[passing].ObservedText())
		}
	}
	if v := verdicts["p95(Missing)<1s"]; v.Passed || v.HasData {
		t.Errorf("a threshold on a step with no requests must fail for lack of data, got %+v", v)
	}
	if result.ThresholdsPassed() {
		t.Error("ThresholdsPassed() = true with a failing threshold")
	}
}

func TestRunThresholdsPass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 0, http.StatusOK)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: gated
    flow: LoadFlow
    vus: 1
    iterations: 3
    thresholds: [p99<10s, errors<1%]
`, "gated")

	result, err := run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !result.ThresholdsPassed() || len(result.Thresholds) != 2 {
		t.Errorf("thresholds = %+v, want two passing verdicts", result.Thresholds)
	}
}

func TestRunAbortRuleStopsTheRunEarly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 2*time.Millisecond, http.StatusInternalServerError)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: guarded
    flow: LoadFlow
    vus: 2
    duration: 20s
    abort:
      - when: errors>50%
        window: 200ms
`, "guarded")
	cfg.FrameInterval = 50 * time.Millisecond

	result, err := run(cfg)
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("error = %v, want ErrAborted", err)
	}
	if !strings.Contains(result.AbortReason, "errors>50% over 200ms") || !strings.Contains(result.AbortReason, "100%") {
		t.Errorf("AbortReason = %q, want the rule and the observed rate", result.AbortReason)
	}
	if result.Summary.Elapsed > 5*time.Second {
		t.Errorf("Elapsed = %v: the abort did not stop a 20s run early", result.Summary.Elapsed)
	}
	if result.Report.Total.Count == 0 {
		t.Error("an aborted run still reports what it measured")
	}
}

func TestRunAbortRuleRespectsDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 0, http.StatusInternalServerError)
	cfg, run := scenarioRun(t, srv.URL, `
load:
  - name: guarded
    flow: LoadFlow
    vus: 1
    duration: 300ms
    abort:
      - when: errors>50%
        delay: 10s
`, "guarded")
	cfg.FrameInterval = 50 * time.Millisecond

	result, err := run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v (an abort rule inside its delay must not fire)", err)
	}
	if result.AbortReason != "" {
		t.Errorf("AbortReason = %q, want none", result.AbortReason)
	}
}

func TestRunStreamsIntervalFrames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 2*time.Millisecond, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	var (
		mu     sync.Mutex
		frames []IntervalFrame
	)
	result, err := Run(t.Context(), Config{
		Flow:          flow,
		VUs:           2,
		Duration:      300 * time.Millisecond,
		FrameInterval: 50 * time.Millisecond,
		OnFrame: func(f IntervalFrame) {
			mu.Lock()
			frames = append(frames, f)
			mu.Unlock()
		},
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(frames) < 4 {
		t.Fatalf("got %d frames over a 300ms run at 50ms, want several", len(frames))
	}
	if int64(len(frames)) != result.Frames {
		t.Errorf("Result.Frames = %d, OnFrame saw %d", result.Frames, len(frames))
	}

	var total int64
	all := make([]loadmetrics.Frame, 0, len(frames))
	for i, f := range frames {
		if f.Seq != int64(i) {
			t.Errorf("frame %d has Seq %d", i, f.Seq)
		}
		if f.Final != (i == len(frames)-1) {
			t.Errorf("frame %d Final = %v", i, f.Final)
		}
		for key, entry := range f.Frame.Entries {
			total += entry.Count
			if entry.Hist == nil || entry.Hist.TotalCount() != entry.Count {
				t.Errorf("frame %d entry %v lacks a per-step histogram", i, key)
			}
		}
		all = append(all, f.Frame)
	}
	if total != result.Report.Total.Count {
		t.Errorf("frames carry %d requests, the report %d", total, result.Report.Total.Count)
	}

	// Merging the streamed frames reproduces the final report - which is
	// what lets a collector merge frames from several machines.
	merged := loadmetrics.Merge(all)
	if merged.Total.Count != result.Report.Total.Count || merged.Total.P95 != result.Report.Total.P95 {
		t.Errorf("merged frames = %+v, report = %+v", merged.Total, result.Report.Total)
	}
}

func TestRunStopperEndsTheRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 2*time.Millisecond, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	stopper := NewStopper()
	time.AfterFunc(150*time.Millisecond, func() { stopper.Stop("frames endpoint unreachable for 1m0s") })

	result, err := Run(t.Context(), Config{
		Flow:     flow,
		Executor: mload.ExecutorRampingVUs,
		Stages:   []mload.Stage{{Duration: 20 * time.Second, Target: 2}},
		StartVUs: 2,
		Stopper:  stopper,
	}, services, nil)
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("error = %v, want ErrAborted", err)
	}
	if result.AbortReason != "frames endpoint unreachable for 1m0s" {
		t.Errorf("AbortReason = %q", result.AbortReason)
	}
	if result.Summary.Elapsed > 5*time.Second {
		t.Errorf("Elapsed = %v: the stop did not end the run", result.Summary.Elapsed)
	}
}

func TestRunThinkTimePacesIterations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 0, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	result, err := Run(t.Context(), Config{
		Flow:      flow,
		VUs:       1,
		Duration:  250 * time.Millisecond,
		ThinkTime: mload.ThinkTime{Min: 100 * time.Millisecond, Max: 100 * time.Millisecond},
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if n := result.Summary.Iterations; n < 2 || n > 4 {
		t.Errorf("Iterations = %d, want about 3 with a 100ms think time over 250ms", n)
	}
}

// TestRunDoesNotRecordInterruptedRequests: a request the scheduler cancels
// at the end of a graceful ramp-down measures the generator giving up, not
// the target failing, so it must not show up as an error.
func TestRunDoesNotRecordInterruptedRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 5*time.Second, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")

	result, err := Run(t.Context(), Config{
		Flow:             flow,
		Executor:         mload.ExecutorRampingVUs,
		StartVUs:         2,
		Stages:           []mload.Stage{{Duration: 50 * time.Millisecond, Target: 2}, {Duration: 10 * time.Millisecond, Target: 0}},
		GracefulRampDown: 50 * time.Millisecond,
		GracefulStop:     50 * time.Millisecond,
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v (interrupted iterations are not a setup failure)", err)
	}
	if result.Summary.Interrupted != 2 {
		t.Errorf("Interrupted = %d, want 2", result.Summary.Interrupted)
	}
	if result.Report.Total.Count != 0 || result.Report.Total.ErrorCount != 0 {
		t.Errorf("report recorded %+v for canceled requests, want nothing", result.Report.Total)
	}
}

// TestArrivalRatePoolGrowsLazily: VUs beyond pre_allocated_vus are only
// built when the schedule needs them.
func TestArrivalRatePoolGrowsLazily(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load run in short mode")
	}

	srv := newInflightServer(t, 0, http.StatusOK)
	flow, services := setupFlow(t, twoStepFlowYAML(srv.URL), "LoadFlow")
	cfg := Config{
		Flow:            flow,
		Executor:        mload.ExecutorConstantArrivalRate,
		Rate:            10,
		Duration:        time.Second,
		PreAllocatedVUs: 1,
		MaxVUs:          50,
	}

	pool, err := newWorkerPool(t.Context(), cfg, services, nil)
	if err != nil {
		t.Fatalf("newWorkerPool failed: %v", err)
	}
	defer pool.release()

	if got := len(pool.built()); got != 1 {
		t.Fatalf("built %d workers up front, want the 1 pre-allocated", got)
	}
	if _, err := pool.get(7); err != nil {
		t.Fatalf("get(7): %v", err)
	}
	if got := len(pool.built()); got != 2 {
		t.Errorf("built %d workers after using VU 7, want 2", got)
	}
	if _, err := pool.get(50); err == nil {
		t.Error("get beyond max_vus must fail")
	}
}

func TestConfigFromScenarioCarriesEveryField(t *testing.T) {
	scenario := mload.Scenario{
		Name:             "full",
		Executor:         mload.ExecutorRampingArrivalRate,
		StartRate:        1,
		Stages:           []mload.Stage{{Duration: time.Second, Target: 5}},
		TimeUnit:         time.Minute,
		PreAllocatedVUs:  2,
		MaxVUs:           4,
		GracefulStop:     time.Second,
		GracefulRampDown: 2 * time.Second,
		ThinkTime:        mload.ThinkTime{Min: time.Millisecond, Max: 2 * time.Millisecond},
		Thresholds:       []mload.Condition{{Metric: mload.MetricP95}},
		Abort:            []mload.AbortRule{{Window: time.Second}},
	}
	cfg := ConfigFromScenario(scenario, nil)
	back := fmt.Sprintf("%v %v %v %v %v %v %v %v %v %v %v",
		cfg.Executor, cfg.StartRate, cfg.Stages, cfg.TimeUnit, cfg.PreAllocatedVUs, cfg.MaxVUs,
		cfg.GracefulStop, cfg.GracefulRampDown, cfg.ThinkTime, cfg.Thresholds, cfg.Abort)
	want := fmt.Sprintf("%v %v %v %v %v %v %v %v %v %v %v",
		scenario.Executor, scenario.StartRate, scenario.Stages, scenario.TimeUnit, scenario.PreAllocatedVUs, scenario.MaxVUs,
		scenario.GracefulStop, scenario.GracefulRampDown, scenario.ThinkTime, scenario.Thresholds, scenario.Abort)
	if back != want {
		t.Errorf("ConfigFromScenario dropped a field:\n got %s\nwant %s", back, want)
	}
	if cfg.PoolSize() != 4 {
		t.Errorf("PoolSize() = %d, want max_vus 4", cfg.PoolSize())
	}
}
