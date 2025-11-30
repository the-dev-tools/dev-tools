# Application-Level Cascade Delete System

This document details the design for implementing **application-level cascading deletes** that work independently of SQLite's `ON DELETE CASCADE` constraints, enabling the frontend to remain "dumb" while receiving explicit delete events for all affected entities.

## 1. Problem Statement

### Current State

```ascii
                        [ DELETE HTTP Request ]
                                  |
                                  v
                         +----------------+
                         |   RPC Layer    |
                         |  HttpDelete()  |
                         +----------------+
                                  |
                          (Single DELETE)
                                  |
                                  v
                         +----------------+
                         |    SQLite      |
                         |   CASCADE      |
                         +----------------+
                                  |
              +-------------------+-------------------+
              |         |         |         |        |
              v         v         v         v        v
          [Headers] [Params] [Body]  [Asserts] [Responses]
              |         |         |         |        |
          (DELETED SILENTLY - NO EVENTS PUBLISHED)
                                  |
                                  v
                         +----------------+
                         |  EventStream   |
                         +----------------+
                                  |
                     (Only parent DELETE event)
                                  |
                                  v
                         +----------------+
                         |   Frontend     |
                         |  TanStack DB   |
                         +----------------+
                                  |
                   (Child entities ORPHANED in cache!)
```

### The Gap

| Layer | What Happens | Problem |
|-------|--------------|---------|
| Backend RPC | Deletes parent HTTP record | Only knows about parent |
| SQLite CASCADE | Silently deletes all children | Backend unaware of what was deleted |
| EventStream | Publishes single `delete` event | Only parent ID transmitted |
| Frontend Cache | Removes parent from TanStack DB | **Children remain in separate collections** |

### Why This Matters

The frontend maintains **separate collections** for each entity type:

```typescript
// Each entity type has its own TanStack DB collection
const httpCollection = createApiCollection(HttpSchema, transport);
const headerCollection = createApiCollection(HttpHeaderSchema, transport);
const paramCollection = createApiCollection(HttpSearchParamSchema, transport);
const assertCollection = createApiCollection(HttpAssertSchema, transport);
// ... etc
```

When a parent HTTP is deleted, SQLite cascades the delete to children, but:
1. Backend only publishes `HttpEvent{Type: "delete", ID: parentID}`
2. Frontend removes parent from `httpCollection`
3. **Children remain in `headerCollection`, `paramCollection`, etc.**
4. Frontend must either:
   - Implement cascade logic (not "dumb")
   - Tolerate stale/orphaned data
   - Re-sync entire collection (expensive)

---

## 2. Solution: Explicit Application-Level Cascades

### Design Principle

> **The backend explicitly queries and publishes DELETE events for ALL entities affected by a cascade, BEFORE the actual delete.**

This allows the frontend to remain completely "dumb" - it just applies `insert`, `update`, `delete` events as they arrive.

### Architecture Overview

```ascii
                        [ DELETE HTTP Request ]
                                  |
                                  v
                    +---------------------------+
                    |       RPC Layer           |
                    |      HttpDelete()         |
                    +---------------------------+
                                  |
                    1. Query cascade graph
                                  |
                                  v
                    +---------------------------+
                    |    Cascade Registry       |
                    | (Defines parent→children) |
                    +---------------------------+
                                  |
                    2. Collect all child IDs
                                  |
                                  v
           +------------------------------------------+
           |        Pre-Delete Query Phase            |
           |                                          |
           |  headers := ListByHttpID(ctx, httpID)    |
           |  params := ListByHttpID(ctx, httpID)     |
           |  bodyForms := ListByHttpID(ctx, httpID)  |
           |  asserts := ListByHttpID(ctx, httpID)    |
           |  responses := ListByHttpID(ctx, httpID)  |
           +------------------------------------------+
                                  |
                    3. Execute delete (CASCADE still works)
                                  |
                                  v
                    +---------------------------+
                    |     SQLite DELETE         |
                    |   (CASCADE as safety)     |
                    +---------------------------+
                                  |
                    4. Publish events for ALL entities
                                  |
                                  v
           +------------------------------------------+
           |         Post-Delete Publish Phase        |
           |                                          |
           |  for _, h := range headers {             |
           |    headerStream.Publish(DeleteEvent)     |
           |  }                                       |
           |  for _, p := range params {              |
           |    paramStream.Publish(DeleteEvent)      |
           |  }                                       |
           |  // ... all children ...                 |
           |  httpStream.Publish(DeleteEvent)         |
           +------------------------------------------+
                                  |
                                  v
                    +---------------------------+
                    |      Frontend             |
                    |    (Stays "dumb")         |
                    +---------------------------+
                                  |
            Receives explicit DELETE for each entity
                                  |
                    +---------------------------+
                    | headerCollection.delete() |
                    | paramCollection.delete()  |
                    | httpCollection.delete()   |
                    +---------------------------+
```

