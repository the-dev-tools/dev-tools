# Mutation System Specification

## Overview

The mutation system (`packages/server/pkg/mutation/`) provides a unified approach to handling data mutations with:

1. **Automatic cascade event collection** - Collects delete events for all child entities before DB CASCADE removes them
2. **Transaction management** - Begin/Commit/Rollback lifecycle with proper cleanup
3. **Auto-publish to sync streamers** - Events flow to real-time sync after successful commit
4. **Dev-only replay recording** - JSONL event recording for debugging (build tag controlled)

### Problem Statement

When deleting parent entities (File, HTTP, Flow, Workspace), the database CASCADE constraints automatically remove child records. However, the sync system needs to know about ALL deleted entities to notify connected clients. Without explicit tracking, cascade-deleted children become "invisible" to the sync layer.

**Before mutation system:**

```
Client deletes HTTP -> DB deletes headers/params via CASCADE -> Sync only knows about HTTP
```

**With mutation system:**

```
Client deletes HTTP -> Mutation collects headers/params BEFORE delete -> DB CASCADE runs ->
Commit auto-publishes ALL events -> Sync notifies clients about HTTP, headers, params
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              RPC Handler                                    │
│  ┌─────────────┐    ┌─────────────┐    ┌──────────────────────────────────┐│
│  │   FETCH     │    │   CHECK     │    │              ACT                 ││
│  │  (Reader)   │ -> │  (Memory)   │ -> │                                  ││
│  │  Get data   │    │  Validate   │    │  ┌────────────────────────────┐  ││
│  └─────────────┘    └─────────────┘    │  │    mutation.Context        │  ││
│                                         │  │  ┌──────────────────────┐ │  ││
│                                         │  │  │ Begin(ctx)           │ │  ││
│                                         │  │  │ DeleteFile/HTTP/Flow │ │  ││
│                                         │  │  │ Commit(ctx)          │ │  ││
│                                         │  │  └──────────────────────┘ │  ││
│                                         │  └────────────────────────────┘  ││
│                                         └──────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────┘
                                                      │
                                                      │ On Commit
                                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Publisher                                       │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │ PublishAll(events []Event)                                              │ │
│  │   - Routes events by EntityType                                         │ │
│  │   - Publishes to appropriate SyncStreamer                               │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                                      │
                                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Event Streamers                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ FileStreamer │  │ HTTPStreamer │  │ FlowStreamer │  │    ...       │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
                                                      │
                                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Connected Clients                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                       │
│  │   Client A   │  │   Client B   │  │   Client C   │                       │
│  │  (Desktop)   │  │   (Web UI)   │  │   (CLI)      │                       │
│  └──────────────┘  └──────────────┘  └──────────────┘                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Types

### EntityType (`event.go`)

Identifies the type of entity being mutated. Uses `uint16` for compact storage and fast comparison (no string comparisons at runtime).

```go
type EntityType uint16

const (
    // Workspace entities
    EntityWorkspace EntityType = iota
    EntityWorkspaceUser
    EntityEnvironment
    EntityEnvironmentValue
    EntityTag

    // HTTP entities
    EntityHTTP
    EntityHTTPHeader
    EntityHTTPParam
    EntityHTTPBodyForm
    EntityHTTPBodyURL
    EntityHTTPBodyRaw
    EntityHTTPAssert
    EntityHTTPResponse
    EntityHTTPResponseHeader
    EntityHTTPResponseAssert
    EntityHTTPVersion

    // Flow entities
    EntityFlow
    EntityFlowNode
    EntityFlowNodeHTTP
    EntityFlowNodeFor
    EntityFlowNodeForEach
    EntityFlowNodeCondition
    EntityFlowNodeJS
    EntityFlowEdge
    EntityFlowVariable
    EntityFlowTag

    // File system
    EntityFile
)
```

### Operation (`event.go`)

The type of mutation operation:

```go
type Operation uint8

