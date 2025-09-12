# Request Overlay Delta — Phased Delivery (Thin Slices)

This breaks the overlay-only refactor into small, independently shippable phases so we can land changes safely, keep origin behaviors intact, and avoid read-time bridges or data migration.

Guardrails (apply to every phase)
- No read-time bridge code; readers merge overlay + origin only.
- No DB migrations; do not write legacy delta rows.
- Prefer RPCs/APIs in tests; avoid direct DB manipulation.
- Keep origin (non-delta) flows unchanged for Queries/Headers/Asserts.
- Each phase updates tests only for its scope; other families remain untouched.

Phase 1 — Queries: Copy + List (overlay-only)
- Scope
  - Implement `QueryDeltaExampleCopy` to seed overlay order with RefKindOrigin entries (no State/Delta writes).
  - Remove any query read-time projection/bridge in `QueryDeltaList`.
  - Keep all query writers (create/update/reset/delete/move) as-is for now.
- Tests updated
  - Only tests that list delta queries after copy: use `QueryDeltaExampleCopy` then `QueryDeltaList`.
  - Replace any direct DB seeding used for list assertions with the copy RPC.
- Acceptance
  - Query delta list suites pass with overlay-only list; no bridge in query list.
  - Headers/Asserts tests unaffected.
- Rollback
  - Revert only query copy/list changes.

Phase 2 — Queries: DeltaCreate + DeltaMove (overlay writers)
- Scope
  - `QueryDeltaCreate`: create overlay delta row + append to overlay order as RefKindDelta.
  - `QueryDeltaMove`: overlay rank-based move across any two query entries (origin-ref or delta-only).
- Tests updated
  - Replace DB inserts with `QueryDeltaCreate`.
  - Use `QueryDeltaMove` for reordering in delta examples.
- Acceptance
  - Query delta create/move tests pass; list reflects new order without bridges.
- Rollback
  - Revert query create/move RPC changes.

Phase 3 — Queries: DeltaUpdate + Reset + Delete (overlay writers)
- Scope
  - `QueryDeltaUpdate`: StateStore upsert for origin-ref; DeltaStore update for delta-only.
  - `QueryDeltaReset`: overlay reset (clear state for origin-ref; reset delta values for delta-only).
  - `QueryDeltaDelete`: remove overlay order ref; delete DeltaStore row (delta-only) or tombstone origin-ref.
- Tests updated
  - Convert remaining query delta tests to use overlay RPCs for modify/reset/delete.
- Acceptance
  - All query delta suites (comprehensive) pass; SourceKind transitions (ORIGIN→MIXED/DELTA) are correct.
- Rollback
  - Revert these writer RPCs only; earlier phases remain intact.

Phase 4 — Headers: Copy + List (overlay-only)
- Scope
  - `HeaderDeltaExampleCopy` seeds overlay order; `HeaderDeltaList` stays overlay-only (verify no bridges).
- Tests updated
  - Header list tests use copy + overlay list; no DB direct writes.
- Acceptance
  - Header delta list passes; queries remain green.
- Rollback
  - Revert header copy/list changes.

Phase 5 — Headers: Writers + Move (overlay)
- Scope
  - Implement `HeaderDeltaCreate/Update/Reset/Delete` and `HeaderDeltaMove` mirroring query semantics.
- Tests updated
  - Move header delta tests to overlay RPCs for all writes.
- Acceptance
  - Header delta suites pass; queries remain green.
- Rollback
  - Revert header writer changes.

Phase 6 — Asserts: Copy + List (overlay-only)
- Scope
  - `AssertDeltaExampleCopy` seeds overlay order; `AssertDeltaList` overlay-only (no bridges).
- Tests updated
  - Assert list tests use copy + overlay list.
- Acceptance
  - Assert delta list passes; queries/headers remain green.
- Rollback
  - Revert assert copy/list changes.

Phase 7 — Asserts: Writers + Move (overlay)
- Scope
  - Implement `AssertDeltaCreate/Update/Reset/Delete` and `AssertDeltaMove` mirroring query semantics.
- Tests updated
  - Assert delta tests call overlay RPCs for all writes.
- Acceptance
  - Assert delta suites pass; queries/headers remain green.
- Rollback
  - Revert assert writer changes.

Phase 8 — Test fixture stabilization (CAS/parallelism)
- Scope
  - Serialize example creation in shared helpers to avoid “concurrent tail advance detected …”.
  - Keep change test-only; do not alter runtime repos.
- Acceptance
  - Flakes due to example append CAS disappear in repeated runs.
- Rollback
  - Drop helper serialization if unnecessary.

Phase 9 — Final sweep (no bridges, parity check)
- Scope
  - Verify no read-time bridge remnants across queries/headers/asserts.
  - Remove dead code paths; ensure docs/tests reference overlay RPCs only for deltas.
- Acceptance
  - `go test ./internal/api/rrequest -run 'Delta|QueryDelta|HeaderDelta|AssertDelta' -count=1` passes locally.
  - CI JSON run (`pnpm nx run server:test:ci`) shows green for these suites.
- Rollback
  - Revert sweep commits only.

Per-Phase Validation Commands
- Focused: `go test ./internal/api/rrequest -run 'QueryDelta' -count=1` (Phase 1–3)
- Focused: `go test ./internal/api/rrequest -run 'HeaderDelta' -count=1` (Phase 4–5)
- Focused: `go test ./internal/api/rrequest -run 'AssertDelta' -count=1` (Phase 6–7)
- CI JSON: `pnpm nx run server:test:ci` and inspect `packages/server/dist/go-test.json`.

Notes
- rcollectionitem work stays separate; ensure it remains green after each phase.
- Keep PRs narrowly scoped to the phase; update only the necessary tests.
