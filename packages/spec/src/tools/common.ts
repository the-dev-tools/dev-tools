/**
 * Common schemas and utilities for tool definitions
 * These are shared building blocks used across multiple tool schemas
 *
 * IMPORTANT: Enums and types are DERIVED from the generated Protobuf types
 * in @the-dev-tools/spec/buf/api/flow/v1/flow_pb to ensure consistency
 * with the TypeSpec definitions.
 */

import { Schema } from 'effect';

// =============================================================================
// Import enums from generated Protobuf (derived from TypeSpec)
// =============================================================================
import {
  ErrorHandling as PbErrorHandling,
  HandleKind as PbHandleKind,
} from '../../dist/buf/typescript/api/flow/v1/flow_pb.ts';

// =============================================================================
// Common Field Schemas
// =============================================================================

/**
 * ULID identifier schema - used for all entity IDs
 * Matches the `Id` type in TypeSpec (main.tsp)
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
 * Corresponds to Flow.flowId in flow.tsp
 */
export const FlowId = UlidId.pipe(
  Schema.annotations({
    identifier: 'flowId',
    description: 'The ULID of the workflow',
  }),
);

/**
 * Node ID - references a node within a workflow
 * Corresponds to Node.nodeId in flow.tsp
 */
export const NodeId = UlidId.pipe(
  Schema.annotations({
    identifier: 'nodeId',
    description: 'The ULID of the node',
  }),
);

/**
 * Edge ID - references an edge connection
 * Corresponds to Edge.edgeId in flow.tsp
 */
export const EdgeId = UlidId.pipe(
  Schema.annotations({
    identifier: 'edgeId',
    description: 'The ULID of the edge',
  }),
);

// =============================================================================
// Position Schema (matches Position model in flow.tsp)
// =============================================================================

/**
 * Canvas position for nodes
 * Derived from: model Position { x: float32; y: float32; } in flow.tsp
 */
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

export const OptionalPosition = Schema.optional(Position).pipe(
  Schema.annotations({
    description: 'Position on the canvas (optional)',
  }),
);

// =============================================================================
// Enums DERIVED from TypeSpec/Protobuf definitions
// =============================================================================

/**
 * SAFETY NET: These types and Record<> patterns ensure TypeScript will ERROR
 * if the backend adds new enum values to TypeSpec that we haven't handled.
 *
 * How it works:
 * 1. We exclude UNSPECIFIED (protobuf default) from each enum type
 * 2. We use Record<ExcludedEnum, string> which REQUIRES all enum values as keys
 * 3. If backend adds e.g. PARALLEL to HandleKind, TypeScript errors:
 *    "Property 'PARALLEL' is missing in type..."
 *
 * This turns silent drift into compile-time errors!
 */

// Types that exclude the UNSPECIFIED protobuf default value
type ValidHandleKind = Exclude<PbHandleKind, PbHandleKind.UNSPECIFIED>;
type ValidErrorHandling = Exclude<PbErrorHandling, PbErrorHandling.UNSPECIFIED>;

/**
 * Helper: Creates a Schema.Literal from all values in an enum mapping.
 * This ensures the schema automatically includes all mapped values.
 */
function literalFromValues<T extends Record<number, string>>(mapping: T) {
  const values = Object.values(mapping) as [string, ...string[]];
  return Schema.Literal(...values);
}

/**
 * Error handling strategies for loop nodes
 * Derived from: enum ErrorHandling { Ignore, Break } in flow.tsp
 *
 * EXHAUSTIVE: Record<ValidErrorHandling, string> forces all enum values to be present.
 * If backend adds a new value, TypeScript will error until it's added here.
 */
const errorHandlingValues: Record<ValidErrorHandling, string> = {
  [PbErrorHandling.IGNORE]: 'ignore',
  [PbErrorHandling.BREAK]: 'break',
};

export const ErrorHandling = literalFromValues(errorHandlingValues).pipe(
  Schema.annotations({
    identifier: 'ErrorHandling',
    description: 'How to handle errors: "ignore" continues, "break" stops the loop',
  }),
);

/**
 * Source handle types for connecting nodes
 * Derived from: enum HandleKind { Then, Else, Loop } in flow.tsp
 *
 * EXHAUSTIVE: Record<ValidHandleKind, string> forces all enum values to be present.
 * If backend adds a new value (e.g., PARALLEL), TypeScript will error until it's added here.
 */
const handleKindValues: Record<ValidHandleKind, string> = {
  [PbHandleKind.THEN]: 'then',
  [PbHandleKind.ELSE]: 'else',
  [PbHandleKind.LOOP]: 'loop',
};

export const SourceHandle = literalFromValues(handleKindValues).pipe(
  Schema.annotations({
    identifier: 'SourceHandle',
    description:
      'Output handle for branching nodes. Use "then"/"else" for Condition nodes, "loop"/"then" for For/ForEach nodes.',
  }),
);

/**
 * API documentation categories
 */
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
// Display Name Schema
// =============================================================================

/**
 * Display name for nodes
 */
export const NodeName = Schema.String.pipe(
  Schema.minLength(1),
  Schema.maxLength(100),
  Schema.annotations({
    description: 'Display name for the node',
    examples: ['Transform Data', 'Fetch User', 'Check Status'],
  }),
);

// =============================================================================
// Code Schema (for JS nodes)
// =============================================================================

/**
 * JavaScript code for JS nodes
 */
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

// =============================================================================
// Condition Expression Schema
// =============================================================================

/**
 * Boolean condition expression using expr-lang syntax
 */
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
