/**
 * Runtime tool schema index - converts Effect Schemas to JSON Schema tool definitions.
 */

import { JSONSchema, Schema } from 'effect';

export * from '@the-dev-tools/spec-lib/common';
export * from '../../dist/ai-tools/v1/execution.ts';
export * from '../../dist/ai-tools/v1/exploration.ts';
export * from '../../dist/ai-tools/v1/mutation.ts';

import { ExecutionSchemas } from '../../dist/ai-tools/v1/execution.ts';
import { ExplorationSchemas } from '../../dist/ai-tools/v1/exploration.ts';
import { MutationSchemas } from '../../dist/ai-tools/v1/mutation.ts';

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

  if ('$ref' in record && typeof record['$ref'] === 'string') {
    const defName = record['$ref'].replace('#/$defs/', '');
    const resolved = defs[defName];
    if (resolved) {
      const { $ref: _, ...rest } = record;
      return { ...(resolveRefs(resolved, defs) as Record<string, unknown>), ...rest };
    }
  }

  if ('allOf' in record && Array.isArray(record['allOf']) && record['allOf'].length === 1) {
    const first = record['allOf'][0] as Record<string, unknown>;
    if ('$ref' in first) {
      const { allOf: _, ...rest } = record;
      return { ...(resolveRefs(first, defs) as Record<string, unknown>), ...rest };
    }
  }

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
// Auto-generated Tool Definitions
// =============================================================================

export const executionSchemas = Object.values(ExecutionSchemas).map((s) =>
  schemaToToolDefinition(s as Schema.Schema<unknown, unknown>),
);

export const explorationSchemas = Object.values(ExplorationSchemas).map((s) =>
  schemaToToolDefinition(s as Schema.Schema<unknown, unknown>),
);

export const mutationSchemas = Object.values(MutationSchemas).map((s) =>
  schemaToToolDefinition(s as Schema.Schema<unknown, unknown>),
);

/** All tool schemas combined - ready for AI tool calling */
export const allToolSchemas = [...executionSchemas, ...explorationSchemas, ...mutationSchemas];

// =============================================================================
// Effect Schemas (for runtime validation)
// =============================================================================

export const EffectSchemas = {
  Execution: ExecutionSchemas,
  Exploration: ExplorationSchemas,
  Mutation: MutationSchemas,
} as const;

// =============================================================================
// Validation Helper
// =============================================================================

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
