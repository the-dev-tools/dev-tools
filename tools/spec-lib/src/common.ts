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
    description: 'A ULID (Universally Unique Lexicographically Sortable Identifier)',
    examples: ['01ARZ3NDEKTSV4RRFFQ69G5FAV'],
    title: 'ULID',
  }),
);

/**
 * Flow ID - references a workflow
 */
export const FlowId = UlidId.pipe(
  Schema.annotations({
    description: 'The ULID of the workflow',
    identifier: 'flowId',
  }),
);

/**
 * Node ID - references a node within a workflow
 */
export const NodeId = UlidId.pipe(
  Schema.annotations({
    description: 'The ULID of the node',
    identifier: 'nodeId',
  }),
);

/**
 * Edge ID - references an edge connection
 */
export const EdgeId = UlidId.pipe(
  Schema.annotations({
    description: 'The ULID of the edge',
    identifier: 'edgeId',
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
    description: 'Position on the canvas',
    identifier: 'Position',
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
//
// SYNC WARNING: These values are hardcoded to avoid circular dependencies with
// packages/spec. They MUST match the protobuf definitions in:
//   api/flow/v1/flow.proto -> ErrorHandling, HandleKind enums
//
// If the protobuf enums change, update these literals accordingly.
// =============================================================================

export const ErrorHandling = Schema.Literal('ignore', 'break').pipe(
  Schema.annotations({
    description: 'How to handle errors: "ignore" continues, "break" stops the loop',
    identifier: 'ErrorHandling',
  }),
);

export const SourceHandle = Schema.Literal('then', 'else', 'loop').pipe(
  Schema.annotations({
    description:
      'Output handle for branching nodes. Use "then"/"else" for Condition nodes, "loop"/"then" for For/ForEach nodes.',
    identifier: 'SourceHandle',
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
    description: 'Category of the API',
    identifier: 'ApiCategory',
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
    examples: ['Transform_Data', 'Fetch_User', 'Check_Status'],
  }),
);

export const JsCode = Schema.String.pipe(
  Schema.annotations({
    description:
      'Function body only. Access node outputs via ctx["NodeName"]. MUST have a return statement. Auto-wrapped with "export default function(ctx) { ... }". IMPORTANT: Always return an object (not array/primitive) so properties are directly accessible in conditions.',
    examples: [
      'const data = ctx["Fetch User"].response.body; return { userId: data.id };',
      'const items = ctx["HTTP"].response.body; return { items, count: items.length };',
    ],
  }),
);

export const ConditionExpression = Schema.String.pipe(
  Schema.annotations({
    description:
      'Boolean expression using expr-lang syntax. NEVER use {{}} template syntax. Access node outputs via NodeName.field (underscores replace spaces). When a JS node returns an array/primitive directly, it is wrapped as .result. Use len() for array length. ForEach nodes expose .item (current value) and .key (index). For For/ForEach nodes, this is the REQUIRED break condition - the loop exits early when this evaluates to true.',
    examples: ['Get_User.response.status == 200', 'ForEach_Loop.key >= 3', 'Counter.count >= 10'],
  }),
);

// =============================================================================
// Type Exports
// =============================================================================

export type Position = typeof Position.Type;
export type ErrorHandling = typeof ErrorHandling.Type;
export type SourceHandle = typeof SourceHandle.Type;
export type ApiCategory = typeof ApiCategory.Type;
