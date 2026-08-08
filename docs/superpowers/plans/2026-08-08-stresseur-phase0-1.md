# Stresseur Phase 0 + Phase 1 (independent items) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut every seam load testing needs (YAML contract, engine levers, metrics envelope, CI Action) with zero default-behavior change, per spec `docs/superpowers/specs/2026-08-08-load-testing-design.md` §4–§5.

**Architecture:** Wave 1 = four file-disjoint lanes safe to run in parallel (Task 1 YAML contract, Task 2 engine levers, Task 3 metrics envelope, Task 4 GitHub Action). Wave 2 = dependent integration (Task 5 CLI load mode, Task 6 benchmark) — **do not dispatch Wave 2 until Wave 1 is merged and green**. Desktop charts are a separate future plan.

**Tech Stack:** Go 1.25 (workspace), sqlc/SQLite, TypeSpec→buf codegen, cobra CLI, GitHub composite Action, `github.com/HdrHistogram/hdrhistogram-go`.

## Global Constraints

- **Zero default-behavior change.** No new flags/options ⇒ byte-identical output. Golden tests (Task 1 Step 1) are the enforcement mechanism. Existing tests may not be modified except where a task explicitly says so.
- **Commands need the env:** prefix everything with `direnv exec .`; for nx also `env NX_SOCKET_DIR=/tmp/nx-tmp` (e.g. `direnv exec . env NX_SOCKET_DIR=/tmp/nx-tmp pnpm nx run server:test`).
- **Go tests:** `cd packages/server && direnv exec ../.. go test ./path/ -run TestName -v -timeout 30s` for single tests; full: `direnv exec . env NX_SOCKET_DIR=/tmp/nx-tmp pnpm nx run server:test` / `cli:test`.
- **Lint before declaring done:** `direnv exec . env NX_SOCKET_DIR=/tmp/nx-tmp pnpm nx run server:lint`. Fix at root cause; **no `//nolint`, no suppressions** — including pre-existing issues your diff touches.
- **No raw SQL** (norawsql linter), **no reads inside transactions** (notxread linter).
- **Never hand-edit generated code** (`packages/spec/dist/`, `packages/db/pkg/sqlc/gen/`); regenerate via `spec:build` / `db:generate`.
- **Commits:** conventional style (`feat:`, `fix:`, `test:`), small and frequent. **Never add a `Co-Authored-By` line.** Never push. Never touch `main`.
- **Model separation:** proto ↔ model ↔ sqlc types stay distinct (`m` prefix models bridge).
- Error messages name the offending value and the valid alternatives.
- YAML export must stay deterministic (stable ordering) — the AI-fixer roadmap depends on diffable exports.

---

### Task 1: YAML contract — golden corpus, `version:` field, assertion-import fix, `run:` ordering

**Files:**

- Create: `packages/server/pkg/translate/yamlflowsimplev2/golden_test.go`
- Create: `packages/server/pkg/translate/yamlflowsimplev2/testdata/golden/` (corpus, ~8 files)
- Modify: `packages/server/pkg/translate/yamlflowsimplev2/types.go` (add `Version` field; top-level struct at :16-27)
- Modify: `packages/server/pkg/translate/yamlflowsimplev2/converter.go` (version validation)
- Modify: `packages/server/pkg/translate/yamlflowsimplev2/exporter.go` (emit `version: 2`; implementation entry at :23)
- Modify: `packages/server/pkg/translate/yamlflowsimplev2/converter_node.go` (assertion fix in `processRequestStep` :282-373)
- Modify: `packages/server/pkg/translate/yamlflowsimplev2/converter_flow.go` (`HTTPAssociatedData` :126-134, flow result assembly ~:263)
- Modify: `apps/cli/internal/runner/runner.go` (`RunMultipleFlows` :42-145 — topological order + explicit unknown-dep error)
- Test: `apps/cli/internal/runner/runner_test.go` (create if absent)

**Interfaces:**

- Consumes: nothing from other tasks.
- Produces: `YamlFlowFormatV2.Version int` (yaml key `version`, 0 = absent = treated as 2). `mhttp.HTTPAssert` records now populated in `ioworkspace.WorkspaceBundle.HTTPAsserts` on import. `RunMultipleFlows` executes flows in dependency order and returns error `unknown dependency %q in run block (known flows: %s)` for bad deps. Task 5 relies on all three.

