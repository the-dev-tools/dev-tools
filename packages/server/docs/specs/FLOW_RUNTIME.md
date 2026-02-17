# Flow Runtime: Deployed Multi-Tenant Execution System

## 1. Overview

The Flow Runtime is a standalone service that executes deployed flows in response to external events. It transforms DevTools from a local-first API testing tool into a **serverless-like automation platform** where flows act as deployed programs, triggered by webhooks, messaging platforms, schedules, or other event sources.

### Goals

- **Deploy flows** as versioned, immutable artifacts that run independently of the desktop app
- **Multi-tenant isolation** — each tenant (workspace) gets its own Turso database with full data isolation
- **Event-driven triggers** — Telegram, WhatsApp, webhooks, cron, and extensible to any event source
- **Concurrent execution** — many flows run simultaneously with fair scheduling across tenants
- **Persistent state** — all execution history, results, and conversational state are stored durably
- **Pause & Resume** — flows can wait for external input (e.g., a user reply in Telegram) and resume later

### Non-Goals (V1)

- Distributed execution across multiple machines (single-binary deployment first)
- Sub-millisecond latency guarantees
- Visual flow editing within the runtime (deploy from desktop/CLI only)
- Custom user authentication for triggers (handled by event source platforms)

---

## 2. Core Concepts

### 2.1 Deployment

A **Deployment** is an immutable snapshot of a flow and all its dependencies, ready for execution.

```
  +--------------------------+        +---------------------------+
  |   Desktop / CLI          |        |   Flow Runtime            |
  |                          |        |                           |
  |   Flow (mutable)         | -----> |   Deployment (immutable)  |
  |   - edit nodes           | deploy |   - versioned snapshot    |
  |   - test locally         |        |   - bound to triggers     |
  |   - iterate              |        |   - runs independently    |
  +--------------------------+        +---------------------------+
```

A deployment captures:
- Flow graph (nodes + edges)
- Node configurations (HTTP requests, JS code, AI prompts, etc.)
- Flow variables (defaults)
- Environment variable bindings (references, not values — resolved at execution time)
- Metadata (name, version, created_at, deployed_by)

Deployments are **versioned**. Multiple versions of the same flow can exist. Triggers bind to **tags**, not directly to deployments.

### 2.2 Tag

A **Tag** is a named, movable pointer to a specific deployment — like a git branch/tag or a Docker image tag.

```
  Tags are movable pointers to immutable deployments
  ===================================================

  Flow: "Auth API"

  Tags (mutable refs)            Deployments (immutable snapshots)
  ===================            =================================

  :latest --------+
                  +-----------> v5  (Feb 17, 14:30)
  :staging -------+

  :production ----------------> v4  (Feb 14, 09:00)

  :v3 (locked) ---------------> v3  (Feb 10, 11:15)

  :v2 (locked) ---------------> v2  (Feb 5, 16:45)

                                v1  (Feb 1, 08:00)  <-- no tag, still exists
```

**Tag types:**

- **Predefined tags** — created automatically, have special behavior:
  - `:latest` — auto-assigned on every deploy (always points to newest version)
  - `:production` — predefined but not auto-assigned (user explicitly promotes)
  - `:staging` — predefined but not auto-assigned
- **Version tags** — auto-created, immutable:
  - `:v1`, `:v2`, `:v3` — created on each deploy, locked (cannot be moved)
- **Custom tags** — user-defined, mutable:
  - `:canary`, `:feature-x`, `:rollback-safe`, etc.
  - Free-form naming, user creates and moves them

**Tag behaviors:**

```
  Deploy v5                          Promote staging -> production
  =========                          ================================

  Before:                            Before:
    :latest --> v4                     :staging    --> v5
    :v4     --> v4                     :production --> v4

  After:                             After:
    :latest --> v5  (auto-moved)       :staging    --> v5  (unchanged)
    :v5     --> v5  (auto-created)     :production --> v5  (moved)
    :v4     --> v4  (unchanged)

  Rollback production                Tag in URL
  ======================             ==========

  Before:                            POST /t/{path}             --> resolves :latest
    :production --> v5               POST /t/{path}:production  --> resolves :production
                                     POST /t/{path}:v3          --> resolves :v3
  After:
    :production --> v4  (moved back)
```

**Tag-level configuration:**

Tags can carry their own config overrides that apply when that tag is resolved:

```
  Tag: :production                    Tag: :staging
  ====================                ====================
  env_override:                       env_override:
    BASE_URL: https://api.prod.com      BASE_URL: https://api.staging.com
    LOG_LEVEL: warn                     LOG_LEVEL: debug
  variables_override:                 variables_override:
    timeout: 30                         timeout: 5
    retry_count: 3                      retry_count: 1
```

When an execution runs through a tag, the resolution order is:
1. Tag's `env_override` and `variables_override` (highest priority)
2. Deployment's `env_bindings` and `flow_variables`
3. Tenant-level environment variables (lowest priority)

### 2.3 Trigger

A **Trigger** is a configured event source bound to a **tag**. When the trigger fires, it resolves the tag to a deployment and creates an execution.

```
  +-------------+    +-------------+    +-------------+    +-------------+
  |   Webhook   |    |  Telegram   |    |  WhatsApp   |    |   Cron      |
  |   HTTP POST |    |  Bot API    |    |  Business   |    |   Schedule  |
  +------+------+    +------+------+    +------+------+    +------+------+
         |                  |                  |                  |
         +------------------+------------------+------------------+
                            |
                            v
                   +--------+--------+
                   |  TriggerEvent   |
                   |  (normalized)   |
                   +-----------------+
                            |
                            v
                   +--------+--------+
                   |   Execution     |
                   +-----------------+
```

```
  Trigger -> Tag -> Deployment resolution
  ========================================

  Trigger: "Prod Webhook"
       |
       | bound to tag: :production
       |
       v
  Tag :production --> Deployment v4
       |
       +-- env_override: {BASE_URL: "https://api.prod.com"}
       |
       v
  Execute Deployment v4
  with merged config
```

Each trigger:
- Belongs to a single tenant (workspace)
- Bound to a **tag** (e.g., `:production`, `:latest`, `:v3`)
- Resolves the tag to a deployment at execution time (not at config time)
- Has its own configuration (e.g., Telegram bot token, cron expression)
- Produces a normalized `TriggerEvent` that the execution engine consumes
- Can be enabled/disabled independently

**Moving a tag instantly updates all triggers bound to it** — no trigger reconfiguration needed. For example, moving `:production` from v4 to v5 means all production triggers immediately start running v5 on the next event.

### 2.4 Execution

An **Execution** is a single run of a deployed flow, initiated by a trigger event. It records which tag and deployment version were resolved at creation time (for audit/debugging).

```
  Execution Lifecycle:
  ====================

  +--------+     +----------+     +------------------------------------------+
  | QUEUED | --> | RUNNING  | --> | COMPLETED | FAILED | CANCELED            |
  +--------+     +----+-----+     +------------------------------------------+
                      |
                      |  (WaitForInput node)
                      v
                 +---------+     (new event)     +----------+
                 | WAITING | ------------------> | RUNNING  | --> ...
                 +---------+                     +----------+
```

- `QUEUED` — trigger fired, execution waiting for a worker slot
- `RUNNING` — actively executing nodes
- `WAITING` — paused, waiting for external input (e.g., user reply)
- `COMPLETED` — all nodes finished successfully
- `FAILED` — a node failed and the flow's error policy stopped execution
- `CANCELED` — manually canceled or timed out

### 2.5 Session

A **Session** groups related executions for conversational/stateful triggers (Telegram, WhatsApp). A session represents an ongoing conversation with a single user/chat.