---

## 3. Cascade Graph Definition

### Entity Relationships

The following diagram shows all cascading relationships in the system:

```ascii
                              +-------------+
                              | Workspaces  |
                              +-------------+
                                    |
                    +---------------+---------------+
                    |               |               |
                    v               v               v
              +----------+    +----------+    +----------+
              |   HTTP   |    |   Flow   |    |  Files   |
              +----------+    +----------+    +----------+
                    |               |               |
         +----+----+----+----+     |          (SET NULL)
         |    |    |    |    |     |               |
         v    v    v    v    v     |               v
      [Hdr][Prm][Bdy][Ast][Rsp]    |          [Children]
         |                   |     |
         v                   v     +----+----+----+
    [Deltas]            [RspHdr]   |    |    |    |
                        [RspAst]   v    v    v    v
                                 [Node][Edge][Var][Tag]
                                   |
                        +----+----+----+----+----+
                        |    |    |    |    |    |
                        v    v    v    v    v    v
                     [For][Each][HTTP][Cond][Noop][JS]
                                   |
                                   v
                            [NodeExecution]
```

### Cascade Registry (Go)

```go
// packages/server/pkg/cascade/registry.go

package cascade

import "the-dev-tools/server/pkg/idwrap"

// EntityType identifies a database entity
type EntityType string

const (
    EntityHTTP              EntityType = "http"
    EntityHTTPHeader        EntityType = "http_header"
    EntityHTTPSearchParam   EntityType = "http_search_param"
    EntityHTTPBodyForm      EntityType = "http_body_form"
    EntityHTTPBodyUrlEncoded EntityType = "http_body_urlencoded"
    EntityHTTPBodyRaw       EntityType = "http_body_raw"
    EntityHTTPAssert        EntityType = "http_assert"
    EntityHTTPResponse      EntityType = "http_response"
    EntityHTTPResponseHeader EntityType = "http_response_header"
    EntityHTTPResponseAssert EntityType = "http_response_assert"
    EntityFlow              EntityType = "flow"
    EntityFlowNode          EntityType = "flow_node"
    EntityFlowEdge          EntityType = "flow_edge"
    EntityFlowVariable      EntityType = "flow_variable"
    EntityFlowTag           EntityType = "flow_tag"
    EntityFile              EntityType = "file"
)

// CascadeRule defines a parent→child relationship
type CascadeRule struct {
    Parent      EntityType
    Child       EntityType
    ForeignKey  string          // Column name in child table
    OnDelete    OnDeleteAction  // CASCADE or SET_NULL
}

type OnDeleteAction int

const (
    OnDeleteCascade OnDeleteAction = iota
    OnDeleteSetNull
)

// CascadeGraph defines all cascade relationships
var CascadeGraph = []CascadeRule{
    // HTTP → Children
    {Parent: EntityHTTP, Child: EntityHTTPHeader, ForeignKey: "http_id", OnDelete: OnDeleteCascade},
    {Parent: EntityHTTP, Child: EntityHTTPSearchParam, ForeignKey: "http_id", OnDelete: OnDeleteCascade},
    {Parent: EntityHTTP, Child: EntityHTTPBodyForm, ForeignKey: "http_id", OnDelete: OnDeleteCascade},
    {Parent: EntityHTTP, Child: EntityHTTPBodyUrlEncoded, ForeignKey: "http_id", OnDelete: OnDeleteCascade},
    {Parent: EntityHTTP, Child: EntityHTTPBodyRaw, ForeignKey: "http_id", OnDelete: OnDeleteCascade},
    {Parent: EntityHTTP, Child: EntityHTTPAssert, ForeignKey: "http_id", OnDelete: OnDeleteCascade},
    {Parent: EntityHTTP, Child: EntityHTTPResponse, ForeignKey: "http_id", OnDelete: OnDeleteCascade},

    // HTTP Delta → Parent HTTP (self-referential)
    {Parent: EntityHTTP, Child: EntityHTTP, ForeignKey: "parent_http_id", OnDelete: OnDeleteCascade},

    // HTTP Response → Children
    {Parent: EntityHTTPResponse, Child: EntityHTTPResponseHeader, ForeignKey: "response_id", OnDelete: OnDeleteCascade},
    {Parent: EntityHTTPResponse, Child: EntityHTTPResponseAssert, ForeignKey: "response_id", OnDelete: OnDeleteCascade},

    // HTTP Header Delta → Parent Header (self-referential)
    {Parent: EntityHTTPHeader, Child: EntityHTTPHeader, ForeignKey: "parent_header_id", OnDelete: OnDeleteCascade},

    // Flow → Children
    {Parent: EntityFlow, Child: EntityFlowNode, ForeignKey: "flow_id", OnDelete: OnDeleteCascade},
    {Parent: EntityFlow, Child: EntityFlowEdge, ForeignKey: "flow_id", OnDelete: OnDeleteCascade},
    {Parent: EntityFlow, Child: EntityFlowVariable, ForeignKey: "flow_id", OnDelete: OnDeleteCascade},
    {Parent: EntityFlow, Child: EntityFlowTag, ForeignKey: "flow_id", OnDelete: OnDeleteCascade},

    // Flow Version → Parent Flow (self-referential)
    {Parent: EntityFlow, Child: EntityFlow, ForeignKey: "version_parent_id", OnDelete: OnDeleteCascade},

    // Flow Node → Children (node type tables)
    // Note: These are 1:1 relationships, handled by flow_node delete

    // File → Children (SET NULL - orphans, not cascades)
    {Parent: EntityFile, Child: EntityFile, ForeignKey: "parent_id", OnDelete: OnDeleteSetNull},
}

// GetCascadeChildren returns all direct children for a given entity type
func GetCascadeChildren(parent EntityType) []CascadeRule {
    var children []CascadeRule
    for _, rule := range CascadeGraph {
        if rule.Parent == parent && rule.OnDelete == OnDeleteCascade {
            children = append(children, rule)
        }
    }
    return children
}
```