const (
    OpInsert Operation = iota
    OpUpdate
    OpDelete
)
```

### Event (`event.go`)

A single mutation event that will be published after commit:

```go
type Event struct {
    Entity      EntityType    // What type of entity
    Op          Operation     // Insert/Update/Delete
    ID          idwrap.IDWrap // Entity's primary key
    WorkspaceID idwrap.IDWrap // For routing to correct subscribers
    IsDelta     bool          // True for delta/versioned entities
    Payload     any           // For insert/update - the entity data
    Patch       any           // For update - the changed fields only
}
```

### Context (`context.go`)

Manages a mutation transaction with automatic cascade event collection:

```go
type Context struct {
    db        *sql.DB
    tx        *sql.Tx
    q         *gen.Queries
    events    []Event        // Collected events for publishing
    recorder  Recorder       // Dev-only JSONL recorder
    publisher Publisher      // Auto-publish after commit
}
```

**Key Methods:**

| Method             | Description                                            |
| ------------------ | ------------------------------------------------------ |
| `New(db, ...opts)` | Create a new mutation context                          |
| `Begin(ctx)`       | Start a transaction                                    |
| `Rollback()`       | Abort and cleanup (safe to call multiple times)        |
| `Commit(ctx)`      | Commit transaction, record events (dev), auto-publish  |
| `Queries()`        | Get sqlc queries bound to transaction                  |
| `TX()`             | Get underlying transaction (for custom operations)     |
| `Events()`         | Get collected events (for manual publishing if needed) |
| `Reset()`          | Clear events for context reuse                         |

### Publisher (`publish.go`)

Interface for routing events to the appropriate streamers:

```go
type Publisher interface {
    PublishAll(events []Event)
}
```

**Implementation Example:**

```go
type filePublisher struct {
    stream eventstream.SyncStreamer[FileTopic, FileEvent]
}

func (p *filePublisher) PublishAll(events []mutation.Event) {
    for _, evt := range events {
        if evt.Op != mutation.OpDelete {
            continue
        }
        switch evt.Entity {
        case mutation.EntityFile:
            p.stream.Publish(FileTopic{WorkspaceID: evt.WorkspaceID}, FileEvent{
                Type: eventTypeDelete,
                File: &apiv1.File{
                    FileId:      evt.ID.Bytes(),
                    WorkspaceID: evt.WorkspaceID.Bytes(),
                },
            })
        case mutation.EntityHTTP:
            // Route to HTTP streamer
        case mutation.EntityFlow:
            // Route to Flow streamer
        }
    }
}
```

## Cascade Relationships

The mutation system automatically collects events for all child entities before the parent is deleted.

### File Cascade

```
File
├── HTTP (content_type = HTTP)
│   └── [HTTP cascade - see below]
├── HTTP Delta (content_type = HTTP_DELTA)
│   └── [HTTP cascade - see below]
└── Flow (content_type = FLOW)
    └── [Flow cascade - see below]
