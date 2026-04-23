---
desktop: major
---

First stable release.

### New protocols and flow nodes

- **GraphQL requests**: full request editor with query/variables, dark theme tokens, CLI support, YAML export/import, response history, and delta overrides with assertions.
- **WebSocket**: connection and send flow nodes, request panel, and tables for persisting messages and headers.
- **Wait node**: pause flow execution for a configurable duration.
- **Sub-flow**: new Run Sub Flow node plus SubFlowTrigger and SubFlowReturn, enabling flows to invoke other flows with typed inputs/outputs.

### Flow engine

- Flow runner overhaul with improved node execution and error propagation.
- Flow-level error field and node ID mapping for more precise failure attribution.
- Copy/paste support extended to GraphQL, WebSocket, and sub-flow nodes.

### Expression editor

- Autocomplete for built-in functions inside `{{ }}`: `uuid()`, `uuid("v4")`, `uuid("v7")`, `ulid()`, `now()`.
- Dot-chain completion on `now()`: `.Unix()`, `.UnixMilli()`, `.UnixMicro()`, `.UnixNano()`.
- New `faker` namespace for fake data — type `faker.` to browse 35 generators including `name()`, `email()`, `phoneNumber()`, `url()`, `ipv4()`, `ipv6()`, `macAddress()`, `username()`, `password()`, `word()`, `sentence()`, `paragraph()`, `date()`, `timestamp()`, `uuid()`, `randomInt(min, max)`.

### Delta system

- GraphQL delta support with snapshot/override semantics for name, URL, query, variables, description, headers, and assertions.
