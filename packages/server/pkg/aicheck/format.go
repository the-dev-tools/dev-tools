package aicheck

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func scoreText(v float64) string {
	return strconv.FormatFloat(round(v, 2), 'f', -1, 64)
}

func thresholdText(kind string, v float64) string {
	if kind == KindLatency || kind == KindTTFT {
		return FormatMS(v)
	}
	return formatNum(v)
}

// RunLines are one run's check lines (`flow run`):
//
//	✓ AskAssistant latency 812 ms ≤ 4.0 s
//	✗ AskAssistant judge 3 (min 4): States a refund policy not in the docs.
func RunLines(step string, spec *Spec, results []Result) []string {
	var out []string
	for _, r := range results {
		mark := "✓"
		if !r.Passed {
			mark = "✗"
		}
		var line string
		switch {
		case r.Skipped:
			line = fmt.Sprintf("– %s %s skipped: %s", step, r.Kind, r.Reason)
		case r.Kind == KindJudge && r.Score != nil:
			line = fmt.Sprintf("%s %s judge %s (min %s): %s", mark, step, scoreText(*r.Score), formatNum(spec.Judge.MinScore), r.Reason)
		case r.Passed && r.Kind == KindSchema:
			line = fmt.Sprintf("%s %s schema", mark, step)
		default:
			line = fmt.Sprintf("%s %s %s %s", mark, step, r.Kind, r.Reason)
			if !r.Passed && (r.Kind == KindSchema || r.Kind == KindJudge) {
				line = fmt.Sprintf("%s %s %s: %s", mark, step, r.Kind, r.Reason)
			}
		}
		out = append(out, strings.TrimRight(line, " :"))
	}
	return out
}

// SummaryLines are a step's check lines over many runs (`ci`):
//
//	✓ AskAssistant schema 12 of 12
//	✓ AskAssistant latency 12 of 12, p95 1.8 s ≤ 4.0 s
//	✗ AskAssistant judge 9 of 12, score 3.8 (min 4): States a refund policy not in the docs.
func SummaryLines(step string, sums []Summary) []string {
	var out []string
	for _, s := range sums {
		if s.Status == StatusSkipped {
			reason := s.Reason
			if reason == "" {
				reason = "the step did not run"
			}
			out = append(out, fmt.Sprintf("– %s %s skipped: %s", step, s.Kind, reason))
			continue
		}
		mark := "✓"
		if s.Passed < s.Runs {
			mark = "✗"
		}
		line := fmt.Sprintf("%s %s %s %d of %d", mark, step, s.Kind, s.Passed, s.Runs)
		switch s.Kind {
		case KindLatency, KindTTFT:
			line += fmt.Sprintf(", p95 %s ≤ %s", FormatMS(Percentile(s.Values, 95)), FormatMS(*s.Threshold))
		case KindTokens:
			if len(s.Values) > 0 && s.Threshold != nil {
				line += fmt.Sprintf(", max %s ≤ %s", formatNum(Percentile(s.Values, 100)), formatNum(*s.Threshold))
			}
		case KindJudge:
			if s.MeanScore != nil {
				line += fmt.Sprintf(", score %s (min %s)", scoreText(*s.MeanScore), thresholdText(s.Kind, *s.Threshold))
			}
		}
		if s.Skipped > 0 {
			line += fmt.Sprintf(", %d skipped", s.Skipped)
		}
		if s.Passed < s.Runs && s.Reason != "" {
			line += ": " + s.Reason
		}
		if s.Status == StatusFailed {
			line += fmt.Sprintf(" [below %s]", strconv.FormatFloat(round(*s.FailBelow*100, 1), 'f', -1, 64)+"%")
		}
		out = append(out, line)
	}
	return out
}

// StatsLine is the judge's work: "Judge: 4 calls, 8 cached, 1.1 s". "" when nothing was judged.
func StatsLine(s Stats) string {
	if s.Calls == 0 && s.Cached == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("%d call", s.Calls)}
	if s.Calls != 1 {
		parts[0] += "s"
	}
	if s.Cached > 0 {
		parts = append(parts, fmt.Sprintf("%d cached", s.Cached))
	}
	if s.Elapsed > 0 {
		parts = append(parts, FormatMS(float64(s.Elapsed)/float64(time.Millisecond)))
	}
	return "Judge: " + strings.Join(parts, ", ")
}

// Since returns the difference between two stats (one flow's share of a command's).
func (s Stats) Since(before Stats) Stats {
	d := Stats{Calls: s.Calls - before.Calls, Judged: s.Judged - before.Judged, Cached: s.Cached - before.Cached,
		Skipped: s.Skipped - before.Skipped, InputTokens: s.InputTokens - before.InputTokens,
		OutputTokens: s.OutputTokens - before.OutputTokens, Elapsed: s.Elapsed - before.Elapsed}
	if len(s.Latencies) > len(before.Latencies) {
		d.Latencies = append([]time.Duration(nil), s.Latencies[len(before.Latencies):]...)
	}
	return d
}

// ReportOnlyHint is printed once when checks failed but no quality.fail_below makes them count.
const ReportOnlyHint = "Checks report only: add quality: { fail_below: 90% } to the flow file to fail on them."
