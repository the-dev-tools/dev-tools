# Flow Execution Runtime Specification

## Architectural Overview

`packages/server/pkg/flow/runner/flowlocalrunner/flowlocalrunner.go` hosts the in-process scheduler responsible for evaluating flow graphs. The runner wraps the node map, edge map, and root node alongside bookkeeping such as `PendingAtmoicMap` (fan-in counters) and a pooled `tracking.VariableTracker`. Invocations are launched through `RunWithEvents`, which returns structured node state snapshots, human-readable log payloads, and aggregate flow status updates through a `runner.FlowEventChannels` bundle. The API surface (`packages/server/internal/api/rflow/rflow.go`) composes this runner with Connect RPC handlers, performs permission checks, pre-registers HTTP request executions to avoid racing side-channel responses, and multiplexes node events to database writers, WebSocket/SSE listeners, and execution logs.

```
+--------------+      +------------------+      +-----------------------+
| rflow RPC    | ---> | FlowLocalRunner   | ---> | FlowEventChannels      |
| (auth, I/O)  |      | (mode selection,  |      | - node states/logs     |
|              |      |  scheduling)      |      | - flow lifecycle       |
+--------------+      +------------------+      +-----------------------+
         |                   |                              |
         |                   v                              v
         |         +------------------+        +-----------------------+
         |         | Node Var System  |        | Tracking + Iteration  |
         |         | (sync.RWMutex,   |        | (runner.IterationCtx) |
         |         | base vars)       |        +-----------------------+
         v
+--------------------+
| Side channels      |
| (request resp bus) |
+--------------------+
```

## Scheduling Modes and Synchronisation

The runner supports three execution modes via `ExecutionMode`:

- **Auto** delegates to `selectExecutionMode`, which inspects node count, branching, and loop structure. Small flows (≤6 nodes, single inbound edges) run in **Single** mode for cache friendliness and reduced mutex pressure.
- **Single** mode (`runNodesSingle`) walks the queue sequentially, copying predecessor outputs into per-node inputs. It reuses a single goroutine, relies on the request-global `sync.RWMutex` for variable writes, and always emits RUNNING→terminal status transitions.
- **Multi** mode (`runNodesMultiNoTimeout` / `runNodesMultiWithTimeout`) slices the ready queue into batches of at most `MaxParallelism()` (min of `GOMAXPROCS` and `NumCPU`). Each batch fans out goroutines guarded by context deadlines. Predecessor completion is synchronised by `waitForPredecessors`, which waits on channel signals stored in a `sync.Map`. `PendingAtmoicMap` prevents premature dispatch when multiple upstream branches converge.

Shared optimisations:

- Variable tracking reuses instances from `trackerPool` to avoid heap churn on large flows.
- Cancellation propagates immediately: a failed node cancels the per-batch context and triggers `sendCanceledStatuses` so downstream listeners see consistent terminal states.
- For timeouts, `runNodesMultiWithTimeout` wraps node execution in per-node deadlines while still respecting global cancellation.

```
Single mode:
  queue --> [node] --success--> append(next)
           (mutex-guarded writes)

Multi mode:
  ready set         goroutine pool         completion fan-in
  [A B C D] ---> |spawn ≤ MaxParallelism| ---> resultChan --> status emitter
```

## Node Implementations