```
  Session = (trigger_id, external_chat_id)

  +-- Session Scope ----------------------------------------+
  |                                                         |
  |  Execution 1          Execution 2          Execution 3  |
  |  (user: "hello")      (user: "my name")   (user: "bye")|
  |       |                     |                    |      |
  |       v                     v                    v      |
  |  [accumulated variables persist across executions]      |
  |  [AI memory / conversation history carries forward]     |
  |                                                         |
  +---------------------------------------------------------+
  |  TTL: 1 hour (configurable)                             |
  +---------------------------------------------------------+
```

- Session is identified by: `(trigger_id, external_chat_id)`
- Holds conversation state (variables, memory) that persists across executions
- Allows a flow to "remember" previous interactions
- Has a TTL (configurable per trigger) after which it resets

---

## 3. Architecture

### 3.1 High-Level System Architecture

```
  +------------------------------------------------------------------+
  |                        Flow Runtime                               |
  |                                                                   |
  |  +----------------+   +----------------+   +-------------------+  |
  |  |    Trigger      |   |   Execution    |   |   Deployment      |  |
  |  |    Gateway      |   |   Scheduler    |   |   Store           |  |
  |  |                |   |                |   |                   |  |
  |  |  * Webhook     |   |  * Queue       |   |  * Flow snapshots |  |
  |  |  * Telegram    +-->|  * Workers     |<--+  * Node configs   |  |
  |  |  * WhatsApp    |   |  * Fairness    |   |  * Var defaults   |  |
  |  |  * Cron        |   |  * Limits      |   |                   |  |
  |  +----------------+   +-------+--------+   +-------------------+  |
  |                               |                                   |
  |                        +------v-------+                           |
  |                        |FlowLocalRunner|  (existing engine)       |
  |                        |              |                           |
  |                        |  * Nodes     |                           |
  |                        |  * Variables |                           |
  |                        |  * JS Worker |                           |
  |                        +------+-------+                           |
  |                               |                                   |
  |  +----------------+   +------v--------+   +-------------------+   |
  |  |   Session       |   |  Execution    |   |   Tenant          |   |
  |  |   Store         |   |  History      |   |   Config          |   |
  |  |                |   |               |   |                   |   |
  |  |  * Chat state  |   |  * Results    |   |  * Env variables  |   |
  |  |  * Variables   |   |  * Node logs  |   |  * Secrets        |   |
  |  |  * Memory      |   |  * Errors     |   |  * Rate limits    |   |
  |  +----------------+   +---------------+   +-------------------+   |
  |                                                                   |
  +-----------------------------+-------------------------------------+
                                |
                                v
  +-----------------------------+-------------------------------------+
  |                     Turso / LibSQL                                |
  |                                                                   |
  |  +--------------+  +--------------+  +--------------+             |
  |  | Tenant A DB  |  | Tenant B DB  |  | Tenant N DB  |   ...      |
  |  | (workspace)  |  | (workspace)  |  | (workspace)  |             |
  |  +--------------+  +--------------+  +--------------+             |
  |                                                                   |
  |  +--------------+                                                 |
  |  | Control DB   |  (tenants, triggers, routing metadata)          |
  |  +--------------+                                                 |
  +-------------------------------------------------------------------+
```

### 3.2 Database Architecture — Turso Multi-DB

Each tenant (workspace) gets its own Turso database. A shared **control database** holds cross-tenant routing metadata.

```
  Database Topology
  =================

  +-------------------+       +-------------------+
  |   Control DB      |       |   Tenant DB       |
  |   (shared)        |       |   (per workspace) |
  |                   |       |                   |
  |  * tenant         |  1:1  |  * deployment     |
  |  * trigger_route  +------>|  * trigger        |
  |  * tenant_limits  |       |  * execution      |
  |                   |       |  * session         |
  |                   |       |  * checkpoint      |
  |                   |       |  * node_execution  |
  +-------------------+       +-------------------+

  Request Flow:
  =============

  Inbound event
       |
       v
  Control DB: lookup trigger_route by webhook_path
       |
       +-- workspace_id, deployment_id, turso_db_url
       |
       v
  Tenant DB: load deployment, create execution, run flow
```

**Why Turso per-tenant:**
- **Natural isolation** — no workspace_id filtering bugs can leak data across tenants
- **Independent scaling** — hot tenants don't slow down cold ones
- **Simple backups** — per-tenant backup/restore without affecting others
- **Edge replicas** — Turso can replicate tenant DBs to edge locations near their trigger sources
- **Schema migrations** — can be rolled out per-tenant (canary deployments)

**Control DB tables** (minimal, read-heavy):

```sql
-- Tenant registry
CREATE TABLE tenant (
    id              TEXT PRIMARY KEY,       -- workspace_id (ULID)
    name            TEXT NOT NULL,
    turso_db_url    TEXT NOT NULL,           -- libsql://tenant-xxx.turso.io
    turso_auth_token TEXT NOT NULL,          -- encrypted
    status          TEXT NOT NULL DEFAULT 'active',  -- active, suspended, deleted
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Fast trigger routing (denormalized for O(1) lookup on inbound events)
CREATE TABLE trigger_route (
    webhook_path    TEXT PRIMARY KEY,       -- /t/{ulid} or /tg/{id} or /wa/{id}
    trigger_id      TEXT NOT NULL,
    tenant_id       TEXT NOT NULL,
    trigger_type    TEXT NOT NULL,          -- webhook, telegram, whatsapp
    enabled         INTEGER NOT NULL DEFAULT 1,

    FOREIGN KEY (tenant_id) REFERENCES tenant(id)
);

-- Per-tenant resource limits
CREATE TABLE tenant_limits (
    tenant_id                TEXT PRIMARY KEY,
    max_concurrent_executions INTEGER NOT NULL DEFAULT 10,
    max_queue_depth          INTEGER NOT NULL DEFAULT 100,
    max_executions_per_hour  INTEGER NOT NULL DEFAULT 1000,
    max_active_sessions      INTEGER NOT NULL DEFAULT 100,
    execution_timeout_sec    INTEGER NOT NULL DEFAULT 300,
    wait_timeout_sec         INTEGER NOT NULL DEFAULT 3600,

    FOREIGN KEY (tenant_id) REFERENCES tenant(id)
);
```

**Connection management:**

```
  +---------------------------+
  |   Connection Pool Manager |
  |                           |
  |   tenant_id --> *sql.DB   |
  |                           |
  |   * LRU eviction          |
  |   * Max open connections  |
  |   * Health checks         |
  |   * Lazy initialization   |
  +---------------------------+
        |          |          |
        v          v          v
  +--------+  +--------+  +--------+
  | Turso  |  | Turso  |  | Turso  |
  | DB (A) |  | DB (B) |  | DB (N) |
  +--------+  +--------+  +--------+
```

The runtime maintains a pool of Turso connections keyed by `tenant_id`. Connections are lazily opened on first access and evicted when idle. The existing `devtoolsdb` package and sqlc-generated queries work unchanged — they receive a `*sql.DB` / `*sql.Tx` regardless of whether it points to local SQLite or remote Turso.

### 3.3 Trigger Gateway

The Trigger Gateway is the inbound layer. It receives events from external sources, normalizes them, and enqueues executions.

```
  Trigger Gateway Internals
  =========================

  HTTP Server (shared with runtime)
       |
       +-- POST /t/{path}     --> WebhookAdapter
       +-- POST /tg/{id}      --> TelegramAdapter
       +-- POST /wa/{id}      --> WhatsAppAdapter
       +-- GET  /wa/{id}      --> WhatsApp verification
       |
       v
  +-------------------+
  |  Route Resolver    |
  |                   |
  |  1. Parse path    |
  |  2. Control DB:   |
  |     trigger_route |
  |  3. Verify auth   |
  +--------+----------+
           |
           v
  +--------+-----------+
  |  Trigger Adapter    |
  |                    |
  |  * Normalize event |
  |  * Validate input  |
  |  * Extract chat_id |
  +--------+-----------+
           |
           v
  +--------+-----------+
  |  Event Sink         |
  |                    |
  |  * Tenant DB:      |
  |    create execution|
  |  * Queue for       |
  |    scheduler       |
  +--------------------+


  Cron Scheduler (internal, no HTTP)
  ===================================

  +--------------------+
  |  Cron Ticker       |
  |                   |
  |  * Load schedule  |
  |    triggers from  |
  |    all tenant DBs |
  |  * Fire on match  |
  |  * Feed to Sink   |
  +--------------------+
```

