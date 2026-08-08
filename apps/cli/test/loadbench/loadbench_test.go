// Package loadbench_test is a self-contained benchmark harness for the CLI's
// load mode (apps/cli/internal/loadrun): it measures sustained RPS against a
// local, in-process HTTP target with a fixed handler latency, so the numbers
// describe the load engine's own overhead ceiling rather than any real
// network or backend.
//
// It has no external network dependency whatsoever - the target server is
// httptest.NewServer, bound to loopback, and every request in this package
// stays on that connection.
//
// Two kinds of test live here, deliberately split by whether they belong in
// the default suite:
//
//   - Fixture correctness (this file). Proves the chained-flow fixture used
//     by the benchmark really chains - step N's request carries step N-1's
//     response value, not a hardcoded one - and that the single-GET fixture
//     hits the target exactly once per iteration. Both run in well under a
//     second and run unconditionally, like any other test in this repo.
//   - The RPS/percentile matrix (integration_loadbench_test.go). Minutes of
//     wall-clock time, hardware-sensitive, gated behind the loadbench_integration
//     build tag and RUN_LOADBENCH=true. See that file for how to run it and
//     where its results end up.
package loadbench_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/common"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/loadrun"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	gqlresolver "github.com/the-dev-tools/dev-tools/packages/server/pkg/graphql/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// benchLatency (the target server's fixed per-request handler latency used by
// the actual benchmark matrix) lives in integration_loadbench_test.go, the
// only place that references it - the fixture-correctness tests in this file
// use zero latency instead (see newBenchTarget's latency parameter): they
// only care about wiring, and zero latency keeps them fast. Declaring it here
// anyway would make it unused whenever this package is built without the
// loadbench_integration tag, i.e. always, outside a benchmark run.

// chainHeader is the response header the /chain endpoint uses to hand its
// issued token to whichever step reads it next. It deliberately has no
// hyphen: {{ }} interpolation runs the path through expr-lang
// (packages/server/pkg/expression), which parses a hyphenated segment as
// subtraction between two identifiers, not as a map key.
const chainHeader = "Chaintoken"

// chainHop is one /chain request as the target server saw it: the token the
// requester claimed as "prev", and the token this response issues in return.
// A correctly wired chained flow produces prev[i] == issued[i-1] for every
// hop after the first.
type chainHop struct {
	prev   string
	issued string
}

// benchTarget is the local, in-process HTTP target every flow in this
// package drives. It has two routes: /single, for the single-GET config, and
// /chain, for the 5-step chained config.
type benchTarget struct {
	*httptest.Server
	requests atomic.Int64

	tokens atomic.Int64

	mu       sync.Mutex
	record   bool
	chainLog []chainHop
}

