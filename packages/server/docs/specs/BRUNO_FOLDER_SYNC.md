# Bruno Import & YAML Folder Sync Plan

## Overview

This document describes the plan to add:

1. **Bruno Import** — Parse `.bru` collections and convert them into DevTools' data model
2. **DevTools YAML Folder Sync** — Bidirectional filesystem sync using DevTools' own YAML format, inspired by Bruno's folder sync architecture
3. **CLI Runner Integration** — Execute imported/synced collections from the CLI

The goal is to support Bruno users migrating to DevTools, while building a first-class filesystem-based workflow using DevTools' own YAML format for git-friendly, local-first API development.

---

## Part 1: Bruno Import

### 1.1 Architecture Fit

Bruno import follows the exact same pattern as the existing HAR, Postman, and curl importers:

```
.bru files on disk
    → tbruno.ConvertBrunoCollection()
        → BrunoResolved (mhttp.HTTP, mfile.File, etc.)
            → importer.RunImport() + services.Create()
                → SQLite DB
```

| Layer | Location | Pattern |
|-------|----------|---------|
| CLI Command | `apps/cli/cmd/import.go` | Add `importBrunoCmd` |
| Translator | `packages/server/pkg/translate/tbruno/` | New package |
| BRU Parser | `packages/server/pkg/translate/tbruno/bruparser/` | Hand-written recursive descent |
| Importer | `apps/cli/internal/importer/` | Existing `RunImport()` callback |

### 1.2 BRU Parser (`tbruno/bruparser/`)

The `.bru` format has three block types that need a hand-written parser (no external dependency needed):

**Dictionary blocks** (key-value):
```bru
headers {
  content-type: application/json
  ~disabled-header: value
}
```

**Text blocks** (freeform content):
```bru
body:json {
  {
    "key": "value"
  }
}
```

**List blocks** (arrays inside dictionary blocks):
```bru
meta {
  tags: [
    regression
    smoke
  ]
}
```

#### Files

```
packages/server/pkg/translate/tbruno/bruparser/
├── parser.go          # .bru → BruFile struct (recursive descent)
├── serializer.go      # BruFile → .bru string (for round-trip tests)
├── types.go           # BruFile, Block, KeyValue types
└── parser_test.go     # Test with real .bru samples
```

#### Core Types

```go
package bruparser

// BruFile represents a fully parsed .bru file
type BruFile struct {
    Meta          Meta
    HTTP          *HTTPBlock       // get/post/put/delete/patch/options/head/connect/trace block
    GraphQL       *GraphQLBlock
    Params        *ParamsBlock     // params:query, params:path
    Headers       []KeyValue
    Auth          *AuthBlock       // auth:bearer, auth:basic, auth:apikey, etc.
    Body          *BodyBlock       // body:json, body:xml, body:text, body:form-urlencoded, body:multipart-form, body:graphql
    Script        *ScriptBlock     // script:pre-request, script:post-response
    Tests         string           // freeform JS
    Vars          *VarsBlock       // vars:pre-request, vars:post-response
    Assertions    []AssertEntry    // assert block
    Docs          string           // docs block
}

type Meta struct {
    Name string
    Type string // "http", "graphql", "grpc"
    Seq  int
    Tags []string
}

type KeyValue struct {
    Key     string
    Value   string
    Enabled bool // false if prefixed with ~
}

type AssertEntry struct {
    Expression string // e.g. "res.status eq 200"
    Enabled    bool
}
```

#### Parser Implementation Notes

- **Line-based parsing**: Read line by line, detect block openings (`blockname {`), track nesting depth
- **Dictionary vs Text blocks**: Dictionary blocks have `key: value` lines; text blocks (body, script, tests, docs) have freeform content with 2-space indent
- **Disabled items**: `~` prefix means disabled (`Enabled: false`)
- **Multiline values**: `'''...'''` syntax with optional `@contentType()` annotation
- **Quoted keys**: Keys can be quoted with single quotes for special characters
- **Assert delimiter**: Assert keys use `: ` (space after colon) as the delimiter since keys can contain colons

### 1.3 Bruno Collection Reader (`tbruno/`)

Reads the full Bruno collection directory structure and converts to DevTools models.

#### Files