---

## 4. Implementation Pattern

### Delete Handler Structure

```go
// packages/server/internal/api/rhttp/rhttp_delete.go

func (h *HttpServiceRPC) HttpDelete(
    ctx context.Context,
    req *connect.Request[apiv1.HttpDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
    httpID, err := idwrap.Parse(req.Msg.GetHttpId())
    if err != nil {
        return nil, connect.NewError(connect.CodeInvalidArgument, err)
    }

    // Step 1: Validate permissions OUTSIDE transaction
    httpRecord, err := h.hs.Get(ctx, httpID)
    if err != nil {
        return nil, connect.NewError(connect.CodeNotFound, err)
    }

    workspaceID := httpRecord.WorkspaceID
    if err := h.validateWorkspaceAccess(ctx, workspaceID); err != nil {
        return nil, err
    }

    // Step 2: Collect all cascading children BEFORE delete
    cascadeData, err := h.collectHttpCascadeData(ctx, httpID)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Step 3: Execute delete in minimal transaction
    tx, err := h.DB.BeginTx(ctx, nil)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }
    defer devtoolsdb.TxnRollback(tx)

    if err := h.hs.TX(tx).Delete(ctx, httpID); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    if err := tx.Commit(); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Step 4: Publish delete events for ALL affected entities
    h.publishHttpCascadeDeletes(workspaceID, cascadeData)

    return connect.NewResponse(&emptypb.Empty{}), nil
}

// CascadeData holds all entities that will be deleted
type HttpCascadeData struct {
    HTTP              mhttp.HTTP
    Headers           []mhttpheader.HttpHeader
    SearchParams      []mhttpsearchparam.HttpSearchParam
    BodyForms         []mhttpbodyform.HttpBodyForm
    BodyUrlEncoded    []mhttpbodyurlencoded.HttpBodyUrlEncoded
    BodyRaw           *mhttpbodyraw.HttpBodyRaw
    Asserts           []mhttpassert.HttpAssert
    Responses         []mhttpresponse.HttpResponse
    ResponseHeaders   []mhttpresponseheader.HttpResponseHeader
    ResponseAsserts   []mhttpresponseassert.HttpResponseAssert
    DeltaHTTPs        []mhttp.HTTP  // Child delta records
}

func (h *HttpServiceRPC) collectHttpCascadeData(ctx context.Context, httpID idwrap.IDWrap) (*HttpCascadeData, error) {
    data := &HttpCascadeData{}

    // Get the HTTP record itself
    http, err := h.hs.Get(ctx, httpID)
    if err != nil {
        return nil, err
    }
    data.HTTP = http

    // Collect all direct children
    data.Headers, _ = h.httpHeaderService.ListByHttpID(ctx, httpID)
    data.SearchParams, _ = h.httpSearchParamService.ListByHttpID(ctx, httpID)
    data.BodyForms, _ = h.httpBodyFormService.ListByHttpID(ctx, httpID)
    data.BodyUrlEncoded, _ = h.httpBodyUrlEncodedService.ListByHttpID(ctx, httpID)
    data.BodyRaw, _ = h.bodyService.GetByHttpID(ctx, httpID)
    data.Asserts, _ = h.httpAssertService.ListByHttpID(ctx, httpID)
    data.Responses, _ = h.httpResponseService.ListByHttpID(ctx, httpID)

    // Collect response children (nested cascade)
    for _, resp := range data.Responses {
        headers, _ := h.httpResponseHeaderService.ListByResponseID(ctx, resp.ID)
        data.ResponseHeaders = append(data.ResponseHeaders, headers...)

        asserts, _ := h.httpResponseAssertService.ListByResponseID(ctx, resp.ID)
        data.ResponseAsserts = append(data.ResponseAsserts, asserts...)
    }

    // Collect delta HTTP records (self-referential cascade)
    data.DeltaHTTPs, _ = h.hs.ListByParentID(ctx, httpID)

    // Recursively collect delta children's cascade data
    // (Headers, params, etc. belonging to deltas)
    for _, delta := range data.DeltaHTTPs {
        deltaData, err := h.collectHttpCascadeData(ctx, delta.ID)
        if err != nil {
            continue
        }
        data.Headers = append(data.Headers, deltaData.Headers...)
        data.SearchParams = append(data.SearchParams, deltaData.SearchParams...)
        // ... merge all child collections
    }

    return data, nil
}

func (h *HttpServiceRPC) publishHttpCascadeDeletes(workspaceID idwrap.IDWrap, data *HttpCascadeData) {
    topic := HttpTopic{WorkspaceID: workspaceID}

    // Publish in reverse order: children first, then parent
    // This allows frontend to process correctly if order matters

    // Response children (deepest level)
    for _, ra := range data.ResponseAsserts {
        h.httpResponseAssertStream.Publish(
            HttpResponseAssertTopic{WorkspaceID: workspaceID},
            HttpResponseAssertEvent{Type: eventTypeDelete, HttpResponseAssert: convertResponseAssert(ra)},
        )
    }
    for _, rh := range data.ResponseHeaders {
        h.httpResponseHeaderStream.Publish(
            HttpResponseHeaderTopic{WorkspaceID: workspaceID},
            HttpResponseHeaderEvent{Type: eventTypeDelete, HttpResponseHeader: convertResponseHeader(rh)},
        )
    }

    // Responses
    for _, r := range data.Responses {
        h.httpResponseStream.Publish(
            HttpResponseTopic{WorkspaceID: workspaceID},
            HttpResponseEvent{Type: eventTypeDelete, HttpResponse: convertResponse(r)},
        )
    }

    // Direct HTTP children
    for _, a := range data.Asserts {
        h.httpAssertStream.Publish(
            HttpAssertTopic{WorkspaceID: workspaceID},
            HttpAssertEvent{Type: eventTypeDelete, IsDelta: a.IsDelta, HttpAssert: convertAssert(a)},
        )
    }
    for _, bf := range data.BodyForms {
        h.httpBodyFormStream.Publish(
            HttpBodyFormTopic{WorkspaceID: workspaceID},
            HttpBodyFormEvent{Type: eventTypeDelete, IsDelta: bf.IsDelta, HttpBodyForm: convertBodyForm(bf)},
        )
    }
    // ... all other children

    // Delta HTTP records
    for _, delta := range data.DeltaHTTPs {
        h.stream.Publish(topic, HttpEvent{Type: eventTypeDelete, IsDelta: true, Http: convertHttp(delta)})
    }

    // Finally, the parent HTTP
    h.stream.Publish(topic, HttpEvent{Type: eventTypeDelete, IsDelta: data.HTTP.IsDelta, Http: convertHttp(data.HTTP)})
}
```

