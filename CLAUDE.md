# Claude Development Notes

This file contains important information for Claude to understand the codebase and development patterns.

## Project Overview

DevTools is a free, open-source Postman-style API tester that runs locally. It features:

- Browser request recording via Chrome extension
- Visual flow builder for API test chaining
- 100% local execution with no cloud dependencies
- CI/CD integration capabilities
- React-based frontend with TypeScript

## Key Commands

### Development

- `nix develop` - Enter the development shell
- `nix develop -c pnpm nx run client:dev` - Run the client development server
- `nix develop -c pnpm nx run client:lint` - Run ESLint
- `nix develop -c pnpm nx run client:lint --fix` - Run ESLint with auto-fix
- `nix develop -c pnpm nx run client:typecheck` - Run TypeScript type checking

### Testing

- Always run both lint and typecheck before committing changes
- Use `--fix` flag with lint to auto-fix formatting issues

## Recent Features

### Node Duplication and Copy/Paste (2024-12-28)

Implemented a comprehensive node duplication system for the flow editor:

1. **Single Node Duplication** - Right-click context menu "Duplicate" option

   - Duplicates the selected node without edges
   - Creates independent deltas for REQUEST nodes
   - Positions new node 50px offset from original

2. **Copy/Paste with Ctrl+C/V** - Multi-node selection support

   - Preserves edges between selected nodes
   - Maintains relative positioning
   - Creates new deltas for all REQUEST nodes

3. **Data Preservation**

   - Copies all delta data (headers, query parameters, body)
   - Preserves HTTP method overrides from delta endpoints
   - Maintains variable references in headers/queries (not resolved to constants)

4. **Architecture**
   - All copy/paste logic extracted to `packages/client/src/flow/copy-paste.ts`
   - Minimal changes to `flow.tsx` and `node.tsx` (only integration points)
   - Easy to remove later if needed (temporary feature)

#### Technical Details

- REQUEST nodes get new delta endpoints and examples (hidden entities)
- Control flow nodes (CONDITION, FOR, FOR_EACH) create their required child nodes
- NO_OP nodes are not duplicated individually (they're part of control structures)
- HTTP method is copied from the original delta endpoint to preserve overrides

## Code Style Guidelines

1. **Imports** - Let the linter handle import ordering
2. **Types** - Avoid using `any` types; use proper type annotations
3. **Comments** - Do not add comments unless specifically requested
4. **Error Handling** - Wrap API calls in try-catch blocks for delta operations
5. **Git Commits** - Never commit unless explicitly asked by the user

## Architecture Notes

### Flow System

- Uses React Flow (@xyflow/react) for node-based UI
- Connect RPC for backend communication
- Effect-TS for functional programming patterns
- Delta system for storing modifications without changing originals
- See [Copy-Paste Implementation](./docs/COPY-PASTE-IMPLEMENTATION.md) for node duplication details

### Delta System

- Base examples are read-only templates
- Delta endpoints/examples store overrides
- `SourceKind.ORIGIN` indicates inherited (non-overridden) values
- Delta entities are created as "hidden" to not appear in regular lists

### Request Nodes

- Each REQUEST node has:
  - `endpointId` - Original endpoint reference
  - `exampleId` - Original example reference
  - `deltaEndpointId` - Node-specific endpoint overrides
  - `deltaExampleId` - Node-specific example overrides
- HTTP method can be overridden at the delta endpoint level