```
packages/server/pkg/translate/tbruno/
├── bruparser/           # .bru parser (above)
├── converter.go         # Main conversion: directory → BrunoResolved
├── converter_test.go    # Tests with sample collections
├── types.go             # BrunoResolved, ConvertOptions
├── collection.go        # bruno.json / opencollection.yml detection + parsing
├── environment.go       # Environment .bru/.yml → menv conversion
└── testdata/            # Sample Bruno collections for tests
    ├── bru-format/
    │   ├── bruno.json
    │   ├── collection.bru
    │   ├── environments/
    │   │   ├── dev.bru
    │   │   └── prod.bru
    │   ├── users/
    │   │   ├── folder.bru
    │   │   ├── get-users.bru
    │   │   └── create-user.bru
    │   └── auth/
    │       ├── folder.bru
    │       └── login.bru
    └── yml-format/
        ├── opencollection.yml
        └── ...
```

#### Core Types

```go
package tbruno

// BrunoResolved contains all entities extracted from a Bruno collection,
// following the same pattern as tpostmanv2.PostmanResolvedV2 and harv2.HARResolved.
type BrunoResolved struct {
    // HTTP entities
    HTTPRequests       []mhttp.HTTP
    HTTPHeaders        []mhttp.HTTPHeader
    HTTPSearchParams   []mhttp.HTTPSearchParam
    HTTPBodyForms      []mhttp.HTTPBodyForm
    HTTPBodyUrlencoded []mhttp.HTTPBodyUrlencoded
    HTTPBodyRaw        []mhttp.HTTPBodyRaw
    HTTPAsserts        []mhttp.HTTPAssert

    // File hierarchy (folders + request files)
    Files []mfile.File

    // Environments
    Environments    []menv.Env
    EnvironmentVars []menv.Variable

    // Flow (optional — one flow with sequential request nodes)
    Flow         *mflow.Flow
    FlowNodes    []mflow.Node
    RequestNodes []mflow.NodeRequest
    FlowEdges    []mflow.Edge
}

type ConvertOptions struct {
    WorkspaceID   idwrap.IDWrap
    FolderID      *idwrap.IDWrap  // Optional parent folder to import into
    CollectionName string         // Override collection name
    CreateFlow     bool           // Whether to generate a flow from the collection
    GenerateFiles  bool           // Whether to create File entries for hierarchy
}

// ConvertBrunoCollection reads a Bruno collection directory and returns resolved entities.
func ConvertBrunoCollection(collectionPath string, opts ConvertOptions) (*BrunoResolved, error)
```

#### Conversion Logic

1. **Detect format**: Check for `bruno.json` (BRU format) or `opencollection.yml` (YML format)
2. **Parse config**: Read collection name, version, ignore patterns from config file
3. **Walk directory tree** (depth-first):
   - Skip ignored paths (`node_modules`, `.git`, dotenv files)
   - For each directory: create `mfile.File` with `ContentTypeFolder`
   - For each `.bru` file: parse with `bruparser.Parse()`, convert to `mhttp.HTTP` + children
   - For `folder.bru`: extract folder-level metadata (auth, headers) — apply as defaults to child requests
   - For `collection.bru`: extract collection-level defaults
   - For `environments/*.bru`: convert to `menv.Env` + `menv.Variable`
4. **Map to DevTools models**:
   - BRU `meta.name` → `mhttp.HTTP.Name`
   - BRU method block (`get`, `post`, etc.) → `mhttp.HTTP.Method` + `mhttp.HTTP.Url`
   - BRU `headers {}` → `[]mhttp.HTTPHeader`
   - BRU `params:query {}` → `[]mhttp.HTTPSearchParam`
   - BRU `body:json {}` → `mhttp.HTTPBodyRaw` with `BodyKind = Raw`
   - BRU `body:form-urlencoded {}` → `[]mhttp.HTTPBodyUrlencoded`
   - BRU `body:multipart-form {}` → `[]mhttp.HTTPBodyForm`
   - BRU `assert {}` → `[]mhttp.HTTPAssert`
   - BRU `meta.seq` → `mfile.File.Order`
   - Directory nesting → `mfile.File.ParentID` hierarchy
5. **Generate flow** (optional): Create a linear flow with request nodes ordered by `seq`

#### Mapping Table: Bruno → DevTools