```

**File types:**

- `ContentTypeHTTP` - Cascades to HTTP deletion
- `ContentTypeHTTPDelta` - Cascades to HTTP (delta) deletion
- `ContentTypeFlow` - Cascades to Flow deletion
- `ContentTypeFolder` - No content cascade, just file record

### HTTP Cascade

```
HTTP
├── HTTPHeader[]         (idx: http_header_http_idx)
├── HTTPSearchParam[]    (idx: http_search_param_http_idx)
├── HTTPBodyForm[]       (idx: http_body_form_http_idx)
├── HTTPBodyUrlEncoded[] (idx: http_body_urlencoded_http_idx)
├── HTTPBodyRaw          (idx: http_body_raw_http_idx)
└── HTTPAssert[]         (idx: http_assert_http_idx)
```

**Query count:** 6 queries for single delete, 6 queries for batch delete (IN clause)

### Flow Cascade

```
Flow
├── FlowNode[]     (idx: flow_node_idx1)
├── FlowEdge[]     (idx: flow_edge_idx1)
└── FlowVariable[] (idx: flow_variable_ordering)
```

**Query count:** 3 queries for single delete, 3 queries for batch delete (IN clause)

### Workspace Cascade (Deep)

```
Workspace
├── HTTP[]       -> [HTTP cascade for each]
├── Flow[]       -> [Flow cascade for each]
├── File[]
├── Environment[]
├── Tag[]
└── WorkspaceUser[]
```

**Note:** Workspace deletion is a deep cascade - it collects events for all HTTP children, Flow children, etc.

## Usage in RPC Handlers

The mutation system integrates with the FETCH-CHECK-ACT pattern described in `BACKEND_ARCHITECTURE_V2.md`.

### Basic Pattern

```go
func (f *FileServiceRPC) FileDelete(ctx context.Context, req *connect.Request[apiv1.FileDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
    // =========================================================
    // 1. FETCH (Outside transaction - uses Reader/DB pool)
    // =========================================================
    deleteItems := make([]mutation.FileDeleteItem, 0, len(req.Msg.Items))
    for _, fileDelete := range req.Msg.Items {
        fileID, err := idwrap.NewFromBytes(fileDelete.FileId)
        if err != nil {
            return nil, connect.NewError(connect.CodeInvalidArgument, err)
        }

        existingFile, err := f.fs.GetFile(ctx, fileID)
        if err != nil {
            if errors.Is(err, sfile.ErrFileNotFound) {
                return nil, connect.NewError(connect.CodeNotFound, err)
            }
            return nil, connect.NewError(connect.CodeInternal, err)
        }

        // =========================================================
        // 2. CHECK (In-memory validation)
        // =========================================================
        rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, f.us, existingFile.WorkspaceID))
        if rpcErr != nil {
            return nil, rpcErr
        }

        deleteItems = append(deleteItems, mutation.FileDeleteItem{
            ID:          existingFile.ID,
            WorkspaceID: existingFile.WorkspaceID,
            ContentID:   existingFile.ContentID,
            ContentKind: existingFile.ContentType,
        })
    }

    // =========================================================
    // 3. ACT (Transaction with auto-publish)
    // =========================================================
    mut := mutation.New(f.DB, mutation.WithPublisher(&filePublisher{stream: f.stream}))
    if err := mut.Begin(ctx); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }
    defer mut.Rollback() // Safe cleanup if Commit not reached

    if err := mut.DeleteFileBatch(ctx, deleteItems); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    return connect.NewResponse(&emptypb.Empty{}), nil
}
```

### Key Points

1. **Defer Rollback** - Always defer `mut.Rollback()` immediately after `Begin()`. It's safe to call even after successful commit.

2. **Build delete items outside transaction** - FETCH phase gathers data and validates using Readers (non-blocking).

3. **Batch operations when possible** - Use `DeleteFileBatch`, `DeleteHTTPBatch`, `DeleteFlowBatch` for better performance.

4. **Commit auto-publishes** - If a Publisher is configured, events are published automatically on successful commit.

## Auto-Publishing Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Begin()   │ --> │   Delete*   │ --> │  Commit()   │
│ Start TX    │     │ Track events│     │ Commit TX   │
└─────────────┘     └─────────────┘     │ Record JSONL│
                                        │ PublishAll()│
                                        └─────────────┘
                                              │
                                              ▼
                                   ┌─────────────────────┐
                                   │    Publisher        │
                                   │ PublishAll(events)  │
                                   │   for evt := range  │
                                   │     route by Entity │
                                   │     stream.Publish  │
                                   └─────────────────────┘
```

**Commit sequence:**

1. Record events to JSONL file (dev builds only, noop in prod)
2. Execute `tx.Commit()`
3. If publisher configured, call `publisher.PublishAll(events)`
4. Return success

**Note:** Events are published AFTER successful commit to ensure data consistency. If commit fails, no events are published.

