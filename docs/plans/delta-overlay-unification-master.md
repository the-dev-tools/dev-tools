# Delta & Overlay Unification — Master Plan

## Executive Summary
The current overlay/delta implementation duplicates logic across headers, queries, form bodies, URL-encoded bodies, and order-only collection items. Each family ships custom sqlc wrappers, bespoke handler adapters, and ad-hoc flow merge logic (`packages/server/internal/api/rrequest/rrequest.go:105`, `packages/server/pkg/overlay/merge/merge.go:28`, `packages/server/pkg/service/soverlayform/soverlayform.go:11`). This plan delivers a staged migration to a single registry-driven abstraction that:
- Guarantees behavioural parity between RPC handlers and flow execution.
- Makes drag-and-drop ordering, SourceKind calculation, and reset semantics reusable.
- Enables future architectural shifts (schema consolidation, TypeSpec redesign) without another large refactor.

## Strategic Goals
1. **Consistency:** one overlay path for CRUD, ordering, and runtime merge across all override-aware resources.
2. **Extensibility:** onboarding a new resource (e.g. experimental payload type) requires registry configuration rather than bespoke plumbing.
3. **Maintainability:** reduce boilerplate adapters and the risk of divergence between UI and flow behaviour.
4. **Optionality:** unlock future storage or API redesigns by first centralizing behaviour.

## Phase Breakdown

### Phase 1 — Foundations
- **OverlayFamily registry definition:** add `overlayregistry` package (`packages/server/pkg/overlay/registry`) describing `OverlayFamily`, `OrderHooks`, `StateHooks`, `DeltaHooks`, RPC builders, and feature flags (`orderOnly`, `supportsSuppression`, etc.).
- **Service normalization:** wrap `soverlay{header,query,form,urlenc}` so each exposes method parity: `Count`, `SelectAsc`, `LastRank`, `MaxRevision`, `Upsert`, `InsertIgnore`, `DeleteByRef`, `Exists`, `GetDelta`, `UpdateDelta`, `DeleteDelta`, `GetState`, `UpsertState`, `ClearOverrides`, `Suppress`, `Unsuppress`. Avoid modifying sqlc-generated code; use thin adapters stored alongside the registry.
- **Order-only mode scaffolding:** design registry hooks for resources without delta/state tables (collection items) so they can reuse ordering APIs.
- **Documentation:** draft `docs/overlay/registry.md` with examples, call flow diagrams, and migration checklist for future contributors.

### Phase 2 — Backend Integration Spine
- **Registry-backed helpers:** implement reusable operations (`List`, `CreateDelta`, `Update`, `Reset`, `Delete`, `Move`) inside `overlayregistry` that delegate to registered hooks and emit domain-specific RPC builders.
- **RequestRPC refactor:** teach `packages/server/internal/api/rrequest` to look up families by key (`headers`, `queries`, `forms`, `urlenc`, `asserts?`) instead of constructing manual adapters. Keep permission checks, validation, and seeding logic local.
- **Flow runtime alignment:** update `packages/server/pkg/overlay/merge.Manager` and `overlay/resolve.Request` to consult registry entries for all supported families, ensuring the same merge algorithm is used in runtime execution (`packages/server/internal/api/rflow/rflow.go:1200`).
- **Utility consolidation:** move SourceKind derivation, seed helpers, and rank operations into shared modules accessible from both RPC and flow paths.

### Phase 3 — Incremental Family Rollout
- **Headers pilot:** (see `docs/plans/delta-overlay-headers-plan.md`) wire headers through the registry, validating with existing delta list/reset tests.
- **Queries migration:** reuse registry infrastructure; update request handlers and flow merge to eliminate query-specific adapters; ensure propagation helpers still function.
- **Form bodies & URL-encoded bodies:** port overlay CRUD and merge logic; align with `rbody` delta reset tests and `resolve.Request` integration.
- **Collection items (order-only):** leverage order-only registry mode to unify folder/endpoint/example ranking (`packages/spec/api/collection-item/item.tsp:27`), preparing ground for possible future overrides.
- **Optional extras:** evaluate asserts or other overlay-capable families once core set stabilizes.

