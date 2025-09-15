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

## Command Cheatsheet

- Run all server tests with race:
  - `cd packages/server && go test ./... -race -count=1`
- Focus a package:
  - `go test ./pkg/flow/runner/flowlocalrunner -race -count=1`
- Repo tasks:
  - `task test` or `pnpm nx run server:test`
