# Real-time Sync & Delta System

## Overview
The backend implements a specific pattern to support **TanStack DB** (and similar client-side replication strategies) on the frontend. This ensures that the UI is always in sync with the server state without manual refreshing.

## Architectural Pattern

### 1. RPC Interface
The standard pattern for a resource (e.g., `Http`) involves three types of RPCs:

*   **`*Collection` (e.g., `HttpCollection`)**
    *   **Purpose:** Returns the full initial state of a resource list.
    *   **Usage:** Called when the application loads or a view is initialized to get the "State of the World".

*   **`*Sync` (e.g., `HttpSync`)**
    *   **Purpose:** A server-streaming RPC that provides real-time delta updates.
    *   **Snapshot:** Can optionally provide an initial snapshot to ensure consistency before streaming begins.
    *   **Stream:** Pushes `Insert`, `Update`, and `Delete` events as they happen on the server.

*   **Mutations (`*Insert`, `*Update`, `*Delete`)**
    *   **Action:** Performs the actual database operation (write).
    *   **Side Effect:** Crucially, upon a successful transaction, it **publishes** an event to the `eventstream`.

### 2. Event Streaming (`packages/server/pkg/eventstream`)
*   **Generic Streamer:** `SyncStreamer[Topic, Event]` is a helper that manages subscriptions and broadcasting.
*   **Topics:** Events are scoped (usually by `WorkspaceID`) to ensure users only receive updates relevant to them.
*   **Access Control:** The `Sync` RPC applies filters (e.g., checking workspace membership) before establishing the stream, ensuring security.

### 3. Frontend Integration
*   **Library:** `@tanstack/react-db`.
*   **Mechanism:** The client subscribes to the `*Sync` endpoints.
*   **Reactivity:** Incoming `Insert`/`Update`/`Delete` messages are applied directly to the client-side in-memory database. This updates the UI reactively.

## Implementation Details
- **Sync Services:** Located in `packages/server/internal/api/` (e.g., `rhttp/rhttp_sync.go`).
- **Event Definition:** Events are typically defined in the Proto files (`packages/spec`) and correspond to the mutation types.
