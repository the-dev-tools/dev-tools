package loadrun

import (
	"strings"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

func testFlows() []mflow.Flow {
	return []mflow.Flow{{Name: "Checkout"}, {Name: "Browse"}}
}

func testScenarios() []mload.Scenario {
	return []mload.Scenario{
		{Name: "checkout-baseline", FlowName: "Checkout", Executor: mload.ExecutorConstantVUs, VUs: 10, Duration: 30 * time.Second},
		{Name: "browse-smoke", FlowName: "Browse", Executor: mload.ExecutorConstantVUs, VUs: 1, MaxIterations: 25},
	}
}

func TestOptionsEnabled(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want bool
	}{
		{"nothing set", Options{}, false},
		{"scenario", Options{Scenario: "x"}, true},
		{"vus", Options{VUs: 4}, true},
		{"duration alone does not enable by value", Options{Duration: time.Second}, false},
		{"iterations alone does not enable by value", Options{Iterations: 10}, false},
		{"explicitly requested with no values", Options{Requested: true}, true},
		{"explicit zero vus is still a load run", Options{Requested: true, VUs: 0}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.opts.Enabled(); got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveConfigFromScenario(t *testing.T) {
	flows := testFlows()
	cfg, err := ResolveConfig(Options{Scenario: "checkout-baseline"}, testScenarios(), flows, "")
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}
	if cfg.ScenarioName != "checkout-baseline" {
		t.Errorf("ScenarioName = %q", cfg.ScenarioName)
	}
	if cfg.Flow == nil || cfg.Flow.Name != "Checkout" {
		t.Fatalf("Flow = %+v, want the Checkout flow", cfg.Flow)
	}
	if cfg.Flow != &flows[0] {
		t.Error("expected the resolved flow to point at the caller's slice entry")
	}
	if cfg.VUs != 10 || cfg.Duration != 30*time.Second {
		t.Errorf("profile not carried through: %+v", cfg)
	}
}

func TestResolveConfigUnknownScenario(t *testing.T) {
	_, err := ResolveConfig(Options{Scenario: "nope"}, testScenarios(), testFlows(), "")
	if err == nil {
		t.Fatal("expected an error for an unknown scenario")
	}
	for _, want := range []string{"nope", "checkout-baseline", "browse-smoke"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveConfigScenarioWithoutLoadBlock(t *testing.T) {
	_, err := ResolveConfig(Options{Scenario: "anything"}, nil, testFlows(), "")
	if err == nil {
		t.Fatal("expected an error when the file has no load block")
	}
	for _, want := range []string{"anything", "load:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveConfigScenarioNamesMissingFlow(t *testing.T) {
	scenarios := []mload.Scenario{{Name: "orphan", FlowName: "Vanished", VUs: 1, MaxIterations: 1}}
	_, err := ResolveConfig(Options{Scenario: "orphan"}, scenarios, testFlows(), "")
	if err == nil {
		t.Fatal("expected an error when the scenario's flow is not present")
	}
	for _, want := range []string{"orphan", "Vanished", "Checkout", "Browse"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveConfigFromFlags(t *testing.T) {
	cfg, err := ResolveConfig(
		Options{VUs: 5, Duration: 90 * time.Second, Iterations: 200},
		nil, testFlows(), "Browse")
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}
	if cfg.ScenarioName != "" {
		t.Errorf("ScenarioName = %q, want empty for a flag-driven run", cfg.ScenarioName)
	}
	if cfg.Flow == nil || cfg.Flow.Name != "Browse" {
		t.Fatalf("Flow = %+v", cfg.Flow)
	}
	if cfg.VUs != 5 || cfg.Duration != 90*time.Second || cfg.MaxIterations != 200 {
		t.Errorf("profile not carried through: %+v", cfg)
	}
}

func TestResolveConfigFromFlagsPicksTheOnlyFlow(t *testing.T) {
	flows := []mflow.Flow{{Name: "Solo"}}
	cfg, err := ResolveConfig(Options{VUs: 2, Iterations: 4}, nil, flows, "")
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}
	if cfg.Flow == nil || cfg.Flow.Name != "Solo" {
		t.Fatalf("Flow = %+v, want the single flow", cfg.Flow)
	}
}

func TestResolveConfigFromFlagsAmbiguousFlow(t *testing.T) {
	_, err := ResolveConfig(Options{VUs: 2, Iterations: 4}, nil, testFlows(), "")
	if err == nil {
		t.Fatal("expected an error when the flow to load-test is ambiguous")
	}
	for _, want := range []string{"Checkout", "Browse", "--scenario"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveConfigFromFlagsUnknownFlow(t *testing.T) {
	_, err := ResolveConfig(Options{VUs: 2, Iterations: 4}, nil, testFlows(), "Ghost")
	if err == nil {
		t.Fatal("expected an error for an unknown flow name")
	}
	for _, want := range []string{"Ghost", "Checkout", "Browse"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveConfigFromFlagsNeedsStopCondition(t *testing.T) {
	_, err := ResolveConfig(Options{VUs: 2}, nil, testFlows(), "Checkout")
	if err == nil {
		t.Fatal("expected an error when neither duration nor iterations is set")
	}
	for _, want := range []string{"--duration", "--iterations"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveConfigRejectsNegativeVUs(t *testing.T) {
	_, err := ResolveConfig(Options{VUs: -1, Iterations: 1}, nil, testFlows(), "Checkout")
	if err == nil {
		t.Fatal("expected an error for negative vus")
	}
	if !strings.Contains(err.Error(), "--vus") {
		t.Errorf("error %q does not mention --vus", err)
	}
}

// TestResolveConfigMissingVUs covers the shapes that reach load mode without
// any virtual users: an explicit --vus 0, and --duration/--iterations passed
// on their own. Both must say what is missing rather than fall back to a
// functional run.
func TestResolveConfigMissingVUs(t *testing.T) {
	cases := map[string]Options{
		"explicit zero":   {Requested: true, VUs: 0, Iterations: 4},
		"duration only":   {Requested: true, Duration: 30 * time.Second},
		"iterations only": {Requested: true, Iterations: 100},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ResolveConfig(opts, nil, testFlows(), "Checkout")
			if err == nil {
				t.Fatal("expected an error when no virtual users were requested")
			}
			for _, want := range []string{"--vus", "--scenario"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}
