/**
 * Effect Schema definitions for mutation (write) tools
 * These tools modify workflow state: create nodes, update configs, manage connections
 */

import { Schema } from 'effect';

import {
  ConditionExpression,
  EdgeId,
  ErrorHandling,
  FlowId,
  JsCode,
  NodeId,
  NodeName,
  OptionalPosition,
  SourceHandle,
  UlidId,
} from './common.ts';

// =============================================================================
// Node Creation Schemas
// =============================================================================

/**
 * Create a new JavaScript node in the workflow
 */
export const CreateJsNode = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to add the node to',
    }),
  ),
  name: NodeName,
  code: JsCode,
  position: OptionalPosition,
}).pipe(
  Schema.annotations({
    identifier: 'createJsNode',
    title: 'Create JavaScript Node',
    description:
      'Create a new JavaScript node in the workflow. JS nodes can transform data, make calculations, or perform custom logic.',
  }),
);

/**
 * Create a new HTTP request node
 */
export const CreateHttpNode = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to add the node to',
    }),
  ),
  name: NodeName,
  httpId: UlidId.pipe(
    Schema.annotations({
      description: 'The ULID of the HTTP request definition to use',
    }),
  ),
  position: OptionalPosition,
}).pipe(
  Schema.annotations({
    identifier: 'createHttpNode',
    title: 'Create HTTP Node',
    description: 'Create a new HTTP request node that makes an API call.',
  }),
);

/**
 * Create a condition (if/else) node
 */
export const CreateConditionNode = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to add the node to',
    }),
  ),
  name: NodeName,
  condition: ConditionExpression,
  position: OptionalPosition,
}).pipe(
  Schema.annotations({
    identifier: 'createConditionNode',
    title: 'Create Condition Node',
    description:
      'Create a condition node that routes flow based on a boolean expression. Has THEN and ELSE output handles.',
  }),
);

/**
 * Create a for-loop node with fixed iterations
 */
export const CreateForNode = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to add the node to',
    }),
  ),
  name: NodeName,
  iterations: Schema.Number.pipe(
    Schema.int(),
    Schema.positive(),
    Schema.annotations({
      description: 'Number of iterations to perform',
    }),
  ),
  condition: ConditionExpression.pipe(
    Schema.annotations({
      description:
        'Optional condition to continue loop using expr-lang syntax (e.g., "i < 10"). Use == for equality (NOT ===)',
    }),
  ),
  errorHandling: ErrorHandling,
  position: OptionalPosition,
}).pipe(
  Schema.annotations({
    identifier: 'createForNode',
    title: 'Create For Loop Node',
    description: 'Create a for-loop node that iterates a fixed number of times.',
  }),
);

/**
 * Create a forEach loop node for iterating arrays/objects
 */
export const CreateForEachNode = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to add the node to',
    }),
  ),
  name: NodeName,
  path: Schema.String.pipe(
    Schema.annotations({
      description: 'Path to the array/object to iterate (e.g., "input.items")',
      examples: ['input.items', 'data.users', 'response.results'],
    }),
  ),
  condition: ConditionExpression.pipe(
    Schema.annotations({
      description:
        'Optional condition to continue iteration using expr-lang syntax. Use == for equality (NOT ===)',
    }),
  ),
  errorHandling: ErrorHandling,
  position: OptionalPosition,
}).pipe(
  Schema.annotations({
    identifier: 'createForEachNode',
    title: 'Create ForEach Loop Node',
    description: 'Create a forEach node that iterates over an array or object.',
  }),
);

// =============================================================================
// Node Update Schemas
// =============================================================================

/**
 * Update JavaScript code in a JS node
 */
export const UpdateNodeCode = Schema.Struct({
  nodeId: NodeId.pipe(
    Schema.annotations({
      description: 'The ULID of the JS node to update',
    }),
  ),
  code: JsCode,
}).pipe(
  Schema.annotations({
    identifier: 'updateNodeCode',
    title: 'Update Node Code',
    description: 'Update the JavaScript code of a JS node.',
  }),
);

/**
 * Update general node configuration (name, position)
 */
export const UpdateNodeConfig = Schema.Struct({
  nodeId: NodeId.pipe(
    Schema.annotations({
      description: 'The ULID of the node to update',
    }),
  ),
  name: Schema.optional(NodeName).pipe(
    Schema.annotations({
      description: 'New display name (optional)',
    }),
  ),
  position: OptionalPosition,
}).pipe(
  Schema.annotations({
    identifier: 'updateNodeConfig',
    title: 'Update Node Config',
    description: 'Update general node properties like name or position.',
  }),
);

// =============================================================================
// Edge (Connection) Schemas
// =============================================================================

