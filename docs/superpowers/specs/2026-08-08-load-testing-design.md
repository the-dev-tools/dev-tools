# Stresseur: Load Testing & Agent-Native SaaS — Design

**Date:** 2026-08-08
**Status:** Draft for review
**Scope:** Phase 0 in implementation-ready detail; Phases 1–3 at planning detail; GTM strategy.

## 1. Product thesis

Development is now agent-driven. Teams ship endpoints in hours via Claude Code/Cursor,
and the API-testing tools of the last era assume a human with a GUI and an afternoon.
Meanwhile API traffic itself is turning agentic — burstier, chained, less forgiving.
APIs have never been produced faster or verified less.

DevTools' unfair advantage: **the test artifact already exists and agents can operate it.**
Recorded traffic → flow → YAML in the repo. The same YAML is the functional test and the
load test. Coding agents read, write, diff, and fix it like any other code.

- **Postman** cannot say this: cloud workspace, human GUI, collections agents don't maintain.
- **k6** cannot say this: the load script is a separate, hand-written artifact divorced
  from functional tests.
- **DevTools**: one artifact, two modes (functional / load), three runtimes (desktop / CI /
  cloud), operable by humans and agents.

One-liner: _"Your API tests are already load tests — and your agents keep them green."_

**Brand architecture (decided 2026-08-08): two brands, one contract.**
**DevTools** stays the free, open-source platform: desktop app, CLI, GitHub Action,
the YAML format, and local load mode. **Stresseur** (stresseur.com) is the commercial
layer: hosted load generation, baselines/history, the PR bot, the AI maintenance
agents, and the MCP server — all running the DevTools engine against the same
repo-resident YAML.

The free DevTools product is the funnel (honors the no-signup, local-first brand).
Stresseur sells the three things that structurally cannot be self-hosted freebies:

1. **Hosted load generation** — fleets, geo, concurrency pools (metered VU-hours)
2. **History & baselines** — "p95 vs last release" as a PR gate (team plans)
3. **AI test maintenance** — bug bot + auto-fix of flow YAML on API changes (seats)

## 2. Current state (verified 2026-08-08)

_State reflects branch base `36d63065`; Phase 0 (this branch) retires the ❌ rows below for YAML schema versioning, load/iteration levers, aggregate stats, and the GitHub Action, plus known-defect rows 1–2 (assertion drop, `run:` ordering)._

| Capability                | State                                                                                            | Evidence                                                        |
| ------------------------- | ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------- |
| Portable YAML flow format | ✅ Self-contained by name; no DB IDs                                                             | `packages/server/pkg/translate/yamlflowsimplev2/types.go:16-27` |
| CLI runs YAML headlessly  | ✅ Embeds full server engine; static `CGO_ENABLED=0` binary; in-memory SQLite per run            | `apps/cli/cmd/flow.go:87-336`, `sqlitemem`                      |
| JS nodes                  | ✅ Node spawned only when flow has JS nodes; ConnectRPC over unix socket                         | `apps/cli/cmd/flow.go:266-291`, `jsrunner.go`                   |
| Per-request latency       | ✅ Measured in Go (`response.duration` ms; node wall-clock ns)                                   | `packages/server/pkg/flow/node/nrequest/nrequest.go:81,190`     |
| Reporters                 | ✅ console / json / junit, exit codes 0/1                                                        | `apps/cli/internal/reporter/reporter.go`                        |
| Engine parallelism        | ⚠️ Dependency-free steps run concurrently; cap hardcoded to CPU count; no external lever         | `flowlocalrunner.go:232-241`, `strategy_multi.go:63-114`        |
| FOR/FOR_EACH loops        | Sequential iterations (`for i := range nr.IterCount`)                                            | `nfor.go:129`                                                   |
| Cancellation              | ✅ `context.WithCancel`                                                                          | `flowlocalrunner.go:142`                                        |
| Event pub/sub             | ✅ Generic `SyncStreamer[Topic, Payload]`, in-memory impl (single-node)                          | `packages/server/pkg/eventstream/eventstream.go`                |
| Auth foothold             | ✅ BetterAuth tables (user/session/account+OAuth) already in schema                              | `packages/db/pkg/sqlc/schema/08_betterauth.sql`                 |
| Plans/quotas/billing      | ❌ None anywhere in schema                                                                       | schema grep 2026-08-08                                          |
| YAML schema versioning    | ❌ No `version:` field; parser is lenient (no `KnownFields`) → additive fields are backward-safe | translate pkg grep 2026-08-08                                   |
| Load/iteration levers     | ❌ No iterations/VU/duration/rate anywhere (YAML, CLI flags, engine API)                         | `flow.go:40-42`                                                 |
| Aggregate stats           | ❌ No percentiles/histograms/throughput anywhere                                                 | reporter + engine grep                                          |
| GitHub Action             | ❌ Docs show DIY snippet only; no published action                                               | `docs/cli.md:73-97`                                             |

