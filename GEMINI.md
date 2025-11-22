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

## Pre-Response Checklist
1.  **Clean Workspace:** Run `git status -sb` to ensure no unexpected files are modified.
2.  **Validation:** Run relevant tests (`task test`) or explain why skipped.
3.  **Formatting:** Ensure code meets project standards (`task lint`).
4.  **Documentation:** Update relevant docs if behavior changed.
5.  **Summary:** Provide a concise summary of changes and verification steps.
