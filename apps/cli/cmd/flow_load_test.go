package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/loadrun"
)

// loadTarget is the system under test: it counts requests and can be told to
// fail them.
type loadTarget struct {
	*httptest.Server
	requests atomic.Int64
	status   atomic.Int64
}

func newLoadTarget(t *testing.T) *loadTarget {
	t.Helper()
	target := &loadTarget{}
	target.status.Store(http.StatusOK)
	target.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		target.requests.Add(1)
		time.Sleep(2 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(target.status.Load()))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(target.Close)
	return target
}

// framesEndpoint is a fake Stresseur ingest endpoint.
type framesEndpoint struct {
	*httptest.Server
	mu       sync.Mutex
	kinds    []string
	auths    []string
	requests int64 // the final report's request count
}

func newFramesEndpoint(t *testing.T) *framesEndpoint {
	t.Helper()
	ep := &framesEndpoint{}
	ep.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body struct {
			Kind       string `json:"kind"`
			LoadReport struct {
				Requests int64 `json:"requests"`
			} `json:"load_report"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("frames endpoint got invalid JSON: %v", err)
		}
		ep.mu.Lock()
		ep.kinds = append(ep.kinds, body.Kind)
		ep.auths = append(ep.auths, r.Header.Get("Authorization"))
		if body.Kind == "report" {
			ep.requests = body.LoadReport.Requests
		}
		ep.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(ep.Close)
	return ep
}

const loadExecutorsDoc = `
workspace_name: Load Executors
flows:
  - name: Shop
    steps:
      - manual_start:
          name: Start
      - request:
          name: PostLogin
          depends_on: Start
          method: POST
          url: %[1]s/login
      - request:
          name: Browse
          depends_on: PostLogin
          method: GET
          url: %[1]s/items
load:
  - name: constant
    flow: Shop
    vus: 2
    duration: 300ms
    thresholds:
      p95: <10s
      errors: <1%%
  - name: ramping
    flow: Shop
    executor: ramping-vus
    stages:
      - { duration: 150ms, target: 3 }
      - { duration: 150ms, target: 0 }
    think_time: { min: 1ms, max: 5ms }
  - name: arrival
    flow: Shop
    executor: constant-arrival-rate
    rate: 40
    duration: 300ms
    pre_allocated_vus: 2
    max_vus: 6
  - name: ramping-arrival
    flow: Shop
    executor: ramping-arrival-rate
    start_rate: 10
    stages:
      - { duration: 300ms, target: 50 }
    pre_allocated_vus: 2
    max_vus: 6
    thresholds:
      steps:
        PostLogin:
          p99: <10s
  - name: gated
    flow: Shop
    vus: 1
    iterations: 3
    thresholds:
      - errors<1%%
  - name: guarded
    flow: Shop
    vus: 2
    duration: 30s
    abort:
      - when: errors>50%%
        window: 200ms
`

type loadInvocation struct {
	scenario  string
	vusScale  float64
	loadFile  string
	reports   []string
	flowFile  string
	frameTick time.Duration
}

// runLoadCommand drives `flow run` the way the binary does, restoring the
// package-level flag state afterwards.
func runLoadCommand(t *testing.T, inv loadInvocation) error {
	t.Helper()

	prevFormats, prevQuiet, prevOpts := reportFormats, quietMode, loadOpts
	prevLoadFile, prevInterval := loadFile, frameInterval
	t.Cleanup(func() {
		reportFormats, quietMode, loadOpts = prevFormats, prevQuiet, prevOpts
		loadFile, frameInterval = prevLoadFile, prevInterval
	})

	reportFormats = inv.reports
	quietMode = true
	loadOpts = loadrun.Options{Scenario: inv.scenario, VUsScale: inv.vusScale}
	loadFile = inv.loadFile
	frameInterval = inv.frameTick
	if frameInterval == 0 {
		frameInterval = 50 * time.Millisecond
	}

	yamlflowRunCmd.SetContext(context.Background())
	return yamlflowRunCmd.RunE(yamlflowRunCmd, []string{inv.flowFile})
}

func writeLoadDoc(t *testing.T, targetURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shop.yaml")
	if err := os.WriteFile(path, []byte(fmt.Sprintf(loadExecutorsDoc, targetURL)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type jsonLoadReportDoc struct {
	LoadReport struct {
		Scenario          string `json:"scenario"`
		Executor          string `json:"executor"`
		Requests          int64  `json:"requests"`
		Iterations        int64  `json:"iterations"`
		DroppedIterations int64  `json:"dropped_iterations"`
		ThresholdsPassed  *bool  `json:"thresholds_passed"`
		Aborted           string `json:"aborted"`
		Report            struct {
			PerStep []struct {
				Step string `json:"step"`
			} `json:"perStep"`
			Thresholds []struct {
				Expression string `json:"expression"`
				Success    bool   `json:"success"`
			} `json:"thresholds"`
		} `json:"report"`
	} `json:"load_report"`
}

func readLoadReport(t *testing.T, path string) jsonLoadReportDoc {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc jsonLoadReportDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode load report: %v\n%s", err, data)
	}
	return doc
}

// TestFlowRunLoadExecutorsEndToEnd runs every executor through the command,
// with a JSON report and a frames endpoint, against a local target.
func TestFlowRunLoadExecutorsEndToEnd(t *testing.T) {
	t.Setenv("DEVTOOLS_FRAMES_TOKEN", "frames-secret")

	cases := []struct {
		scenario   string
		executor   string
		thresholds int
	}{
		{"constant", "constant-vus", 2},
		{"ramping", "ramping-vus", 0},
		{"arrival", "constant-arrival-rate", 0},
		{"ramping-arrival", "ramping-arrival-rate", 1},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			target := newLoadTarget(t)
			frames := newFramesEndpoint(t)
			flowFile := writeLoadDoc(t, target.URL)
			jsonPath := filepath.Join(t.TempDir(), "report.json")

			err := runLoadCommand(t, loadInvocation{
				scenario: tc.scenario,
				flowFile: flowFile,
				reports:  []string{"json:" + jsonPath, "frames:" + frames.URL},
			})
			if err != nil {
				t.Fatalf("flow run --scenario %s: %v", tc.scenario, err)
			}

			doc := readLoadReport(t, jsonPath).LoadReport
			if doc.Executor != tc.executor || doc.Scenario != tc.scenario {
				t.Errorf("report executor/scenario = %q/%q", doc.Executor, doc.Scenario)
			}
			if doc.Iterations == 0 {
				t.Fatal("no iterations ran")
			}
			if got := target.requests.Load(); doc.Requests != got || doc.Requests != 2*doc.Iterations {
				t.Errorf("requests = %d, iterations = %d, target saw %d", doc.Requests, doc.Iterations, got)
			}
			for _, row := range doc.Report.PerStep {
				if row.Step == "Start" {
					t.Error("the manual_start node is counted as a request")
				}
			}
			if len(doc.Report.Thresholds) != tc.thresholds {
				t.Errorf("thresholds = %+v, want %d", doc.Report.Thresholds, tc.thresholds)
			}
			if tc.thresholds > 0 && (doc.ThresholdsPassed == nil || !*doc.ThresholdsPassed) {
				t.Errorf("thresholds_passed = %v, want true", doc.ThresholdsPassed)
			}

			frames.mu.Lock()
			defer frames.mu.Unlock()
			if len(frames.kinds) < 2 || frames.kinds[len(frames.kinds)-1] != "report" {
				t.Fatalf("frames endpoint received %v, want frames then the report", frames.kinds)
			}
			for _, kind := range frames.kinds[:len(frames.kinds)-1] {
				if kind != "frame" {
					t.Errorf("unexpected post kind %q before the report", kind)
				}
			}
			for _, auth := range frames.auths {
				if auth != "Bearer frames-secret" {
					t.Errorf("Authorization = %q", auth)
				}
			}
			if frames.requests != doc.Requests {
				t.Errorf("final report via frames says %d requests, JSON says %d", frames.requests, doc.Requests)
			}
		})
	}
}

func TestFlowRunLoadThresholdFailureExitCode(t *testing.T) {
	target := newLoadTarget(t)
	target.status.Store(http.StatusInternalServerError)
	jsonPath := filepath.Join(t.TempDir(), "report.json")

	err := runLoadCommand(t, loadInvocation{
		scenario: "gated",
		flowFile: writeLoadDoc(t, target.URL),
		reports:  []string{"json:" + jsonPath},
	})
	if !errors.Is(err, loadrun.ErrThresholdsFailed) {
		t.Fatalf("error = %v, want ErrThresholdsFailed", err)
	}
	if code := ExitCode(err); code != ExitThresholdsFailed {
		t.Errorf("ExitCode = %d, want %d", code, ExitThresholdsFailed)
	}
	doc := readLoadReport(t, jsonPath).LoadReport
	if doc.ThresholdsPassed == nil || *doc.ThresholdsPassed {
		t.Errorf("thresholds_passed = %v, want false", doc.ThresholdsPassed)
	}
}

func TestFlowRunLoadAbortExitCode(t *testing.T) {
	target := newLoadTarget(t)
	target.status.Store(http.StatusServiceUnavailable)
	jsonPath := filepath.Join(t.TempDir(), "report.json")

	start := time.Now()
	err := runLoadCommand(t, loadInvocation{
		scenario: "guarded",
		flowFile: writeLoadDoc(t, target.URL),
		reports:  []string{"json:" + jsonPath},
	})
	if code := ExitCode(err); code != ExitAborted {
		t.Fatalf("ExitCode(%v) = %d, want %d", err, code, ExitAborted)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("a 30s scenario took %v: the abort rule did not stop it", elapsed)
	}
	if doc := readLoadReport(t, jsonPath).LoadReport; !strings.Contains(doc.Aborted, "errors>50%") {
		t.Errorf("aborted = %q", doc.Aborted)
	}
}

func TestFlowRunLoadFile(t *testing.T) {
	target := newLoadTarget(t)
	flowFile := writeLoadDoc(t, target.URL)
	loadPath := filepath.Join(t.TempDir(), "stresseur.load.yaml")
	if err := os.WriteFile(loadPath, []byte(`
load:
  - name: from-file
    flow: Shop
    executor: constant-arrival-rate
    rate: 20
    duration: 200ms
    pre_allocated_vus: 1
    max_vus: 3
`), 0o600); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(t.TempDir(), "report.json")

	if err := runLoadCommand(t, loadInvocation{
		scenario: "from-file",
		flowFile: flowFile,
		loadFile: loadPath,
		reports:  []string{"json:" + jsonPath},
	}); err != nil {
		t.Fatalf("flow run --load-file: %v", err)
	}
	if doc := readLoadReport(t, jsonPath).LoadReport; doc.Scenario != "from-file" || doc.Iterations != 4 {
		t.Errorf("load-file scenario report = %+v, want 4 iterations of from-file", doc)
	}

	// A name defined in both places is ambiguous.
	clash := filepath.Join(t.TempDir(), "clash.yaml")
	if err := os.WriteFile(clash, []byte("- { name: constant, flow: Shop, vus: 1, iterations: 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := runLoadCommand(t, loadInvocation{scenario: "constant", flowFile: flowFile, loadFile: clash, reports: []string{"console"}})
	if err == nil || !strings.Contains(err.Error(), "also defined") {
		t.Errorf("error = %v, want a duplicate-name error", err)
	}
}

func TestFlowRunLoadVUsScale(t *testing.T) {
	target := newLoadTarget(t)
	jsonPath := filepath.Join(t.TempDir(), "report.json")
	if err := runLoadCommand(t, loadInvocation{
		scenario: "gated", // vus: 1, iterations: 3
		vusScale: 2,
		flowFile: writeLoadDoc(t, target.URL),
		reports:  []string{"json:" + jsonPath},
	}); err != nil {
		t.Fatalf("flow run --vus-scale 2: %v", err)
	}
	if doc := readLoadReport(t, jsonPath).LoadReport; doc.Iterations != 6 {
		t.Errorf("iterations = %d, want 6 (the iteration budget scales with the VUs)", doc.Iterations)
	}
}

func TestFlowRunFramesNeedsALoadRun(t *testing.T) {
	target := newLoadTarget(t)
	frames := newFramesEndpoint(t)
	flowFile := writeLoadDoc(t, target.URL)

	prevFormats, prevQuiet := reportFormats, quietMode
	t.Cleanup(func() { reportFormats, quietMode = prevFormats, prevQuiet })
	reportFormats = []string{"frames:" + frames.URL}
	quietMode = true
	yamlflowRunCmd.SetContext(context.Background())

	err := yamlflowRunCmd.RunE(yamlflowRunCmd, []string{flowFile, "Shop"})
	if err == nil || !strings.Contains(err.Error(), "needs a load run") {
		t.Errorf("error = %v, want frames to require a load run", err)
	}
}

func TestExitCode(t *testing.T) {
	both := errors.Join(
		fmt.Errorf("%w: x", loadrun.ErrThresholdsFailed),
		fmt.Errorf("%w: y", loadrun.ErrAborted),
	)
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{errors.New("boom"), 1},
		{fmt.Errorf("%w: p95<1s", loadrun.ErrThresholdsFailed), ExitThresholdsFailed},
		{fmt.Errorf("%w: frames endpoint unreachable", loadrun.ErrAborted), ExitAborted},
		{both, ExitAborted},
	}
	for _, tc := range cases {
		if got := ExitCode(tc.err); got != tc.want {
			t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
