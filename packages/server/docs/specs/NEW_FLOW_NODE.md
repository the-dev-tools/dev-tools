# Adding a New Flow Node Type

This guide walks through every layer that needs changes when adding a new flow node type (e.g., WebSocket, gRPC, GraphQL). Use HTTP and GraphQL as reference implementations.

---

## Overview of Layers

Adding a new node type touches these layers (roughly in dependency order):

0. Key interfaces & concepts (`FlowNode`, `VariableIntrospector`)
1. TypeSpec definitions (API contract)
2. Database schema + queries
3. Server model layer
4. Server service layer (reader/writer)
5. Mutation package (insert/update helpers)
6. Node executor package (runtime logic)
7. Server RPC handlers (CRUD, sync, delta, exec, node config CRUD)
8. Event streaming (sync publishers)
9. Server wiring (`serverrun.go`)
10. Flow integration (builder, runner, duplicate, copy/paste, YAML export/import, workspace bundle)
11. Client components (flow node UI, registration, collection schemas)
12. Request-type node extras (deltas, responses, versions, assertions) — only for request-type nodes

---

## 0. Key Interfaces & Concepts

Before diving into the layers, understand these core interfaces that every node type must implement:

### FlowNode Interface

Every node executor must implement `node.FlowNode` (defined in `packages/server/pkg/flow/node/node.go`):

```go
type FlowNode interface {
    GetID() idwrap.IDWrap
    GetName() string
    RunSync(ctx context.Context, req *FlowNodeRequest) FlowNodeResult
    RunAsync(ctx context.Context, req *FlowNodeRequest, resultChan chan FlowNodeResult)
}
```

### FlowNodeResult

```go
type FlowNodeResult struct {
    NextNodeID  []idwrap.IDWrap
    Err         error
    SkipFinalStatus bool        // Used by FOR/FOREACH to handle their own status logging
    AuxiliaryID     *idwrap.IDWrap  // Links execution to response (e.g., HTTP response ID)
}
```

- **`AuxiliaryID`**: Request-type nodes set this to the response ID so the client can display the response in the node's output panel.
- **`NextNodeID`**: Determined via `mflow.GetNextNodeID(req.EdgeSourceMap, nodeID, handle)`. Different handles route to different edges (e.g., `HandleUnspecified` for normal flow, `HandleTrue`/`HandleFalse` for conditions, `HandleLoop` for loops).

### Node Architecture Patterns

Nodes fall into several patterns that affect how they integrate:

- **Request-type executors** (HTTP, GraphQL, WS Connection): Execute external requests, produce responses, set `AuxiliaryID`. Need deltas, responses, versions, assertions, reference service schemas.
- **Logic executors** (For, ForEach, Condition, JS): Control flow or transform data, write output variables. Need reference service schemas.
- **Orchestrator nodes** (AI): Discover connected nodes via special edge handles (`HandleAiProvider`, `HandleAiMemory`, `HandleAiTool`) and coordinate multi-node execution. Can spawn sub-runners for tool execution.
- **Passive nodes** (Memory): Don't produce output variables — they're state containers read by other nodes at runtime via edge connections. Don't need reference service entries.
- **Sub-executor nodes** (WS Send, AI Provider): Referenced by a parent node either by name (WS Send → WS Connection) or by edge handle (AI → AI Provider). May produce their own output variables.

### VariableIntrospector (Optional)

Nodes can optionally implement `node.VariableIntrospector` for AI agent integration:

```go
type VariableIntrospector interface {
    GetRequiredVariables() []string  // e.g., ["otherNode.response.body.id"]
    GetOutputVariables() []string    // e.g., ["response.status", "response.body"]
}
```

This enables AI nodes to understand what data a node needs and produces. Use `expression.ExtractVarKeysFromMultiple()` to extract variable references from template strings.

---

## 1. TypeSpec Definitions

**Files:** `packages/spec/api/<type>.tsp`

Define the full API surface:

- **Models**: Base item, delta, sync insert/update/delete/upsert messages
- **Service methods**: Collection, Insert, Update, Delete, Sync (server-streaming), plus Delta variants
- **Sub-entities**: Headers, assertions, etc. — each needs its own full CRUD + sync + delta set

After editing `.tsp` files:
```bash
pnpm nx run spec:build
```

This generates:
- `packages/spec/dist/buf/go/` — Go protobuf + Connect RPC
- `packages/spec/dist/buf/typescript/` — TypeScript protobuf types
- `packages/spec/dist/tanstack-db/typescript/` — TanStack DB collection schemas

**Gotcha:** The tanstack-db schemas are auto-generated. They must be registered in the `schemas_v1_api_<type>` array in the generated file, and that array must be included in `packages/spec/dist/tanstack-db/typescript/v1/api.ts`.

---

## 2. Database Schema + Queries

### Schema

**File:** `packages/db/pkg/sqlc/schema/<NN>_<type>.sql`

Typical columns for a request-type node:
```sql
CREATE TABLE <type> (
    id              BLOB PRIMARY KEY NOT NULL,
    workspace_id    BLOB NOT NULL,
    folder_id       BLOB,
    name            TEXT NOT NULL DEFAULT '',
    url             TEXT NOT NULL DEFAULT '',
    -- type-specific fields...
    description     TEXT NOT NULL DEFAULT '',

    -- Delta fields
    parent_<type>_id BLOB,
    is_delta         BOOLEAN NOT NULL DEFAULT FALSE,
    is_snapshot      BOOLEAN NOT NULL DEFAULT FALSE,
    delta_name       TEXT,
    delta_url        TEXT,
    -- delta_ for each mutable field...

    created_at      INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL DEFAULT 0,

    FOREIGN KEY (workspace_id) REFERENCES workspace(id)
);
```

Also create sub-entity tables (headers, assertions, etc.) with the same delta pattern.

### Queries