---

## 5. Event Order Guarantees

### Why Order Matters

When the frontend receives delete events, the order can affect correctness:

```ascii
Scenario A: Parent deleted first (WRONG ORDER)
─────────────────────────────────────────────────
Time →  [DELETE HTTP] → [DELETE Header] → [DELETE Param]
                                    ↓
                        Frontend receives "DELETE Header"
                        Tries to find parent HTTP for context
                        HTTP already deleted → Error/Orphan

Scenario B: Children deleted first (CORRECT ORDER)
─────────────────────────────────────────────────
Time →  [DELETE Header] → [DELETE Param] → [DELETE HTTP]
                                    ↓
                        Each delete is independent
                        Parent deleted last
                        Clean cascade ✓
```

### Implementation: Topological Sort

```go
// Publish events in reverse topological order (deepest children first)

func (h *HttpServiceRPC) publishHttpCascadeDeletes(workspaceID idwrap.IDWrap, data *HttpCascadeData) {
    // Level 3: Deepest children (response headers, response asserts)
    for _, rh := range data.ResponseHeaders {
        h.publishResponseHeaderDelete(workspaceID, rh)
    }
    for _, ra := range data.ResponseAsserts {
        h.publishResponseAssertDelete(workspaceID, ra)
    }

    // Level 2: Responses, HTTP children
    for _, r := range data.Responses {
        h.publishResponseDelete(workspaceID, r)
    }
    for _, header := range data.Headers {
        h.publishHeaderDelete(workspaceID, header)
    }
    for _, param := range data.SearchParams {
        h.publishSearchParamDelete(workspaceID, param)
    }
    // ... all HTTP children

    // Level 1: Delta HTTPs (children of the parent HTTP)
    for _, delta := range data.DeltaHTTPs {
        h.publishHttpDelete(workspaceID, delta, true)
    }

    // Level 0: Parent HTTP (last)
    h.publishHttpDelete(workspaceID, data.HTTP, data.HTTP.IsDelta)
}
```

