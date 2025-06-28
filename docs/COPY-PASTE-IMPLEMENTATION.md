# Copy-Paste Implementation Guide

This document describes the implementation of the node duplication and copy-paste functionality in the DevTools flow builder.

## Overview

The copy-paste system allows users to duplicate nodes in the flow builder through two mechanisms:
1. **Context Menu Duplication** - Right-click on a single node and select "Duplicate"
2. **Keyboard Copy/Paste** - Select nodes and use Ctrl+C/Ctrl+V (or Cmd+C/Cmd+V on Mac)

## Architecture

### File Structure

```
packages/client/src/flow/
├── copy-paste.ts      # Core copy-paste logic (new file)
├── flow.tsx           # Minimal changes for keyboard handling
└── node.tsx           # Minimal changes for context menu
```

### Design Principles

1. **Modularity** - All copy-paste logic is contained in `copy-paste.ts`
2. **Minimal Integration** - Changes to existing files are kept to a minimum
3. **Type Safety** - Proper TypeScript types throughout
4. **Delta Preservation** - All node-specific modifications are preserved

## Implementation Details

### Core Functions

#### `copyNode(options: CopyNodeOptions): Promise<CopyNodeResult>`

The main function that handles copying a single node with all its data.

**Parameters:**
- `dataClient` - Data client for API calls
- `makeNode` - Function to create new nodes
- `makeEdge` - Function to create new edges
- `nameTransform` - Optional function to transform node names (defaults to adding "_copy")
- `nodeId` - ID of the node to copy
- `position` - Position for the new node

**Behavior:**
- Fetches full node data including all properties
- For REQUEST nodes:
  - Creates new delta endpoint and example (hidden entities)
  - Copies HTTP method and URL from original delta endpoint
  - Copies all headers, query parameters, and body data
- For control flow nodes (CONDITION, FOR, FOR_EACH):
  - Creates required child nodes (THEN, ELSE, LOOP)
  - Connects them with appropriate edges
- For other nodes:
  - Duplicates with transformed name

#### `copyDeltaData(dataClient, originalDeltaExampleId, deltaExampleId, exampleId)`

Internal function that copies all delta data from one example to another.

**What it copies:**
1. **Headers** - Only overridden headers (where `source !== SourceKind.ORIGIN`)
2. **Query Parameters** - Only overridden parameters
3. **Body Data** - For RAW body type, copies the entire body content

**Important:** Variable references (e.g., `{{ request_0.response.body.token }}`) are preserved as-is, not resolved to their values.

#### `setupCopyPasteHandlers(...)`

Sets up keyboard event handlers for copy/paste functionality.

**Features:**
- Captures selected nodes on Ctrl+C
- Preserves edges between selected nodes
- Maintains relative positioning when pasting
- Creates independent copies with new IDs

#### `duplicateNodeFromMenu(...)`

Simplified function for single-node duplication from context menu.

**Features:**
- Takes node position directly to avoid type conflicts
- Uses callback pattern for better type safety
- Positions new node 50px offset from original

### Data Flow

1. **Copy Operation (Ctrl+C)**
   ```typescript
   Selected Nodes → Fetch Full Data → Store in Memory
   ```

2. **Paste Operation (Ctrl+V)**
   ```typescript
   Stored Nodes → Calculate Positions → Create New Nodes → Map IDs → Recreate Edges
   ```

3. **Context Menu Duplicate**
   ```typescript
   Single Node → Copy with Offset → Add to Flow
   ```

### REQUEST Node Specifics

REQUEST nodes are special because they reference external data:

```typescript
{
  collectionId: Uint8Array,      // Collection this request belongs to
  endpointId: Uint8Array,         // Original endpoint (template)
  exampleId: Uint8Array,          // Original example (template)
  deltaEndpointId: Uint8Array,    // Node-specific endpoint overrides
  deltaExampleId: Uint8Array      // Node-specific example overrides
}
```

When duplicating:
1. New delta endpoint is created (hidden)
2. HTTP method and URL are copied from original delta endpoint
3. New delta example is created (hidden)
4. All headers, queries, and body are copied to new delta

### Edge Handling

- **Single Node Duplication**: No edges are copied
- **Multi-Node Copy/Paste**: Only edges between selected nodes are recreated
- **Control Flow Nodes**: Automatically create required edges to child nodes

## Variable References

The system preserves variable references in their template form:

```
Original: Authorization: Bearer {{ login_request.response.body.token }}
Copy:     Authorization: Bearer {{ login_request.response.body.token }}
```

This ensures that copied nodes maintain their dynamic behavior rather than copying static values.

## Error Handling

All API operations are wrapped in try-catch blocks with console error logging:
- Header copy failures don't block the operation
- Query parameter copy failures are logged but non-fatal
- Body copy failures are handled gracefully

## Future Considerations

