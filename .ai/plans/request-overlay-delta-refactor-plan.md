# Request Overlay Delta Refactor Plan (No Bridge, No Migration)

This plan delivers a clean, overlay‑only delta system for Request APIs (queries, headers, asserts) without any read‑time “bridge” logic and without migrating existing data. It makes overlay the single source of truth for delta examples, while origin examples continue to use established services.

## Goals and Non‑Goals

- Goals
  - Make overlay (order/state/delta) the single write/read model for delta examples.
  - Keep origin flows unchanged (continue using existing origin services).
  - Update Request RPCs to write to overlay directly; lists read overlay+origin only.
  - Update tests to use Request RPCs for delta flows (no direct DB delta inserts), ensuring correct SourceKind and ordering.
  - Stabilize test fixtures where example linked‑list CAS conflicts arise.

- Non‑Goals
  - No read‑time projection (“bridge”) from legacy DB delta rows into overlay.
  - No data migration scripts or background conversion of existing DB records.

## Guiding Principles

- Overlay order (ranked) drives list and move within delta examples.
- Overlay state defines MIXED by storing only fields that differ from origin; overlay core compares state to origin to compute SourceKind.
- Overlay delta‑only entries represent DELTA items.
- Origin examples continue to use existing services (no overlay writes/reads there).

## Current Architecture (concise)