**File:** `packages/db/pkg/sqlc/queries/<type>.sql`

Required queries:
- `Create<Type>` — insert
- `Get<Type>` — by ID
- `Get<Type>sByWorkspaceID` — list for workspace (filter `is_delta = FALSE AND is_snapshot = FALSE`)
- `Get<Type>DeltasByWorkspaceID` — list deltas
- `Update<Type>` — full update
- `Update<Type>Delta` — update delta fields only
- `Delete<Type>` — by ID
- Same pattern for sub-entities (headers, assertions)

After editing `.sql` files:
```bash
pnpm nx run db:generate
```

**Gotcha:** The workspace listing query MUST filter `is_delta = FALSE AND is_snapshot = FALSE` to exclude deltas and snapshots from the base collection. Forgetting this causes snapshot/delta entries to appear as base items.

**Gotcha — Sub-entity delta columns:** Sub-entity tables (headers, assertions) need delta columns too (`parent_<type>_header_id`, `is_delta`, `delta_header_key`, etc.). These can be added inline in the initial schema or via a separate delta schema file (e.g., `09_graphql_delta.sql` uses `ALTER TABLE`). The migration that creates the tables should include these columns from the start. If the schema and queries reference delta columns but the table doesn't have them, all delta operations will fail at runtime (400 errors on update, SQL errors on select).

**Gotcha — `UpdateDelta` query required:** Each entity/sub-entity that supports deltas needs TWO update queries: `Update<Type>` for base fields and `Update<Type>Delta` for delta-specific fields (`delta_*` columns). The delta update handler must call `UpdateDelta`, NOT `Update` — the base `Update` only modifies base columns and ignores delta fields entirely.

**Gotcha — `Create` must pass delta fields:** When sqlc generates a `Create<Type>Params` struct with delta fields, the service's `Create` method must populate them from the model. Otherwise, delta records are inserted with zero-valued delta fields, and `is_delta` stays `false`. Check that all delta-related fields (`ParentID`, `IsDelta`, `DeltaKey`, etc.) are mapped in the `Create` call.

---

## 3. Server Model Layer

**File:** `packages/server/pkg/model/m<type>/m<type>.go`

Define pure Go domain structs:
```go
type <Type> struct {
    ID          idwrap.IDWrap
    WorkspaceID idwrap.IDWrap
    // fields...

    // Delta fields
    Parent<Type>ID *idwrap.IDWrap
    IsDelta        bool
    IsSnapshot     bool
    DeltaName      *string
    // delta_ for each mutable field...
}
```

These bridge between API types (protobuf) and DB types (sqlc gen). Keep them pure Go — no dependencies on protobuf or sqlc packages.

---

## 4. Server Service Layer

**Files:** `packages/server/pkg/service/s<type>/`

Split into reader and writer following the SQLite deadlock prevention pattern:

- **Reader** (`reader.go`): Read-only operations using `*sql.DB` connection pool
  - `Get(ctx, id)`, `GetByWorkspaceID(ctx, wsID)`, `GetDeltasByWorkspaceID(ctx, wsID)`
- **Writer** (inline or `writer.go`): Write operations using `*sql.Tx`
  - `Create(ctx, model)`, `Update(ctx, model)`, `Delete(ctx, id)`
- **Service struct**: Wraps queries, provides `TX(tx)` method to create transactional writer

Pattern:
```go
type <Type>Service struct {
    queries *gen.Queries
}

func (s <Type>Service) TX(tx *sql.Tx) *<Type>Service {
    return &<Type>Service{queries: gen.New(tx)}
}
```

**Gotcha:** Reads MUST happen OUTSIDE transactions. Writers operate INSIDE transactions. This prevents SQLite deadlocks. The `notxread` linter enforces this.

---

## 5. Mutation Package

**Files:** `packages/server/pkg/mutation/insert_<type>.go`, `update_<type>.go`

Create insert/update helper functions that both execute the DB operation AND track the event:

```go
type <Type>InsertItem struct {
    ID          idwrap.IDWrap
    WorkspaceID idwrap.IDWrap
    IsDelta     bool
    Params      gen.Create<Type>Params
}

func (c *Context) Insert<Type>(ctx context.Context, item <Type>InsertItem) error {
    if err := c.q.Create<Type>(ctx, item.Params); err != nil {
        return err
    }
    c.track(Event{
        Entity:      Entity<Type>,
        Op:          OpInsert,
        ID:          item.ID,
        WorkspaceID: item.WorkspaceID,
        IsDelta:     item.IsDelta,
    })
    return nil
}
```

Also register the entity constant in `packages/server/pkg/mutation/mutation.go`:
```go
const Entity<Type> = "<type>"
```

**CRITICAL GOTCHA — Sync Payload:** The mutation helper's `track()` call does NOT include a `Payload`. The CRUD handler MUST add a SEPARATE `mut.Track()` call WITH the model as `Payload` for sync events to work. Without this, the publisher can't create the API model, and the sync event is silently dropped. Example:

```go
// In the CRUD handler:
if err := mut.Insert<Type>(ctx, ...); err != nil {
    return nil, err
}
// This second Track is REQUIRED for sync to work:
mut.Track(mutation.Event{
    Entity:  mutation.Entity<Type>,
    Op:      mutation.OpInsert,
    ID:      item.ID,
    Payload: model,  // <-- Without this, sync breaks silently
})
```

This applies to BOTH insert and update operations. The delete path is different — the publisher constructs a minimal model from the event ID, so no payload is needed.

---

## 6. Node Executor Package

**File:** `packages/server/pkg/flow/node/n<type>/n<type>.go`

Each node type needs a dedicated executor package that implements the `FlowNode` interface. This is the runtime logic that executes when the flow runner reaches this node.

