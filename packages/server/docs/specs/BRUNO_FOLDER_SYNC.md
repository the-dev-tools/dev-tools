# OpenCollection Import & OpenYAML Folder Sync Plan

## Overview

This document describes the plan to add:

1. **OpenCollection YAML Import** — Parse Bruno's OpenCollection YAML collections and convert them into DevTools' OpenYAML format in a separate folder
2. **OpenYAML Folder Sync** — Bidirectional filesystem sync using DevTools' own "OpenYAML" format (requests + flows), with the **folder as the source of truth** and SQLite as a runtime cache
3. **Workspace Sync Modes** — A workspace can be synced to a local folder, opened from a Bruno collection (auto-converted), or used without sync (current behavior)
4. **CLI Runner Integration** — Execute collections from OpenYAML folders

### Key Concepts

- **OpenYAML** — DevTools' own YAML format for collections. Includes both HTTP requests and flows. One file per request, one file per flow, folder hierarchy maps to the file tree.
- **OpenCollection YAML** — Bruno's YAML format (`opencollection.yml` root). We import FROM this format.
- **Folder = Source of Truth** — The OpenYAML folder is what gets committed to git, shared with teammates, and survives across machines. It is the canonical data.
- **SQLite = Runtime Cache** — SQLite is populated from the folder and provides fast indexed queries for the UI. It can be fully rebuilt from the folder at any time.
- **Workspace Sync Modes** — Each workspace can optionally be linked to a folder on disk.

### Sources

