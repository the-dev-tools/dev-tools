/**
 * Effect Schema definitions for exploration (read-only) tools
 * These tools retrieve information without modifying state
 */

import { Schema } from 'effect';

import { ApiCategory, FlowId, NodeId, UlidId } from './common.ts';

// =============================================================================
// Workflow Graph Schemas
// =============================================================================

/**
 * Get complete workflow structure
 */
export const GetWorkflowGraph = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to retrieve',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'getWorkflowGraph',
    title: 'Get Workflow Graph',
    description:
      'Get the complete workflow graph including all nodes and edges. Use this to understand the current structure of the workflow.',
  }),
);

/**
 * Get detailed information about a specific node
 */
export const GetNodeDetails = Schema.Struct({
  nodeId: NodeId.pipe(
    Schema.annotations({
      description: 'The ULID of the node to inspect',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'getNodeDetails',
    title: 'Get Node Details',
    description:
      'Get detailed information about a specific node including its configuration, code (for JS nodes), and connections.',
  }),
);

// =============================================================================
// Template Schemas
// =============================================================================

/**
 * Get a specific node template by name
 */
export const GetNodeTemplate = Schema.Struct({
  templateName: Schema.String.pipe(
    Schema.annotations({
      description:
        'The name of the template to retrieve (e.g., "http-aggregator", "js-transformer")',
      examples: ['http-aggregator', 'js-transformer', 'conditional-router', 'data-validator'],
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'getNodeTemplate',
    title: 'Get Node Template',
    description:
      'Read a markdown template that provides guidance on implementing a specific type of node or pattern.',
  }),
);

/**
 * Search available templates
 */
export const SearchTemplates = Schema.Struct({
  query: Schema.String.pipe(
    Schema.annotations({
      description: 'Search query to find relevant templates',
      examples: ['http', 'transform', 'loop', 'condition'],
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'searchTemplates',
    title: 'Search Templates',
    description: 'Search for available templates by keyword or pattern.',
  }),
);

// =============================================================================
// Execution History Schemas
// =============================================================================

/**
 * Get workflow execution history
 */
export const GetExecutionHistory = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow',
    }),
  ),
  limit: Schema.optional(
    Schema.Number.pipe(
      Schema.int(),
      Schema.positive(),
      Schema.annotations({
        description: 'Maximum number of executions to return (default: 10)',
      }),
    ),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'getExecutionHistory',
    title: 'Get Execution History',
    description: 'Get the history of past workflow executions.',
  }),
);

/**
 * Get execution logs for debugging
 */
export const GetExecutionLogs = Schema.Struct({
  flowId: Schema.optional(FlowId).pipe(
    Schema.annotations({
      description: 'Filter to only show executions for nodes in this workflow',
    }),
  ),
  limit: Schema.optional(
    Schema.Number.pipe(
      Schema.int(),
      Schema.positive(),
      Schema.annotations({
        description: 'Maximum number of node executions to return (default: 10)',
      }),
    ),
  ),
  executionId: Schema.optional(UlidId).pipe(
    Schema.annotations({
      description: 'Optional: specific execution ID to get logs for',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'getExecutionLogs',
    title: 'Get Execution Logs',
    description:
      'Get the latest execution logs. Returns only the most recent execution per node to avoid showing full history.',
  }),
);

// =============================================================================
// API Documentation Schemas
// =============================================================================

/**
 * Search for API documentation
 */
export const SearchApiDocs = Schema.Struct({
  query: Schema.String.pipe(
    Schema.annotations({
      description:
        'Search query - API name, description keywords, or use case (e.g., "send message", "payment", "telegram bot")',
      examples: ['send message', 'payment processing', 'telegram bot', 'slack notification'],
    }),
  ),
  category: Schema.optional(ApiCategory),
  limit: Schema.optional(
    Schema.Number.pipe(
      Schema.int(),
      Schema.positive(),
      Schema.annotations({
        description: 'Maximum results to return (default: 5)',
      }),
    ),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'searchApiDocs',
    title: 'Search API Docs',
    description:
      'Search for API documentation by name, description, or keywords. Returns lightweight metadata for matching APIs. Use getApiDocs to load full documentation for a specific API.',
  }),
);

/**
 * Get full API documentation
 */
export const GetApiDocs = Schema.Struct({
  apiId: Schema.String.pipe(
    Schema.annotations({
      description: 'API identifier from search results (e.g., "slack", "stripe", "telegram")',
      examples: ['slack', 'stripe', 'telegram', 'github', 'notion'],
    }),
  ),
  forceRefresh: Schema.optional(Schema.Boolean).pipe(
    Schema.annotations({
      description: 'Force refresh from source, bypassing cache',
    }),
  ),
  endpoint: Schema.optional(Schema.String).pipe(
    Schema.annotations({
      description:
        'Optional filter to focus on specific endpoint (e.g., "chat.postMessage", "sendMessage")',
      examples: ['chat.postMessage', 'sendMessage', 'charges.create'],
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'getApiDocs',
    title: 'Get API Docs',
    description:
      'Load full documentation for a specific API. Call this after searchApiDocs to get complete endpoint details, authentication info, and examples.',
  }),
);

// =============================================================================
// Exports
// =============================================================================

export const ExplorationSchemas = {
  GetWorkflowGraph,
  GetNodeDetails,
  GetNodeTemplate,
  SearchTemplates,
  GetExecutionHistory,
  GetExecutionLogs,
  SearchApiDocs,
  GetApiDocs,
} as const;

export type GetWorkflowGraph = typeof GetWorkflowGraph.Type;
export type GetNodeDetails = typeof GetNodeDetails.Type;
export type GetNodeTemplate = typeof GetNodeTemplate.Type;
export type SearchTemplates = typeof SearchTemplates.Type;
export type GetExecutionHistory = typeof GetExecutionHistory.Type;
export type GetExecutionLogs = typeof GetExecutionLogs.Type;
export type SearchApiDocs = typeof SearchApiDocs.Type;
export type GetApiDocs = typeof GetApiDocs.Type;
