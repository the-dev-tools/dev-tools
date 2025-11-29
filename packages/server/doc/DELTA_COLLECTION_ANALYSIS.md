# Delta Collection & Sync Consistency Report

## Question
"Are we sending stuff like that in Collection as unset or not?"

## Analysis

### 1. The Difference: Snapshot vs. Stream
-   **Collection (Snapshot/GET):** Represents the **current state** (Noun).
-   **Sync (Stream/PATCH):** Represents a **state change** (Verb).

### 2. Collection Logic (`HttpDeltaCollection`)
**Current Implementation:**
```go
if http.DeltaName != nil {
    delta.Name = http.DeltaName
}
// If nil, delta.Name is left nil (omitted from Protobuf message)
```

**Frontend Interpretation:**
When the frontend receives the initial collection:
1.  It inserts the item into the local TanStack DB.
2.  If `Name` is missing (undefined), the local record has `name: undefined`.
3.  The `useDeltaState` hook sees `delta.name` is undefined and correctly falls back to the Origin value.

**Verdict:** **Correct.**
We do *not* need to send an explicit `UNSET` signal in a Collection response. Omitting the field correctly represents "no override exists".

### 3. Sync Logic (`HttpDeltaSync`)
**Current Implementation (Fixed):**
```go
if http.DeltaName != nil {
    // Send KIND_VALUE("New Name")
} else {
    // Send KIND_UNSET
}
```

**Frontend Interpretation:**
When the frontend receives this update:
1.  It runs `Protobuf.mergeDelta`.
2.  If it sees `KIND_VALUE`, it updates the field.
3.  If it sees `KIND_UNSET`, it *explicitly sets the field to undefined* in the local store.

**Verdict:** **Correct (after recent fix).**
This explicit signal is required because `mergeDelta` needs to distinguish between "Don't touch this field" (missing) and "Remove this field" (UNSET).

## Conclusion
The system is now consistent.
-   **Collections** implicitly unset fields by omitting them.
-   **Sync Updates** explicitly unset fields using `KIND_UNSET`.

No changes are required for the `Collection` endpoints.