---

## 6. Frontend Behavior (Unchanged)

With this design, the frontend **does not change**. It continues to:

1. Subscribe to each entity's sync stream
2. Apply `insert`, `update`, `delete` events as they arrive
3. Trust that all necessary events will be received

```typescript
// packages/client/src/api-new/collection.internal.tsx
// NO CHANGES NEEDED - already handles delete events correctly

Match.when(
    { $typeName: schema.sync.delete.typeName },
    (_) => void write({
        type: 'delete',
        value: Protobuf.createAlike(schema.item, _) as Item
    }),
),
```

The frontend remains "dumb" - it just processes the events it receives.

---

## 7. Database Schema Considerations

### Keep SQLite CASCADE as Safety Net

```sql
-- SQLite CASCADE constraints remain as defense-in-depth
-- Application cascade logic is the PRIMARY mechanism
-- SQLite CASCADE catches any edge cases we miss

FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
```

### Why Keep Both?

| Mechanism | Role | Failure Mode |
|-----------|------|--------------|
| **Application Cascade** | Primary - query and publish events | If service crashes mid-cascade, some events not published |
| **SQLite CASCADE** | Safety net - ensures data consistency | Silent deletion (no events), but DB stays consistent |

### Transaction Pattern

```go
// Application cascade: Query BEFORE transaction, publish AFTER commit
// SQLite cascade: Happens DURING transaction as safety net

// 1. Query all children (BEFORE tx)
cascadeData, _ := collectCascadeData(ctx, parentID)

// 2. Delete parent in transaction (SQLite CASCADE as backup)
tx, _ := db.BeginTx(ctx, nil)
service.TX(tx).Delete(ctx, parentID)
tx.Commit()

// 3. Publish events (AFTER commit - all or nothing)
publishCascadeDeletes(cascadeData)
```

---

## 8. Special Cases

### 8.1 Self-Referential Cascades (Deltas)

