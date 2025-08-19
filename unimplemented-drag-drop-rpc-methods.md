# Unimplemented Drag & Drop RPC Methods

This document lists all RPC methods related to drag/drop and ordering functionality in DevTools. These methods are either defined but unimplemented (returning empty responses) or completely missing from the API specification.

## 🔄 Currently Defined But Unimplemented

These methods exist in the TypeSpec API and have Go handlers, but only return empty responses:

### Collection Management
- **`CollectionMove`** - Move collections within workspace
  - **Location**: `packages/spec/api/collection.tsp:44`, `packages/server/internal/api/rcollection/rcollection.go:237`
  - **Request**: `CollectionMoveRequest` with position, targetCollectionId
  - **Response**: `CollectionMoveResponse` (empty)

- **`CollectionItemMove`** - Move items between collections/folders  
  - **Location**: `packages/spec/api/collection-item/item.tsp:69`, `packages/server/internal/api/rcollectionitem/collectionitem.go:177`
  - **Request**: `CollectionItemMoveRequest` with itemId, collectionId, parentFolderId, targetCollectionId, targetParentFolderId, position
  - **Response**: `CollectionItemMoveResponse` (empty)

### Workspace & Environment
- **`WorkspaceMove`** - Move workspaces
  - **Location**: `packages/spec/api/workspace.tsp:73`, `packages/server/internal/api/rworkspace/rworkspace.go:536`
  - **Request**: `WorkspaceMoveRequest` with position, targetWorkspaceId
  - **Response**: `WorkspaceMoveResponse` (empty)

- **`EnvironmentMove`** - Move environments
  - **Location**: `packages/spec/api/environment.tsp:45`, `packages/server/internal/api/renv/renv.go:192`
  - **Request**: `EnvironmentMoveRequest` with position, targetEnvironmentId
  - **Response**: `EnvironmentMoveResponse` (empty)

### Variables
- **`VariableMove`** - Move global variables
  - **Location**: `packages/spec/api/variable.tsp:53`, `packages/server/internal/api/rvar/rvar.go:226`
  - **Request**: `VariableMoveRequest` with position, targetVariableId
  - **Response**: `VariableMoveResponse` (empty)

- **`FlowVariableMove`** - Move flow-specific variables
  - **Location**: `packages/spec/api/flow-variable.tsp:44`, `packages/server/internal/api/rflowvariable/rflowvariable.go:302`
  - **Request**: `FlowVariableMoveRequest` with position, targetFlowVariableId
  - **Response**: `FlowVariableMoveResponse` (empty)

### Request Components
- **`QueryMove`** / **`QueryDeltaMove`** - Move query parameters
  - **Location**: `packages/spec/api/collection-item/request.tsp:122,124`, `packages/server/internal/api/rrequest/rrequest.go:2266,2271`
  - **Request**: `QueryMoveRequest`/`QueryDeltaMoveRequest` with position, targetQueryId
  - **Response**: `QueryMoveResponse`/`QueryDeltaMoveResponse` (empty)

- **`HeaderMove`** / **`HeaderDeltaMove`** - Move request headers
  - **Location**: `packages/spec/api/collection-item/request.tsp:126,128`, `packages/server/internal/api/rrequest/rrequest.go:2276,2281`
  - **Request**: `HeaderMoveRequest`/`HeaderDeltaMoveRequest` with position, targetHeaderId
  - **Response**: `HeaderMoveResponse`/`HeaderDeltaMoveResponse` (empty)

- **`BodyFormMove`** / **`BodyFormDeltaMove`** - Move form body fields
  - **Location**: `packages/spec/api/collection-item/body.tsp:123,125`, `packages/server/internal/api/rbody/rbody.go:1017,1022`
  - **Request**: `BodyFormMoveRequest`/`BodyFormDeltaMoveRequest` with position, targetBodyFormId
  - **Response**: `BodyFormMoveResponse`/`BodyFormDeltaMoveResponse` (empty)

- **`BodyUrlEncodedMove`** / **`BodyUrlEncodedDeltaMove`** - Move URL-encoded body fields
  - **Location**: `packages/spec/api/collection-item/body.tsp:127,129`, `packages/server/internal/api/rbody/rbody.go:1027,1032`
  - **Request**: `BodyUrlEncodedMoveRequest`/`BodyUrlEncodedDeltaMoveRequest` with position, targetBodyUrlEncodedId
  - **Response**: `BodyUrlEncodedMoveResponse`/`BodyUrlEncodedDeltaMoveResponse` (empty)

### Examples
- **`ExampleMove`** - Move API examples
  - **Location**: `packages/spec/api/collection-item/example.tsp:112`, `packages/server/internal/api/ritemapiexample/ritemapiexample.go:1352`
  - **Request**: `ExampleMoveRequest` with position, targetExampleId
  - **Response**: `ExampleMoveResponse` (empty)

## ❌ Missing Move Methods

These methods are completely missing from the API specification and need to be defined:

### Flow Components
- **`NodeMove`** - Move flow nodes within canvas
  - **Purpose**: Drag/drop positioning of nodes in flow editor
  - **Needed**: Position updates, connection validation
  - **Priority**: High (core flow editor functionality)

- **`EdgeMove`** - Move/reorder flow edges
  - **Purpose**: Reorder connection priority, visual organization
  - **Needed**: Edge sequencing, handle management
  - **Priority**: Medium (visual organization)

### Collection Structure
- **`FolderMove`** - Move folders within collections
  - **Purpose**: Reorganize folder hierarchy via drag/drop
  - **Needed**: Parent-child relationship updates
  - **Priority**: High (basic organization)

- **`EndpointMove`** - Move endpoints within folders
  - **Purpose**: Reorder API endpoints within folder structure
  - **Needed**: Position tracking, folder assignment
  - **Priority**: Medium (organization)

### Organizational
- **`TagMove`** - Move/reorder tags
  - **Purpose**: Reorder tag display, priority management
  - **Needed**: Display order, visual organization
  - **Priority**: Low (cosmetic)

## 📋 Move Position Infrastructure

All move operations use the `Resource.MovePosition` enum:

```typescript
enum MovePosition {
  MOVE_POSITION_UNSPECIFIED: 0,
  MOVE_POSITION_AFTER: 1,
  MOVE_POSITION_BEFORE: 2,
}
```

## 🎯 Implementation Priority

### High Priority (Core Functionality)
1. `CollectionItemMove` - Basic collection organization
2. `FolderMove` - Folder hierarchy management
3. `NodeMove` - Flow editor drag/drop
4. `HeaderMove`/`HeaderDeltaMove` - Request building

### Medium Priority (Enhanced UX)
5. `QueryMove`/`QueryDeltaMove` - Query parameter organization
6. `BodyFormMove`/`BodyFormDeltaMove` - Form data organization
7. `ExampleMove` - Example organization
8. `EndpointMove` - API endpoint organization

### Lower Priority (Polish)
9. `VariableMove`/`FlowVariableMove` - Variable organization
10. `EnvironmentMove` - Environment organization
11. `WorkspaceMove` - Workspace organization
12. `EdgeMove` - Flow edge organization
13. `TagMove` - Tag organization

## 🔧 Implementation Notes

- All implemented methods currently return empty responses
- Move operations require proper database transaction handling
- UI components need integration with drag/drop libraries
- Delta system integration needed for request component moves
- Flow nodes have special positioning requirements (x, y coordinates)
- Validation needed for circular dependencies and invalid moves