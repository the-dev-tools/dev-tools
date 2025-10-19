# Server Testing Guide

This document explains how to write fast, safe, and deterministic tests for the Go server using in‑memory SQLite and the existing test helpers in this repo.

## TL;DR

- For service/repository tests: use `sqlitemem.NewSQLiteMem` (single connection, pure Go sqlite, fastest).
- For API/RPC tests that require multiple components sharing the same DB: use `testutil.CreateBaseDB` (shared in‑memory sqlite DSN).
- Give every test its own database. Keep transactions short and commit before invoking code that reads the DB.
- Seed state with `BaseTestServices.CreateTempCollection` and use `mwauth.CreateAuthedContext` for authed RPC calls.

## Building Blocks

- `packages/server/pkg/testutil/testutil.go`
  - `CreateBaseDB(ctx, t)`: returns a `BaseDBQueries` with a unique, shared in‑memory SQLite DB per call (via `dbtest.GetTestDB`). Good for multi‑component tests (RPC + services).
  - `BaseDBQueries.GetBaseServices()`: returns ready‑to‑use services (collection, user, workspace, workspace-user).
  - `BaseTestServices.CreateTempCollection(...)`: quickly creates a workspace, user, workspace-user, and collection.
  - `BaseTestServices.CreateCollectionRPC()`: constructs an RPC bound to the same DB/services.

- `packages/db/pkg/sqlitemem/sqlitemem.go`
  - `NewSQLiteMem(ctx)`: constructs a true `:memory:` sqlite database (pure Go `modernc.org/sqlite`) with tables created via `sqlc.CreateLocalTables`.
  - Sets `MaxOpenConns(1)`; ideal for service/repository tests that stay within one DB instance.

- `packages/db/pkg/dbtest` (Linux/Mac/Windows variants)
  - `GetTestDB(ctx)`: provides a unique, shared in‑memory sqlite DSN like `file:testdb_<ulid>?mode=memory&cache=shared` (CGO driver). Multiple connections share the same in‑memory DB within the same process.

## When To Use What

- Use `sqlitemem.NewSQLiteMem` when:
  - You are testing a single service/repository layer with one `*sql.DB` (fastest startup, no CGO).
  - Your test can run entirely within a single connection and short transactions.

- Use `testutil.CreateBaseDB` when:
  - Multiple modules/services/RPCs need to share the same in‑memory DB and may use separate connections.
  - You want the convenience of `BaseTestServices` seeding helpers.

## Quickstart Patterns

### Service/Repository test (fastest)

```go
func TestService_FastMem(t *testing.T) {
    ctx := context.Background()
    db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
    require.NoError(t, err)
    t.Cleanup(cleanup)

    queries, err := gen.Prepare(ctx, db)
    require.NoError(t, err)

    svc := yourservice.New(queries, logger)

    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err)
    defer tx.Rollback()

    // Write state within TX
    err = svc.CreateSomethingTX(ctx, tx, params)
    require.NoError(t, err)
    require.NoError(t, tx.Commit()) // Make visible to subsequent reads

    // Call read paths or additional APIs that expect committed data
    got, err := svc.GetSomething(ctx, id)
    require.NoError(t, err)
    assert.NotNil(t, got)
}
```

### API/RPC test (shared DB across components)

```go
func TestAPI_SharedDB(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    t.Cleanup(base.Close)
    services := base.GetBaseServices()

    // Seed: workspace, user, workspace-user, collection
    wsID, wuID, userID, colID := idwrap.NewNow(), idwrap.NewNow(), idwrap.NewNow(), idwrap.NewNow()
    services.CreateTempCollection(t, ctx, wsID, wuID, userID, colID)

    // RPC that shares the same DB and services
    rpc := rcollection.New(base.DB, services.Cs, services.Ws, services.Us)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Exercise RPC
    req := connect.NewRequest(&collectionv1.CollectionGetRequest{CollectionId: colID.Bytes()})
    resp, err := rpc.CollectionGet(authed, req)
    require.NoError(t, err)
    assert.Equal(t, colID.Bytes(), resp.Msg.Collection.Id)
}
```

### Mixed components sharing one DB handle

If a test constructs an RPC and also needs direct `Queries` access in the same connection pool:

```go
queries, err := gen.Prepare(ctx, rpc.DB) // use the same *sql.DB as the RPC
require.NoError(t, err)
// Now create services with `queries` to guarantee shared visibility
```

## Transactions & Visibility

- Insert/seed using `*_TX` service methods inside a transaction, then `Commit()` before invoking code that reads the DB (RPCs or services that don’t share that TX).
- For truly isolated sequences, prefer one DB per test and use new transactions per sub‑scenario.

## Parallelism

- It’s safe to use `t.Parallel()` when each test constructs its own DB (`sqlitemem.NewSQLiteMem` or `testutil.CreateBaseDB`).
- Avoid sharing the same `*sql.DB` across `t.Parallel()` subtests unless each subtest creates its own DB instance.

## Avoiding “database is locked”

