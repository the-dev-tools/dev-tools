package yamlflowsimplev2

import (
	"strings"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

const loadTestFlows = `
workspace_name: Load Test
flows:
  - name: Checkout Flow
    steps:
      - manual_start:
          name: Start
`

func convertLoadYAML(t *testing.T, yamlDoc string) ([]mload.Scenario, error) {
	t.Helper()

	bundle, err := ConvertSimplifiedYAML([]byte(yamlDoc), GetDefaultOptions(idwrap.NewNow()))
	if err != nil {
		return nil, err
	}
	return bundle.LoadScenarios, nil
}

// TestLoadBlockImport pins the shape of the additive `load:` block: every
// field lands on the bundle's scenarios, durations are parsed, and the
// executor defaults to constant-vus when omitted.
func TestLoadBlockImport(t *testing.T) {
	yamlDoc := loadTestFlows + `
load:
  - name: checkout-baseline
    flow: Checkout Flow
    executor: constant-vus
    vus: 10
    duration: 30s
    iterations: 500
`

	scenarios, err := convertLoadYAML(t, yamlDoc)
	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 load scenario, got %d", len(scenarios))
	}

	got := scenarios[0]
	want := mload.Scenario{
		Name:          "checkout-baseline",
		FlowName:      "Checkout Flow",
		Executor:      mload.ExecutorConstantVUs,
		VUs:           10,
		Duration:      30 * time.Second,
		MaxIterations: 500,
	}
	if got != want {
		t.Fatalf("scenario mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestLoadBlockDefaultsExecutorToConstantVUs(t *testing.T) {
	yamlDoc := loadTestFlows + `
load:
  - name: no-executor
    flow: Checkout Flow
    vus: 2
    iterations: 4
`

	scenarios, err := convertLoadYAML(t, yamlDoc)
	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 load scenario, got %d", len(scenarios))
	}
	if scenarios[0].Executor != mload.ExecutorConstantVUs {
		t.Fatalf("executor = %q, want %q", scenarios[0].Executor, mload.ExecutorConstantVUs)
	}
}

// TestLoadBlockRejectsUnsupportedExecutor holds the error message to the
// contract: it must name the offending value, the executors this build
// accepts, and where the rest are coming from.
func TestLoadBlockRejectsUnsupportedExecutor(t *testing.T) {
	for _, executor := range []string{"ramping-vus", "constant-arrival-rate", "nonsense"} {
		t.Run(executor, func(t *testing.T) {
			yamlDoc := loadTestFlows + `
load:
  - name: bad-executor
    flow: Checkout Flow
    executor: ` + executor + `
    vus: 1
    iterations: 1
`

			_, err := convertLoadYAML(t, yamlDoc)
			if err == nil {
				t.Fatalf("expected executor %q to be rejected", executor)
			}
			for _, want := range []string{executor, "constant-vus", "Phase 2"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestLoadBlockRejectsUnknownFlow(t *testing.T) {
	yamlDoc := loadTestFlows + `
load:
  - name: orphan
    flow: Nonexistent Flow
    vus: 1
    iterations: 1
`

	_, err := convertLoadYAML(t, yamlDoc)
	if err == nil {
		t.Fatal("expected unknown flow reference to be rejected")
	}
	for _, want := range []string{"Nonexistent Flow", "Checkout Flow"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestLoadBlockValidation(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantSubs []string
	}{
		{
			name: "missing name",
			body: `
load:
  - flow: Checkout Flow
    vus: 1
    iterations: 1
`,
			wantSubs: []string{"name is required"},
		},
		{
			name: "duplicate name",
			body: `
load:
  - name: dupe
    flow: Checkout Flow
    vus: 1
    iterations: 1
  - name: dupe
    flow: Checkout Flow
    vus: 1
    iterations: 1
`,
			wantSubs: []string{"duplicate", "dupe"},
		},
		{
			name: "missing flow",
			body: `
load:
  - name: no-flow
    vus: 1
    iterations: 1
`,
			wantSubs: []string{"flow is required", "no-flow"},
		},
		{
			name: "non-positive vus",
			body: `
load:
  - name: zero-vus
    flow: Checkout Flow
    vus: 0
    iterations: 1
`,
			wantSubs: []string{"vus", "zero-vus"},
		},
		{
			name: "no stop condition",
			body: `
load:
  - name: unbounded
    flow: Checkout Flow
    vus: 1
`,
			wantSubs: []string{"duration", "iterations", "unbounded"},
		},
		{
			name: "unparseable duration",
			body: `
load:
  - name: bad-duration
    flow: Checkout Flow
    vus: 1
    duration: 30 seconds
`,
			wantSubs: []string{"30 seconds", "bad-duration"},
		},
		{
			name: "negative iterations",
			body: `
load:
  - name: negative-iters
    flow: Checkout Flow
    vus: 1
    iterations: -5
`,
			wantSubs: []string{"iterations", "negative-iters"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := convertLoadYAML(t, loadTestFlows+tc.body)
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

// TestLoadBlockExportsDeterministically proves the load block survives the
// import -> export round trip in declaration order, with durations normalized
// to Go's canonical form so re-exporting is a no-op.
func TestLoadBlockExportsDeterministically(t *testing.T) {
	yamlDoc := loadTestFlows + `
load:
  - name: zeta-scenario
    flow: Checkout Flow
    executor: constant-vus
    vus: 4
    duration: 90s
  - name: alpha-scenario
    flow: Checkout Flow
    vus: 2
    iterations: 10
`

	bundle, err := ConvertSimplifiedYAML([]byte(yamlDoc), GetDefaultOptions(idwrap.NewNow()))
	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}

	out, err := MarshalSimplifiedYAML(bundle)
	if err != nil {
		t.Fatalf("MarshalSimplifiedYAML failed: %v", err)
	}

	got := string(out)
	// Declaration order is preserved (not sorted), so zeta comes first.
	zetaAt := strings.Index(got, "zeta-scenario")
	alphaAt := strings.Index(got, "alpha-scenario")
	if zetaAt < 0 || alphaAt < 0 {
		t.Fatalf("exported YAML lost a scenario:\n%s", got)
	}
	if zetaAt > alphaAt {
		t.Errorf("expected declaration order to be preserved, got:\n%s", got)
	}
	// 90s normalizes to Go's canonical 1m30s.
	if !strings.Contains(got, "duration: 1m30s") {
		t.Errorf("expected canonical duration in export, got:\n%s", got)
	}
	// A scenario with no duration must not emit an empty duration key.
	if strings.Contains(got, `duration: ""`) {
		t.Errorf("expected absent duration to be omitted, got:\n%s", got)
	}

	// Re-exporting the exported document is a no-op.
	reBundle, err := ConvertSimplifiedYAML(out, GetDefaultOptions(idwrap.NewNow()))
	if err != nil {
		t.Fatalf("re-import failed: %v", err)
	}
	reOut, err := MarshalSimplifiedYAML(reBundle)
	if err != nil {
		t.Fatalf("re-export failed: %v", err)
	}
	if string(reOut) != got {
		t.Errorf("export is not stable:\nfirst:\n%s\nsecond:\n%s", got, reOut)
	}
}

// TestLoadBlockAbsentEmitsNothing guards the zero-default-behavior-change
// constraint on the YAML side: documents without a load block must export
// exactly as they did before the block existed.
func TestLoadBlockAbsentEmitsNothing(t *testing.T) {
	bundle, err := ConvertSimplifiedYAML([]byte(loadTestFlows), GetDefaultOptions(idwrap.NewNow()))
	if err != nil {
		t.Fatalf("ConvertSimplifiedYAML failed: %v", err)
	}
	if len(bundle.LoadScenarios) != 0 {
		t.Fatalf("expected no load scenarios, got %d", len(bundle.LoadScenarios))
	}

	out, err := MarshalSimplifiedYAML(bundle)
	if err != nil {
		t.Fatalf("MarshalSimplifiedYAML failed: %v", err)
	}
	if strings.Contains(string(out), "load:") {
		t.Errorf("expected no load key in export, got:\n%s", out)
	}
}
