# Backend Architecture V2: Service Layer & Concurrency

This document outlines the architectural standards for the backend service layer, specifically designed to maximize SQLite concurrency performance and enforce type-safe transaction management.

## 1. Core Philosophy: Segregation of Responsibilities

To prevent database locks and ensure clean code architecture, we strictly separate **Reading** (Data Retrieval) from **Writing** (Data Modification).

### The Problem with Monolithic Services

In the legacy pattern (`HTTPService`), a single struct held both `Get()` and `Create()` methods. This allowed:

- Accidental Writes without a Transaction (using the auto-commit DB pool).
- Accidental Heavy Reads inside a Transaction (blocking other writers).

### The Solution: Split Services

We split every domain service into two distinct types:

1.  **The Reader (`*Reader`)**:
    - **Role**: Read-Only access to the database.
    - **Connection**: Uses the `*sql.DB` connection pool.
    - **Concurrency**: Non-blocking. Multiple readers can run in parallel with one writer.
    - **Lifecycle**: Long-lived singleton, injected into RPC handlers.

2.  **The Writer (`*Writer`)**:
    - **Role**: Write-only access (Create, Update, Delete).
    - **Connection**: Uses a `*sql.Tx` (Transaction).
    - **Concurrency**: Serialized. Only one writer active at a time.
    - **Lifecycle**: Transient. Created _only_ inside a transaction block and discarded immediately after Commit/Rollback.

## 2. Implementation Pattern

### 2.1. The Reader

Defined in `pkg/service/{domain}/reader.go`.

```go
package shttp

import (
    "context"
    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/server/pkg/db"
)

type Reader struct {
    q *gen.Queries
}

// NewReader accepts the generic Queryable interface (usually *sql.DB)
func NewReader(db db.Queryable) *Reader {
    return &Reader{
        q: gen.New(db),
    }
}

func (r *Reader) Get(ctx context.Context, id string) (*mhttp.HTTP, error) {
    // Pure Read - Uses DB Pool
    return r.q.GetHTTP(ctx, id)
}
```

### 2.2. The Writer

Defined in `pkg/service/{domain}/writer.go`.

```go
package shttp

import (
    "context"
    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/server/pkg/db"
)

type Writer struct {
    q *gen.Queries
}

// NewWriter REQUIRES a DBTX interface (satisfied by *sql.Tx)
// It is impossible to create this with a raw *sql.DB if we enforce stricter types,
// but usually we rely on convention or a specific Tx interface.
func NewWriter(tx db.DBTX) *Writer {
    return &Writer{
        q: gen.New(tx),
    }
}

func (w *Writer) Create(ctx context.Context, item *mhttp.HTTP) error {
    // Pure Write - Uses Exclusive Transaction Lock
    return w.q.CreateHTTP(ctx, item)
}
```

## 3. The "Fetch-Check-Act" Concurrency Pattern

To maximize throughput with SQLite's WAL mode, all RPC handlers **MUST** follow this 3-phase execution flow.

### Phase 1: FETCH (Read-Only)

- **Goal**: Gather all data needed for the operation.
- **Context**: No Transaction. Uses `Reader` services.
- **Performance**: Fast, parallel, non-blocking.
- **Example**: fetching User, Workspace, Existing Entity.

### Phase 2: CHECK (Logic)

- **Goal**: Validate permissions, business rules, and inputs.
- **Context**: Pure Go memory.
- **Performance**: Instant.
- **Example**: `if user.Role != Admin { return Error }`

### Phase 3: ACT (Write-Only)

- **Goal**: Persist changes.
- **Context**: `BeginTx(Immediate)`. Uses `Writer` services.
- **Performance**: Blocking (Serialized). Must be kept **extremely short**.
- **Rules**:
  - No HTTP requests.
  - No complex loops or heavy calculations.
  - No "Reads" unless absolutely necessary for data integrity (e.g., "Read-Modify-Write" where the read must be locked).

## 4. Example: Refactored RPC Handler

```go
func (h *HttpServiceRPC) Insert(ctx context.Context, req *Request) (*Response, error) {
    // =================================================================
    // 1. FETCH (Parallel, Non-Blocking)
    // =================================================================
    // We use the Reader to get data from the pool.
    existing, err := h.httpReader.Get(ctx, req.Id)
    if err == nil {
        return nil, ErrAlreadyExists
    }

    // =================================================================
    // 2. CHECK (In-Memory)
    // =================================================================
    if err := h.validator.Validate(req); err != nil {
        return nil, err
    }

    // =================================================================
    // 3. ACT (Serialized, Blocking)
    // =================================================================
    // Start the transaction strictly for the Write phase.
    tx, err := h.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer devtoolsdb.TxnRollback(tx)

    // Instantiate the Writer using the Transaction
    writer := shttp.NewWriter(tx)

    // Perform the write
    if err := writer.Create(ctx, converter.ToModel(req)); err != nil {
        return nil, err
    }

    if err := tx.Commit(); err != nil {
        return nil, err
    }

    return &Response{}, nil
}
```

## 5. Migration Strategy (Strangler Fig)

To migrate existing services without breaking the build:

1.  **Extract**: Create `reader.go` and `writer.go` for a service (e.g., `shttp`).
2.  **Bridge**: Update the existing `HTTPService` struct to embed `*Reader` and forward calls.
    ```go
    type HTTPService struct {
        reader *Reader
        // ...
    }
    func (s *HTTPService) Get(ctx, id) { return s.reader.Get(ctx, id) }
    ```
3.  **Adopt**: In RPC handlers, start injecting `*Reader` directly instead of `HTTPService`.
4.  **Cleanup**: Once `HTTPService` is unused, remove it.

## 6. SQLite Configuration Context

This architecture relies on specific SQLite settings (already configured in `pkg/sqlitelocal`):

- `_journal_mode=WAL`: Enables Readers to run while a Writer is active.
- `_txlock=immediate`: `BeginTx` acquires the Write lock immediately, preventing deadlock.
- `_busy_timeout=5000`: Writers wait in a queue rather than failing immediately if the DB is busy.
