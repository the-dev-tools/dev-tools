# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Core Mandates
1. **Environment:** This project uses a Nix flake environment with `direnv`. Use `pnpm nx` for project tasks and `task` (Taskfile) for orchestrated workflows.
2. **Context Awareness:** Read `README.md` for domain-specific vocabulary (flow nodes, delta system) before starting complex tasks.
3. **File Editing:** Verify files exist before editing. Use `git status` and `git diff` to verify changes. **Never** revert changes you didn't author unless instructed. **Never** commit changes unless explicitly asked.
4. **Verification:** Always test, lint, and compile after making changes.

## Common Commands

### Build & Run
```bash
task dev:desktop                    # Full desktop app (Electron + React + Go Server)
pnpm nx run server:dev              # Go server with hot reload
pnpm nx run client:dev              # React frontend
pnpm nx run spec:build              # Regenerate from TypeSpec (after editing .tsp files)
pnpm nx run db:generate             # Regenerate sqlc (after editing .sql files)
cd apps/cli && task build:release   # Build CLI binary
```

### Testing
```bash
task test                           # All unit tests
pnpm nx run server:test             # Server tests only
pnpm nx run db:test                 # DB tests only
pnpm nx run cli:test                # CLI tests only

# Single Go test (use -run for specific functions)
cd packages/server && go test -run TestFunctionName ./path/to/package/ -v -timeout 30s
cd packages/db && go test -run TestFunctionName ./path/to/package/ -v -timeout 10s
cd apps/cli && go test -run TestFunctionName ./path/to/package/ -v -timeout 30s
```

### Lint & Fix
```bash
task lint                           # ESLint + format checks + golangci-lint
task fix                            # Prettier + Syncpack auto-fix
pnpm nx run server:lint             # Go linters (golangci-lint + norawsql + notxread)
```

### Benchmarks
```bash
task benchmark:run                  # Run benchmarks
task benchmark:baseline             # Save baseline
task benchmark:compare              # Compare against baseline
```

### Version Plans
```bash
task version-plan project=desktop   # Create version plan (projects: desktop, cli, api-recorder-extension)
```

## Project Overview

DevTools is a local-first, open-source API testing platform (Postman alternative) — desktop app, CLI, and Chrome extension. Features request recording, visual flow building, and CI/CD integration.

## Architecture

### Monorepo Structure
- **`apps/desktop`** — Electron app (TypeScript/React, electron-vite)
- **`apps/cli`** — Go CLI (cobra). Embeds `packages/worker-js` (TypeScript worker bundled via tsup)
- **`packages/server`** — Go backend (Connect RPC, SQLite/LibSQL)
- **`packages/client`** — React frontend (TanStack Router/Query, Effect-TS, React Flow)
- **`packages/ui`** — Shared React component library (React Aria, Tailwind Variants, Storybook)
- **`packages/db`** — Go database package (`devtoolsdb`), sqlc generated code, SQLite drivers
- **`packages/spec`** — TypeSpec definitions → Protobuf → Go/TypeScript codegen (single source of truth)
- **`packages/worker-js`** — TypeScript worker bundled into CLI binary
- **`tools/`** — Custom Go linters (`norawsql`, `notxread`), benchmarking, spec emitter, ESLint config

### Go Workspace
`go.work` with Go 1.25. Modules: `apps/cli`, `packages/db`, `packages/server`, `packages/spec`, and tools.

### Naming Conventions
- **Services:** `s` prefix (`shttp`, `senv`, `sflow`, `suser`, `sworkspace`)
- **Models:** `m` prefix (`mhttp`, `mflow`, `menv`, `muser`)
- **RPC handlers:** `r` prefix (`rhttp`, `rflowv2`, `renv`)
- **IDs:** All use `idwrap.IDWrap` (ULID-based) from `packages/server/pkg/idwrap`

