package reporter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	load_metricsv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/load_metrics/v1"
)

// LoadRunMeta describes the load run that produced a report: which profile
// ran, and what the scheduler actually managed to do.
type LoadRunMeta struct {
	// ScenarioName is the `load:` block entry the profile came from, empty
	// for a run configured entirely from flags.
	ScenarioName string
	FlowName     string
	VUs          int
	// Duration and MaxIterations are the configured stop conditions.
	Duration      time.Duration
	MaxIterations int64
	// Iterations, Errors and Elapsed are what actually happened.
	Iterations int64
	Errors     int64
	Elapsed    time.Duration
	// WorkerVersion identifies the binary that produced the numbers, so
	// baselines from different builds are not silently compared.
	WorkerVersion string
}

// LoadReport is a completed load run in the shape the reporters need:
// metadata, the full (step, status-class) breakdown, and the same data folded
// to one row per step for the console table.
type LoadReport struct {
	Meta   LoadRunMeta
	Report loadmetrics.Report
	ByStep loadmetrics.Report
}

// loadReportSink is implemented by reporters that can render a load report.
// Reporters that cannot (JUnit has no place to put it) simply do not.
type loadReportSink interface {
	SetLoadReport(report *LoadReport)
}

// SetLoadReport hands the load report to every reporter that can use it. It
// must be called before Flush. Reporters that never receive one behave
// exactly as they did before load mode existed.
func (g *ReporterGroup) SetLoadReport(report *LoadReport) {
	for _, reporter := range g.reporters {
		if sink, ok := reporter.(loadReportSink); ok {
			sink.SetLoadReport(report)
		}
	}
}

const (
	// loadTableMinStepWidth keeps the Step column at its published width for
	// ordinary step names; longer names widen it rather than being truncated,
	// since a truncated step name cannot be matched back to the flow.
	loadTableMinStepWidth = 16
	loadTableStatWidth    = 9
	loadTableRPSWidth     = 8
	// loadTableTotalRow is the label of the whole-run row, always printed
	// last.
	loadTableTotalRow = "TOTAL"
)

// FormatLoadTable renders the aggregate console table: one row per step plus
// a TOTAL row, sorted by step name so the output is stable across runs.
func FormatLoadTable(report LoadReport) string {
	steps := make([]string, 0, len(report.ByStep.PerStep))
	stepWidth := loadTableMinStepWidth
	for key := range report.ByStep.PerStep {
		steps = append(steps, key.Step)
		if len(key.Step)+1 > stepWidth {
			stepWidth = len(key.Step) + 1
		}
	}
	sort.Strings(steps)

	var b strings.Builder
	writeRow := func(step, p50, p95, p99, rps, errPct string) {
		fmt.Fprintf(&b, "%-*s%-*s%-*s%-*s%-*s%s\n",
			stepWidth, step,
			loadTableStatWidth, p50,
			loadTableStatWidth, p95,
			loadTableStatWidth, p99,
			loadTableRPSWidth, rps,
			errPct)
	}
	writeStats := func(label string, stats loadmetrics.Stats) {
		writeRow(label,
			formatLoadDuration(stats.P50),
			formatLoadDuration(stats.P95),
			formatLoadDuration(stats.P99),
			fmt.Sprintf("%.1f", stats.RPS),
			fmt.Sprintf("%.1f", errorPercent(stats)))
	}

	writeRow("Step", "p50", "p95", "p99", "RPS", "Err%")
	for _, step := range steps {
		writeStats(step, report.ByStep.PerStep[loadmetrics.Key{Step: step}])
	}
	writeStats(loadTableTotalRow, report.ByStep.Total)

	return b.String()
}

// LoadMetricsScope states what a load run measures, and by extension what it
// keeps memory-flat. It is surfaced in both the console output and the JSON
// report so nobody has to infer the boundary from a suspiciously empty table.
const LoadMetricsScope = "Counts HTTP request steps only; GraphQL, WebSocket and sub-flow steps run but are neither counted nor memory-bounded."