- Keep transactions short; do writes in TX then commit; avoid long‑running concurrent writes.
- Use one DB per test (no cross‑test sharing).
- `sqlitemem` sets `MaxOpenConns(1)` to eliminate multi‑writer contention inside a test.
- With `dbtest.GetTestDB`, the shared in‑memory DSN supports multiple connections; keep concurrent writes minimal. If needed, we can tune the DSN (e.g., `_busy_timeout=5000`, `_foreign_keys=true`) in `dbtest`.

## Seeding & Auth

- Use `BaseTestServices.CreateTempCollection` for quick setup of workspace, user, workspace-user, and collection.
- Wrap contexts with `mwauth.CreateAuthedContext(ctx, userID)` for RPCs that require authentication/authorization.

## Recommended Defaults

- Service/repository: `sqlitemem.NewSQLiteMem` + transactions.
- RPC/API: `testutil.CreateBaseDB` + `BaseTestServices` helpers + authed context.
- One DB per test; commit before reads; keep tests independent and short.

## Checklist

- [ ] One DB per test (`sqlitemem` or `testutil`)
- [ ] Short TXs; `Commit()` before downstream reads
- [ ] Seed via `CreateTempCollection` (if needed)
- [ ] Use `mwauth.CreateAuthedContext` for authed RPCs
- [ ] Prefer table‑driven tests and `t.Run` for clarity
- [ ] Use `t.Parallel()` only when each subtest has its own DB

## General Go Testing Patterns

These patterns help make tests deterministic, readable, and race‑free beyond DB usage.

### Table‑Driven + Subtests

- Define a small struct of inputs/expectations; iterate with `t.Run(tc.name, func(t *testing.T){ ... })`.
- Isolate setup per case; prefer one DB or one runner instance per subtest.
- Use `t.Parallel()` only if each subtest creates its own resources (DB, temp dirs, ports).

### Contexts and Deadlines

- Always use `context.WithTimeout` in tests; avoid `context.Background()`.
- Pass cancelable contexts into goroutines; in goroutines, check `ctx.Err()` and return early to avoid leaks.
- Use `t.Cleanup(cancel)` to guarantee teardown.

### Deterministic Concurrency

- Prefer synchronizing with channels or `sync.WaitGroup` over `time.Sleep`.
- If you buffer channels, size them based on expected batch size to avoid unintentional blocking.
- When consuming a stream, assert after the producing goroutine closes the channel (use a `done` chan to know when to stop reading).

### Assertions and Diffs

- Standard library: `testing` is enough for most cases.
- For better diffs on complex structs or maps, use `go-cmp`:
  - `if diff := cmp.Diff(want, got); diff != "" { t.Fatalf("(-want +got)\n%s", diff) }`
- Optionally `stretchr/testify` for `require`/`assert` convenience or `mock` when stubbing deps.

### Race Detector and Flake Hardening

- Run with `-race -count=1` in CI to catch data races and avoid cached results hiding flakes.
- Keep per‑test timeouts short (100–500ms) to detect hangs quickly.
- Ensure all goroutines exit: close channels you own; cancel contexts you created.

### Fuzzing Critical Transforms

- For JSON normalization/compression paths, add fuzz tests (`go test -fuzz=Fuzz -run=^$`) to catch edge cases and panics.

## Runner/Streaming Specific Tips

The flow runner and streaming API are concurrent; prefer these testing tactics:

- Build minimal node graphs with tiny fake nodes (e.g., a node returning an error, a cancel‑aware node respecting `ctx.Done()`), then assert final per‑node states:
  - Failing node → `FAILURE`
  - Nodes aborted by failure → `CANCELED`
  - Explicit cancel (e.g., `runner.ErrFlowCanceledByThrow`) → `CANCELED`
  - Async per‑batch timeout (deadline exceeded) → `FAILURE` (by design)
- When testing stream processing, read until the producer closes the channel, then assert on the collected states instead of asserting mid‑stream.
- Persisting RUNNING vs terminal states: if you stub services, record call ordering and assert terminal upserts aren’t regressed by RUNNING upserts.

## Flow Node Testing Framework

The server includes a comprehensive testing framework for flow nodes located at `packages/server/pkg/flow/node/testing/`. This framework provides consistent patterns, reduced boilerplate, and built-in validation for all node types.

### Quick Start

```go
import nodetesting "the-dev-tools/server/pkg/flow/node/testing"

func TestMyNode(t *testing.T) {
    creator := func() node.FlowNode {
        return mynode.New(idwrap.NewNow(), "test", config)
    }

    opts := nodetesting.DefaultTestNodeOptions()
    opts.EdgeMap = edge.EdgesMap{
        creator().GetID(): {
            edge.HandleThen: {idwrap.NewNow()},
        },
    }

    nodetesting.TestNodeSuccess(t, creator(), opts)
}
```

### Framework Benefits

- **Reduced boilerplate**: ~50 lines → ~10 lines per test
- **Consistent patterns**: All nodes use the same testing approach
- **Built-in validation**: Status event handling, timeout testing, error scenarios
- **Table-driven support**: Easy to add multiple test cases
- **Node-specific helpers**: FOR loops, IF conditions, variable operations

