//go:build loadbench_integration

// This file is the actual RPS/percentile matrix: minutes of wall-clock time,
// and its numbers are only as good as the machine that produced them - which
// is exactly why it does not run by default. It is gated by both a build tag
// and an environment variable, following this repo's integration-test
// convention (see e.g. packages/server/pkg/flow/node/nai's
// integration_setup_test.go, which pairs //go:build ai_integration with
// RUN_AI_INTEGRATION_TESTS).
//
// Run it with:
//
//	RUN_LOADBENCH=true go test -tags loadbench_integration \
//	  -run TestLoadBenchMatrix ./apps/cli/test/loadbench/ -v -timeout 900s
//
// It prints a results table (and a per-step breakdown for the chained
// config) to the test log. Nothing in this file writes to
// docs/superpowers/specs/phase0-bench.md automatically - transcribe real
// numbers from the logged output by hand. That is deliberate: the table is
// proof of what ran, not a substitute for a human deciding the numbers are
// sane before they gate anything.
//
// # A note on VUs=50 and ephemeral ports
//
// At VUs=50 against this package's fast (5ms) local target, request
// throughput is high enough (thousands/sec) that packages/server/pkg/httpclient.New()'s
// reliance on http.DefaultTransport's default MaxIdleConnsPerHost=2 causes
// most connections to be closed and redialed rather than reused. On macOS
// that can exhaust the ephemeral port range (49152-65535, ~16k ports, 30s
// TIME_WAIT) within the measured window, surfacing as "dial tcp ...: connect:
// can't assign requested address" transport errors - confirmed with a raw
// net/http repro outside the flow engine entirely, see task-6-report.md.
// This is a real, reproducible property of the CLI's HTTP client under
// sustained high-throughput single-host load on this class of machine, not a
// bug in this harness. Two things keep one cell's port pressure from
// contaminating another's numbers: cells run VUs-major (all VUs=1, then all
// VUs=10, then both VUs=50 cells last) so a high-VU cell can never run
// before a lower one, each cell tears its server and DB down (via t.Run's
// own Cleanup stack) before the next one starts, and every cell is preceded
// by a `cooldown` sleep longer than this machine's TIME_WAIT duration so it
// starts from a genuinely clean ephemeral port range rather than whatever
// the previous cell left draining.
package loadbench_test

import (
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/loadrun"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
)

// benchLatency is the target server's fixed per-request handler latency. It
// stands in for a real backend's floor cost: every RPS number this package
// produces is an upper bound on what the load engine could ever deliver
// against something slower than this.
const benchLatency = 5 * time.Millisecond

// warmupDuration is discarded before every measured window. It exists so
// Go's HTTP connection pool and the scenario's goroutines have a moment to
// settle before the numbers that matter start accumulating - not because
// this in-process target has anything like a JIT or a cache to warm.
const warmupDuration = 1 * time.Second

// cooldown separates cells so one cell's connection churn (see the port
// exhaustion note above) fully drains before the next cell starts dialing.
// It must exceed this machine's TCP TIME_WAIT duration (2*MSL; `sysctl
// net.inet.tcp.msl` read 15000ms here, so TIME_WAIT is ~30s) or the very
// next cell inherits whatever fraction of the ~16k-port ephemeral range the
// previous cell left draining - which is exactly what a first attempt at
// this harness got wrong: a 2s cooldown after a VUs=10 cell (12.5k
// connections opened) left ~10k ports still in TIME_WAIT, so the following
// VUs=50 cell started with almost no headroom and failed 98.5% of its
// requests - a number that measured leftover port pressure, not VUs=50
// itself. 35s clears it with margin.
const cooldown = 35 * time.Second

// benchVULevels is the VUs axis of the matrix, fixed by the task contract.
// Order matters: it is walked outermost (see TestLoadBenchMatrix), so the
// highest tier - the one that provokes ephemeral port exhaustion on this
// target - runs last and cannot contaminate a lower tier's numbers.
var benchVULevels = []int{1, 10, 50}