| Bruno (.bru) | DevTools Model | Notes |
|---|---|---|
| `meta.name` | `mhttp.HTTP.Name` | |
| `meta.seq` | `mfile.File.Order` | Float64 ordering |
| `meta.type` | Determines request type | "http", "graphql" |
| `get/post/put/...` block | `mhttp.HTTP.Method` | Block name = method |
| `url` in method block | `mhttp.HTTP.Url` | |
| `body` in method block | `mhttp.HTTP.BodyKind` | "json"→Raw, "form"→FormData, etc. |
| `headers {}` | `[]mhttp.HTTPHeader` | `~` prefix → `Enabled: false` |
| `params:query {}` | `[]mhttp.HTTPSearchParam` | `~` prefix → `Enabled: false` |
| `params:path {}` | Embedded in URL | DevTools uses URL template vars |
| `body:json {}` | `mhttp.HTTPBodyRaw` | `BodyKind: Raw` |
| `body:xml {}` | `mhttp.HTTPBodyRaw` | `BodyKind: Raw` |
| `body:text {}` | `mhttp.HTTPBodyRaw` | `BodyKind: Raw` |
| `body:form-urlencoded {}` | `[]mhttp.HTTPBodyUrlencoded` | |
| `body:multipart-form {}` | `[]mhttp.HTTPBodyForm` | |
| `auth:bearer` | `mhttp.HTTPHeader` | Converted to Authorization header |
| `auth:basic` | `mhttp.HTTPHeader` | Converted to Authorization header |
| `auth:apikey` | `mhttp.HTTPHeader` | Key/value in header or query param |
| `assert {}` | `[]mhttp.HTTPAssert` | Expression syntax may differ |
| `script:pre-request {}` | Not imported (log warning) | DevTools uses JS nodes in flows |
| `script:post-response {}` | Not imported (log warning) | DevTools uses JS nodes in flows |
| `tests {}` | Not imported (log warning) | DevTools uses assert system |
| `vars:pre-request {}` | Not imported (log warning) | DevTools uses flow variables |
| `docs {}` | `mhttp.HTTP.Description` | |
| Directory structure | `mfile.File` hierarchy | Folder nesting preserved |
| `folder.bru` | Folder metadata | Auth/headers applied to children |
| `environments/*.bru` | `menv.Env` + `menv.Variable` | |
| `bruno.json` | Collection config | Name used as root folder name |

### 1.4 CLI Command

Add `import bruno <directory>` to the existing import command tree:

```go
// apps/cli/cmd/import.go — add alongside importCurlCmd, importPostmanCmd, importHarCmd

var importBrunoCmd = &cobra.Command{
    Use:   "bruno [directory]",
    Short: "Import a Bruno collection directory",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        collectionPath := args[0]
        return importer.RunImport(ctx, logger, workspaceID, folderID,
            func(ctx context.Context, services *common.Services, wsID idwrap.IDWrap, folderIDPtr *idwrap.IDWrap) error {
                resolved, err := tbruno.ConvertBrunoCollection(collectionPath, tbruno.ConvertOptions{
                    WorkspaceID:   wsID,
                    FolderID:      folderIDPtr,
                    GenerateFiles: true,
                    CreateFlow:    false,
                })
                if err != nil {
                    return err
                }
                // Save all resolved entities via services (same pattern as Postman/HAR)
                return saveResolved(ctx, services, resolved)
            },
        )
    },
}
```

### 1.5 Testing Strategy

- **Parser unit tests**: Port/create test cases from real `.bru` files — parse, verify struct output
- **Round-trip tests**: Parse `.bru` → serialize back → parse again, verify equality
- **Converter tests**: Use `testdata/` sample collections → verify `BrunoResolved` output
- **Integration test**: Import sample collection into in-memory SQLite → verify all entities created correctly

---

## Part 2: DevTools YAML Folder Sync

This is the core feature — a bidirectional filesystem sync that uses DevTools' own YAML format, inspired by Bruno's chokidar-based sync but built for Go.

### 2.1 Design Goals

1. **Git-friendly**: YAML files that diff and merge cleanly
2. **Human-readable**: Developers can edit requests in their editor/IDE
3. **Bidirectional**: Changes in the UI persist to disk; changes on disk reflect in the UI
4. **Compatible**: Similar UX to Bruno's folder sync (users understand the concept)
5. **DevTools-native**: Uses DevTools' own YAML format, not `.bru` format

### 2.2 DevTools Collection YAML Format

Inspired by `yamlflowsimplev2` but simplified for individual request files (not full flows).

#### Directory Structure

```
my-collection/
├── devtools.yaml              # Collection config (name, version, settings)
├── environments/
│   ├── dev.yaml
│   └── prod.yaml
├── users/
│   ├── _folder.yaml           # Folder metadata (optional)
│   ├── get-users.yaml         # Individual request
│   └── create-user.yaml
├── auth/
│   ├── _folder.yaml
│   └── login.yaml
└── flows/                     # Optional: flow definitions
    └── smoke-test.yaml        # Uses yamlflowsimplev2 format
```

#### Collection Config (`devtools.yaml`)

```yaml
version: "1"
name: My API Collection
settings:
  base_url: "https://api.example.com"
  timeout: 30
```