### Known defects that intersect this work

| Defect                                                                      | Impact                                                                                               | Where                                                                                               |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| HTTP assertions silently dropped on YAML import                             | Export→import round-trip loses assertions; imported "tests" assert nothing                           | `converter_node.go:282-373` (never populates asserts); contrast GraphQL `converter_node.go:642-653` |
| `run:` block ignores dependency order                                       | Strict list-order execution; unknown dep names silently skip the flow                                | `apps/cli/internal/runner/runner.go:109-144`                                                        |
| Stale `${var}` syntax in shipped CLI examples                               | Examples send literal `${user_id}` strings; correct syntax is `{{ Node.index }}` / `{{ Node.item }}` | `apps/cli/test/yamlflow/example_run_yamlflow.yaml:20` et al.                                        |
| `flow.timeout` / `flow.metadata` parsed but never read                      | Dead schema fields                                                                                   | `types.go:67-73`                                                                                    |
| GraphQL-only / WS-only exports use ad-hoc shapes that cannot be re-imported | Round-trip broken for those exports                                                                  | `rexportv2/export.go:257-342`                                                                       |
| CLI JS nodes broken on Windows                                              | Go dials `unix`; worker binds named pipe on win32                                                    | `jsrunner.go:63` vs `worker-js/src/main.ts:32-36`                                                   |
| `devtools version` prints v0.1.0 (package is 1.0.3)                         | Cosmetic; breaks support triage                                                                      | `apps/cli/cmd/version.go:13`                                                                        |

## 3. Target architecture

### 3.1 The contract: repo-resident YAML + run profiles

Flows stay exactly as they are. Load configuration is a **separate `load:` section
referencing flows by name** — a flow is never edited to be load-tested:

```yaml
version: 2 # NEW — additive, required going forward on export
workspace_name: Shop
flows:
  - name: Checkout Flow
    steps: [...] # unchanged

load: # NEW — Phase 1 (local), Phase 2 (cloud adds regions/etc.)
  - name: checkout-baseline
    flow: Checkout Flow
    executor: ramping-vus # constant-vus | ramping-vus | constant-arrival-rate
    stages:
      - { target: 200, duration: 2m }
      - { target: 200, duration: 5m }
    thresholds:
      - p95(CreateOrder) < 300ms # step-scoped percentile
      - error_rate < 0.5% # scenario-scoped
```

Thresholds make load runs **gate-able**: pass/fail, exit code, PR comment.
A `load:` entry with no thresholds is exploratory; with thresholds it is CI-enforceable.

Design rules for the YAML (they serve the AI fixer later):

- **Deterministic export** — stable key order, minimal churn, so agent-authored patches diff cleanly.
- **Versioned** — `version: 2` emitted on export; parser accepts absent (=2) or 2; errors
  clearly on >2. Old binaries ignore the unknown key (lenient parser, verified).

### 3.2 The engine: our CLI is the load worker (not k6)

The CLI already executes the full flow semantics in Go. A load worker is the same
binary with new levers. k6 is rejected as the core engine because it would require a
permanently-maintained lossy transpiler (chaining, extraction, assertions, `{{ }}`
resolution, JS nodes) and its results would subtly disagree with functional runs.
k6 remains a possible future adapter for extreme raw-RPS; not in any planned phase.

New engine concepts (all opt-in; default behavior byte-identical to today):

- **RunProfile** — `{ executor, vus, stages, duration, maxIterations }` consumed by a
  scenario scheduler that runs N concurrent flow executions (VU = one flow-execution
  loop). Sits _above_ `flowlocalrunner`; loop nodes are untouched.
- **Lean execution mode** — response bodies released after extraction/assertion;
  no per-iteration persistence; aggregates only. Required to keep memory flat at volume.
