/**
 * Integration with Effect Schema Tool Definitions
 *
 * This file demonstrates how to use the Effect Schema-based tool definitions
 * from @the-dev-tools/spec instead of the hand-written JSON schemas.
 *
 * MIGRATION PATH:
 * 1. Import from @the-dev-tools/spec/tools (dynamic generation)
 *    OR @the-dev-tools/spec/tools/schemas (pre-generated JSON)
 * 2. Replace usage of local schemas with spec schemas
 * 3. Delete local schema files once migration is complete
 *
 * BENEFITS:
 * - Single source of truth in @the-dev-tools/spec
 * - Type-safe schema definitions with runtime validation
 * - Auto-generated JSON Schema for AI tool calling
 * - Annotations (descriptions, examples) are code, not strings
 * - When spec updates, regenerate schemas: `pnpm --filter @the-dev-tools/spec generate:schemas`
 */

// =============================================================================
// Option 1: Import pre-generated schemas (recommended for production)
// =============================================================================
// These are generated at build time via `pnpm generate:schemas` - no runtime overhead
// Use this when you don't need runtime validation, just the JSON Schema for AI tools
//
// import {
//   allToolSchemas,
//   mutationSchemas,
//   explorationSchemas,
//   executionSchemas,
//   createJsNodeSchema,
// } from '@the-dev-tools/spec/tools/schemas';

// =============================================================================
// Option 2: Import Effect Schemas directly (for validation + generation)
// =============================================================================
// These generate JSON Schema at runtime but also provide:
// - Runtime validation via Schema.decodeUnknown
// - Type inference via Schema.Type
// - Encoding/decoding transformations
//
import {
  allToolSchemas,
  createJsNodeSchema,
  EffectSchemas,
  executionSchemas,
  explorationSchemas,
  mutationSchemas,
  type ToolDefinition,
} from '@the-dev-tools/spec/tools';

// =============================================================================
// Re-export for backward compatibility
// =============================================================================

// Tool schema collections
export { allToolSchemas, executionSchemas, explorationSchemas, mutationSchemas };

// Individual mutation schemas
export {
  connectNodesSchema,
  createConditionNodeSchema,
  createForEachNodeSchema,
  createForNodeSchema,
  createHttpNodeSchema,
  createJsNodeSchema,
  createVariableSchema,
  deleteNodeSchema,
  disconnectNodesSchema,
  updateNodeCodeSchema,
  updateNodeConfigSchema,
  updateVariableSchema,
} from '@the-dev-tools/spec/tools';

// Individual exploration schemas
export {
  getApiDocsSchema,
  getExecutionHistorySchema,
  getExecutionLogsSchema,
  getNodeDetailsSchema,
  getNodeTemplateSchema,
  getWorkflowGraphSchema,
  searchApiDocsSchema,
  searchTemplatesSchema,
} from '@the-dev-tools/spec/tools';

// Individual execution schemas
export {
  runWorkflowSchema,
  stopWorkflowSchema,
  validateWorkflowSchema,
} from '@the-dev-tools/spec/tools';

// =============================================================================
// Type exports
// =============================================================================

export type { ToolDefinition };

// =============================================================================
// Validation utilities using Effect Schema
// =============================================================================

import { Schema } from 'effect';

/**
 * Validate tool input against the Effect Schema
 *
 * Example usage:
 * ```typescript
 * const result = validateToolInput('createJsNode', {
 *   flowId: '01ARZ3NDEKTSV4RRFFQ69G5FAV',
 *   name: 'Transform Data',
 *   code: 'return { result: ctx.value * 2 };'
 * });
 *
 * if (result.success) {
 *   console.log('Valid input:', result.data);
 * } else {
 *   console.error('Validation errors:', result.errors);
 * }
 * ```
 */
export function validateToolInput(
  toolName: string,
  input: unknown,
): { success: true; data: unknown } | { success: false; errors: string[] } {
  // Find the matching Effect Schema
  const schemaMap: Record<string, Schema.Schema<unknown, unknown>> = {
    // Mutation
    createJsNode: EffectSchemas.Mutation.CreateJsNode,
    createHttpNode: EffectSchemas.Mutation.CreateHttpNode,
    createConditionNode: EffectSchemas.Mutation.CreateConditionNode,
    createForNode: EffectSchemas.Mutation.CreateForNode,
    createForEachNode: EffectSchemas.Mutation.CreateForEachNode,
    updateNodeCode: EffectSchemas.Mutation.UpdateNodeCode,
    updateNodeConfig: EffectSchemas.Mutation.UpdateNodeConfig,
    connectNodes: EffectSchemas.Mutation.ConnectNodes,
    disconnectNodes: EffectSchemas.Mutation.DisconnectNodes,
    deleteNode: EffectSchemas.Mutation.DeleteNode,
    createVariable: EffectSchemas.Mutation.CreateVariable,
    updateVariable: EffectSchemas.Mutation.UpdateVariable,
    // Exploration
    getWorkflowGraph: EffectSchemas.Exploration.GetWorkflowGraph,
    getNodeDetails: EffectSchemas.Exploration.GetNodeDetails,
    getNodeTemplate: EffectSchemas.Exploration.GetNodeTemplate,
    searchTemplates: EffectSchemas.Exploration.SearchTemplates,
    getExecutionHistory: EffectSchemas.Exploration.GetExecutionHistory,
    getExecutionLogs: EffectSchemas.Exploration.GetExecutionLogs,
    searchApiDocs: EffectSchemas.Exploration.SearchApiDocs,
    getApiDocs: EffectSchemas.Exploration.GetApiDocs,
    // Execution
    runWorkflow: EffectSchemas.Execution.RunWorkflow,
    stopWorkflow: EffectSchemas.Execution.StopWorkflow,
    validateWorkflow: EffectSchemas.Execution.ValidateWorkflow,
  };

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

// =============================================================================
// Example: Using schemas with Claude/Anthropic API
// =============================================================================

/**
 * Example of how to use the schemas with Claude's tool_use API
 *
 * ```typescript
 * import Anthropic from '@anthropic-ai/sdk';
 * import { allToolSchemas } from '@the-dev-tools/spec/tools';
 *
 * const client = new Anthropic();
 *
 * const response = await client.messages.create({
 *   model: 'claude-sonnet-4-20250514',
 *   max_tokens: 1024,
 *   tools: allToolSchemas.map(schema => ({
 *     name: schema.name,
 *     description: schema.description,
 *     input_schema: schema.parameters,
 *   })),
 *   messages: [{ role: 'user', content: 'Create a JS node that doubles the input' }],
 * });
 * ```
 */

// =============================================================================
// Debug: Print schema to verify generation
// =============================================================================

if (import.meta.url === `file://${process.argv[1]}`) {
  console.log('Example createJsNode schema:');
  console.log(JSON.stringify(createJsNodeSchema, null, 2));
  console.log('\nTotal schemas:', allToolSchemas.length);
}
