package loadrun

import (
	"strings"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

func TestResolveConfigScalesAScenario(t *testing.T) {
	scenarios := []mload.Scenario{{
		Name: "spike", FlowName: "Checkout", Executor: mload.ExecutorRampingArrivalRate,
		StartRate: 30, Stages: []mload.Stage{{Duration: time.Second, Target: 90}},
		PreAllocatedVUs: 9, MaxVUs: 30,
	}}

	cfg, err := ResolveConfig(Options{Scenario: "spike", VUsScale: 1.0 / 3, RateScale: 1.0 / 3}, scenarios, testFlows(), "")
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}
	if cfg.StartRate != 10 || cfg.Stages[0].Target != 30 {
		t.Errorf("rates not scaled: start %v, stage %v", cfg.StartRate, cfg.Stages[0].Target)
	}
	if cfg.PreAllocatedVUs != 3 || cfg.MaxVUs != 10 {
		t.Errorf("VU pool not scaled: %d/%d", cfg.PreAllocatedVUs, cfg.MaxVUs)
	}
	if scenarios[0].Stages[0].Target != 90 {
		t.Error("scaling must not mutate the caller's scenarios")
	}
}

func TestResolveConfigScalesFlagProfile(t *testing.T) {
	cfg, err := ResolveConfig(Options{VUs: 10, Duration: time.Second, VUsScale: 0.5}, nil, testFlows(), "Browse")
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}
	if cfg.VUs != 5 {
		t.Errorf("VUs = %d, want 5", cfg.VUs)
	}
	if cfg.Executor != mload.ExecutorConstantVUs {
		t.Errorf("Executor = %q, want constant-vus", cfg.Executor)
	}
}

func TestResolveConfigUnsetScalesAreIdentity(t *testing.T) {
	cfg, err := ResolveConfig(Options{Scenario: "checkout-baseline"}, testScenarios(), testFlows(), "")
	if err != nil {
		t.Fatalf("ResolveConfig failed: %v", err)
	}
	if cfg.VUs != 10 {
		t.Errorf("VUs = %d, want 10 unscaled", cfg.VUs)
	}
}

func TestResolveConfigRejectsBadScales(t *testing.T) {
	for _, opts := range []Options{
		{Scenario: "checkout-baseline", VUsScale: -1},
		{Scenario: "checkout-baseline", RateScale: -0.5},
	} {
		_, err := ResolveConfig(opts, testScenarios(), testFlows(), "")
		if err == nil || !strings.Contains(err.Error(), "scale") {
			t.Errorf("ResolveConfig(%+v) error = %v, want a scale error", opts, err)
		}
	}
}
