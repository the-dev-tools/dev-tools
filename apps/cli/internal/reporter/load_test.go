package reporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	load_metricsv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/load_metrics/v1"
)

func sampleLoadReport() LoadReport {
	return LoadReport{
		Meta: LoadRunMeta{
			ScenarioName:  "checkout-baseline",
			FlowName:      "Checkout",
			VUs:           4,
			Duration:      30 * time.Second,
			Iterations:    512,
			Errors:        3,
			Elapsed:       30 * time.Second,
			WorkerVersion: "v9.9.9",
		},
		Report: loadmetrics.Report{
			Total: loadmetrics.Stats{Count: 1024, ErrorCount: 1, Bytes: 4096, P50: 95 * time.Millisecond, RPS: 285.14},
			PerStep: map[loadmetrics.Key]loadmetrics.Stats{
				{Step: "CreateOrder", StatusClass: loadmetrics.StatusClass2xx}: {
					Count: 511, Bytes: 2048, P50: 120 * time.Millisecond, RPS: 141.9,
				},
				{Step: "CreateOrder", StatusClass: loadmetrics.StatusClass5xx}: {
					Count: 1, ErrorCount: 1, P50: 5 * time.Millisecond, RPS: 0.1,
				},
			},
		},
		ByStep: loadmetrics.Report{
			Total: loadmetrics.Stats{
				Count: 1024, ErrorCount: 1,
				P50: 95 * time.Millisecond, P95: 290 * time.Millisecond, P99: 470 * time.Millisecond,
				RPS: 285.14,
			},
			PerStep: map[loadmetrics.Key]loadmetrics.Stats{
				{Step: "CreateOrder"}: {
					Count: 512, ErrorCount: 1,
					P50: 120 * time.Millisecond, P95: 310 * time.Millisecond, P99: 480 * time.Millisecond,
					RPS: 142.0,
				},
				{Step: "ConfirmOrder"}: {
					Count: 512,
					P50: 70 * time.Millisecond, P95: 200 * time.Millisecond, P99: 300 * time.Millisecond,
					RPS: 143.1,
				},
			},
		},
	}
}