**Trigger Adapter Interface:**

```go
type TriggerAdapter interface {
    // Type returns the trigger type identifier.
    Type() TriggerType

    // Start begins listening for events (webhook registration, polling loop, etc.)
    Start(ctx context.Context, config TriggerConfig, sink EventSink) error

    // Stop gracefully shuts down the adapter.
    Stop(ctx context.Context) error

    // SendResponse sends a reply back through the event source (e.g., Telegram message).
    SendResponse(ctx context.Context, session SessionRef, payload ResponsePayload) error
}

type EventSink interface {
    // Enqueue submits a trigger event for execution.
    Enqueue(ctx context.Context, event TriggerEvent) (ExecutionID, error)
}
```

**TriggerEvent (normalized input):**

```go
type TriggerEvent struct {
    TriggerID    idwrap.IDWrap
    WorkspaceID  idwrap.IDWrap
    Timestamp    time.Time

    // Tag resolution (filled by gateway after resolving tag -> deployment)
    TagName      string            // e.g., "production" (from trigger config or URL override)
    DeploymentID idwrap.IDWrap     // Resolved deployment
    EnvOverride  map[string]string // Merged from tag config
    VarsOverride map[string]any    // Merged from tag config

    // Source-specific identity for session tracking
    ExternalChatID string  // e.g., Telegram chat_id, WhatsApp phone number
    ExternalUserID string  // e.g., Telegram user_id

    // The event payload, normalized into a map accessible as flow variables
    // e.g., {"message.text": "hello", "message.from": "John", "message.chat_id": "123"}
    Payload map[string]any

    // Raw payload for advanced use (available as {{trigger.raw}})
    RawPayload []byte
}
```

### 3.4 Execution Scheduler

The scheduler manages concurrent flow execution with tenant fairness.

```
  Execution Scheduler
  ====================

                +----------------------------------+
                |        Execution Queue            |
                |                                  |
                |  Tenant A:  [ E1 ] [ E2 ] [ E3 ] |
                |  Tenant B:  [ E4 ]                |
                |  Tenant C:  [ E5 ] [ E6 ]         |
                +----------------+-----------------+
                                 |
                                 v
                +----------------+-----------------+
                |        Fair Scheduler             |
                |                                  |
                |  Round-robin across tenants       |
                |  Respect per-tenant quota         |
                |  Skip tenants at concurrency cap  |
                +----------------+-----------------+
                                 |
                +----------------+------------------+
                |                |                  |
                v                v                  v
          +-----------+   +-----------+   +-----------+
          | Worker 1  |   | Worker 2  |   | Worker N  |
          |           |   |           |   |           |
          | Tenant DB |   | Tenant DB |   | Tenant DB |
          | Runner    |   | Runner    |   | Runner    |
          | JS Worker |   | JS Worker |   | JS Worker |
          +-----------+   +-----------+   +-----------+
```

**Concurrency Limits (configurable per tenant via `tenant_limits`):**

| Limit | Default | Description |
|-------|---------|-------------|
| `max_concurrent_executions` | 10 | Max simultaneous running flows per tenant |
| `max_queue_depth` | 100 | Max queued executions per tenant before rejecting |
| `execution_timeout` | 5m | Max wall-clock time for a single execution |
| `max_waiting_sessions` | 50 | Max paused executions waiting for external input |

### 3.5 Deployment Store

Manages immutable flow snapshots within each tenant's Turso DB.

```
  Deploy Flow (with auto-tagging)
  ================================

  Desktop/CLI                     Runtime
  ==========                     =======

  1. User clicks "Deploy"
         |
         v
  2. Snapshot flow:
     nodes, edges,
     configs, vars
         |
         v
  3. POST DeploymentCreate -----> 4. Validate snapshot
     (Connect RPC)                    |
                                      v
                                 5. Tenant DB:
                                    INSERT deployment
                                    (version N+1)
                                      |
                                      v
                                 6. Auto-create tags:
                                    :latest --> v(N+1)  (move)
                                    :v(N+1) --> v(N+1)  (create, locked)
                                      |
                                      v
                                 7. Update trigger_route
                                    in Control DB
                                      |
                                      v
                                 8. Return deployment_id
                                    + tag info


  Tag Operations (post-deploy)
  =============================

  Move tag:
    TagMove(:production, v5)  --> :production now points to v5
                                  all triggers bound to :production
                                  immediately pick up v5

  Rollback:
    TagMove(:production, v4)  --> :production now points to v4
                                  instant, zero downtime

  Create custom tag:
    TagCreate(:canary, v5)    --> new tag pointing to v5
                                  triggers can bind to :canary

  Delete custom tag:
    TagDelete(:canary)        --> tag removed
                                  triggers bound to it become inactive

  Locked tags (version tags) cannot be moved or deleted:
    TagMove(:v3, v5)          --> ERROR: tag :v3 is locked
```

**Relationship to existing versioning:**
The current `version_parent_id` system captures point-in-time snapshots for execution history. Deployments extend this: a deployment is a version snapshot plus trigger configuration and runtime metadata.

**Git analogy:**

```
  Git                          Flow Runtime
  ===                          ============

  commit (immutable)           deployment (immutable snapshot)
  branch (mutable ref)         tag (mutable pointer)
  tag (immutable ref)          version tag :v3 (locked pointer)
  HEAD                         :latest (auto-moved on deploy)
  checkout <branch>            trigger bound to tag
  git tag -f prod <commit>     TagMove(:production, v5)
  git revert                   TagMove(:production, v4)
```

### 3.6 Session Store

Manages conversational state for stateful triggers.

```
  Session Lifecycle
  =================

  Event arrives with (trigger_id=T1, chat_id=C42)
       |
       v
  Session exists for (T1, C42)?
       |                |
      NO               YES
       |                |
       v                v
  Create session    Has WAITING execution?
  (new vars, no         |            |
   memory)             NO           YES
       |                |            |
       v                v            v
  New execution    New execution   Resume execution
  (fresh start)    (session vars   (deserialize
                    carried over)   checkpoint)
```

**Session data:**
- Accumulated variables (persisted across executions within the session)
- AI memory/conversation history (for AI nodes)
- Execution history within the session
- Last activity timestamp

---

## 4. Data Model

### 4.1 Tenant DB Tables (per-workspace Turso database)

```
  Tenant DB Entity Relationships
  ==============================

  deployment ----< deployment_tag >---- trigger ----< execution
       |               |                                 |
       |               +-- env_override             session >---- execution
       |               +-- variables_override            |
       |                                            execution_checkpoint
       |
       +-- snapshot_data (nodes, edges, configs)


  Tag Resolution (on every trigger event)
  ========================================

  trigger.tag_name           deployment_tag              deployment
  ================           ==============              ==========

  "production"    ---lookup--> tag: production  ---ref--> v4 (immutable)
                               env_override: {...}
                               locked: false
```