1. **Undo/Redo** - The system doesn't currently integrate with undo/redo
2. **Cross-Flow Copy** - Currently only works within the same flow
3. **Performance** - Large node selections may cause UI lag during paste
4. **Persistence** - Copied nodes are only stored in memory

## Testing Scenarios

1. **Single REQUEST Node**
   - Verify delta endpoint/example creation
   - Check HTTP method preservation
   - Validate header/query/body copying

2. **Control Flow Nodes**
   - Ensure child nodes are created
   - Verify edge connections
   - Check positioning of child nodes

3. **Multi-Node Selection**
   - Test edge preservation
   - Verify relative positioning
   - Check ID mapping

4. **Variable References**
   - Ensure variables aren't resolved
   - Verify syntax preservation
   - Test with complex expressions

## Code References

Key locations in the codebase:
- Node duplication logic: `packages/client/src/flow/copy-paste.ts:171`
- HTTP method copying: `packages/client/src/flow/copy-paste.ts:276`
- Header/query copying: `packages/client/src/flow/copy-paste.ts:65`
- Keyboard handlers: `packages/client/src/flow/copy-paste.ts:340`

## Removing Copy-Paste Functionality

If you need to remove the copy-paste feature in the future, follow these steps:

### 1. Delete the copy-paste module
```bash
rm packages/client/src/flow/copy-paste.ts
```

### 2. Revert changes in flow.tsx

Remove the import:
```diff
- import { setupCopyPasteHandlers } from './copy-paste';
```

Remove from the destructuring:
```diff
- const { addEdges, addNodes, getEdges, getNode, getNodes, screenToFlowPosition, setEdges, setNodes } = useReactFlow<Node, Edge>();
+ const { addEdges, addNodes, getEdges, getNode, screenToFlowPosition, setNodes } = useReactFlow<Node, Edge>();
```

Remove the dataClient:
```diff
- const { dataClient } = useRouteContext({ from: '__root__' });
```

Remove the useEffect import:
```diff
- import { PropsWithChildren, Suspense, use, useCallback, useEffect, useMemo } from 'react';
+ import { PropsWithChildren, Suspense, use, useCallback, useMemo } from 'react';
```

Remove the entire useEffect block (lines ~231-249):
```diff
- // Setup copy/paste
- useEffect(() => {
-   const handleKeyDown = setupCopyPasteHandlers(
-     getNodes,
-     getEdges,
-     dataClient,
-     makeNode,
-     makeEdge,
-     addNodes,
-     addEdges,
-     setNodes,
-     setEdges,
-     isReadOnly
-   );
-
-   document.addEventListener('keydown', handleKeyDown);
-   return () => {
-     document.removeEventListener('keydown', handleKeyDown);
-   };
- }, [addEdges, addNodes, dataClient, getEdges, getNodes, isReadOnly, makeEdge, makeNode, setNodes, setEdges]);
```

### 3. Revert changes in node.tsx

Remove the imports:
```diff
- import { duplicateNodeFromMenu } from './copy-paste';
- import { Edge, useMakeEdge } from './edge';
```

If Edge is not used elsewhere, also remove it from the edge import:
```diff
- import { Edge, useMakeEdge } from './edge';
+ import { useMakeEdge } from './edge';
```

Remove from the destructuring:
```diff
- const { addEdges, addNodes, deleteElements, getEdges, getNode, getZoom, setNodes } = useReactFlow();
+ const { deleteElements, getEdges, getNode, getZoom } = useReactFlow();
```

Remove the hooks:
```diff
- const { dataClient } = useRouteContext({ from: '__root__' });
- const makeNode = useMakeNode();
- const makeEdge = useMakeEdge();
```

Remove the entire Duplicate MenuItem (lines ~195-217):
```diff
- <MenuItem
-   onAction={async () => {
-     const node = getNode(id);
-     if (!node) return;
-     
-     await duplicateNodeFromMenu(
-       id,
-       node.position,
-       dataClient,
-       makeNode,
-       makeEdge,
-       (newNodes, newEdges) => {
-         // Deselect all nodes first
-         setNodes((nodes) => nodes.map((n) => ({ ...n, selected: false })));
-         // Add new nodes as selected
-         addNodes(newNodes.map((n) => Node.fromDTO(n, { selected: true })));
-         addEdges(newEdges.map((e) => Edge.fromDTO(e)));
-       }
-     );
-   }}
- >
-   Duplicate
- </MenuItem>
```

### Summary

The copy-paste feature was designed to be easily removable:
- All core logic is isolated in `copy-paste.ts`
- Changes to existing files are minimal and clearly marked
- No modifications to data structures or other components
- Simply delete the module and revert the integration points

Total changes to revert:
- `flow.tsx`: ~23 lines
- `node.tsx`: ~30 lines
- Delete `copy-paste.ts`: ~500 lines