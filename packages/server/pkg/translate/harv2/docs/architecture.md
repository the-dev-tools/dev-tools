# HAR Import Architecture (v2)

This document describes the architecture of the HAR importer (`harv2`), focusing on how it translates linear HAR entries into the relational model of the application (Flows, Nodes, HTTP Requests, and Files).

## Entity Creation Flow

The importer processes each HAR entry to create a set of interconnected entities. It specifically handles "Delta" requests to allow for overrides and alternative configurations while maintaining a clean file system structure.

```text
                                  [HAR Import Process]
                                           |
                                           v
                                  +------------------+
                                  | Parse HAR Entry  |
                                  +------------------+
                                           |
                                           | (1) Create Models
                                           v
        +----------------------+    +----------------------+    +----------------------+
        | mhttp.HTTP (Base)    |    | mhttp.HTTP (Delta)   |    | mnnode.MNode         |
        |----------------------|    |----------------------|    |----------------------|
        | ID: HTTP_1           |    | ID: HTTP_DELTA_1     |    | ID: NODE_1           |
        | Method: GET          |    | Method: GET          |    | Kind: REQUEST        |
        | Url: /api/v1/users   |    | IsDelta: true        |    | Name: request_1      |
        | ...                  |    | ParentHttpID: HTTP_1 |    | ...                  |
        +----------+-----------+    +----------+-----------+    +----------+-----------+
                   ^                           ^                           ^
                   |                           |                           |
                   | (2) Link to Node          | (3) Link to Node          |
                   +------------+  +-----------+                           |
                                |  |                                       |
                                |  |                                       |
                       +--------+--+---------+                             |
                       | mnrequest.MNRequest | ----------------------------+
                       |---------------------| (4) Config for Node
                       | FlowNodeID: NODE_1  |
                       | HttpID: *HTTP_1     |
                       | DeltaHttpID:        |
                       |   *HTTP_DELTA_1     |
                       +---------------------+

                                           |
                                           | (5) Create File System Entries
                                           v
        +----------------------+                        +----------------------+
        | mfile.File (Base)    |                        | mfile.File (Delta)   |
        |----------------------|                        |----------------------|
        | ID: HTTP_1           |                        | ID: HTTP_DELTA_1     |
        | Type: HTTP           |                        | Type: HTTP_DELTA     |
        | FolderID: FOLDER_A   |<--[Same Folder]------->| FolderID: FOLDER_A   |
        | ContentID: HTTP_1    |                        | ContentID:           |
        +----------------------+                        |   HTTP_DELTA_1       |
                                                        +----------------------+

                                           |
                                           | (6) Dependencies (Edges)
                                           v
                                  +------------------+
                                  | edge.Edge        |
                                  |------------------|
                                  | Source: NODE_0   | (Previous Request)
                                  | Target: NODE_1   | (This Request)
                                  +------------------+
```

## Dependency Resolution (DepFinder)

The `depfinder` component is responsible for detecting data dependencies between requests. It scans responses from previous requests and identifies if those values are used in subsequent requests.

### How it works

1.  **Indexing (Response Processing):** When a response is encountered in the HAR log, `depfinder` parses it (e.g., flattens JSON) and indexes unique values.
2.  **Matching (Request Processing):** When a new request is processed, its fields (URL, Headers, Body) are scanned.
3.  **Replacement:** If a value matches an indexed value from a previous response, it is replaced with a template reference (e.g., `{{request_1.response.body.id}}`).

```text
   [Previous Request]                     [DepFinder Index]
   "request_1"                            +-----------------------+
        |                                 | Value   | Source Path |
        v                                 |---------|-------------|
   Response:                              | "123"   | request_1.  |
   {                                      |         | response.   |
     "id": "123",  ---------------------> |         | body.id     |
     "token": "abc"                       +-----------------------+
   }                                                  ^
                                                      |
                                                      | (Lookup "123")
                                                      |
   [Current Request]                                  |
   "request_2"                                        |
        |                                             |
        v                                             |
   Request:                                           |
   GET /users/123  -----------------------------------+
        |
        | (Replace Match)
        v
   Result Model (mhttp.HTTP):
   GET /users/{{request_1.response.body.id}}
```

### Integration with Delta System

When a dependency is found and replaced in the **Base Request** (`mhttp.HTTP`), the **Delta Request** is created as a copy _after_ this replacement.

- **Base Request:** `Url: /users/{{...}}`
- **Delta Request:** `Url: /users/{{...}}` (Inherits the template)

This ensures that both the base definition and the active delta configuration correctly reference the dynamic data from the previous step in the flow.