// TestFormatLoadTable pins the aggregate table's exact shape - column
// headings, widths, ordering and rounding - because it is a published output
// format, not incidental formatting.
func TestFormatLoadTable(t *testing.T) {
	got := FormatLoadTable(sampleLoadReport())

	want := "" +
		"Step            p50      p95      p99      RPS     Err%\n" +
		"ConfirmOrder    70ms     200ms    300ms    143.1   0.0\n" +
		"CreateOrder     120ms    310ms    480ms    142.0   0.2\n" +
		"TOTAL           95ms     290ms    470ms    285.1   0.1\n"

	if got != want {
		t.Errorf("table mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestFormatLoadTableWidensForLongStepNames keeps the columns aligned when a
// step name outgrows the default column, instead of truncating it.
func TestFormatLoadTableWidensForLongStepNames(t *testing.T) {
	report := LoadReport{
		ByStep: loadmetrics.Report{
			Total: loadmetrics.Stats{Count: 1, P50: time.Millisecond, RPS: 1},
			PerStep: map[loadmetrics.Key]loadmetrics.Stats{
				{Step: "AnExtremelyLongStepNameIndeed"}: {Count: 1, P50: time.Millisecond, RPS: 1},
			},
		},
	}

	got := FormatLoadTable(report)
	want := "" +
		"Step                          p50      p95      p99      RPS     Err%\n" +
		"AnExtremelyLongStepNameIndeed 1ms      0s       0s       1.0     0.0\n" +
		"TOTAL                         1ms      0s       0s       1.0     0.0\n"

	if got != want {
		t.Errorf("table mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestFormatLoadHeader covers the context block printed above the table,
// including the scope note that tells the reader which steps the numbers
// actually cover.
func TestFormatLoadHeader(t *testing.T) {
	got := FormatLoadHeader(sampleLoadReport().Meta)

	want := "" +
		"\n=== Load Run: checkout-baseline ===\n" +
		"Flow: Checkout | VUs: 4 | Duration: 30s\n" +
		"Iterations: 512 | Iteration errors: 3 | Elapsed: 30.00s\n" +
		LoadMetricsScope + "\n\n"

	if got != want {
		t.Errorf("header mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatLoadHeaderWithoutScenario(t *testing.T) {
	got := FormatLoadHeader(LoadRunMeta{FlowName: "Solo", VUs: 2, MaxIterations: 50, Iterations: 50})

	want := "" +
		"\n=== Load Run ===\n" +
		"Flow: Solo | VUs: 2 | Max iterations: 50\n" +
		"Iterations: 50 | Iteration errors: 0 | Elapsed: 0s\n" +
		LoadMetricsScope + "\n\n"

	if got != want {
		t.Errorf("header mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLoadMetricsScopeNamesTheUncoveredNodes pins the substance of the note
// rather than its wording, so the promise cannot quietly narrow.
func TestLoadMetricsScopeNamesTheUncoveredNodes(t *testing.T) {
	for _, want := range []string{"HTTP request", "GraphQL", "WebSocket", "sub-flow"} {
		if !strings.Contains(LoadMetricsScope, want) {
			t.Errorf("LoadMetricsScope %q does not mention %q", LoadMetricsScope, want)
		}
	}
}

func TestFormatLoadDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{443 * time.Microsecond, "443µs"},
		{time.Millisecond, "1ms"},
		{120 * time.Millisecond, "120ms"},
		{1500 * time.Millisecond, "1.50s"},
		{2 * time.Minute, "120.00s"},
	}
	for _, tc := range cases {
		if got := formatLoadDuration(tc.in); got != tc.want {
			t.Errorf("formatLoadDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestLoadStatusClassProtoRoundTrip is the guard on the Go <-> generated-proto
// status class mapping. loadmetrics.StatusClass is the source of truth; if
// either side grows a value without the other, this fails.
func TestLoadStatusClassProtoRoundTrip(t *testing.T) {
	all := []loadmetrics.StatusClass{
		loadmetrics.StatusClass2xx,
		loadmetrics.StatusClass3xx,
		loadmetrics.StatusClass4xx,
		loadmetrics.StatusClass5xx,
		loadmetrics.StatusClassError,
		loadmetrics.StatusClassTimeout,
	}

	seen := make(map[load_metricsv1.LoadStatusClass]bool, len(all))
	for _, class := range all {
		proto := LoadStatusClassToProto(class)
		if proto == load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_UNSPECIFIED {
			t.Errorf("StatusClass %q maps to UNSPECIFIED", class)
		}
		if seen[proto] {
			t.Errorf("StatusClass %q collides with an earlier class on %v", class, proto)
		}
		seen[proto] = true

		if back := LoadStatusClassFromProto(proto); back != class {
			t.Errorf("round trip of %q produced %q", class, back)
		}
	}

	// Every enum value the generated code declares, except UNSPECIFIED, must
	// be reachable - otherwise the spec has a class Go cannot produce.
	for value, name := range load_metricsv1.LoadStatusClass_name {
		enum := load_metricsv1.LoadStatusClass(value)
		if enum == load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_UNSPECIFIED {
			continue
		}
		if !seen[enum] {
			t.Errorf("generated enum %s (%d) has no loadmetrics.StatusClass", name, value)
		}
	}

	if got := LoadStatusClassToProto("not-a-class"); got != load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_UNSPECIFIED {
		t.Errorf("unknown class mapped to %v, want UNSPECIFIED", got)
	}
	if got := LoadStatusClassFromProto(load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_UNSPECIFIED); got != "" {
		t.Errorf("UNSPECIFIED mapped to %q, want empty", got)
	}
}

// TestJSONReporterWithoutLoadReportIsUnchanged is the zero-default-change
// guard: with no load flags the JSON report is still a bare array of flow
// results, exactly as it has always been.
func TestJSONReporterWithoutLoadReportIsUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	rep := newJSONReporter(path)
	rep.HandleFlowResult(model.FlowRunResult{FlowName: "FlowA", Status: "success"})
	if err := rep.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var results []model.FlowRunResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("report is not a bare array of flow results: %v\n%s", err, data)
	}
	if len(results) != 1 || results[0].FlowName != "FlowA" {
		t.Errorf("unexpected results: %+v", results)
	}
}

// TestJSONReporterWithLoadReport pins the additive load_report object: run
// metadata plus the spec's LoadRunReport, with status classes carried as the
// generated enum's canonical names.
func TestJSONReporterWithLoadReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	group, err := NewReporterGroup([]ReportSpec{{Format: ReportFormatJSON, Path: path}}, ReporterOptions{})
	if err != nil {
		t.Fatalf("NewReporterGroup failed: %v", err)
	}
	group.HandleFlowResult(model.FlowRunResult{FlowName: "Checkout", Status: "success"})

	loadReport := sampleLoadReport()
	group.SetLoadReport(&loadReport)
	if err := group.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	var doc struct {
		Flows      []model.FlowRunResult `json:"flows"`
		LoadReport *struct {
			Scenario      string `json:"scenario"`
			Flow          string `json:"flow"`
			VUs           int    `json:"vus"`
			Duration      string `json:"duration"`
			Iterations    int64  `json:"iterations"`
			Errors        int64  `json:"errors"`
			Elapsed       string `json:"elapsed"`
			WorkerVersion string `json:"worker_version"`
			Report        struct {
				Total struct {
					Count string `json:"count"`
					Rps   float64 `json:"rps"`
				} `json:"total"`
				PerStep []struct {
					Step        string `json:"step"`
					StatusClass string `json:"statusClass"`
					Count       string `json:"count"`
				} `json:"perStep"`
			} `json:"report"`
		} `json:"load_report"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, data)
	}

	if len(doc.Flows) != 1 || doc.Flows[0].FlowName != "Checkout" {
		t.Errorf("flow results not carried through: %+v", doc.Flows)
	}
	if doc.LoadReport == nil {
		t.Fatalf("load_report missing:\n%s", data)
	}
	if doc.LoadReport.Scenario != "checkout-baseline" {
		t.Errorf("scenario = %q", doc.LoadReport.Scenario)
	}
	if doc.LoadReport.Flow != "Checkout" {
		t.Errorf("flow = %q", doc.LoadReport.Flow)
	}
	if doc.LoadReport.VUs != 4 {
		t.Errorf("vus = %d", doc.LoadReport.VUs)
	}
	if doc.LoadReport.Duration != "30s" {
		t.Errorf("duration = %q", doc.LoadReport.Duration)
	}
	if doc.LoadReport.Iterations != 512 {
		t.Errorf("iterations = %d", doc.LoadReport.Iterations)
	}
	if doc.LoadReport.Errors != 3 {
		t.Errorf("errors = %d", doc.LoadReport.Errors)
	}
	if doc.LoadReport.WorkerVersion != "v9.9.9" {
		t.Errorf("worker_version = %q", doc.LoadReport.WorkerVersion)
	}
	if doc.LoadReport.Report.Total.Count != "1024" {
		t.Errorf("total.count = %q, want \"1024\"", doc.LoadReport.Report.Total.Count)
	}
	if len(doc.LoadReport.Report.PerStep) != 2 {
		t.Fatalf("per-step rows = %d, want 2: %s", len(doc.LoadReport.Report.PerStep), data)
	}
	// Sorted for determinism: 2xx before 5xx for the same step.
	first := doc.LoadReport.Report.PerStep[0]
	if first.Step != "CreateOrder" || first.StatusClass != "LOAD_STATUS_CLASS_TWO_XX" || first.Count != "511" {
		t.Errorf("first per-step row = %+v", first)
	}
	second := doc.LoadReport.Report.PerStep[1]
	if second.StatusClass != "LOAD_STATUS_CLASS_FIVE_XX" {
		t.Errorf("second per-step row = %+v", second)
	}
}

// TestJSONLoadReportIsDeterministic guards against map-iteration order
// leaking into the file: two serializations of the same report are identical.
func TestJSONLoadReportIsDeterministic(t *testing.T) {
	loadReport := sampleLoadReport()

	var first []byte
	for i := range 5 {
		path := filepath.Join(t.TempDir(), "report.json")
		rep := newJSONReporter(path)
		rep.HandleFlowResult(model.FlowRunResult{FlowName: "Checkout"})
		if setter, ok := rep.(loadReportSink); ok {
			setter.SetLoadReport(&loadReport)
		} else {
			t.Fatal("json reporter does not accept a load report")
		}
		if err := rep.Flush(); err != nil {
			t.Fatalf("Flush failed: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read report: %v", err)
		}
		if i == 0 {
			first = data
			continue
		}
		if string(data) != string(first) {
			t.Fatalf("serialization %d differs:\n%s\n---\n%s", i, first, data)
		}
	}
}