### Backend Layers (Server)
1. **RPC Layer** (`packages/server/internal/api`) — Connect RPC handlers. All follow **Fetch-Check-Act**:
   - **FETCH**: Read data via Reader services (non-blocking, parallel)
   - **CHECK**: Validate permissions/rules (pure Go, in memory)
   - **ACT**: Write via Writer services inside a short transaction
   - Publishes events to `eventstream` after successful transactions
2. **Service Layer** (`packages/server/pkg/service`) — Domain logic, split into Reader (read-only, `*sql.DB` pool) and Writer (write-only, `*sql.Tx`, per-transaction)
3. **Model Layer** (`packages/server/pkg/model`) — Pure Go domain structs bridging API (Proto) and DB (sqlc) types
4. **Data Access** (`packages/db/pkg/sqlc`) — sqlc-generated code. Schema in `schema/`, queries in `queries/`, output in `gen/`

Large RPC handlers are split by concern: `rhttp_crud.go`, `rhttp_exec.go`, `rhttp_delta_*.go`, etc.

### Codegen Pipeline
`pnpm nx run spec:build` runs: TypeSpec compile → buf generate → post-process. Output in `packages/spec/dist/`:
- `buf/go/` — Go protobuf + Connect RPC
- `buf/typescript/` — TypeScript protobuf types
- `tanstack-db/typescript/` — TanStack DB types

### TypeScript/React
- **Strictness:** `@tsconfig/strictest`, no `any`
- **Styling:** Tailwind CSS v4
- **State:** Effect-TS + TanStack Query
- **Formatting:** Prettier (single quotes, JSX single quotes). ESLint with perfectionist import sorting
- **Dependencies:** Pnpm catalog mode (strict) — all versions centralized in `pnpm-workspace.yaml`
- **No TS unit tests** — quality enforced via ESLint + strict TypeScript

## Go Patterns

### Testing
- **Isolated service tests:** `sqlitemem.NewSQLiteMem(ctx)` — single-connection in-memory SQLite
- **RPC/integration tests:** `testutil.CreateBaseDB` / `dbtest.GetTestDB(ctx)` — shared-cache in-memory SQLite
- **One DB per test.** Never share DB instances across tests
- **Seeding:** `BaseTestServices.CreateTempCollection` for workspace/user/collection state
- **`t.Parallel()`** only if each subtest creates its own independent DB
- **Transactions:** Keep short. Use `devtoolsdb.TxnRollback` in defer. Commit before reading from a different connection
- **Server tests run with `-p 8`** (8 parallel test packages)

### Integration Tests
For tests requiring external services (APIs, cloud):
- File naming: `integration_*.go`
- Build tags: `//go:build ai_integration`
- Env var guard: `if os.Getenv("RUN_XX_INTEGRATION") != "true" { t.Skip() }`

### Linting
Server lint (`pnpm nx run server:lint`) includes:
- `golangci-lint` with extensive rules (govet, gosec, errorlint, revive, exhaustive, etc.)
- `go tool norawsql` — enforces sqlc usage, no raw SQL strings
- `go tool notxread` — prevents reads inside transactions (SQLite deadlock prevention)

### Best Practices
- Functional design, lean packages. No complex OOP hierarchies
- SQLite reads **before** transactions. Transactions as short as possible
- Map errors to Connect RPC codes (`connect.CodeNotFound`, not `CodePermissionDenied` for missing resources — prevents ID enumeration)
- Strict model separation: API types (Proto) ↔ Domain types (model) ↔ Storage types (sqlc gen)

## Domain Documentation
- **Flow Engine & Nodes:** `packages/server/docs/specs/FLOW.md`
- **HTTP & Proxy:** `packages/server/docs/specs/HTTP.md`
- **Real-time Sync:** `packages/server/docs/specs/SYNC.md`
- **Mutation System:** `packages/server/docs/specs/MUTATION.md`
- **Service Architecture:** `packages/server/docs/specs/BACKEND_ARCHITECTURE_V2.md`
- **Bulk Operations:** `packages/server/docs/specs/BULK_SYNC_TRANSACTION_WRAPPERS.md`