```go
package n<type>

type Node<Type> struct {
    FlowNodeID idwrap.IDWrap
    Name       string
    // Type-specific config and dependencies...
}

func New(id idwrap.IDWrap, name string, /* deps */) *Node<Type> {
    return &Node<Type>{FlowNodeID: id, Name: name}
}

func (n *Node<Type>) GetID() idwrap.IDWrap   { return n.FlowNodeID }
func (n *Node<Type>) GetName() string        { return n.Name }

func (n *Node<Type>) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
    nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.GetID(), mflow.HandleUnspecified)
    result := node.FlowNodeResult{NextNodeID: nextID}

    // 1. Read input variables (deep copy VarMap to prevent concurrent access)
    varMapCopy := node.DeepCopyVarMap(req)

    // 2. Execute the node's logic (HTTP request, GraphQL query, etc.)
    // 3. Build output map and write to flow variables
    err := node.WriteNodeVarBulk(req, n.Name, outputMap)

    // 4. For request-type nodes: set AuxiliaryID to link to response
    result.AuxiliaryID = &responseID

    return result
}
```

**Existing executor packages** (for reference):
- `nrequest` — HTTP requests
- `ngraphql` — GraphQL queries
- `nwsconnection` — WebSocket connections
- `nwssend` — WebSocket send messages
- `nif` — Condition nodes
- `nfor` — For loops
- `nforeach` — ForEach loops
- `njs` — JavaScript execution
- `nai` — AI nodes
- `naiprovider` — AI provider nodes
- `nmemory` — AI memory nodes
- `nwait` — Wait/delay nodes
- `nstart` — Manual start nodes

**Side Response Channel (request-type nodes only):**

HTTP and GraphQL nodes send responses through a side channel for async DB persistence:
```go
// The builder passes these channels into node constructors
respChan chan nrequest.NodeRequestSideResp
gqlRespChan chan ngraphql.NodeGraphQLSideResp
```

The executor sends the response + request data through this channel after execution. A separate goroutine persists the response to the DB and publishes sync events. This decouples response storage from the flow execution path.

---

## 7. Server RPC Handlers

**Files:** `packages/server/internal/api/r<type>/`

Organize by concern:
- `r<type>.go` — Service struct, constructor, streamers, mutation publisher
- `r<type>_crud.go` — CRUD operations (Collection, Insert, Update, Delete)
- `r<type>_crud_<sub>.go` — Sub-entity CRUD (headers, assertions)
- `r<type>_delta.go` — Delta operations
- `r<type>_exec.go` — Execution logic
- `r<type>_converter.go` — Model <-> API type converters, sync response builders

### Mutation Publisher

The publisher routes mutation events to the correct sync streamer:

```go
func (p *publisher) PublishAll(events []mutation.Event) {
    for _, evt := range events {
        switch evt.Entity {
        case mutation.Entity<Type>:
            p.publish<Type>(evt)
        case mutation.Entity<Type>Header:
            p.publish<Type>Header(evt)
        }
    }
}
```

Each `publish<Type>` method must handle insert/update (from Payload) and delete (from event ID):

```go
func (p *publisher) publish<Type>(evt mutation.Event) {
    switch evt.Op {
    case mutation.OpInsert, mutation.OpUpdate:
        // MUST type-assert Payload to get the model
        if m, ok := evt.Payload.(m<type>.<Type>); ok {
            model = ToAPI<Type>(m)
        }
    case mutation.OpDelete:
        // Delete can construct from ID alone
        model = &pb.<Type>{<Type>Id: evt.ID.Bytes()}
    }
    if model != nil {
        p.streamers.<Type>.Publish(topic, event)
    }
}
```

### All CRUD handlers follow Fetch-Check-Act:
1. **FETCH**: Read data via pool services (outside transaction)
2. **CHECK**: Validate permissions/rules (pure Go)
3. **ACT**: Write via mutation context inside transaction, then commit (auto-publishes)

### Node Config CRUD Handlers

In addition to the entity's own CRUD (e.g., `rhttp`, `rgraphql`), each node type needs **node config CRUD** in the flow handler:

**File:** `packages/server/internal/api/rflowv2/rflowv2_node_<type>.go`

These manage the type-specific flow node record (e.g., `flow_node_http` table) that links a flow node to its entity:

- `Node<Type>Collection` — Lists all type-specific nodes across accessible flows
- `Node<Type>Insert` — Creates the type-specific node record (links nodeID to entityID)
- `Node<Type>Update` — Updates entity references (e.g., change which HTTP request a node points to)
- `Node<Type>Delete` — Removes the type-specific record
- `Node<Type>Sync` — Streams type-specific node mutations in real-time

**Pattern:** The sync handler filters the generic node event stream for the specific node kind, then looks up the type-specific config:

```go
func (s *FlowServiceV2RPC) streamNode<Type>Sync(ctx context.Context, send func(*resp) error) error {
    // Subscribe to generic node stream, filter by NODE_KIND_<TYPE>
    events, _ := s.nodeStream.Subscribe(ctx, filter)
    for evt := range events {
        if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_<TYPE> { continue }
        // Look up type-specific config (e.g., httpId, deltaHttpId)
        nodeCfg, _ := s.n<type>s.GetNode<Type>(ctx, nodeID)
        // Build sync response with config data
    }
}
```

**Gotcha:** The mutation payload for node config uses a wrapper struct (e.g., `nodeHttpWithFlow`) that includes the config, flowID, and base node. This provides context for the sync publisher.

---

## 8. Event Streaming

**Defined in:** The RPC handler file (`r<type>.go`)

Each syncable entity needs:
```go
type <Type>Topic struct{ WorkspaceID idwrap.IDWrap }
type <Type>Event struct {
    Type       string
    <Type>     *pb.<Type>
    IsDelta    bool
}
```

The streamer is typed: `eventstream.SyncStreamer[<Type>Topic, <Type>Event]`

Streamers are aggregated in a struct and wired in `serverrun.go`.

---

## 9. Server Wiring (`serverrun.go`)