HTTP records can have delta children (overrides):

```ascii
HTTP (parent)
    ├── HTTP Delta 1
    │       ├── Header Delta 1a
    │       └── Param Delta 1a
    └── HTTP Delta 2
            ├── Header Delta 2a
            └── Param Delta 2a
```

When deleting the parent HTTP, we must:
1. Recursively collect all delta HTTPs
2. Collect their children (headers, params, etc.)
3. Publish deletes for everything

```go
func (h *HttpServiceRPC) collectHttpCascadeData(ctx context.Context, httpID idwrap.IDWrap) (*HttpCascadeData, error) {
    data := &HttpCascadeData{}

    // ... collect direct children ...

    // Recursively collect delta children
    deltas, _ := h.hs.ListByParentID(ctx, httpID)
    for _, delta := range deltas {
        deltaData, _ := h.collectHttpCascadeData(ctx, delta.ID) // Recursive!
        data.merge(deltaData)
    }

    return data, nil
}
```

### 8.2 SET NULL Cascades (Files)

Files use `ON DELETE SET NULL` for `parent_id`:

```sql
FOREIGN KEY (parent_id) REFERENCES files (id) ON DELETE SET NULL
```

This means deleting a folder **orphans** its children rather than deleting them:

```ascii
Before Delete:              After Delete:
Folder A                    (deleted)
  ├── File 1                File 1 (parent_id = NULL)
  └── File 2                File 2 (parent_id = NULL)
```

**For SET NULL relationships:**
- Don't collect children for deletion
- Optionally publish `update` events with `parent_id: null`
- Or let frontend infer orphaning when parent disappears

### 8.3 Batch Deletes

When deleting multiple items:

```go
func (h *HttpServiceRPC) HttpBatchDelete(ctx context.Context, req *connect.Request[apiv1.HttpBatchDeleteRequest]) {
    var allCascadeData []*HttpCascadeData

    // Collect cascade data for ALL items BEFORE any deletes
    for _, id := range req.Msg.GetHttpIds() {
        httpID, _ := idwrap.Parse(id)
        data, _ := h.collectHttpCascadeData(ctx, httpID)
        allCascadeData = append(allCascadeData, data)
    }

    // Delete all in single transaction
    tx, _ := h.DB.BeginTx(ctx, nil)
    for _, id := range req.Msg.GetHttpIds() {
        httpID, _ := idwrap.Parse(id)
        h.hs.TX(tx).Delete(ctx, httpID)
    }
    tx.Commit()

    // Publish all cascade deletes
    for _, data := range allCascadeData {
        h.publishHttpCascadeDeletes(workspaceID, data)
    }
}
```

---

## 9. Performance Considerations

### Query Cost

Collecting cascade data requires multiple queries:

| Entity Type | Queries per Parent |
|-------------|-------------------|
| HTTP | 1 (parent) + 7 (children) + N (responses × 2) |
| Flow | 1 (parent) + 4 (children) |
| File | 1 (parent) + 0 (SET NULL, no cascade) |

### Optimization Strategies

1. **Batch Child Queries**: Use `IN` clauses for multiple parents
2. **Parallel Queries**: Run independent child queries concurrently
3. **Lazy Loading**: For very deep hierarchies, paginate collection

```go
// Parallel collection example
func (h *HttpServiceRPC) collectHttpCascadeDataParallel(ctx context.Context, httpID idwrap.IDWrap) (*HttpCascadeData, error) {
    var wg sync.WaitGroup
    data := &HttpCascadeData{}
    var mu sync.Mutex

    // Run child queries in parallel
    wg.Add(5)
    go func() { defer wg.Done(); headers, _ := h.httpHeaderService.ListByHttpID(ctx, httpID); mu.Lock(); data.Headers = headers; mu.Unlock() }()
    go func() { defer wg.Done(); params, _ := h.httpSearchParamService.ListByHttpID(ctx, httpID); mu.Lock(); data.SearchParams = params; mu.Unlock() }()
    go func() { defer wg.Done(); forms, _ := h.httpBodyFormService.ListByHttpID(ctx, httpID); mu.Lock(); data.BodyForms = forms; mu.Unlock() }()
    go func() { defer wg.Done(); asserts, _ := h.httpAssertService.ListByHttpID(ctx, httpID); mu.Lock(); data.Asserts = asserts; mu.Unlock() }()
    go func() { defer wg.Done(); responses, _ := h.httpResponseService.ListByHttpID(ctx, httpID); mu.Lock(); data.Responses = responses; mu.Unlock() }()
    wg.Wait()

    return data, nil
}
```

