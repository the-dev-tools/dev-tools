---
cli: major
---

First stable release.

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
