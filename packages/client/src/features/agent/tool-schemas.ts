/**
 * Runtime tool schema utilities - converts Effect Schemas to JSON Schema tool definitions.
 * These utilities are used by the agent to handle AI tool calling.
 */

import { Either, JSONSchema, ParseResult, Schema } from 'effect';

import { ExecutionSchemas } from '@the-dev-tools/spec/tools/execution';
import { ExplorationSchemas } from '@the-dev-tools/spec/tools/exploration';
import { MutationSchemas } from '@the-dev-tools/spec/tools/mutation';

// Re-export schemas for convenience
export { ExecutionSchemas, ExplorationSchemas, MutationSchemas };
export * from '@the-dev-tools/spec-lib/common';

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
 * Validate tool input against the Effect Schema.
 * Returns Either<data, errors> - Right on success, Left on failure.
 */
export function validateToolInput(
  toolName: string,
  input: unknown,
): Either.Either<unknown, string[]> {
  const schema = schemaMap[toolName];
  if (!schema) {
    return Either.left([`Unknown tool: ${toolName}`]);
  }

  return Schema.decodeUnknownEither(schema)(input).pipe(
    Either.mapLeft((error) => [ParseResult.TreeFormatter.formatErrorSync(error)]),
  );
}
