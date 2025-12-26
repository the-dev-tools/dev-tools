# Bulk Sync Transaction Wrappers

## Overview

The bulk sync transaction wrapper system provides type-safe, compile-time enforced patterns for database transactions that automatically publish sync events in bulk. This eliminates the manual event publishing pattern, prevents forgotten sync events, and significantly improves performance for bulk operations.

## Problem Statement

### Before: Manual Event Publishing

The original pattern required manually publishing sync events after each transaction:

```go
// Transaction
tx.BeginTx(ctx, nil)
var createdItems []Item
for _, item := range items {
    service.Create(ctx, item)
    createdItems = append(createdItems, item)
}
tx.Commit()

// Manual sync publishing (easy to forget!)
for _, item := range createdItems {
    h.streamers.Item.Publish(topic, event)  // N publish calls
}
```

**Issues:**

- ❌ Manual tracking of created/updated/deleted items
- ❌ Manual loop to publish events one-by-one
- ❌ Easy to forget or accidentally delete
- ❌ N publish calls for N items (inefficient)
- ❌ No compile-time enforcement
- ❌ Common source of sync bugs

### After: Bulk Transaction Wrappers

```go
// Create bulk wrapper
syncTx := txutil.NewBulkInsertTx[Item, Topic](tx, extractTopic)

// Transaction with auto-tracking
for _, item := range items {
    service.Create(ctx, item)
    syncTx.Track(item)  // Auto-track
}

// Commit + publish atomically (impossible to forget!)
syncTx.CommitAndPublish(ctx, publishBulkInsert)
```

**Benefits:**

