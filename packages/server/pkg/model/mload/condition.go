package mload

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Metric names a statistic a Condition compares.
type Metric string

const (
	MetricP50 Metric = "p50"
	MetricP90 Metric = "p90"
	MetricP95 Metric = "p95"
	MetricP99 Metric = "p99"
	MetricMax Metric = "max"
	// MetricErrors is the failed-request ratio (non-2xx/3xx and transport
	// failures over all requests). It is written as a percentage ("1%") or a
	// ratio ("0.01").
	MetricErrors Metric = "errors"
	// MetricRPS is requests per second.
	MetricRPS Metric = "rps"
)

// metricAliases maps accepted spellings onto canonical metrics.
var metricAliases = map[string]Metric{
	"p50":        MetricP50,
	"p90":        MetricP90,
	"p95":        MetricP95,
	"p99":        MetricP99,
	"max":        MetricMax,
	"errors":     MetricErrors,
	"error_rate": MetricErrors,
	"rps":        MetricRPS,
}

// Metrics lists the canonical metrics in the order they are written out, so
// exports and error messages are deterministic.
var Metrics = []Metric{MetricP50, MetricP90, MetricP95, MetricP99, MetricMax, MetricErrors, MetricRPS}

// IsLatency reports whether m is a latency statistic, whose values are
// durations.
func (m Metric) IsLatency() bool {
	switch m {
	case MetricP50, MetricP90, MetricP95, MetricP99, MetricMax:
		return true
	case MetricErrors, MetricRPS:
		return false
	}
	return false
}

// Operator is a comparison.
type Operator string

const (
	OpLess         Operator = "<"
	OpLessEqual    Operator = "<="
	OpGreater      Operator = ">"
	OpGreaterEqual Operator = ">="
)

// Condition is a comparison of one run statistic against a limit, such as
// `p95<300ms`, `p95(PostLogin)<500ms` or `errors<1%`.
//
// The same grammar serves thresholds (a condition that must hold for the run
// to pass) and abort rules (a condition that, once it holds, stops the run).
type Condition struct {
	// Step scopes the condition to one request step; empty means the whole
	// run.
	Step   string
	Metric Metric
	Op     Operator
	// Value is the limit exactly as written ("300ms", "1%"), kept so a
	// condition exports the way it was authored.
	Value string
	// Limit is Value normalized for comparison: nanoseconds for latency
	// metrics, a 0..1 ratio for errors, requests per second for rps.
	Limit float64
}

// String renders the condition in its canonical single-expression form.
func (c Condition) String() string {
	if c.Step != "" {
		return fmt.Sprintf("%s(%s)%s%s", c.Metric, c.Step, c.Op, c.Value)
	}
	return fmt.Sprintf("%s%s%s", c.Metric, c.Op, c.Value)
}

// Holds reports whether observed (in Limit's units) satisfies the condition.
func (c Condition) Holds(observed float64) bool {
	switch c.Op {
	case OpLess:
		return observed < c.Limit
	case OpLessEqual:
		return observed <= c.Limit
	case OpGreater:
		return observed > c.Limit
	case OpGreaterEqual:
		return observed >= c.Limit
	}
	return false
}

// FormatObserved renders an observed value (in Limit's units) for a report:
// a rounded duration for latencies, a percentage for errors, two decimals for
// rps.
func (c Condition) FormatObserved(observed float64) string {
	switch {
	case c.Metric.IsLatency():
		return formatLatency(time.Duration(observed))
	case c.Metric == MetricErrors:
		return strconv.FormatFloat(observed*100, 'f', -1, 64) + "%"
	default:
		return strconv.FormatFloat(observed, 'f', 2, 64)
	}
}

func formatLatency(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return d.Round(time.Microsecond).String()
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	default:
		return d.Round(10 * time.Millisecond).String()
	}
}

