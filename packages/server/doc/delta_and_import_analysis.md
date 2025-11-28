# Delta System and HAR Import Analysis

## Executive Summary
This report analyzes the current implementation of the Delta System and HAR Import logic in the backend and frontend. 
**Key Finding:** The backend correctly implements a non-destructive Delta system where updates are stored as separate rows. However, the HAR Import logic (`harv2`) currently **does not support overwriting or updating existing requests**. It always creates new entities (Flows, Requests, Deltas) with new IDs, which leads to duplication rather than updates. Additionally, the HAR import creates "shell" Delta requests without explicit Delta Headers, relying on inheritance from the simultaneously created Base Request.

## 1. Frontend Handling (Delta Consumption)
The frontend (`packages/client/src/routes/http/delta.tsx`) is designed to handle Deltas correctly:
-   It uses a routing structure that identifies both the `httpId` (Base) and `deltaHttpId` (Delta).
-   It fetches the Delta entity.
-   The UI likely merges the Base and Delta state for display (inheriting values where Delta fields are null).

## 2. Backend Handling (Delta Storage)
The backend (`packages/server/internal/api/rhttp/rhttp_delta.go`) implements a robust Delta storage mechanism:
-   **Storage**: Deltas are stored in the `http` table with `IsDelta = true` and a `ParentHttpID` pointing to the Base Request.
-   **Inheritance**: Delta fields (`DeltaName`, `DeltaUrl`, `DeltaMethod`) are nullable. A `nil` value implies inheritance from the Base Request.
-   **Immutability**: The `HttpDeltaUpdate` RPC only updates the Delta row. It **never mutates** the Base Request, preserving the original state.
-   **Child Entities**: Headers, Search Params, and Body Forms also support Delta rows (e.g., `http_header` table has `DeltaKey`, `DeltaValue`, etc.).

## 3. HAR Import Analysis (`harv2`)
The HAR Import logic (`packages/server/pkg/translate/harv2`) converts HAR files into internal models.
-   **Process**: For each HAR entry, it creates:
    1.  A **New Base Request** (`mhttp.HTTP`) with all headers/params from the HAR.
    2.  A **New Delta Request** (`mhttp.HTTP`) that links to the Base Request.
-   **Delta Shell**: The created Delta Request is a "shell" (only the main HTTP object). It **does not create Delta Headers or Delta Params**.
    -   *Implication*: The Delta Request inherits *all* headers/params from the Base Request. Since the Base Request is created from the same HAR entry, this results in the Delta effectively matching the HAR.
-   **No Overwrite Logic**: The import process uses `idwrap.NewNow()` to generate **new IDs** for every entity (Flow, Node, Request).
    -   It does **not** check for existing entities to update.
    -   Importing the same HAR twice results in **two separate copies** of the Flow and Requests.

## 4. Addressing User Concerns

### "Backend should not mutate etc etc it should store delta"
**Status: Verified Correct.**
The backend adheres to this principle. Operations on Deltas (`HttpDeltaUpdate`) are strictly isolated to the Delta rows. The Base Request remains untouched.

### "I think for overwrite Ids should be same"
**Status: Issue Identified.**
The current HAR import logic (`harv2` + `rimportv2`) **does not support overwriting**.
-   It always generates new IDs.
-   There is no logic to match an incoming HAR entry to an existing Request ID or Flow ID.
-   **Recommendation**: To support overwriting/updating, the Import logic needs to accept an optional "Target ID" or implement a matching strategy (e.g., by Name/URL within a Workspace) to identify existing entities and update their Delta rows instead of creating new ones.

### "How HAR file turn into header and delta"
**Status: Simplified Implementation.**
-   Currently, the HAR import converts the HAR entry into a **Base Request** (with all headers) and a **Delta Request** (empty shell).
-   It does **not** calculate the difference between an existing request and the HAR (because it always creates a new Base Request).
-   If the goal is to "Apply HAR as Delta to Existing Request", the current logic is insufficient. It would need to:
    1.  Load the Existing Base Request.
    2.  Compare HAR headers vs. Base headers.
    3.  Create `HttpHeaderDelta` entries for the differences.