- [OpenCollection YAML Format (Bruno Docs)](https://docs.usebruno.com/opencollection-yaml/overview)
- [YAML Structure Reference](https://docs.usebruno.com/opencollection-yaml/structure-reference)
- [YAML Samples](https://docs.usebruno.com/opencollection-yaml/samples)
- [OpenCollection Spec](https://spec.opencollection.com/)

---

## Architecture: SQLite + Folder Sync Coexistence

### Current State

```
Desktop App (React UI)
    ↕  Connect RPC over Unix Socket
Go Server
    ↕  Reader/Writer services
Single SQLite DB (state.db in userData)
    └── All workspaces, requests, flows, environments in one DB
```

- One `state.db` file contains ALL workspaces
- Workspace model: `ID, Name, Updated, Order, ActiveEnv, GlobalEnv` — **no path/folder field**
- All data lives exclusively in SQLite
- Real-time UI sync via in-memory eventstream

### Proposed State: Folder-First Architecture

```
Desktop App (React UI)
    ↕  Connect RPC over Unix Socket
Go Server
    ↕  Reader/Writer services
    ├── SQLite DB (state.db) ←── RUNTIME CACHE (disposable, rebuildable)
    │   └── workspace.sync_path = "/path/to/my-collection"
    │
    └── SyncCoordinator (per synced workspace)
            ↕  bidirectional
        OpenYAML Folder ←── SOURCE OF TRUTH (git, shared, portable)
            /path/to/my-collection/
            ├── devtools.yaml
            ├── requests/
            └── flows/
```

### Design Principle: Folder is the Source of Truth

> **Scope:** The folder-as-source-of-truth / SQLite-as-cache behavior described in this section
> **only applies to synced workspaces** (Mode 2: Sync to Folder, Mode 3: Import from Bruno).
> Non-synced workspaces (Mode 1) continue to use SQLite as the sole data store — no folder
> is involved, and no reconciliation or file watching occurs. The SyncCoordinator is never
> created for Mode 1 workspaces.

The OpenYAML folder is the **canonical data store**. SQLite is a **runtime cache** that can be fully rebuilt from the folder at any time. This is the same model Bruno uses — their Redux store is just a runtime view of what's on disk.

**Why the folder must be the source of truth:**
- `git pull` brings new changes → folder has the latest data → SQLite must update to match
- Teammate edits a request in their editor → saves → watcher picks it up → SQLite updates → UI reflects
- Delete `state.db` → server starts → reads folder → SQLite rebuilt → nothing lost
- The folder is what gets committed, pushed, reviewed in PRs, and shared across machines

**Why SQLite is still valuable as a cache:**
- Fast indexed queries (no YAML parsing on every read)
- Transactions for atomic multi-entity operations
- Existing services, RPC handlers, and runner all work with SQLite
- Real-time eventstream already wired to SQLite changes
- Supports non-synced workspaces (Mode 1) that live only in SQLite

| Direction | Trigger | Behavior |
|-----------|---------|----------|
| **Folder → SQLite** | File watcher detects change, or git pull, or startup | Parse YAML → upsert into SQLite → eventstream → UI updates |
| **SQLite → Folder** | UI edit via RPC handler | Write to SQLite → serialize to YAML → write to disk |
| **Startup** | Server starts with synced workspace | Read entire folder → populate/reconcile SQLite |
| **Git pull** | Watcher detects batch changes | Re-parse changed files → update SQLite → UI refreshes |
| **Conflict** | File changed on disk while UI was editing | **Folder wins** — disk state overwrites SQLite |
| **Rebuild** | `state.db` deleted or corrupted | Full re-read from folder → SQLite rebuilt from scratch |

### Reconciliation on Startup

When a synced workspace is opened, the SyncCoordinator must reconcile SQLite with the folder:

```
1. Walk the OpenYAML folder, build a map of path → parsed content
2. Read all entities for this workspace from SQLite
3. Compare:
   a. File exists on disk but not in SQLite → INSERT (new file from git pull)
   b. File exists in both, content differs → UPDATE SQLite from disk (folder wins)
   c. Entity in SQLite but no file on disk → DELETE from SQLite (file was deleted/moved)
   d. File and SQLite match → no-op
4. Rebuild pathToID / idToPath maps
5. Start file watcher for live changes
```

This ensures that after a `git pull`, `git merge`, `git checkout`, or any external file changes, SQLite is always in sync with the folder.

### Workspace Schema Change

Add `sync_path` and `sync_format` to the workspace model:

```sql
-- New columns on workspaces table
ALTER TABLE workspaces ADD COLUMN sync_path TEXT;         -- NULL = no sync
ALTER TABLE workspaces ADD COLUMN sync_format TEXT;       -- "open_yaml" | "opencollection"
ALTER TABLE workspaces ADD COLUMN sync_enabled BOOLEAN NOT NULL DEFAULT 0;
```

```go
// Updated workspace model
type Workspace struct {
    ID              idwrap.IDWrap
    Name            string
    Updated         time.Time
    Order           float64
    ActiveEnv       idwrap.IDWrap
    GlobalEnv       idwrap.IDWrap
    FlowCount       int32
    CollectionCount int32
    // NEW: Folder sync fields
    SyncPath        *string          // nil = no sync, else absolute path to folder
    SyncFormat      *string          // "open_yaml" (our format) or nil
    SyncEnabled     bool             // Whether sync is currently active
}
```

---

## Workspace Sync Modes

### Mode 1: No Sync (Default — Current Behavior)

```
User creates workspace → data lives only in SQLite
SyncPath = nil, SyncEnabled = false
```

Nothing changes from the current behavior. Workspaces work exactly as they do today. **SQLite is the sole data store** — no folder sync, no file watcher, no reconciliation. All existing services, RPC handlers, and eventstream work unchanged.

### Mode 2: Sync to Folder (OpenYAML)

```
User creates workspace → links to a folder → folder becomes source of truth
SyncPath = "/Users/dev/my-api-collection", SyncFormat = "open_yaml", SyncEnabled = true
```

**Two sub-scenarios:**

**A) Export to new folder (existing workspace → empty folder):**
1. User has an existing workspace in DevTools (data in SQLite)
2. User clicks "Sync to Folder" → picks/creates an empty directory
3. Server sets `sync_path` on the workspace
4. SyncCoordinator starts → exports all SQLite data to OpenYAML files in the folder
5. File watcher starts → from now on, folder is the source of truth
6. User can `git init && git add . && git commit` to start versioning

**B) Open existing folder (OpenYAML folder → new workspace):**
1. User clicks "Open Folder" → picks a directory with `devtools.yaml`
2. Server creates a new workspace with `sync_path` set
3. SyncCoordinator starts → reads entire folder → populates SQLite cache
4. File watcher starts → folder is the source of truth
5. This is the common flow after `git clone` on a new machine

### Mode 3: Import from Bruno (OpenCollection → OpenYAML)

```
User opens Bruno collection → DevTools converts to OpenYAML in a NEW folder → syncs there
SyncPath = "/Users/dev/my-api-devtools/", SyncFormat = "open_yaml", SyncEnabled = true
```

**Flow:**
1. User clicks "Import Bruno Collection" → picks directory with `opencollection.yml`
2. Server parses the OpenCollection YAML directory
3. Server creates a new workspace and populates SQLite with the converted data
4. Server creates a NEW folder (e.g., next to the Bruno folder, or user picks location)
5. SyncCoordinator exports SQLite data to OpenYAML format in the new folder
6. File watcher starts → bidirectional sync is live on the NEW folder
7. Original Bruno folder is NOT modified

**Why a separate folder?** The Bruno folder uses OpenCollection YAML format (different schema). We don't want to:
- Corrupt the Bruno collection
- Mix two different YAML formats in one folder
- Create confusion about which tool owns the folder

---

## Part 1: OpenCollection YAML Import

### 1.1 Architecture Fit

```
OpenCollection .yml directory
    → topencollection.ConvertOpenCollection()
        → OpenCollectionResolved (mhttp.HTTP, mfile.File, mflow.Flow, etc.)
            → SQLite (workspace created + populated)
                → SyncCoordinator exports to OpenYAML folder
```

| Layer | Location | Pattern |
|-------|----------|---------|
| CLI Command | `apps/cli/cmd/import.go` | Add `importBrunoCmd` |
| RPC Endpoint | `packages/server/internal/api/` | "Import Bruno Collection" |
| Translator | `packages/server/pkg/translate/topencollection/` | New package |
| Importer | `apps/cli/internal/importer/` | Existing `RunImport()` callback |

### 1.2 OpenCollection YAML Format Reference

#### Directory Structure

```
my-bruno-collection/
├── opencollection.yml       # Collection root config
├── environments/
│   └── development.yml
├── users/
│   ├── folder.yml           # Folder configuration
│   ├── create-user.yml
│   ├── get-user.yml
│   └── delete-user.yml
└── orders/
    └── create-order.yml
```

#### Collection Root (`opencollection.yml`)

```yaml
opencollection: "1.0.0"
info:
  name: "My API Collection"
  summary: "A collection for testing our REST API"
  version: "2.1.0"
  authors:
    - name: "Jane Doe"
      email: "[email protected]"
```

#### Request File Structure

```yaml
info:
  name: Create User
  type: http
  seq: 5
  tags: [smoke, regression]

http:
  method: POST
  url: https://api.example.com/users
  headers:
    - name: Content-Type
      value: application/json
    - name: Authorization
      value: "Bearer {{token}}"
      disabled: true
  params:
    - name: filter
      value: active
      type: query
    - name: id
      value: "123"
      type: path
  body:
    type: json
    data: |-
      {
        "name": "John Doe",
        "email": "john@example.com"
      }
  auth:
    type: bearer
    token: "{{token}}"

runtime:
  assertions:
    - expression: res.status
      operator: eq
      value: "201"

settings:
  encodeUrl: true

docs: |-
  Creates a new user account.
```

### 1.3 Go Types for OpenCollection Parsing

```go
package topencollection

// --- Collection Root ---
type OpenCollectionRoot struct {
    OpenCollection string            `yaml:"opencollection"`
    Info           OpenCollectionInfo `yaml:"info"`
}

type OpenCollectionInfo struct {
    Name    string                 `yaml:"name"`
    Summary string                `yaml:"summary,omitempty"`
    Version string                `yaml:"version,omitempty"`
    Authors []OpenCollectionAuthor `yaml:"authors,omitempty"`
}

// --- Request File ---
type OCRequest struct {
    Info     OCRequestInfo `yaml:"info"`
    HTTP     *OCHTTPBlock  `yaml:"http,omitempty"`
    Runtime  *OCRuntime    `yaml:"runtime,omitempty"`
    Settings *OCSettings   `yaml:"settings,omitempty"`
    Docs     string        `yaml:"docs,omitempty"`
}

type OCRequestInfo struct {
    Name string   `yaml:"name"`
    Type string   `yaml:"type"`
    Seq  int      `yaml:"seq,omitempty"`
    Tags []string `yaml:"tags,omitempty"`
}

type OCHTTPBlock struct {
    Method  string     `yaml:"method"`
    URL     string     `yaml:"url"`
    Headers []OCHeader `yaml:"headers,omitempty"`
    Params  []OCParam  `yaml:"params,omitempty"`
    Body    *OCBody    `yaml:"body,omitempty"`
    Auth    *OCAuth    `yaml:"auth,omitempty"`
}

type OCHeader struct {
    Name     string `yaml:"name"`
    Value    string `yaml:"value"`
    Disabled bool   `yaml:"disabled,omitempty"`
}

type OCParam struct {
    Name     string `yaml:"name"`
    Value    string `yaml:"value"`
    Type     string `yaml:"type"`
    Disabled bool   `yaml:"disabled,omitempty"`
}

type OCBody struct {
    Type string      `yaml:"type"` // json|xml|text|form-urlencoded|multipart-form|graphql|none
    Data interface{} `yaml:"data"` // string for raw, []OCFormField for forms
}

type OCFormField struct {
    Name        string `yaml:"name"`
    Value       string `yaml:"value"`
    Disabled    bool   `yaml:"disabled,omitempty"`
    ContentType string `yaml:"contentType,omitempty"`
}

type OCAuth struct {
    Type      string `yaml:"type"`               // none|inherit|basic|bearer|apikey|...
    Token     string `yaml:"token,omitempty"`     // bearer
    Username  string `yaml:"username,omitempty"`  // basic
    Password  string `yaml:"password,omitempty"`  // basic
    Key       string `yaml:"key,omitempty"`       // apikey
    Value     string `yaml:"value,omitempty"`     // apikey
    Placement string `yaml:"placement,omitempty"` // apikey: header|query
}

type OCRuntime struct {
    Scripts    []OCScript    `yaml:"scripts,omitempty"`
    Assertions []OCAssertion `yaml:"assertions,omitempty"`
    Actions    []OCAction    `yaml:"actions,omitempty"`
}

type OCAssertion struct {
    Expression string `yaml:"expression"`
    Operator   string `yaml:"operator"`
    Value      string `yaml:"value,omitempty"`
}

type OCSettings struct {
    EncodeUrl       *bool `yaml:"encodeUrl,omitempty"`
    Timeout         *int  `yaml:"timeout,omitempty"`
    FollowRedirects *bool `yaml:"followRedirects,omitempty"`
    MaxRedirects    *int  `yaml:"maxRedirects,omitempty"`
}

type OCEnvironment struct {
    Name      string          `yaml:"name"`
    Variables []OCEnvVariable `yaml:"variables"`
}

type OCEnvVariable struct {
    Name    string `yaml:"name"`
    Value   string `yaml:"value"`
    Enabled *bool  `yaml:"enabled,omitempty"`
    Secret  *bool  `yaml:"secret,omitempty"`
}
```

### 1.4 Converter

```go
type OpenCollectionResolved struct {
    HTTPRequests       []mhttp.HTTP
    HTTPHeaders        []mhttp.HTTPHeader
    HTTPSearchParams   []mhttp.HTTPSearchParam
    HTTPBodyForms      []mhttp.HTTPBodyForm
    HTTPBodyUrlencoded []mhttp.HTTPBodyUrlencoded
    HTTPBodyRaw        []mhttp.HTTPBodyRaw
    HTTPAsserts        []mhttp.HTTPAssert
    Files              []mfile.File
    Environments       []menv.Env
    EnvironmentVars    []menv.Variable
}

func ConvertOpenCollection(collectionPath string, opts ConvertOptions) (*OpenCollectionResolved, error)
```

#### Mapping Table: OpenCollection → DevTools

| OpenCollection YAML | DevTools Model | Notes |
|---|---|---|
| `info.name` | `mhttp.HTTP.Name` | |
| `info.seq` | `mfile.File.Order` | Float64 ordering |
| `http.method` | `mhttp.HTTP.Method` | Uppercase |
| `http.url` | `mhttp.HTTP.Url` | |
| `http.headers` | `[]mhttp.HTTPHeader` | `disabled` → `Enabled: false` |
| `http.params` (query) | `[]mhttp.HTTPSearchParam` | |
| `http.body.type: json/xml/text` | `mhttp.HTTPBodyRaw` | `BodyKind: Raw` |
| `http.body.type: form-urlencoded` | `[]mhttp.HTTPBodyUrlencoded` | |
| `http.body.type: multipart-form` | `[]mhttp.HTTPBodyForm` | |
| `http.auth.type: bearer` | `mhttp.HTTPHeader` | → `Authorization: Bearer <token>` |
| `http.auth.type: basic` | `mhttp.HTTPHeader` | → `Authorization: Basic <b64>` |
| `http.auth.type: apikey` | Header or SearchParam | Based on `placement` |
| `runtime.assertions` | `[]mhttp.HTTPAssert` | `expr operator value` format |
| `runtime.scripts` | Not imported (log warning) | DevTools uses JS flow nodes |
| `docs` | `mhttp.HTTP.Description` | |
| Directory structure | `mfile.File` hierarchy | Nesting preserved |
| `environments/*.yml` | `menv.Env` + `menv.Variable` | |

### 1.5 Package Structure

```
packages/server/pkg/translate/topencollection/
├── types.go              # YAML struct definitions
├── converter.go          # Directory → OpenCollectionResolved
├── converter_test.go     # Tests with sample collections
├── collection.go         # opencollection.yml parsing
├── environment.go        # Environment conversion
├── auth.go               # Auth → header/param conversion
├── body.go               # Body type → mhttp body conversion
└── testdata/
    └── basic-collection/
        ├── opencollection.yml
        ├── environments/
        │   ├── dev.yml
        │   └── prod.yml
        ├── users/
        │   ├── folder.yml
        │   ├── get-users.yml
        │   └── create-user.yml
        └── auth/
            └── login.yml
```

---

## Part 2: OpenYAML Format (DevTools' Own Format)

### 2.1 Design Goals

- **Includes flows** — not just HTTP requests, but full flow definitions
- **One file per entity** — each request and flow is its own `.yaml` file
- **Git-friendly** — clean diffs, merge-friendly structure
- **Human-editable** — developers can edit in any text editor or IDE
- **Flat top-level** — `name`, `method`, `url` at root (no `info`/`http` nesting like OpenCollection)
- **Reuses existing `yamlflowsimplev2` types** — request types (`YamlRequestDefV2`, `HeaderMapOrSlice`, `YamlBodyUnion`, `AssertionsOrSlice`) and flow types (`YamlFlowFlowV2`) are imported directly, not duplicated. Only `gopkg.in/yaml.v3` for YAML parsing.

### 2.2 Directory Structure

```
my-collection/
├── devtools.yaml              # Collection config
├── environments/
│   ├── dev.yaml
│   └── prod.yaml
├── users/                     # Folder = directory
│   ├── _folder.yaml           # Optional folder metadata
│   ├── get-users.yaml         # HTTP request
│   └── create-user.yaml       # HTTP request
├── auth/
│   ├── _folder.yaml
│   └── login.yaml
└── flows/                     # Flow definitions
    ├── smoke-test.yaml        # Flow (yamlflowsimplev2 format)
    └── ci-regression.yaml
```

### 2.3 Collection Config (`devtools.yaml`)

```yaml
version: "1"
name: My API Collection
```

This file identifies the directory as a DevTools OpenYAML collection. Its presence is how we detect the format (analogous to `opencollection.yml` for Bruno).

### 2.4 Request File Format

```yaml
name: Get Users
method: GET
url: "{{base_url}}/users"
description: "Fetch all users with optional pagination"
order: 1

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

```yaml
name: Create User
method: POST
url: "{{base_url}}/users"
order: 2

headers:
  - name: Content-Type
    value: application/json

body:
  type: raw
  raw: |
    {
      "name": "John Doe",
      "email": "john@example.com"
    }

assertions:
  - "res.status eq 201"
  - "res.body.id neq null"
```

```yaml
name: Upload File
method: POST
url: "{{base_url}}/upload"
order: 3

body:
  type: form_data
  form_data:
    - name: file
      value: "@./fixtures/test.png"
      description: "File to upload"
    - name: description
      value: "Test upload"
```

### 2.5 Flow File Format

Flows use the existing `yamlflowsimplev2` format — this is already implemented and working:

```yaml
# flows/smoke-test.yaml
name: Smoke Test
variables:
  - name: auth_token
    type: string
    default: ""

steps:
  - request:
      name: Login
      method: POST
      url: "{{base_url}}/auth/login"
      body:
        type: raw
        raw: '{"email": "test@example.com", "password": "test"}'

  - request:
      name: Get Profile
      depends_on: [Login]
      method: GET
      url: "{{base_url}}/users/me"
      headers:
        Authorization: "Bearer {{Login.response.body.token}}"

  - js:
      name: Validate Response
      depends_on: [Get Profile]
      code: |
        if (response.status !== 200) throw new Error("Failed");
```

### 2.6 Environment File

```yaml
name: Development
variables:
  - name: base_url
    value: "http://localhost:3000"
  - name: token
    value: "dev-token-123"
    secret: true
```

### 2.7 Folder Metadata (`_folder.yaml`)

```yaml
name: Users API
order: 1
description: "User management endpoints"
```

### 2.8 OpenYAML Go Types

> **Reuse Policy:** The `topenyaml` package reuses types from `yamlflowsimplev2` wherever possible
> to avoid duplicating YAML marshaling logic. The types below reference:
> - `yamlflowsimplev2.YamlRequestDefV2` — request fields (name, method, url, headers, body, etc.)
> - `yamlflowsimplev2.HeaderMapOrSlice` — flexible header/param list (map or slice with custom marshal)
> - `yamlflowsimplev2.YamlNameValuePairV2` — individual name/value/enabled entry
> - `yamlflowsimplev2.YamlBodyUnion` — flexible body (raw string, JSON map, or structured with type)
> - `yamlflowsimplev2.AssertionsOrSlice` — assertions (string shorthand or structured)
> - `yamlflowsimplev2.YamlAssertionV2` — individual assertion entry
> - `yamlflowsimplev2.YamlFlowFlowV2` — flow definition (used directly for flow files)
>
> Only types that have no equivalent in `yamlflowsimplev2` are defined in `topenyaml`.

```go
package topenyaml

import (
    yfs "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

// CollectionConfig represents devtools.yaml
type CollectionConfig struct {
    Version string `yaml:"version"`
    Name    string `yaml:"name"`
}

// RequestFile represents a single request .yaml file.
// Embeds YamlRequestDefV2 for the core request fields (Name, Method, URL,
// Headers, QueryParams, Body, Assertions, Description) — all of which reuse
// the existing custom marshalers (HeaderMapOrSlice, YamlBodyUnion, etc.).
// Adds Order for file-tree ordering (not present in YamlRequestDefV2).
type RequestFile struct {
    yfs.YamlRequestDefV2 `yaml:",inline"`
    Order                float64 `yaml:"order,omitempty"`
}

// EnvironmentFile represents an environment .yaml file.
// NOTE: This differs from yamlflowsimplev2.YamlEnvironmentV2 which uses
// map[string]string for variables. OpenYAML environments need per-variable
// metadata (secret flag), so we define our own type here.
type EnvironmentFile struct {
    Name      string        `yaml:"name"`
    Variables []EnvVariable `yaml:"variables"`
}

type EnvVariable struct {
    Name   string `yaml:"name"`
    Value  string `yaml:"value"`
    Secret bool   `yaml:"secret,omitempty"`
}

type FolderMeta struct {
    Name        string  `yaml:"name,omitempty"`
    Order       float64 `yaml:"order,omitempty"`
    Description string  `yaml:"description,omitempty"`
}
```

**Type Reuse Summary:**

| OpenYAML Need | Reused From `yamlflowsimplev2` | Notes |
|---|---|---|
| Request fields | `YamlRequestDefV2` (embedded) | Name, Method, URL, Headers, QueryParams, Body, Assertions, Description |
| Headers / Query params | `HeaderMapOrSlice` → `[]YamlNameValuePairV2` | Supports both map and list YAML forms |
| Body | `*YamlBodyUnion` | Supports raw string, JSON map, form_data, urlencoded |
| Assertions | `AssertionsOrSlice` → `[]YamlAssertionV2` | Supports string shorthand and structured |
| Flow files | `YamlFlowFlowV2` (used directly) | No wrapper needed — flow .yaml files ARE `YamlFlowFlowV2` |
| Environments | **New** `EnvironmentFile` | Needs `secret` field not in `YamlEnvironmentV2` |
| Folder metadata | **New** `FolderMeta` | No equivalent in `yamlflowsimplev2` |
| Collection config | **New** `CollectionConfig` | No equivalent in `yamlflowsimplev2` |

### 2.9 Package Structure

```
packages/server/pkg/translate/topenyaml/
├── types.go           # YAML struct definitions
├── parser.go          # Read collection directory → DevTools models
├── serializer.go      # DevTools models → YAML files on disk
├── request.go         # Single request YAML ↔ mhttp conversion
├── flow.go            # Delegates to yamlflowsimplev2 for flow parsing
├── environment.go     # Environment YAML ↔ menv conversion
├── folder.go          # _folder.yaml handling
├── collection.go      # devtools.yaml config
└── parser_test.go     # Round-trip tests
```

### 2.10 Conversion Functions

```go
// ReadCollection reads an OpenYAML directory into DevTools models.
// Uses yamlflowsimplev2 converter functions internally:
//   - convertToHTTPHeaders() for HeaderMapOrSlice → []mhttp.HTTPHeader
//   - convertToHTTPSearchParams() for HeaderMapOrSlice → []mhttp.HTTPSearchParam
//   - convertBodyStruct() for *YamlBodyUnion → mhttp body types
func ReadCollection(collectionPath string, opts ReadOptions) (*ioworkspace.WorkspaceBundle, error)

// WriteCollection exports a workspace bundle to an OpenYAML directory.
func WriteCollection(collectionPath string, bundle *ioworkspace.WorkspaceBundle) error

// ReadRequest parses a single request YAML file into a RequestFile
// (which embeds yamlflowsimplev2.YamlRequestDefV2).
func ReadRequest(data []byte) (*RequestFile, error)

// WriteRequest serializes DevTools models to a single request YAML.
func WriteRequest(http mhttp.HTTP, headers []mhttp.HTTPHeader,
    params []mhttp.HTTPSearchParam, body interface{},
    asserts []mhttp.HTTPAssert) ([]byte, error)

// ReadFlow parses a single flow YAML file (delegates to yamlflowsimplev2).
// Flow files are yamlflowsimplev2.YamlFlowFlowV2 — no topenyaml wrapper needed.
func ReadFlow(data []byte, opts yfs.ConvertOptionsV2) (*yfs.YamlFlowDataV2, error)

// WriteFlow serializes a single flow to YAML (delegates to yamlflowsimplev2 exporter).
func WriteFlow(flow yfs.YamlFlowFlowV2) ([]byte, error)
```

---

## Part 3: Folder Sync Engine

### 3.1 File Watcher (`packages/server/pkg/foldersync/`)

```
packages/server/pkg/foldersync/
├── watcher.go         # fsnotify-based CollectionWatcher
├── debouncer.go       # Write stabilization (80ms coalescing)
├── filter.go          # Ignore .git, node_modules, non-.yaml
├── selftrack.go       # Self-write tracker (prevent infinite loops)
├── sync.go            # SyncCoordinator (bidirectional orchestrator)
├── types.go           # Event types, SyncOptions
└── watcher_test.go    # Integration tests
```

### 3.2 Watcher

```go
type EventType int
const (
    EventFileCreated EventType = iota
    EventFileChanged
    EventFileDeleted
    EventDirCreated
    EventDirDeleted
)

type WatchEvent struct {
    Type    EventType
    Path    string // Absolute path
    RelPath string // Relative to collection root
}

type CollectionWatcher struct {
    collectionPath string
    watcher        *fsnotify.Watcher
    debouncer      *Debouncer
    selfWrites     *SelfWriteTracker
    events         chan WatchEvent
}

func NewCollectionWatcher(path string, opts WatcherOptions) (*CollectionWatcher, error)
func (w *CollectionWatcher) Start(ctx context.Context) error
func (w *CollectionWatcher) Events() <-chan WatchEvent
func (w *CollectionWatcher) Stop() error
```

### 3.3 Debouncer (80ms stabilization)

```go
type Debouncer struct {
    threshold time.Duration // 80ms
    timers    map[string]*time.Timer
    mu        sync.Mutex
    output    chan WatchEvent
}
```

### 3.4 Self-Write Tracker

```go
type SelfWriteTracker struct {
    mu       sync.Mutex
    writes   map[string]time.Time
    lifetime time.Duration // 2s suppression window
}

func (t *SelfWriteTracker) MarkWrite(path string)
func (t *SelfWriteTracker) IsSelfWrite(path string) bool
```

### 3.5 SyncCoordinator

The central orchestrator. One instance per synced workspace.

```go
type SyncCoordinator struct {
    collectionPath string
    workspaceID    idwrap.IDWrap

    // Components
    watcher    *CollectionWatcher
    selfWrites *SelfWriteTracker
    autosaver  *AutoSaver

    // State mapping (UID preservation across re-parses)
    mu       sync.RWMutex
    pathToID map[string]idwrap.IDWrap
    idToPath map[idwrap.IDWrap]string

    // Services (read/write to SQLite)
    db       *sql.DB
    services *common.Services

    // Real-time sync to UI
    publisher eventstream.Publisher
}

type SyncOptions struct {
    CollectionPath string
    WorkspaceID    idwrap.IDWrap
    DB             *sql.DB
    Services       *common.Services
    Publisher       eventstream.Publisher
}

func NewSyncCoordinator(opts SyncOptions) (*SyncCoordinator, error)
func (s *SyncCoordinator) Start(ctx context.Context) error
func (s *SyncCoordinator) Stop() error

// Reconcile reads the entire folder and reconciles SQLite cache to match.
// Called on startup, after git pull, or when opening a synced workspace.
// The folder always wins — SQLite is rebuilt to match the folder state.
func (s *SyncCoordinator) Reconcile(ctx context.Context) error

// ExportAll writes all SQLite data for this workspace to the folder.
// Called ONCE when sync is first enabled on an existing (non-synced) workspace.
// After this initial export, the folder becomes the source of truth.
func (s *SyncCoordinator) ExportAll(ctx context.Context) error
```

#### Folder → SQLite (external changes: git pull, editor saves, etc.)

```
File change detected (watcher)
    → Debounce (80ms)
    → Skip if self-write
    → Classify file type (request .yaml, flow .yaml, environment, folder meta)
    → Parse YAML → intermediate types
    → Look up entity by path→ID mapping
    → Begin transaction
        → If new file: INSERT into SQLite cache
        → If changed file: UPDATE SQLite cache (folder wins)
        → If deleted file: DELETE from SQLite cache
    → Commit transaction
    → Publish events to eventstream → UI updates in real-time
```

#### SQLite → Folder (UI edits)

```
UI edit → RPC handler
    → Write to SQLite cache (for immediate UI responsiveness)
    → Serialize entity to YAML
    → Mark path in self-write tracker
    → Atomic write to disk (temp file + rename) ← this is the canonical write
    → Watcher detects → self-write tracker suppresses → no loop
```

**Key difference from "SQLite is king" model:** The RPC handler writes to BOTH SQLite and disk. The disk write is the one that matters — if SQLite is lost, the disk file survives. The SQLite write is for UI performance (instant queries without re-parsing YAML).

#### Git Pull / Branch Switch (batch reconciliation)

```
User runs `git pull` or `git checkout` outside DevTools
    → Watcher detects batch of file changes
    → For each changed/added/deleted file:
        → Update SQLite cache to match folder state
    → Publish batch events to eventstream → UI refreshes
```

This is the critical flow that "SQLite is king" would break — after git pull, the folder has the truth and SQLite must catch up.

### 3.6 AutoSaver (500ms debounce)

```go
type AutoSaver struct {
    delay   time.Duration // 500ms
    timers  map[idwrap.IDWrap]*time.Timer
    mu      sync.Mutex
    writeFn func(entityID idwrap.IDWrap) error
}

func (a *AutoSaver) ScheduleSave(entityID idwrap.IDWrap)
func (a *AutoSaver) Flush() // Force-save all pending (graceful shutdown)
```

### 3.7 Sync Manager (Server-Level)

Manages all active SyncCoordinators across workspaces.

```go
// packages/server/pkg/foldersync/manager.go

type SyncManager struct {
    mu           sync.RWMutex
    coordinators map[idwrap.IDWrap]*SyncCoordinator // workspaceID → coordinator
    db           *sql.DB
    services     *common.Services
    publisher    eventstream.Publisher
}

func NewSyncManager(db *sql.DB, services *common.Services, publisher eventstream.Publisher) *SyncManager

// StartSync begins folder sync for a workspace.
func (m *SyncManager) StartSync(ctx context.Context, workspaceID idwrap.IDWrap, path string) error

// StopSync stops folder sync for a workspace.
func (m *SyncManager) StopSync(workspaceID idwrap.IDWrap) error

// IsActive returns whether a workspace has active sync.
func (m *SyncManager) IsActive(workspaceID idwrap.IDWrap) bool

// RestoreAll starts sync for all workspaces that have sync_enabled=true.
// Called on server startup. For each synced workspace:
// 1. Reconcile SQLite cache from folder (folder wins)
// 2. Start file watcher
func (m *SyncManager) RestoreAll(ctx context.Context) error

// Shutdown stops all coordinators gracefully.
func (m *SyncManager) Shutdown() error
```

### 3.8 Safety Mechanisms

| Mechanism | Implementation |
|---|---|
| Path validation | `filepath.Rel()` must not escape collection root |
| Filename sanitization | Strip invalid chars, truncate at 255 |
| Write stabilization | 80ms debounce on watcher events |
| Autosave debounce | 500ms debounce on SQLite→disk writes |
| Self-write suppression | 2s window to suppress watcher events from our writes |
| Atomic writes | Write temp file → `os.Rename()` |
| UID preservation | `pathToID` map persists during session |
| Conflict resolution | Folder always wins (it's the source of truth) |
| Large file guard | Skip files >5MB |
| Cross-platform | `filepath.Clean/Rel/Join`, handle `\r\n` |
| Recursive watch | Walk tree on start, add subdirs on `DirCreated` |
| Max depth | 20 levels |

---

## Part 4: RPC Endpoints

### 4.1 Workspace Sync API

New TypeSpec definitions for folder sync management:

```
// Folder sync operations on workspaces

// Enable folder sync on a workspace
EnableFolderSync(workspaceId, folderPath) → SyncStatus

// Disable folder sync
DisableFolderSync(workspaceId) → void

// Get sync status
GetFolderSyncStatus(workspaceId) → SyncStatus

// Import Bruno collection → create workspace + OpenYAML folder
ImportBrunoCollection(brunoFolderPath, outputFolderPath) → Workspace

// Export workspace to OpenYAML folder
ExportToFolder(workspaceId, folderPath) → void

type SyncStatus {
    enabled: boolean
    folderPath: string
    lastSyncAt: timestamp
    fileCount: number
    errors: string[]  // Any sync errors
}
```

### 4.2 Desktop UI Integration

**New UI elements needed:**
- Workspace settings: "Link to Folder" button with folder picker
- Workspace settings: "Unlink Folder" button
- Status bar: sync status indicator (synced, syncing, error)
- Import dialog: "Import Bruno Collection" with source folder picker + destination folder picker
- New workspace dialog: option to "Create from folder" or "Create empty"

---

## Part 5: CLI Integration

### 5.1 CLI Import Command

```
devtools import bruno <bruno-dir> --output <open-yaml-dir> --workspace <id>
```

### 5.2 CLI Run from Folder

```
devtools run <open-yaml-dir>                        # Run all requests
devtools run <open-yaml-dir>/users/get-users.yaml   # Single request
devtools run <open-yaml-dir>/flows/smoke-test.yaml  # Run a flow
devtools run <open-yaml-dir> --env dev              # With environment
```

---

## Implementation Phases

### Phase 1: OpenCollection YAML Parser + Converter

**Scope**: Parse Bruno's OpenCollection YAML directories → DevTools models.

**Files**:
```
packages/server/pkg/translate/topencollection/
├── types.go, converter.go, collection.go, environment.go
├── auth.go, body.go, converter_test.go
└── testdata/basic-collection/...
```

**Deps**: `gopkg.in/yaml.v3` (existing), `mhttp`, `mfile`, `menv`

### Phase 2: OpenYAML Format (Requests + Flows)

**Scope**: DevTools' own YAML format — parser + serializer with round-trip support. Flows delegate to existing `yamlflowsimplev2`.

**Files**:
```
packages/server/pkg/translate/topenyaml/
├── types.go, parser.go, serializer.go, request.go
├── flow.go, environment.go, folder.go, collection.go
└── parser_test.go
```

**Deps**: `gopkg.in/yaml.v3`, `yamlflowsimplev2` (for flows), `mhttp`, `mfile`, `mflow`

### Phase 3: Workspace Schema + Migration

**Scope**: Add `sync_path`, `sync_format`, `sync_enabled` to workspace table and model.

**Files**:
- `packages/db/pkg/sqlc/schema/` — new migration SQL
- `packages/db/pkg/sqlc/queries/` — updated workspace queries
- `packages/server/pkg/model/mworkspace/` — updated model
- `packages/server/pkg/service/sworkspace/` — updated reader/writer
- Run `pnpm nx run db:generate` to regenerate sqlc

### Phase 4: File Watcher + Sync Engine

**Scope**: `fsnotify` watcher, debouncer, self-write tracker, SyncCoordinator, SyncManager.

**Files**:
```
packages/server/pkg/foldersync/
├── watcher.go, debouncer.go, filter.go, selftrack.go
├── sync.go, manager.go, types.go
└── watcher_test.go
```

**Deps**: `github.com/fsnotify/fsnotify`, Phase 2 (OpenYAML), Phase 3 (workspace schema)

### Phase 5: RPC Endpoints + CLI Import

**Scope**: TypeSpec definitions, RPC handlers for sync management, CLI `import bruno` command.

**Files**:
- `packages/spec/` — new TypeSpec definitions
- `packages/server/internal/api/` — RPC handlers
- `apps/cli/cmd/import.go` — `importBrunoCmd`

**Deps**: Phase 1 (OpenCollection parser), Phase 4 (sync engine)

### Phase 6: Desktop Integration

**Scope**: Electron folder picker, workspace sync settings UI, status indicators.

**Files**:
- `packages/client/` — React hooks/services for sync
- `packages/ui/` — sync status components
- `apps/desktop/` — Electron IPC for folder picker

**Deps**: Phase 5 (RPC endpoints)

### Phase 7: CLI Collection Runner

**Scope**: `devtools run <folder>` command.

**Files**:
- `apps/cli/cmd/run.go`

**Deps**: Phase 2 (OpenYAML format), existing runner

---

## Phase Dependency Graph

```
Phase 1: OpenCollection Parser ──────────────────────────┐
                                                         │
Phase 2: OpenYAML Format ──┬────────────────────────────┤
                            │                            │
Phase 3: Workspace Schema ──┤                            │
                            │                            │
                            └──→ Phase 4: Sync Engine ───┼──→ Phase 5: RPC + CLI Import
                                                         │
                                                         └──→ Phase 6: Desktop UI
Phase 2 ─────────────────────────────────────────────────────→ Phase 7: CLI Runner
```

**Parallel work:**
- Phase 1, 2, 3 can all be developed in parallel
- Phase 4 depends on 2+3
- Phase 5 depends on 1+4
- Phase 7 depends only on 2

---

## External Dependencies

| Dependency | Purpose | Already in use? |
|---|---|---|
| `gopkg.in/yaml.v3` | YAML parsing | Yes (`yamlflowsimplev2`) |
| `github.com/fsnotify/fsnotify` | Filesystem notifications | No (new) |
