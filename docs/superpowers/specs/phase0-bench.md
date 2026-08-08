# Phase 0 benchmark: RPS/worker

Status: **dev-hardware only. Fly shared-cpu-machine numbers are not yet measured.**

This is Phase 0 work item 7 (`docs/superpowers/plans/2026-08-08-stresseur-phase0-1.md`
§4.1.7): measure what one load worker (one CLI process, one `load run`) can actually
generate, using the real `loadrun` engine (`apps/cli/internal/loadrun`), not a
reimplementation of its scheduling. It answers "RPS per worker" for two reference
flows, which is the input spec §3.5 ("workers are sized by benchmark ... 500 VUs ≈ a
handful of machines, not 500") and spec §6 ("benchmark finalizes the unit economics")
both name as a prerequisite.

**These numbers gate §3.5 capacity math and §6 pricing only as upper-bound local
numbers.** They were produced on a laptop against an in-process target with no network,
no TLS, no real backend work, and no other tenants competing for the CPU. A Fly
shared-cpu machine (the actual worker in production) has different CPU/network
characteristics, runs one worker per machine (fewer noisy neighbors than a laptop, but
weaker single-core performance than an M-series Mac), and any real target has
non-trivial latency of its own. Until that number exists, treat everything below as
"the engine's own overhead ceiling, measured somewhere convenient" - a ceiling real
capacity math must stay under, not a floor it can plan against.

## Environment

|           |                                                                                                                                                                                                                    |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Date      | 2026-08-08                                                                                                                                                                                                         |
| Machine   | Apple M2 Max, 12 logical CPUs (`runtime.NumCPU()`), 32 GB RAM                                                                                                                                                      |
| OS/Arch   | darwin/arm64 (`GOOS`/`GOARCH`)                                                                                                                                                                                     |
| Go        | go1.25.5 (`runtime.Version()`)                                                                                                                                                                                     |
| Lean mode | on (always on for load runs - see `loadrun.go` package doc)                                                                                                                                                        |
| Target    | local, in-process `httptest.NewServer`, loopback only, fixed 5ms handler latency (`time.Sleep(5*time.Millisecond)`), no external network dependency of any kind                                                    |
| Engine    | `apps/cli/internal/loadrun.Run`, driven in-process exactly as `apps/cli/internal/loadrun/loadrun_test.go`'s `setupFlow` helper does (yamlflow import → in-memory SQLite workspace → `flowbuilder` → `loadrun.Run`) |

## Methodology

- **Harness**: `apps/cli/test/loadbench/`. `loadbench_test.go` holds the target server,
  the two flow fixtures, and fixture-correctness tests that run unconditionally (no
  build tag, no env guard - they run in `cli:test` like any other test).
  `integration_loadbench_test.go` holds the actual RPS/percentile matrix, gated by both
  `//go:build loadbench_integration` and `RUN_LOADBENCH=true`, following this repo's
  integration-test convention (paired build tag + env guard, e.g.
  `packages/server/pkg/flow/node/nai`'s `ai_integration` / `RUN_AI_INTEGRATION_TESTS`).
- **Configs**:
  - **(a) single-get** - one request node (`GET /single`).
  - **(b) chained-5-step** - five request nodes in a real dependency chain. Each step
    reads a token the previous step's response issued (via a response header,
    `Chaintoken` - see "why a header, not the body" below) and sends it back as a query
    parameter on the next request. `TestChainedFlowReallyChains` and
    `TestChainedFlowMultipleIterationsStillChain` (both unguarded, run in `cli:test`)
    assert step N's request carries exactly step N-1's issued token, so this is
    real chaining, not five independent requests wearing a trenchcoat.
- **VUs**: 1, 10, 50, per the task contract.
- **Stop condition**: `Duration`, not `MaxIterations` - simpler to reason about and
  guarantees the ≥5s floor unconditionally. A 1s warmup run (discarded) precedes every
  measured window, so Go's connection pool and the scenario's goroutines aren't
  measured mid-ramp. Measured-window length is tiered by VU level, since this
  fixed-5ms-latency target caps one VU at ~200 req/s and low-VU cells need more wall
  time to accumulate a comparable sample: 10s at VUs=1, 8s at VUs=10, 6s at VUs=50. All
  tiers clear well over a thousand requests; see the table for exact counts.
- **Isolation between cells**: each (config, VUs) cell runs in its own `t.Run` subtest,
  with its own target server and its own in-memory SQLite workspace, torn down before
  the next cell starts. Cells run VUs-major (all VUs=1 first, then all VUs=10, then
  both VUs=50 cells last) and are separated by a 35s cooldown - both exist specifically
  to stop one cell's numbers from contaminating another's; see "Anomaly investigated"
  below for why that isolation turned out to matter.
- **What "RPS" means here**: `loadmetrics.Report.Total.RPS` is
  `(requests recorded)/(wall time covered)`, and a request is recorded whether it
  succeeded or failed (see `vuWorker.record` in `loadrun.go`) - so it is an _attempt_
  rate, not a success rate. At VUs=1 and VUs=10 every attempt succeeded, so the
  distinction is moot there. At VUs=50 it is not; see below.
- **Why a header, not the body, carries the chained value**: load runs always run in
  lean mode, which replaces the entire decoded response body with a fixed placeholder
  string once assertions run (`nrequest.LeanBodyPlaceholder`) - a flow that tried to
  chain via `response.body.someField` would silently chain the placeholder, not a real
  per-response value. Response _headers_ are untouched by lean mode, so the target
  returns its issued token via a `Chaintoken` response header instead. (It has no
  hyphen: `{{ }}` interpolation runs through `expr-lang`, which parses a hyphenated
  segment as subtraction between two identifiers rather than as a map key.)

## Results

RPS is requests/sec (attempts, per above). P50/P95/P99 are per-request latency
(HDR histogram, includes both successes and failures - a fast-failing dial attempt at
VUs=50 pulls percentiles in a different direction than a slow one, see below).

| Config         | VUs | Requests | Iterations | Elapsed | RPS     | Error % | P50     | P95      | P99       | Max       |
| -------------- | --- | -------- | ---------- | ------- | ------- | ------- | ------- | -------- | --------- | --------- |
| single-get     | 1   | 1,554    | 1,554      | 10.005s | 155.3   | 0.0%    | 6.503ms | 6.835ms  | 7.091ms   | 8.343ms   |
| chained-5-step | 1   | 1,560    | 312        | 10.029s | 155.5   | 0.0%    | 6.515ms | 6.875ms  | 7.071ms   | 8.543ms   |
| single-get     | 10  | 12,516   | 12,516     | 8.002s  | 1,564.1 | 0.0%    | 6.323ms | 6.687ms  | 7.327ms   | 18.927ms  |
| chained-5-step | 10  | 12,600   | 2,520      | 8.026s  | 1,569.8 | 0.0%    | 6.359ms | 6.651ms  | 7.215ms   | 14.975ms  |
| single-get     | 50  | 25,544   | 25,544     | 6.014s  | 4,247.6 | 12.9%   | 5.879ms | 11.375ms | 156.927ms | 860.159ms |
| chained-5-step | 50  | 26,807   | 7,778      | 6.055s  | 4,427.0 | 42.0%\* | 5.907ms | 7.355ms  | 195.199ms | 350.207ms |

\* Iteration error rate. The chain aborts a step's dependents once that step fails
(`depends_on` never satisfies), so per-_iteration_ failure overstates per-_request_
failure for the chained config: 3,266 of 26,807 individual requests failed (12.2%),
concentrated at Step1 (2,633 of 7,778 attempts, 33.9%) and falling steeply at each
later hop (Step2 8.0%, Step3 2.9%, Step4 1.0%, Step5 0.9%) as fewer iterations survive
to reach them.

**This table is from the final run against the lint-clean committed code** (the run
this task's gates section also reports). An earlier run against functionally
identical code (before two cosmetic lint fixes - see "Files changed") measured
single-get/chained-5-step VUs=50 error rates of 14.1%/37.7% instead of this run's
12.9%/42.0%; VUs=1 and VUs=10 were stable to three figures across both runs. See
"run-to-run variability" under the anomaly section below - this spread is itself
part of the finding, not noise to average away. Raw command output for every run,
including the `t.Run` subtest breakdown and per-step DIAG lines, is preserved
verbatim in this task's report (`task-6-report.md`).

### Scaling sanity check (VUs=1 → VUs=10)

At a fixed 5ms handler latency the theoretical ceiling is linear in VUs
(`VUs / latency`). Measured:

- single-get: 155.3 → 1,564.1 RPS = **x10.1**
- chained-5-step: 155.5 → 1,569.8 RPS = **x10.1**

Both configs scale within noise of perfectly linear, and both are error-free at these
tiers. This is the harness's own precondition check (`sanityCheckScaling` in
`integration_loadbench_test.go`, run automatically): a flat curve here would mean the
harness was serializing somewhere, and the numbers would be measuring this package's
bugs instead of the engine. It passed on every run once cell isolation was fixed (see
below) - the curve is real.

## Anomaly investigated: VUs=50 request failures

The first full-matrix run showed VUs=50 failing 30-98% of iterations depending on
ordering, with a p99 in the _seconds_. That is exactly the "if it doesn't [scale],
investigate before writing the doc" case this task calls out, so before anything below
was accepted as real, it was root-caused rather than reported as-is.

**Root cause**: `packages/server/pkg/httpclient.New()` (the CLI's actual HTTP client,
used unmodified by every `loadrun` VU worker) builds an `http.Client` with no custom
`Transport`, so it falls back to `http.DefaultTransport`, whose
`MaxIdleConnsPerHost` defaults to 2. At VUs=50 against a target this fast, the client
issues thousands of requests/sec to one host; with only 2 idle connections kept per
host, nearly every request redials instead of reusing a keep-alive connection. On
macOS the ephemeral port range is 49152-65535 (~16k ports,
`sysctl net.inet.ip.portrange.*`) and `TIME_WAIT` lasts 2×MSL ≈ 30s
(`sysctl net.inet.tcp.msl` = 15000ms here), so sustained redialing at thousands/sec
exhausts the range within single-digit seconds. This was confirmed independently of
the flow engine with a ~60-line raw `net/http` repro (50 goroutines, a fresh
`http.Client` each, hammering a local `httptest.Server` for 6s): 21.4% of requests
failed with `dial tcp 127.0.0.1:PORT: connect: can't assign requested address` -
textbook ephemeral port exhaustion, not a flow-engine or flake.

**Why the first matrix run was worse than that repro (30-98% vs ~15-21%)**: the first
version of this harness ran all six cells back-to-back with only a 2s gap and no
resource teardown between them. A VUs=10 cell alone opens ~12,500 short-lived
connections; with a 30s `TIME_WAIT`, a 2s gap starts the next cell with most of the
ephemeral range still occupied by the _previous_ cell's sockets - so the reported
"VUs=50" failure rate was really measuring leftover port pressure from VUs=10, not
VUs=50 itself, and got worse the more cells had already run. Fixed by: running cells
VUs-major so nothing runs after a VUs=50 cell, wrapping every cell in its own `t.Run`
so its server and DB are torn down before the next cell starts, and - the part that
actually mattered - a 35s cooldown before every cell, longer than this machine's
`TIME_WAIT`. The numbers in the table above are from the run after that fix; they
matched the isolated single-cell repro's error rate (~14-15% for single-get) far more
closely than the contaminated run did, which is itself evidence the fix addressed the
right thing.

**Is this a load-engine bug?** Not one this task should fix - `httpclient.New()` is
shared production code with call sites well beyond load mode, and retuning its
connection pool is a separate, deliberate change with its own review, not a
benchmark-harness side effect. It is a real, reproducible characteristic worth
recording for whoever does own that decision: **a single worker sustaining
several-thousand req/s against one host, for more than a few seconds, can exhaust
local ephemeral ports on this class of machine** without an explicit
`MaxIdleConnsPerHost`/`MaxConnsPerHost` tuned for the expected VU count. Two mitigating
factors for how much this matters in practice:

1. It is a _rate_ problem, not strictly a _VUs_ problem - it happens because 50 VUs
   against a 5ms target reach ~4,200-4,500 req/s. A real target with realistic latency
   (tens to hundreds of ms) would let the same 50 VUs reach only a fraction of that
   rate, buying comparable headroom before hitting the same wall. This benchmark's
   target is deliberately unrealistically fast (that is the point - it isolates engine
   overhead), and that is exactly what makes it also the case most likely to trip this.
2. Effective _successful_ throughput at VUs=50 was still far above VUs=10, not flat:
   in the reported run, single-get did 22,240 successful requests in 6.014s (3,698.0
   successful req/s); chained-5-step did 23,541 successful individual requests in the
   same window (3,887.9 successful req/s), or 4,512 fully-completed 5-step iterations
   (3,725.8 successful req/s expressed the same way - iterations x steps / elapsed).
   All three land within ~5% of each other despite two different flow shapes -
   consistent with one shared underlying ceiling (port cycling rate), not something
   specific to chaining. (The earlier run showed the same pattern at slightly higher
   absolute numbers: ~3,850-4,050 successful req/s - see below.)

**Run-to-run variability**: two complete matrix runs against functionally identical
code (see "Files changed" for the no-op lint diff between them) produced consistent
VUs=1/VUs=10 numbers (RPS within 1% of each other, both runs zero errors, both scaled
~x10.0-10.1) but noticeably different VUs=50 error rates:

| Run                                                                     | single-get VUs=50 error% | chained-5-step VUs=50 error% |
| ----------------------------------------------------------------------- | ------------------------ | ---------------------------- |
| Pre lint-fix (same logic, cosmetic diff only)                           | 14.1%                    | 37.7%                        |
| Post lint-fix - **this is the run reported in the Results table above** | 12.9%                    | 42.0%                        |

This is expected, not a measurement bug: ephemeral port availability at the moment a
cell starts depends on exactly how much of the previous cooldown's `TIME_WAIT` backlog
has drained, which is a real-clock race against the kernel, not something either this
harness or the load engine controls. Treat any single VUs=50 error-rate figure as "this
class of machine, under this specific artifact, lands somewhere in the low-teens to
low-40s percent" rather than a precise number - the imprecision is itself the finding.

**Capacity-math implication**: do not read "4,247.6 RPS at VUs=50" (or "4,481.6" from
the other run) as a clean 50-VU number the way the VUs=1/VUs=10 rows can be read. Read
it as "on this machine, against this unrealistically fast target, roughly 3,700-4,050
req/s got through across two runs, and 13-42% of attempts hit a client
connection-pooling limit that a real target's latency would likely mask." §3.5's
"workers are sized by benchmark" should size against the VUs=10 row (clean, linear,
zero errors, stable across repeated runs) until either a Fly-hardware run or a
realistic-latency target confirms whether the VUs=50 ceiling is a laptop-and-loopback
artifact or something that also shows up in production.

## What this does and doesn't tell you

- **Tells you**: the engine's own per-request overhead above a bare 5ms floor is
  small and stable - roughly 1.3-1.5ms at P50 (6.3-6.5ms measured vs 5ms configured)
  across every error-free cell, whether chained or not, whether at 1 VU or 10. Building
  the request, running it through `expr-lang` interpolation, evaluating assertions, and
  folding the result into `loadmetrics` costs about that much per request on this
  hardware. That overhead is flat with VU count and with chain position (all 5 steps of
  the chained flow show essentially the same P50), which is the shape you want: it
  means the _engine_ isn't the bottleneck at these VU levels, whatever else is.
- **Does not tell you**: what a Fly shared-cpu machine can sustain (not yet measured -
  the explicit gap this doc flags), what a realistic-latency target changes about the
  VUs=50 picture (not yet measured), or where the true per-worker VU ceiling sits once
  connection pooling is tuned for the expected load-testing use case (out of scope for
  this task; a note for whoever picks up `httpclient.go` next).

## Reproducing this

```bash
# Fixture correctness (fast, runs in the default suite too):
go test ./apps/cli/test/loadbench/...

# The full matrix (~5 minutes, mostly cooldown; see the `cooldown` doc comment
# in integration_loadbench_test.go for why it can't be shorter on this OS):
RUN_LOADBENCH=true go test -tags loadbench_integration \
  -run TestLoadBenchMatrix ./apps/cli/test/loadbench/ -v -timeout 900s
```