```sql
-- Immutable deployment snapshot (like a git commit)
CREATE TABLE deployment (
    id              BLOB PRIMARY KEY,       -- ULID
    flow_id         BLOB NOT NULL,          -- Source flow (from desktop)
    version         INTEGER NOT NULL,       -- Auto-incrementing per flow
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active',  -- active, archived
    snapshot_data   BLOB NOT NULL,          -- Compressed JSON: full flow graph + node configs
    flow_variables  BLOB,                   -- Compressed JSON: variable defaults
    env_bindings    BLOB,                   -- JSON: env var name references
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by      BLOB                    -- User ID
);
CREATE INDEX idx_deployment_flow ON deployment(flow_id, version);

-- Named mutable pointer to a deployment (like a git tag/branch)
CREATE TABLE deployment_tag (
    id              BLOB PRIMARY KEY,       -- ULID
    flow_id         BLOB NOT NULL,          -- Scoped to a flow
    deployment_id   BLOB NOT NULL,          -- Currently points to this deployment
    name            TEXT NOT NULL,           -- e.g., "latest", "production", "staging", "v3"
    type            TEXT NOT NULL DEFAULT 'custom',
    -- predefined: system-managed (latest, production, staging)
    -- version:    auto-created on deploy, immutable (v1, v2, v3)
    -- custom:     user-created, mutable (canary, feature-x, etc.)
    locked          INTEGER NOT NULL DEFAULT 0,  -- If 1, tag cannot be moved (version tags)

    -- Tag-level config overrides (applied on top of deployment config)
    env_override      BLOB,                 -- JSON: env var overrides for this tag
    variables_override BLOB,                -- JSON: flow variable overrides for this tag

    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (flow_id) REFERENCES deployment(flow_id),
    FOREIGN KEY (deployment_id) REFERENCES deployment(id),
    UNIQUE(flow_id, name)                   -- One tag name per flow
);
CREATE INDEX idx_tag_flow ON deployment_tag(flow_id);
CREATE INDEX idx_tag_deployment ON deployment_tag(deployment_id);

-- Trigger configuration (binds to a tag, not a deployment)
CREATE TABLE trigger (
    id              BLOB PRIMARY KEY,       -- ULID
    flow_id         BLOB NOT NULL,          -- Which flow this trigger is for
    tag_name        TEXT NOT NULL,           -- Tag to resolve (e.g., "production", "latest")

    type            TEXT NOT NULL,           -- webhook, schedule, telegram, whatsapp, custom
    name            TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    config          BLOB NOT NULL,          -- JSON: type-specific configuration

    -- Webhook-specific (denormalized for fast lookup)
    webhook_path    TEXT UNIQUE,            -- e.g., /t/{ulid} -- unique URL path

    -- Session settings
    session_enabled INTEGER NOT NULL DEFAULT 0,
    session_ttl_sec INTEGER NOT NULL DEFAULT 3600,  -- 1 hour default

    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Execution record (records resolved tag + deployment at creation time)
CREATE TABLE execution (
    id              BLOB PRIMARY KEY,       -- ULID
    deployment_id   BLOB NOT NULL,          -- Resolved deployment (snapshot at trigger time)
    tag_name        TEXT,                   -- Which tag was resolved (for audit)
    trigger_id      BLOB,                   -- NULL if manually triggered
    session_id      BLOB,                   -- NULL if no session

    status          TEXT NOT NULL DEFAULT 'queued',
    -- queued, running, waiting, completed, failed, canceled
    error           TEXT,
    trigger_payload BLOB,                   -- Compressed JSON: normalized trigger event
    resolved_env    BLOB,                   -- Compressed JSON: merged env vars (tag + deployment + tenant)
    duration_ms     INTEGER,

    queued_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME,
    completed_at    DATETIME,

    FOREIGN KEY (deployment_id) REFERENCES deployment(id),
    FOREIGN KEY (trigger_id) REFERENCES trigger(id),
    FOREIGN KEY (session_id) REFERENCES session(id)
);
CREATE INDEX idx_execution_status ON execution(status);
CREATE INDEX idx_execution_trigger ON execution(trigger_id);
CREATE INDEX idx_execution_session ON execution(session_id);
CREATE INDEX idx_execution_tag ON execution(tag_name);

-- Tag movement audit log (append-only)
CREATE TABLE deployment_tag_history (
    id              BLOB PRIMARY KEY,       -- ULID
    tag_id          BLOB NOT NULL,
    tag_name        TEXT NOT NULL,           -- Denormalized for querying after tag deletion
    flow_id         BLOB NOT NULL,
    action          TEXT NOT NULL,           -- created, moved, deleted
    from_deployment_id BLOB,                -- NULL on create
    to_deployment_id   BLOB,                -- NULL on delete
    from_version    INTEGER,                -- Human-readable version number
    to_version      INTEGER,
    performed_by    BLOB,                   -- User ID
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (tag_id) REFERENCES deployment_tag(id)
);
CREATE INDEX idx_tag_history_tag ON deployment_tag_history(tag_id);
CREATE INDEX idx_tag_history_flow ON deployment_tag_history(flow_id);

-- Conversational session
CREATE TABLE session (
    id                BLOB PRIMARY KEY,     -- ULID
    trigger_id        BLOB NOT NULL,
    external_chat_id  TEXT NOT NULL,         -- Platform-specific chat/conversation ID
    external_user_id  TEXT,                  -- Platform-specific user ID

    variables         BLOB,                 -- Compressed JSON: accumulated session variables
    memory            BLOB,                 -- Compressed JSON: AI conversation history
    status            TEXT NOT NULL DEFAULT 'active',  -- active, expired, archived

    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at        DATETIME,

    FOREIGN KEY (trigger_id) REFERENCES trigger(id),
    UNIQUE(trigger_id, external_chat_id)
);
CREATE INDEX idx_session_trigger_chat ON session(trigger_id, external_chat_id);
CREATE INDEX idx_session_expires ON session(expires_at);

-- Execution checkpoints (for pause/resume)
CREATE TABLE execution_checkpoint (
    id              BLOB PRIMARY KEY,       -- ULID
    execution_id    BLOB NOT NULL UNIQUE,

    -- Serialized execution state
    runner_state    BLOB NOT NULL,          -- Compressed: completed nodes, pending map, variable state
    waiting_node_id BLOB NOT NULL,          -- The node that caused the pause
    waiting_for     TEXT NOT NULL,          -- "external_message", "approval", "timer"

    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME,               -- Max time to wait before auto-failing

    FOREIGN KEY (execution_id) REFERENCES execution(id)
);
CREATE INDEX idx_checkpoint_execution ON execution_checkpoint(execution_id);
```

**Note:** No `workspace_id` column in tenant DB tables — the entire database is scoped to one workspace. This eliminates an entire class of data leakage bugs.

### 4.2 Trigger Config Schemas (stored as JSON in `trigger.config`)

**Webhook:**
```json
{
    "secret": "hmac-sha256-secret",
    "allowed_ips": ["0.0.0.0/0"],
    "response_mode": "async",
    "payload_mapping": {
        "body": "trigger.body",
        "headers": "trigger.headers",
        "query": "trigger.query"
    }
}
```

**Telegram:**
```json
{
    "bot_token": "encrypted:...",
    "allowed_commands": ["/start", "/help", "/run"],
    "allowed_chat_types": ["private", "group"],
    "polling_interval_sec": 1
}
```

**WhatsApp:**
```json
{
    "phone_number_id": "123456",
    "access_token": "encrypted:...",
    "verify_token": "webhook-verify-token",
    "allowed_phone_numbers": []
}
```

**Schedule:**
```json
{
    "cron": "0 */5 * * *",
    "timezone": "UTC",
    "payload": {
        "scheduled": true,
        "run_type": "periodic"
    }
}
```

---

## 5. New Node Types

### 5.1 Trigger Start Node

Replaces `MANUAL_START` in deployed flows. Provides access to trigger event data.

```
  +---------------------+
  |   Trigger Start     |
  |                     |
  |  type: "telegram"   |
  |  payload: {...}     |
  |  chat_id: "12345"   |
  |  session.vars: {}   |
  +----------+----------+
             |
             v
       (next node)
```