// FormatLoadHeader renders the one-off context lines printed above the table.
// It is deliberately separate so the table itself stays exactly as published.
func FormatLoadHeader(meta LoadRunMeta) string {
	title := "Load Run"
	if meta.ScenarioName != "" {
		title = "Load Run: " + meta.ScenarioName
	}

	stop := make([]string, 0, 2)
	if meta.Duration > 0 {
		stop = append(stop, "Duration: "+meta.Duration.String())
	}
	if meta.MaxIterations > 0 {
		stop = append(stop, fmt.Sprintf("Max iterations: %d", meta.MaxIterations))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n=== %s ===\n", title)
	fmt.Fprintf(&b, "Flow: %s | VUs: %d", meta.FlowName, meta.VUs)
	for _, s := range stop {
		fmt.Fprintf(&b, " | %s", s)
	}
	fmt.Fprintf(&b, "\nIterations: %d | Iteration errors: %d | Elapsed: %s\n",
		meta.Iterations, meta.Errors, formatLoadDuration(meta.Elapsed))
	fmt.Fprintf(&b, "%s\n\n", LoadMetricsScope)
	return b.String()
}

func errorPercent(stats loadmetrics.Stats) float64 {
	if stats.Count == 0 {
		return 0
	}
	return 100 * float64(stats.ErrorCount) / float64(stats.Count)
}

// formatLoadDuration renders a latency compactly enough to fit the table's
// columns: microseconds below a millisecond, whole milliseconds below a
// second, seconds above that.
func formatLoadDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// loadStatusClassToProto maps the Go status classes onto the generated enum.
// loadmetrics.StatusClass is the source of truth for these values; the
// generated LoadStatusClass mirrors it (see api/load-metrics.tsp).
var loadStatusClassToProto = map[loadmetrics.StatusClass]load_metricsv1.LoadStatusClass{
	loadmetrics.StatusClass2xx:     load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_TWO_XX,
	loadmetrics.StatusClass3xx:     load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_THREE_XX,
	loadmetrics.StatusClass4xx:     load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_FOUR_XX,
	loadmetrics.StatusClass5xx:     load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_FIVE_XX,
	loadmetrics.StatusClassError:   load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_ERROR,
	loadmetrics.StatusClassTimeout: load_metricsv1.LoadStatusClass_LOAD_STATUS_CLASS_TIMEOUT,
}

// LoadStatusClassToProto converts an aggregation status class to the wire
// enum. An unrecognized class yields UNSPECIFIED rather than guessing.
func LoadStatusClassToProto(class loadmetrics.StatusClass) load_metricsv1.LoadStatusClass {
	return loadStatusClassToProto[class]
}

// LoadStatusClassFromProto is the inverse of LoadStatusClassToProto.
// UNSPECIFIED, and any value this build does not know, yield "".
func LoadStatusClassFromProto(class load_metricsv1.LoadStatusClass) loadmetrics.StatusClass {
	for goClass, protoClass := range loadStatusClassToProto {
		if protoClass == class {
			return goClass
		}
	}
	return ""
}

// jsonLoadReport is the additive `load_report` object of the JSON report. The
// run's metrics ride in Report as the spec's LoadRunReport, serialized with
// protojson so the CLI's report really is the N=1 case of the same message
// the Phase 2 wire protocol carries.
type jsonLoadReport struct {
	Scenario      string          `json:"scenario,omitempty"`
	Flow          string          `json:"flow"`
	VUs           int             `json:"vus"`
	Duration      string          `json:"duration,omitempty"`
	MaxIterations int64           `json:"max_iterations,omitempty"`
	Iterations    int64           `json:"iterations"`
	Errors        int64           `json:"errors"`
	Elapsed       string          `json:"elapsed"`
	WorkerVersion string          `json:"worker_version"`
	MetricsScope  string          `json:"metrics_scope"`
	Report        json.RawMessage `json:"report"`
}

// jsonLoadDocument is what the JSON reporter writes when a load report is
// present. Without one it keeps writing the bare array of flow results it
// always has, so nothing changes for existing consumers.
type jsonLoadDocument struct {
	Flows      []model.FlowRunResult `json:"flows"`
	LoadReport *jsonLoadReport       `json:"load_report"`
}

func buildJSONLoadReport(report *LoadReport) (*jsonLoadReport, error) {
	proto := loadRunReportProto(report)
	// Deterministic protojson output: the package intentionally randomizes
	// whitespace, so it is normalized back through encoding/json.
	raw, err := protojson.Marshal(proto)
	if err != nil {
		return nil, fmt.Errorf("serializing load report: %w", err)
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, fmt.Errorf("normalizing load report: %w", err)
	}
	compact, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("normalizing load report: %w", err)
	}

	out := &jsonLoadReport{
		Scenario:      report.Meta.ScenarioName,
		Flow:          report.Meta.FlowName,
		VUs:           report.Meta.VUs,
		MaxIterations: report.Meta.MaxIterations,
		Iterations:    report.Meta.Iterations,
		Errors:        report.Meta.Errors,
		Elapsed:       report.Meta.Elapsed.String(),
		WorkerVersion: report.Meta.WorkerVersion,
		MetricsScope:  LoadMetricsScope,
		Report:        compact,
	}
	if report.Meta.Duration > 0 {
		out.Duration = report.Meta.Duration.String()
	}
	return out, nil
}

// loadRunReportProto converts the merged Go report into the generated
// LoadRunReport. Per-step rows are sorted by (step, status class) so the
// serialized report never depends on Go's map iteration order.
//
// Thresholds and the environment fingerprint are left unset: their shapes are
// frozen in Phase 0 but nothing evaluates or collects them until Phase 2.
func loadRunReportProto(report *LoadReport) *load_metricsv1.LoadRunReport {
	keys := make([]loadmetrics.Key, 0, len(report.Report.PerStep))
	for key := range report.Report.PerStep {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Step != keys[j].Step {
			return keys[i].Step < keys[j].Step
		}
		return keys[i].StatusClass < keys[j].StatusClass
	})

	perStep := make([]*load_metricsv1.LoadRunStepStats, 0, len(keys))
	for _, key := range keys {
		stats := report.Report.PerStep[key]
		perStep = append(perStep, &load_metricsv1.LoadRunStepStats{
			Step:        key.Step,
			StatusClass: LoadStatusClassToProto(key.StatusClass),
			Count:       stats.Count,
			ErrorCount:  stats.ErrorCount,
			Bytes:       stats.Bytes,
			P50Us:       stats.P50.Microseconds(),
			P90Us:       stats.P90.Microseconds(),
			P95Us:       stats.P95.Microseconds(),
			P99Us:       stats.P99.Microseconds(),
			MaxUs:       stats.Max.Microseconds(),
			Rps:         float32(stats.RPS),
		})
	}

	return &load_metricsv1.LoadRunReport{
		Total:   loadStatsProto(report.Report.Total),
		PerStep: perStep,
	}
}

func loadStatsProto(stats loadmetrics.Stats) *load_metricsv1.LoadStats {
	return &load_metricsv1.LoadStats{
		Count:      stats.Count,
		ErrorCount: stats.ErrorCount,
		Bytes:      stats.Bytes,
		P50Us:      stats.P50.Microseconds(),
		P90Us:      stats.P90.Microseconds(),
		P95Us:      stats.P95.Microseconds(),
		P99Us:      stats.P99.Microseconds(),
		MaxUs:      stats.Max.Microseconds(),
		Rps:        float32(stats.RPS),
	}
}