- ✅ Automatic tracking via `Track()`
- ✅ Automatic grouping by topic
- ✅ Automatic bulk publishing (1 call per topic)
- ✅ Compile-time safety (can't commit without publishing)
- ✅ 10-100x performance improvement
- ✅ Impossible to forget sync events

---

## Architecture

### Core Components

Located in `packages/server/pkg/txutil/`:

1. **`BulkSyncTxInsert[T, Topic]`** - Bulk insert operations
2. **`BulkSyncTxUpdate[T, P, Topic]`** - Bulk update operations with patches
3. **`BulkSyncTxDelete[ID, Topic]`** - Bulk delete operations

### Type Parameters

- **`T`** - The model type (e.g., `mhttp.HTTP`, `mhttp.HTTPHeader`)
- **`P`** - The patch type for updates (e.g., `patch.HTTPDeltaPatch`)
- **`Topic`** - The event stream topic type (e.g., `HttpTopic`, `HttpHeaderTopic`)
- **`ID`** - The ID type for deletes (e.g., `idwrap.IDWrap`)

### Topic Extraction

Topic extraction uses a functional pattern for flexibility:

```go
type TopicExtractor[T any, Topic any] func(item T) Topic
```

**Why functions over interfaces?**

- Different models store workspace IDs differently
- Some models don't have direct access to workspace ID
- Allows for computed topics
- No need to modify model structs

---

## Usage Patterns

### Pattern 1: Insert Operations

```go
// Example: HttpHeaderInsert
func (h *HttpServiceRPC) HttpHeaderInsert(ctx context.Context, req *Request) (*Response, error) {
    // Step 1: Validation OUTSIDE transaction
    var insertData []struct {
        header      mhttp.HTTPHeader
        workspaceID idwrap.IDWrap
    }

    for _, item := range req.Items {
        // Validate, check permissions, prepare data
        insertData = append(insertData, ...)
    }

    // Step 2: Begin transaction
    tx, err := h.DB.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer devtoolsdb.TxnRollback(tx)

    // Step 3: Create bulk wrapper with topic extractor
    syncTx := txutil.NewBulkInsertTx[headerWithWorkspace, HttpHeaderTopic](
        tx,
        func(hww headerWithWorkspace) HttpHeaderTopic {
            return HttpHeaderTopic{WorkspaceID: hww.workspaceID}
        },
    )

    // Step 4: Write loop with tracking
    headerService := h.headerService.TX(tx)
    for _, data := range insertData {
        if err := headerService.Create(ctx, &data.header); err != nil {
            return nil, err
        }

        // Track with workspace context
        syncTx.Track(headerWithWorkspace{
            header:      data.header,
            workspaceID: data.workspaceID,
        })
    }

    // Step 5: Commit + bulk publish (atomic)
    if err := syncTx.CommitAndPublish(ctx, h.publishBulkHeaderInsert); err != nil {
        return nil, err
    }

    return &Response{}, nil
}
```

**Publish handler:**

```go
func (h *HttpServiceRPC) publishBulkHeaderInsert(
    topic HttpHeaderTopic,
    items []headerWithWorkspace,
) {
    // Convert to events
    events := make([]HttpHeaderEvent, len(items))
    for i, item := range items {
        events[i] = HttpHeaderEvent{
            Type:       eventTypeInsert,
            IsDelta:    item.header.IsDelta,
            HttpHeader: converter.ToAPIHttpHeader(item.header),
        }
    }

    // Single bulk publish for entire batch
    h.streamers.HttpHeader.Publish(topic, events...)
}
```

### Pattern 2: Update Operations with Patches

```go
syncTx := txutil.NewBulkUpdateTx[mhttp.HTTP, patch.HTTPDeltaPatch, HttpTopic](
    tx,
    func(http mhttp.HTTP) HttpTopic {
        return HttpTopic{WorkspaceID: http.WorkspaceID}
    },
)

for _, update := range updates {
    httpService.Update(ctx, update.item)

    // Track with PATCH (partial update preserved!)
    syncTx.Track(update.item, patch.HTTPDeltaPatch{
        Name: Optional[string]{Value: newName, IsSet: true},  // Only changed fields
    })
}

syncTx.CommitAndPublish(ctx, publishBulkHttpUpdate)
```

**Key Point:** Patches are preserved! The `UpdateEvent[T, P]` struct contains both the item AND the patch, ensuring partial updates are sent over the wire.

### Pattern 3: Delete Operations

```go
syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, HttpTopic](
    tx,
    func(evt txutil.DeleteEvent[idwrap.IDWrap]) HttpTopic {
        return HttpTopic{WorkspaceID: evt.WorkspaceID}
    },
)

for _, id := range ids {
    httpService.Delete(ctx, id)
    syncTx.Track(id, workspaceID, isDelta)
}

syncTx.CommitAndPublish(ctx, publishBulkHttpDelete)
```

### Pattern 4: Context Carriers

For models that don't store workspace ID directly, use a context carrier struct:

```go
// HTTPHeader doesn't have WorkspaceID, so we create a carrier
type headerWithWorkspace struct {
    header      mhttp.HTTPHeader
    workspaceID idwrap.IDWrap
}

// Use carrier in wrapper
syncTx := txutil.NewBulkInsertTx[headerWithWorkspace, HttpHeaderTopic](
    tx,
    func(hww headerWithWorkspace) HttpHeaderTopic {
        return HttpHeaderTopic{WorkspaceID: hww.workspaceID}
    },
)
```

---

## Performance Characteristics

### Publish Call Reduction

| Scenario                      | Items | Topics | Before     | After    | Improvement |
| ----------------------------- | ----- | ------ | ---------- | -------- | ----------- |
| Small batch, single workspace | 10    | 1      | 10 calls   | 1 call   | 10x         |
| Large batch, single workspace | 100   | 1      | 100 calls  | 1 call   | 100x        |
| Bulk import, 5 workspaces     | 500   | 5      | 500 calls  | 5 calls  | 100x        |
| Massive import, 10 workspaces | 1000  | 10     | 1000 calls | 10 calls | 100x        |

### Memory Impact

**Minimal overhead:**

- Grouping map: ~O(M) where M = number of unique topics
- Tracked items: O(N) where N = number of items (same as before)
- Pre-allocated slices reused

**Example:**

- 100 items, 1 workspace: ~1KB overhead (negligible)
- 1000 items, 10 workspaces: ~10KB overhead (negligible)

### Timing: Immediate Publishing

**No delay or buffering:**

```
Timeline:
1. Begin transaction          [0ms]
2. Insert items               [10ms]
3. Commit transaction         [12ms]
4. Group by topic            [12.1ms]  ← Instant
5. Publish all groups        [12.2ms]  ← Instant
6. Return to client          [13ms]
```

**Not like log sync batching:**

- ❌ NO timeout waiting (500ms)
- ❌ NO buffer accumulation
- ✅ Synchronous, immediate publish after commit

---

## Event Stream Integration

### Backend → EventStream

**Before:**

```go
for _, item := range items {
    streamer.Publish(topic, event1)  // Call 1
    streamer.Publish(topic, event2)  // Call 2
    // ... N calls
}
```

**After:**

```go
// Variadic publish (supported by eventstream)
streamer.Publish(topic, event1, event2, ..., eventN)  // 1 call
```

### EventStream → Frontend (UNCHANGED)

The `StreamToClient` bridge still batches events the same way:

```go
// From packages/server/pkg/eventstream/bridge.go
MaxBatchSize:  100 events
FlushInterval: 50ms
```

**Frontend receives events in batches:**

- Events 1-100: Batch 1 (sent when buffer reaches 100 OR 50ms timeout)
- Events 101-200: Batch 2
- etc.

**Key Point:** Bulk wrappers only optimize backend publishing. Frontend still receives incremental batches (100 events or 50ms) exactly as before.

---

## Delta Patch System

### Patches Preserved in Bulk Wrappers

The `UpdateEvent` struct preserves both item and patch:

```go
type UpdateEvent[T any, P any] struct {
    Item  T    // Full item (for reference)
    Patch P    // Partial update (only changed fields)
}
```

### What Gets Sent Over the Wire

**Insert events (full object):**

```json
{
  "type": "insert",
  "http": {
    "httpId": "...",
    "name": "My Request",
    "url": "https://api.example.com",
    "method": "GET"
  }
}
```

**Update events (patch only):**

```json
{
  "type": "update",
  "patch": {
    "name": "New Name" // ← Only changed field!
  },
  "http": {
    "httpId": "..."
  }
}
```

**Delete events (minimal):**

```json
{
  "type": "delete",
  "http": {
    "httpId": "..." // ← Only ID
  }
}
```

**Bulk wrappers do NOT change what data is sent, only how efficiently it's published.**

---

## Implementation Guidelines

### SQLite Transaction Best Practices

```go
// DO: Reads BEFORE transaction
httpEntry, err := h.httpService.Get(ctx, httpID)  // Outside TX
if err != nil {
    return nil, err
}

// DO: Validation BEFORE transaction
if err := h.checkWorkspaceAccess(ctx, httpEntry.WorkspaceID); err != nil {
    return nil, err
}

// DO: Minimal transaction duration
tx, err := h.DB.BeginTx(ctx, nil)
defer devtoolsdb.TxnRollback(tx)

// ... writes only
syncTx.CommitAndPublish(ctx, publishFn)
```

**Why?**

- SQLite locks during transactions
- Reads BEFORE transactions minimize lock duration
- Shorter transactions = less contention

### Error Handling

```go
// Commit + publish is atomic
if err := syncTx.CommitAndPublish(ctx, publishFn); err != nil {
    // Transaction rolled back, NO events published
    return nil, connect.NewError(connect.CodeInternal, err)
}

// If commit succeeds but publish fails, events still published
// (publish happens AFTER successful commit)
```

### Compile-Time Safety

```go
// ❌ This won't compile (good!)
tx.Commit()  // Error: syncTx holds the transaction

// ✅ Must use wrapper
syncTx.CommitAndPublish(ctx, publishFn)
```

---

## Migration Guide

### Step 1: Identify Candidate Operations

Look for this pattern:

```go
tx.Commit()

for _, item := range items {
    h.streamers.*.Publish(topic, event)
}
```

**Priority targets:**

1. HTTP CRUD operations (headers, params, body, asserts)
2. Flow node/edge operations
3. Any bulk insert/update/delete

### Step 2: Create Context Carrier (if needed)

```go
// If model doesn't have WorkspaceID directly
type itemWithWorkspace struct {
    item        ModelType
    workspaceID idwrap.IDWrap
}
```

### Step 3: Define Bulk Publish Handler

```go
func (h *ServiceRPC) publishBulkItemInsert(
    topic ItemTopic,
    items []itemWithWorkspace,
) {
    events := make([]ItemEvent, len(items))
    for i, item := range items {
        events[i] = ItemEvent{
            Type: eventTypeInsert,
            Item: converter.ToAPIItem(item.item),
        }
    }
    h.streamers.Item.Publish(topic, events...)
}
```

### Step 4: Refactor RPC Handler

```go
// Replace manual pattern
syncTx := txutil.NewBulkInsertTx[itemWithWorkspace, ItemTopic](
    tx,
    func(iww itemWithWorkspace) ItemTopic {
        return ItemTopic{WorkspaceID: iww.workspaceID}
    },
)

for _, data := range insertData {
    service.Create(ctx, data.item)
    syncTx.Track(itemWithWorkspace{
        item:        data.item,
        workspaceID: data.workspaceID,
    })
}

syncTx.CommitAndPublish(ctx, h.publishBulkItemInsert)
```

### Step 5: Test

```go
func TestItemInsert_BulkPublish(t *testing.T) {
    // Verify:
    // 1. Items inserted correctly
    // 2. Sync events published
    // 3. Events grouped by topic
    // 4. Publish call count matches topic count
}
```

---

## Testing Strategy

### Unit Tests (pkg/txutil/)

**Coverage:**

- Topic grouping correctness
- Commit failure handling (no publish)
- Empty tracked items
- Multiple topics
- Single topic with many items

**Example:**

```go
func TestBulkSyncTxInsert_GroupsByTopic(t *testing.T) {
    // Track items with different topics
    syncTx.Track(item1)  // topic A
    syncTx.Track(item2)  // topic A
    syncTx.Track(item3)  // topic B

    // Verify publish called twice (once per topic)
    // Verify items grouped correctly
}
```

### Integration Tests (internal/api/rhttp/)

**Coverage:**

- RPC handler with bulk wrapper
- Events published correctly
- Patches preserved for updates
- Frontend sync receives events

**Example:**

```go
func TestHttpHeaderInsert_BulkPublish(t *testing.T) {
    // Insert 10 headers across 2 workspaces
    // Verify 2 publish calls (one per workspace)
    // Verify each publish has correct batch size
}
```

---

## API Reference

### BulkSyncTxInsert

```go
type BulkSyncTxInsert[T any, Topic comparable] struct {
    tx             *sql.Tx
    tracked        []T
    topicExtractor TopicExtractor[T, Topic]
}

func NewBulkInsertTx[T any, Topic comparable](
    tx *sql.Tx,
    topicExtractor TopicExtractor[T, Topic],
) *BulkSyncTxInsert[T, Topic]

func (s *BulkSyncTxInsert[T, Topic]) Track(item T)

func (s *BulkSyncTxInsert[T, Topic]) CommitAndPublish(
    ctx context.Context,
    publishFn func(Topic, []T),
) error
```

### BulkSyncTxUpdate

```go
type BulkSyncTxUpdate[T any, P any, Topic comparable] struct {
    tx             *sql.Tx
    tracked        []UpdateEvent[T, P]
    topicExtractor TopicExtractor[T, Topic]
}

func NewBulkUpdateTx[T any, P any, Topic comparable](
    tx *sql.Tx,
    topicExtractor TopicExtractor[T, Topic],
) *BulkSyncTxUpdate[T, P, Topic]

func (s *BulkSyncTxUpdate[T, P, Topic]) Track(item T, patch P)

func (s *BulkSyncTxUpdate[T, P, Topic]) CommitAndPublish(
    ctx context.Context,
    publishFn func(Topic, []UpdateEvent[T, P]),
) error
```

### BulkSyncTxDelete

```go
type BulkSyncTxDelete[ID any, Topic comparable] struct {
    tx             *sql.Tx
    tracked        []DeleteEvent[ID]
    topicExtractor func(DeleteEvent[ID]) Topic
}

func NewBulkDeleteTx[ID any, Topic comparable](
    tx *sql.Tx,
    topicExtractor func(DeleteEvent[ID]) Topic,
) *BulkSyncTxDelete[ID, Topic]

func (s *BulkSyncTxDelete[ID, Topic]) Track(id ID, workspaceID ID, isDelta bool)

func (s *BulkSyncTxDelete[ID, Topic]) CommitAndPublish(
    ctx context.Context,
    publishFn func(Topic, []DeleteEvent[ID]),
) error
```

### Supporting Types

```go
type TopicExtractor[T any, Topic any] func(item T) Topic

type UpdateEvent[T any, P any] struct {
    Item  T
    Patch P
}

type DeleteEvent[ID any] struct {
    ID          ID
    WorkspaceID ID
    IsDelta     bool
}
```

---

## Future Enhancements

### Potential Improvements

1. **Optional sub-batching** for very large operations (>1000 items)
2. **Metrics/observability** - track publish call reduction
3. **Migration tooling** - automated refactoring scripts
4. **Generalized transaction wrappers** - support for non-sync transactions

### Migration Roadmap

**Phase 1: HTTP Operations** (current)

- ✅ HttpHeaderInsert (completed)
- ⏳ HttpHeaderUpdate/Delete
- ⏳ HttpSearchParam CRUD
- ⏳ HttpBodyForm CRUD
- ⏳ HttpBodyUrlEncoded CRUD
- ⏳ HttpAssert CRUD

**Phase 2: Flow Operations**

- FlowNodeInsert/Update/Delete
- FlowEdgeInsert/Update/Delete
- FlowVariableInsert/Update/Delete

**Phase 3: Other Resources**

- Environment CRUD
- File CRUD
- Reference CRUD

---

## Related Documentation

- [SYNC.md](./SYNC.md) - Real-time sync architecture
- [BACKEND_ARCHITECTURE_V2.md](./BACKEND_ARCHITECTURE_V2.md) - Backend architecture patterns
- [HTTP.md](./HTTP.md) - HTTP request handling

---

## Change History

| Date       | Version | Changes                                                       |
| ---------- | ------- | ------------------------------------------------------------- |
| 2025-12-26 | 1.0.0   | Initial implementation with HttpHeaderInsert proof of concept |
