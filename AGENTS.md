# Repository Guidelines

## Project Structure & Module Organization
- apps: user-facing apps (`apps/cli` Go CLI, `apps/desktop` Electron, `apps/api-recorder-extension`).
- packages: core libraries and services (`packages/server` Go API, `packages/client` React/TS, `packages/ui` components, `packages/spec` API definitions, `packages/db`, `packages/worker-js`).
- tooling: Nx + pnpm monorepo (`nx.json`, `pnpm-workspace.yaml`), Nix/direnv for dev shells, shared configs in `configs/`, `tools/`, and `docs/`.

## Build, Test, and Development Commands
- Enter dev shell: `nix develop` (recommended) then run commands below.
- Run desktop in dev: `task dev:desktop` (wraps `nx run desktop:dev`).
- Lint all: `task lint` (ESLint, format check, Go linters where defined).
- Tests (local): `task test` (delegates to Nx targets).
- Tests (CI JSON): `task test:ci` (writes test output to `dist/*`).
- Go server: `pnpm nx run server:dev | build | test | lint`.
- Spec build: `pnpm nx run spec:build` (TypeSpec → protobuf/codegen).
- Storybook: `pnpm nx run ui:storybook` or `pnpm nx run client:storybook`.

## Coding Style & Naming Conventions
- TypeScript: strict config (`tsconfig.base.json`), 2‑space indent, single quotes; format with Prettier (`prettier.config.mjs`).
- Linting: ESLint via `@the-dev-tools/eslint-config`; run `task lint` and fix with `task fix`.
- Go: target Go 1.24; formatting via `gofmt`; lint with `golangci-lint` (`pnpm nx run server:lint`).
- Paths/names: use kebab-case for folders, camelCase for TS identifiers, PascalCase for React components, `*_test.go` for Go tests.

## Testing Guidelines
- Primary tests live in `packages/server` (Go). Run `pnpm nx run server:test` or `task test`.
- Prefer table‑driven tests for Go; place integration fixtures under `packages/server/test`.
- For UI or TS modules, add tests under the package and wire a `test` Nx target before merging.

## Commit & Pull Request Guidelines
- Commits: concise, present‑tense, scope in prefix when helpful (e.g., `server: fix reorder logic`).
- PRs: include summary, rationale, linked issues, and screenshots for UI changes.
- Versioning: include an Nx Version Plan (`nx release plan`) for user‑facing changes.
- Gates: CI must pass; run `task lint` and `task test` locally first.

## Security & Configuration Tips
- Do not commit secrets. Use `.env.development` locally; `.envrc`/direnv manages env loading.
- Keep generated artifacts out of VCS; write to `dist/`.

## Agent‑Specific Notes
- Prefer Nx/Task targets over raw commands; keep changes minimal and scoped.
- Update docs when altering commands or project layout. See `CLAUDE.md` for deeper architecture and workflows.
