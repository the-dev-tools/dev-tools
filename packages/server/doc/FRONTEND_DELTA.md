# Frontend Delta & Sync System Analysis

This document details how the frontend (`packages/client`) implements the real-time synchronization system ("Delta System") using **TanStack DB** and **Connect RPC**.

## 1. The Sync Lifecycle (Stream Processing)

The frontend maintains a live connection to the server. Events flow from the server stream into the local TanStack DB.

```ascii
                                   [ Server ]
                                        |
                           (Stream: Insert/Update/Delete)
                                        |
                                        v
                                [ Client Buffer ]
                                        |
                          (Wait for Initial Snapshot?)
                           /            |           \
                       (No)           (Yes)          (Yes)
                         |              |              |
                     [Buffer]      [ Snapshot ]   [ Process ]
                                   [ Loaded!  ]        |
                                                       v
                                               [ Type Matcher ]
                                         (Checks $typeName of msg)
                                         /       |          |      \
                                  [Insert]   [Update]    [Upsert]  [Delete]
                                     |           |          |          |
                                     |           |      (Check DB)     |
                                     |           |      /        \     |
                                     |           |   (Found)  (Empty)  |
                                     v           v      |        |     v
                              [Write New]    [Merge] [Merge] [Write] [Remove]
                                     |           |      |        |     |
                                     v           v      v        v     v
                                   +---------------------------------------+
                                   |      TanStack DB (Local Store)        |
                                   +---------------------------------------+
```

## 2. Delta Merging Strategy

When an `Update` (or `Upsert` on existing item) is received, the frontend merges it using `Protobuf.mergeDelta`. It does **not** simply replace the object.

**Logic:**

- **Field Present:** Overwrite local value.
- **Field Missing:** Keep local value (preserve existing data).
- **Field is "Unset":** Explicitly remove the field (set to undefined/null).

```ascii
   Current Local State               Incoming Delta (Server)
+-----------------------+           +--------------------------+
| ID:     "User_123"    |           | ID:     "User_123"       |
| Name:   "Alice"       |           | Name:   "Alice Cooper"   | (Update)
| Role:   "Admin"       |           | (Role missing)           | (Keep)
| Status: "Active"      |           | Status: {kind: UNSET}    | (Delete)
+-----------------------+           +--------------------------+
            |                                     |
            +----------------+--------------------+
                             |
                     [ Protobuf.mergeDelta ]
                             |
                             v
                    Resulting New State
            +----------------------------------+
            | ID:     "User_123"               |
            | Name:   "Alice Cooper"  (Changed)|
            | Role:   "Admin"         (Kept)   |
            | Status: undefined       (Removed)|
            +----------------------------------+
```

## 3. Origin vs. Delta (ID Relationships)

The frontend often uses a "Dual Collection" pattern for scenarios like **Overrides** (e.g., changing a Request's URL only for a specific Test Run).

- **Origin Collection**: The source of truth (e.g., the saved Request).
- **Delta Collection**: The overrides (linked to the Origin).

**ID Rule:**

- **Origin ID**: The permanent ID of the base item.
- **Delta ID**: A unique ID for the _override record_ itself.
- **Linking**: The Delta record contains an `originId` field pointing to the Base item.

```ascii
      [ Origin Collection ]                  [ Delta Collection ]
         (Saved Request)                     (Temporary Override)

   +-------------------------+          +---------------------------+
   | ID: "REQ_A"             | <------- | OriginID: "REQ_A"         |
   | Method: "GET"           |    Link  | ID: "DELTA_777" (Unique)  |
   | URL: "http://api.com"   |          | URL: "http://dev.local"   |
   +-------------------------+          +---------------------------+
              ^                                       ^
              |                                       |
              +------------------+--------------------+
                                 |
                       [ useDeltaState Hook ]
                 (Logic: "Visual Merge" for UI)
                                 |
                 1. Check Delta Collection for
                    "originId == REQ_A"
                                 |
                   +-------------+-------------+
                   |                           |
             (Found Match)                (No Match)
                   |                           |
           Return Delta Value          Return Origin Value
          "http://dev.local"             "http://api.com"
```

## Core Components

### 1. `ApiCollection` Wrapper

**Location**: `packages/client/src/api-new/collection.internal.tsx`
Wraps `createCollection` from TanStack DB. Handles buffering stream events and the Initial Snapshot vs Stream race conditions.

### 2. Sync Lifecycle

1.  **Stream**: Opens immediately.
2.  **Buffer**: Queues events until Snapshot is done.
3.  **Snapshot**: Fetches full state via Unary RPC.
4.  **Apply**: Inserts Snapshot -> Replays Buffer -> Listens to Stream.

### 3. Optimistic Updates

**Location**: `packages/client/src/api-new/collection.internal.tsx` -> `makeUtils`

1.  **User Action**: UI triggers `insert/update`.
2.  **Optimistic Write**: `onMutate` updates TanStack DB immediately.
3.  **Network Request**: Sends RPC.
4.  **Wait**: `waitForSync` polls until the Server Stream echoes an event timestamped _after_ the mutation, ensuring consistency.

## Key Files

- `packages/client/src/api-new/collection.internal.tsx`: Sync loop & Optimistic logic.
- `packages/client/src/api-new/protobuf.tsx`: `mergeDelta` implementation.
- `packages/client/src/utils/delta.tsx`: `useDeltaState` for Origin/Delta merging.
