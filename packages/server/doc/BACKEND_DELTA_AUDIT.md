# Backend Delta System verification Report

## Executive Summary

The backend correctly implements the storage and isolation of Delta records. However, a **critical defect** exists in the Real-time Sync logic (`rhttp_converter.go`). The system fails to propagate "Unset" operations to the frontend, leading to stale UI states where overrides cannot be removed without a full page refresh.

## Findings

### 1. Storage & Isolation (Passed)

- **Delta Storage**: `HttpDeltaInsert`/`Update` correctly manipulate separate `mhttp.HTTP` records with `IsDelta=true`.
- **Base Protection**: `HttpDeltaUpdate` properly verifies `IsDelta=true` before writing, ensuring the Base Request is never mutated.
- **Stream Isolation**: The `HttpSync` (Base) and `HttpDeltaSync` (Delta) streams are correctly filtered using the `IsDelta` flag.

### 2. "Unset" Propagation (Failed)

**The Bug**: When a Delta field (e.g., `DeltaName`) is set to `nil` in the database (indicating "Remove Override" / "Inherit from Base"), the Sync converter **fails to send the `UNSET` signal** to the frontend.

**Current Logic (`httpDeltaSyncResponseFrom`):**

```go
if http.DeltaName != nil {
    // Sends KIND_VALUE
}
// ELSE: Do nothing (Field is left undefined in Protobuf message)
```

**Frontend Behavior (`Protobuf.mergeDelta`):**

- If a field is undefined in the incoming Delta message, the frontend **preserves the existing local value**.
- **Consequence**: Once an override is set, it cannot be removed via Sync. The UI will continue to show the old override value even after the backend has cleared it.

**Required Fix:**
The converter must explicitly handle the `nil` case during `eventTypeUpdate` by sending `KIND_UNSET`.

```go
if http.DeltaName != nil {
    // Send KIND_VALUE
} else {
    // Send KIND_UNSET
}
```

### 3. Scope of Fix

This issue affects **all** Delta Sync converters:

- `httpDeltaSyncResponseFrom` (Name, Method, URL)
- `httpHeaderDeltaSyncResponseFrom` (Key, Value, Enabled, Description)
- `httpSearchParamDeltaSyncResponseFrom`
- `httpBodyFormDataDeltaSyncResponseFrom`
- `httpBodyUrlEncodedDeltaSyncResponseFrom`
- `httpAssertDeltaSyncResponseFrom`

## Recommendation

Refactor `packages/server/internal/api/rhttp/rhttp_converter.go` to include `else` blocks for all Delta fields in `Update` events, setting the corresponding `_KIND_UNSET` union type.
