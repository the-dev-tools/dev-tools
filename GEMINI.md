# Gemini Context & Instructions

## Core Operational Mandates
1.  **Efficiency:** Always run commands like `task test`, `pnpm nx`, or `npm install` in the background (using `&`) and check their logs periodically to avoid getting stuck or timing out.
2.  **Environment:** Always assume execution within a `nix develop` environment. Use `pnpm nx` for project tasks and `task` (Taskfile) for orchestrated workflows. **CRITICAL:** Always run commands using `direnv exec . <command>` to ensure the correct environment (e.g., `NX_TUI`, `TASK_OUTPUT`) is loaded.
3.  **Context Awareness:** Read `README.md` for domain specific vocabulary (flow nodes, delta system) before starting complex tasks.
4.  **File Editing:**
    - Verify files exist before editing.
    - Use `git status` and `git diff` to verify changes.
    - **Never** revert changes you didn't author unless instructed.
    - **Never** commit changes unless explicitly asked.
4.  **Verification:** Always test and lint and try to compile the project after change to be sure.

## Project Overview
DevTools is a local-first, open-source API testing platform (Postman alternative) that runs as a desktop app, CLI, and Chrome extension. It features request recording, visual flow building, and CI/CD integration.

## Repository Architecture
- **Monorepo Tooling:** Nx, pnpm, Nix (for reproducible environments).
- **Key Directories:**
  - `apps/desktop`: Electron desktop application (TypeScript/React).
  - `apps/cli`: Go CLI. Key internals: `internal/runner` (execution engine), `internal/importer`.
  - `apps/api-recorder-extension`: Chrome extension (currently disabled).
  - `packages/server`: Go backend (Connect RPC, SQLite/LibSQL).
  - `packages/client`: React frontend services/hooks.
  - `packages/ui`: Shared React component library (Storybook).
  - `packages/db`: Shared SQL drivers and `sqlc` generated code.
  - `packages/spec`: TypeSpec definitions (single source of truth for API/Protobuf).
  - `packages/worker-js`: TypeScript worker bundled into CLI binary.
- **Tools Directory (`tools/`):**
  - `benchmark/`: Go benchmarking tool for performance testing.
  - `norawsql/`: Custom Go linter to detect raw SQL (enforces sqlc usage).
  - `notxread/`: Custom Go linter to detect non-TX-bound reads in transactions (prevents SQLite deadlocks).
  - `modmigrate/`: Tool to migrate Go module paths to idiomatic GitHub paths.
  - `spec-lib/`: TypeSpec emitter library for custom code generation.
  - `eslint/`: Shared ESLint configuration package.
  - `storybook/`: Storybook configuration and composition.
  - `gha-scripts/`: GitHub Actions helper scripts.

## Development Workflows

### Build & Run
- **Desktop App:** `task dev:desktop` (starts Electron + React + Go Server).
- **Server (Go):** `pnpm nx run server:dev` (hot reload).
- **UI (Web):** `pnpm nx run client:dev`.
- **Storybook:** `task storybook` (component library dev).
- **Spec Generation:** `pnpm nx run spec:build` (run after editing `.tsp` files; outputs to `packages/spec/dist`).
- **Database:** `pnpm nx run db:generate` (run after editing `sqlc.yaml` or `.sql` files).
- **Version Plan:** `task version-plan` (create a version plan for release management).
- **CLI Release:** `cd apps/cli && task build:release` (builds local binary).

### Testing & Quality
- **Lint:** `task lint` (runs ESLint, formatting checks).
- **Test (Quick):** `task test` (runs unit tests).
- **Test (CI):** `task test:ci`.
- **Fix:** `task fix` (runs Prettier and Syncpack).
- **Go Benchmarks:** Use `task benchmark:run` to run, `task benchmark:baseline` to save baseline, and `task benchmark:compare` to compare.

## Implementation Guidelines

### CLI (Go)
- **Entrypoint:** `apps/cli/main.go`.
- **Build System:** `task` (Taskfile) handles cross-compilation and embedding.
- **Worker Embedding:** `packages/worker-js` is a TypeScript worker bundled via `tsup` and embedded into the CLI binary. If you change `packages/worker-js`, you must rebuild the CLI.
- **Architecture:** Uses `cobra` for commands. Core logic in `apps/cli/internal/runner` (headless execution).

### Go (Server)
- **Pattern:** Functional design, lean packages. Avoid complex OOP hierarchies.
- **RPC:** Defined in `packages/spec` (TypeSpec). Implemented in `packages/server/internal/api`.
- **Database:** Use `sqlc` generated structs in `packages/db/pkg/sqlc/gen`.
- **Transactions:** Short-lived. Use `devtoolsdb.TxnRollback` in defer.
- **Concurrency:** Channel-based coordination or `sync.WaitGroup`.

### Testing Patterns (Server)
- **Service/Repository Tests:** Use `sqlitemem.NewSQLiteMem` for fastest, single-connection, in-memory SQLite. Best for testing isolated services.
- **RPC/Integration Tests:** Use `testutil.CreateBaseDB` when multiple components (RPC + Services) need to share a DB.
- **Isolation:** One DB per test. Never share DB instances across tests.
- **Transactions:** Keep TXs short. Commit before reading in a different connection (e.g., RPC calls).
- **Seeding:** Use `BaseTestServices.CreateTempCollection` to quickly seed workspace/user/collection state.
- **Parallelism:** Safe to use `t.Parallel()` *only* if each subtest creates its own independent DB.