// ParseCondition parses a single-expression condition:
//
//	metric[(step)] op value
//
// where metric is one of p50, p90, p95, p99, max, errors (alias error_rate)
// or rps; op is <, <=, > or >=; and value is a Go duration for latency
// metrics, a percentage or 0..1 ratio for errors, and a number for rps.
// Whitespace around the parts is ignored.
func ParseCondition(expr string) (Condition, error) {
	s := strings.TrimSpace(expr)
	if s == "" {
		return Condition{}, errors.New("condition is empty")
	}

	opAt := strings.IndexAny(s, "<>=")
	if opAt < 0 {
		return Condition{}, fmt.Errorf("condition %q has no comparison operator (<, <=, >, >=)", expr)
	}
	head := strings.TrimSpace(s[:opAt])
	rest := s[opAt:]

	metricName, step := head, ""
	if open := strings.Index(head, "("); open >= 0 {
		if !strings.HasSuffix(head, ")") {
			return Condition{}, fmt.Errorf("condition %q: unbalanced \"(\" in %q; scope a step as metric(StepName)", expr, head)
		}
		metricName = strings.TrimSpace(head[:open])
		step = strings.TrimSpace(head[open+1 : len(head)-1])
		if step == "" {
			return Condition{}, fmt.Errorf("condition %q: empty step name in %q", expr, head)
		}
	}

	return parseComparison(expr, step, metricName, rest)
}

// ParseComparison parses the map form of a condition, where the metric and
// optional step come from keys and the comparison from the value:
// ParseComparison("PostLogin", "p95", "<500ms").
func ParseComparison(step, metric, comparison string) (Condition, error) {
	expr := metric + comparison
	if step != "" {
		expr = fmt.Sprintf("%s(%s)%s", metric, step, comparison)
	}
	return parseComparison(expr, step, metric, strings.TrimSpace(comparison))
}

func parseComparison(expr, step, metricName, rest string) (Condition, error) {
	metric, ok := metricAliases[strings.ToLower(strings.TrimSpace(metricName))]
	if !ok {
		names := make([]string, 0, len(Metrics))
		for _, m := range Metrics {
			names = append(names, string(m))
		}
		return Condition{}, fmt.Errorf("condition %q: unknown metric %q (known metrics: %s)",
			expr, metricName, strings.Join(names, ", "))
	}

	var op Operator
	switch {
	case strings.HasPrefix(rest, "<="):
		op = OpLessEqual
	case strings.HasPrefix(rest, ">="):
		op = OpGreaterEqual
	case strings.HasPrefix(rest, "<"):
		op = OpLess
	case strings.HasPrefix(rest, ">"):
		op = OpGreater
	default:
		return Condition{}, fmt.Errorf("condition %q: expected a comparison operator (<, <=, >, >=) before %q", expr, rest)
	}
	value := strings.TrimSpace(rest[len(op):])
	if value == "" {
		return Condition{}, fmt.Errorf("condition %q: missing value after %q", expr, op)
	}

	limit, err := parseLimit(metric, value)
	if err != nil {
		return Condition{}, fmt.Errorf("condition %q: %w", expr, err)
	}

	return Condition{Step: step, Metric: metric, Op: op, Value: value, Limit: limit}, nil
}

func parseLimit(metric Metric, value string) (float64, error) {
	switch {
	case metric.IsLatency():
		d, err := time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("value %q is not a duration (e.g. 300ms, 1.5s)", value)
		}
		if d < 0 {
			return 0, fmt.Errorf("value %q is negative", value)
		}
		return float64(d), nil

	case metric == MetricErrors:
		number, isPercent := strings.CutSuffix(value, "%")
		ratio, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return 0, fmt.Errorf("value %q is not a percentage (e.g. 1%%) or a 0..1 ratio", value)
		}
		if isPercent {
			ratio /= 100
		}
		if ratio < 0 {
			return 0, fmt.Errorf("value %q is negative", value)
		}
		if ratio > 1 {
			return 0, fmt.Errorf("value %q is above 100%%", value)
		}
		return ratio, nil

	default:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("value %q is not a number", value)
		}
		if n < 0 {
			return 0, fmt.Errorf("value %q is negative", value)
		}
		return n, nil
	}
}

// AbortRule stops a run early once Condition holds over the trailing Window.
// Rules are not evaluated before Delay has elapsed, so a cold start does not
// trip them.
type AbortRule struct {
	Condition Condition
	// Window is the trailing period the condition is evaluated over. Zero
	// means DefaultAbortWindow.
	Window time.Duration
	// Delay postpones the first evaluation from the start of the run.
	Delay time.Duration
}

// String renders the rule for a report: "errors>20% over 30s".
func (r AbortRule) String() string {
	window := r.Window
	if window <= 0 {
		window = DefaultAbortWindow
	}
	return fmt.Sprintf("%s over %s", r.Condition, window)
}