### Phase 4 — Consolidation & Cleanup
- **Remove legacy adapters:** delete `headerOrderStore`, `queryOrderStore`, hand-rolled overlay helpers, and redundant seeding utilities.
- **Schema convergence assessment:** revisit Option 5 (polymorphic overlay tables) after behaviour stabilizes; prototype migration scripts; profile performance.
- **TypeSpec evolution:** consider adding a unified `DeltaOrder.Interface` once backend behaviour is fully consolidated to reduce API duplication.
- **Testing framework:** introduce shared test harnesses for overlay families (common fixture builder, SourceKind snapshot assertions, drag-and-drop scenarios).

### Phase 5 — Long-Term Enhancements
- **Caching/materialized views:** if runtime merge remains hot, evaluate precomputation backed by TTL caches or triggers.
- **Feature-flag migration controls:** ensure each family can toggle between legacy and registry paths for staged rollouts and rollback safety.
- **Observability:** standardize structured logs/metrics emitted by overlay operations for debugging (move history, reset counts, override adoption).

## Deliverables & Milestones
- Registry package with tests validating hook invocation order and error propagation.
- Refactored header/query/form/url handlers with baseline benchmarks compared against current implementation.
- Flow runtime smoke tests demonstrating merged payload equality before/after migration.
- Updated developer documentation (registry usage guide, migration cookbook).
- Cleanup PR removing legacy adapters once all families complete migration.

## Dependencies & Tooling
- Requires up-to-date sqlc-generated code (`packages/server/pkg/service/soverlay*.go`).
- Leverage existing ranking utilities (`packages/server/pkg/overlay/rank`) and ID wrappers.
- Maintain compatibility with Connect RPC definitions generated from TypeSpec (`packages/spec/api/collection-item/request.tsp`).
- Ensure tests leverage in-memory SQLite fixtures (`packages/server/pkg/testutil`) for deterministic overlay state validation.

## Risks & Mitigations
- **API regressions:** mitigate via end-to-end tests and temporary feature flags toggling registry usage per family.
- **Behaviour drift:** compare SourceKind outputs using golden tests; ensure reset/move semantics match the current system (`packages/server/internal/api/rrequest/rrequest_delta_reset_sync_test.go:118`).
- **Performance regressions:** benchmark `HeaderDeltaList`, `QueryDeltaList`, and flow execution; cache registry lookups; pre-create adapter structs.
- **Migration fatigue:** deliver in small, reviewable PRs (headers → queries → bodies → order-only) to avoid overwhelming reviewers.
- **Operational churn:** coordinate with frontend teams when altering ordering semantics; provide documentation for any behavioural nuance changes.

## Success Metrics
- 100% of overlay-aware endpoints route through registry-backed helpers.
- Flow execution uses registry merges for headers, queries, forms, and URL bodies without additional special cases.
- Drag-and-drop ordering and reset functionality behave identically (confirmed via automated tests and UX sign-off).
- Onboarding a new overlay family requires fewer than ~100 LOC (registry entry + tests) instead of bespoke wiring.
- Post-migration benchmarks show neutral or improved latency for critical endpoints (HeaderDeltaList, QueryDeltaList) and flow execution.

## Open Questions & Follow-Ups
- Should asserts join the registry or remain bespoke until overrides become first-class?
- Do we enforce a common schema for overlay tables in the short term, or wait until after rollout?
- How do we expose registry configuration for feature flagging (environment variable, config file, build tag)?
- What telemetry is needed to monitor adoption (e.g., percentage of flows using delta overrides)?
- Are there opportunities to DRY seeding logic across families while keeping behaviour consistent?
