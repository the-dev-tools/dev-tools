package reporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func arrivalLoadReport() LoadReport {
	report := sampleLoadReport()
	report.Meta.Executor = "constant-arrival-rate"
	report.Meta.Profile = "40 iterations/1s, 2-10 VUs, 30s"
	report.Meta.Dropped = 7
	report.Meta.Interrupted = 1
	report.Meta.Thresholds = []LoadThresholdResult{
		{Expression: "p95<300ms", Passed: true, Observed: "290ms"},
		{Expression: "errors<0.01%", Passed: false, Observed: "0.1%"},
	}
	report.Meta.AbortReason = "abort rule errors>20% over 30s held (observed 25%)"
	return report
}

func TestFormatLoadHeaderOtherExecutors(t *testing.T) {
	got := FormatLoadHeader(arrivalLoadReport().Meta)

	want := "" +
		"\n=== Load Run: checkout-baseline ===\n" +
		"Flow: Checkout | Executor: constant-arrival-rate | 40 iterations/1s, 2-10 VUs, 30s\n" +
		"Iterations: 512 | Iteration errors: 3 | Requests: 1024 | Dropped iterations: 7 | Interrupted: 1 | Elapsed: 30.00s\n" +
		LoadMetricsScope + "\n\n"

	if got != want {
		t.Errorf("header mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatLoadVerdicts(t *testing.T) {
	got := FormatLoadVerdicts(arrivalLoadReport().Meta)

	want := "" +
		"\nThresholds:\n" +
		"  PASS  p95<300ms (observed 290ms)\n" +
		"  FAIL  errors<0.01% (observed 0.1%)\n" +
		"\nAborted: abort rule errors>20% over 30s held (observed 25%)\n"

	if got != want {
		t.Errorf("verdicts mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	if got := FormatLoadVerdicts(sampleLoadReport().Meta); got != "" {
		t.Errorf("a run without thresholds or an abort prints nothing extra, got %q", got)
	}
}

// TestJSONLoadReportNewFieldsAreAdditive checks the new fields land in the
// JSON report and that a constant-vus run without thresholds gains nothing
// beyond the always-present executor and requests counts.
func TestJSONLoadReportNewFieldsAreAdditive(t *testing.T) {
	write := func(report LoadReport) map[string]any {
		t.Helper()
		path := filepath.Join(t.TempDir(), "report.json")
		group, err := NewReporterGroup([]ReportSpec{{Format: ReportFormatJSON, Path: path}}, ReporterOptions{})
		if err != nil {
			t.Fatal(err)
		}
		group.SetLoadReport(&report)
		if err := group.Flush(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			LoadReport map[string]any `json:"load_report"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		return doc.LoadReport
	}

	plain := write(sampleLoadReport())
	for _, absent := range []string{"dropped_iterations", "interrupted_iterations", "thresholds_passed", "aborted"} {
		if _, ok := plain[absent]; ok {
			t.Errorf("constant-vus report without thresholds carries %q", absent)
		}
	}
	if plain["executor"] != "constant-vus" {
		t.Errorf("executor = %v, want constant-vus for a report with none set", plain["executor"])
	}
	if plain["requests"] != float64(1024) {
		t.Errorf("requests = %v, want 1024", plain["requests"])
	}
	if report, _ := plain["report"].(map[string]any); report["thresholds"] != nil {
		t.Errorf("report.thresholds = %v, want absent", report["thresholds"])
	}

	full := write(arrivalLoadReport())
	if full["executor"] != "constant-arrival-rate" {
		t.Errorf("executor = %v", full["executor"])
	}
	if full["dropped_iterations"] != float64(7) || full["interrupted_iterations"] != float64(1) {
		t.Errorf("dropped/interrupted = %v/%v", full["dropped_iterations"], full["interrupted_iterations"])
	}
	if full["thresholds_passed"] != false {
		t.Errorf("thresholds_passed = %v, want false", full["thresholds_passed"])
	}
	if !strings.Contains(full["aborted"].(string), "errors>20%") {
		t.Errorf("aborted = %v", full["aborted"])
	}

	report, _ := full["report"].(map[string]any)
	verdicts, _ := report["thresholds"].([]any)
	if len(verdicts) != 2 {
		t.Fatalf("report.thresholds = %v, want 2 verdicts", report["thresholds"])
	}
	first, _ := verdicts[0].(map[string]any)
	if first["expression"] != "p95<300ms" || first["success"] != true || first["observedValue"] != "290ms" {
		t.Errorf("first verdict = %v", first)
	}
	second, _ := verdicts[1].(map[string]any)
	if second["success"] != nil && second["success"] != false {
		t.Errorf("second verdict = %v, want a failure", second)
	}
}
