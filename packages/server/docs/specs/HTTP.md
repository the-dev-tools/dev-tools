# HTTP Specification

## Overview
The HTTP system in DevTools is designed to handle the definition, execution, and persistence of HTTP requests. It acts as the core "Postman-like" engine, managing collections of requests, their execution history, and real-time synchronization with the frontend.

## Core Concepts

### 1. Request Definition
A Request is the fundamental unit, defining *what* to send.
- **Method & URL:** Standard HTTP verbs and endpoints.
- **Headers:** Key-value pairs (supports enabled/disabled state).
- **Body:** Supports various types:
    - `None`
    - `Raw` (Text, JSON, XML, HTML)
    - `FormData` (Multipart)
    - `UrlEncoded`
    - `Binary` (File uploads)
- **Authentication:** Auth configurations (Bearer, Basic, etc.) are abstracted into the header generation logic.

### 2. The "Delta" System
To support non-destructive edits and history tracking, requests often use a delta mechanism.
- **Base Request:** The saved state of a request in a collection.
- **Delta:** A temporary or modified state (e.g., when a user types in the UI but hasn't saved).
- **Parent Relationship:** Deltas link back to a `ParentHttpID`.
- **Field Overrides:** Deltas store only what changed (conceptually), though the storage model may store full snapshots for simplicity depending on the implementation.

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
    - `IsDelta` (bool): Marks a request as a transient edit.
    - `ParentHttpID` (UUID): Links a delta to its source.
    - `DisplayOrder` (float): Manages sorting in the collection list.

## Database Schema
- **`http` table:** Stores the core request metadata (Method, URL).
- **`http_header`, `http_param`, `http_body`:** normalized tables linked by `http_id`.
- **`http_response`:** Stores the execution results (History).