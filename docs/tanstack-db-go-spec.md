# TanStack DB Go Handler Generation

This document captures the current shape of the server-side code generation that
bridges the TypeSpec TanStack collections to the Go RPC layer. It acts as the
checkpoint for the work completed on branch `tanstack-db-go-handlers`.

## Goals

- Reuse the existing TypeSpec pipeline (Alloy emitters) to emit Go code that
  handles Connect RPC boilerplate for TanStack collections.
- Keep project-specific business logic (permissions, service calls) in
  hand-written Go while moving request decoding, principal extraction, and
  response wiring into generated code.
- Provide a seam that other RPC services can adopt incrementally without large
  rewrites.

## Architecture

- `tools/spec-lib/src/tanstack-db/go-emitter.ts` is the new Alloy emitter. It
  reads metadata captured by the `@TanStackDB.collection` decorator and writes a
  `*_gen.go` companion beside each RPC service. Generated files include:
  - `EnvironmentHandler` struct implementing Connect handler methods.
  - `EnvironmentHooks` interface that the handwritten service implements.
  - `EnvironmentPrincipal` extraction and guard rails around missing hooks.
  - Strongly typed decode helpers that convert protobuf request items into Go
    structs with `idwrap.IDWrap` conversions.
- `packages/server/internal/api/renv/renv.go` now:
  - Owns a generated `EnvironmentHandler` instance.
  - Implements `EnvironmentHooks` to keep business logic (permission checks,
    service calls) local.
  - Exposes `environmentPrincipal` so the generated code can extract user state.
- `packages/server/internal/api/renv/renv_gen.go` is generated and should not be
  edited by hand. It only depends on the TypeSpec output and project-level
  helper imports.

## Developer Workflow

1. Run `nix develop -c pnpm nx run spec:build` to regenerate the Go handlers
   whenever the TypeSpec changes.
2. Implement the `*Hooks` interface inside the handwritten RPC file; here we use
   the existing services (`senv.EnvService`, `suser.UserService`, etc.).
3. Connect RPC methods delegate to the generated handler so Connect glue stays
   consistent.
4. Tests continue to run under `nix develop -c go test ./packages/server/internal/api/...`.

## Next Steps

- Expand the emitter coverage to other TanStack collections once their Go RPCs
  have matching hook implementations.
- Consider sharing common helper functions (e.g., permission adapters) across
  generated files if patterns emerge.

This spec doubles as a reminder that the generated Go files are the source of
truth for handler scaffolding and should be committed alongside TypeSpec
changes.***
