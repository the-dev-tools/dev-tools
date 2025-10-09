# Overlay Registry Pilot — Headers

## Objective
Deliver the first production use of the overlay registry by migrating header CRUD, ordering, reset, and flow merge logic from bespoke adapters to registry-driven helpers without changing Connect RPC contracts or TypeSpec definitions.

## Current Pain Points
- `RequestRPC` carries custom structs (`headerOrderStore`, `headerStateStore`, `headerDeltaStore`) and manual seeding/merge logic diverging from queries/forms (`packages/server/internal/api/rrequest/rrequest.go:105`).
- Flow execution relies on `overlay/merge.Manager` for headers but only partially mirrors handler behaviour (`packages/server/internal/api/rflow/rflow.go:1292`), leading to drift risk.
- Overlay services expose sqlc-specific method names (`Count`, `SelectAsc`, `InsertIgnore`) but with inconsistent signatures compared to other families (`packages/server/pkg/service/soverlayheader/soverlayheader.go:37` vs `packages/server/pkg/service/soverlayform/soverlayform.go:37`).

## Deliverables
1. Registry configuration for headers with order/state/delta hooks and RPC builders.
2. Registry-backed implementations for header list/create/update/delete/reset/move.
3. Flow runtime merge pipeline updated to fetch header overlays through the registry.
4. Extended automated tests covering SourceKind transitions, drag-and-drop ordering, reset behaviour, and flow execution parity.
5. Documentation snippet describing how to consume the registry for future families (linked from the master plan).

## Work Breakdown

### 1. Service Normalization & Adapters
- Introduce `overlayregistry/adapters/header.go` with lightweight wrappers around `soverlayheader.Service` exposing the standardized method set (`OrderHooks`, `StateHooks`, `DeltaHooks`).
- Ensure conversions from sqlc byte slices → `idwrap.IDWrap` happen inside the adapter to keep registry callers agnostic of storage details.
- Add unit tests using an in-memory SQLite instance (`packages/server/pkg/testutil`) to validate adapter behaviour (count, insert, update, resolve example lookups).

### 2. Registry Bootstrap
- Create `packages/server/pkg/overlay/registry/registry.go` defining `OverlayFamily`, `FamilyConfig`, helper functions (`List`, `CreateDelta`, `Update`, `Delete`, `Reset`, `Move`), and registration map with concurrency-safe initialization.
- Register the header family with a unique key (`overlayregistry.Headers`) including:
  - Order hooks (count/select/upsert/delete/exists).
  - State hooks (get/upsert/clear/suppress/unsuppress).
  - Delta hooks (insert/update/get/delete/exists).
  - RPC builders translating `overlayregistry.Merged` into `requestv1.HeaderDeltaListItem` using `packages/server/pkg/translate/theader`.
  - SourceKind resolution mirroring `determineHeaderDeltaType` logic.
- Provide seed helpers to mirror current behaviour when an overlay order table is empty (load origin headers ordered via `sexampleheader.HeaderService`).

### 3. Handler Migration
- Refactor `RequestRPC.HeaderDeltaList` to call `overlayregistry.List` with the header family, keeping permission checks, origin/delta example lookup, and seeding intact.
- Update `HeaderDeltaCreate`, `HeaderDeltaUpdate`, `HeaderDeltaDelete`, `HeaderDeltaReset`, and `HeaderDeltaMove` to delegate to registry helpers. Resolve example IDs through registry-provided resolvers rather than direct service calls whenever possible.
- Maintain propagation helpers (`syncHeaderDeltaFromOrigin`) but ensure they operate on registry results.
- Add feature flag (e.g., environment variable `OVERLAY_HEADERS_REGISTRY=1`) for staged rollout; default to registry path once validated.

### 4. Flow Runtime Integration
- Update `overlay/merge.Manager` and `overlay/resolve.Request` to obtain header merge data via registry rather than hard-coded service calls.
- Ensure `resolve.Request` still seeds delta examples with default state when overlay tables are empty.
- Verify `rflow` uses the same SourceKind/ordering semantics by running existing flow integration tests and adding targeted assertions if necessary.

### 5. Validation & Hardening
- Extend `rrequest_header_delta_list_fix_test.go`, `rrequest_header_delta_regression_test.go`, and `rrequest_delta_reset_sync_test.go` to assert registry-driven outputs (including SourceKind values from `deltav1.SourceKind`).
- Add new tests:
  - Snapshot test comparing sorted order before/after migration.
  - Flow execution test verifying headers merged via registry match legacy merged payloads.
  - Error propagation test ensuring registry surfaces underlying sqlc failures with existing Connect status codes.
- Conduct manual QA in the desktop app: create delta headers, move items, reset to origin, and observe UI states.

### 6. Cleanup & Documentation
- Remove legacy adapter structs and helper functions once registry usage is enabled by default.
- Update `docs/plans/delta-overlay-unification-master.md` references and add a short `docs/overlay/registry.md` quickstart for other teams.
- Capture rollout notes (feature flag timeline, QA checklist, metrics) for future migrations.

## Timeline & Estimates
- Service normalization & adapters: ~1 day including tests.
- Registry bootstrap + header registration: ~2 days.
- Handler migration + feature flagging: ~2 days.
- Flow runtime integration: ~1 day.
- Validation, tests, documentation: ~1–2 days.
Total: **6–8 working days**, assuming dedicated focus and timely code review.

## Risks & Mitigations
- **Behaviour drift:** mitigate with golden assertions comparing SourceKind and ordering results between legacy and registry outputs; maintain feature flag for quick rollback.
- **Permission regressions:** keep existing `permcheck` calls untouched and add tests verifying unauthorized requests remain denied.
- **Performance issues:** benchmark `HeaderDeltaList` before/after; if necessary cache registry adapters or memoize seed results.
- **Partial rollout confusion:** document environment variable toggle and communicate to QA/desktop teams before enabling by default.

## Success Criteria
- All header delta endpoints and flow merges operate through registry helpers with no behavioural regressions.
- Feature flag flipped on in production without incidents; legacy adapters removable.
- Benchmarks show no statistically significant degradation in header list/update/move operations.
- Documentation empowers other engineers to repeat the process for queries, forms, and URL bodies.
