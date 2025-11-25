# Gemini Context & Instructions

## Project Overview
DevTools is a local-first, open-source API testing platform (Postman alternative) that runs as a desktop app, CLI, and Chrome extension. It features request recording, visual flow building, and CI/CD integration.

## Repository Architecture
- **Monorepo Tooling:** Nx, pnpm, Nix (for reproducible environments).
- **Key Directories:**
  - `apps/desktop`: Electron desktop application (TypeScript/React).
  - `apps/cli`: Go CLI.
  - `apps/api-recorder-extension`: Chrome extension.
  - `packages/server`: Go backend (Connect RPC, SQLite/LibSQL).
  - `packages/client`: React frontend services/hooks.
  - `packages/ui`: Shared React component library (Storybook).
  - `packages/db`: Shared SQL drivers and `sqlc` generated code.
  - `packages/spec`: TypeSpec definitions (single source of truth for API/Protobuf).

## Core Operational Mandates
1.  **Environment:** Always assume execution within a `nix develop` environment. Use `pnpm nx` for project tasks and `task` (Taskfile) for orchestrated workflows.
2.  **Context Awareness:** Read `CLAUDE.md` and `README.md` for domain specific vocabulary (flow nodes, delta system) before starting complex tasks.
3.  **File Editing:**
    - Verify files exist before editing.
    - Use `git status` and `git diff` to verify changes.
    - **Never** revert changes you didn't author unless instructed.
    - **Never** commit changes unless explicitly asked.
4.  **Verification:** Always test and lint and try to compile the project after change to be sure.

## Development Workflows

### Build & Run
- **Desktop App:** `task dev:desktop` (starts Electron + React + Go Server).
- **Server (Go):** `pnpm nx run server:dev` (hot reload).
- **UI (Web):** `pnpm nx run client:dev`.
- **Spec Generation:** `pnpm nx run spec:build` (run this after editing `.tsp` files).
- **Database:** `pnpm nx run db:generate` (run after editing `sqlc.yaml` or `.sql` files).

### Testing & Quality
- **Lint:** `task lint` (runs ESLint, formatting checks).
- **Test (Quick):** `task test` (runs unit tests).
- **Test (CI):** `task test:ci`.
- **Fix:** `task fix` (runs Prettier and Syncpack).
- **Go Benchmarks:** Use `go test -bench` as described in `CLAUDE.md` for performance-critical paths.

## Implementation Guidelines

### Go (Server)
- **Pattern:** Functional design, lean packages. Avoid complex OOP hierarchies.
- **RPC:** Defined in `packages/spec` (TypeSpec). Implemented in `packages/server/internal/api`.
- **Database:** Use `sqlc` generated structs in `packages/server/pkg/gen`.
- **Transactions:** Short-lived. Use `devtoolsdb.TxnRollback` in defer.
- **Concurrency:** Channel-based coordination or `sync.WaitGroup`.

### TypeScript/React (Client/Desktop)
- **State/Effects:** Effect-TS and TanStack Query.
- **Styling:** Tailwind CSS v4. Co-located with components.
- **Components:** PascalCase. Storybook located in `packages/ui`.
- **Strictness:** No `any`. Strict compiler settings.

## Backend Architecture & Patterns

### Architectural Layers
1.  **RPC Layer (`packages/server/internal/api`)**
    *   **Role:** Entry point for Connect RPC requests. Handles authentication, request validation, permission checks, and response formatting.
    *   **Transaction Management:** Explicitly manages database transactions (`BeginTx`, `Commit`, `Rollback`). Passes transactional context down to services.
    *   **Orchestration:** Coordinates multiple services (e.g., HTTP, User, Workspace) to fulfill a request.
    *   **Real-time Sync:** Publishes events to `eventstream` after successful transactions for client synchronization.

2.  **Service Layer (`packages/server/pkg/service`)**
    *   **Role:** Encapsulates domain logic and database interactions.
    *   **Structure:** Organized by domain entity (e.g., `shttp`, `suser`).
    *   **Transaction Support:** Services implement a `TX(tx *sql.Tx)` method to return a generic-bound instance, allowing the RPC layer to compose atomic operations across multiple services.
    *   **Abstraction:** Operates on **Internal Models**, decoupling the RPC layer from DB implementation details.

