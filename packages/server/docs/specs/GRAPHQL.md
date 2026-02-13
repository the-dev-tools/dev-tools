# GraphQL Specification

## Overview

The GraphQL system adds first-class GraphQL request support to DevTools. It enables users to compose GraphQL queries/mutations, execute them against any GraphQL endpoint, introspect schemas for autocompletion and documentation, and view responses -- all following the same architecture patterns as the existing HTTP system.

## Reference Implementation

This design is informed by [Bruno](https://github.com/usebruno/bruno)'s GraphQL implementation, adapted to DevTools' TypeScript + Go stack (TypeSpec, Connect RPC, TanStack React DB, CodeMirror 6).

### What Bruno Does

- **Query Editor**: CodeMirror with `codemirror-graphql` for syntax highlighting, schema-aware autocompletion, real-time validation, and query formatting via Prettier
- **Variables Editor**: JSON editor for GraphQL variables with prettify support
- **Schema Introspection**: Fetches schema via standard introspection query (`getIntrospectionQuery()` from `graphql` lib), caches result, builds `GraphQLSchema` object via `buildClientSchema()`
- **Documentation Explorer**: Custom component that navigates the `GraphQLSchema` type map with breadcrumb navigation, search, and clickable type references
- **Request Execution**: HTTP POST with `Content-Type: application/json`, body `{ "query": "...", "variables": {...} }`
- **Tabbed UI**: Query (default), Variables, Headers, Auth, Docs tabs

### What We Include

- Query editor with schema-aware autocompletion and validation (via `cm6-graphql` for CodeMirror 6)
- Variables editor (JSON)
- Headers (key-value table for manual auth and custom headers)
- Schema introspection and caching in SQLite
- Documentation explorer
- Request execution and response display

### What We Exclude (For Now)

- **Scripts/hooks**: Pre/post-request scripts (not needed)
- **Variable extraction**: Already handled automatically by DevTools
- **Auth UI**: Users set auth manually via headers; dedicated auth UI added later
- **Delta system**: Not needed initially; can be added later

---

## Core Concepts

### 1. Request Definition

A GraphQL request defines what to send to a GraphQL endpoint.

- **URL**: The GraphQL endpoint (e.g., `https://api.example.com/graphql`)
- **Query**: The GraphQL query/mutation string
- **Variables**: JSON string of variables to pass with the query
- **Headers**: Key-value pairs with enable/disable toggle (used for auth tokens, custom headers)

Unlike HTTP requests, GraphQL is always:
- Method: **POST**
- Content-Type: **application/json**
- Body: `{ "query": "...", "variables": {...} }`

### 2. Schema Introspection

GraphQL's self-documenting nature is a key feature:

1. User clicks "Fetch Schema" in the UI
2. Backend sends the standard introspection query to the endpoint (with user's headers for auth)
3. Backend returns the raw introspection JSON
4. Frontend builds a `GraphQLSchema` object via `buildClientSchema()` from the `graphql` JS library
5. Schema enables: autocompletion in the query editor, validation/linting, and the documentation explorer

Schema introspection results are stored in SQLite (not localStorage like Bruno) for persistence and consistency.

### 3. Execution & Response

When a GraphQL request is "Run":

1. **Interpolation**: Variables (`{{ varName }}`) are substituted into URL, query, variables, and header values
2. **Construction**: Build JSON body `{ "query": "...", "variables": {...} }`
3. **Transmission**: HTTP POST via the existing Go HTTP client (`httpclient` package)
4. **Response**: Status, headers, body (JSON), timing, and size are captured
5. **Persistence**: Response stored in `graphql_response` table, linked to the GraphQL request

---

## Architecture

### Design Decision: Separate Entity Type

GraphQL is a **new entity type** rather than an extension of HTTP because:

1. HTTP's `BodyKind` enum (`FormData`/`UrlEncoded`/`Raw`) doesn't conceptually fit GraphQL's `query + variables` model
2. GraphQL requires schema storage -- an entirely new concern that doesn't belong on HTTP
3. Execution is fundamentally simpler (always POST, always JSON, fixed body structure)
4. Follows the existing pattern where each protocol is its own entity

GraphQL does **not** use the delta system initially to keep scope manageable.

### File System Integration

A new `GraphQL` value is added to the `FileKind` enum in `file-system.tsp`, allowing GraphQL requests to appear in the workspace sidebar tree alongside HTTP requests and flows.

---

## Backend

### API Layer (`packages/server/internal/api/rgraphql`)

- **Role**: Entry point for Connect RPC
- **Responsibilities**:
  - Validates incoming Protobuf messages
  - Orchestrates transactions (Fetch-Check-Act pattern)
  - Calls the Service Layer
  - Publishes events to `eventstream` for real-time UI updates
- **Key RPC Operations**:
  - `GraphQLRun`: Execute a GraphQL request
  - `GraphQLIntrospect`: Fetch schema via introspection query
  - `GraphQLDuplicate`: Clone a GraphQL request
  - Standard CRUD for GraphQL entity and headers
  - Streaming sync for TanStack DB real-time collections
- **Files**: `rgraphql.go` (service struct, streamers), `rgraphql_exec.go` (execution), `rgraphql_crud.go` (management), `rgraphql_sync.go` (streaming)

### Service Layer (`packages/server/pkg/service/sgraphql`)

- **Role**: Business logic and data access adapter
- **Pattern**: Reader (non-blocking, `*sql.DB`) + Writer (transactional, `*sql.Tx`)
- **Responsibilities**:
  - Converts between Internal Models (`mgraphql`) and DB Models (`gen`)
  - Executes `sqlc` queries
  - Handles duplication logic (copying headers)

### Domain Model (`packages/server/pkg/model/mgraphql`)

Pure Go structs decoupled from DB and API:

```go
type GraphQL struct {
    ID          idwrap.IDWrap
    WorkspaceID idwrap.IDWrap
    FolderID    *idwrap.IDWrap
    Name        string
    Url         string
    Query       string      // GraphQL query/mutation string
    Variables   string      // JSON string of variables
    Description string
    LastRunAt   *int64
    CreatedAt   int64
    UpdatedAt   int64
}

type GraphQLHeader struct {
    ID           idwrap.IDWrap
    GraphQLID    idwrap.IDWrap
    Key          string
    Value        string
    Description  string
    Enabled      bool
    DisplayOrder float32
}

type GraphQLResponse struct {
    ID        idwrap.IDWrap
    GraphQLID idwrap.IDWrap
    Status    int32
    Body      []byte
    Time      int64
    Duration  int32
    Size      int32
}

type GraphQLResponseHeader struct {
    ID         idwrap.IDWrap
    ResponseID idwrap.IDWrap
    Key        string
    Value      string
}
```

### GraphQL Executor (`packages/server/pkg/graphql/executor.go`)

Analogous to `packages/server/pkg/http/request/request.go` but simpler:

```go
func PrepareGraphQLRequest(gql mgraphql.GraphQL, headers []mgraphql.GraphQLHeader, varMap map[string]any) (*http.Request, error)
func PrepareIntrospectionRequest(url string, headers []mgraphql.GraphQLHeader, varMap map[string]any) (*http.Request, error)
```

Both always produce HTTP POST with `Content-Type: application/json`. The introspection variant uses the well-known introspection query string.

---

## Database Schema

### Tables

- **`graphql`**: Core request metadata (name, url, query, variables)
- **`graphql_header`**: Request headers (key, value, enabled, order)
- **`graphql_response`**: Execution results (status, body, duration, size)
- **`graphql_response_header`**: Response headers

No delta fields. No assertions table (can be added later).

Schema file: `packages/db/pkg/sqlc/schema/08_graphql.sql`

---

## Frontend

### CodeMirror 6 GraphQL Integration

- **Package**: `cm6-graphql` (official CM6 GraphQL extension from GraphiQL monorepo)
- **Features**: Syntax highlighting, schema-aware autocompletion, linting/validation
- **Location**: `packages/client/src/features/graphql-editor/index.tsx`
- **Hook**: `useGraphQLEditorExtensions(schema?: GraphQLSchema)` returns CM6 extensions

Also adds `'graphql'` to the prettier language support in `packages/client/src/features/expression/prettier.tsx`.

### Page Components (`packages/client/src/pages/graphql/`)

Following the pattern of `packages/client/src/pages/http/`:

| Component | Description |
|-----------|-------------|
| `page.tsx` | Main page with resizable request/response split panels |
| `request/top-bar.tsx` | URL input, Send button, Fetch Schema button |
| `request/panel.tsx` | Tabbed panel: Query, Variables, Headers, Docs |
| `request/query-editor.tsx` | CodeMirror with `cm6-graphql` extensions |
| `request/variables-editor.tsx` | CodeMirror with JSON language |
| `request/header.tsx` | Headers key-value table |
| `request/doc-explorer.tsx` | Schema documentation browser |
| `response/body.tsx` | Response body viewer (JSON syntax highlighting) |

### Documentation Explorer

Custom component (not importing GraphiQL's, which has heavy context dependencies):

- **Navigation**: Stack-based with breadcrumbs (root -> type -> field)
- **Root view**: Lists Query, Mutation, Subscription root types
- **Type view**: Fields with types, arguments, descriptions
- **Search**: Debounced filter across type/field names
- **Type links**: Clickable references that push onto navigation stack
- **Built with**: React Aria components, Tailwind CSS, `graphql` JS library's type introspection APIs

### Routing

Route: `/(dashboard)/(workspace)/workspace/$workspaceIdCan/(graphql)/graphql/$graphqlIdCan/`

Added to `packages/client/src/shared/routes.tsx` and sidebar file tree handler.

---

## TypeSpec Definition

File: `packages/spec/api/graphql.tsp`

```typespec
using DevTools;
namespace Api.GraphQL;

@TanStackDB.collection
model GraphQL {
  @primaryKey graphqlId: Id;
  name: string;
  url: string;
  query: string;
  variables: string;
  lastRunAt?: Protobuf.WellKnown.Timestamp;
}

@TanStackDB.collection
model GraphQLHeader {
  @primaryKey graphqlHeaderId: Id;
  @foreignKey graphqlId: Id;
  key: string;
  value: string;
  enabled: boolean;
  description: string;
  order: float32;
}

@TanStackDB.collection(#{ isReadOnly: true })
model GraphQLResponse {
  @primaryKey graphqlResponseId: Id;
  @foreignKey graphqlId: Id;
  status: int32;
  body: string;
  time: Protobuf.WellKnown.Timestamp;
  duration: int32;
  size: int32;
}

@TanStackDB.collection(#{ isReadOnly: true })
model GraphQLResponseHeader {
  @primaryKey graphqlResponseHeaderId: Id;
  @foreignKey graphqlResponseId: Id;
  key: string;
  value: string;
}

model GraphQLRunRequest { graphqlId: Id; }
op GraphQLRun(...GraphQLRunRequest): {};

model GraphQLDuplicateRequest { graphqlId: Id; }
op GraphQLDuplicate(...GraphQLDuplicateRequest): {};

model GraphQLIntrospectRequest { graphqlId: Id; }
model GraphQLIntrospectResponse { sdl: string; introspectionJson: string; }
op GraphQLIntrospect(...GraphQLIntrospectRequest): GraphQLIntrospectResponse;
```

---

## Implementation Order

1. TypeSpec + code generation (`graphql.tsp`, `FileKind.GraphQL`, run `spec:build`)
2. Database schema + sqlc (`08_graphql.sql`, queries, `sqlc.yaml`, run `db:generate`)
3. Go models (`mgraphql/`)
4. Go services (`sgraphql/` - reader, writer, mapper for each entity)
5. Go executor (`pkg/graphql/executor.go`)
6. Go RPC handlers (`rgraphql/` - CRUD, exec, introspect, sync)
7. Server wiring (`server.go` - streamers, services, cascade handlers)
8. Frontend packages (`cm6-graphql`, `graphql` npm deps)
9. Frontend components (pages, editor, doc explorer, routes)

---

## Files Changed / Created

### New Files

```
packages/spec/api/graphql.tsp
packages/db/pkg/sqlc/schema/08_graphql.sql
packages/db/pkg/sqlc/queries/graphql.sql
packages/server/pkg/model/mgraphql/mgraphql.go
packages/server/pkg/service/sgraphql/  (sgraphql.go, reader.go, writer.go, mapper.go, header*.go, response*.go)
packages/server/pkg/graphql/executor.go
packages/server/internal/api/rgraphql/  (rgraphql.go, _crud.go, _crud_header.go, _exec.go, _converter.go, _sync.go)
packages/client/src/features/graphql-editor/index.tsx
packages/client/src/pages/graphql/  (page.tsx, tab.tsx, request/*, response/*, routes/*)
```

### Modified Files

```
packages/spec/api/main.tsp                          (add graphql.tsp import)
packages/spec/api/file-system.tsp                   (add GraphQL to FileKind)
packages/db/pkg/sqlc/sqlc.yaml                      (add graphql column overrides)
packages/server/cmd/server/server.go                 (wire services, streamers, cascade)
packages/client/package.json                         (add cm6-graphql, graphql deps)
packages/client/src/shared/routes.tsx                (add GraphQL routes)
packages/client/src/features/expression/prettier.tsx (add graphql language)
```

---

## Verification

1. `direnv exec . pnpm nx run spec:build` succeeds
2. `direnv exec . pnpm nx run db:generate` succeeds
3. `direnv exec . pnpm nx run server:dev` starts without errors
4. `direnv exec . pnpm nx run client:dev` builds successfully
5. `direnv exec . task lint` passes
6. `direnv exec . task test` passes
7. E2E: Create GraphQL request -> enter endpoint -> write query -> fetch schema -> verify autocompletion -> send request -> verify response display -> browse docs