## Replay System

The replay system records all mutation events to JSONL files for debugging and replay testing. It is controlled by Go build tags.

### Build Tags

| Build                | Recorder                  | Overhead         |
| -------------------- | ------------------------- | ---------------- |
| `go build -tags=dev` | File-based JSONL recorder | ~1-5ms per batch |
| `go build` (prod)    | nil (noop)                | Zero             |

### Dev Recorder (`replay_dev.go`)

```go
//go:build dev

type fileRecorder struct {
    dir  string
    mu   sync.Mutex
    file *os.File
    day  string
}

func newRecorder() Recorder {
    dir := os.Getenv("DEVTOOLS_REPLAY_DIR")
    if dir == "" {
        dir = filepath.Join(os.TempDir(), "devtools-replay")
    }
    _ = os.MkdirAll(dir, 0755)
    return &fileRecorder{dir: dir}
}
```

**Features:**

- Writes to `$DEVTOOLS_REPLAY_DIR/<date>.jsonl` (or temp dir if not set)
- Daily file rotation
- Thread-safe with mutex
- Append-only for crash safety

### Prod Recorder (`replay_prod.go`)

```go
//go:build !dev

func newRecorder() Recorder {
    return nil  // Zero allocation, zero overhead
}
```

### JSONL Format

Each line is a JSON object containing a batch of events:

```json
{
  "ts": 1704067200000,
  "e": [
    { "t": 23, "op": 2, "id": "01HQXYZ...", "ws": "01HQABC..." },
    { "t": 5, "op": 2, "id": "01HQDEF...", "ws": "01HQABC...", "d": true }
  ]
}
```

**Fields:**

| Field    | Type   | Description                              |
| -------- | ------ | ---------------------------------------- |
| `ts`     | int64  | Unix milliseconds timestamp              |
| `e`      | array  | Array of events                          |
| `e[].t`  | uint16 | EntityType numeric value                 |
| `e[].op` | uint8  | Operation (0=Insert, 1=Update, 2=Delete) |
| `e[].id` | string | Entity ID (ULID)                         |
| `e[].ws` | string | Workspace ID (ULID)                      |
| `e[].d`  | bool   | IsDelta flag (omitted if false)          |

**Example daily file:** `/tmp/devtools-replay/2024-01-01.jsonl`

## Performance

### Query Efficiency

The mutation system is designed for minimal query overhead:

| Operation           | Queries | Strategy                        |
| ------------------- | ------- | ------------------------------- |
| DeleteHTTP (single) | 7       | 6 child queries + 1 delete      |
| DeleteHTTPBatch (N) | 7       | 6 IN-clause queries + N deletes |
| DeleteFlow (single) | 4       | 3 child queries + 1 delete      |
| DeleteFlowBatch (N) | 4       | 3 IN-clause queries + N deletes |
| DeleteFile (single) | Varies  | 1 + content cascade             |
| DeleteWorkspace     | Many    | Full workspace cascade          |

**Key optimization:** Batch operations use `IN` clause queries, keeping query count constant regardless of item count.

### Database Indexes Used

All child collection queries use dedicated indexes:

**HTTP Children:**

- `http_header_http_idx` - Headers by HTTP ID
- `http_search_param_http_idx` - Params by HTTP ID
- `http_body_form_http_idx` - Body forms by HTTP ID
- `http_body_urlencoded_http_idx` - URL encoded by HTTP ID
- `http_body_raw_http_idx` - Raw body by HTTP ID
- `http_assert_http_idx` - Asserts by HTTP ID

**Flow Children:**

- `flow_node_idx1` - Nodes by Flow ID
- `flow_edge_idx1` - Edges by Flow ID
- `flow_variable_ordering` - Variables by Flow ID

### Memory Allocation

- Events slice pre-allocated with capacity 64: `make([]Event, 0, 64)`
- Batch maps pre-allocated with known capacity: `make(map[...]..., len(items))`
- ID slices pre-allocated: `make([]idwrap.IDWrap, len(items))`

