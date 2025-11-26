### Phase 3: Integration via Dependency Injection (Revised)

**Goal:** Inject the `RequestResolver` into the Flow Service, replacing manual component loading.

1.  **Update `FlowServiceV2RPC` struct:**
    - Add `resolver resolver.RequestResolver` field.
    - Update `New` constructor to accept it.
2.  **Update `rflowv2.buildNodeRequest` (or `buildRequestFlowNode`):**
    - Instead of manually fetching `httpRecord`, `headers`, `queries`, etc., call `s.resolver.Resolve(ctx, cfg.HttpID, cfg.DeltaExampleID)`.
    - The `Resolve` method returns a `*delta.ResolveHTTPOutput`.
    - Use the resolved components to call `nrequest.New`.
    - _Crucial:_ `nrequest.New` currently takes `mhttp` types. `delta.ResolveHTTPOutput` provides exactly these types. This should be a direct mapping.
3.  **Wiring (main.go or api.go):**
    - Construct `StandardResolver`.
    - Pass it to `rflowv2.New`.