#### Request File (`get-users.yaml`)

```yaml
name: Get Users
method: GET
url: "{{base_url}}/users"
description: "Fetch all users with optional pagination"

headers:
  - name: Authorization
    value: "Bearer {{token}}"
  - name: Accept
    value: application/json
  - name: X-Debug
    value: "true"
    enabled: false

query_params:
  - name: page
    value: "1"
  - name: limit
    value: "10"
    enabled: false

body:
  type: none  # none | raw | form-data | urlencoded
```

#### Request with JSON body (`create-user.yaml`)

```yaml
name: Create User
method: POST
url: "{{base_url}}/users"

headers:
  - name: Content-Type
    value: application/json

body:
  type: raw
  content: |
    {
      "name": "John Doe",
      "email": "john@example.com"
    }

assertions:
  - "res.status eq 201"
  - "res.body.id neq null"
```

#### Request with form body (`upload.yaml`)

```yaml
name: Upload File
method: POST
url: "{{base_url}}/upload"

body:
  type: form-data
  fields:
    - name: file
      value: "@./fixtures/test.png"
      description: "File to upload"
    - name: description
      value: "Test upload"
```

#### Environment (`environments/dev.yaml`)

```yaml
name: Development
variables:
  - name: base_url
    value: "http://localhost:3000"
  - name: token
    value: "dev-token-123"
    secret: true
```

#### Folder metadata (`_folder.yaml`)

```yaml
name: Users API
order: 1
description: "User management endpoints"

# Folder-level defaults (applied to all requests in this folder)
defaults:
  headers:
    - name: X-Api-Version
      value: "v2"
```

#### Flow file (`flows/smoke-test.yaml`)

```yaml
# Uses existing yamlflowsimplev2 format exactly
name: Smoke Test
steps:
  - request:
      name: Login
      method: POST
      url: "{{base_url}}/auth/login"
      body:
        type: raw
        content: '{"email": "test@example.com", "password": "test"}'
  - request:
      name: Get Profile
      depends_on: [Login]
      method: GET
      url: "{{base_url}}/users/me"
      headers:
        Authorization: "Bearer {{Login.response.body.token}}"
```

### 2.3 YAML Serializer/Deserializer (`tyamlcollection/`)

New package for the collection YAML format:

```
packages/server/pkg/translate/tyamlcollection/
├── types.go           # YAML struct definitions with yaml tags
├── parser.go          # YAML file → DevTools models
├── serializer.go      # DevTools models → YAML files
├── collection.go      # devtools.yaml config handling
├── request.go         # Single request YAML ↔ mhttp conversion
├── environment.go     # Environment YAML ↔ menv conversion
├── folder.go          # _folder.yaml ↔ folder metadata
└── parser_test.go     # Round-trip tests
```

#### Core Types

```go
package tyamlcollection

// CollectionConfig represents devtools.yaml
type CollectionConfig struct {
    Version  string            `yaml:"version"`
    Name     string            `yaml:"name"`
    Settings *CollectionSettings `yaml:"settings,omitempty"`
}

type CollectionSettings struct {
    BaseURL string `yaml:"base_url,omitempty"`
    Timeout int    `yaml:"timeout,omitempty"`
}

// RequestFile represents a single .yaml request file
type RequestFile struct {
    Name        string           `yaml:"name"`
    Method      string           `yaml:"method"`
    URL         string           `yaml:"url"`
    Description string           `yaml:"description,omitempty"`
    Headers     []HeaderEntry    `yaml:"headers,omitempty"`
    QueryParams []HeaderEntry    `yaml:"query_params,omitempty"`
    Body        *BodyDef         `yaml:"body,omitempty"`
    Assertions  []AssertionEntry `yaml:"assertions,omitempty"`
}

type HeaderEntry struct {
    Name        string `yaml:"name"`
    Value       string `yaml:"value"`
    Enabled     *bool  `yaml:"enabled,omitempty"`     // Default: true
    Description string `yaml:"description,omitempty"`
}

type BodyDef struct {
    Type       string        `yaml:"type"`                  // "none", "raw", "form-data", "urlencoded"
    Content    string        `yaml:"content,omitempty"`     // For raw bodies
    Fields     []HeaderEntry `yaml:"fields,omitempty"`      // For form-data / urlencoded
}

type AssertionEntry struct {
    Value       string `yaml:"value,omitempty"`
    Enabled     *bool  `yaml:"enabled,omitempty"`
    Description string `yaml:"description,omitempty"`
}

// Simplified assertion: can be just a string
// Custom unmarshaler handles both "res.status eq 200" and {value: ..., enabled: false}

type EnvironmentFile struct {
    Name      string         `yaml:"name"`
    Variables []EnvVariable  `yaml:"variables"`
}

type EnvVariable struct {
    Name   string `yaml:"name"`
    Value  string `yaml:"value"`
    Secret bool   `yaml:"secret,omitempty"`
}

type FolderMeta struct {
    Name        string `yaml:"name,omitempty"`
    Order       int    `yaml:"order,omitempty"`
    Description string `yaml:"description,omitempty"`
    Defaults    *FolderDefaults `yaml:"defaults,omitempty"`
}

type FolderDefaults struct {
    Headers []HeaderEntry `yaml:"headers,omitempty"`
}
```