**File:** `packages/server/cmd/serverrun/serverrun.go`

Wire in this order:
1. Create the service (reader/writer)
2. Create the in-memory sync streamer
3. Pass service + streamer into the RPC handler constructor
4. Register Connect RPC routes

**Gotcha:** If you add a service to an existing handler's dependency struct (e.g., adding `GraphQLAssert` to the flow handler), you must update BOTH the deps struct AND the constructor wiring.

---

## 10. Flow Integration

### 10a. Flow Builder

**File:** `packages/server/pkg/flow/flowbuilder/builder.go`

The flow builder bridges DB models to runtime executor instances. It needs:

1. **Builder struct** — Add the new node service field:
```go
type Builder struct {
    Node<Type> *sflow.Node<Type>Service
    // For request-type nodes, also add entity services:
    <Type>       *s<type>.<Type>Service
    <Type>Header *s<type>.<Type>HeaderService
    // ...
}
```

2. **Constructor** — Add the service parameter to `New()` and wire it.

3. **`BuildNodes()`** — Add a case in the node kind switch to create the executor:
```go
case mflow.NODE_KIND_<TYPE>:
    cfg, err := b.Node<Type>.GetNode<Type>(ctx, nodeModel.ID)
    if err != nil { return nil, nil, err }
    // For request-type nodes: resolve deltas
    resolved, err := b.<Type>Resolver.Resolve(ctx, *cfg.EntityID, cfg.DeltaEntityID)
    // Create executor instance
    flowNodeMap[nodeModel.ID] = n<type>.New(nodeModel.ID, nodeModel.Name, resolved, ...)
```

**Gotcha:** The builder runs OUTSIDE transactions (read-only). For request-type nodes, the resolver merges base + delta data before creating the executor, so the executor receives fully-resolved data.

### 10b. Flow Runner / Executor

**Files:** `packages/server/internal/api/rflowv2/rflowv2_exec*.go`

The flow runner uses the builder's output (`map[IDWrap]FlowNode`) to execute nodes. No switch statement needed here — the runner calls `node.RunSync()` or `node.RunAsync()` polymorphically.

For request-type nodes, the executor file (`rflowv2_node_<type>_exec.go` or in `pkg/flow/node/n<type>/`) handles:
- Variable resolution and request preparation
- Sending the request and capturing the response
- Writing output variables to the flow's VarMap
- Setting `AuxiliaryID` to the response ID
- Sending response data through the side channel for DB persistence

### 10c. Flow Duplicate

**File:** `packages/server/internal/api/rflowv2/rflowv2_flow.go`

Flow duplication fetches ALL node type-specific data. Add a case to the `nodeDetail` struct and the switch:

```go
type nodeDetail struct {
    node    mflow.Node
    // ...existing fields...
    <type>Node *mflow.Node<Type>
}

// In the switch:
case mflow.NODE_KIND_<TYPE>:
    if d, err := s.n<type>s.GetNode<Type>(ctx, n.ID); err == nil {
        detail.<type>Node = d
    }
```

**Gotcha:** For request-type nodes (HTTP, GraphQL), the duplicate also fetches the associated entity (e.g., the HTTP request) to display in the duplicated flow's node config panel.

### 10d. Copy/Paste

**File:** `packages/server/internal/api/rflowv2/rflowv2_copy_paste.go`

Two functions need updates:

**Copy** (`FlowNodesCopy`): Add case in the switch to fetch type-specific data:
```go
case flowv1.NODE_KIND_<TYPE>:
    s.populate<Type>Bundle(ctx, entityID, &bundle)
```

**Paste** (`FlowNodesPaste`): Handle:
- ID remapping (old ID -> new ID)
- Reference resolution (USE_EXISTING vs create new)
- Variable reference remapping in string fields
- DB creation inside transaction
- Sync event publishing

**Gotcha:** Don't forget to remap variable references in ALL string fields that can contain `{{var.name}}` expressions. Missing one causes pasted nodes to reference wrong variables.

### 10e. YAML Export/Import

**Files:** `packages/server/pkg/translate/yamlflowsimplev2/`

- **`types.go`**: Add `YamlStep<Type>` struct with `YamlStepCommon` inline. Add field to `YamlStepWrapper`.
- **`exporter.go`**: Add case in step export loop. Add to `isValid` check.
- **`converter_node.go`**: Add case to `getStepCommon()` for position data. Add processing in `processSteps()`.
- **`converter_flow.go`**: Add to `mergeFlowData` if template merging is needed.

### 10f. Workspace Bundle

**Files:** `packages/server/pkg/ioworkspace/`

- **`types.go`**: Add `<Type>s []m<type>.<Type>` (and sub-entities like headers, assertions) to `WorkspaceBundle`. Update `CountEntities()`.
- **`exporter.go`**: Fetch entities from DB in the export function.
- **`importer.go`**: Add service, counter, and import wiring.
- **`importer_flow.go`**: Add `import<Type>` function if flow-specific import logic is needed.

---

## 11. Client Components

### Flow Node Component

**File:** `packages/client/src/pages/flow/nodes/<type>.tsx`

**CRITICAL — Module-level default:** Always create the default node value at module scope:
```typescript
const defaultNode<Type> = create(Node<Type>Schema);
```

NEVER use `create()` inline as a `useLiveQuery` fallback. This causes an infinite re-render loop because `useLiveQuery` → `useDeltaState` subscription sees a new object reference every render.

**Pattern:**
```typescript
const data = useLiveQuery(
    (_) => _.from({ item: collection }).where(...).findOne(),
    [collection, id],
).data ?? defaultNode<Type>;  // module-level default
```

### URL Component

Extract the URL display into a separate component (e.g., `<TypeUrl />`), matching the HTTP/GraphQL pattern. This keeps the node component clean and allows reuse.

### Node Registration

**File:** `packages/client/src/pages/flow/edit.tsx`