**Output variables:**
```
{{Trigger.type}}              -- "webhook", "telegram", "whatsapp", "schedule"
{{Trigger.payload}}           -- Normalized event payload
{{Trigger.raw}}               -- Raw payload bytes
{{Trigger.chat_id}}           -- External chat ID (messaging triggers)
{{Trigger.user_id}}           -- External user ID (messaging triggers)
{{Trigger.timestamp}}         -- Event timestamp
{{Trigger.session.variables}} -- Accumulated session variables (if session enabled)
```

### 5.2 Wait For Input Node

Pauses execution and waits for an external event (e.g., a user reply in Telegram).

```
  (previous node)
        |
        v
  +-----+-------------------+
  |   Wait For Input        |
  |                         |
  |  prompt: "What is your  |
  |    name?"               |
  |  timeout: 5m            |
  |  validation: expr       |
  +-----+-------------------+
        |
        |  execution paused
        |  (checkpoint saved)
        |
   ~~~~ | ~~~~ time passes ~~~~
        |
        |  new message arrives
        |  (checkpoint loaded)
        |
        v
  +-----+-------------------+
  |  output:                |
  |    input.text = "Alice" |
  |    timed_out = false    |
  +-----+-------------------+
        |
        v
   (next node)
```

**Configuration:**
- `timeout` — Max time to wait (default: 5 minutes)
- `prompt` — Optional message to send before waiting (e.g., "Please reply with your choice")
- `validation` — Optional expression to validate the input before accepting

**Behavior:**
1. Optionally send a prompt message via the trigger's `SendResponse`
2. Serialize execution state to `execution_checkpoint`
3. Set execution status to `WAITING`
4. Release the worker goroutine
5. On matching inbound event: deserialize state, inject new input, resume execution
6. On timeout: fail the execution or continue with default value

**Output variables:**
```
{{WaitNode.input}}         -- The received input payload
{{WaitNode.input.text}}    -- Message text (messaging triggers)
{{WaitNode.timed_out}}     -- Boolean, true if wait expired
```

### 5.3 Send Response Node

Sends a message back through the trigger's channel (Telegram reply, WhatsApp message, webhook callback).

```
  (previous node)
        |
        v
  +-----+-----------------------+
  |   Send Response             |
  |                             |
  |  message: "Hello            |
  |   {{Trigger.user_id}}!"     |
  |  format: markdown           |
  |  reply_markup:              |
  |    [Yes] [No]               |
  +-----+-----------------------+
        |
        |  message sent via
        |  TriggerAdapter.SendResponse()
        v
  (next node)
```

**Configuration:**
- `message` — Text message (supports variable interpolation)
- `format` — plain, markdown, html (platform-dependent)
- `attachments` — Optional file/image attachments
- `reply_markup` — Optional buttons/keyboard (Telegram inline keyboard, WhatsApp buttons)

**Behavior:**
1. Resolve variables in message template
2. Call `TriggerAdapter.SendResponse` with formatted payload
3. Store sent message in execution history

---

## 6. Execution Lifecycle

### 6.1 Simple Flow (Webhook -> Process -> Done)

```
  HTTP POST /t/{path} or /t/{path}:production
       |
       v
  +----+--------+     +---------+     +-----------+     +------------+
  | Trigger     |     | Control |     | Tenant    |     |            |
  | Gateway     |---->| DB      |---->| DB        |---->| Scheduler  |
  |             |     | lookup  |     |           |     |            |
  | parse :tag  |     | route   |     | 1. resolve|     +-----+------+
  | from URL    |     |         |     |    tag    |           |
  +-------------+     +---------+     | 2. get    |           v
                                      |    deploy |    +------+-------+
                                      | 3. merge  |    |   Worker      |
                                      |    env    |    |               |
                                      | 4. create |    | 1. Load deploy|
                                      |    exec   |    | 2. Merged env |
                                      +-----------+    | 3. Build nodes|
                                                       | 4. Run flow   |
                                                       | 5. Save history|
                                                       | 6. COMPLETED  |
                                                       +--------------+
```

### 6.2 Conversational Flow (Telegram -> Reply -> Wait -> Reply -> Done)

```
  Telegram msg: "hello"                    Telegram msg: "Alice"
       |                                        |
       v                                        v
  +----+--------+                          +----+--------+
  | Gateway:    |                          | Gateway:    |
  | new session |                          | find session|
  | new exec    |                          | has WAITING |
  +------+------+                          +------+------+
         |                                        |
         v                                        v
  +------+------+                          +------+---------+
  | Worker:     |                          | Worker:        |
  | TriggerStart|                          | Load checkpoint|
  |      |      |                          | Inject "Alice" |
  |      v      |                          |      |         |
  | SendResponse|                          |      v         |
  | "What is    |                          | Process name   |
  |  your name?"|                          |      |         |
  |      |      |                          |      v         |
  |      v      |                          | SendResponse   |
  | WaitForInput|                          | "Hello, Alice!"|
  |  (pause)    |                          |      |         |
  |      |      |                          |      v         |
  |  checkpoint |                          |  COMPLETED     |
  |  WAITING    |                          +----------------+
  +-------------+
```

### 6.3 Scheduled Flow (Cron -> Run -> Notify)

```
  Cron ticker fires
       |
       v
  +----+-----------+     +----------------+     +----------------+
  | Create         |     |    Worker       |     |   (optional)   |
  | TriggerEvent   |---->|    Execute      |---->|   Send alert   |
  | payload:       |     |    flow         |     |   on failure   |
  | {scheduled: t} |     |    normally     |     |                |
  +----------------+     +----------------+     +----------------+
```

---

## 7. Multi-Tenancy

### 7.1 Isolation Model

```
  Multi-Tenant Isolation via Turso
  =================================

  +-------------------------------------------------------------------+
  |  Flow Runtime Process                                             |
  |                                                                   |
  |  +------------------+                                             |
  |  |  Control DB      |  Shared: routing, limits, tenant registry   |
  |  +------------------+                                             |
  |                                                                   |
  |  Tenant A                    Tenant B                             |
  |  +---------------------+    +---------------------+               |
  |  | Turso DB (isolated)  |    | Turso DB (isolated)  |              |
  |  |                     |    |                     |               |
  |  | deployments         |    | deployments         |               |
  |  | triggers            |    | triggers            |               |
  |  | executions          |    | executions          |               |
  |  | sessions            |    | sessions            |               |
  |  | checkpoints         |    | checkpoints         |               |
  |  +---------------------+    +---------------------+               |
  |                                                                   |
  |  Worker Pool:                                                     |
  |  [W1: Tenant A] [W2: Tenant B] [W3: Tenant A] [W4: idle]        |
  |                                                                   |
  |  Fairness: round-robin, per-tenant caps, no noisy neighbor        |
  +-------------------------------------------------------------------+
```

**Data isolation:**
- Each workspace gets a dedicated Turso database — no cross-tenant data leakage possible
- Control DB only holds routing metadata (trigger paths, tenant URLs), no business data
- Tenant DB has no `workspace_id` columns — the entire DB is the isolation boundary

**Execution isolation:**
- Per-tenant concurrency limits enforced by scheduler
- Per-tenant execution queues with fair scheduling
- Per-tenant rate limiting on trigger inbound events
- JS worker processes are not shared across tenants (fresh per execution or pooled per tenant)

**Secret isolation:**
- Trigger credentials (bot tokens, API keys) encrypted at rest in tenant DB
- Environment variables resolved per-tenant at execution time
- Secrets never appear in execution logs or node history
- Turso auth tokens stored encrypted in control DB

### 7.2 Resource Limits (per tenant, stored in `tenant_limits`)

