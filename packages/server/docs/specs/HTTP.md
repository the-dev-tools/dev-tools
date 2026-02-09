# HTTP Specification

## Overview

The HTTP system in DevTools is designed to handle the definition, execution, and persistence of HTTP requests. It acts as the core "Postman-like" engine, managing collections of requests, their execution history, and real-time synchronization with the frontend.

## Core Concepts

### 1. Request Definition

A Request is the fundamental unit, defining _what_ to send.

- **Method & URL:** Standard HTTP verbs and endpoints.
- **Headers:** Key-value pairs (supports enabled/disabled state).
- **Body:** Supports various types:
  - `None`
  - `Raw` (Text, JSON, XML, HTML)
  - `FormData` (Multipart)
  - `UrlEncoded`
  - `Binary` (File uploads)
- **Authentication:** Auth configurations (Bearer, Basic, etc.) are abstracted into the header generation logic.

### 2. The Delta & Snapshot System

Every `http` record is classified by two independent boolean columns (`is_delta`, `is_snapshot`) into one of three mutually exclusive states:

| `is_delta` | `is_snapshot` | State        | Description                                                                                 |
| ---------- | ------------- | ------------ | ------------------------------------------------------------------------------------------- |
| `FALSE`    | `FALSE`       | **Base**     | The canonical saved request in a collection. Visible in the sidebar.                        |
| `TRUE`     | `FALSE`       | **Delta**    | An inheritance-based override of a base record. Inherits all fields, overrides selectively. |
| `FALSE`    | `TRUE`        | **Snapshot** | An immutable, fully-resolved copy captured at execution time.                               |

The combination `is_delta=TRUE, is_snapshot=TRUE` is **invalid** and enforced by:

- A `CHECK` constraint in the DDL schema (for fresh databases).
- `BEFORE INSERT/UPDATE` triggers (for migrated databases).

#### Base Records

The normal saved requests users see in the workspace tree.

#### Delta Records

Deltas implement an **inheritance/override system** on top of base records. A delta is a child of a base record that inherits every field from its parent and selectively overrides only the fields it specifies. This is used when users edit a request in the UI before saving — the edits are stored as a delta, leaving the original base untouched.

- **Parent Relationship:** Every delta links back to a `ParentHttpID` (enforced by a `CHECK` constraint: `is_delta = FALSE OR parent_http_id IS NOT NULL`). Deleting the parent cascades to all its deltas.
- **NULL-means-inherit:** Each overridable field has a corresponding `delta_*` column (e.g., `delta_url`, `delta_method`, `delta_name`, `delta_body_kind`, `delta_description`). A `NULL` delta field means "inherit from the parent base". Only non-NULL delta fields override the base value.
- **Child entity inheritance:** The same override pattern extends to child entities (headers, params, body forms, URL-encoded entries, body raw, asserts). Each child delta can either:
  - **Override** an existing parent child (via `parent_http_header_id`, `parent_http_search_param_id`, etc.) — inheriting its fields and selectively overriding them.
  - **Add** a new child (when the parent link is `NULL`) — appended to the resolved collection.
- **Resolution:** `packages/server/pkg/delta` merges base + delta into a fully resolved view at read time. The resolver walks every field and child entity, applying the override-or-inherit logic, and returns a complete HTTP request with `IsDelta = false`.

#### Snapshot Records

Immutable point-in-time captures created when a request is executed via `HttpRun`.

- **Fully resolved:** If the executed request was a delta, the snapshot stores the merged result, not raw delta data.
- **Deep cloned:** All child entities (headers, params, body, asserts, response) are copied with new IDs.
- **Linked to versions:** The snapshot ID matches the `http_version.id` that triggered it.
- **Hidden from sidebar:** `GetHTTPsByWorkspaceID` excludes snapshots; they are only surfaced via the version/history UI.

### 3. Execution & History

When a request is "Run":

1.  **Interpolation:** Variables (Environment/Flow) are substituted into the URL, headers, and body.
2.  **Transmission:** The request is sent via the Go HTTP client.
3.  **Response:** The raw response (Status, Headers, Body, Timing) is captured.
4.  **Persistence:** The execution result is saved as a **History Item**, linked to the original Request. This ensures the history is immutable even if the request definition changes later.

## Backend Architecture

### API Layer (`packages/server/internal/api/rhttp`)

- **Role:** Entry point for ConnectRPC.
- **Responsibilities:**
  - Validates incoming Protobuf messages.
  - Orchestrates transactions.
  - Calls the Service Layer.
  - Publishes events to the `eventstream` for real-time UI updates.
- **Files:** `rhttp_exec.go` (execution), `rhttp_crud.go` (management).

### Service Layer (`packages/server/pkg/service/shttp`)

- **Role:** Business logic and data access adapter.
- **Responsibilities:**
  - Converts between Internal Models (`mhttp`) and DB Models (`gen`).
  - Executes `sqlc` queries.
  - Handles complex logic like "Duplicating a Request" (which involves copying headers, params, body, etc.).

### Domain Model (`packages/server/pkg/model/mhttp`)

- **Pure Go Structs:** Decoupled from DB and API.
- **Key Fields:**
  - `IsDelta` (bool): Marks a request as an inherited override of a base record (see Delta Records).
  - `IsSnapshot` (bool): Marks a request as an immutable version snapshot.
  - `ParentHttpID` (UUID): Links a delta to its parent base record for inheritance.
  - `Delta*` fields (`DeltaName *string`, `DeltaUrl *string`, etc.): Nullable override fields. `nil` = inherit from parent.
  - `DisplayOrder` (float): Manages sorting in the collection list.

## Database Schema

- **`http` table:** Stores the core request metadata (Method, URL).
- **`http_header`, `http_param`, `http_body`:** normalized tables linked by `http_id`.
- **`http_response`:** Stores the execution results (History).