- **Configurable concurrency** — `CreateFlowRunner` gains functional options
  (`WithMaxConcurrency(n)`); absent options preserve today's CPU-derived default.

### 3.3 Metrics envelope (the hard-to-retrofit piece — designed in Phase 0)

Defined in TypeSpec (`packages/spec`) so Go/TS types are generated, one source of truth:

- **MetricFrame** — per (scenario, step, status-class) HDR histogram of latency +
  counters (requests, errors by taxonomy, bytes), flushed at a fixed interval (5s)
  and at run end. Workers stream frames; controllers/CLI merge them (HDR histograms
  merge losslessly).
- **RunReport** — merged frames + threshold verdicts + environment fingerprint
  (worker version, region, machine class) for baseline comparability.
- **FailureArtifact** — _sampled_ full request/response captures with resolved
  variables, keyed by (step, error class). Cap per run. This is the substrate the
  bug bot and AI fixer reason over; it is not optional telemetry.

The existing CLI JSON report becomes the N=1 degenerate case of RunReport
(new fields are additive; old consumers ignore them).

### 3.4 Three runtimes, one contract

| Runtime                      | Brand         | What runs                                        | Who pays              | Phase |
| ---------------------------- | ------------- | ------------------------------------------------ | --------------------- | ----- |
| Desktop                      | DevTools      | flows from workspace (today) + local load charts | free                  | 1     |
| CI (published GitHub Action) | DevTools      | functional flows on every PR; smoke-load allowed | customer's CI minutes | 1     |
| Cloud fleet (Fly Machines)   | **Stresseur** | load scenarios; geo; baselines                   | metered VU-hours      | 2     |

### 3.5 Stresseur control plane (Phase 2)