### Available Node Types

The framework supports all node types:

- **FOR nodes**: Status event collection, iteration testing
- **FOREACH nodes**: Collection iteration support
- **IF nodes**: Condition evaluation and branch selection
- **NOOP nodes**: Basic execution testing
- **START nodes**: Entry point testing
- **JS nodes**: JavaScript execution (requires creator function)
- **REQUEST nodes**: HTTP request testing (requires creator function)

### Core Test Functions

```go
// Basic success test
nodetesting.TestNodeSuccess(t, node, opts)

// Error handling test
nodetesting.TestNodeError(t, node, opts, func(req *node.FlowNodeRequest) {
    req.Timeout = 1 * time.Nanosecond // Force error
})

// Timeout behavior test
nodetesting.TestNodeTimeout(t, node, opts)

// Async execution test
nodetesting.TestNodeAsync(t, node, opts)

// Multiple test cases
testCases := []nodetesting.NodeTestCase{
    {Name: "Success", TestFunc: successTest},
    {Name: "Error", TestFunc: errorTest},
}
nodetesting.RunNodeTests(t, node, testCases)
```

### Built-in Test Suites

Use the framework's built-in test suites for comprehensive coverage:

```go
// Run all built-in tests for a node type
tests := nodetesting.IFNodeTests()
for _, testCase := range tests.TestCases {
    t.Run(testCase.Name, func(t *testing.T) {
        testNode := tests.CreateNode()
        testCase.TestFunc(t, nodetesting.NewTestContext(t), testNode)
    })
}

// Run tests for all supported node types
allTests := nodetesting.AllNodeTests()
for nodeType, nodeTests := range allTests {
    t.Run(nodeType, func(t *testing.T) {
        // Run nodeTests.TestCases...
    })
}
```

### Test Context Helpers

The framework provides a rich test context for complex scenarios:

```go
ctx := nodetesting.NewTestContext(t)

// Create requests with defaults
req := ctx.CreateNodeRequest(nodeID, nodeName)

// Create with custom options
req := ctx.CreateNodeRequest(nodeID, nodeName, nodetesting.NodeRequestOptions{
    VarMap: map[string]any{"key": "value"},
    Timeout: 5 * time.Second,
})

// Create iteration contexts for loops
iterCtx := ctx.CreateIterationContext(1, 3)

// Status collection and validation
collector := ctx.GetStatusCollector()
validator := ctx.GetStatusValidator()
```

### Migration Guide

Existing manual tests can be easily migrated to the framework. See `packages/server/pkg/flow/node/testing/MIGRATION.md` for detailed examples and step-by-step instructions.

Typical migration:

```go
// Before: 50+ lines of manual setup
func TestMyNode_Manual(t *testing.T) {
    id := idwrap.NewNow()
    node := mynode.New(id, "test", config)
    // ... lots of boilerplate ...
    result := node.RunSync(ctx, req)
    // ... manual assertions ...
}

// After: 10 lines with framework
func TestMyNode_Framework(t *testing.T) {
    creator := func() node.FlowNode {
        return mynode.New(idwrap.NewNow(), "test", config)
    }
    opts := nodetesting.DefaultTestNodeOptions()
    nodetesting.TestNodeSuccess(t, creator(), opts)
}
```

### Framework in CI

The node testing framework integrates seamlessly with existing CI:

- **No additional setup required**: Framework tests run with standard `go test`
- **JSON output compatible**: Works with `task test:ci` for CI reporting
- **Race detection ready**: Use `-race` flag as with other tests
- **Parallel test friendly**: Each test gets isolated context

Run framework tests in CI:

```bash
# All node tests
go test ./pkg/flow/node/testing/... -race -count=1

# Specific node type tests
go test ./pkg/flow/node/nif -v -run TestIFNode_Framework

# Framework integration tests
go test ./pkg/flow/node -v -run TestNodeVariableOperations_Framework
```

### Documentation

- **README.md**: Comprehensive framework documentation
- **QUICKSTART.md**: 5-minute getting started guide
- **MIGRATION.md**: Step-by-step migration examples
- **TESTING_STRATEGY.md**: When to use normal unit tests vs framework
- **example_test.go**: Working code examples

### Testing Strategy: Normal Unit Tests vs Framework

Understanding when to use each approach is crucial for effective testing:

- **Normal Unit Tests**: Use for business logic, algorithms, utilities, and data transformations
- **Flow Node Framework**: Use for node execution patterns, status events, and integration behavior

See `packages/server/pkg/flow/node/testing/TESTING_STRATEGY.md` for a comprehensive guide with examples, decision matrices, and best practices.

## Command Cheatsheet

- Run all server tests with race:
  - `cd packages/server && go test ./... -race -count=1`
- Focus a package:
  - `go test ./pkg/flow/runner/flowlocalrunner -race -count=1`
- Run node testing framework:
  - `go test ./pkg/flow/node/testing/... -v`
- Run framework-based node tests:
  - `go test ./pkg/flow/node/... -v -run Framework`
- Repo tasks:
  - `task test` or `pnpm nx run server:test`