Register the node component and settings component in the node type maps:

```typescript
import { <Type>Node, <Type>Settings } from './nodes/<type>';

// In the nodeTypes map:
const nodeTypes = {
    [NodeKind.<TYPE>]: <Type>Node,
    // ...
};

// In the settings components map:
const settingsMap = {
    [NodeKind.<TYPE>]: <Type>Settings,
    // ...
};
```

### Add Node Menu

**File:** `packages/client/src/pages/flow/add-node.tsx`

Add the new node type to the "Add Node" menu so users can drag it onto the canvas.

### Route & Tab Registration (Request-Type Nodes)

For request-type nodes that have their own page (HTTP, GraphQL, WebSocket), you need route files that open tabs:

**Files:**
- `packages/client/src/pages/<type>/routes/<type>/$<type>IdCan/index.tsx` — main route
- `packages/client/src/pages/<type>/routes/<type>/$<type>IdCan/route.tsx` — parent route (context)
- `packages/client/src/pages/<type>/tab.tsx` — tab component + tab ID generator

**CRITICAL — Use both `onEnter` AND `onStay`:** TanStack Router has two lifecycle hooks:
- `onEnter` — fires when a route first matches (navigating FROM a different route template)
- `onStay` — fires when a route stays matched but params change (navigating between items of the same type, e.g., GraphQL A → GraphQL B)

Both hooks must call `openTab()` with the same logic. Without `onStay`, navigating between items of the same type won't create a tab for the second item — the content renders correctly but the tab bar won't show the new tab.

```typescript
export const Route = createFileRoute('/(dashboard)/(workspace)/workspace/$workspaceIdCan/(<type>)/<type>/$<type>IdCan/')({
  component: <Type>Page,
  onEnter: async (match) => {
    const { <type>Id } = match.context;
    await openTab({
      id: <type>TabId({ <type>Id }),
      match,
      node: <<Type>Tab <type>Id={<type>Id} />,
    });
  },
  onStay: async (match) => {
    const { <type>Id } = match.context;
    await openTab({
      id: <type>TabId({ <type>Id }),
      match,
      node: <<Type>Tab <type>Id={<type>Id} />,
    });
  },
});
```

### File Tree Drag-to-Canvas

**File:** `packages/client/src/features/file-system/index.tsx`

For request-type nodes, add handling so users can drag an entity from the sidebar file tree onto the flow canvas to create a node.

### Top Bar — Delta-Aware Wiring

The request page top bar must use `useDeltaState` + `DeltaResetButton` for the name field, and use the delta-aware URL component (e.g., `<GraphQLUrl>`) rather than an inline `ReferenceField`. Also wire:
- **Delta collection** for delta-aware delete (`if (deltaId) deltaCollection.delete(...)`)
- **Delta-aware send** — send with `deltaId ?? originId`, not just `originId`
- **Transaction flushing** — wait for BOTH base and delta collection transactions before sending

Without `useDeltaState`, the top bar reads directly from the base collection and won't show delta overrides. Without `DeltaResetButton`, users can't reset individual fields back to the base value.

### Delta Reset Buttons in Editors

For CodeMirror-based editors (query, variables, raw body), add `DeltaResetButton` in a small toolbar row above the editor:
```typescript
<div className={tw`flex h-full flex-col`}>
  {!isReadOnly && (
    <div className={tw`flex items-center justify-end gap-2 pb-2`}>
      <DeltaResetButton {...deltaOptions} />
    </div>
  )}
  <CodeMirror className={tw`flex-1`} ... />
</div>
```

The editors may already use `useDeltaState` for reading/writing values but still be missing the `DeltaResetButton` — check both.

### Sub-entity Tables

For headers, assertions, etc., create table components following the pattern in `packages/client/src/pages/graphql/request/assert.tsx`:
- Use `useApiCollection(Schema)` to get the collection
- Use `useLiveQuery` with `eq`/`or` filters on the parent ID
- Use `DeltaCheckbox`, `DeltaReference`, `ColumnActionDeleteDelta` for delta-aware columns

These components have per-cell `DeltaResetButton` built-in. No additional reset button is needed at the table level.

---

## 12. Request-Type Node Extras (Deltas, Responses, Versions)

If the new node is a **request-type** (like HTTP, GraphQL, WebSocket) — not just a logic node (If, For, JS) — it needs several additional layers. Reference: `c0585188` (GraphQL delta/assertion/response commit).

### 12a. Delta System

Deltas allow users to create variants of a request without duplicating it. Required pieces:

- **DB schema**: Separate delta schema file (`schema/09_graphql_delta.sql`) with delta-specific columns on the main table (`delta_name`, `delta_url`, etc.) and `parent_<type>_id` FK
- **Delta SQL queries**: `Update<Type>Delta` — updates only delta fields, not base fields. `Get<Type>DeltasByWorkspaceID` — lists deltas for a workspace
- **RPC handlers**: `r<type>_crud_delta.go` — Delta CRUD (Insert, Update, Delete, Collection, Sync) for the main entity and each sub-entity (e.g., `r<type>_crud_header_delta.go`)
- **Delta converter**: `r<type>_delta_converter.go` — Converts between delta API types and model types
- **Resolver**: `packages/server/pkg/<type>/resolver/resolver.go` — Merges base + delta fields at read time. Called during execution and anywhere resolved data is needed
- **Patch types**: `packages/server/pkg/patch/patch.go` — Add a `<Type>Patch` struct that tracks which fields were actually changed (using `Optional[T]` for set/unset semantics)
- **Delta helpers**: `packages/server/pkg/delta/delta.go` — Generic utilities for delta field merging

**Gotcha:** The `Writer.Update()` method MUST check `IsDelta` to prevent deltas from overwriting parent base fields. The GraphQL commit fixed this exact bug.

**File System Delta Integration** — For deltas to appear in the sidebar file tree:

