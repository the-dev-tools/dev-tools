# RPC Layer Testing Strategy: In-Memory Integration Tests

## Objective

To prevent regressions in Data Access and Response Mapping logic (e.g., ensuring Delta Collections return correct values and exclude non-deltas) without the overhead of full end-to-end tests.

## The Problem

Current unit tests might mock too much or miss the integration between the Service layer, the Database, and the RPC Converter. The recent issue with `HttpHeaderDeltaCollection` returning "empty" deltas or base items as deltas highlights the need to verify:

1.  **Filtering:** Are we only returning `IsDelta=true` records?
2.  **Mapping:** are `ParentID` fields correctly mapped to the Protobuf response?
3.  **Data Fidelity:** Are nullable Delta fields (e.g., `DeltaValue`) correctly populated in the response?

## Proposed Solution: `rhttp_integration_test.go`

We should leverage the existing `sqlitemem` and `testutil` packages to create lightweight integration tests directly against the RPC handlers.

### Test Structure

#### 1. Setup

Use `testutil.NewTestServer(t)` (or similar) which:

- Spins up an in-memory SQLite database.
- Applies the schema.
- Initializes the Service and RPC layers.
- Creates a default Workspace and User.

#### 2. Seeding

Create helper functions to seed specific scenarios:

```go
// Seed a Base Header
baseHeader := s.CreateHeader(ctx, httpID, "Content-Type", "application/json")

// Seed a Delta Header (Override)
deltaHeader := s.CreateDeltaHeader(ctx, baseHeader.ID, "Content-Type", "application/xml")
```

#### 3. Execution

Call the RPC method directly (bypassing the network layer, or using `httptest`):

```go
resp, err := rpc.HttpHeaderDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
```

#### 4. Assertion

Verify the response structure deeply:

```go
assert.Len(t, resp.Msg.Items, 1) // Should only have the Delta, not the Base
item := resp.Msg.Items[0]
assert.Equal(t, deltaHeader.ID.Bytes(), item.DeltaHttpHeaderId)
assert.Equal(t, baseHeader.ID.Bytes(), item.HttpHeaderId) // Crucial check!
assert.Equal(t, "application/xml", *item.Value)           // Verify override value
```

## Implementation Plan

1.  **Create/Update Test File**: `packages/server/internal/api/rhttp/rhttp_collection_test.go`.
2.  **Scenarios to Cover**:
    - **Delta Collection filtering**: Ensure `IsDelta=false` items are NOT returned.
    - **ID Mapping**: Ensure `HttpHeaderId` in the response matches `ParentHttpHeaderID` from DB, not the Delta's own ID.
    - **Unset Fields**: Verify that if a Delta field is nil in DB, it is omitted (or nil) in the response.
    - **Value Propagation**: Verify that if a Delta field is set, it appears in the response.

## Example Test Code

```go
func TestHttpHeaderDeltaCollection_ReturnsCorrectDeltas(t *testing.T) {
    ctx, s := testutil.SetupRequestTest(t) // Hypothetical setup helper

    // 1. Setup Base Request & Header
    req := s.CreateRequest(ctx, "Base Request")
    baseHeader := s.CreateHeader(ctx, req.ID, "X-Base", "true")

    // 2. Create Delta Header
    // This should create a row with IsDelta=true, ParentID=baseHeader.ID
    deltaHeader := s.CreateHeaderDelta(ctx, baseHeader.ID, nil, ptr("false"), nil, nil)

    // 3. Call RPC
    resp, err := s.RPC.HttpHeaderDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
    require.NoError(t, err)

    // 4. Verify
    require.Len(t, resp.Msg.Items, 1)

    item := resp.Msg.Items[0]
    assert.Equal(t, deltaHeader.ID.Bytes(), item.DeltaHttpHeaderId)
    assert.Equal(t, baseHeader.ID.Bytes(), item.HttpHeaderId, "Should point to parent")
    assert.Equal(t, "false", *item.Value)
}
```
