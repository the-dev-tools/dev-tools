/**
 * Tool Schema Definitions using Effect Schema
 *
 * This module provides type-safe tool definitions that can be:
 * 1. Used directly as TypeScript types for validation
 * 2. Converted to JSON Schema for AI tool calling (Claude, MCP, etc.)
 *
 * The schemas are the single source of truth for tool definitions.
 * When spec changes, these schemas should be updated accordingly.
 */

import { JSONSchema, Schema } from 'effect';

// Re-export all schemas
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

/**
 * Standard tool definition format for AI tool calling
 */
export interface ToolDefinition {
  name: string;
  description: string;
  parameters: object;
}

// =============================================================================
// JSON Schema Generation
// =============================================================================

/**
 * Recursively resolve $ref references in a JSON Schema
 */
function resolveRefs(
  obj: unknown,
  defs: Record<string, unknown>,
): unknown {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map((item) => resolveRefs(item, defs));
  }

  const record = obj as Record<string, unknown>;

  // Handle $ref
  if ('$ref' in record && typeof record.$ref === 'string') {
    const refPath = record.$ref;
    const defName = refPath.replace('#/$defs/', '');
    const resolved = defs[defName];
    if (resolved) {
      // Merge any sibling properties (like description) with resolved ref
      const { $ref: _, ...rest } = record;
      const resolvedObj = resolveRefs(resolved, defs) as Record<string, unknown>;
      return { ...resolvedObj, ...rest };
    }
  }

  // Handle allOf with single $ref (common pattern from Effect)
  if ('allOf' in record && Array.isArray(record.allOf) && record.allOf.length === 1) {
    const first = record.allOf[0] as Record<string, unknown>;
    if ('$ref' in first) {
      const { allOf: _, ...rest } = record;
      const resolved = resolveRefs(first, defs) as Record<string, unknown>;
      return { ...resolved, ...rest };
    }
  }

  // Recursively process all properties
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(record)) {
    if (key === '$defs' || key === '$schema') {
      continue; // Skip $defs and $schema in output
    }
    result[key] = resolveRefs(value, defs);
  }
  return result;
}

/**
 * Convert an Effect Schema to a tool definition with JSON Schema parameters
 */
function schemaToToolDefinition<A, I, R>(
  schema: Schema.Schema<A, I, R>,
): ToolDefinition {
  // Generate JSON Schema - Effect puts annotations in the $defs
  const jsonSchema = JSONSchema.make(schema) as {
    $schema: string;
    $defs: Record<string, unknown>;
    $ref: string;
  };

  const defs = jsonSchema.$defs ?? {};

  // Extract the definition name from $ref (e.g., "#/$defs/createJsNode" -> "createJsNode")
  const refPath = jsonSchema.$ref ?? '';
  const defName = refPath.replace('#/$defs/', '');
  const def = defs[defName] as {
    title?: string;
    description?: string;
    type: string;
    properties: Record<string, unknown>;
    required?: string[];
  } | undefined;

  // Get name from identifier (the key in $defs) and description from the definition
  const name = defName || 'unknown';
  const description = def?.description ?? '';

  // Resolve all $refs in properties to inline nested schemas
  const resolvedProperties = def?.properties
    ? resolveRefs(def.properties, defs)
    : {};

  // Build flattened parameters schema
  const parameters = def
    ? {
        type: def.type,
        properties: resolvedProperties,
        required: def.required,
        additionalProperties: false,
      }
    : jsonSchema;

  return {
    name,
    description,
    parameters,
  };
}

// =============================================================================
// Generated Tool Definitions
// =============================================================================

// Mutation tools
export const createJsNodeSchema = schemaToToolDefinition(MutationSchemas.CreateJsNode);
export const createHttpNodeSchema = schemaToToolDefinition(MutationSchemas.CreateHttpNode);
export const createConditionNodeSchema = schemaToToolDefinition(MutationSchemas.CreateConditionNode);
export const createForNodeSchema = schemaToToolDefinition(MutationSchemas.CreateForNode);
export const createForEachNodeSchema = schemaToToolDefinition(MutationSchemas.CreateForEachNode);
export const updateNodeCodeSchema = schemaToToolDefinition(MutationSchemas.UpdateNodeCode);
export const updateNodeConfigSchema = schemaToToolDefinition(MutationSchemas.UpdateNodeConfig);
export const connectNodesSchema = schemaToToolDefinition(MutationSchemas.ConnectNodes);
export const disconnectNodesSchema = schemaToToolDefinition(MutationSchemas.DisconnectNodes);
export const deleteNodeSchema = schemaToToolDefinition(MutationSchemas.DeleteNode);
export const createVariableSchema = schemaToToolDefinition(MutationSchemas.CreateVariable);
export const updateVariableSchema = schemaToToolDefinition(MutationSchemas.UpdateVariable);

// Exploration tools
export const getWorkflowGraphSchema = schemaToToolDefinition(ExplorationSchemas.GetWorkflowGraph);
export const getNodeDetailsSchema = schemaToToolDefinition(ExplorationSchemas.GetNodeDetails);
export const getNodeTemplateSchema = schemaToToolDefinition(ExplorationSchemas.GetNodeTemplate);
export const searchTemplatesSchema = schemaToToolDefinition(ExplorationSchemas.SearchTemplates);
export const getExecutionHistorySchema = schemaToToolDefinition(ExplorationSchemas.GetExecutionHistory);
export const getExecutionLogsSchema = schemaToToolDefinition(ExplorationSchemas.GetExecutionLogs);
export const searchApiDocsSchema = schemaToToolDefinition(ExplorationSchemas.SearchApiDocs);
export const getApiDocsSchema = schemaToToolDefinition(ExplorationSchemas.GetApiDocs);

// Execution tools
export const runWorkflowSchema = schemaToToolDefinition(ExecutionSchemas.RunWorkflow);
export const stopWorkflowSchema = schemaToToolDefinition(ExecutionSchemas.StopWorkflow);
export const validateWorkflowSchema = schemaToToolDefinition(ExecutionSchemas.ValidateWorkflow);

// =============================================================================
// Grouped Exports
// =============================================================================

export const mutationSchemas = [
  createJsNodeSchema,
  createHttpNodeSchema,
  createConditionNodeSchema,
  createForNodeSchema,
  createForEachNodeSchema,
  updateNodeCodeSchema,
  updateNodeConfigSchema,
  connectNodesSchema,
  disconnectNodesSchema,
  deleteNodeSchema,
  createVariableSchema,
  updateVariableSchema,
];

export const explorationSchemas = [
  getWorkflowGraphSchema,
  getNodeDetailsSchema,
  getNodeTemplateSchema,
  searchTemplatesSchema,
  getExecutionHistorySchema,
  getExecutionLogsSchema,
  searchApiDocsSchema,
  getApiDocsSchema,
];

export const executionSchemas = [
  runWorkflowSchema,
  stopWorkflowSchema,
  validateWorkflowSchema,
];

/**
 * All tool schemas combined - ready for AI tool calling
 */
export const allToolSchemas = [
  ...explorationSchemas,
  ...mutationSchemas,
  ...executionSchemas,
];

// =============================================================================
// Effect Schemas (for validation/parsing)
// =============================================================================

/**
 * Raw Effect Schema objects for use with Effect's decode/encode
 */
export const EffectSchemas = {
  Mutation: MutationSchemas,
  Exploration: ExplorationSchemas,
  Execution: ExecutionSchemas,
} as const;