1. **TypeSpec `FileKind` enum** (`packages/spec/api/file-system.tsp`): Add `<Type>Delta` to the enum. Run `pnpm nx run spec:build`.
2. **Go model** (`packages/server/pkg/model/mfile/mfile.go`): Add `ContentType<Type>Delta` constant + `Is<Type>Delta()` helper + update `String()`, `ContentTypeFromString()`, `IsValidContentType()`.
3. **File handler converters** (`packages/server/internal/api/rfile/rfile.go`): Add cases in `toAPIFileKind()` and `fromAPIFileKind()`.
4. **Public converter** (`packages/server/internal/converter/converter.go`): Add case in `ToAPIFileKind()`.
5. **DB schema** (`packages/db/pkg/sqlc/schema/03_files.sql`): Update the `CHECK (content_kind IN (...))` constraint to include the new value. Also update the `content_id IS NOT NULL` check to include the new kind.
6. **Migration** (`packages/server/internal/migrations/`): Create a migration that recreates the `files` table with the expanded CHECK constraint. SQLite doesn't support `ALTER TABLE ... ALTER CONSTRAINT`, so the migration must: create `files_new` with new constraints → copy data → drop old → rename → recreate all indexes. Follow the pattern in `01KKFQT8_add_websocket_tables.go:updateFilesCheckConstraintWebSocket()`.
7. **Client file tree** (`packages/client/src/features/file-system/index.tsx`):
   - Import `<Type>DeltaCollectionSchema` and `<Type>DeltaSchema`
   - Add `Match.when(FileKind.<TYPE>_DELTA, ...)` in `FileItem`
   - Add delta children query + "New delta" menu item to the parent `<Type>File` component
   - Create `<Type>DeltaFile` component using `useDeltaState` for field resolution
   - Exclude delta kind from drag-and-drop in `shouldAcceptItemDrop`

**CRITICAL GOTCHA — SQLite CHECK constraint:** The `files` table has a `CHECK (content_kind IN (...))` constraint that whitelists allowed values. Adding a new `FileKind` enum value in TypeSpec is not enough — if the DB constraint doesn't include the new integer value, `FileInsert` will return a 500 error. Always update the schema AND create a migration.

### 12b. Response Tracking

Responses are stored every time a request is executed:

- **Model**: Response struct (e.g., `mgraphql.GraphQLResponse`) with status, body, duration, size, timestamps
- **Response headers**: Separate model + service for response headers
- **Response assertions**: Results of assertion evaluation stored per response
- **Service**: `pkg/service/s<type>/response.go` — Reader/writer for responses, response headers, response assertions
- **RPC handlers**: `r<type>_crud_response_assert.go` — Response assertion collection + sync
- **Response helper**: `packages/server/pkg/<type>/response/response.go` — Response processing logic
- **Client UI**: Response panel, response header viewer, response assertion viewer, response history

### 12c. Version / History Tracking

Versions create snapshots of a request at execution time for history:

- **DB**: Version table linking a version ID to the request ID + user ID
- **RPC handler**: `r<type>_crud_version.go` — Version collection + sync endpoints
- **Snapshot creation**: During execution, clone the request + all sub-entities (headers, assertions) into a snapshot entry
- **Client UI**: History view showing past versions with their responses

### 12d. Assertion System

Assertions validate response data after execution:

- **Model**: Assert struct with value (expression), enabled, display_order, delta fields
- **Service**: `pkg/service/s<type>/assert.go` — Full CRUD reader/writer
- **Execution**: `r<type>_exec_assert.go` — Expression evaluation engine, context building (response fields as variables), result tracking
- **Response assertions**: Store pass/fail results per response for display
- **Client UI**: Assertion editor table (request panel), assertion results viewer (response panel)

### 12e. Reference Service

**File:** `packages/server/internal/api/rreference/rreference.go`

Add the new node type to the reference service so variables like `{{ NodeName.response.body }}` resolve correctly across flow nodes. Only nodes that write output variables (via `WriteNodeVarBulk`) need a reference service entry. Passive nodes (e.g., Memory) that don't produce output should be skipped — the `default` case handles them correctly.

### 12f. Migration

**File:** `packages/server/internal/migrations/01XXXXX_add_<type>_<feature>.go`

Create a migration that adds the new tables. Follow the existing pattern — register it in `migrations_test.go`.

---

## Common Pitfalls

### Sync Events Silently Dropped
The most insidious bug. If a CRUD handler calls `mut.Insert<Type>()` or `mut.Update<Type>()` without a separate `mut.Track(Event{Payload: model})`, the sync event has no payload. The publisher silently drops it (model is nil). The client never receives the update. Data persists in DB but UI doesn't reflect changes until page refresh. Always verify sync works end-to-end.

### SQLite Deadlocks
All reads MUST happen before opening a transaction. The `notxread` linter catches this, but only for direct reads — be careful with service methods that might read internally.

### Snapshot/Delta Filtering
Workspace listing queries must filter `is_delta = FALSE AND is_snapshot = FALSE`. Without this, snapshots created during execution appear as base items in the collection.

### Double Event Tracking
The mutation helpers (`Insert<Type>`, `Update<Type>`) track events internally without payloads. The CRUD handler then adds a second `mut.Track()` with the payload. This is intentional — the first event is silently ignored by the publisher, the second is published. Don't remove either one.

### Client Re-render Loops
Using `create(Schema)` as an inline fallback in `useLiveQuery` creates a new object reference every render, triggering infinite re-renders through the delta state subscription system. Always hoist to module scope.

### Variable Reference Remapping
In copy/paste, any string field that can contain `{{var.name}}` variable references must be remapped when pasting. Missing a field causes pasted nodes to reference variables from the source flow.