Own it; do not outsource to Trigger.dev. It is core product logic, small, and the team
already writes exactly this kind of Go. (Trigger.dev Cloud pricing/concurrency also fits
poorly: we'd pay for orchestration concurrency we can express as one DB state machine.)

- **Service**: new module in the monorepo reusing `idwrap`, `eventstream` patterns, sqlc
  discipline. DB: LibSQL/Turso (stays in the SQLite family) — Postgres only if
  multi-writer needs force it later.
- **State machine**: `pending → validating → provisioning → ramping → running →
draining → complete | failed | canceled`, persisted per run; transitions idempotent.
- **Worker lifecycle**: Fly Machines API; workers boot the DevTools CLI image with a
  short-lived run token (Stresseur orchestrates; the DevTools engine executes — one
  execution semantics everywhere); heartbeat + frame ingest over ConnectRPC; controller destroys machines at
  `draining`; a reconciler sweep destroys orphans (crash-safety) and enforces max-age.
- **Ingest**: frames land in run-scoped tables; live progress fans out over the existing
  eventstream abstraction backed by a durable bridge (LibSQL table poll or NATS —
  decision deferred to Phase 2 detail plan).
- **Capacity math**: VUs are concurrent flow-loops per worker; workers are sized by
  benchmark (Phase 0 produces the RPS/worker baseline). 500 VUs ≈ a handful of
  machines, not 500.

### 3.6 Multi-tenancy, quotas, abuse

- Accounts: BetterAuth (already in schema) + orgs.
- Stresseur plans + per-org limits enforced at `validating`:

|                          | Stresseur Free | Stresseur Team | Stresseur Scale |
| ------------------------ | -------------- | -------------- | --------------- |
| Concurrent load runs/org | 1              | 3              | 10              |
| Max VUs                  | 50             | 500            | 5,000           |
| Max duration             | 5 min          | 30 min         | 4 h             |
| Regions                  | 1              | 3              | all             |
| History/baselines        | 7 days         | 90 days        | custom          |

(Values illustrative; finalize with design partners.)

- Excess runs **queue** rather than fail (per-org FIFO; fairness cap per org so one
  tenant cannot drain the fleet).
- **Target verification is launch-blocking**: hosted load-gen without it is a DDoS
  cannon. Require proof of ownership per target host before any cloud run
  (DNS TXT record or response-header echo — the Loader.io/Grafana k6 model), plus
  hard per-plan egress caps and a global blocklist (gov, known-shared infra).

### 3.7 The PR-native surfaces (Phase 2/3 — the attention engine)

**Namespace status (2026-08-08):** stresseur.com + stresseur.dev owned;
`github.com/stresseur` org owned; npm `stresseur` published (placeholder 0.0.2,
homepage stresseur.com). Remaining: register the GitHub **App** named `stresseur` to
hold the slug — the bot identity (`stresseur[bot]`) is the product surface, and App
slugs are first-come. Brand split per §1: DevTools = platform, Stresseur = service.

**GitHub App ("Stresseur")**, the `cursor review`-style motion:

- On PR open: run the repo's functional flows against the preview/staging URL
  (env-mapping config in repo maps environments → URLs/secrets refs; file naming is
  open decision §8.3).
- On comment **`@stresseur load checkout-baseline`**: fire a cloud load run against the
  preview env, reply with a rich comment — pass/fail vs thresholds, p50/p95/p99 vs
  baseline, slowest steps, error taxonomy, link to full report.
- Every comment in a public repo is distribution ("Powered by Stresseur" footer).

**MCP server** (thin layer over the same runs API): `list_flows`, `run_flow`,
`run_load_scenario`, `get_report`, `compare_to_baseline`, `verify_target`. This makes
DevTools the tool coding agents reach for on their own — same API, zero extra backend.

### 3.8 Stresseur AI: test maintenance (Phase 3 — the moat)

- **Bug bot**: on functional failure in CI/cloud, reads FailureArtifacts + the PR diff,
  classifies (API regression vs test rot vs flaky infra), comments a triage verdict.
- **YAML fixer**: for test-rot cases (endpoint renamed, field moved, auth header
  changed), proposes a patch to the flow YAML as a suggested commit on the same PR.
  Deterministic YAML export + rich artifacts make these patches small and reviewable.
- Trust ladder: suggest-only → auto-commit-behind-approval → auto-fix-with-audit-log.
  Never silently change thresholds — the fixer maintains _tests_, not _standards_.

### 3.9 Repo topology & the open-core line

Stresseur is proprietary; the DevTools monorepo is public Apache-2.0. Mixed visibility
in one repo is impossible, so: **new private repo `stresseur/stresseur`** under the
existing org, created when Phase 2 starts (Phase 0/1 are entirely open-repo work).

The line that keeps two repos from becoming a coordination tax: **the contract lives
in the open repo.** TypeSpec definitions (MetricFrame / RunReport / FailureArtifact,
worker ingest protocol) stay in `packages/spec`; the open CLI emits them, and
Stresseur consumes the generated Go/TS packages by version. Stresseur never imports
DevTools internals — only published packages and the YAML/spec contracts. (Apache-2.0
makes reuse legally trivial; the separation is for IP hygiene, security of
billing/abuse code, independent deploy cadence, and a clean story under diligence.)

Open (trust + adoption assets): engine, CLI including load mode, YAML format, GitHub
Action, worker protocol spec, worker image Dockerfile — "the thing that fires requests
at your API is auditable" is a selling point for a load-testing product, and it
preempts what-does-the-worker-phone-home FUD. Closed (the business): control plane,
fleet orchestration, quotas/billing, baselines service, GitHub App, AI agents.
Undecided: the thin MCP shim (lean open — it's distribution; value stays server-side).
Never move an existing open capability closed — that's the community rug-pull this
plan is explicitly structured to avoid.

## 4. Phase 0 — foundations without breakage

**Goal:** cut every seam load testing needs, ship zero behavior change by default,
and retire the correctness debt that would otherwise become load-bearing.

### 4.0 Guardrail zero: characterization tests before surgery

Before touching the converter: **golden-file round-trip tests** (export→import→export
byte-stable across the testdata corpus + a new corpus covering every step type).
Lock current behavior first; then each intentional change lands with an updated golden
and a changelog entry. This is the mechanism that makes "we won't break things" a
property instead of a hope.

### 4.1 Work items

1. **YAML `version:` field.** Emit `version: 2` on export; accept absent/2 on import;
   clear error on >2. Old binaries ignore it (lenient parser — verified, no
   `KnownFields` anywhere).
2. **Fix assertion import drop** (`converter_node.go`): populate `HTTPAsserts` the way
   GraphQL already does; add round-trip assertion tests. ⚠️ Behavior change — see 4.2.
3. **Fix `run:` ordering**: topological sort from `depends_on`; **error** on unknown
   dep names instead of silent skip. ⚠️ Behavior change — see 4.2.
4. **Engine levers**: `CreateFlowRunner` functional options (`WithMaxConcurrency`);
   RunProfile scheduler skeleton (constant-vus only in Phase 0); lean execution mode
   flag threaded through node request context. Defaults preserve current behavior
   exactly; server call sites untouched.
5. **CLI local load mode**: `flow run --scenario <name>` (reads `load:` block) plus
   shorthand `--vus/--duration/--iterations`; prints aggregate table (p50/95/99, RPS,
   error rate); `--report json` gains additive RunReport fields. No flags → today's
   behavior, byte-identical output.
6. **TypeSpec: MetricFrame / RunReport / FailureArtifact** + codegen. HDR histogram
   dependency in Go (`hdrhistogram-go`); merge helper + tests.
7. **Benchmark harness**: measure RPS/worker for a simple-GET flow and a 5-step chained
   flow on dev hardware + one Fly shared-cpu machine class. Output feeds capacity math
   and pricing. (Engine was built for correctness, not throughput — this number gates
   Phase 2 promises.)
8. **CI**: extend `cli:test:integration` with scenario-mode cases; keep full server
   suite green (`-p 8`) as the no-breakage gate for the shared engine packages.

### 4.2 Behavior-change register (the "won't break things" contract)

Every intentional change, its blast radius, and its comms. Nothing else may change
observable behavior; golden tests enforce that.

| #   | Change                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Who feels it                                                                                                                           | Mitigation / comms                                                                                                                    |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | YAML assertions now enforced on import                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Files whose assertions were silently ignored may now fail runs — **their tests start testing**                                         | Minor version bump; changelog headline; release note shows how to delete/adjust assertions; docs page "why did my flow start failing" |
| 2   | `run:` executes in dependency order with strict failure modes: unknown flow/dep names abort **pre-flight** (nothing executes; previously flows before the bad entry ran first), malformed `run:` entries error instead of being silently skipped, dependency-failure skips are reported explicitly, and the aggregate failure message now includes the failing flow's name and status; a failed dependency now **skips** the dependent and **continues** the run — previously the entire run aborted at that point, so flows after the failure now execute where previously nothing did | Files listing flows out of dep order; typo'd flow/dep names that silently skipped flows; scripts parsing the old failure-message shape | Same release; error messages name the offending value and list valid flow names; changelog enumerates all four deltas                 |
| 3   | `version: 2` appears in exports                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | None (old parsers ignore unknown keys — verified)                                                                                      | Changelog note                                                                                                                        |
| 4   | New CLI flags / report fields                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | None (additive; JSON consumers ignoring unknown fields unaffected)                                                                     | Changelog note                                                                                                                        |

Release as **CLI/desktop minor** via Nx version plan (never manual bumps).
Explicitly _not_ in Phase 0 (logged, tracked, separate hygiene PRs): Windows JS-node
socket fix, `devtools version` string, stale `${var}` examples cleanup, dead
`flow.timeout/metadata` fields, GraphQL/WS export shapes.

### 4.3 Phase 0 acceptance

- `devtools flow run f.yaml` output is byte-identical to pre-change for the golden corpus.
- `devtools flow run f.yaml --vus 50 --duration 60s` produces a RunReport with correct
  percentiles (validated against a reference implementation on synthetic latencies).
- Round-trip retains assertions; `run:` respects dependencies; full server + CLI suites green.
- Benchmark note published in-repo: RPS/worker for the two reference flows.

## 5. Phases 1–3 — the attention plan

Each phase ends in a **launch**, each launch is standalone news, each widens the funnel
for the next. Sequencing: credibility (DevTools, free, OSS) → distribution (Stresseur
PR bot) → differentiation (Stresseur AI).

### Phase 1 — "Your recorded traffic is now a test suite" (DevTools, free wedge)

**Ships:** published GitHub Action (`devtools/run-flows@v1`: install binary, run flows,
JUnit + PR annotation + job summary); local load mode polished; desktop load-results
view (charts: latency percentiles over time, RPS, errors).
**Launches:**

- _Show HN: record your API traffic in Chrome, replay it as CI tests_ — the
  record→flow→PR-gate loop demoed in 90 seconds. OSS + local + no signup = HN-native.
- _Load test from your laptop with the tests you already have_ — dev-Twitter/Reddit
  follow-up; k6 comparison content ("no script, same flow").
  **Metric:** Action installs; weekly PRs gated by a DevTools check.

### Phase 2 — "Tag the bot, get a load test" (the Stresseur launch)

**Ships:** control plane + Fly fleet, target verification, plans/quotas/queueing,
baselines & history, Stresseur GitHub App with `@stresseur load <scenario>` comment
trigger and rich result comments, billing (seats + metered VU-hours).
**Launches:**

- _`@stresseur load` on any PR_ — the cursor-review-style motion; public-repo comments
  are the growth loop. Free tier for OSS repos to seed visibility.
- _Perf budgets as PR gates_ — "don't merge if p95 regresses" with baseline diffs.
- Design-partner case studies (5–10 agent-native teams recruited during Phase 1).
  **Metric:** orgs with ≥1 cloud run/week; VU-hours; % of runs triggered from PRs.

### Phase 3 — "The test suite that maintains itself" (Stresseur AI, the moat)

**Ships:** Stresseur MCP server; bug bot triage; YAML fixer with suggested commits;
scheduled runs; multi-region scenarios.
**Launches:**

- _Your agent load-tests before it merges_ — MCP demo inside Claude Code/Cursor:
  agent ships endpoint → runs scenario → reads report → adjusts → merges.
- _Self-healing API tests_ — the fixer patching a flow live on a real PR. This is the
  viral AI moment; time it with a model-partner or launch-week slot if possible.
  **Metric:** fixer-suggested commits merged; MCP tool-calls/week; net revenue retention.

## 6. Pricing shape (v1, keep simple)

- **DevTools (free forever)** — desktop, CLI, Action, local load: unlimited. No paywall
  ever crosses into the platform.
- **Stresseur Free** — cloud taste-tier (see quota table in §3.6).
- **Stresseur Team ($/seat/mo)** — history/baselines, PR bot, org management + included
  VU-hour credits; overage metered.
- **Stresseur Scale (custom)** — high concurrency, all regions, SSO, audit, private
  regions.

VU-hour COGS on Fly shared-cpu machines is cents/hour → healthy margin on metered;
benchmark (Phase 0 item 7) finalizes the unit economics. Seats price the AI
maintenance + collaboration value, not the compute.

## 7. Risks & mitigations

| Risk                                            | Mitigation                                                                                                            |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Engine RPS/worker disappoints                   | Phase 0 benchmark gates all Phase 2 promises; lean mode; worker fan-out is linear; k6 adapter remains an escape hatch |
| Assertion fix breaks users' green builds        | It un-breaks them (tests silently asserted nothing); comms per register 4.2; version-gated release                    |
| Hosted load-gen abuse                           | Target verification launch-blocking; egress caps; blocklist; per-org fairness                                         |
| eventstream is in-memory/single-node            | Cloud uses durable bridge (decision in Phase 2 plan); local behavior unchanged                                        |
| YAML becomes public contract while under-tested | Guardrail zero: golden round-trip corpus before any converter change                                                  |
| Free tier funds nothing                         | It's the funnel by design; costs are customer-side (CI) or laptop-side; cloud free tier is capped tightly             |
| Fly single-provider dependency                  | Worker protocol is provider-agnostic (machine = container + token + ingest URL); second provider addable later        |

## 8. Open decisions (deferred, with owners-to-be)

1. Durable event bridge for cloud (LibSQL poll vs NATS) — Phase 2 plan.
2. Control-plane DB (LibSQL/Turso vs Postgres) — Phase 2 plan; start LibSQL.
3. Env-mapping config for the GitHub App (how PR → target URL) — Phase 2. Includes the
   file's name: `stresseur.yml` vs `devtools.yml` (the DevTools Action and Stresseur
   App may share one file; brand split argues for `stresseur.yml` owning cloud concerns).
4. OSS-repo free-tier abuse guardrails (verification still required?) — Phase 2.
5. Fixer trust ladder defaults (suggest-only at launch) — Phase 3.
6. Whether `constant-arrival-rate` (open-model workload) ships in Phase 1 or 2 — after
   Phase 0 benchmark shows scheduler overhead.

## 9. Out of scope (this design)

- Replacing k6/Gatling for extreme-RPS synthetic benchmarking (>50k RPS single-target).
- Browser-level load testing (we load-test APIs, not pages).
- Windows CLI JS-node fix and other §2 hygiene defects (tracked separately).
- Self-hosted cloud runners (possible later: "bring your own Fly org" — not v1).
