package mload

import (
	"strings"
	"testing"
	"time"
)

func TestParseCondition(t *testing.T) {
	cases := []struct {
		expr      string
		step      string
		metric    Metric
		op        Operator
		value     string
		limit     float64
		canonical string
	}{
		{"p95<300ms", "", MetricP95, OpLess, "300ms", float64(300 * time.Millisecond), "p95<300ms"},
		{"p95 < 300ms", "", MetricP95, OpLess, "300ms", float64(300 * time.Millisecond), "p95<300ms"},
		{"p99<=1s", "", MetricP99, OpLessEqual, "1s", float64(time.Second), "p99<=1s"},
		{"max<2s", "", MetricMax, OpLess, "2s", float64(2 * time.Second), "max<2s"},
		{"p50(PostLogin)<50ms", "PostLogin", MetricP50, OpLess, "50ms", float64(50 * time.Millisecond), "p50(PostLogin)<50ms"},
		{"p90( Get Item ) < 1.5s", "Get Item", MetricP90, OpLess, "1.5s", float64(1500 * time.Millisecond), "p90(Get Item)<1.5s"},
		{"errors<1%", "", MetricErrors, OpLess, "1%", 0.01, "errors<1%"},
		{"errors<0.5%", "", MetricErrors, OpLess, "0.5%", 0.005, "errors<0.5%"},
		{"errors<0.02", "", MetricErrors, OpLess, "0.02", 0.02, "errors<0.02"},
		{"error_rate<0.5%", "", MetricErrors, OpLess, "0.5%", 0.005, "errors<0.5%"},
		{"errors>20%", "", MetricErrors, OpGreater, "20%", 0.2, "errors>20%"},
		{"rps>=100", "", MetricRPS, OpGreaterEqual, "100", 100, "rps>=100"},
		{"rps(Search)>12.5", "Search", MetricRPS, OpGreater, "12.5", 12.5, "rps(Search)>12.5"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := ParseCondition(tc.expr)
			if err != nil {
				t.Fatalf("ParseCondition(%q) error = %v", tc.expr, err)
			}
			if got.Step != tc.step || got.Metric != tc.metric || got.Op != tc.op || got.Value != tc.value {
				t.Errorf("ParseCondition(%q) = %+v", tc.expr, got)
			}
			if got.Limit != tc.limit {
				t.Errorf("Limit = %v, want %v", got.Limit, tc.limit)
			}
			if got.String() != tc.canonical {
				t.Errorf("String() = %q, want %q", got.String(), tc.canonical)
			}
		})
	}
}

func TestParseConditionErrors(t *testing.T) {
	cases := map[string][]string{
		"":                {"empty"},
		"p95":             {"operator"},
		"p96<300ms":       {"p96", "p95"},
		"p95<300":         {"300", "duration"},
		"p95<-1s":         {"negative"},
		"errors<abc":      {"abc"},
		"errors<150%":     {"150%"},
		"rps<fast":        {"fast"},
		"p95(<300ms":      {"("},
		"p95()<300ms":     {"step"},
		"p95<":            {"value"},
		"p95=300ms":       {"operator"},
		"latency<300ms":   {"latency"},
		"errors<1%%":      {"1%%"},
		"p95(Login)<<1s":  {"<1s"},
		"p95(Login)x<1s":  {"x"},
		"errors<-5%":      {"negative"},
		"rps>-1":          {"negative"},
		"p95<1s trailing": {"1s trailing"},
	}

	for expr, wantSubs := range cases {
		t.Run(expr, func(t *testing.T) {
			_, err := ParseCondition(expr)
			if err == nil {
				t.Fatalf("ParseCondition(%q) succeeded, want error", expr)
			}
			for _, want := range wantSubs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestParseComparison(t *testing.T) {
	got, err := ParseComparison("PostLogin", "p95", "<500ms")
	if err != nil {
		t.Fatalf("ParseComparison error = %v", err)
	}
	if got.String() != "p95(PostLogin)<500ms" {
		t.Errorf("String() = %q", got.String())
	}

	if _, err := ParseComparison("", "p95", "500ms"); err == nil {
		t.Error("a comparison without an operator must be rejected")
	}
}

func TestConditionHolds(t *testing.T) {
	cond, err := ParseCondition("p95<300ms")
	if err != nil {
		t.Fatal(err)
	}
	if !cond.Holds(float64(299 * time.Millisecond)) {
		t.Error("299ms should satisfy p95<300ms")
	}
	if cond.Holds(float64(300 * time.Millisecond)) {
		t.Error("300ms should not satisfy p95<300ms")
	}

	ge, err := ParseCondition("rps>=10")
	if err != nil {
		t.Fatal(err)
	}
	if !ge.Holds(10) || ge.Holds(9.99) {
		t.Error("rps>=10 compares wrongly")
	}
}

func TestConditionFormatObserved(t *testing.T) {
	latency, _ := ParseCondition("p95<300ms")
	if got := latency.FormatObserved(float64(312 * time.Millisecond)); got != "312ms" {
		t.Errorf("latency observed = %q, want 312ms", got)
	}
	errs, _ := ParseCondition("errors<1%")
	if got := errs.FormatObserved(0.0125); got != "1.25%" {
		t.Errorf("errors observed = %q, want 1.25%%", got)
	}
	rps, _ := ParseCondition("rps>10")
	if got := rps.FormatObserved(12.345); got != "12.35" {
		t.Errorf("rps observed = %q, want 12.35", got)
	}
}