// newBenchTarget starts a target server. latency is slept inside the handler
// before it responds, on every request. When record is true, every /chain
// request is appended to chainLog (guarded by mu) for a correctness test to
// inspect afterwards; the benchmark matrix always passes record=false so the
// measured path pays no bookkeeping cost beyond the plain atomic counters.
func newBenchTarget(t *testing.T, latency time.Duration, record bool) *benchTarget {
	t.Helper()

	bt := &benchTarget{record: record}

	mux := http.NewServeMux()
	mux.HandleFunc("/single", func(w http.ResponseWriter, r *http.Request) {
		bt.requests.Add(1)
		sleep(latency)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/chain", func(w http.ResponseWriter, r *http.Request) {
		bt.requests.Add(1)
		sleep(latency)

		prev := r.URL.Query().Get("prev")
		issued := fmt.Sprintf("tok-%d", bt.tokens.Add(1))

		if bt.record {
			bt.mu.Lock()
			bt.chainLog = append(bt.chainLog, chainHop{prev: prev, issued: issued})
			bt.mu.Unlock()
		}

		w.Header().Set(chainHeader, issued)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"token":%q}`, issued)
	})

	bt.Server = httptest.NewServer(mux)
	t.Cleanup(bt.Close)
	return bt
}

func sleep(d time.Duration) {
	if d > 0 {
		time.Sleep(d)
	}
}

// hops returns a snapshot of the recorded /chain requests, safe to range
// over after the run that produced them has finished.
func (bt *benchTarget) hops() []chainHop {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	return append([]chainHop(nil), bt.chainLog...)
}

// singleGetFlowYAML is config (a): one request node, no chaining.
func singleGetFlowYAML(baseURL string) string {
	return fmt.Sprintf(`
workspace_name: LoadBench Workspace
flows:
  - name: SingleGet
    steps:
      - manual_start:
          name: Start
      - request:
          name: Step1
          depends_on: Start
          method: GET
          url: %s/single
`, baseURL)
}

// chainedFlowYAML is config (b): five request nodes in a dependency chain,
// where step N sends the token step N-1's response issued. This is what
// TestChainedFlowReallyChains checks isn't a lie.
func chainedFlowYAML(baseURL string) string {
	return fmt.Sprintf(`
workspace_name: LoadBench Workspace
flows:
  - name: ChainedFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: Step1
          depends_on: Start
          method: GET
          url: %[1]s/chain
      - request:
          name: Step2
          depends_on: Step1
          method: GET
          url: %[1]s/chain
          query_params:
            prev: '{{ Step1.response.headers.%[2]s }}'
      - request:
          name: Step3
          depends_on: Step2
          method: GET
          url: %[1]s/chain
          query_params:
            prev: '{{ Step2.response.headers.%[2]s }}'
      - request:
          name: Step4
          depends_on: Step3
          method: GET
          url: %[1]s/chain
          query_params:
            prev: '{{ Step3.response.headers.%[2]s }}'
      - request:
          name: Step5
          depends_on: Step4
          method: GET
          url: %[1]s/chain
          query_params:
            prev: '{{ Step4.response.headers.%[2]s }}'
`, baseURL, chainHeader)
}

// chainedFlowSteps names the chained config's steps in dependency order, for
// callers that want a per-step breakdown.
var chainedFlowSteps = []string{"Step1", "Step2", "Step3", "Step4", "Step5"}

// setupFlow imports a yamlflow document into a fresh in-memory workspace and
// returns the flow plus the services a load run needs.
//
// This duplicates apps/cli/internal/loadrun's own setupFlow test helper
// (which is unexported, so it cannot be imported) and, in turn, the CLI's
// setup in cmd/flow.go. Load-mode's docs already flag this as a known
// duplication of cmd wiring; for a benchmark, replicating a proven pattern is
// the right call over inventing a third way.
func setupFlow(t *testing.T, yamlDoc, flowName string) (*mflow.Flow, runner.RunnerServices) {
	t.Helper()

	ctx := t.Context()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("create in-memory db: %v", err)
	}
	t.Cleanup(cleanup)

	services, err := common.CreateServices(ctx, db, logger)
	if err != nil {
		t.Fatalf("create services: %v", err)
	}

	workspaceID := idwrap.NewNow()
	bundle, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlDoc), yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("convert yaml: %v", err)
	}

	builder := flowbuilder.New(
		&services.Node, &services.NodeRequest, &services.NodeFor, &services.NodeForEach,
		&services.NodeIf, &services.NodeJS, &services.NodeAI, &services.NodeAiProvider,
		&services.NodeMemory, &services.NodeGraphQL, &services.NodeWsConnection,
		&services.NodeWsSend, &services.NodeWait, &services.NodeSubFlowTrigger,
		&services.NodeSubFlowReturn, &services.NodeRunSubFlow, &services.WebSocket,
		&services.WebSocketHeader, &services.GraphQL, &services.GraphQLHeader,
		&services.Workspace, &services.Variable, &services.FlowVariable,
		resolver.NewStandardResolver(
			&services.HTTP, &services.HTTPHeader, services.HTTPSearchParam,
			services.HTTPBodyRaw, services.HTTPBodyForm, services.HTTPBodyUrlEncoded,
			services.HTTPAssert,
		),
		gqlresolver.NewStandardResolver(
			services.GraphQL.Reader(), &services.GraphQLHeader, &services.GraphQLAssert,
		),
		services.Logger,
		scredential.NewLLMProviderFactory(&services.Credential),
	)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	bundle.Workspace.ID = workspaceID
	if err := services.Workspace.TX(tx).Create(ctx, &bundle.Workspace); err != nil {
		_ = tx.Rollback()
		t.Fatalf("create workspace: %v", err)
	}
	importOpts := ioworkspace.GetDefaultImportOptions(workspaceID)
	importOpts.PreserveIDs = true
	if _, err := ioworkspace.New(services.Queries, logger).Import(ctx, tx, bundle, importOpts); err != nil {
		_ = tx.Rollback()
		t.Fatalf("import bundle: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit import: %v", err)
	}

	flows, err := services.Flow.GetFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get flows: %v", err)
	}
	var flow *mflow.Flow
	for i := range flows {
		if flows[i].Name == flowName {
			flow = &flows[i]
			break
		}
	}
	if flow == nil {
		t.Fatalf("flow %q not found among %d imported flows", flowName, len(flows))
	}

	return flow, runner.RunnerServices{
		NodeService:         services.Node,
		EdgeService:         services.FlowEdge,
		FlowVariableService: services.FlowVariable,
		Builder:             builder,
	}
}

// TestChainedFlowReallyChains proves the fixture behind the "5-step chained
// flow" benchmark config is not five independent requests in a trenchcoat:
// each step's request must carry the exact token the previous step's
// response issued.
//
// It runs without RUN_LOADBENCH because it guards the thing that would make
// every chained-flow benchmark number meaningless if it silently broke: a
// static or mis-wired URL would still produce a report, just not one that
// measures chaining. VUs=1 with a single iteration keeps the target server's
// recorded request order unambiguous - concurrent VUs would interleave the
// log and this check would need per-chain correlation it doesn't have.
func TestChainedFlowReallyChains(t *testing.T) {
	target := newBenchTarget(t, 0, true)
	flow, services := setupFlow(t, chainedFlowYAML(target.URL), "ChainedFlow")

	result, err := loadrun.Run(t.Context(), loadrun.Config{
		Flow:          flow,
		VUs:           1,
		MaxIterations: 1,
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Summary.Errors != 0 {
		t.Fatalf("Summary.Errors = %d, want 0", result.Summary.Errors)
	}

	hops := target.hops()
	if len(hops) != len(chainedFlowSteps) {
		t.Fatalf("server saw %d /chain requests, want %d", len(hops), len(chainedFlowSteps))
	}

	if hops[0].prev != "" {
		t.Errorf("Step1 sent prev=%q, want empty (it is the first hop, nothing precedes it)", hops[0].prev)
	}
	for i := 1; i < len(hops); i++ {
		if hops[i].prev != hops[i-1].issued {
			t.Errorf("%s sent prev=%q, want %q (%s's issued token) - the chain is broken",
				chainedFlowSteps[i], hops[i].prev, hops[i-1].issued, chainedFlowSteps[i-1])
		}
	}

	// Every issued token must be distinct, or a flow that hardcodes (say)
	// Step1's token into every subsequent request could pass the adjacency
	// check above by accident.
	seen := make(map[string]bool, len(hops))
	for _, h := range hops {
		if seen[h.issued] {
			t.Fatalf("token %q was issued more than once - the target stopped varying per request", h.issued)
		}
		seen[h.issued] = true
	}
}

// TestChainedFlowMultipleIterationsStillChain runs the same fixture for
// three iterations on a single VU, so the per-iteration hop count and the
// last iteration's chain are both checked - proving the chain resets cleanly
// each iteration rather than accidentally carrying a stale token forward.
func TestChainedFlowMultipleIterationsStillChain(t *testing.T) {
	const iterations = 3

	target := newBenchTarget(t, 0, true)
	flow, services := setupFlow(t, chainedFlowYAML(target.URL), "ChainedFlow")

	result, err := loadrun.Run(t.Context(), loadrun.Config{
		Flow:          flow,
		VUs:           1,
		MaxIterations: iterations,
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Summary.Errors != 0 {
		t.Fatalf("Summary.Errors = %d, want 0", result.Summary.Errors)
	}

	hops := target.hops()
	wantHops := iterations * len(chainedFlowSteps)
	if len(hops) != wantHops {
		t.Fatalf("server saw %d /chain requests, want %d", len(hops), wantHops)
	}

	for iter := range iterations {
		start := iter * len(chainedFlowSteps)
		iterHops := hops[start : start+len(chainedFlowSteps)]

		if iterHops[0].prev != "" {
			t.Errorf("iteration %d: Step1 sent prev=%q, want empty", iter, iterHops[0].prev)
		}
		for i := 1; i < len(iterHops); i++ {
			if iterHops[i].prev != iterHops[i-1].issued {
				t.Errorf("iteration %d: %s sent prev=%q, want %q",
					iter, chainedFlowSteps[i], iterHops[i].prev, iterHops[i-1].issued)
			}
		}
	}
}

// TestSingleGetFlowHitsTargetOncePerIteration is the single-GET config's
// equivalent sanity check: every iteration is exactly one request, and the
// report agrees with what the server actually saw.
func TestSingleGetFlowHitsTargetOncePerIteration(t *testing.T) {
	const iterations = 3

	target := newBenchTarget(t, 0, false)
	flow, services := setupFlow(t, singleGetFlowYAML(target.URL), "SingleGet")

	result, err := loadrun.Run(t.Context(), loadrun.Config{
		Flow:          flow,
		VUs:           1,
		MaxIterations: iterations,
	}, services, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if got := target.requests.Load(); got != iterations {
		t.Errorf("server saw %d requests, want %d", got, iterations)
	}
	if result.Report.Total.Count != iterations {
		t.Errorf("Report.Total.Count = %d, want %d", result.Report.Total.Count, iterations)
	}
	if result.Summary.Errors != 0 {
		t.Errorf("Summary.Errors = %d, want 0", result.Summary.Errors)
	}
}