/**
 * Connect two nodes with an edge
 */
export const ConnectNodes = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow',
    }),
  ),
  sourceId: NodeId.pipe(
    Schema.annotations({
      description: 'The ULID of the source node',
    }),
  ),
  targetId: NodeId.pipe(
    Schema.annotations({
      description: 'The ULID of the target node',
    }),
  ),
  sourceHandle: Schema.optional(SourceHandle).pipe(
    Schema.annotations({
      description:
        'Output handle for branching nodes ONLY. Use "then"/"else" for Condition nodes, "loop"/"then" for For/ForEach nodes. OMIT this parameter for Manual Start, JS, and HTTP nodes.',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'connectNodes',
    title: 'Connect Nodes',
    description:
      'Create an edge connection between two nodes. IMPORTANT: For sequential flows (Manual Start, JS, HTTP nodes), do NOT specify sourceHandle - omit it entirely. Only use sourceHandle for Condition nodes (then/else) and Loop nodes (loop/then).',
  }),
);

/**
 * Remove an edge connection
 */
export const DisconnectNodes = Schema.Struct({
  edgeId: EdgeId.pipe(
    Schema.annotations({
      description: 'The ULID of the edge to remove',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'disconnectNodes',
    title: 'Disconnect Nodes',
    description: 'Remove an edge connection between nodes.',
  }),
);

/**
 * Delete a node from the workflow
 */
export const DeleteNode = Schema.Struct({
  nodeId: NodeId.pipe(
    Schema.annotations({
      description: 'The ULID of the node to delete',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'deleteNode',
    title: 'Delete Node',
    description: 'Delete a node from the workflow. Also removes all connected edges.',
  }),
);

// =============================================================================
// Variable Schemas
// =============================================================================

/**
 * Create a new workflow variable
 */
export const CreateVariable = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow',
    }),
  ),
  key: Schema.String.pipe(
    Schema.minLength(1),
    Schema.annotations({
      description: 'Variable name (used to reference it in expressions)',
      examples: ['apiKey', 'baseUrl', 'maxRetries'],
    }),
  ),
  value: Schema.String.pipe(
    Schema.annotations({
      description: 'Variable value',
    }),
  ),
  description: Schema.optional(Schema.String).pipe(
    Schema.annotations({
      description: 'Description of what the variable is for (optional)',
    }),
  ),
  enabled: Schema.optional(Schema.Boolean).pipe(
    Schema.annotations({
      description: 'Whether the variable is active (default: true)',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'createVariable',
    title: 'Create Variable',
    description: 'Create a new workflow variable that can be referenced in node expressions.',
  }),
);

/**
 * Update an existing workflow variable
 */
export const UpdateVariable = Schema.Struct({
  flowVariableId: UlidId.pipe(
    Schema.annotations({
      description: 'The ULID of the variable to update',
    }),
  ),
  key: Schema.optional(Schema.String).pipe(
    Schema.annotations({
      description: 'New variable name (optional)',
    }),
  ),
  value: Schema.optional(Schema.String).pipe(
    Schema.annotations({
      description: 'New variable value (optional)',
    }),
  ),
  description: Schema.optional(Schema.String).pipe(
    Schema.annotations({
      description: 'New description (optional)',
    }),
  ),
  enabled: Schema.optional(Schema.Boolean).pipe(
    Schema.annotations({
      description: 'Whether the variable is active (optional)',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'updateVariable',
    title: 'Update Variable',
    description: 'Update an existing workflow variable.',
  }),
);

// =============================================================================
// Exports
// =============================================================================

export const MutationSchemas = {
  CreateJsNode,
  CreateHttpNode,
  CreateConditionNode,
  CreateForNode,
  CreateForEachNode,
  UpdateNodeCode,
  UpdateNodeConfig,
  ConnectNodes,
  DisconnectNodes,
  DeleteNode,
  CreateVariable,
  UpdateVariable,
} as const;

export type CreateJsNode = typeof CreateJsNode.Type;
export type CreateHttpNode = typeof CreateHttpNode.Type;
export type CreateConditionNode = typeof CreateConditionNode.Type;
export type CreateForNode = typeof CreateForNode.Type;
export type CreateForEachNode = typeof CreateForEachNode.Type;
export type UpdateNodeCode = typeof UpdateNodeCode.Type;
export type UpdateNodeConfig = typeof UpdateNodeConfig.Type;
export type ConnectNodes = typeof ConnectNodes.Type;
export type DisconnectNodes = typeof DisconnectNodes.Type;
export type DeleteNode = typeof DeleteNode.Type;
export type CreateVariable = typeof CreateVariable.Type;
export type UpdateVariable = typeof UpdateVariable.Type;
