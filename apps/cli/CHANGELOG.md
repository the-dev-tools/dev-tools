## 1.4.0 (2026-10-10)

### 🚀 Features

- AI checks are now part of the engine. A request or graphql step's `expect:` block (the Stresseur syntax, unchanged) is validated on import and evaluated by `devtools flow run` after the flow's steps finish, so it never counts in step timings. Results are printed under the flow's table and written to the JSON report as each step's `checks[]` (`kind`, `passed`, `score`, `reason`, `judge_model`, `cached`, `value`, `limit`). ([4b494b2f](https://github.com/the-dev-tools/dev-tools/commit/4b494b2f))

  - **Checks:** `schema`/`schema_at`, `max_latency_ms`, the new `max_ttft_ms`, token budgets from `usage`, and a G-Eval `judge` (Anthropic or any OpenAI-compatible API). The judge uses injection-resistant prompting, logprob weighting or borderline re-sampling, and runs in parallel.
  - **Judge key:** `STRESSEUR_JUDGE_API_KEY`, `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`, from the environment or `./.env`. Without one, judge checks are skipped with a notice.
  - **Cache:** verdicts are cached in `--judge-cache`, default `.devtools/judge-cache.json`.
  - **Exit code:** failed checks fail a flow only with `quality: { fail_below: … }`.

  Server-Sent Events responses are now read as they arrive. A text/event-stream response streams automatically; a request step's `stream: openai | anthropic | vercel-ai | sse` picks the preset that assembles the text, and `stream_timeout_ms` bounds an open stream (default 30 s). Assertions, `expect:` and later steps can read `response.text`, `response.events`, `response.event_count`, `response.ttft_ms` and the streamed `response.usage`.

### ❤️ Thank You

- moosebay

## 1.3.1 (2026-10-10)

### 🩹 Fixes

- Fix four flow-engine bugs that could let a run pass when it should fail, or slow it down: ([28a953f1](https://github.com/the-dev-tools/dev-tools/commit/28a953f1))

  - `response.duration` in `assertions:` is now the measured request time in ms. It was always 0, so `response.duration < N` never failed.
  - A failed assertion now shows each value it checked, e.g. `(got response.duration = 5120)` or `(got response.body.name = "Outdoor")`, instead of only the status code. Values under names like `token` or `password` are redacted. When a body field is compared on a non-JSON response, such as a 404 HTML page, the error now names the status and content type instead of only `invalid operation: int(string)`.
  - JS steps no longer add a fixed ~3 s per run. The worker is polled every 25 ms and is ready in about 0.1 s.
  - Import now rejects a step that reads another step's output, such as `{{ Ask.response.body.answer }}`, without depending on that step. The error names the step and the missing `depends_on`. Before, such a step ran at flow start with the template unfilled, and the flow still passed.

### ❤️ Thank You

- moosebay

## 1.3.0 (2026-10-09)

### 🚀 Features

- Load mode grows from constant-vus to four executors: `ramping-vus` (stages with graceful ramp-down), and the open-model `constant-arrival-rate` and `ramping-arrival-rate`, which start iterations on schedule whatever the response time and report dropped iterations once `max_vus` are busy. A `load:` entry can add `think_time`, `thresholds` (e.g. `p95: <300ms`, `errors: <1%`, per step under `steps:`), checked at the end and failing the run with exit code 99, and `abort` rules evaluated every metrics frame that stop a run early with exit code 108. `--report frames:<url>` streams every metrics frame, with per-step HDR histograms, and the final report to an HTTP endpoint (bearer token from `DEVTOOLS_FRAMES_TOKEN`, retried in the background, and the run stops if the endpoint is unreachable for 60s); `--vus-scale`/`--rate-scale` split one profile across machines, `--load-file` reads scenarios from a separate file with the same schema, and the summary now shows the request count it is based on. ([c5002b3d](https://github.com/the-dev-tools/dev-tools/commit/c5002b3d))

### ❤️ Thank You

- moosebay

## 1.2.2 (2026-10-07)

### 🩹 Fixes

- The CLI no longer prints "Config file not found, creating default config file" or writes a default `~/.devtools.yaml` on first run; a config file is still read when present. ([1dff1083](https://github.com/the-dev-tools/dev-tools/commit/1dff1083))

### ❤️ Thank You

- moosebay

## 1.2.1 (2026-10-07)

### 🩹 Fixes

- The CLI can be embedded in another Go program: `cmd.Root()` returns its root command, and each release publishes `devtools-go-src-<version>.tar.gz`, the Go source of the CLI and the modules it imports with the generated code included, to build against. ([03382c8b](https://github.com/the-dev-tools/dev-tools/commit/03382c8b))

### ❤️ Thank You

- moosebay

## 1.2.0 (2026-10-07)

### 🚀 Features

- Flows can now declare `cleanup:` steps that always run after the flow's normal steps, whether they passed or failed, so data a test creates is deleted even when a step in the middle fails and the next run starts from the same state. Cleanup steps are `request` or `graphql` steps that can read earlier outputs (`{{ PostProducts.response.body.id }}`); they run in listed order (`depends_on` may reorder them among themselves), a step that reads a step which never ran is reported as skipped, and a failing cleanup step fails an otherwise passing flow while an earlier failure stays the reported error. Cleanup steps appear after the normal steps in the console table and in JSON and JUnit reports, and run after every iteration in load runs without being measured. ([f204ce9e](https://github.com/the-dev-tools/dev-tools/commit/f204ce9e))
- Flows can read the cookies they've received: `{{ cookies.csrftoken }}`, or `{{ cookies["XSRF-TOKEN"] }}` for names that aren't identifiers. This makes double-submit CSRF protection (Django, Laravel, Angular, many Express apps) testable: copy the CSRF cookie into the header the server checks. Values are URL-decoded the way client code reads them, the latest value of each cookie wins, and a flow variable named `cookies` takes precedence. ([60054c5f](https://github.com/the-dev-tools/dev-tools/commit/60054c5f))

### ❤️ Thank You

- moosebay

## 1.1.3 (2026-10-07)

### 🩹 Fixes

- A failing assertion now prints one line that shows what the server answered: `Error: assertion failed: response.status == 200 (got 401: {"error":"Invalid credentials"})`. Non-2xx responses include a short body excerpt; 2xx bodies are left out because they can carry tokens. The flag reference no longer prints under a failing flow, and the error no longer repeats four times. ([c8520dd6](https://github.com/the-dev-tools/dev-tools/commit/c8520dd6))

### ❤️ Thank You

- moosebay

## 1.1.2 (2026-10-07)

### 🩹 Fixes

- Load runs now work for flows whose steps use an earlier step's response, such as a login token passed to the next request. Lean mode used to drop every response body, so those steps failed on every iteration and the run reported the target as unreachable. Bodies a later step reads are now kept; the rest are still dropped to keep memory flat. ([83a2a5b4](https://github.com/the-dev-tools/dev-tools/commit/83a2a5b4))
- Assertions and conditions can now compare numbers from JSON responses: `response.body.qty == 3`, `response.body.price > 100` and arithmetic work. Response numbers used to compare as text, so equality was always false and `>`/`<` failed with a type error. Values copied into later requests with `{{ }}` keep their exact original text, as before. ([3cd2d960](https://github.com/the-dev-tools/dev-tools/commit/3cd2d960))

### ❤️ Thank You

- moosebay

## 1.1.1 (2026-08-09)

### 🩹 Fixes

- Fix Windows release builds: platform variables now resolve shell-agnostically, so win32-x64 and win32-ia32 binaries build and cross-compile correctly. Republishes all six CLI platform binaries. ([ad36e8dc](https://github.com/the-dev-tools/dev-tools/commit/ad36e8dc))

### ❤️ Thank You

- moosebay

## 1.1.0 (2026-08-08)

### 🚀 Features

- Load testing foundations: local load mode (`flow run --vus/--duration` and `load:` scenarios with percentile tables + `load_report` JSON), versioned yamlflow schema (`version: 2`), HTTP assertions in yamlflow files now enforced on import (previously silently dropped), `run:` blocks execute in dependency order with strict failure modes (failed dependencies skip-and-continue instead of aborting the run), `run-flows` GitHub Action for CI, and real build-version reporting in `devtools version`. ([9a3152a3](https://github.com/the-dev-tools/dev-tools/commit/9a3152a3))

### ❤️ Thank You

- moosebay

## 1.0.3 (2026-07-05)

### 🩹 Fixes

- ### Bug fixes ([9ba98e85](https://github.com/the-dev-tools/dev-tools/commit/9ba98e85))

  - **Windows binaries restored.** A stale `@nx/eslint` patch made dependency installation fail hard on Windows CI, so 1.0.2 shipped without `win32-x64`/`win32-ia32` binaries. The dead patch is removed; this release ships all platforms again. Includes the file `display_order` repair migration from 1.0.2 ([#44](https://github.com/the-dev-tools/dev-tools/issues/44)).

### ❤️ Thank You

- moosebay

## 1.0.2 (2026-07-05)

### 🩹 Fixes

- ### Bug fixes ([#44](https://github.com/the-dev-tools/dev-tools/issues/44))

  - **Startup migration repairs pathological file ordering.** The embedded server now repacks `files.display_order` values that converged to float32 MAX (a bug in desktop order generation) back to small sequential numbers, preserving relative order. Databases shared with a broken desktop workspace recover automatically. ([#44](https://github.com/the-dev-tools/dev-tools/issues/44))

### ❤️ Thank You

- moosebay

## 1.0.1 (2026-05-05)

### 🩹 Fixes

- ### Bug fixes ([#42](https://github.com/the-dev-tools/dev-tools/issues/42))

  - **Loop break condition now sees inner-node outputs.** For/ForEach break expressions are evaluated **after** each iteration's children run, so they can reference values produced during that iteration (e.g. `{{ http_1.response.body.done }}`). Previously the check ran before children, so any expression referencing a not-yet-written variable failed the entire flow on the first iteration. Missing identifiers are now treated as "don't break" (loops are still bounded by iteration count). ForEach semantics also aligned with For: an expression that evaluates true exits the loop. ([#42](https://github.com/the-dev-tools/dev-tools/issues/42))

  ### Other

  - New `break_condition` field on `for` / `for_each` steps in YAML workspaces, so loops in CLI-driven flows can exit on a runtime predicate without needing the UI.

### ❤️ Thank You

- moosebay

# 1.0.0 (2026-04-24)

### 🚀 Features

- First stable release. ([f31075cf](https://github.com/the-dev-tools/dev-tools/commit/f31075cf))

  ### New protocols and flow nodes

  - **GraphQL requests**: query/variables, assertions, response history, YAML export/import.
  - **WebSocket**: connection and send flow nodes with message capture.
  - **Wait node**: pause flow execution for a configurable duration.
  - **Sub-flow**: Run Sub Flow node plus Sub-Flow Trigger and Sub-Flow Return for composing flows.

  ### Flow engine

  - Flow runner overhaul with improved node execution and error propagation.
  - Flow-level error field and node ID mapping for more precise failure attribution.

  ### Expression editor

  - Built-in `uuid()`, `uuid("v4")`, `uuid("v7")`, `ulid()`, `now()` helpers inside `{{ }}`.
  - Dot-chain on `now()`: `.Unix()`, `.UnixMilli()`, `.UnixMicro()`, `.UnixNano()`.
  - `faker.*` namespace (35 generators — `name()`, `email()`, `phoneNumber()`, `url()`, `ipv4()`, `word()`, `sentence()`, `paragraph()`, `date()`, `timestamp()`, `uuid()`, `randomInt(min, max)`, ...) for fake test data.

  ### AI

  - AI agent with tool execution, streaming, and multi-provider support (OpenAI, Anthropic, Gemini), credential vault encryption, and variable introspection.

### ❤️ Thank You

- moosebay

## 0.2.2 (2026-02-26)

### 🩹 Fixes

- Fix AI node export and credential env var name sanitization ([36fd3671](https://github.com/the-dev-tools/dev-tools/commit/36fd3671))

### ❤️ Thank You

- ElecTwix @ElecTwix

## 0.2.1 (2026-02-09)

### 🩹 Fixes

- Revert env vars from {{ env.varName }} back to flat {{ varName }} syntax ([b4914257](https://github.com/the-dev-tools/dev-tools/commit/b4914257))

### ❤️ Thank You

- ElecTwix @ElecTwix

## 0.2.0 (2026-02-07)

### 🚀 Features

- Add AI node support with multi-provider LLM integration (OpenAI, Anthropic, Gemini), credential vault encryption, and variable introspection system ([fb11df2a](https://github.com/the-dev-tools/dev-tools/commit/fb11df2a))

### 🩹 Fixes

- Fix JavaScript node result encoding ([7cfbd0dd](https://github.com/the-dev-tools/dev-tools/commit/7cfbd0dd))
- Show stack trace for JavaScript node errors ([0697ebc8](https://github.com/the-dev-tools/dev-tools/commit/0697ebc8))

### ❤️ Thank You

- ElecTwix @ElecTwix
- Tomas Zaluckij @Tomaszal

## 0.1.0 (2026-01-06)

### 🚀 Features

- First public release of DevTools Desktop and CLI apps! 🎉 ([e279c6d7](https://github.com/the-dev-tools/dev-tools/commit/e279c6d7))

### ❤️ Thank You

- Tomas Zaluckij @Tomaszal