### TypeScript/React (Client/Desktop)
- **State/Effects:** Effect-TS and TanStack Query.
- **Styling:** Tailwind CSS v4. Co-located with components.
- **Components:** PascalCase. Storybook located in `packages/ui`.
- **Strictness:** No `any`. Strict compiler settings.

## Backend Architecture & Patterns

### Architectural Layers
1.  **RPC Layer (`packages/server/internal/api`)**
    *   **Role:** Entry point for Connect RPC requests. Handles authentication, request validation, permission checks, and response formatting.
    *   **Fetch-Check-Act Pattern:** All RPC handlers follow this 3-phase flow to maximize SQLite concurrency:
        1.  **FETCH**: Gather data using Reader services (non-blocking, parallel).
        2.  **CHECK**: Validate permissions and business rules (pure Go memory).
        3.  **ACT**: Persist changes using Writer services inside a transaction (serialized, kept short).
    *   **Real-time Sync:** Publishes events to `eventstream` after successful transactions for client synchronization.

2.  **Service Layer (`packages/server/pkg/service`)**
    *   **Role:** Encapsulates domain logic and database interactions.
    *   **Structure:** Organized by domain entity (e.g., `shttp`, `suser`).
    *   **Reader/Writer Pattern:** Services are split into two types:
        -   `*Reader`: Read-only, uses `*sql.DB` pool, non-blocking, long-lived singleton.
        -   `*Writer`: Write-only, uses `*sql.Tx`, serialized, transient (created per-transaction).
    *   **Abstraction:** Operates on **Internal Models**, decoupling the RPC layer from DB implementation details.

3.  **Model Layer (`packages/server/pkg/model`)**
    *   **Role:** Defines the core domain entities used throughout the application (e.g., `mhttp`, `muser`).
    *   **Decoupling:** Acts as the bridge and buffer between the **API Layer** (Proto generated types) and the **Data Layer** (`sqlc` generated types).
    *   **Purity:** purely Go structs, often with helper methods for domain logic, independent of database tags or JSON serialization concerns (where possible).

4.  **Data Access Layer (`packages/db/pkg/sqlc`)**
    *   **Tooling:** `sqlc` generates type-safe Go code from SQL queries.
    *   **Location:** Generated code resides in `packages/db/pkg/sqlc/gen`. Source SQL is split into `packages/db/pkg/sqlc/schema/` (DDL) and `packages/db/pkg/sqlc/queries/` (DML).
    *   **Pattern:** Services wrap these generated queries, handling conversion between DB models (`gen.Http`) and Internal Models (`mhttp.HTTP`).

### Translation Layer
    *   **Internal:** Explicit conversion functions (often in `converter` or inline) map between Proto messages (API), Internal Models (Service), and DB Models (Storage).
    *   **External:** `packages/server/pkg/translate` handles import/export for formats like HAR, Curl, and Postman.

## Domain Documentation
- **Flow Engine & Nodes:** Read `packages/server/docs/specs/FLOW.md` for details on the execution engine, node types, and variable system.
- **HTTP & Proxy:** Read `packages/server/docs/specs/HTTP.md` for request recording, execution, and import/export logic.
- **Real-time Sync:** Read `packages/server/docs/specs/SYNC.md` for the Deep dive into the Real-time Sync / TanStack DB pattern and Event Streaming.
- **Mutation System:** Read `packages/server/docs/specs/MUTATION.md` for details on automatic cascade event collection and transaction management.
- **Service Architecture:** Read `packages/server/docs/specs/BACKEND_ARCHITECTURE_V2.md` for Reader/Writer service pattern and Fetch-Check-Act concurrency pattern.
- **Bulk Operations:** Read `packages/server/docs/specs/BULK_SYNC_TRANSACTION_WRAPPERS.md` for bulk sync transaction patterns.

### Security Best Practices
-   **ID Enumeration Prevention:** When a user requests a resource (Workspace, Flow, etc.) they do not have access to, the server must return `CodeNotFound` (or `ErrWorkspaceNotFound`), **NOT** `CodePermissionDenied`. This prevents attackers from probing the existence of private resources by distinguishing between "does not exist" and "access denied" responses.
    -   *Implementation:* In RPC handlers or Validators, after checking ownership/permission, if the check fails, return the specific `NotFound` error for that entity type.

### Go Best Practices & Philosophy
-   **SQLite Transaction Best Practice:** Due to SQLite's locking mechanisms, reads should generally be performed *before* initiating a transaction. Transactions should be kept as short-lived as possible and contain only necessary writes to minimize contention and potential deadlocks. If reads are required, they must happen outside the transaction or explicitly as part of it, understanding the implications for concurrency.
-   **Functional/Procedural:** Preference for simple struct-based services and pure functions over complex OOP hierarchies.
-   **Rich Error Handling:** Map internal errors to specific Connect RPC error codes (e.g., `connect.CodeNotFound`, `connect.CodePermissionDenied`).
-   **Model Separation:** Strict separation between API types (Proto), Domain types (Internal), and Storage types (DB) prevents tight coupling and leakage of implementation details.
-   **Service Structure:** For large services (e.g., `rhttp`, `rflowv2`), split files by domain/functionality (e.g., `rhttp_exec.go`, `rhttp_sync.go`) rather than keeping everything in one huge file.
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