/**
 * Tool Schema Definitions using Effect Schema
 *
 * Single source of truth for AI tool definitions.
 * Adding a new tool: just add to MutationSchemas/ExplorationSchemas/ExecutionSchemas.
 * Everything else is automatic.
 */

import { JSONSchema, Schema } from 'effect';

// Re-export all schemas for direct use
export * from './common.ts';
export * from './execution.ts';
export * from './exploration.ts';
export * from './mutation.ts';

// Import schema groups
import { ExecutionSchemas } from './execution.ts';
import { ExplorationSchemas } from './exploration.ts';
import { MutationSchemas } from './mutation.ts';

// =============================================================================
// Tool Definition Type
// =============================================================================

export interface ToolDefinition {
  name: string;
  description: string;
  parameters: object;
}

// =============================================================================
// JSON Schema Generation
// =============================================================================

/** Recursively resolve $ref references in a JSON Schema */
function resolveRefs(obj: unknown, defs: Record<string, unknown>): unknown {
  if (obj === null || typeof obj !== 'object') return obj;
  if (Array.isArray(obj)) return obj.map((item) => resolveRefs(item, defs));

  const record = obj as Record<string, unknown>;

  // Handle $ref
  if ('$ref' in record && typeof record.$ref === 'string') {
    const defName = record.$ref.replace('#/$defs/', '');
    const resolved = defs[defName];
    if (resolved) {
      const { $ref: _, ...rest } = record;
      return { ...(resolveRefs(resolved, defs) as Record<string, unknown>), ...rest };
    }
  }

  // Handle allOf with single $ref (common Effect pattern)
  if ('allOf' in record && Array.isArray(record.allOf) && record.allOf.length === 1) {
    const first = record.allOf[0] as Record<string, unknown>;
    if ('$ref' in first) {
      const { allOf: _, ...rest } = record;
      return { ...(resolveRefs(first, defs) as Record<string, unknown>), ...rest };
    }
  }

  // Recursively process all properties
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(record)) {
    if (key === '$defs' || key === '$schema') continue;
    result[key] = resolveRefs(value, defs);
  }
  return result;
}

/** Convert an Effect Schema to a tool definition with JSON Schema parameters */
function schemaToToolDefinition<A, I, R>(schema: Schema.Schema<A, I, R>): ToolDefinition {
  const jsonSchema = JSONSchema.make(schema) as {
    $schema: string;
    $defs: Record<string, unknown>;
    $ref: string;
  };

  const defs = jsonSchema.$defs ?? {};
  const defName = (jsonSchema.$ref ?? '').replace('#/$defs/', '');
  const def = defs[defName] as {
    description?: string;
    type: string;
    properties: Record<string, unknown>;
    required?: string[];
  } | undefined;

  return {
    name: defName || 'unknown',
    description: def?.description ?? '',
    parameters: def
      ? {
          type: def.type,
          properties: resolveRefs(def.properties, defs),
          required: def.required,
          additionalProperties: false,
        }
      : jsonSchema,
  };
}

// =============================================================================
// Auto-generated Tool Definitions (no manual listing needed)
// =============================================================================

export const mutationSchemas = Object.values(MutationSchemas).map(schemaToToolDefinition);
export const explorationSchemas = Object.values(ExplorationSchemas).map(schemaToToolDefinition);
export const executionSchemas = Object.values(ExecutionSchemas).map(schemaToToolDefinition);

/** All tool schemas combined - ready for AI tool calling */
export const allToolSchemas = [...explorationSchemas, ...mutationSchemas, ...executionSchemas];

// =============================================================================
// Effect Schemas (for runtime validation)
// =============================================================================

export const EffectSchemas = {
  Mutation: MutationSchemas,
  Exploration: ExplorationSchemas,
  Execution: ExecutionSchemas,
} as const;

// =============================================================================
// Validation Helper
// =============================================================================

// Build schema map dynamically - no manual maintenance needed
const schemaMap: Record<string, Schema.Schema<unknown, unknown>> = Object.fromEntries(
  Object.entries(EffectSchemas).flatMap(([, group]) =>
    Object.entries(group).map(([name, schema]) => [
      name.charAt(0).toLowerCase() + name.slice(1),
      schema as Schema.Schema<unknown, unknown>,
    ]),
  ),
);

/**
 * Validate tool input against the Effect Schema
 *
 * @example
 * const result = validateToolInput('createJsNode', {
 *   flowId: '01ARZ3NDEKTSV4RRFFQ69G5FAV',
 *   name: 'Transform Data',
 *   code: 'return { result: ctx.value * 2 };'
 * });
 */
export function validateToolInput(
  toolName: string,
  input: unknown,
): { success: true; data: unknown } | { success: false; errors: string[] } {
  const schema = schemaMap[toolName];
  if (!schema) {
    return { success: false, errors: [`Unknown tool: ${toolName}`] };
  }

  try {
    const decoded = Schema.decodeUnknownSync(schema)(input);
    return { success: true, data: decoded };
  } catch (error) {
    if (error instanceof Error) {
      return { success: false, errors: [error.message] };
    }
    return { success: false, errors: ['Unknown validation error'] };
  }
}