- **HTTP request node** (`packages/server/pkg/flow/node/nrequest/nrequest.go`) prepares requests via `request.PrepareRequestWithTracking`, logging sanitized headers and capturing read variables. It dispatches through `request.SendRequestWithContext` using the shared `httpclient`, then writes both request/response envelopes back into the flow variable map. Assertions are evaluated by `response.ResponseCreate`, which leverages `expression` helpers; failures are propagated while still streaming structured payloads over `NodeRequestSideRespChan`.
- **Loop nodes** (`nfor` and `nforeach`) materialise iteration contexts. They annotate `runner.IterationContext` with hierarchical labels, emit RUNNING/SUCCESS logs for every iteration, and invoke `flowlocalrunner.RunNodeSync` recursively for loop bodies. `nfor` iterates a fixed count with optional break condition, whereas `nforeach` evaluates an iterable expression (slice or map) through `expression.ExpressionEvaluateAsIter` and writes `item`, `key`, and `totalItems` bindings. Both honour `mnfor.ErrorHandling` policies to ignore, break, or surface errors.
- **JavaScript node** (`njs`) serialises the current variable map into protobuf (`structpb.Value`), forwards it to the external NodeJS executor over Connect RPC, and writes raw results back into the var map. Errors are wrapped so the runner can mark the node failed without corrupting shared state.
- **No-op node** (`nnoop`) simply forwards control to the next edge handle; it is frequently used as a join/label anchor.

## HTTP Request/Response Pipeline

The request package performs variable substitution, MIME-aware encoding (including multipart form boundaries), and security scrubbing before logging. It also redacts sensitive headers (Authorization) and truncates large bodies for audit logs. Response handling persists canonical copies of headers with delta detection, compresses large bodies using Zstd, and evaluates assertions against a merged environment built from flow variables plus the immediate response binding. The assertion engine (`packages/server/pkg/expression/expression.go`) normalises templated strings (supports `{{var}}` syntax), compiles expressions with caching (`expr` VM), and converts complex Go structs into JSON-like maps while avoiding repeated allocations.

```
[Flow vars] --replace--> request.Prepare --> httpclient --> response.ResponseCreate
      ^                          |                |                 |
      |                          |                v                 v
      |                    LogPreparedRequest    Lap timer      AssertCouple[]
      |                                                         (expression VM)
      +-------------tracker.TrackRead/Write<-----------------------------+
```

## API Surface and Data Flow (`rflow`)

`packages/server/internal/api/rflow/rflow.go` orchestrates incoming flow runs. It resolves flow graphs, merges overlays, constructs node instances (wrapping request nodes in `preRegisteredRequestNode` so webhooks can match executions), and wires the runner into Connect-streamed RPCs. The module multiplexes node status events to:

1. persistence layers (`snodeexecution`, `sassertres`) via batch upserts with retry,
2. live subscribers (console logs, request inspector) through channels, and
3. tag/example services for analytics updates.

It also hydrates environment variables, attaches HTTP clients with credential scopes, and ensures workspace permissions via `permcheck` before any runner call. Multi-tenant concerns (per-workspace caches, TTL) are addressed by `cachettl` guarded fetches, ensuring small flows spin up quickly by reusing cached node definitions while large flows rely on streaming updates rather than blocking RPC responses.

## Behaviour on Small vs Large Flows

- **Small, linear flows** favour `ExecutionModeSingle`, minimising context switching and maximising locality; variable writes occur under a single `sync.RWMutex`, and node outputs are flattened to keep the var map shallow.
- **Large or branched flows** flip to multi-mode, exploiting CPU parallelism while coordinating dependencies with predecessor signals. The queueing logic keeps fan-in gates (`PendingAtmoicMap`) accurate so nodes only dispatch when all parents succeed. Memory pressure is mitigated by reusing trackers and by emitting flattened output snapshots lazily (only when tracking is enabled or on error paths).
- Loop-heavy flows rely on iteration contexts to avoid duplicate log spam and to keep nested executions traceable. Each loop iteration creates isolated execution IDs so multi-threaded downstream nodes can still correlate to their parent loop.

## Synchronisation and Safety Guarantees

Every node execution clones the read-only parts of `FlowNodeRequest` so goroutines never share `ExecutionID` or tracker state. Writes funnel through helper functions (`node.WriteNodeVar*`) that honour the `sync.RWMutex` inside the request. Context cancellation (user aborts, timeouts, assertion failures) bubbles through `context.Context` so pending goroutines exit early; cleanup code emits CANCELED states for anything still running, ensuring observers never see dangling RUNNING entries. These patterns keep the engine safe under multithreaded load without sacrificing determinism for single-threaded scenarios.
