# OpenCollection Import & YAML Folder Sync Plan

## Overview

This document describes the plan to add:

1. **OpenCollection YAML Import** — Parse Bruno's OpenCollection YAML collections (`.yml` format with `opencollection.yml` root) and convert them into DevTools' data model
2. **DevTools YAML Folder Sync** — Bidirectional filesystem sync using DevTools' own YAML format, inspired by Bruno's folder sync architecture
3. **CLI Runner Integration** — Execute imported/synced collections from the CLI

The goal is to support Bruno users migrating to DevTools, while building a first-class filesystem-based workflow using DevTools' own YAML format for git-friendly, local-first API development.

### Why OpenCollection YAML only (no .bru)?

Bruno is moving to the OpenCollection YAML format as the recommended standard going forward. The legacy `.bru` DSL format is being phased out. By focusing exclusively on the YAML format:

- No custom parser needed — we use standard `gopkg.in/yaml.v3` (already a dependency)
- Forward-compatible with Bruno's direction
- Simpler codebase to maintain
- YAML tooling (linting, schema validation, IDE support) works out of the box

### Sources

- [OpenCollection YAML Format (Bruno Docs)](https://docs.usebruno.com/opencollection-yaml/overview)
- [YAML Structure Reference](https://docs.usebruno.com/opencollection-yaml/structure-reference)
- [YAML Samples](https://docs.usebruno.com/opencollection-yaml/samples)
- [OpenCollection Spec](https://spec.opencollection.com/)
- [OpenCollection GitHub](https://github.com/opencollection-dev/opencollection)
- [RFC Discussion](https://github.com/usebruno/bruno/discussions/6634)

---

## Part 1: OpenCollection YAML Import

### 1.1 Architecture Fit

OpenCollection import follows the exact same pattern as existing HAR, Postman, and curl importers:

```
OpenCollection .yml files on disk
    → topencollection.ConvertOpenCollection()
        → OpenCollectionResolved (mhttp.HTTP, mfile.File, etc.)
            → importer.RunImport() + services.Create()
                → SQLite DB
```

| Layer | Location | Pattern |
|-------|----------|---------|
| CLI Command | `apps/cli/cmd/import.go` | Add `importBrunoCmd` |
| Translator | `packages/server/pkg/translate/topencollection/` | New package |
| Importer | `apps/cli/internal/importer/` | Existing `RunImport()` callback |

### 1.2 OpenCollection YAML Format Reference

#### Directory Structure

```
my-collection/
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

Each `.yml` request file has these top-level sections:

```yaml
info:
  name: Create User
  type: http               # http | graphql | grpc | ws
  seq: 5
  tags:
    - smoke
    - regression

http:
  method: POST             # GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD|TRACE|CONNECT
  url: https://api.example.com/users
  headers:
    - name: Content-Type
      value: application/json
    - name: Authorization
      value: "Bearer {{token}}"
      disabled: true        # Optional, marks header as disabled
  params:
    - name: filter
      value: active
      type: query           # query | path
    - name: id
      value: "123"
      type: path
  body:
    type: json              # json | xml | text | form-urlencoded | multipart-form | graphql | none
    data: |-
      {
        "name": "John Doe",
        "email": "john@example.com"
      }
  auth:                     # none | inherit | basic | bearer | apikey | digest | oauth2 | awsv4 | ntlm
    type: bearer
    token: "{{token}}"

runtime:
  scripts:
    - type: before-request
      code: |-
        const timestamp = Date.now();
        bru.setVar("timestamp", timestamp);
    - type: after-response
      code: |-
        console.log(res.status);
    - type: tests
      code: |-
        test("should return 201", function() {
          expect(res.status).to.equal(201);
        });
  assertions:
    - expression: res.status
      operator: eq
      value: "201"
    - expression: res.body.name
      operator: isString
  actions:
    - type: set-variable
      phase: after-response
      selector:
        expression: res.body.token
        method: jsonPath
      variable:
        name: auth_token
        scope: collection

settings:
  encodeUrl: true
  timeout: 0
  followRedirects: true
  maxRedirects: 5

docs: |-
  # Create User
  Creates a new user account in the system.
```

#### Auth Variants

```yaml
# No auth
auth: none

# Inherit from parent
auth: inherit

# Bearer token
auth:
  type: bearer
  token: "{{token}}"

# Basic auth
auth:
  type: basic
  username: admin
  password: secret

# API Key
auth:
  type: apikey
  key: x-api-key
  value: "{{api-key}}"
  placement: header        # header | query
```

#### Body Variants

```yaml
# JSON body
body:
  type: json
  data: |-
    {"key": "value"}

# XML body
body:
  type: xml
  data: |-
    <root><key>value</key></root>

# Text body
body:
  type: text
  data: "plain text content"

# Form URL-encoded
body:
  type: form-urlencoded
  data:
    - name: username
      value: johndoe
    - name: password
      value: secret123

# Multipart form data
body:
  type: multipart-form
  data:
    - name: file
      value: "@/path/to/file.pdf"
      contentType: application/pdf
    - name: description
      value: "My file"
```

#### Environment File (`environments/dev.yml`)

```yaml
name: development
variables:
  - name: api_url
    value: http://localhost:3000
    enabled: true
    secret: false
    type: text
```

#### Folder Config (`folder.yml`)

Contains folder-level metadata, auth, headers, and scripts that apply to all requests in the folder.

### 1.3 Go Types for OpenCollection Parsing

```go
package topencollection

// --- Collection Root ---

type OpenCollectionRoot struct {
    OpenCollection string               `yaml:"opencollection"`
    Info           OpenCollectionInfo    `yaml:"info"`
}

type OpenCollectionInfo struct {
    Name    string                    `yaml:"name"`
    Summary string                   `yaml:"summary,omitempty"`
    Version string                   `yaml:"version,omitempty"`
    Authors []OpenCollectionAuthor    `yaml:"authors,omitempty"`
}

type OpenCollectionAuthor struct {
    Name  string `yaml:"name"`
    Email string `yaml:"email,omitempty"`
}

// --- Request File ---

type OCRequest struct {
    Info     OCRequestInfo     `yaml:"info"`
    HTTP     *OCHTTPBlock      `yaml:"http,omitempty"`
    Runtime  *OCRuntime        `yaml:"runtime,omitempty"`
    Settings *OCSettings       `yaml:"settings,omitempty"`
    Docs     string            `yaml:"docs,omitempty"`
}

type OCRequestInfo struct {
    Name string   `yaml:"name"`
    Type string   `yaml:"type"`           // "http", "graphql", "grpc", "ws"
    Seq  int      `yaml:"seq,omitempty"`
    Tags []string `yaml:"tags,omitempty"`
}

type OCHTTPBlock struct {
    Method  string        `yaml:"method"`
    URL     string        `yaml:"url"`
    Headers []OCHeader    `yaml:"headers,omitempty"`
    Params  []OCParam     `yaml:"params,omitempty"`
    Body    *OCBody       `yaml:"body,omitempty"`
    Auth    *OCAuth       `yaml:"auth,omitempty"` // Can also be string "inherit"/"none"
}

type OCHeader struct {
    Name     string `yaml:"name"`
    Value    string `yaml:"value"`
    Disabled bool   `yaml:"disabled,omitempty"`
}

type OCParam struct {
    Name     string `yaml:"name"`
    Value    string `yaml:"value"`
    Type     string `yaml:"type"`             // "query" | "path"
    Disabled bool   `yaml:"disabled,omitempty"`
}

type OCBody struct {
    Type string      `yaml:"type"` // "json"|"xml"|"text"|"form-urlencoded"|"multipart-form"|"graphql"|"none"
    Data interface{} `yaml:"data"` // string for raw types, []OCFormField for form types
}

type OCFormField struct {
    Name        string `yaml:"name"`
    Value       string `yaml:"value"`
    Disabled    bool   `yaml:"disabled,omitempty"`
    ContentType string `yaml:"contentType,omitempty"` // For multipart file uploads
}

type OCAuth struct {
    Type      string `yaml:"type"`                  // "none"|"inherit"|"basic"|"bearer"|"apikey"|...
    Token     string `yaml:"token,omitempty"`        // For bearer
    Username  string `yaml:"username,omitempty"`     // For basic
    Password  string `yaml:"password,omitempty"`     // For basic
    Key       string `yaml:"key,omitempty"`          // For apikey
    Value     string `yaml:"value,omitempty"`        // For apikey
    Placement string `yaml:"placement,omitempty"`    // For apikey: "header"|"query"
}

// --- Runtime ---

type OCRuntime struct {
    Scripts    []OCScript    `yaml:"scripts,omitempty"`
    Assertions []OCAssertion `yaml:"assertions,omitempty"`
    Actions    []OCAction    `yaml:"actions,omitempty"`
}

type OCScript struct {
    Type string `yaml:"type"` // "before-request"|"after-response"|"tests"
    Code string `yaml:"code"`
}

type OCAssertion struct {
    Expression string `yaml:"expression"`
    Operator   string `yaml:"operator"`
    Value      string `yaml:"value,omitempty"`
}

type OCAction struct {
    Type     string     `yaml:"type"`     // "set-variable"
    Phase    string     `yaml:"phase"`    // "after-response"
    Selector OCSelector `yaml:"selector"`
    Variable OCVariable `yaml:"variable"`
}

type OCSelector struct {
    Expression string `yaml:"expression"`
    Method     string `yaml:"method"`
}

type OCVariable struct {
    Name  string `yaml:"name"`
    Scope string `yaml:"scope"`
}

// --- Settings ---

type OCSettings struct {
    EncodeUrl       *bool `yaml:"encodeUrl,omitempty"`
    Timeout         *int  `yaml:"timeout,omitempty"`
    FollowRedirects *bool `yaml:"followRedirects,omitempty"`
    MaxRedirects    *int  `yaml:"maxRedirects,omitempty"`
}

// --- Environment ---

type OCEnvironment struct {
    Name      string          `yaml:"name"`
    Variables []OCEnvVariable `yaml:"variables"`
}

type OCEnvVariable struct {
    Name    string `yaml:"name"`
    Value   string `yaml:"value"`
    Enabled *bool  `yaml:"enabled,omitempty"`
    Secret  *bool  `yaml:"secret,omitempty"`
    Type    string `yaml:"type,omitempty"` // "text"
}
```

### 1.4 Converter (`topencollection/converter.go`)

```go
package topencollection

// OpenCollectionResolved contains all entities extracted from an OpenCollection,
// following the same pattern as tpostmanv2.PostmanResolvedV2 and harv2.HARResolved.
type OpenCollectionResolved struct {
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
    WorkspaceID    idwrap.IDWrap
    FolderID       *idwrap.IDWrap  // Optional parent folder to import into
    CollectionName string          // Override collection name
    CreateFlow     bool            // Whether to generate a flow from the collection
    GenerateFiles  bool            // Whether to create File entries for hierarchy
}

// ConvertOpenCollection reads an OpenCollection YAML directory and returns resolved entities.
func ConvertOpenCollection(collectionPath string, opts ConvertOptions) (*OpenCollectionResolved, error)
```

#### Conversion Logic

1. **Detect format**: Check for `opencollection.yml` in the directory root
2. **Parse root config**: Read collection name, version from `opencollection.yml`
3. **Walk directory tree** (depth-first):
   - Skip ignored paths (`node_modules`, `.git`, dotenv files)
   - For each directory: create `mfile.File` with `ContentTypeFolder`
   - For each `.yml` file (not `opencollection.yml`, not `folder.yml`, not in `environments/`):
     - Parse with `yaml.Unmarshal()` into `OCRequest`
     - Convert to `mhttp.HTTP` + child entities
   - For `folder.yml`: extract folder metadata
   - For `environments/*.yml`: convert to `menv.Env` + `menv.Variable`
4. **Map to DevTools models** (see mapping table)
5. **Generate flow** (optional): Create a linear flow with request nodes ordered by `seq`

#### Mapping Table: OpenCollection YAML → DevTools

| OpenCollection YAML | DevTools Model | Notes |
|---|---|---|
| `info.name` | `mhttp.HTTP.Name` | |
| `info.seq` | `mfile.File.Order` | Float64 ordering |
| `info.type` | Determines request type | "http" only for now |
| `http.method` | `mhttp.HTTP.Method` | Uppercase |
| `http.url` | `mhttp.HTTP.Url` | |
| `http.headers` | `[]mhttp.HTTPHeader` | `disabled: true` → `Enabled: false` |
| `http.params` (type=query) | `[]mhttp.HTTPSearchParam` | Filter by `type: query` |
| `http.params` (type=path) | Embedded in URL | DevTools uses URL template vars |
| `http.body.type: json` | `mhttp.HTTPBodyRaw`, `BodyKind: Raw` | |
| `http.body.type: xml` | `mhttp.HTTPBodyRaw`, `BodyKind: Raw` | |
| `http.body.type: text` | `mhttp.HTTPBodyRaw`, `BodyKind: Raw` | |
| `http.body.type: form-urlencoded` | `[]mhttp.HTTPBodyUrlencoded` | |
| `http.body.type: multipart-form` | `[]mhttp.HTTPBodyForm` | |
| `http.auth.type: bearer` | `mhttp.HTTPHeader` | → `Authorization: Bearer <token>` |
| `http.auth.type: basic` | `mhttp.HTTPHeader` | → `Authorization: Basic <base64>` |
| `http.auth.type: apikey` | `mhttp.HTTPHeader` or `mhttp.HTTPSearchParam` | Based on `placement` |
| `runtime.assertions` | `[]mhttp.HTTPAssert` | Convert `expr operator value` format |
| `runtime.scripts` | Not imported (log warning) | DevTools uses JS nodes in flows |
| `runtime.actions` | Not imported (log warning) | DevTools uses flow variables |
| `docs` | `mhttp.HTTP.Description` | |
| `settings` | Not imported (log info) | DevTools has own request settings |
| Directory structure | `mfile.File` hierarchy | Folder nesting preserved |
| `folder.yml` | Folder metadata | Name used for folder |
| `environments/*.yml` | `menv.Env` + `menv.Variable` | |
| `opencollection.yml` | Collection config | Name used as root folder name |

### 1.5 Package Structure

```
packages/server/pkg/translate/topencollection/
├── types.go              # OCRequest, OCHTTPBlock, etc. (YAML struct definitions)
├── converter.go          # Main conversion: directory → OpenCollectionResolved
├── converter_test.go     # Tests with sample collections
├── collection.go         # opencollection.yml parsing
├── environment.go        # Environment .yml → menv conversion
├── auth.go               # Auth type → header/param conversion
├── body.go               # Body type → mhttp body conversion
└── testdata/             # Sample OpenCollection directories for tests
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
            ├── folder.yml
            └── login.yml
```

### 1.6 CLI Command

Add `import bruno <directory>` to the existing import command tree:

```go
// apps/cli/cmd/import.go — add alongside importCurlCmd, importPostmanCmd, importHarCmd

var importBrunoCmd = &cobra.Command{
    Use:   "bruno [directory]",
    Short: "Import a Bruno OpenCollection YAML directory",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        collectionPath := args[0]
        return importer.RunImport(ctx, logger, workspaceID, folderID,
            func(ctx context.Context, services *common.Services, wsID idwrap.IDWrap, folderIDPtr *idwrap.IDWrap) error {
                resolved, err := topencollection.ConvertOpenCollection(collectionPath, topencollection.ConvertOptions{
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

### 1.7 Testing Strategy

- **YAML parsing tests**: Parse sample `.yml` request files, verify struct output
- **Converter tests**: Use `testdata/` sample collections → verify `OpenCollectionResolved` output
- **Auth conversion tests**: Test all auth types (bearer, basic, apikey) → correct headers/params
- **Body conversion tests**: Test all body types (json, xml, form-urlencoded, multipart-form)
- **Integration test**: Import sample collection into in-memory SQLite → verify all entities created

---

## Part 2: DevTools YAML Folder Sync

This is the core feature — a bidirectional filesystem sync that uses DevTools' own YAML format, inspired by Bruno's chokidar-based sync but built for Go.

### 2.1 Design Goals

1. **Git-friendly**: YAML files that diff and merge cleanly
2. **Human-readable**: Developers can edit requests in their editor/IDE
3. **Bidirectional**: Changes in the UI persist to disk; changes on disk reflect in the UI
4. **Compatible**: Similar UX to Bruno's folder sync (users understand the concept)
5. **DevTools-native**: Uses DevTools' own YAML format
6. **OpenCollection-compatible import**: Can import from OpenCollection and sync in DevTools format

### 2.2 DevTools Collection YAML Format

Simplified for individual request files. Intentionally similar to OpenCollection for easy migration, but with DevTools-specific conventions.

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
    Version  string              `yaml:"version"`
    Name     string              `yaml:"name"`
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
    Type    string        `yaml:"type"`              // "none", "raw", "form-data", "urlencoded"
    Content string        `yaml:"content,omitempty"` // For raw bodies
    Fields  []HeaderEntry `yaml:"fields,omitempty"`  // For form-data / urlencoded
}

type AssertionEntry struct {
    Value       string `yaml:"value,omitempty"`
    Enabled     *bool  `yaml:"enabled,omitempty"`
    Description string `yaml:"description,omitempty"`
}

// Custom unmarshaler handles both "res.status eq 200" and {value: ..., enabled: false}

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
    Name        string          `yaml:"name,omitempty"`
    Order       int             `yaml:"order,omitempty"`
    Description string          `yaml:"description,omitempty"`
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
    Type    EventType
    Path    string // Absolute path
    RelPath string // Relative to collection root
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
    watcher  *CollectionWatcher
    parser   *tyamlcollection.Parser

    // Database → Filesystem
    selfWrites *SelfWriteTracker
    serializer *tyamlcollection.Serializer
    autosaver  *AutoSaver

    // State mapping
    mu       sync.RWMutex
    pathToID map[string]idwrap.IDWrap // filepath → entity ID (UID preservation)
    idToPath map[idwrap.IDWrap]string // entity ID → filepath (reverse mapping)

    // Services
    services *common.Services

    // Event publishing for real-time sync to UI
    publisher *eventstream.Publisher
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
devtools run ./my-collection                       # Run all requests sequentially
devtools run ./my-collection/users/get-users.yaml  # Run single request
devtools run ./my-collection --env dev             # With environment
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

### Phase 1: OpenCollection YAML Parser + Converter

**Scope**: Parse OpenCollection YAML directories and convert to DevTools models.

**Files**:
- `packages/server/pkg/translate/topencollection/types.go`
- `packages/server/pkg/translate/topencollection/converter.go`
- `packages/server/pkg/translate/topencollection/collection.go`
- `packages/server/pkg/translate/topencollection/environment.go`
- `packages/server/pkg/translate/topencollection/auth.go`
- `packages/server/pkg/translate/topencollection/body.go`
- `packages/server/pkg/translate/topencollection/converter_test.go`
- `packages/server/pkg/translate/topencollection/testdata/` (sample collections)

**Dependencies**: `gopkg.in/yaml.v3` (already a dependency), existing models (`mhttp`, `mfile`, `menv`)

**Testing**: Parse sample OpenCollection directories, verify all entities

### Phase 2: CLI Import Command

**Scope**: Add `import bruno <dir>` CLI command.

**Files**:
- `apps/cli/cmd/import.go` (add `importBrunoCmd`)

**Dependencies**: Phase 1 (converter), existing importer infrastructure

**Testing**: End-to-end import into in-memory SQLite

### Phase 3: DevTools YAML Collection Format

**Scope**: Define and implement DevTools' own YAML format for individual request files with round-trip serialization.

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

### Phase 4: File Watcher

**Scope**: `fsnotify`-based watcher with debouncing, filtering, self-write tracking.

**Files**:
- `packages/server/pkg/foldersync/watcher.go`
- `packages/server/pkg/foldersync/debouncer.go`
- `packages/server/pkg/foldersync/filter.go`
- `packages/server/pkg/foldersync/types.go`
- `packages/server/pkg/foldersync/watcher_test.go`

**Dependencies**: `github.com/fsnotify/fsnotify`

**Testing**: Create temp dirs, write files, verify events

### Phase 5: Sync Coordinator

**Scope**: Bidirectional sync engine — disk↔database with eventstream integration.

**Files**:
- `packages/server/pkg/foldersync/sync.go`
- `packages/server/pkg/foldersync/autosaver.go`

**Dependencies**: Phase 3 (YAML format), Phase 4 (watcher), services, eventstream

**Testing**: Full integration tests — modify files on disk, verify DB updates; modify DB, verify files written

### Phase 6: RPC Endpoints + Desktop Integration

**Scope**: TypeSpec definitions for folder sync management, RPC handlers, desktop UI.

**Files**:
- `packages/spec/` (TypeSpec definitions)
- `packages/server/internal/api/` (RPC handlers)
- `packages/client/` (React hooks/services)
- `apps/desktop/` (Electron integration — folder picker, sync status)

**Dependencies**: Phase 5 (sync coordinator)

### Phase 7: CLI Collection Runner

**Scope**: Run requests/flows directly from collection folders.

**Files**:
- `apps/cli/cmd/run.go` (extend with `collection` subcommand)

**Dependencies**: Phase 3 (YAML format), existing runner

---

## Phase Dependency Graph

```
Phase 1: OpenCollection Parser ──→ Phase 2: CLI Import
                                          │
Phase 3: DevTools YAML Format ──┬─────────┼──→ Phase 7: CLI Collection Runner
                                │         │
Phase 4: File Watcher ──────────┤         │
                                │         │
                                └──→ Phase 5: Sync Coordinator ──→ Phase 6: RPC + Desktop
```

Phases 1 and 3-4 can be developed **in parallel** (no dependencies between them).

---

## External Dependencies

| Dependency | Purpose | Phase |
|---|---|---|
| `gopkg.in/yaml.v3` | YAML parsing (already in use by `yamlflowsimplev2`) | 1, 3 |
| `github.com/fsnotify/fsnotify` | Cross-platform file system notifications | 4 |

---

## Key Design Decisions

### Why OpenCollection YAML only (no .bru parser)?

1. **Bruno's direction** — OpenCollection YAML is the recommended format going forward
2. **No custom parser** — standard `gopkg.in/yaml.v3` handles everything
3. **Forward-compatible** — .bru is being phased out
4. **Less code to maintain** — no hand-written PEG parser
5. **Better tooling** — YAML linting, schema validation, IDE support

### Why a separate DevTools YAML format?

1. **Control** — we define the schema, we evolve it independently
2. **Simplicity** — OpenCollection has sections we don't use (`runtime.scripts`, `runtime.actions`)
3. **Flat structure** — our format puts `name`, `method`, `url` at top level (no `info`/`http` nesting)
4. **Consistency** — matches existing `yamlflowsimplev2` conventions
5. **Import, don't adopt** — import OpenCollection, sync in DevTools format

### Why fsnotify instead of polling?

1. **Efficient** — kernel-level notifications, no CPU overhead from polling
2. **Low latency** — events arrive in milliseconds vs polling interval
3. **Standard** — most Go file-watching libraries use fsnotify
4. **Exception**: WSL paths use polling (matching Bruno's approach)

### Why autosave debounce at 500ms?

1. **Matches Bruno** — users expect similar behavior
2. **Prevents rapid writes** — typing in URL field doesn't cause per-keystroke disk writes
3. **Balances responsiveness** — changes appear on disk within ~500ms, fast enough for git workflows
