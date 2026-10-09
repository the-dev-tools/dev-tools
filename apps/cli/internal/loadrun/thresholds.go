package loadrun

import (
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// ThresholdResult is the verdict of one threshold against a completed run.
type ThresholdResult struct {
	Condition mload.Condition
	// Observed is the measured value, in the condition's Limit units. It is
	// meaningless when HasData is false.
	Observed float64
	// HasData is false when the run recorded no request the threshold could
	// be measured on, e.g. a step that never ran. Such a threshold fails:
	// a gate that cannot be checked must not pass silently.
	HasData bool
	Passed  bool
}

// ObservedText renders the observed value for a report.
func (r ThresholdResult) ObservedText() string {
	if !r.HasData {
		return "no data"
	}
	return r.Condition.FormatObserved(r.Observed)
}

// EvaluateThresholds checks every condition against a by-step report (one
// row per step, status classes folded), in order.
func EvaluateThresholds(conditions []mload.Condition, byStep loadmetrics.Report) []ThresholdResult {
	if len(conditions) == 0 {
		return nil
	}
	results := make([]ThresholdResult, 0, len(conditions))
	for _, cond := range conditions {
		observed, ok := observe(cond, byStep)
		results = append(results, ThresholdResult{
			Condition: cond,
			Observed:  observed,
			HasData:   ok,
			Passed:    ok && cond.Holds(observed),
		})
	}
	return results
}

// observe extracts the statistic cond compares from a by-step report. It
// reports false when there is nothing to measure: no requests in scope.
func observe(cond mload.Condition, byStep loadmetrics.Report) (float64, bool) {
	stats := byStep.Total
	if cond.Step != "" {
		var ok bool
		stats, ok = byStep.PerStep[loadmetrics.Key{Step: cond.Step}]
		if !ok {
			return 0, false
		}
	}
	if stats.Count == 0 {
		return 0, false
	}

	switch cond.Metric {
	case mload.MetricP50:
		return float64(stats.P50), true
	case mload.MetricP90:
		return float64(stats.P90), true
	case mload.MetricP95:
		return float64(stats.P95), true
	case mload.MetricP99:
		return float64(stats.P99), true
	case mload.MetricMax:
		return float64(stats.Max), true
	case mload.MetricErrors:
		return float64(stats.ErrorCount) / float64(stats.Count), true
	case mload.MetricRPS:
		return stats.RPS, true
	}
	return 0, false
}

// failedThresholds lists the failed thresholds for an error message, or ""
// when all passed.
func failedThresholds(results []ThresholdResult) string {
	var failed []string
	for _, r := range results {
		if !r.Passed {
			failed = append(failed, r.Condition.String()+" (observed "+r.ObservedText()+")")
		}
	}
	return strings.Join(failed, ", ")
}