### Missing `onStay` in Route Lifecycle
TanStack Router's `onEnter` only fires when a route first enters the matched set. When navigating between items of the same type (e.g., GraphQL A → GraphQL B), the route stays matched — only `onStay` fires. If a route only uses `onEnter` to call `openTab()`, the second item's tab won't appear in the tab bar (though the content renders correctly). Always use both `onEnter` and `onStay`.

### SQLite CHECK Constraint on `files` Table
The `files` table has `CHECK (content_kind IN (0, 1, 2, ...))` that whitelists allowed content type integers. Adding a new `FileKind` in TypeSpec and `ContentType` in Go is not enough — the DB will reject inserts with a 500 error if the integer isn't in the CHECK list. You must update `03_files.sql` AND create a migration that recreates the table with the expanded constraint (SQLite doesn't support `ALTER CONSTRAINT`). See `01KKFQT8_add_websocket_tables.go` for the table-recreation pattern.

### Delta Service Layer — `Create` and `UpdateDelta` Gaps
Three common gaps when adding delta support to sub-entities (headers, assertions):
1. **`Create` ignores delta fields** — The service `Create` method only passes base fields to the sqlc `CreateParams`, even though the struct includes delta fields. Delta records get inserted with `is_delta = false` and nil delta values. Fix: populate `IsDelta`, `ParentID`, and all `Delta*` fields in the `Create` call.
2. **No `UpdateDelta` method** — The service only has a base `Update` that writes base columns. Delta field updates silently do nothing because the `Update` SQL doesn't touch `delta_*` columns. Fix: add an `UpdateDelta` query in sqlc and a corresponding service method.
3. **Handler calls `Update` instead of `UpdateDelta`** — The delta update RPC handler modifies the model's `Delta*` fields in memory, then calls `Update()` which only persists base fields. The delta changes are discarded. Returns 400 if `is_delta` is checked (since the DB row has `is_delta = false` from gap #1). Fix: call `UpdateDelta` in the handler.

### Tab Active State — Parent Route Highlighting on Delta Pages
TanStack Router's link `activeOptions` defaults to `{ exact: false }`. When viewing a delta page (e.g., `/graphql/$id/delta/$deltaId`), the parent route (`/graphql/$id`) also matches as "active". This causes both the original request tab and the delta tab to highlight simultaneously. Fix: add `activeOptions={{ exact: true }}` to tab link components.

---

## WebSocket Implementation Reference

The WebSocket node type (commit `ceb3c505`) is the most complete reference. Here are the exact files that were created/modified:

### New Files Created