| Resource | Default Limit | Description |
|----------|---------------|-------------|
| Deployments | 50 | Active deployments per workspace |
| Triggers | 20 | Active triggers per workspace |
| Concurrent executions | 10 | Simultaneous running flows |
| Executions per hour | 1000 | Rate limit on trigger events |
| Active sessions | 100 | Concurrent conversational sessions |
| Execution timeout | 5m | Max single execution duration |
| Wait timeout | 1h | Max pause duration for WaitForInput |
| Execution history retention | 30d | How long execution logs are kept |

### 7.3 Tenant Lifecycle

```
  Tenant Provisioning
  ====================

  User creates workspace (existing flow)
       |
       v
  +----+-----------+
  | Create Turso   |
  | database via   |
  | Turso Platform |
  | API            |
  +----+-----------+
       |
       v
  +----+-----------+
  | Run schema     |
  | migrations on  |
  | new tenant DB  |
  +----+-----------+
       |
       v
  +----+-----------+
  | Insert into    |
  | Control DB:    |
  | tenant table   |
  +----------------+


  Tenant Suspension
  ==================

  Admin suspends tenant
       |
       v
  Control DB: tenant.status = 'suspended'
       |
       v
  Trigger Gateway: reject all inbound events for tenant
       |
       v
  Scheduler: drain active executions (allow completion, no new ones)
```

---

## 8. Pause & Resume (Checkpoint System)

This is the most complex new capability. It allows a running flow to suspend execution, persist its state, release its worker, and resume later when new input arrives.

### 8.1 What Gets Checkpointed

```go
type RunnerCheckpoint struct {
    // Graph state
    CompletedNodes  []idwrap.IDWrap            // Nodes that finished
    PendingNodes    map[idwrap.IDWrap]int       // Nodes waiting for predecessors
    CurrentNodeID   idwrap.IDWrap              // The WaitForInput node

    // Variable state
    VariableMap     map[string]any             // Full variable state at pause point
    SessionVars     map[string]any             // Session-scoped variables to persist

    // Loop state (if paused inside a loop)
    IterationCtx    *IterationContext
    LoopCounters    map[idwrap.IDWrap]int      // Current iteration index per loop node

    // Metadata
    ExecutionID     idwrap.IDWrap
    DeploymentID    idwrap.IDWrap
    PausedAt        time.Time
}
```

### 8.2 Checkpoint Flow

```
  Pause (save checkpoint)              Resume (load checkpoint)
  =======================              ========================

  Node N executes                      New event for session
       |                                    |
       v                                    v
  Is WaitForInput? --NO--> continue    Load checkpoint from
       |                               tenant DB
      YES                                  |
       |                                    v
       v                               Deserialize
  Serialize state                      RunnerCheckpoint
       |                                    |
       v                                    v
  Compress (zstd)                      Rebuild FlowLocalRunner
       |                               from checkpoint state
       v                                    |
  Store in tenant DB:                       v
  execution_checkpoint                 Inject new input as
       |                               WaitForInput output vars
       v                                    |
  execution.status                          v
  = WAITING                            Resume from node N+1
       |                                    |
       v                                    v
  Release worker                       Continue execution
  goroutine                            normally
       |
       v
  [suspended]
```

### 8.3 Constraints

- Checkpoint data is compressed with zstd (matching existing `node_execution` compression)
- Checkpoints expire after `wait_timeout` — expired checkpoints cause execution to fail
- A flow can pause multiple times (e.g., multi-turn conversation)
- Each pause creates a new checkpoint, replacing the previous one
- JS worker state is **not** checkpointed — JS nodes must be re-executable (idempotent)
- HTTP connections are **not** preserved — request nodes re-execute if they appear after resume

---

## 9. API Surface

New RPC endpoints (Connect RPC, following existing patterns):

### 9.1 Deployment RPCs

```
DeploymentCreate   -- Deploy a flow (snapshot + create deployment)
DeploymentList     -- List deployments for a workspace
DeploymentGet      -- Get deployment details
DeploymentArchive  -- Archive a deployment (disables associated triggers)
```

### 9.2 Trigger RPCs

```
TriggerCreate      -- Create a trigger for a deployment
TriggerUpdate      -- Update trigger configuration
TriggerDelete      -- Delete a trigger
TriggerList        -- List triggers for a workspace
TriggerEnable      -- Enable a trigger
TriggerDisable     -- Disable a trigger
TriggerTest        -- Send a test event to a trigger (dry-run execution)
```

### 9.3 Execution RPCs

```
ExecutionList      -- List executions (filterable by trigger, deployment, status)
ExecutionGet       -- Get execution details + node results
ExecutionCancel    -- Cancel a running or waiting execution
ExecutionRetry     -- Re-run a failed execution with same trigger payload
ExecutionSync      -- Stream real-time execution status updates (SSE, extends eventstream)
```

### 9.4 Session RPCs

```
SessionList        -- List active sessions for a trigger
SessionGet         -- Get session details (variables, history)
SessionDelete      -- Terminate a session (cancels any waiting execution)
```

### 9.5 Tag RPCs

```
TagList            -- List all tags for a flow
TagGet             -- Get tag details (name, deployment, overrides)
TagCreate          -- Create a custom tag pointing to a deployment
TagMove            -- Move a tag to a different deployment (fails if locked)
TagDelete          -- Delete a custom tag (fails for predefined/version tags)
TagUpdate          -- Update tag config (env_override, variables_override)
TagHistory         -- List tag movements (audit log: who moved it, when, from/to)
```

### 9.6 Webhook Inbound (plain HTTP, not Connect RPC)

```
POST /t/{webhook_path}             -- Resolve via trigger's bound tag (default)
POST /t/{webhook_path}:production  -- Explicit tag override in URL
POST /t/{webhook_path}:v3          -- Pin to specific version via URL
POST /tg/{trigger_id}              -- Telegram webhook callback
POST /wa/{trigger_id}              -- WhatsApp webhook callback
GET  /wa/{trigger_id}              -- WhatsApp webhook verification
```

**Tag override in URL:** The `:tag` suffix in webhook URLs allows callers to override the trigger's default tag. This is useful for testing (`:staging`) or pinning to a specific version (`:v3`) without changing trigger configuration. If no tag suffix is provided, the trigger's configured `tag_name` is used.

### 9.7 RPC Request Flow with Turso

```
  Client RPC Request (e.g., ExecutionList)
       |
       v
  +----+-----------+
  | Auth middleware |
  | (BetterAuth)   |
  | extract user,  |
  | workspace_id   |
  +----+-----------+
       |
       v
  +----+-----------+
  | Control DB:    |
  | lookup tenant  |
  | get turso_url  |
  +----+-----------+
       |
       v
  +----+-----------+
  | Connection     |
  | Pool Manager:  |
  | get or create  |
  | *sql.DB for    |
  | this tenant    |
  +----+-----------+
       |
       v
  +----+-----------+
  | RPC Handler    |
  | (Fetch-Check   |
  |  -Act pattern) |
  | uses tenant DB |
  +----------------+
```

---

## 10. Deployment Modes

### 10.1 Embedded Mode (V1 target)

The runtime runs inside the existing DevTools server process. Uses local SQLite for the control DB and Turso for tenant DBs.

```
  +------------------------------------------+
  |   DevTools Server (single binary)         |
  |                                          |
  |   +-- Desktop/CLI RPC handlers           |
  |   +-- Flow Runtime (embedded)            |
  |   |   +-- Trigger Gateway               |
  |   |   +-- Scheduler                     |
  |   |   +-- Workers                       |
  |   +-- Control DB (local SQLite)          |
  |                                          |
  +-------------------+----------------------+
                      |
                      v
            +---------+----------+
            |   Turso Platform    |
            |                    |
            |  +-----+  +-----+ |
            |  |DB A |  |DB B | |
            |  +-----+  +-----+ |
            +--------------------+
```

- Simplest deployment — single binary, no extra infrastructure
- Control DB is local SQLite (fast routing lookups)
- Tenant data lives in Turso (managed, replicated, isolated)
- Good for single-user, small team, or self-hosted use