#### Conversion Functions

```go
// ParseRequestFile reads a .yaml request file and returns DevTools model entities.
func ParseRequestFile(data []byte, opts ParseOptions) (*RequestResolved, error)

// SerializeRequest converts DevTools model entities to YAML bytes for a single request.
func SerializeRequest(http mhttp.HTTP, headers []mhttp.HTTPHeader,
    params []mhttp.HTTPSearchParam, bodyRaw *mhttp.HTTPBodyRaw,
    bodyForms []mhttp.HTTPBodyForm, bodyUrlencoded []mhttp.HTTPBodyUrlencoded,
    asserts []mhttp.HTTPAssert) ([]byte, error)

// ReadCollection reads a full collection directory into DevTools models.
func ReadCollection(collectionPath string, opts ConvertOptions) (*CollectionResolved, error)

// WriteCollection writes a full workspace/collection to disk as YAML files.
func WriteCollection(collectionPath string, bundle *ioworkspace.WorkspaceBundle) error

// WriteRequest writes a single request to a YAML file on disk.
func WriteRequest(filePath string, http mhttp.HTTP, children RequestChildren) error
```

### 2.4 File Watcher (`packages/server/pkg/foldersync/`)

Replaces Bruno's chokidar with Go's `fsnotify`. This is a new package in the server.

```
packages/server/pkg/foldersync/
├── watcher.go         # Core CollectionWatcher using fsnotify
├── debouncer.go       # Write stabilization (coalesce rapid events)
├── filter.go          # Ignore patterns (.git, node_modules, etc.)
├── sync.go            # Bidirectional sync coordinator
├── types.go           # Event types, config
└── watcher_test.go    # Integration tests with temp directories
```

#### Watcher

```go
package foldersync

import "github.com/fsnotify/fsnotify"

type EventType int
const (
    EventFileCreated EventType = iota
    EventFileChanged
    EventFileDeleted
    EventDirCreated
    EventDirDeleted
)

type WatchEvent struct {
    Type     EventType
    Path     string        // Absolute path
    RelPath  string        // Relative to collection root
}

// CollectionWatcher watches a collection directory for filesystem changes.
type CollectionWatcher struct {
    collectionPath string
    ignorePatterns []string
    watcher        *fsnotify.Watcher
    events         chan WatchEvent
    debouncer      *Debouncer
    selfWrites     *SelfWriteTracker // Suppress events from our own writes
}

func NewCollectionWatcher(collectionPath string, opts WatcherOptions) (*CollectionWatcher, error)
func (w *CollectionWatcher) Start(ctx context.Context) error
func (w *CollectionWatcher) Events() <-chan WatchEvent
func (w *CollectionWatcher) Stop() error
```

#### Debouncer

```go
// Debouncer coalesces rapid filesystem events for the same path.
// fsnotify fires multiple events for a single write operation.
// We wait for stabilityThreshold (80ms) of no events before emitting.
type Debouncer struct {
    stabilityThreshold time.Duration // 80ms (matching Bruno)
    timers             map[string]*time.Timer
    mu                 sync.Mutex
    output             chan WatchEvent
}

func NewDebouncer(threshold time.Duration) *Debouncer
func (d *Debouncer) Add(event WatchEvent)
func (d *Debouncer) Events() <-chan WatchEvent
```

#### Self-Write Tracker

```go
// SelfWriteTracker prevents infinite loops when the sync engine writes a file
// and the watcher detects the change.
type SelfWriteTracker struct {
    mu       sync.Mutex
    writes   map[string]time.Time // path → write timestamp
    lifetime time.Duration        // How long to suppress (e.g., 2s)
}

func (t *SelfWriteTracker) MarkWrite(path string)
func (t *SelfWriteTracker) IsSelfWrite(path string) bool
```

#### Key Implementation Notes