**Context you must read first:** `yamlflowsimplev2/README.md`, the GraphQL assertion conversion at `converter_node.go:642-653` (the working pattern to mirror), `converter_template.go:12-40` (merge semantics — assertions **append**), and testdata at `packages/server/internal/api/rimportv2/testdata/ecommerce.yaml`.

- [ ] **Step 1: Characterization goldens (commit BEFORE any fix).** Build a corpus under `testdata/golden/`: one YAML per family — (a) request steps with map-form and list-form headers + assertions + templates via `use_request`, (b) `if`/`for`/`for_each` with `depends_on` handles (`Node.then`, `Node.loop`), (c) `js` + `wait`, (d) graphql with assertions, (e) ws_connection/ws_send, (f) sub-flows (trigger/return/run_sub_flow), (g) `run:` block multi-flow, (h) environments + credentials (with `{{ #env:X }}` placeholders). Reuse/adapt existing fixtures (`ecommerce.yaml`, `apps/cli/test/yamlflow/*.yaml` — note some use broken `${var}` syntax; write goldens with the real `{{ }}` syntax only). Test does: `Import(yaml) → Export(bundle) → yamlA; Import(yamlA) → Export → yamlB; assert yamlA == yamlB` (stability) plus snapshot `yamlA` as `.golden` file with an `-update` flag:

```go
var update = flag.Bool("update", false, "rewrite .golden files")

func TestGoldenRoundTrip(t *testing.T) {
    for _, name := range goldenCases() { // filenames in testdata/golden
        t.Run(name, func(t *testing.T) {
            in := readFile(t, "testdata/golden/"+name+".yaml")
            first := exportAfterImport(t, in)   // ConvertSimplifiedYAML → MarshalSimplifiedYAML
            second := exportAfterImport(t, first)
            if !bytes.Equal(first, second) { t.Fatalf("unstable round-trip") }
            golden := "testdata/golden/" + name + ".golden"
            if *update { writeFile(t, golden, first) }
            want := readFile(t, golden)
            if !bytes.Equal(first, want) { t.Fatalf("golden mismatch (run with -update after intentional changes):\n%s", diff(want, first)) }
        })
    }
}
```