### 10.2 Standalone Mode (V2)

The runtime runs as a separate process/binary from the desktop server.

```
  +-------------------+          +-------------------+
  |  DevTools Server  |          |  Flow Runtime      |
  |  (desktop/CLI)    |   RPC   |  (standalone)      |
  |                   +--------->|                   |
  |  Local SQLite     |  deploy  |  Control DB       |
  |  (user flows)     |          |  (local SQLite)   |
  +-------------------+          +--------+----------+
                                          |
                                          v
                                 +--------+----------+
                                 |  Turso Platform    |
                                 |  (tenant DBs)     |
                                 +-------------------+
```

- Independent scaling — runtime can run on a cloud server
- Desktop app stays local, deploys to remote runtime
- Deploy via CLI: `devtools deploy --runtime-url=https://...`

### 10.3 Scaled Mode (V3)

Multiple stateless runtime instances behind a load balancer. All state in Turso.

```
                        +---------------+
                        |  Load Balancer |
                        +-------+-------+
                                |
              +-----------------+-----------------+
              |                 |                 |
              v                 v                 v
        +-----------+     +-----------+     +-----------+
        | Runtime 1 |     | Runtime 2 |     | Runtime N |
        | (stateless)|     | (stateless)|     | (stateless)|
        +-----------+     +-----------+     +-----------+
              |                 |                 |
              +-----------------+-----------------+
                                |
                        +-------v--------+
                        | Turso Platform  |
                        |                |
                        | Control DB     |
                        | Tenant DB (A)  |
                        | Tenant DB (B)  |
                        | Tenant DB (N)  |
                        +----------------+
```

- Fully stateless runtime instances — all state in Turso
- Horizontal scaling by adding more runtime instances
- WAITING executions resume on any instance (checkpoint in Turso)
- Control DB also in Turso for shared access across instances
- Edge replicas for low-latency trigger reception

---

## 11. Security Considerations

### 11.1 Trigger Authentication

| Trigger Type | Authentication Method |
|--------------|-----------------------|
| Webhook | HMAC-SHA256 signature verification on request body |
| Telegram | Bot API token verification + update secret validation |
| WhatsApp | Webhook verify token + payload signature verification |
| Schedule | Internal only — no external authentication needed |

### 11.2 Secret Management

```
  Secret Flow
  ============

  User enters bot_token in UI
       |
       v
  Encrypt with per-tenant key
       |
       v
  Store in tenant Turso DB
  (trigger.config)
       |
       v
  At execution time:
  decrypt in memory,
  never log, never
  persist in execution
  history
```

- Trigger credentials (bot tokens, API keys) are encrypted at rest using a per-tenant key
- Environment variables referenced by deployments are resolved at execution time, never stored in deployment snapshots
- Execution logs redact values of variables marked as `secret`
- Webhook URLs contain a ULID (unguessable) but should still use signature verification
- Turso auth tokens in control DB are also encrypted at rest

### 11.3 Execution Sandboxing

- JS nodes run in isolated VM contexts (existing `worker-js` sandboxing)
- HTTP request nodes respect an optional allowlist/denylist of target hosts (prevent SSRF)
- Execution timeout prevents runaway flows
- Per-tenant rate limits prevent abuse

---

## 12. Observability

### 12.1 Metrics

Key metrics to expose (Prometheus-compatible):

```
runtime_executions_total{workspace, trigger_type, status}
runtime_executions_active{workspace}
runtime_executions_queued{workspace}
runtime_execution_duration_seconds{workspace, trigger_type}
runtime_trigger_events_total{workspace, trigger_type}
runtime_trigger_errors_total{workspace, trigger_type, error_type}
runtime_sessions_active{workspace}
runtime_checkpoint_size_bytes{workspace}
runtime_turso_connections_active{workspace}
runtime_turso_query_duration_seconds{workspace}
```

### 12.2 Logging

- Execution-scoped structured logging (execution_id, workspace_id, trigger_id in every log line)
- Node-level execution tracing (reuse existing `node_execution` history)
- Trigger event audit log (inbound events, authentication results)

### 12.3 Execution History UI

```
  Desktop App: Runtime Dashboard
  ===============================

  +-------------------------------------------------------------+
  |  Flow: Auth API                                             |
  |                                                             |
  |  Tags                                                       |
  |  +-------------+----------+---------+----------------------+ |
  |  | Tag         | Version  | Type    | Last Moved           | |
  |  +-------------+----------+---------+----------------------+ |
  |  | :latest     | v5       | predef  | 2 hours ago (auto)   | |
  |  | :production | v4       | predef  | 3 days ago           | |
  |  | :staging    | v5       | predef  | 2 hours ago          | |
  |  | :canary     | v5       | custom  | 1 hour ago           | |
  |  | :v5         | v5       | version | 2 hours ago (locked) | |
  |  | :v4         | v4       | version | 3 days ago  (locked) | |
  |  +-------------+----------+---------+----------------------+ |
  |  [Move Tag]  [Create Tag]                                   |
  |                                                             |
  |  Triggers                                                    |
  |  +----------+-----------+--------+----------+---------+     |
  |  | Name     | Type      | Tag    | Status   | Events  |     |
  |  +----------+-----------+--------+----------+---------+     |
  |  | Prod WH  | webhook   | :prod  | enabled  | 342/hr  |     |
  |  | TG Bot   | telegram  | :prod  | enabled  | 28/hr   |     |
  |  | Canary   | webhook   | :canary| enabled  | 5/hr    |     |
  |  +----------+-----------+--------+----------+---------+     |
  |                                                             |
  |  Executions (filtered by :production)                        |
  |  +------+-----------+-----+----------+--------+-----------+ |
  |  | ID   | Trigger   | Tag | Status   | Deploy | Time      | |
  |  +------+-----------+-----+----------+--------+-----------+ |
  |  | e4f2 | Prod WH   | :prod| completed| v4    | 3m ago    | |
  |  | a1b3 | TG Bot    | :prod| waiting  | v4    | 10m ago   | |
  |  | c7d8 | Prod WH   | :prod| failed   | v4    | 15m ago   | |
  |  +------+-----------+-----+----------+--------+-----------+ |
  +-------------------------------------------------------------+


  Tag History (audit log)
  ========================

  +-------------------------------------------------------------+
  |  Tag: :production                                           |
  |                                                             |
  |  +----------+--------+--------+--------+-------------------+ |
  |  | Action   | From   | To     | By     | When              | |
  |  +----------+--------+--------+--------+-------------------+ |
  |  | moved    | v3     | v4     | alice  | Feb 14 09:00      | |
  |  | moved    | v2     | v3     | bob    | Feb 10 11:15      | |
  |  | created  | --     | v2     | alice  | Feb 5  16:45      | |
  |  +----------+--------+--------+--------+-------------------+ |
  +-------------------------------------------------------------+
```

The desktop app should display:
- Tag list per flow with current version, type, and move controls
- Tag history / audit log (who moved what, when)
- List of triggers with their bound tag and enable/disable toggle
- Execution timeline filterable by tag (like a CI/CD pipeline view)
- Per-execution node graph with results (reuse existing flow execution view)
- Session conversation view (for messaging triggers)

---

## 13. Implementation Phases

### Phase 1: Foundation

**Goal:** Turso multi-DB infrastructure + deploy with tags + trigger flows via webhooks.

```
  Phase 1 Scope
  ==============

  Desktop/CLI ---deploy---> Runtime ---webhook---> Execute
       |                       |                      |
       v                       v                      v
  Snapshot flow          Control DB +           FlowLocalRunner
  + auto-tag             Tenant DB (Turso)      (existing engine)
  :latest, :v1           Tags + Triggers
```