- **Recursive watching**: `fsnotify` doesn't watch subdirectories automatically. Walk the tree on start and add watchers for new directories on `EventDirCreated`
- **Initial scan**: On start, walk the tree and emit `EventFileCreated` for all existing `.yaml` files (like chokidar's `ignoreInitial: false`)
- **Ignore patterns**: Filter `.git`, `node_modules`, dotfiles, non-`.yaml` files
- **WSL compatibility**: Detect WSL paths and use polling mode if needed
- **Max depth**: Limit to 20 levels (matching Bruno)

### 2.5 Sync Coordinator (`foldersync/sync.go`)

The central orchestrator that bridges the filesystem watcher with the DevTools database/services.

```go
// SyncCoordinator manages bidirectional sync between filesystem and database.
type SyncCoordinator struct {
    collectionPath string
    workspaceID    idwrap.IDWrap
    format         tyamlcollection.CollectionConfig

    // Filesystem → Database
    watcher     *CollectionWatcher
    parser      *tyamlcollection.Parser

    // Database → Filesystem
    selfWrites  *SelfWriteTracker
    serializer  *tyamlcollection.Serializer
    autosaver   *AutoSaver

    // State mapping
    mu          sync.RWMutex
    pathToID    map[string]idwrap.IDWrap // filepath → entity ID (UID preservation)
    idToPath    map[idwrap.IDWrap]string // entity ID → filepath (reverse mapping)

    // Services
    services    *common.Services

    // Event publishing for real-time sync to UI
    publisher   *eventstream.Publisher
}

func NewSyncCoordinator(opts SyncOptions) (*SyncCoordinator, error)
func (s *SyncCoordinator) Start(ctx context.Context) error
func (s *SyncCoordinator) Stop() error
```

#### Disk → Database Flow

```
Filesystem event (watcher)
    → Debounce (80ms stabilization)
    → Check self-write tracker (skip if we wrote it)
    → Parse YAML file
    → Look up existing entity by path→ID mapping
    → If new file: Create HTTP + File + children via services
    → If changed file: Update HTTP + children via services
    → If deleted file: Delete HTTP + File via services
    → Publish events to eventstream (UI updates in real-time)
```

#### Database → Disk Flow

```
User edits in UI
    → RPC handler persists to database
    → Eventstream publishes change event
    → SyncCoordinator receives event via subscription
    → AutoSaver debounces (500ms, matching Bruno)
    → Serialize entity to YAML
    → Mark path in self-write tracker
    → Write YAML file to disk
    → Watcher detects change → self-write tracker suppresses
```

#### AutoSaver

```go
// AutoSaver handles debounced persistence from database changes to disk.
// Matches Bruno's 500ms debounce behavior.
type AutoSaver struct {
    delay   time.Duration // 500ms
    timers  map[idwrap.IDWrap]*time.Timer
    mu      sync.Mutex
    writeFn func(entityID idwrap.IDWrap) error
}

func (a *AutoSaver) ScheduleSave(entityID idwrap.IDWrap)
func (a *AutoSaver) Flush() // Force all pending saves (for graceful shutdown)
```

### 2.6 Integration with Existing Architecture

#### RPC Layer Integration

New RPC endpoints for folder sync management:

```
// In packages/spec — new TypeSpec definitions

@route("/folder-sync")
interface FolderSync {
    // Open a collection folder for sync
    @post open(workspaceId: string, collectionPath: string): FolderSyncStatus;

    // Close/stop syncing a collection
    @post close(workspaceId: string): void;

    // Get current sync status
    @get status(workspaceId: string): FolderSyncStatus;

    // Export workspace as a collection folder
    @post export(workspaceId: string, outputPath: string): void;
}
```

#### Server Startup

The folder sync coordinator starts/stops with the server:

```go
// packages/server/internal/app/app.go or similar

// On workspace open with folder sync enabled:
coordinator := foldersync.NewSyncCoordinator(foldersync.SyncOptions{
    CollectionPath: "/path/to/collection",
    WorkspaceID:    workspaceID,
    Services:       services,
    Publisher:      publisher,
})
coordinator.Start(ctx)

// On workspace close:
coordinator.Stop()
```

#### Eventstream Integration

Subscribe to entity change events for Database → Disk sync:

```go
// Subscribe to HTTP entity changes for this workspace
sub := publisher.Subscribe(eventstream.Topic{
    WorkspaceID: workspaceID,
    EntityTypes: []eventstream.EntityType{
        eventstream.EntityHTTP,
        eventstream.EntityHTTPHeader,
        eventstream.EntityHTTPSearchParam,
        eventstream.EntityHTTPBodyRaw,
        eventstream.EntityHTTPBodyForm,
        eventstream.EntityHTTPBodyUrlencoded,
        eventstream.EntityHTTPAssert,
        eventstream.EntityFile,
    },
})

for event := range sub.Events() {
    switch event.Op {
    case eventstream.OpInsert, eventstream.OpUpdate:
        autosaver.ScheduleSave(event.ID)
    case eventstream.OpDelete:
        deleteFileFromDisk(event.ID)
    }
}
```

### 2.7 Safety Mechanisms

| Mechanism | Implementation | Matches Bruno? |
|---|---|---|
| Path validation | `filepath.Rel()` must not escape collection root | Yes |
| Filename sanitization | Strip invalid chars, truncate at 255 | Yes |
| Write stabilization | 80ms debounce on watcher events | Yes (stabilityThreshold: 80) |
| Autosave debounce | 500ms debounce on UI changes | Yes |
| Self-write suppression | Track recently-written paths (2s window) | Improved (Bruno re-parses) |
| Atomic writes | Write to temp file, then `os.Rename()` | Improved (Bruno doesn't) |
| UID preservation | `pathToID` map persists across re-parses | Yes |
| Conflict resolution | Disk wins over in-memory (matching Bruno) | Yes |
| Large file handling | Skip files >5MB, warn on collections >20MB | Yes |
| Cross-platform paths | `filepath.Clean/Rel/Join` consistently | Yes |
| Line endings | Handle `\r\n` and `\n` | Yes |

---

## Part 3: CLI Runner Integration

### 3.1 Run from Collection Folder

Add a CLI command to run requests directly from a synced collection folder:

```
devtools run ./my-collection                    # Run all requests sequentially
devtools run ./my-collection/users/get-users.yaml  # Run single request
devtools run ./my-collection --env dev           # With environment
devtools run ./my-collection/flows/smoke-test.yaml # Run a flow
```

This reuses the existing `apps/cli/internal/runner/` infrastructure:

1. Read collection folder → `tyamlcollection.ReadCollection()`
2. Create in-memory SQLite → populate with resolved entities
3. Execute via existing `runner.RunFlow()` or direct HTTP execution
4. Report results

### 3.2 CLI Commands

```go
// apps/cli/cmd/run.go — extend existing run command

var runCollectionCmd = &cobra.Command{
    Use:   "collection [directory-or-file]",
    Short: "Run requests from a DevTools collection folder",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Read collection
        // 2. Import into in-memory DB
        // 3. Execute via runner
        // 4. Report results
    },
}
```

---

## Part 4: Implementation Phases

### Phase 1: BRU Parser (Foundation)

**Scope**: Hand-written `.bru` parser + serializer with full test coverage.

**Files**:
- `packages/server/pkg/translate/tbruno/bruparser/types.go`
- `packages/server/pkg/translate/tbruno/bruparser/parser.go`
- `packages/server/pkg/translate/tbruno/bruparser/serializer.go`
- `packages/server/pkg/translate/tbruno/bruparser/parser_test.go`

**Dependencies**: None (pure Go, no external libs)

**Testing**: Parse real `.bru` files, verify struct output, round-trip test

**Estimate**: Self-contained, no cross-cutting concerns

### Phase 2: Bruno Collection Converter

**Scope**: Walk Bruno collection directory → produce `BrunoResolved` with all DevTools models.

**Files**:
- `packages/server/pkg/translate/tbruno/types.go`
- `packages/server/pkg/translate/tbruno/converter.go`
- `packages/server/pkg/translate/tbruno/collection.go`
- `packages/server/pkg/translate/tbruno/environment.go`
- `packages/server/pkg/translate/tbruno/converter_test.go`
- `packages/server/pkg/translate/tbruno/testdata/` (sample collections)

**Dependencies**: Phase 1 (BRU parser), existing models (`mhttp`, `mfile`, `menv`)

**Testing**: Convert sample Bruno collections, verify all entities

### Phase 3: CLI Import Command

**Scope**: Add `import bruno <dir>` CLI command.

**Files**:
- `apps/cli/cmd/import.go` (add `importBrunoCmd`)

**Dependencies**: Phase 2 (converter), existing importer infrastructure

**Testing**: End-to-end import into in-memory SQLite

### Phase 4: DevTools YAML Collection Format

**Scope**: Define and implement DevTools' own YAML format for individual request files.

**Files**:
- `packages/server/pkg/translate/tyamlcollection/types.go`
- `packages/server/pkg/translate/tyamlcollection/parser.go`
- `packages/server/pkg/translate/tyamlcollection/serializer.go`
- `packages/server/pkg/translate/tyamlcollection/request.go`
- `packages/server/pkg/translate/tyamlcollection/environment.go`
- `packages/server/pkg/translate/tyamlcollection/folder.go`
- `packages/server/pkg/translate/tyamlcollection/collection.go`
- `packages/server/pkg/translate/tyamlcollection/parser_test.go`

**Dependencies**: Existing models, `gopkg.in/yaml.v3`

**Testing**: Round-trip tests (parse → serialize → parse), verify equivalence

### Phase 5: File Watcher

**Scope**: `fsnotify`-based watcher with debouncing, filtering, self-write tracking.

**Files**:
- `packages/server/pkg/foldersync/watcher.go`
- `packages/server/pkg/foldersync/debouncer.go`
- `packages/server/pkg/foldersync/filter.go`
- `packages/server/pkg/foldersync/types.go`
- `packages/server/pkg/foldersync/watcher_test.go`

**Dependencies**: `github.com/fsnotify/fsnotify`

**Testing**: Create temp dirs, write files, verify events

### Phase 6: Sync Coordinator

**Scope**: Bidirectional sync engine — disk↔database with eventstream integration.

**Files**:
- `packages/server/pkg/foldersync/sync.go`
- `packages/server/pkg/foldersync/autosaver.go`

**Dependencies**: Phase 4 (YAML format), Phase 5 (watcher), services, eventstream

**Testing**: Full integration tests — modify files on disk, verify DB updates; modify DB, verify files written

### Phase 7: RPC Endpoints + Desktop Integration

**Scope**: TypeSpec definitions for folder sync management, RPC handlers, desktop UI.

**Files**:
- `packages/spec/` (TypeSpec definitions)
- `packages/server/internal/api/` (RPC handlers)
- `packages/client/` (React hooks/services)
- `apps/desktop/` (Electron integration — folder picker, sync status)

**Dependencies**: Phase 6 (sync coordinator)

### Phase 8: CLI Collection Runner

**Scope**: Run requests/flows directly from collection folders.

**Files**:
- `apps/cli/cmd/run.go` (extend with `collection` subcommand)

**Dependencies**: Phase 4 (YAML format), existing runner

---

## Phase Dependency Graph

```
Phase 1: BRU Parser ──────────────┐
                                   ├──→ Phase 2: Bruno Converter ──→ Phase 3: CLI Import
                                   │
Phase 4: YAML Collection Format ──┼──→ Phase 8: CLI Collection Runner
                                   │
Phase 5: File Watcher ────────────┤
                                   │
                                   └──→ Phase 6: Sync Coordinator ──→ Phase 7: RPC + Desktop
```

Phases 1 and 4-5 can be developed in parallel (no dependencies between them).

---

## External Dependencies

| Dependency | Purpose | Phase |
|---|---|---|
| `github.com/fsnotify/fsnotify` | Cross-platform file system notifications | 5 |
| `gopkg.in/yaml.v3` | YAML parsing (already in use by `yamlflowsimplev2`) | 4 |
| No new dependencies | BRU parser is hand-written | 1 |

---

## Key Design Decisions

### Why DevTools YAML instead of .bru format?

1. **YAML is universal** — every developer knows it, every editor supports it
2. **No custom parser maintenance** — leverage `gopkg.in/yaml.v3` vs maintaining a PEG parser
3. **Extensible** — easy to add new fields as DevTools evolves
4. **Consistent with existing YAML flow format** — `yamlflowsimplev2` already uses YAML
5. **Better tooling** — YAML schema validation, IDE autocomplete via JSON Schema
6. **Import, don't adopt** — import Bruno collections but don't tie DevTools to Bruno's format

### Why hand-written BRU parser instead of PEG library?

1. **Simple grammar** — only 3 block types, line-based parsing
2. **No external dependency** — keeps the import path dependency-free
3. **Better error messages** — hand-written parsers produce clearer diagnostics
4. **One-way** — we only need to parse `.bru` for import, not write it back

### Why fsnotify instead of polling?

1. **Efficient** — kernel-level notifications, no CPU overhead from polling
2. **Low latency** — events arrive in milliseconds vs polling interval
3. **Standard** — most Go file-watching libraries use fsnotify
4. **Exception**: WSL paths use polling (matching Bruno's approach)

### Why autosave debounce at 500ms?

1. **Matches Bruno** — users expect similar behavior
2. **Prevents rapid writes** — typing in URL field doesn't cause per-keystroke disk writes
3. **Balances responsiveness** — changes appear on disk within ~500ms, fast enough for git workflows