| Layer | File | Purpose |
|-------|------|---------|
| DB Schema | `packages/db/pkg/sqlc/schema/10_websocket.sql` | WebSocket + header tables |
| DB Queries | `packages/db/pkg/sqlc/queries/websocket.sql` | CRUD queries |
| Model | `packages/server/pkg/model/mwebsocket/mwebsocket.go` | Domain structs |
| Service | `packages/server/pkg/service/swebsocket/swebsocket.go` | Service + TX |
| Service | `packages/server/pkg/service/swebsocket/header.go` | Header service |
| Service | `packages/server/pkg/service/swebsocket/mapper.go` | DB row -> model conversion |
| RPC Handler | `packages/server/internal/api/rwebsocket/rwebsocket.go` | Full CRUD + sync + publisher |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_connection.go` | Flow node type record |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_connection_reader.go` | Reader |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_connection_writer.go` | Writer |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_connection_mapper.go` | Mapper |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_send.go` | WS Send node type |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_send_reader.go` | Reader |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_send_writer.go` | Writer |
| Flow Node Service | `packages/server/pkg/service/sflow/node_ws_send_mapper.go` | Mapper |
| Node Executor | `packages/server/pkg/flow/node/nwsconnection/nwsconnection.go` | WS Connection runtime executor |
| Node Executor | `packages/server/pkg/flow/node/nwssend/nwssend.go` | WS Send runtime executor |
| Node Config CRUD | `packages/server/internal/api/rflowv2/rflowv2_node_ws_connection.go` | WS Connection node config CRUD + sync |
| Node Config CRUD | `packages/server/internal/api/rflowv2/rflowv2_node_ws_send.go` | WS Send node config CRUD + sync |
| Client Node | `packages/client/src/pages/flow/nodes/ws-connection.tsx` | Flow canvas node UI |
| Client Node | `packages/client/src/pages/flow/nodes/ws-send.tsx` | Flow canvas node UI |
| Client Page | `packages/client/src/pages/websocket/page.tsx` | WS request page |
| Client Page | `packages/client/src/pages/websocket/request/` | Request panel, headers, URL |
| Client Page | `packages/client/src/pages/websocket/response/` | Response panel |
| TypeSpec | `packages/spec/api/websocket.tsp` | API contract |

### Existing Files Modified

| Layer | File | What Changed |
|-------|------|-------------|
| DB Flow Schema | `packages/db/pkg/sqlc/schema/05_flow.sql` | Added `flow_node_ws_connection`, `flow_node_ws_send` tables |
| DB Flow Queries | `packages/db/pkg/sqlc/queries/flow.sql` | Added CRUD queries for WS node types |
| Flow Model | `packages/server/pkg/model/mflow/node_types.go` | Added `NodeWsConnection`, `NodeWsSend` structs |
| Flow Model | `packages/server/pkg/model/mflow/node.go` | Added `NODE_KIND_WS_CONNECTION`, `NODE_KIND_WS_SEND` |
| Flow Service | `packages/server/pkg/service/sflow/node_mapper.go` | Added WS node kind mapping |
| Flow Builder | `packages/server/pkg/flow/flowbuilder/builder.go` | Added WS services + `BuildNodes` cases |
| Flow RPC | `packages/server/internal/api/rflowv2/rflowv2.go` | Added WS services, streamers, node sync cases |
| Copy/Paste | `packages/server/internal/api/rflowv2/rflowv2_copy_paste.go` | Copy: `populateWebSocketBundle`. Paste: ID remap, var remap, DB create, event publish |
| YAML Types | `packages/server/pkg/translate/yamlflowsimplev2/types.go` | Added `YamlStepWsConnection`, `YamlStepWsSend`, `YamlStepWrapper` fields |
| YAML Exporter | `packages/server/pkg/translate/yamlflowsimplev2/exporter.go` | Added WS step export cases |
| YAML Converter | `packages/server/pkg/translate/yamlflowsimplev2/converter_node.go` | Added WS step processing |
| YAML Converter | `packages/server/pkg/translate/yamlflowsimplev2/converter_flow.go` | Added WS to `mergeFlowData` |
| Workspace Bundle | `packages/server/pkg/ioworkspace/types.go` | Added `WebSockets`, `WebSocketHeaders` fields |
| Workspace Export | `packages/server/pkg/ioworkspace/exporter.go` | Added WS entity fetching |
| Workspace Import | `packages/server/pkg/ioworkspace/importer.go` | Added WS service + import wiring |
| Workspace Import | `packages/server/pkg/ioworkspace/importer_flow.go` | Added `importWebSockets` |
| Server Wiring | `packages/server/cmd/serverrun/serverrun.go` | Added WS service, streamer, handler registration |
| Client File Tree | `packages/client/src/features/file-system/index.tsx` | Added WS drag-to-flow-canvas |
| Client Flow | `packages/client/src/pages/flow/node.tsx` | Added WS node rendering case |
| Client Flow | `packages/client/src/pages/flow/add-node.tsx` | Added WS to "Add Node" menu |

### Key Pattern: WS Connection + WS Send (Two Node Types, One Entity)

WebSocket is unique because it has TWO flow node types that share ONE sidebar entity:
- **WS Connection node** — opens the connection (references a `websocket` entity by ID)
- **WS Send node** — sends a message on an open connection (references a WS Connection node by name)

This means:
- The `WsSend` node has a `WsConnectionNodeName` field that must be remapped during copy/paste
- The YAML step for `ws_send` has `ws_connection_node_name` linking to a `ws_connection` step
- When copying, the WS entity + headers are fetched via `populateWebSocketBundle`
- When pasting, WS entities always get new IDs (no USE_EXISTING reference mode for WS)

---

## Checklist

### Spec & DB
- [ ] TypeSpec models + service methods defined
- [ ] `pnpm nx run spec:build` — codegen passes
- [ ] DB schema created (`schema/<NN>_<type>.sql`)
- [ ] Flow node table added to `schema/05_flow.sql` (e.g., `flow_node_<type>`)
- [ ] DB queries created (`queries/<type>.sql`)
- [ ] Flow node queries added to `queries/flow.sql`
- [ ] `pnpm nx run db:generate` — sqlc passes

### Server Model & Service
- [ ] Model structs created (`pkg/model/m<type>/`)
- [ ] Flow node model added to `pkg/model/mflow/node_types.go`
- [ ] Node kind constant added to `pkg/model/mflow/node.go`
- [ ] Service reader/writer created (`pkg/service/s<type>/`)
- [ ] Flow node service created (`pkg/service/sflow/node_<type>*.go` — service, reader, writer, mapper)
- [ ] Node kind mapping added to `pkg/service/sflow/node_mapper.go`

### Mutation & Events
- [ ] Mutation helpers created (`pkg/mutation/insert_<type>.go`, `update_<type>.go`)
- [ ] Entity constant registered in mutation package
- [ ] Sync streamers defined and wired

### Node Executor
- [ ] Executor package created (`pkg/flow/node/n<type>/`)
- [ ] Implements `FlowNode` interface (`GetID`, `GetName`, `RunSync`, `RunAsync`)
- [ ] Implements `VariableIntrospector` (optional, for AI integration)

### RPC & Wiring
- [ ] RPC handlers created (`internal/api/r<type>/`)
- [ ] Node config CRUD handlers created (`rflowv2_node_<type>.go`)
- [ ] Mutation publisher handles the entity type
- [ ] `serverrun.go` — service, streamer, and RPC handler wired

### Flow Integration
- [ ] Flow builder — node kind case + service field added (`pkg/flow/flowbuilder/builder.go`)
- [ ] Flow duplicate — `nodeDetail` case added (`rflowv2_flow.go`)
- [ ] Copy/paste — copy and paste cases added
- [ ] YAML export/import — types, exporter, converter updated
- [ ] Workspace bundle — types, exporter, importer updated

### Client
- [ ] Flow node component created (with module-level default) (`pages/flow/nodes/<type>.tsx`)
- [ ] Node registered in `edit.tsx` (nodeTypes + settingsMap)
- [ ] Added to "Add Node" menu (`add-node.tsx`)
- [ ] Client sub-entity components created (headers, assertions)
- [ ] Route files with `onEnter` + `onStay` tab opening (for request-type nodes)
- [ ] File tree delta integration — `FileKind` enum, model, converters, delta file component (for request-type nodes with deltas)
- [ ] File tree drag-to-canvas (for request-type nodes)

### Delta Service Layer
- [ ] Sub-entity schemas include delta columns (`parent_*_id`, `is_delta`, `delta_*` fields)
- [ ] `Create` method passes delta fields to sqlc params (not just base fields)
- [ ] `UpdateDelta` query + service method exists for each deltable entity/sub-entity
- [ ] Delta RPC handler calls `UpdateDelta`, not `Update`
- [ ] Top bar uses `useDeltaState` + `DeltaResetButton` for name/URL
- [ ] CodeMirror editors (query, variables, raw body) have `DeltaResetButton`
- [ ] Tab links use `activeOptions={{ exact: true }}` to prevent parent route highlighting

### Verification
- [ ] CRUD handlers have `mut.Track()` with Payload for sync
- [ ] All tests pass (`task test`)
- [ ] Lints pass (`task lint`)