- [ ] Turso multi-DB connection pool manager
- [ ] Control DB schema + tenant provisioning
- [ ] Tenant DB schema + migrations
- [ ] `deployment` table + Deployment service (reader/writer)
- [ ] `deployment_tag` table + Tag service (reader/writer)
- [ ] `deployment_tag_history` table + audit logging
- [ ] Auto-tagging on deploy (`:latest` move + `:vN` create locked)
- [ ] Predefined tags (`:latest`, `:production`, `:staging`)
- [ ] Tag RPCs (TagList, TagGet, TagCreate, TagMove, TagDelete, TagUpdate, TagHistory)
- [ ] `trigger` table + Trigger service (webhook type only, bound to tags)
- [ ] `execution` table + Execution service (records resolved tag + deployment)
- [ ] Tag resolution on trigger event (tag -> deployment + merge env/vars)
- [ ] Tag override in webhook URL (`/t/{path}:production`)
- [ ] Webhook HTTP endpoint (`POST /t/{path}`) with control DB routing
- [ ] Execution scheduler (simple bounded goroutine pool)
- [ ] Wire up `FlowLocalRunner` to execute from deployment snapshots
- [ ] Deployment RPC endpoints
- [ ] Trigger RPC endpoints (webhook only)
- [ ] Execution RPC endpoints
- [ ] Trigger Start node (replaces MANUAL_START for deployed flows)

**Outcome:** A flow can be deployed (auto-tagged), tags can be moved/created, triggers resolve tags at runtime, webhook triggers execute against tenant Turso DB with merged config, results are stored with full tag audit trail.

### Phase 2: Messaging & Sessions

**Goal:** Telegram/WhatsApp integration with conversational state.

- [ ] Telegram trigger adapter (webhook + polling modes)
- [ ] WhatsApp Business API trigger adapter
- [ ] `session` table + Session service
- [ ] Send Response node
- [ ] Session variable accumulation
- [ ] AI Memory integration with sessions
- [ ] Session TTL and cleanup

**Outcome:** A Telegram bot can trigger flows, send replies, and maintain conversation context.

### Phase 3: Pause & Resume

**Goal:** Flows can wait for external input mid-execution.

- [ ] `execution_checkpoint` table
- [ ] Runner checkpoint serialization/deserialization
- [ ] Wait For Input node
- [ ] Resume-from-checkpoint execution path
- [ ] Checkpoint expiry and cleanup

**Outcome:** Multi-turn conversational flows (ask question -> wait -> process answer -> ask next question).

### Phase 4: Scheduling & Observability

**Goal:** Cron triggers and production observability.

- [ ] Schedule trigger adapter (cron parser + ticker)
- [ ] Prometheus metrics endpoint
- [ ] Execution history UI in desktop app
- [ ] Deployment management UI
- [ ] Trigger management UI
- [ ] Log aggregation and search

### Phase 5: Scale & Harden

**Goal:** Production-ready multi-tenant deployment.

```
  Phase 5 Target Architecture
  ============================

       Internet
          |
     +----v-----+
     |   LB     |
     +----+-----+
          |
   +------+------+------+
   |      |      |      |
   v      v      v      v
  [R1]  [R2]  [R3]  [R4]   (stateless runtime instances)
   |      |      |      |
   +------+------+------+
          |
     +----v-----+
     |  Turso   |
     |  (all    |
     |   DBs)   |
     +----------+
```

- [ ] Standalone binary mode (separate from desktop server)
- [ ] Control DB in Turso (shared across runtime instances)
- [ ] Per-tenant resource limits enforcement
- [ ] Secret encryption at rest
- [ ] SSRF protection (host allowlist/denylist)
- [ ] Rate limiting middleware
- [ ] Graceful shutdown (drain active executions)
- [ ] Health check and readiness endpoints
- [ ] Edge replica configuration for low-latency regions

---

## 14. Turso-Specific Considerations

### 14.1 Schema Migrations

```
  Migration Strategy
  ===================

  New migration available (e.g., add column to execution table)
       |
       v
  +----+-----------+
  | Control DB:    |
  | list all       |
  | active tenants |
  +----+-----------+
       |
       v
  For each tenant (can be parallelized):
       |
       +---> Open tenant DB connection
       +---> Run migration
       +---> Update migration version in control DB
       +---> Close connection (or return to pool)
       |
       v
  All tenants migrated
```

- Migrations run per-tenant DB — allows canary rollouts (migrate a few tenants first)
- Control DB tracks migration version per tenant
- Failed migrations can be retried per-tenant without affecting others
- New tenants are provisioned with the latest schema

### 14.2 Turso Platform API Usage

```go
// Tenant provisioning via Turso Platform API
type TursoProvisioner struct {
    orgName  string
    apiToken string
    group    string  // Turso database group (determines primary region)
}

func (p *TursoProvisioner) CreateTenantDB(ctx context.Context, tenantID string) (TursoDBInfo, error) {
    // 1. Create database via Turso Platform API
    //    POST https://api.turso.tech/v1/organizations/{org}/databases
    //    { "name": "tenant-{tenantID}", "group": "{group}" }

    // 2. Create auth token for this database
    //    POST https://api.turso.tech/v1/organizations/{org}/databases/{name}/auth/tokens

    // 3. Run schema migrations on new database

    // 4. Return connection info
    return TursoDBInfo{
        URL:       "libsql://tenant-{tenantID}-{org}.turso.io",
        AuthToken: token,
    }, nil
}
```

### 14.3 Local Development

For local development and testing, the multi-DB pattern works with local SQLite files instead of Turso:

```
  Production                    Local Development
  ==========                    =================

  Control DB: Turso             Control DB: ./data/control.db
  Tenant DBs: Turso             Tenant DBs: ./data/tenant-{id}.db

  Connection string:            Connection string:
  libsql://tenant-xxx.turso.io  file:./data/tenant-xxx.db
```

The `devtoolsdb` package already abstracts the database driver — the same sqlc queries work against both local SQLite and remote Turso via the `libsql` Go driver.

### 14.4 Backup & Recovery

- Turso provides automatic daily backups per database
- Point-in-time recovery available per tenant
- Tenant deletion = archive Turso DB (soft delete, recoverable for 30 days)
- No cross-tenant backup coordination needed

---

## 15. Open Questions

1. **Deployment artifact format:** Store as compressed JSON blob (simple) or as normalized rows mirroring the flow tables (queryable but more complex)?

2. **JS worker lifecycle:** One worker per execution (safe, slow cold start) vs pooled per tenant (faster, needs careful isolation) vs shared pool (fastest, weakest isolation)?

3. **Webhook response mode:** Should webhook triggers optionally wait for execution to complete and return the result as the HTTP response body? (Synchronous mode vs fire-and-forget.)

4. **Existing flow compatibility:** Should deployed flows use the exact same node types, or should we introduce deployment-specific variants (e.g., TriggerStart vs ManualStart)?

5. **CLI deploy workflow:** `devtools deploy --flow-id=xxx --runtime-url=yyy` or a declarative config file (`deploy.yaml`)? Should CLI support tag operations like `devtools tag move :production v5`?

6. **Secrets management:** Use the existing BetterAuth system's encryption, a dedicated secrets manager (Vault), or encrypted env files?

7. **Turso group/region strategy:** One group per region (multi-region) or single group (simpler, higher latency for distant tenants)?

8. **Connection pool sizing:** How many concurrent Turso connections per tenant? LRU eviction threshold for idle tenant connections?

9. **Tag garbage collection:** Should old deployments with no tags pointing to them be auto-archived after N days? Or keep all deployment history forever?

10. **Tag permissions:** Should moving `:production` require a different permission level than moving `:staging` or custom tags? Role-based tag access control?

11. **Tag-level rate limits:** Should different tags have independent rate limits? (e.g., `:canary` limited to 10 req/hr, `:production` at 1000 req/hr)
