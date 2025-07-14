# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DevTools is a free, open-source Postman-style API tester that runs locally. It features:

- Browser request recording via Chrome extension
- Visual flow builder for API test chaining
- 100% local execution with no cloud dependencies
- CI/CD integration capabilities
- React-based frontend with TypeScript and Go-based backend

## Key Commands

### Development Environment

The project uses Nix Flakes for reproducible development environments. All commands should be run with `nix develop -c` prefix.

### Core Development Commands

```bash
# Enter development shell
nix develop

# Run client development server
nix develop -c pnpm nx run client:dev

# Run linting
nix develop -c pnpm nx run client:lint
nix develop -c pnpm nx run client:lint --fix

# Run TypeScript type checking
nix develop -c pnpm nx run client:typecheck

# Run tests across all projects
nix develop -c pnpm nx run-many --targets=test

# Run specific package tests
nix develop -c pnpm nx run server:test
nix develop -c pnpm nx run db:test
```

### Multi-Project Commands

```bash
# Run linting and format checking across all projects
task lint

# Run all tests
task test

# Auto-fix common issues (prettier, syncpack)
task fix

# Run multiple targets
nix develop -c pnpm nx run-many --targets=lint,typecheck
```

### Package-Specific Commands

**Server (Go backend):**
- `pnpm nx run server:dev` - Development server
- `pnpm nx run server:build` - Build binary
- `pnpm nx run server:test` - Run tests (outputs to dist/go-test.json)
- `pnpm nx run server:lint` - Run golangci-lint

**Client (React frontend):**
- `pnpm nx run client:dev` - Development server (Vite)
- `pnpm nx run client:build` - Production build
- `pnpm nx run client:preview` - Preview production build

**UI (Component library):**
- `pnpm nx run ui:dev` - React Cosmos playground
- `pnpm nx run ui:lint` - ESLint checking
- `pnpm nx run ui:typecheck` - TypeScript checking

**Spec (API definitions):**
- `pnpm nx run spec:build` - Build TypeSpec and generate protobuf
- `pnpm nx run spec:dev` - Watch mode for spec building

### Pre-commit Checklist

Always run both lint and typecheck before committing changes:
```bash
nix develop -c pnpm nx run client:lint --fix
nix develop -c pnpm nx run client:typecheck
```

## Architecture Overview

### Technology Stack

**Frontend:**
- React 19 with TypeScript
- @xyflow/react for visual flow builder
- TanStack Router/Query for routing and data fetching
- React Aria Components for accessibility
- Tailwind CSS 4 for styling
- Effect-TS for functional programming
- Connect RPC for API communication

**Backend:**
- Go server with Connect RPC
- SQLite/LibSQL for database
- Protocol Buffers for API definitions

### Key Architectural Patterns

#### 1. RPC Communication (Connect RPC)

- Protocol Buffer definitions in `/packages/spec/dist/buf/typescript/`
- `ApiTransport` wraps Connect with Effect-TS interceptors
- Authentication via Magic SDK
- Data Client pattern with automatic caching via `@data-client/react`

#### 2. Delta System

Stores modifications without changing originals:
- Base entities (endpoints, examples) are read-only templates
- Delta entities store overrides as "hidden" entries
- `SourceKind.ORIGIN` = inherited, `SourceKind.DELTA` = override
- REQUEST nodes have both original IDs and delta IDs

#### 3. Flow System

Built on React Flow:
- Node types: REQUEST, CONDITION, FOR, FOR_EACH, JAVASCRIPT, NO_OP
- Debounced client-server synchronization
- Atomic operations for create/update/delete
- Each node has unique ULID, type-specific data, and connection handles

#### 4. Effect-TS Patterns

Used for functional programming:
```typescript
// Common pattern
Effect.gen(function* () {
  const value = yield* someEffect;
  // Sequential operations with generator syntax
})
```
- Dependency injection via Context.Tag
- Error handling with Effect.tryPromise and Effect.catchIf
- Extensive use of pipe, Option, Either, and collection utilities

### Project Structure

```
dev-tools/
├── apps/                    # Applications
│   ├── api-recorder-extension/  # Chrome extension
│   ├── cli/                     # Go CLI tool
│   ├── desktop/                 # Electron app
│   └── web/                     # Web app
├── packages/                # Core packages
│   ├── client/             # React frontend
│   ├── db/                 # Database layer
│   ├── server/             # Go backend
│   ├── spec/               # TypeSpec API definitions
│   ├── ui/                 # Shared UI components
│   └── worker-js/          # JS worker for flow execution
└── configs/               # Configuration packages
```

## Recent Features

### Node Duplication and Copy/Paste (2024-12-28)

Implemented comprehensive node duplication for the flow editor:

1. **Single Node Duplication** - Right-click "Duplicate" option
   - Creates independent deltas for REQUEST nodes
   - Positions new node 50px offset from original

2. **Copy/Paste with Ctrl+C/V** - Multi-node selection support
   - Preserves edges between selected nodes
   - Maintains relative positioning
   - Creates new deltas for all REQUEST nodes

3. **Implementation Details**
   - Logic in `packages/client/src/flow/copy-paste.ts`
   - REQUEST nodes get new delta endpoints and examples
   - HTTP method copied from original delta endpoint
   - Control flow nodes create required child nodes

## Code Style Guidelines

1. **Imports** - Let the linter handle import ordering
2. **Types** - Avoid using `any` types; use proper type annotations
3. **Comments** - Do not add comments unless specifically requested
4. **Error Handling** - Wrap API calls in try-catch blocks for delta operations
5. **Git Commits** - Never commit unless explicitly asked by the user

## Development Notes

### Testing Go Code

```bash
# Run all tests in a package
cd packages/server
go test ./...

# Run specific test file
go test ./internal/api/middleware/mwauth/mwauth_test.go

# Run with verbose output
go test -v ./...
```

### Request Nodes

Each REQUEST node contains:
- `endpointId` - Original endpoint reference
- `exampleId` - Original example reference
- `deltaEndpointId` - Node-specific endpoint overrides
- `deltaExampleId` - Node-specific example overrides

HTTP method can be overridden at the delta endpoint level.

### Important Files

- `flake.nix` - Nix development environment
- `nx.json` - Nx workspace configuration
- `taskfile.yaml` - Task runner for common commands
- `pnpm-workspace.yaml` - pnpm workspace configuration

## License

MIT License + Commons Clause - free for local development and internal business use, but requires a commercial license for hosted/SaaS offerings.

## Important Reminders

- Do what has been asked; nothing more, nothing less
- NEVER create files unless they're absolutely necessary
- ALWAYS prefer editing existing files to creating new ones
- NEVER proactively create documentation files (*.md) unless explicitly requested