## Adding New Entities

To extend the mutation system for a new entity type:

### 1. Add EntityType Constant

In `event.go`, add the new entity type:

```go
const (
    // ... existing types ...

    // New entity group
    EntityMyNewEntity
    EntityMyNewEntityChild
)
```

### 2. Create Delete Item Struct

In a new file `delete_mynewentity.go`:

```go
type MyNewEntityDeleteItem struct {
    ID          idwrap.IDWrap
    WorkspaceID idwrap.IDWrap
    // Add any fields needed for cascade collection
}
```

### 3. Implement Delete Method

```go
func (c *Context) DeleteMyNewEntity(ctx context.Context, id, workspaceID idwrap.IDWrap) error {
    // 1. Collect children before delete
    c.collectMyNewEntityChildren(ctx, id, workspaceID)

    // 2. Track parent delete
    c.track(Event{
        Entity:      EntityMyNewEntity,
        Op:          OpDelete,
        ID:          id,
        WorkspaceID: workspaceID,
    })

    // 3. Delete - DB CASCADE handles children
    return c.q.DeleteMyNewEntity(ctx, id)
}

func (c *Context) collectMyNewEntityChildren(ctx context.Context, parentID, workspaceID idwrap.IDWrap) {
    // Query children using indexed lookups
    if children, err := c.q.GetMyNewEntityChildren(ctx, parentID); err == nil {
        for i := range children {
            c.track(Event{
                Entity:      EntityMyNewEntityChild,
                Op:          OpDelete,
                ID:          children[i].ID,
                WorkspaceID: workspaceID,
            })
        }
    }
}
```

### 4. Implement Batch Delete (Optional but Recommended)

```go
func (c *Context) DeleteMyNewEntityBatch(ctx context.Context, items []MyNewEntityDeleteItem) error {
    if len(items) == 0 {
        return nil
    }

    // Build ID list and lookup map
    ids := make([]idwrap.IDWrap, len(items))
    itemMap := make(map[idwrap.IDWrap]MyNewEntityDeleteItem, len(items))
    for i, item := range items {
        ids[i] = item.ID
        itemMap[item.ID] = item
    }

    // Batch collect children (single IN-clause query)
    c.collectMyNewEntityChildrenBatch(ctx, ids, itemMap)

    // Track parent deletes
    for _, item := range items {
        c.track(Event{
            Entity:      EntityMyNewEntity,
            Op:          OpDelete,
            ID:          item.ID,
            WorkspaceID: item.WorkspaceID,
        })
    }

    // Delete all
    for _, item := range items {
        if err := c.q.DeleteMyNewEntity(ctx, item.ID); err != nil {
            return err
        }
    }

    return nil
}
```

### 5. Add to Publisher

Update your Publisher implementation to route the new entity type:

```go
func (p *myPublisher) PublishAll(events []mutation.Event) {
    for _, evt := range events {
        switch evt.Entity {
        case mutation.EntityMyNewEntity:
            p.myStream.Publish(MyTopic{WorkspaceID: evt.WorkspaceID}, MyEvent{...})
        case mutation.EntityMyNewEntityChild:
            // Route to appropriate streamer
        }
    }
}
```

### 6. Add Index for Child Queries

Ensure sqlc queries for children have proper indexes:

```sql
CREATE INDEX my_new_entity_child_parent_idx ON my_new_entity_child(parent_id);
```

## Related Documentation

- [BACKEND_ARCHITECTURE_V2.md](./BACKEND_ARCHITECTURE_V2.md) - Reader/Writer service pattern, Fetch-Check-Act
- [SYNC.md](./SYNC.md) - Real-time sync and TanStack DB integration
- [BULK_SYNC_TRANSACTION_WRAPPERS.md](./BULK_SYNC_TRANSACTION_WRAPPERS.md) - Bulk operation patterns