- [ ] **Step 2: Run goldens, commit the characterization** (`test: add YAML round-trip golden corpus`). This snapshot INCLUDES today's bugs (assertions vanish) — that is the point.
- [ ] **Step 3 (version field): failing test** — `TestVersionField`: import doc with `version: 2` succeeds; absent succeeds; `version: 3` errors containing `unsupported yamlflow version 3 (this build supports up to 2)`; export output's first mapping key is `version: 2`.
- [ ] **Step 4: implement.** `Version int \`yaml:"version,omitempty"\``on`YamlFlowFormatV2`; validation in `Validate()`(types.go:560); exporter writes`version: 2`first (exporter builds an ordered structure — keep key order deterministic). Run test → PASS. Update goldens with`-update`, eyeball the diff (only `version: 2` line added), commit (`feat: version the yamlflow schema`).
- [ ] **Step 5 (assertion fix): failing test** — `TestHTTPAssertionsImported`: fixture request step with `assertions: [{expression: "response.status == 200"}, ...]` (check exact YAML assert shape in `types.go` `YamlAssertionV2` / exporter `:858` before writing) → after `ConvertSimplifiedYAML`, `result.HTTPAsserts` has the assertions bound to the request's node/example IDs. Also extend one golden case: assertions must survive `import → export` (they already export; the import side is what's broken).
- [ ] **Step 6: implement by mirroring the GraphQL path** (`converter_node.go:642-653`): add an asserts field to `HTTPAssociatedData` (converter_flow.go:126-134), populate it in `processRequestStep` from `finalReq.Assertions` (already merged by `mergeHTTPRequestDataStruct` — converter_template.go:36-38), convert to `[]mhttp.HTTPAssert` and append into the flow result where other HTTP associated data lands (~converter_flow.go:263). Run test → PASS. Update goldens (`-update`), verify diffs show assertions surviving, commit (`fix: import HTTP assertions from yamlflow (were silently dropped)`).
- [ ] **Step 7 (run: ordering): failing tests** in `apps/cli/internal/runner/runner_test.go` — construct `RunMultipleFlows` input where (a) `run:` lists `[B (depends_on A), A]` → execution order is A then B; (b) dep name `Missing` → error exactly matching `unknown dependency "Missing" in run block (known flows: A, B)`; (c) A fails → B reported skipped with reason, exit error non-nil (preserve today's failure-gate semantics, minus the silence).
- [ ] **Step 8: implement.** Parse the `run:` entries with the typed `yamlflowsimplev2` structs (replace the ad-hoc `map[string]interface{}` re-parse at runner.go:44-47), Kahn topological sort (deterministic tie-break: original list order), error on unknown/cyclic deps (`dependency cycle in run block: A → B → A`). Flows still execute sequentially (concurrency is Wave 2 territory). Run tests → PASS, commit (`fix: run block executes in dependency order and rejects unknown deps`).
- [ ] **Step 9: full gates.** `server:test`, `cli:test`, `server:lint` all green. Commit anything outstanding.

---

### Task 2: Engine levers — `CreateFlowRunner` options, scenario scheduler, lean mode

**Files:**

- Modify: `packages/server/pkg/flow/runner/flowlocalrunner/flowlocalrunner.go` (`CreateFlowRunner` :56-69, goroutine derivation :232-241)
- Create: `packages/server/pkg/flow/runner/scenariorunner/scenariorunner.go`
- Create: `packages/server/pkg/flow/runner/scenariorunner/scenariorunner_test.go`
- Modify (lean mode, smallest viable seam): `packages/server/pkg/flow/node/nrequest/nrequest.go` — investigate first; see Step 5.
- Test: extend `flowlocalrunner` tests in-package.

**Interfaces:**

- Consumes: nothing from other tasks.
- Produces (Task 5 wires these):

```go
// flowlocalrunner
type Option func(*FlowLocalRunner)                  // exact receiver type per file
func WithMaxConcurrency(n int) Option               // n<=0 ⇒ ignore, keep default
func CreateFlowRunner(/* existing params */, opts ...Option) *FlowLocalRunner

// scenariorunner (engine-agnostic VU scheduler; knows nothing about flows)
type RunProfile struct {
    VUs           int           // concurrent workers, >=1
    Duration      time.Duration // 0 ⇒ unbounded (use MaxIterations)
    MaxIterations int64         // 0 ⇒ unbounded (use Duration); both 0 = config error
}
type Summary struct {
    Iterations int64
    Errors     int64
    Elapsed    time.Duration
}
// iter runs ONE flow iteration; scenariorunner guarantees ≤VUs concurrent calls,
// stops issuing new iterations at Duration/MaxIterations/ctx-cancel, drains in-flight.
func Run(ctx context.Context, prof RunProfile, iter func(ctx context.Context, vu int, seq int64) error) (Summary, error)
```

- [ ] **Step 1 (options): failing test** — `TestCreateFlowRunnerDefaultUnchanged`: construct runner without options, assert concurrency field equals current CPU-derived value; `TestWithMaxConcurrency`: option sets it; `WithMaxConcurrency(0)` and negative are no-ops.
- [ ] **Step 2: implement** the variadic-options refactor (source-compatible: existing call sites — server `flowexec/session.go:61` and CLI `runner.go:282` — compile untouched; verify with `go build ./...` across the workspace). Run tests → PASS, commit (`feat: configurable max concurrency for flow runner`).
- [ ] **Step 3 (scheduler): failing tests** — table-driven in `scenariorunner_test.go`:
  - concurrency ceiling: iter sleeps 20ms, VUs=5, MaxIterations=50 → high-water concurrent (atomic counter) ≤ 5, Summary.Iterations == 50.
  - duration stop: Duration=150ms, iter takes 20ms → stops issuing after deadline, all in-flight drained, Elapsed ≥ 150ms, no iter call _starts_ after deadline.
  - error counting: every 3rd iter returns error → Summary.Errors == count, run continues (errors never abort the scenario).
  - cancel: ctx canceled mid-run → Run returns ctx.Err(), drains, no goroutine leak (`goleak` if already a repo dep, else assert with `runtime.NumGoroutine` delta tolerance — check go.mod first).
  - config error: VUs=0 or both Duration/MaxIterations zero → error, no work done.
  - run with `-race`.
- [ ] **Step 4: implement** `Run` — worker-per-VU goroutines pulling a shared atomic sequence counter, `errgroup` + context deadline, no unbounded channels. Run tests (`-race`) → PASS, commit (`feat: scenariorunner VU scheduler for load profiles`).
- [ ] **Step 5 (lean mode): investigate then implement the smallest seam.** Read `nrequest.go` fully: find where response bodies are retained in node output (`:81`, `:190` are the timing anchors; body write is nearby). Requirement: an opt-in flag (builder-level or request-node option — choose what needs the fewest call-site changes; document the choice in your report) that, after assertions/extraction complete for an iteration, drops response body retention so memory stays flat across thousands of iterations. Default MUST be current behavior; all existing tests pass unmodified. Failing test first: with lean on, node output for the request has body replaced by a truncation marker (`"[body dropped: lean mode]"`) while `response.status`/`response.duration` remain; with lean off, body intact. Commit (`feat: lean execution mode drops response bodies after assertion`).
- [ ] **Step 6: full gates** — `server:test`, `server:lint`, workspace `go build ./...`. Commit outstanding.

---

### Task 3: Metrics envelope — TypeSpec models + Go HDR aggregation package

**Files:**

- Create: `packages/spec/src/**` new TypeSpec file for load metrics (read the existing `.tsp` layout first and follow its module/namespace conventions — find how existing domains declare models and services; do NOT invent a new pattern)
- Generated: `packages/spec/dist/**` via `spec:build` (never hand-edit)
- Create: `packages/server/pkg/loadmetrics/loadmetrics.go`
- Create: `packages/server/pkg/loadmetrics/loadmetrics_test.go`
- Modify: `packages/server/go.mod` (+ workspace `go.work.sum` as generated) — add `github.com/HdrHistogram/hdrhistogram-go`

**Interfaces:**

- Consumes: nothing from other tasks.
- Produces (Task 5 + Phase 2 ingest rely on these — treat as frozen once merged):

```go
package loadmetrics

type StatusClass string // "2xx","3xx","4xx","5xx","error","timeout"

type Key struct {
    Step        string
    StatusClass StatusClass
}

type Frame struct {
    IntervalStart time.Time
    Interval      time.Duration
    Entries       map[Key]Entry
}
type Entry struct {
    Count      int64
    ErrorCount int64
    Bytes      int64
    Hist       *hdrhistogram.Histogram // 1µs..10min, 3 sig figs
}

func NewAggregator(interval time.Duration) *Aggregator
func (a *Aggregator) Record(k Key, latency time.Duration, bytes int64, isErr bool) // goroutine-safe
func (a *Aggregator) Flush(now time.Time) Frame                                    // drain current interval
func Merge(frames []Frame) Report

type Report struct {
    Total   Stats
    PerStep map[Key]Stats
}
type Stats struct {
    Count, ErrorCount, Bytes int64
    P50, P90, P95, P99, Max  time.Duration
    RPS                      float64 // Count / covered wall time
}
```

TypeSpec models (names frozen; shapes may follow existing proto conventions): `LoadMetricFrame`, `LoadMetricEntry` (step, status_class, count, error_count, bytes, hdr_histogram bytes, plus convenience p50/p90/p95/p99/max in µs), `LoadRunReport`, `LoadFailureArtifact` (step, vu, iteration, request{method,url,headers,body_sample}, response{status,headers,body_sample}, resolved_variables, error, captured_at).

- [ ] **Step 1: failing tests** for the Go package:
  - percentile correctness: record uniform 1..1000ms (step 1ms), assert P50 within 1% of 500ms, P99 within 1% of 990ms (HDR 3-sig-fig tolerance).
  - merge equivalence: two aggregators each fed half the data; `Merge(f1, f2)` percentiles equal one aggregator fed everything (within HDR tolerance).
  - status-class keying: 200→"2xx", 404→"4xx", transport error→"error" (provide `ClassifyStatus(code int, err error) StatusClass` — include timeout detection via `errors.Is(err, context.DeadlineExceeded)`/`os.IsTimeout`).
  - concurrency: 8 goroutines × 10k Records under `-race`.
  - RPS: 100 records over a flushed 5s frame → 20.0.
- [ ] **Step 2: implement** with `hdrhistogram-go` (`go get github.com/HdrHistogram/hdrhistogram-go` in `packages/server`). Run tests (`-race`) → PASS, commit (`feat: loadmetrics HDR aggregation package`).
- [ ] **Step 3: TypeSpec.** Read `packages/spec` layout; add the four models following existing conventions; run `direnv exec . env NX_SOCKET_DIR=/tmp/nx-tmp pnpm nx run spec:build`; verify generated Go + TS appear in `dist/` and the workspace still builds (`go build ./...`). Commit source + generated output in one commit per repo convention (check `git log -- packages/spec/dist` to confirm generated files are committed; follow whatever the repo does). (`feat: load metrics envelope in TypeSpec`).
- [ ] **Step 4: full gates** — `server:test`, `server:lint`, `spec:build` idempotent (second run = no diff). Commit outstanding.

---

### Task 4: GitHub Action `run-flows` (Phase 1, DevTools brand)

**Files:**

- Create: `actions/run-flows/action.yml` (composite)
- Create: `actions/run-flows/README.md`
- Create: `actions/run-flows/testdata/smoke.yamlflow.yaml`
- Create: `.github/workflows/action-test.yaml`
- Modify: `docs/cli.md` (replace the DIY CI snippet at :73-97 with the Action; keep the manual path as an alternative)

**Interfaces:**

- Consumes: released CLI binaries (naming per `.github/workflows/release-go.yaml:9-47` matrix) and `apps/cli/install.sh` (supports `INSTALL_DIR`; check whether it supports version pinning — if not, download release assets directly by tag).
- Produces: `uses: the-dev-tools/dev-tools/actions/run-flows@<ref>` with the contract below. Phase 2's Stresseur App links to the same report JSON schema.

Action contract (implement exactly):

```yaml
inputs:
  file: { required: true, description: 'Path to .yamlflow.yaml' }
  flow: { required: false, description: 'Single flow name; default = run: block' }
  version: { required: false, default: 'latest', description: 'CLI release tag, e.g. cli@1.0.3' }
  report-dir: { required: false, default: '.devtools-reports' }
  fail-on-error: { required: false, default: 'true' }
outputs:
  json-report: { description: 'Path to JSON report' }
  junit-report: { description: 'Path to JUnit XML' }
  success: { description: 'true/false' }
```

Steps inside the composite (all `shell: bash`): resolve + download the right binary for `runner.os/arch` into `$RUNNER_TEMP/devtools/bin`, chmod, run `devtoolscli flow run "$file" ${flow:+"$flow"} --report console --report "json:$report_dir/report.json" --report "junit:$report_dir/junit.xml"`, always generate a job-summary table into `$GITHUB_STEP_SUMMARY` from the JSON (flows, per-flow ✅/❌, durations — `jq` is preinstalled on GitHub runners), set outputs, exit per `fail-on-error`.

- [ ] **Step 1:** Write `smoke.yamlflow.yaml` — 2 flows hitting `https://jsonplaceholder.typicode.com` (mirror the working syntax from `apps/cli/test/yamlflow/ws_run_example.yaml` — do NOT copy the stale `${var}` examples), one with a `run:` block dependency.
- [ ] **Step 2:** Implement `action.yml` per the contract. Every run/step must be OS-guarded for linux/macos (windows out of scope — document in README).
- [ ] **Step 3:** `action-test.yaml` workflow: `on: pull_request: paths: ['actions/**']` + `workflow_dispatch`; job matrix `ubuntu-latest` + `macos-latest`; steps: checkout, `uses: ./actions/run-flows` with `file: actions/run-flows/testdata/smoke.yamlflow.yaml`, assert outputs (`test -f` the reports, `grep` the summary file). Since the runner needs a released binary, `version:` pins the latest published release — verify the download URL resolves with `curl -fsI` in the action itself and fail with a clear message if the release asset naming drifts.
- [ ] **Step 4:** Validate locally what's validatable: `actionlint` if available (`command -v actionlint`), YAML parse (`python3 -c 'import yaml,sys; yaml.safe_load(open(sys.argv[1]))'`), and run the composite's bash blocks standalone against a locally built CLI (`cd apps/cli && direnv exec ../.. task build`) to prove the report/summary generation logic. Document in the report what could only be verified in real CI.
- [ ] **Step 5:** Update `docs/cli.md`, README for the action (inputs/outputs table, two copy-paste examples: PR check, nightly cron). Commit (`feat: run-flows GitHub Action`).

---

### Task 5 (WAVE 2 — dispatch only after Tasks 1–3 merge): CLI load mode

**Files:**

- Modify: `apps/cli/cmd/flow.go` (flags :40-42, run path)
- Create: `apps/cli/internal/loadrun/loadrun.go` (+`loadrun_test.go`) — wires scenariorunner + loadmetrics + flow build/exec per iteration
- Modify: `packages/server/pkg/translate/yamlflowsimplev2/types.go` (+converter/exporter/golden updates) — additive `load:` block
- Modify: `apps/cli/internal/reporter/reporter.go` (aggregate table + additive JSON fields)

**Interfaces:**

- Consumes: Task 1 (`Version`, typed run parsing), Task 2 (`scenariorunner.Run`, `WithMaxConcurrency`, lean mode), Task 3 (`loadmetrics`).
- Produces: `devtoolscli flow run f.yaml --scenario checkout-baseline` and `--vus N --duration 60s [--iterations M]`; `load:` YAML block per spec §3.1 (executor `constant-vus` only in this task; `stages`/`ramping-vus`/thresholds are Phase 2); JSON report gains top-level `load_report` (loadmetrics.Report serialized); console prints:

```
Step            p50      p95      p99      RPS     Err%
CreateOrder     120ms    310ms    480ms    142.0   0.2
TOTAL           95ms     290ms    470ms    285.1   0.1
```

Key execution requirements: fresh node state per iteration (investigate whether `BuildNodes` must re-run per iteration or per-VU — measure both, pick correctness first, note cost in report); lean mode ON for load runs; no per-iteration DB writes; `--vus`+`--scenario` mutually exclusive (cobra `MarkFlagsMutuallyExclusive`); default invocation (no load flags) byte-identical (existing CLI tests unmodified; goldens prove the YAML side).

**Wave 1 review-derived contract additions (binding):**

1. **One `loadmetrics.Aggregator` per VU**, `Merge` at run end — the package's designed contention-avoidance path; do not share one aggregator across VUs.
2. **`Frame.Interval` is real elapsed time**, not the nominal constructor interval — RPS math must use it as such.
3. **Do not wrap the scenario context in `context.WithTimeout(ctx, prof.Duration)`** — `scenariorunner.Run` returns `ctx.Err()` when the caller's ctx dies, so duration-via-ctx would make every successful timed run return `DeadlineExceeded`. Pass Duration only via `RunProfile`.
4. **Drain or bypass `NodeRequestSideRespChan` in load mode** — lean mode drops the decoded body from VarMap but raw response bytes still flow down the persistence side-channel (`nrequest.go:238-248`); load mode must consume/discard that channel so memory stays flat and nothing persists per-iteration.
5. **Lean-mode coverage is request nodes only** (sub-flow propagation needs an `ExecuteSubFlow` signature change — out of scope): flows containing sub-flows/GraphQL/WS still run under load, but the memory-flatness guarantee is documented as request-node-scoped in `--help` and the report.
6. **StatusClass conversion**: write the Go↔generated-proto (`LoadStatusClass`) conversion helper next to where the JSON report is assembled; `loadmetrics.StatusClass` string values are the source of truth.
7. **New nx target `server:test:race`** running `go test -race` for `./pkg/flow/runner/scenariorunner/ ./pkg/loadmetrics/` (the two concurrency-critical packages), and run it in this task's gates. CI-workflow wiring is deferred (hygiene backlog).
8. **`load:` block YAML types are additive** (`YamlLoadScenario` in types.go), exported deterministically, `executor` value `constant-vus` only — any other value errors naming the valid alternatives and that `ramping-vus`/`constant-arrival-rate` arrive in Phase 2. One new golden fixture with a `load:` block.
9. **Exit codes**: a load run that completes reports exit 0 even with request errors (thresholds are Phase 2's gate mechanism); setup/infra failure (bad scenario name, bad flags, target unreachable at iteration 1 for ALL VUs) exits 1.
10. **Console aggregate table** exactly per the Produces block; JSON report gains additive top-level `load_report` (serialized `loadmetrics.Report` + run metadata: scenario name, VUs, duration, iterations, worker version).

Steps follow the same TDD cycle as Wave 1 (failing test → implement → gates → commit); the implementer drafts the step list from these requirements and the Task 1–3 interfaces, and the reviewer holds it to this contract.

### Task 6 (WAVE 2 — after Task 5): RPS/worker benchmark

**Files:** Create `apps/cli/test/loadbench/` (self-contained: local `net/http/httptest` target server with fixed 5ms handler — no external dependencies), bench script + `docs/superpowers/specs/phase0-bench.md` results.

**Contract:** measure sustained RPS at VUs ∈ {1, 10, 50} for (a) single-GET flow, (b) 5-step chained flow, on local hardware; record CPU count, Go version, lean-mode on; write the table to the doc. This number gates spec §3.5 capacity math and §6 pricing — flag in the doc that Fly-hardware numbers are still pending.

---

## Integration notes (controller, not a dispatched task)

- Merge order after Wave 1 review: Task 1 → 2 → 3 → 4 (only expected overlap: none; `go.sum` churn from Task 3 only).
- After merge: run `server:test`, `cli:test`, `db:test`, `server:lint`, `client:lint`, `root:lint:format`; then goldens once more.
- Wave 2 dispatch requires: all Wave 1 tasks merged + gates green.