3.  **Model Layer (`packages/server/pkg/model`)**
    *   **Role:** Defines the core domain entities used throughout the application (e.g., `mhttp`, `muser`).
    *   **Decoupling:** Acts as the bridge and buffer between the **API Layer** (Proto generated types) and the **Data Layer** (`sqlc` generated types).
    *   **Purity:** purely Go structs, often with helper methods for domain logic, independent of database tags or JSON serialization concerns (where possible).

4.  **Data Access Layer (`packages/db/pkg/sqlc`)**
    *   **Tooling:** `sqlc` generates type-safe Go code from SQL queries.
    *   **Location:** Generated code resides in `packages/db/pkg/sqlc/gen`.
    *   **Pattern:** Services wrap these generated queries, handling conversion between DB models (`gen.Http`) and Internal Models (`mhttp.HTTP`).

5.  **Translation Layer**
    *   **Internal:** Explicit conversion functions (often in `converter` or inline) map between Proto messages (API), Internal Models (Service), and DB Models (Storage).
    *   **External:** `packages/server/pkg/translate` handles import/export for formats like HAR, Curl, and Postman.

### Real-time Collection System (TanStack DB Support)
The backend implements a specific pattern to support **TanStack DB** (and similar client-side replication strategies) on the frontend.

1.  **Standard RPC Pattern:**
    *   **`*Collection` (e.g., `HttpCollection`):** Returns the full initial state of a resource list.
    *   **`*Sync` (e.g., `HttpSync`):** A server-streaming RPC that provides real-time delta updates.
        *   **Snapshot:** Can optionally provide an initial snapshot (state-of-the-world) to ensure consistency.
        *   **Stream:** Pushes `Insert`, `Update`, and `Delete` events as they happen.
    *   **Mutations (`*Insert`, `*Update`, `*Delete`):**
        *   Perform the DB operation.
        *   **Crucially:** Publish an event to the `eventstream` system upon success.

2.  **Event Streaming (`packages/server/pkg/eventstream`)**
    *   **Generic Streamer:** `SyncStreamer[Topic, Event]` manages subscriptions and broadcasting.
    *   **Topics:** Events are scoped (e.g., by `WorkspaceID`) to ensure users only receive relevant updates.
    *   **Access Control:** The `Sync` RPC applies filters (e.g., checking workspace membership) before sending events to a connected client.

3.  **Frontend Integration:**
    *   The client (using `@tanstack/react-db`) subscribes to these `*Sync` endpoints.
    *   Incoming `Insert`/`Update`/`Delete` messages are applied to the client-side in-memory database, keeping the UI reactive without manual refetching.

### Go Best Practices & Philosophy
-   **SQLite Transaction Best Practice:** Due to SQLite's locking mechanisms, reads should generally be performed *before* initiating a transaction. Transactions should be kept as short-lived as possible and contain only necessary writes to minimize contention and potential deadlocks. If reads are required, they must happen outside the transaction or explicitly as part of it, understanding the implications for concurrency.
-   **Functional/Procedural:** Preference for simple struct-based services and pure functions over complex OOP hierarchies.
-   **Rich Error Handling:** Map internal errors to specific Connect RPC error codes (e.g., `connect.CodeNotFound`, `connect.CodePermissionDenied`).
-   **Model Separation:** Strict separation between API types (Proto), Domain types (Internal), and Storage types (DB) prevents tight coupling and leakage of implementation details.
-   **Generated Code:** Leverage `sqlc` and `buf` to reduce boilerplate and ensure type safety, but keep generated code isolated.
-   **Code Location:**
    -   `internal`: Code private to the server package (RPC handlers, specific logic).
    -   `pkg`: reusable libraries and domain logic that *could* be imported by other Go modules (though currently mostly used by server).

## Pre-Response Checklist
1.  **Clean Workspace:** Run `git status -sb` to ensure no unexpected files are modified.
2.  **Validation:** Run relevant tests (`task test`) or explain why skipped.
3.  **Formatting:** Ensure code meets project standards (`task lint`).
4.  **Documentation:** Update relevant docs if behavior changed.
5.  **Summary:** Provide a concise summary of changes and verification steps.
