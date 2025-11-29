# System Architecture & Frontend Alignment Report

## 1. Architecture Assessment
The backend employs a **"Wide Table" / Explicit Column** pattern for storing Deltas (e.g., `Name` vs `DeltaName` columns).

**Verdict: Good / Pragmatic.**

### Strengths:
-   **Type Safety:** Go structs and SQL columns enforce types (`string`, `int`) for overrides, avoiding the chaos of untyped JSON blobs.
-   **Performance:** Simple SQL queries. No complex recursive CTEs or JSON parsing required for standard operations.
-   **Simplicity:** Easy to understand. A row is either a Base (uses `Name`) or a Delta (uses `DeltaName`).

### Weaknesses:
-   **Verbosity:** Adding a field requires adding two columns (Base + Delta) and updating multiple structs.
-   **Depth Limitation:** The schema strongly implies a **Depth-1** inheritance model (Base -> Delta). Supporting a chain of overrides (Base -> Delta1 -> Delta2) would be complex and ambiguous with the current `IsDelta` binary flag and dual-column structure.

## 2. Frontend Alignment
The Backend architecture aligns well with the Frontend's **"Origin + Delta"** pattern (`useDeltaState`).

-   **Frontend Logic:** The UI expects to load an "Origin" item and a "Delta" item, merging them visually.
-   **Backend Support:** The backend correctly stores these as distinct entities linked by `ParentID`, allowing the frontend to fetch exactly what it needs.

**The Alignment Gap is Implementation, not Architecture.**
As identified in the Audit (`BACKEND_DELTA_AUDIT.md`), the architecture *supports* the requirement (via nullable columns), but the *converter logic* fails to transmit the "Unset" state.

## 3. Refactoring Recommendations

### A. Immediate Fix (Critical)
**Do not refactor the database.** The current schema is sufficient. Focus entirely on fixing the **Converter Logic** in `rhttp_converter.go` to correctly map `nil` database fields to `KIND_UNSET` Protobuf messages. This bridges the gap between the DB's `NULL` and the Frontend's `undefined` vs `unset` distinction.

### B. Long-Term Considerations
If the product requirement evolves to support **Multi-Layer Inheritance** (e.g., Organization Defaults -> Workspace Defaults -> User Overrides), the current "Wide Table" approach will become a bottleneck.
*   **Recommendation:** Only if multi-layer becomes a requirement, consider moving the "Delta" columns into a structured `JSONB` column or a separate `DeltaValue` table to allow dynamic layering without schema bloat.
*   **Current Status:** For the current "Request Override" feature, the current architecture is optimal.

## Conclusion
The system architecture is sound and well-aligned with the frontend's mental model. The "broken" feeling comes from the bug in the translation layer, not the data model itself.