### Event Publishing Cost

Publishing many events has network cost:

| Approach | Tradeoff |
|----------|----------|
| **Individual Events** | More messages, simpler frontend |
| **Batched Events** | Fewer messages, frontend must handle batches |

Recommendation: Start with individual events. Optimize to batching if performance issues arise.

---

## 10. Migration Path

### Phase 1: Add Cascade Collection (No Breaking Changes)

1. Implement `collectCascadeData()` methods for each entity
2. Call after delete but **before** existing event publish
3. Add new event publishes for children
4. Existing functionality unchanged

### Phase 2: Frontend Observability (Optional)

1. Add metrics to track orphaned entities in frontend
2. Log when frontend receives delete for parent but has cached children
3. Validate cascade completeness

### Phase 3: Optimize (If Needed)

1. Batch cascade data collection
2. Parallelize queries
3. Consider batch event format

---

## 11. Testing Strategy

### Unit Tests

```go
func TestHttpDeleteCascade(t *testing.T) {
    db := sqlitemem.NewSQLiteMem(t)
    services := createTestServices(db)

    // Create parent with children
    http := services.CreateHTTP(ctx)
    header := services.CreateHeader(ctx, http.ID)
    param := services.CreateParam(ctx, http.ID)

    // Capture events
    var events []interface{}
    mockStream := &MockStream{OnPublish: func(e interface{}) { events = append(events, e) }}

    // Delete parent
    rpc := NewHttpServiceRPC(db, services, mockStream)
    rpc.HttpDelete(ctx, &HttpDeleteRequest{HttpId: http.ID})

    // Verify cascade events
    assert.Len(t, events, 3) // header, param, http
    assert.Equal(t, eventTypeDelete, events[0].(HttpHeaderEvent).Type)
    assert.Equal(t, eventTypeDelete, events[1].(HttpSearchParamEvent).Type)
    assert.Equal(t, eventTypeDelete, events[2].(HttpEvent).Type)
}
```

### Integration Tests

```go
func TestHttpDeleteCascade_Integration(t *testing.T) {
    // Full RPC + SQLite test
    db := testutil.CreateBaseDB(t)

    // Create complex hierarchy
    http := createHttpWithChildren(t, db)

    // Subscribe to all streams
    streams := subscribeAllStreams(t, db)

    // Delete via RPC
    client.HttpDelete(ctx, &HttpDeleteRequest{HttpId: http.ID})

    // Verify all streams received delete events
    assertStreamReceivedDelete(t, streams.Headers, http.HeaderIDs)
    assertStreamReceivedDelete(t, streams.Params, http.ParamIDs)
    assertStreamReceivedDelete(t, streams.HTTP, []idwrap.IDWrap{http.ID})

    // Verify database is clean
    assertDatabaseEmpty(t, db, "http_header")
    assertDatabaseEmpty(t, db, "http_search_param")
    assertDatabaseEmpty(t, db, "http")
}
```

---

## 12. Summary

### Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Cascade awareness | Backend | Frontend stays dumb |
| Event granularity | One per entity | Simple frontend logic |
| Event order | Children first | Allows frontend FK validation |
| SQLite CASCADE | Keep as safety | Defense in depth |
| Query timing | Before delete | Ensures we capture all IDs |
| Publish timing | After commit | All or nothing consistency |

### Benefits

1. **Frontend simplicity**: Just apply events, no cascade logic
2. **Explicit auditing**: Every delete is visible in event stream
3. **Debuggability**: Can trace exactly what was deleted
4. **Consistency**: Frontend cache matches backend state
5. **Portability**: Same logic works for any frontend (React, mobile, etc.)

### Costs

1. **Extra queries**: Must collect children before delete
2. **More events**: N events instead of 1
3. **Code complexity**: Backend must know relationship graph

### Next Steps

1. Define `CascadeRule` registry for all entity types
2. Implement `collectCascadeData()` for HTTP entity
3. Add cascade publishing to `HttpDelete` RPC
4. Add tests for cascade scenarios
5. Repeat for Flow, File, and other entities
