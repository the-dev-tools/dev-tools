/**
 * Common schemas and utilities for tool definitions.
 */

import { Schema } from 'effect';

// =============================================================================
// Common Field Schemas
// =============================================================================

/**
 * ULID identifier schema - used for all entity IDs
 */
export const UlidId = Schema.String.pipe(
  Schema.pattern(/^[0-9A-HJKMNP-TV-Z]{26}$/),
  Schema.annotations({
    title: 'ULID',
    description: 'A ULID (Universally Unique Lexicographically Sortable Identifier)',
    examples: ['01ARZ3NDEKTSV4RRFFQ69G5FAV'],
  }),
);

/**
 * Flow ID - references a workflow
 */
export const FlowId = UlidId.pipe(
  Schema.annotations({
    identifier: 'flowId',
    description: 'The ULID of the workflow',
  }),
);

/**
 * Node ID - references a node within a workflow
 */
export const NodeId = UlidId.pipe(
  Schema.annotations({
    identifier: 'nodeId',
    description: 'The ULID of the node',
  }),
);

/**
 * Edge ID - references an edge connection
 */
export const EdgeId = UlidId.pipe(
  Schema.annotations({
    identifier: 'edgeId',
    description: 'The ULID of the edge',
  }),
);

// =============================================================================
// Position Schema
// =============================================================================

export const Position = Schema.Struct({
  x: Schema.Number.pipe(
    Schema.annotations({
      description: 'X coordinate on the canvas',
    }),
  ),
  y: Schema.Number.pipe(
    Schema.annotations({
      description: 'Y coordinate on the canvas',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'Position',
    description: 'Position on the canvas',
  }),
);

export const OptionalPosition = Schema.optional(
  Position.pipe(
    Schema.annotations({
      description: 'Position on the canvas (optional)',
    }),
  ),
);

// =============================================================================
// Enums - hardcoded values (matching protobuf definitions)
// =============================================================================

export const ErrorHandling = Schema.Literal('ignore', 'break').pipe(
  Schema.annotations({
    identifier: 'ErrorHandling',
    description: 'How to handle errors: "ignore" continues, "break" stops the loop',
  }),
);

export const SourceHandle = Schema.Literal('then', 'else', 'loop').pipe(
  Schema.annotations({
    identifier: 'SourceHandle',
    description:
      'Output handle for branching nodes. Use "then"/"else" for Condition nodes, "loop"/"then" for For/ForEach nodes.',
  }),
);

export const ApiCategory = Schema.Literal(
  'messaging',
  'payments',
  'project-management',
  'storage',
  'database',
  'email',
  'calendar',
  'crm',
  'social',
  'analytics',
  'developer',
).pipe(
  Schema.annotations({
    identifier: 'ApiCategory',
    description: 'Category of the API',
  }),
);

// =============================================================================
// Display Name & Code Schemas
// =============================================================================

export const NodeName = Schema.String.pipe(
  Schema.minLength(1),
  Schema.maxLength(100),
  Schema.annotations({
    description: 'Display name for the node',
    examples: ['Transform Data', 'Fetch User', 'Check Status'],
  }),
);

export const JsCode = Schema.String.pipe(
  Schema.annotations({
    description:
      'The function body only. Write code directly - do NOT define inner functions. Use ctx for input. MUST have a return statement. The tool auto-wraps with "export default function(ctx) { ... }". Example: "const result = ctx.value * 2; return { result };"',
    examples: [
      'const result = ctx.value * 2; return { result };',
      'const items = ctx.data.filter(x => x.active); return { items, count: items.length };',
    ],
  }),
);

export const ConditionExpression = Schema.String.pipe(
  Schema.annotations({
    description:
      'Boolean expression using expr-lang syntax. Use == for equality (NOT ===). Use Input to reference previous node output (e.g., "Input.status == 200", "Input.success == true")',
    examples: ['Input.status == 200', 'Input.success == true', 'Input.count > 0'],
  }),
);

// =============================================================================
// Type Exports
// =============================================================================

export type Position = typeof Position.Type;
export type ErrorHandling = typeof ErrorHandling.Type;
export type SourceHandle = typeof SourceHandle.Type;
export type ApiCategory = typeof ApiCategory.Type;