// benchMatrixConfig is one axis of the config dimension: a named flow
// fixture plus the step names it produces, for the per-step breakdown.
type benchMatrixConfig struct {
	name     string
	flowName string
	yamlDoc  func(baseURL string) string
	steps    []string
}

var benchMatrixConfigs = []benchMatrixConfig{
	{name: "single-get", flowName: "SingleGet", yamlDoc: singleGetFlowYAML, steps: []string{"Step1"}},
	{name: "chained-5-step", flowName: "ChainedFlow", yamlDoc: chainedFlowYAML, steps: chainedFlowSteps},
}

// measureDuration returns how long to run the measured window (after
// warmup) for a given VU level. At fixed 5ms handler latency, one VU can
// issue at most ~200 requests/sec, so VUs=1 needs more wall time than VUs=50
// to accumulate a comparable sample; higher VU levels clear a solid sample
// in a few seconds. Every tier still satisfies the >=5s floor.
func measureDuration(vus int) time.Duration {
	switch {
	case vus <= 1:
		return 10 * time.Second
	case vus <= 10:
		return 8 * time.Second
	default:
		return 6 * time.Second
	}
}

// benchRow is one (config, VUs) cell's result, kept around after the loop so
// the summary table and the sanity check can both read it back.
type benchRow struct {
	config    string
	vus       int
	result    loadrun.Result
	errorRate float64
}

func TestLoadBenchMatrix(t *testing.T) {
	if os.Getenv("RUN_LOADBENCH") != "true" {
		t.Skip("set RUN_LOADBENCH=true to run the RPS/worker benchmark (takes roughly 5 minutes of wall-clock, most of it cooldown - see the `cooldown` doc comment)")
	}

	t.Logf("environment: OS=%s ARCH=%s NumCPU=%d GoVersion=%s LeanMode=on Target=local-in-process(+%s handler latency)",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version(), benchLatency)

	var rows []benchRow

	first := true
	for _, vus := range benchVULevels {
		for _, cfg := range benchMatrixConfigs {
			if first {
				first = false
			} else {
				t.Logf("cooling down %s before %s/VUs=%d so leftover TIME_WAIT sockets from the previous cell drain first",
					cooldown, cfg.name, vus)
				time.Sleep(cooldown)
			}

			t.Run(cfg.name+"/VUs="+strconv.Itoa(vus), func(t *testing.T) {
				target := newBenchTarget(t, benchLatency, false)
				flow, services := setupFlow(t, cfg.yamlDoc(target.URL), cfg.flowName)

				if _, err := loadrun.Run(t.Context(), loadrun.Config{
					Flow: flow, VUs: vus, Duration: warmupDuration,
				}, services, nil); err != nil {
					t.Fatalf("%s VUs=%d warmup failed: %v", cfg.name, vus, err)
				}

				dur := measureDuration(vus)
				result, err := loadrun.Run(t.Context(), loadrun.Config{
					Flow: flow, VUs: vus, Duration: dur,
				}, services, nil)
				if err != nil {
					t.Fatalf("%s VUs=%d measurement failed: %v", cfg.name, vus, err)
				}

				errorRate := 0.0
				if result.Summary.Iterations > 0 {
					errorRate = float64(result.Summary.Errors) / float64(result.Summary.Iterations)
				}

				switch {
				case result.Summary.Errors == 0:
					// expected at every tier
				case vus <= 10:
					// Low concurrency should never fail against this target;
					// treat it as a real problem worth investigating.
					t.Errorf("%s VUs=%d: %d/%d iterations errored (%.1f%%) against a target that should never fail at this concurrency",
						cfg.name, vus, result.Summary.Errors, result.Summary.Iterations, 100*errorRate)
				default:
					// VUs=50: expected to show the ephemeral-port artifact
					// documented at the top of this file. Logged, not
					// failed, so the matrix still produces a full row.
					t.Logf("%s VUs=%d: %d/%d iterations errored (%.1f%%) - see the port-exhaustion note in this file's package doc",
						cfg.name, vus, result.Summary.Errors, result.Summary.Iterations, 100*errorRate)
				}

				for k, s := range result.Report.PerStep {
					if s.ErrorCount > 0 {
						t.Logf("  DIAG key=%+v count=%d errCount=%d p50=%s max=%s", k, s.Count, s.ErrorCount, s.P50, s.Max)
					}
				}

				total := result.Report.Total
				t.Logf("RESULT config=%-15s vus=%-3d requests=%-8d iterations=%-6d elapsed=%-12s RPS=%-9.1f errRate=%-6.1f%% p50=%-10s p95=%-10s p99=%-10s max=%s",
					cfg.name, vus, total.Count, result.Summary.Iterations, result.Summary.Elapsed.Round(time.Millisecond),
					total.RPS, 100*errorRate, total.P50, total.P95, total.P99, total.Max)

				for _, step := range cfg.steps {
					if s, ok := result.ByStep.PerStep[loadmetrics.Key{Step: step}]; ok {
						t.Logf("  step=%-8s count=%-8d p50=%-10s p95=%-10s p99=%s", step, s.Count, s.P50, s.P95, s.P99)
					}
				}

				rows = append(rows, benchRow{config: cfg.name, vus: vus, result: result, errorRate: errorRate})
			})
		}
	}

	logMarkdownTable(t, rows)
	sanityCheckScaling(t, rows)
}

