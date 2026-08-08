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