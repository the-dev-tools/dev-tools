package yamlflowsimplev2

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

func mustCondition(t *testing.T, expr string) mload.Condition {
	t.Helper()
	c, err := mload.ParseCondition(expr)
	if err != nil {
		t.Fatalf("ParseCondition(%q): %v", expr, err)
	}
	return c
}

func singleScenario(t *testing.T, body string) mload.Scenario {
	t.Helper()
	scenarios, err := convertLoadYAML(t, loadTestFlows+body)
	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 load scenario, got %d", len(scenarios))
	}
	return scenarios[0]
}

func TestLoadBlockRampingVUs(t *testing.T) {
	got := singleScenario(t, `
load:
  - name: ramp
    flow: Checkout Flow
    executor: ramping-vus
    start_vus: 2
    stages:
      - duration: 30s
        target: 20
      - { duration: 1m, target: 20 }
      - { duration: 10s, target: 0 }
    graceful_ramp_down: 5s
    graceful_stop: 10s
`)
	want := mload.Scenario{
		Name:     "ramp",
		FlowName: "Checkout Flow",
		Executor: mload.ExecutorRampingVUs,
		StartVUs: 2,
		Stages: []mload.Stage{
			{Duration: 30 * time.Second, Target: 20},
			{Duration: time.Minute, Target: 20},
			{Duration: 10 * time.Second, Target: 0},
		},
		GracefulRampDown: 5 * time.Second,
		GracefulStop:     10 * time.Second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scenario mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestLoadBlockConstantArrivalRate(t *testing.T) {
	got := singleScenario(t, `
load:
  - name: steady
    flow: Checkout Flow
    executor: constant-arrival-rate
    rate: 50
    time_unit: 1s
    duration: 2m
    pre_allocated_vus: 10
    max_vus: 100
`)
	want := mload.Scenario{
		Name:            "steady",
		FlowName:        "Checkout Flow",
		Executor:        mload.ExecutorConstantArrivalRate,
		Rate:            50,
		TimeUnit:        time.Second,
		Duration:        2 * time.Minute,
		PreAllocatedVUs: 10,
		MaxVUs:          100,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scenario mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestLoadBlockRampingArrivalRate(t *testing.T) {
	got := singleScenario(t, `
load:
  - name: spike
    flow: Checkout Flow
    executor: ramping-arrival-rate
    start_rate: 5
    time_unit: 1m
    stages:
      - { duration: 1m, target: 300 }
      - { duration: 30s, target: 0 }
    pre_allocated_vus: 5
    max_vus: 50
`)
	want := mload.Scenario{
		Name:            "spike",
		FlowName:        "Checkout Flow",
		Executor:        mload.ExecutorRampingArrivalRate,
		StartRate:       5,
		TimeUnit:        time.Minute,
		Stages:          []mload.Stage{{Duration: time.Minute, Target: 300}, {Duration: 30 * time.Second, Target: 0}},
		PreAllocatedVUs: 5,
		MaxVUs:          50,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scenario mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestLoadBlockThinkTime(t *testing.T) {
	fixed := singleScenario(t, `
load:
  - name: fixed
    flow: Checkout Flow
    vus: 1
    iterations: 1
    think_time: 500ms
`)
	if fixed.ThinkTime != (mload.ThinkTime{Min: 500 * time.Millisecond, Max: 500 * time.Millisecond}) {
		t.Errorf("fixed think time = %+v", fixed.ThinkTime)
	}

	ranged := singleScenario(t, `
load:
  - name: ranged
    flow: Checkout Flow
    vus: 1
    iterations: 1
    think_time: { min: 1s, max: 3s }
`)
	if ranged.ThinkTime != (mload.ThinkTime{Min: time.Second, Max: 3 * time.Second}) {
		t.Errorf("ranged think time = %+v", ranged.ThinkTime)
	}
}

func TestLoadBlockThresholdsMapForm(t *testing.T) {
	got := singleScenario(t, `
load:
  - name: gated
    flow: Checkout Flow
    vus: 1
    duration: 10s
    thresholds:
      errors: <1%
      p95: <300ms
      steps:
        PostLogin:
          p95: <500ms
        Browse:
          p99: <=1s
          rps: '>=10'
`)
	want := []mload.Condition{
		mustCondition(t, "p95<300ms"),
		mustCondition(t, "errors<1%"),
		mustCondition(t, "p99(Browse)<=1s"),
		mustCondition(t, "rps(Browse)>=10"),
		mustCondition(t, "p95(PostLogin)<500ms"),
	}
	if !reflect.DeepEqual(got.Thresholds, want) {
		t.Fatalf("thresholds mismatch:\n got: %v\nwant: %v", got.Thresholds, want)
	}
}

func TestLoadBlockThresholdsListForm(t *testing.T) {
	got := singleScenario(t, `
load:
  - name: gated
    flow: Checkout Flow
    vus: 1
    duration: 10s
    thresholds:
      - p95<300ms
      - p95(PostLogin) < 500ms
      - errors<1%
`)
	// Conditions are held in one canonical order whichever form they were
	// written in: run-wide first, then per step by name.
	want := []mload.Condition{
		mustCondition(t, "p95<300ms"),
		mustCondition(t, "errors<1%"),
		mustCondition(t, "p95(PostLogin)<500ms"),
	}
	if !reflect.DeepEqual(got.Thresholds, want) {
		t.Fatalf("thresholds mismatch:\n got: %v\nwant: %v", got.Thresholds, want)
	}
}

func TestLoadBlockAbortRules(t *testing.T) {
	got := singleScenario(t, `
load:
  - name: guarded
    flow: Checkout Flow
    vus: 1
    duration: 10m
    abort:
      - errors>20%
      - when: p95>2s
        window: 1m
        delay: 30s
`)
	want := []mload.AbortRule{
		{Condition: mustCondition(t, "errors>20%")},
		{Condition: mustCondition(t, "p95>2s"), Window: time.Minute, Delay: 30 * time.Second},
	}
	if !reflect.DeepEqual(got.Abort, want) {
		t.Fatalf("abort mismatch:\n got: %+v\nwant: %+v", got.Abort, want)
	}
}

func TestLoadBlockExecutorValidation(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantSubs []string
	}{
		{
			name: "ramping-vus without stages",
			body: `
  - name: s
    flow: Checkout Flow
    executor: ramping-vus
    start_vus: 1
`,
			wantSubs: []string{"stages", "ramping-vus"},
		},
		{
			name: "ramping-vus fractional target",
			body: `
  - name: s
    flow: Checkout Flow
    executor: ramping-vus
    stages: [{ duration: 1s, target: 2.5 }]
`,
			wantSubs: []string{"2.5", "whole"},
		},
		{
			name: "ramping-vus never has a VU",
			body: `
  - name: s
    flow: Checkout Flow
    executor: ramping-vus
    stages: [{ duration: 1s, target: 0 }]
`,
			wantSubs: []string{"at least one VU"},
		},
		{
			name: "ramping-vus zero total duration",
			body: `
  - name: s
    flow: Checkout Flow
    executor: ramping-vus
    stages: [{ duration: 0s, target: 3 }]
`,
			wantSubs: []string{"positive"},
		},
		{
			name: "stage with bad duration",
			body: `
  - name: s
    flow: Checkout Flow
    executor: ramping-vus
    stages: [{ duration: soon, target: 3 }]
`,
			wantSubs: []string{"stage 1", "soon"},
		},
		{
			name: "negative stage target",
			body: `
  - name: s
    flow: Checkout Flow
    executor: ramping-arrival-rate
    pre_allocated_vus: 1
    stages: [{ duration: 1s, target: -3 }]
`,
			wantSubs: []string{"stage 1", "-3"},
		},
		{
			name: "iterations on ramping-vus",
			body: `
  - name: s
    flow: Checkout Flow
    executor: ramping-vus
    iterations: 10
    stages: [{ duration: 1s, target: 3 }]
`,
			wantSubs: []string{"iterations", "ramping-vus", "constant-vus"},
		},
		{
			name: "stages on constant-vus",
			body: `
  - name: s
    flow: Checkout Flow
    vus: 1
    duration: 1s
    stages: [{ duration: 1s, target: 3 }]
`,
			wantSubs: []string{"stages", "constant-vus"},
		},
		{
			name: "vus on arrival rate",
			body: `
  - name: s
    flow: Checkout Flow
    executor: constant-arrival-rate
    vus: 3
    rate: 1
    duration: 1s
    pre_allocated_vus: 1
`,
			wantSubs: []string{"vus", "pre_allocated_vus"},
		},
		{
			name: "constant-arrival-rate without rate",
			body: `
  - name: s
    flow: Checkout Flow
    executor: constant-arrival-rate
    duration: 1s
    pre_allocated_vus: 1
`,
			wantSubs: []string{"rate"},
		},
		{
			name: "constant-arrival-rate without duration",
			body: `
  - name: s
    flow: Checkout Flow
    executor: constant-arrival-rate
    rate: 1
    pre_allocated_vus: 1
`,
			wantSubs: []string{"duration"},
		},
		{
			name: "arrival rate without pre-allocated VUs",
			body: `
  - name: s
    flow: Checkout Flow
    executor: constant-arrival-rate
    rate: 1
    duration: 1s
`,
			wantSubs: []string{"pre_allocated_vus"},
		},
		{
			name: "max_vus below pre-allocated",
			body: `
  - name: s
    flow: Checkout Flow
    executor: constant-arrival-rate
    rate: 1
    duration: 1s
    pre_allocated_vus: 5
    max_vus: 2
`,
			wantSubs: []string{"max_vus", "pre_allocated_vus"},
		},
		{
			name: "bad time unit",
			body: `
  - name: s
    flow: Checkout Flow
    executor: constant-arrival-rate
    rate: 1
    time_unit: second
    duration: 1s
    pre_allocated_vus: 1
`,
			wantSubs: []string{"time_unit", "second"},
		},
		{
			name: "graceful_stop on constant-vus",
			body: `
  - name: s
    flow: Checkout Flow
    vus: 1
    duration: 1s
    graceful_stop: 5s
`,
			wantSubs: []string{"graceful_stop", "constant-vus"},
		},
		{
			name: "think time min above max",
			body: `
  - name: s
    flow: Checkout Flow
    vus: 1
    duration: 1s
    think_time: { min: 3s, max: 1s }
`,
			wantSubs: []string{"think_time", "min"},
		},
		{
			name: "think time not a duration",
			body: `
  - name: s
    flow: Checkout Flow
    vus: 1
    duration: 1s
    think_time: a while
`,
			wantSubs: []string{"think_time", "a while"},
		},
		{
			name: "unknown threshold metric",
			body: `
  - name: s
    flow: Checkout Flow
    vus: 1
    duration: 1s
    thresholds:
      p42: <1s
`,
			wantSubs: []string{"threshold", "p42", `"s"`},
		},
		{
			name: "bad threshold expression",
			body: `
  - name: s
    flow: Checkout Flow
    vus: 1
    duration: 1s
    thresholds: [p95 300ms]
`,
			wantSubs: []string{"threshold", "p95 300ms"},
		},
		{
			name: "bad abort window",
			body: `
  - name: s
    flow: Checkout Flow
    vus: 1
    duration: 1s
    abort:
      - when: errors>5%
        window: forever
`,
			wantSubs: []string{"abort", "forever"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := convertLoadYAML(t, loadTestFlows+"\nload:"+tc.body)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			for _, want := range tc.wantSubs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

const fullLoadBlock = `
load:
  - name: ramp
    flow: Checkout Flow
    executor: ramping-vus
    start_vus: 1
    stages:
      - { duration: 30s, target: 10 }
      - { duration: 30s, target: 0 }
    graceful_ramp_down: 5s
    think_time: { min: 1s, max: 3s }
    thresholds:
      - p95<300ms
      - errors<1%
      - p95(PostLogin)<500ms
    abort:
      - errors>20%
      - when: p95>2s
        window: 1m
        delay: 30s
  - name: steady
    flow: Checkout Flow
    executor: constant-arrival-rate
    rate: 12.5
    time_unit: 1m
    duration: 90s
    pre_allocated_vus: 2
    max_vus: 8
    graceful_stop: 15s
    think_time: 250ms
  - name: spike
    flow: Checkout Flow
    executor: ramping-arrival-rate
    start_rate: 0
    stages: [{ duration: 10s, target: 100 }]
    pre_allocated_vus: 10
`

// TestLoadBlockNewFieldsExportDeterministically extends the round-trip
// guarantee to every field the new executors added.
func TestLoadBlockNewFieldsExportDeterministically(t *testing.T) {
	bundle, err := ConvertSimplifiedYAML([]byte(loadTestFlows+fullLoadBlock), GetDefaultOptions(idwrap.NewNow()))
	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}
	out, err := MarshalSimplifiedYAML(bundle)
	if err != nil {
		t.Fatalf("MarshalSimplifiedYAML failed: %v", err)
	}
	got := string(out)

	for _, want := range []string{
		"executor: ramping-vus",
		"start_vus: 1",
		"graceful_ramp_down: 5s",
		"min: 1s",
		"max: 3s",
		"p95: <300ms",
		"errors: <1%",
		"PostLogin:",
		"- errors>20%",
		"when: p95>2s",
		"window: 1m0s",
		"delay: 30s",
		"rate: 12.5",
		"time_unit: 1m0s",
		"duration: 1m30s",
		"pre_allocated_vus: 2",
		"max_vus: 8",
		"graceful_stop: 15s",
		"think_time: 250ms",
		"executor: ramping-arrival-rate",
		"target: 100",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("export is missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"vus: 0", "rate: 0", "start_rate: 0", "iterations: 0"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("export contains zero-valued %q:\n%s", unwanted, got)
		}
	}

	reBundle, err := ConvertSimplifiedYAML(out, GetDefaultOptions(idwrap.NewNow()))
	if err != nil {
		t.Fatalf("re-import failed: %v\n%s", err, got)
	}
	if !reflect.DeepEqual(reBundle.LoadScenarios, bundle.LoadScenarios) {
		t.Errorf("re-import changed the scenarios:\nfirst:  %+v\nsecond: %+v", bundle.LoadScenarios, reBundle.LoadScenarios)
	}
	reOut, err := MarshalSimplifiedYAML(reBundle)
	if err != nil {
		t.Fatalf("re-export failed: %v", err)
	}
	if string(reOut) != got {
		t.Errorf("export is not stable:\nfirst:\n%s\nsecond:\n%s", got, reOut)
	}
}

// TestParseLoadScenariosStandalone covers load entries kept outside the flow
// file (e.g. stresseur.load.yaml): the same schema, the same validation.
func TestParseLoadScenariosStandalone(t *testing.T) {
	flows := []string{"Checkout Flow"}

	fromBlock, err := ParseLoadScenarios([]byte(fullLoadBlock), flows)
	if err != nil {
		t.Fatalf("ParseLoadScenarios(load: block) failed: %v", err)
	}
	inline, err := convertLoadYAML(t, loadTestFlows+fullLoadBlock)
	if err != nil {
		t.Fatalf("inline load block failed: %v", err)
	}
	if !reflect.DeepEqual(fromBlock, inline) {
		t.Errorf("a standalone load file must decode exactly like the inline block:\nfile:   %+v\ninline: %+v", fromBlock, inline)
	}

	bare := strings.TrimPrefix(strings.TrimSpace(fullLoadBlock), "load:")
	fromList, err := ParseLoadScenarios([]byte(bare), flows)
	if err != nil {
		t.Fatalf("ParseLoadScenarios(bare list) failed: %v", err)
	}
	if !reflect.DeepEqual(fromList, inline) {
		t.Errorf("a bare list must decode like the load: block")
	}

	_, err = ParseLoadScenarios([]byte(fullLoadBlock), []string{"Other Flow"})
	if err == nil || !strings.Contains(err.Error(), "Other Flow") {
		t.Errorf("unknown flow error = %v, want it to name the known flows", err)
	}

	_, err = ParseLoadScenarios([]byte("name: just a map"), flows)
	if err == nil {
		t.Error("a document that is neither a list nor a load: block must be rejected")
	}
}