// logMarkdownTable prints the config x VUs -> RPS/p50/p95/p99 table in the
// exact shape it belongs in docs/superpowers/specs/phase0-bench.md, so
// filling in that doc is a transcription of this log, not a re-derivation.
func logMarkdownTable(t *testing.T, rows []benchRow) {
	t.Log("markdown table (paste into phase0-bench.md):")
	t.Log("| Config | VUs | Requests | Iterations | Elapsed | RPS | Error% | P50 | P95 | P99 |")
	t.Log("|---|---|---|---|---|---|---|---|---|---|")
	for _, r := range rows {
		total := r.result.Report.Total
		t.Logf("| %s | %d | %d | %d | %s | %.1f | %.1f%% | %s | %s | %s |",
			r.config, r.vus, total.Count, r.result.Summary.Iterations,
			r.result.Summary.Elapsed.Round(time.Millisecond), total.RPS, 100*r.errorRate, total.P50, total.P95, total.P99)
	}
}

// sanityCheckScaling pins the matrix's own precondition: at a fixed 5ms
// handler latency, RPS must scale with VUs, or these numbers do not measure
// what the doc claims they measure. A flat curve between VUs=1 and VUs=10
// means this harness serialized somewhere - that would be a bug in this
// package, not a finding about the load engine, and it must be fixed before
// any number here is trusted. VUs=50 is deliberately excluded from this
// check: its expected port-exhaustion errors legitimately suppress RPS
// growth on this machine, which the check would otherwise misreport as a
// harness bug.
func sanityCheckScaling(t *testing.T, rows []benchRow) {
	byConfig := make(map[string]map[int]float64)
	for _, r := range rows {
		if byConfig[r.config] == nil {
			byConfig[r.config] = make(map[int]float64)
		}
		byConfig[r.config][r.vus] = r.result.Report.Total.RPS
	}

	for cfg, byVU := range byConfig {
		rps1, rps10 := byVU[1], byVU[10]
		if rps1 <= 0 {
			continue
		}
		const minScaleFactor = 3.0
		if rps10 < rps1*minScaleFactor {
			t.Errorf("%s: RPS at VUs=10 (%.1f) is not substantially above VUs=1 (%.1f, x%.1f) - "+
				"expected at least x%.1f on a %s-latency target; investigate before trusting these numbers",
				cfg, rps10, rps1, rps10/rps1, minScaleFactor, benchLatency)
		} else {
			t.Logf("sanity check ok: %s scaled x%.1f from VUs=1 (%.1f RPS) to VUs=10 (%.1f RPS)",
				cfg, rps10/rps1, rps1, rps10)
		}
	}
}
