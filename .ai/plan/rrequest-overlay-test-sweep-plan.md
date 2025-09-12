# rrequest Overlay Test Sweep Plan (Finish Full Suite)

## Objective
Bring the entire `packages/server/internal/api/rrequest` test suite to green under overlay-only delta semantics (no read-time bridge), keeping origin-only flows intact and deterministic.

## Summary Of Remaining Gaps
- Some header tests still assume “auto-create on list” and/or read DB state directly:
  - rrequest_copy_paste_test.go
  - rrequest_header_integration_test.go
- A few comprehensive/edge tests rely on DB reads or pre-overlay semantics:
  - rrequest_delta_comprehensive_test.go (bulk, nested chain)
- Origin-only header collection suites show count/order mismatches and occasional DB locks:
  - rrequest_header_collection_comprehensive_test.go

## Guiding Principles
- Overlay flows (delta examples) must be seeded explicitly via `HeaderDeltaExampleCopy`/`QueryDeltaExampleCopy`/`AssertDeltaExampleCopy` before listing.
- Overlay assertions must use RPC lists (`*DeltaList`) and SourceKind, not read DB rows.
- Origin-only flows keep using existing services and RPCs; linked-list integrity remains verified via service where it makes sense.
- No read-time bridge logic anywhere; tests must not expect list-time side effects.

## Work Items (by file)

1) rrequest_copy_paste_test.go
- Seed overlay for delta examples before all delta list reads.
- Adjust expected counts:
  - After seeding delta1 with origin (3 ORIGIN items) and copying 2 modified/new items, delta2 should show 5 items (3 ORIGIN + 2 DELTA/MIXED), not 4.
- Keep copy-filter logic (skip ORIGIN, copy MIXED/DELTA only).
- Verify via `HeaderDeltaList` only; remove any DB assertions.

2) rrequest_header_integration_test.go
- Ensure every delta-view step calls `HeaderDeltaExampleCopy(collectionExampleID → deltaExampleID)` before `HeaderDeltaList`.
- Use overlay IDs from `HeaderDeltaList` for `HeaderDeltaMove` operations.
- Update text/expectations from “auto-create” to “seed overlay then list”.

3) rrequest_header_collection_comprehensive_test.go (origin-only)
- Keep origin RPC flows; for header counts in this suite:
  - Update `verifyHeaderCount` to use `GetHeadersOrdered` (service) for count to avoid any filtering differences in `HeaderList` vs internal state.
- Investigate table lock errors in concurrency subtests; mitigate by:
  - Setting a small busy timeout on the test DB, or
  - Reducing concurrency level within tests (keep logical verification intact).
- Ensure linked-list verification continues to use service accessors.

4) rrequest_delta_comprehensive_test.go
- BulkOperationPerformance
  - Replace DB assertion (`eqs.GetExampleQueriesByExampleID(delta)`) with overlay list (`QueryDeltaList`) size check after seeding copy.
  - Keep timing measurement around `QueryDeltaExampleCopy`.
- NestedDeltaChain
  - Use overlay IDs from `HeaderDeltaList` when creating nested deltas.
  - Validate results via `HeaderDeltaList` (overlay) only.
- InvalidParentRelationship
  - Ensure error expectations align with the current implementation; if a delta header ID is passed as parent, validate correct error or fallback to origin parent depending on intended semantics.

5) Test-wide helpers
- Add `seedHeaderOverlay(t, rpc, deltaID, originID)` test helper to wrap `HeaderDeltaExampleCopy` safely.
- Extend serialization helper if needed for headers in heavy append loops (currently CAS issues were seen mainly for examples/queries).

## Acceptance Criteria
- `go test ./packages/server/internal/api/rrequest -count=1` passes locally.
- Overlay-focused subsets remain green:
  - `-run 'QueryDelta'`  `-run 'HeaderDelta(Create|List|Move|Reset)'`  `-run 'AssertDelta'`.
- No tests rely on read-time bridging; all overlay delta state/order come from explicit RPC writers and copy seed.

## Risks & Mitigations
- Expectation drift after seeding (counts increase):
  - Mitigation: Update expected counts to reflect ORIGIN items present post-seed.
- Concurrency flakes in header collection suite:
  - Mitigation: favor service-based integrity checks; consider small busy timeout or reduced goroutine fan-out if necessary.

## Execution Order
1. Convert rrequest_copy_paste_test.go (seeding, counts, overlay assertions).
2. Convert rrequest_header_integration_test.go (seeding, overlay IDs for moves, assertions).
3. Fix rrequest_delta_comprehensive_test.go (bulk overlay counts; nested chain overlay IDs).
4. Adjust rrequest_header_collection_comprehensive_test.go (count method; optional busy timeout; keep origin logic).
5. Full suite run and polish remaining minors.

## Out-of-Scope (deferred)
- Adding `soverlayassert` (overlay service for asserts). Current plan stabilizes asserts using existing DB writers and overlay-only lists.

