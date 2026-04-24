# 1.0.0 (2026-04-24)

### 🚀 Features

- First stable release. ([364e2296](https://github.com/the-dev-tools/dev-tools/commit/364e2296))

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