- Origin tables hold canonical items (example_query, example_header, asserts, etc.).
- Overlay services (pkg/overlay/* + pkg/service/soverlay*) provide:
  - OrderStore: overlay order (ranks) per delta example (RefKindOrigin or RefKindDelta).
  - StateStore: overrides keyed by (deltaExampleID, originID) → MIXED.
  - DeltaStore: standalone delta-only rows keyed by (deltaExampleID, deltaID) → DELTA.
  - Core merge (pkg/overlay/core) produces Merged list (Values + Origin + SourceKind).

## High‑Level Changes by Family

Applies consistently to Query, Header, Assert families within `packages/server/internal/api/rrequest/rrequest.go`.

### 1) DeltaExampleCopy (seed overlay order only)

- Current issues: copy sometimes creates DB delta rows and/or tests insert delta rows directly; overlay List then lacks correct order/state (without bridging).
- Target behavior:
  - Do NOT write DB delta rows for the delta example.
  - Seed overlay OrderStore for the delta example by inserting RefKindOrigin entries for each origin item in the origin order.
  - Do NOT write StateStore overrides or DeltaStore rows during copy.
  - Result in List: items appear as ORIGIN in delta (no overrides yet).

### 2) DeltaCreate (overlay only)

- Create overlay DeltaStore row (empty/default values initially) and append to overlay OrderStore as RefKindDelta.
- No changes in origin tables for delta‑only items.

### 3) DeltaUpdate (overlay only)

- For delta‑only IDs (RefKindDelta): update DeltaStore (values + enabled + desc).
- For origin‑ref IDs (RefKindOrigin): Upsert StateStore overrides (only the provided fields), leaving unspecified fields nil (inherit from origin). SourceKind becomes MIXED when any override differs from origin.

### 4) DeltaReset (overlay only)

- Use overlay core.Reset:
  - For origin‑ref items: clear StateStore overrides → item becomes ORIGIN.
  - For delta‑only items: clear DeltaStore values (or reset to defaults) → still DELTA but empty values; UI may filter if desired.

### 5) DeltaDelete (overlay only)

- Use overlay core.Delete:
  - For delta‑only: delete DeltaStore row and remove Overlay order ref.
  - For origin‑ref: remove Overlay order ref and mark StateStore as suppressed (tombstone) so the origin item disappears from overlay list without touching origin tables.

### 6) Move / DeltaMove

- Non‑delta Move (QueryMove/HeaderMove/AssertMove) for origin examples:
  - Keep using existing origin services (e.g., `HeaderService.MoveHeader`).

- DeltaMove (QueryDeltaMove/HeaderDeltaMove/AssertDeltaMove) for delta examples:
  - Use overlay core.Move (rank‑based) to reorder the two overlay entries identified by IDs.

### 7) List / DeltaList

- Non‑delta List: keep origin list behavior.
- DeltaList: read from overlay OrderStore + StateStore + DeltaStore only; fetch origin values for merge via origin services; build RPC items with SourceKind from overlay core.
- No read‑time seeding or projection allowed.

## Detailed Implementation Tasks

All paths below are in `packages/server/internal/api/rrequest/rrequest.go` unless noted.

1) Remove any read‑time projection code (bridge)
   - QueryDeltaList: ensure there is no logic that mirrors DB delta rows into overlay at list time.
   - HeaderDeltaList / AssertDeltaList: confirm no projection logic exists.

2) Queries – overlay‑first
   - QueryDeltaExampleCopy(originEx, deltaEx)
     - Load ordered origin queries via `sexamplequery.GetExampleQueriesByExampleID(originEx)`.
     - For each origin ID, compute next rank (using OrderStore.LastRank/Upsert) and insert RefKindOrigin order row for `deltaEx`.
     - Do not write StateStore/DeltaStore at copy time.
   - QueryDeltaCreate
     - `overcore.CreateDelta` (DeltaStore.Insert + OrderStore.Upsert).
     - `overcore.Update` (optional initial values).
   - QueryDeltaUpdate
     - Resolve ID scope via `soverlayquery`: if delta‑only → DeltaStore.Update; else → StateStore.Upsert.
   - QueryDeltaReset
     - Resolve scope via `soverlayquery`, then `overcore.Reset`.
   - QueryDeltaDelete
     - Resolve scope via `soverlayquery`, then `overcore.Delete`.
   - QueryDeltaMove
     - Resolve scope via `soverlayquery`, then `overcore.Move`.
   - QueryMove (non‑delta)
     - Keep origin flow or no‑op if API is deprecated; ensure tests use DeltaMove for delta examples.
   - QueryDeltaList
     - Ensure input includes `ExampleId` (delta) and `OriginId` (origin); error if missing.
     - Use overlay `core.List` with:
       - Fetch overlay order/state/delta via `soverlayquery.Service` adapters (already present).
       - `originVals`: map originID → Values from `sexamplequery.GetExampleQueriesByExampleID(originID)`.
       - `build`: construct `requestv1.QueryDeltaListItem` with SourceKind from `core.Merged`.
     - Do not seed or backfill overlay here; copy is responsible for seeding.

3) Headers – overlay‑first (mirror queries)
   - HeaderDeltaExampleCopy: seed overlay order only (RefKindOrigin).
   - HeaderDeltaCreate/Update/Reset/Delete/DeltaMove: use overlay core.
   - HeaderMove (non‑delta): keep origin service.
   - HeaderDeltaList: use overlay `core.List` with header origin fetcher; no read‑time seeding.

4) Asserts – overlay‑first (mirror queries)
   - AssertDeltaExampleCopy: seed overlay order only (RefKindOrigin).
   - AssertDeltaCreate/Update/Reset/Delete/DeltaMove: overlay core.
   - AssertMove (non‑delta): keep origin service.
   - AssertDeltaList: overlay `core.List` with assert origin fetcher; no read‑time seeding.

5) Tests – update to overlay writers (no DB delta inserts)
   - Files (patterns):
     - `rrequest_delta_source_type_test.go`
     - `rrequest_delta_comprehensive_test.go`
     - `rrequest_delta_test.go`
     - Any other rrequest_* delta tests inserting directly into DB for delta examples.
   - Replace direct delta writes with Request RPCs:
     - Copy origin → delta: `QueryDeltaExampleCopy` / `HeaderDeltaExampleCopy` / `AssertDeltaExampleCopy`.
     - Create delta: `*DeltaCreate` RPCs.
     - Modify delta: `*DeltaUpdate` RPCs.
     - Reset/Delete/Move: call the overlay RPC variants.
     - List: use `*DeltaList` RPCs.
   - Keep origin behaviors via origin RPCs/services (e.g., `QueryCreate` on origin example).

6) Concurrency & CAS in example tests (no code migration)
   - Problem: “concurrent tail advance detected …” arises during example append CAS in setup.
   - Mitigation (test‑only): ensure example creation for the same endpoint is sequential in test helpers (avoid `t.Parallel` around that specific setup), or gate with a local mutex.
   - Do not change runtime repository logic.

7) Timeouts / heavy suites (optional)
   - If specific resource‑limit tests exceed the 60s CI JSON timeout, consider reducing fixture size for CI.

## Acceptance Criteria

- All rrequest delta suites pass without any read‑time bridging:
  - `TestDeltaSourceTypes` (ORIGIN → MIXED transitions only after *DeltaUpdate),
  - `TestQueryDeltaComprehensive`, `TestHeaderDelta*`, `TestAssertDelta*` (create/update/reset/delete/move/list),
  - Integration and edge cases are stable.
- rcollectionitem remains green post‑changes.
- No code remains that infers overlay state/order from DB delta rows at list time.

## Risks & Mitigations

- Risk: Some callers still write delta rows directly to DB → overlay lists won’t show them.
  - Mitigation: Update tests to call overlay RPCs for delta flows; document this in test helpers.
- Risk: MIXED detection inaccurate if overrides aren’t persisted via StateStore.
  - Mitigation: Ensure DeltaUpdate for origin‑ref writes StateStore only for provided fields.
- Risk: CAS flakes in example creation (unrelated to overlay)
  - Mitigation: serialize test setup for example creation.

## Rollout Strategy (PR Slices)

1) Queries overlay (copy/create/update/reset/delete/move/list) + minimal test updates focused on queries.
2) Headers overlay parity + header tests.
3) Asserts overlay parity + assert tests.
4) Fixture stabilization for example creation (tests only).
5) Final sweep: remove any vestigial bridge code and re‑run CI.

## Validation Steps

- Commands
  - `task test` or `pnpm nx run server:test` until green
  - Focused runs: `go test ./internal/api/rrequest -run 'Delta|QueryDelta|HeaderDelta|AssertDelta' -count=1`
  - CI JSON: `pnpm nx run server:test:ci` and inspect `packages/server/dist/go-test.json`.

- Manual sanity checks (optional)
  - Create origin example with a few queries; copy to delta; list delta (expect ORIGIN).
  - Update one query via DeltaUpdate; list delta (expect MIXED + updated values).
  - Create delta‑only; list delta (expect DELTA); move around via DeltaMove; reset/delete.

## Do/Don’t Summary

- DO: write overlay in Request RPCs for all delta flows; seed order during copy only; compute SourceKind via overlay core at list time.
- DO: keep origin operations as they are; route non‑delta flows to origin services.
- DO: update tests to use Request RPCs for delta writes.
- DON’T: add read‑time projection or fallback seeding; don’t migrate DB data.

