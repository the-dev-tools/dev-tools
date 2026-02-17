/* eslint-disable @typescript-eslint/await-thenable, @typescript-eslint/no-base-to-string, @typescript-eslint/no-confusing-void-expression, @typescript-eslint/no-unnecessary-condition, @typescript-eslint/no-unnecessary-type-conversion, @typescript-eslint/no-unsafe-argument, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-return, @typescript-eslint/restrict-template-expressions */
import type { Transport } from '@connectrpc/connect';
import { eq } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { FileKind } from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import { FlowItemState, FlowService, HandleKind, NodeKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { HttpBodyKind, HttpMethod } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { request } from '~/shared/api';
import { queryCollection } from '~/shared/lib';
import type { FlowContextData, ToolCall, ToolResult } from './types';

type CollectionUtils = ReturnType<typeof import('~/shared/api').useApiCollection>['utils'];
type CollectionData = ReturnType<typeof import('~/shared/api').useApiCollection>;

/**
 * Normalizes JS code references by replacing whitespace with underscores in node names.
 * - ["Node Name"].field → ["Node_Name"].field
 */
function normalizeJsCodeReferences(code: string): string {
  if (!code) return code;

  // Pattern: ["NodeName"] - replace whitespace in node name with underscores
  return code.replace(/\["([^"]+)"\]/g, (_, nodeName) => `["${nodeName.replace(/\s+/g, '_')}"]`);
}

/**
 * Normalizes condition expressions by:
 * - Removing bracket/quote syntax: ["NodeName"].field → NodeName.field
 * - Replacing whitespace with underscores in node names
 * - Converting JS strict equality/inequality to expr-lang operators
 */
function normalizeConditionSyntax(expr: string): string {
  if (!expr) return expr;

  // Pattern: ["NodeName"] - convert to plain identifier with underscores
  let normalized = expr.replace(/\["([^"]+)"\]/g, (_, nodeName) => nodeName.replace(/\s+/g, '_'));

  // Convert JS strict equality/inequality to expr-lang operators
  normalized = normalized.replace(/===/g, '==');
  normalized = normalized.replace(/!==/g, '!=');

  return normalized;
}

/**
 * Normalizes node names by replacing whitespace with underscores.
 */
function normalizeNodeName(name: string): string {
  if (!name) return name;
  return name.replace(/\s+/g, '_');
}

interface Collections {
  aiCollection: { utils: CollectionUtils };
  conditionCollection: { utils: CollectionUtils };
  edgeCollection: { utils: CollectionUtils };
  executionCollection: CollectionData;
  fileCollection: CollectionData;
  forCollection: { utils: CollectionUtils };
  forEachCollection: { utils: CollectionUtils };
  httpAssertCollection: { utils: CollectionUtils };
  httpBodyRawCollection: { utils: CollectionUtils };
  httpCollection: { utils: CollectionUtils };
  httpHeaderCollection: { utils: CollectionUtils };
  httpSearchParamCollection: { utils: CollectionUtils };
  jsCollection: { utils: CollectionUtils };
  nodeCollection: { utils: CollectionUtils };
  nodeHttpCollection: { utils: CollectionUtils };
  variableCollection: { utils: CollectionUtils };
}

interface ToolExecutorContext {
  collections: Collections;
  flowContext: FlowContextData;
  sessionCreatedNodeIds: Set<string>;
  transport: Transport;
  waitForFlowCompletion: () => Promise<void>;
  workspaceId: Uint8Array;
}

const parseUlid = (id: string): Uint8Array => Ulid.fromCanonical(id).bytes;

const HANDLE_KIND_MAP: Record<string, HandleKind> = {
  ai_tools: HandleKind.AI_TOOLS,
  else: HandleKind.ELSE,
  loop: HandleKind.LOOP,
  then: HandleKind.THEN,
};

const HTTP_METHOD_MAP: Record<string, HttpMethod> = {
  DELETE: HttpMethod.DELETE,
  GET: HttpMethod.GET,
  HEAD: HttpMethod.HEAD,
  OPTIONS: HttpMethod.OPTIONS,
  PATCH: HttpMethod.PATCH,
  POST: HttpMethod.POST,
  PUT: HttpMethod.PUT,
};

const NODE_KIND_NAMES: Record<number, string> = {
  [NodeKind.AI]: 'Ai',
  [NodeKind.CONDITION]: 'Condition',
  [NodeKind.FOR]: 'For',
  [NodeKind.FOR_EACH]: 'ForEach',
  [NodeKind.HTTP]: 'HTTP',
  [NodeKind.JS]: 'JavaScript',
  [NodeKind.MANUAL_START]: 'ManualStart',
  [NodeKind.UNSPECIFIED]: 'Unknown',
};

const FLOW_ITEM_STATE_NAMES: Record<number, string> = {
  [FlowItemState.CANCELED]: 'Canceled',
  [FlowItemState.FAILURE]: 'Failure',
  [FlowItemState.RUNNING]: 'Running',
  [FlowItemState.SUCCESS]: 'Success',
  [FlowItemState.UNSPECIFIED]: 'Idle',
};

const AGENT_MAX_FILE_ORDER = 1_000_000_000;

const areBytesEqual = (left: Uint8Array, right: Uint8Array): boolean => {
  if (left.length !== right.length) return false;
  for (let i = 0; i < left.length; i++) {
    if (left[i] !== right[i]) return false;
  }
  return true;
};

const getNextAgentFileOrder = async (fileCollection: CollectionData, workspaceId: Uint8Array): Promise<number> => {
  const files = await queryCollection((_) => _.from({ item: fileCollection }));

  let maxOrder = 0;
  for (const file of files) {
    if (typeof file !== 'object' || file === null) continue;

    const fileData = file as Record<string, unknown>;
    const fileWorkspaceId = fileData['workspaceId'];

    if (!(fileWorkspaceId instanceof Uint8Array)) continue;
    if (!areBytesEqual(fileWorkspaceId, workspaceId)) continue;

    const order = fileData['order'];
    if (typeof order !== 'number') continue;
    if (!Number.isFinite(order)) continue;
    if (Math.abs(order) > AGENT_MAX_FILE_ORDER) continue;
    if (order > maxOrder) maxOrder = order;
  }

  return maxOrder + 1;
};

const MUTATION_TOOLS = new Set([
  'connectChain',
  'createAiNode',
  'createConditionNode',
  'createForEachNode',
  'createForNode',
  'createHttpNode',
  'createJsNode',
  'deleteNode',
  'disconnectNodes',
  'patchHttpNode',
  'updateNode',
]);

export const executeToolCall = async (
  toolCall: ToolCall,
  flowId: Uint8Array,
  context: ToolExecutorContext,
): Promise<ToolResult> => {
  const { arguments: args, id, name } = toolCall;
  const isMutation = MUTATION_TOOLS.has(name);

  try {
    const result = await executeToolInternal(name, args, flowId, context);
    return { isMutation, result, toolCallId: id };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : String(error),
      isMutation,
      result: null,
      toolCallId: id,
    };
  }
};

const executeToolInternal = async (
  name: string,
  args: Record<string, unknown>,
  flowId: Uint8Array,
  context: ToolExecutorContext,
): Promise<unknown> => {
  const { collections, flowContext, transport, workspaceId } = context;
  const {
    aiCollection,
    conditionCollection,
    edgeCollection,
    executionCollection,
    fileCollection,
    forCollection,
    forEachCollection,
    httpAssertCollection,
    httpBodyRawCollection,
    httpCollection,
    httpHeaderCollection,
    httpSearchParamCollection,
    jsCollection,
    nodeCollection,
    nodeHttpCollection,
    variableCollection,
  } = collections;

  switch (name) {
    case 'connectChain': {
      const nodeIds = args.nodeIds as (string | string[])[];
      const handleOverride = args.sourceHandle as string | undefined;
      if (handleOverride && !['ai_tools', 'else', 'loop', 'then'].includes(handleOverride)) {
        throw new Error(`Invalid sourceHandle "${handleOverride}". Valid values: "then", "else", "loop", "ai_tools".`);
      }
      if (!nodeIds || nodeIds.length < 2) {
        throw new Error('connectChain requires at least 2 elements.');
      }

      // Validate: no consecutive nested arrays
      for (let i = 0; i < nodeIds.length - 1; i++) {
        if (Array.isArray(nodeIds[i]) && Array.isArray(nodeIds[i + 1])) {
          throw new Error(
            `connectChain: consecutive nested arrays at positions ${i} and ${i + 1} are not allowed. ` +
              `Insert a shared fan-in node between the groups, or split into separate connectChain calls. ` +
              `Example: instead of ["A",["B","C"],["D","E"],"F"], use ["A",["B","C"],"Mid"] then ["Mid",["D","E"],"F"].`,
          );
        }
      }

      // Validate: parallel groups have ≥2 unique IDs
      for (let i = 0; i < nodeIds.length; i++) {
        const el = nodeIds[i]!;
        if (Array.isArray(el)) {
          const unique = new Set(el);
          if (unique.size < 2) {
            throw new Error(`connectChain: parallel group at position ${i} must have at least 2 unique node IDs.`);
          }
          if (unique.size !== el.length) {
            throw new Error(`connectChain: parallel group at position ${i} contains duplicate node IDs.`);
          }
        }
      }

      // Expand consecutive element pairs into edge pairs
      const edgePairs: [string, string][] = [];
      for (let i = 0; i < nodeIds.length - 1; i++) {
        const current = nodeIds[i]!;
        const next = nodeIds[i + 1]!;
        const sources = Array.isArray(current) ? current : [current];
        const targets = Array.isArray(next) ? next : [next];
        for (const s of sources) for (const t of targets) edgePairs.push([s, t]);
      }

      const edgeIds: string[] = [];
      const errors: string[] = [];

      // Process SEQUENTIALLY to avoid parallel race conditions
      for (let idx = 0; idx < edgePairs.length; idx++) {
        const [sourceIdStr, targetIdStr] = edgePairs[idx]!;

        try {
          const sourceId = parseUlid(sourceIdStr);
          const targetId = parseUlid(targetIdStr);
          const edgeId = Ulid.generate().bytes;

          // Query live edges to check for existing outgoing connections
          const existingEdges = await queryCollection((_) =>
            _.from({ e: edgeCollection }).where((_) => eq(_.e.sourceId, sourceId)),
          );

          const duplicateEdge = existingEdges.find((e) => Ulid.construct(e.targetId).toCanonical() === targetIdStr);
          if (duplicateEdge) {
            errors.push(`Edge ${idx}: Edge from ${sourceIdStr} to ${targetIdStr} already exists. Skipped.`);
            continue;
          }

          // Determine handle kind for branching nodes
          const sourceNode = flowContext.nodes.find((n) => n.id === sourceIdStr);
          const isBranching = sourceNode && ['Condition', 'For', 'ForEach'].includes(sourceNode.kind);
          const isAiSource = sourceNode?.kind === 'Ai';

          // Validate handle is valid for the specific node type
          if (isBranching && handleOverride) {
            const validHandles = sourceNode.kind === 'Condition' ? ['then', 'else'] : ['then', 'loop'];
            if (!validHandles.includes(handleOverride)) {
              errors.push(
                `Edge ${idx}: Invalid sourceHandle "${handleOverride}" for ${sourceNode.kind} node "${sourceNode.name}". ` +
                  `Valid handles: ${validHandles.join(', ')}. Skipped.`,
              );
              continue;
            }
          }

          if (isAiSource && handleOverride) {
            const validHandles = ['ai_tools'];
            if (!validHandles.includes(handleOverride)) {
              errors.push(
                `Edge ${idx}: Invalid sourceHandle "${handleOverride}" for Ai node "${sourceNode.name}". ` +
                  `Valid handles: ${validHandles.join(', ')}. Skipped.`,
              );
              continue;
            }
          }

          const edgeHandle = isBranching
            ? (HANDLE_KIND_MAP[handleOverride ?? 'then'] ?? HandleKind.THEN)
            : isAiSource && handleOverride
              ? HANDLE_KIND_MAP[handleOverride]
              : undefined;

          await edgeCollection.utils.insert({
            edgeId,
            flowId,
            sourceId,
            targetId,
            ...(edgeHandle !== undefined ? { sourceHandle: edgeHandle } : {}),
          });

          edgeIds.push(Ulid.construct(edgeId).toCanonical());
        } catch (error) {
          errors.push(`Edge ${idx}: ${error instanceof Error ? error.message : String(error)}`);
        }
      }

      return {
        edgeIds,
        edgesCreated: edgeIds.length,
        ...(errors.length > 0 ? { errors } : {}),
      };
    }

    case 'createAiNode': {
      const nodeId = Ulid.generate().bytes;
      const position = (args.position as { x: number; y: number }) ?? { x: 0, y: 0 };
      const nodeName = normalizeNodeName(args.name as string);
      const prompt = args.prompt as string;
      const maxIterations = (args.maxIterations as number | undefined) ?? 5;

      if (!Number.isInteger(maxIterations) || maxIterations <= 0) {
        throw new Error(`maxIterations must be a positive integer, got: ${maxIterations}`);
      }

      // Call both inserts before awaiting to ensure optimistic updates happen
      // synchronously before any sync responses can arrive from the server
      const nodePromise = nodeCollection.utils.insert({
        flowId,
        kind: NodeKind.AI,
        name: nodeName,
        nodeId,
        position,
      });

      const aiPromise = aiCollection.utils.insert({
        maxIterations,
        nodeId,
        prompt,
      });

      await Promise.all([nodePromise, aiPromise]);

      const canonicalId = Ulid.construct(nodeId).toCanonical();
      context.sessionCreatedNodeIds.add(canonicalId);
      return { name: nodeName, nodeId: canonicalId };
    }

    case 'createConditionNode': {
      const nodeId = Ulid.generate().bytes;
      const position = (args.position as { x: number; y: number }) ?? { x: 0, y: 0 };
      const condition = normalizeConditionSyntax(args.condition as string);
      const nodeName = normalizeNodeName(args.name as string);

      // Call both inserts before awaiting to ensure optimistic updates happen
      // synchronously before any sync responses can arrive from the server
      const nodePromise = nodeCollection.utils.insert({
        flowId,
        kind: NodeKind.CONDITION,
        name: nodeName,
        nodeId,
        position,
      });

      const conditionPromise = conditionCollection.utils.insert({
        condition,
        nodeId,
      });

      await Promise.all([nodePromise, conditionPromise]);

      {
        const canonicalId = Ulid.construct(nodeId).toCanonical();
        context.sessionCreatedNodeIds.add(canonicalId);
        return { name: nodeName, nodeId: canonicalId };
      }
    }

    case 'createForEachNode': {
      // Validate path is provided
      const rawPath = args.path as string | undefined;
      if (!rawPath || rawPath.trim() === '') {
        throw new Error(
          'path is required for ForEach nodes. ' +
            'Provide an expression for the array/object to iterate. ' +
            'Example: HTTP_Request.response.body.items',
        );
      }

      // Validate break condition is provided
      const rawCondition = args.condition as string | undefined;
      if (!rawCondition || rawCondition.trim() === '') {
        throw new Error(
          'condition (break condition) is required for ForEach nodes. ' +
            'Provide an expression that evaluates to true to exit the loop early. ' +
            'Example: ForEach_Loop.key >= 5',
        );
      }

      const nodeId = Ulid.generate().bytes;
      const position = (args.position as { x: number; y: number }) ?? { x: 0, y: 0 };
      const path = normalizeConditionSyntax(rawPath);
      const condition = normalizeConditionSyntax(rawCondition);
      const errorHandling = args.errorHandling as string;
      const nodeName = normalizeNodeName(args.name as string);

      // Call both inserts before awaiting to ensure optimistic updates happen
      // synchronously before any sync responses can arrive from the server
      const nodePromise = nodeCollection.utils.insert({
        flowId,
        kind: NodeKind.FOR_EACH,
        name: nodeName,
        nodeId,
        position,
      });

      const forEachPromise = forEachCollection.utils.insert({
        condition,
        errorHandling: errorHandling === 'break' ? 1 : 0,
        nodeId,
        path,
      });

      await Promise.all([nodePromise, forEachPromise]);

      {
        const canonicalId = Ulid.construct(nodeId).toCanonical();
        context.sessionCreatedNodeIds.add(canonicalId);
        return { name: nodeName, nodeId: canonicalId };
      }
    }

    case 'createForNode': {
      // Validate iterations is a positive integer
      const iterations = args.iterations as number | undefined;
      if (iterations === undefined || iterations === null) {
        throw new Error('iterations is required for For nodes. Specify the number of times to iterate.');
      }
      if (!Number.isInteger(iterations) || iterations <= 0) {
        throw new Error(`iterations must be a positive integer, got: ${iterations}`);
      }

      // Validate break condition is provided
      const rawCondition = args.condition as string | undefined;
      if (!rawCondition || rawCondition.trim() === '') {
        throw new Error(
          'condition (break condition) is required for For nodes. ' +
            'Provide an expression that evaluates to true to exit the loop early. ' +
            'Example: Counter.count >= 10',
        );
      }

      const nodeId = Ulid.generate().bytes;
      const position = (args.position as { x: number; y: number }) ?? { x: 0, y: 0 };
      const condition = normalizeConditionSyntax(rawCondition);
      const errorHandling = args.errorHandling as string;
      const nodeName = normalizeNodeName(args.name as string);

      // Call both inserts before awaiting to ensure optimistic updates happen
      // synchronously before any sync responses can arrive from the server
      const nodePromise = nodeCollection.utils.insert({
        flowId,
        kind: NodeKind.FOR,
        name: nodeName,
        nodeId,
        position,
      });

      const forPromise = forCollection.utils.insert({
        condition,
        errorHandling: errorHandling === 'break' ? 1 : 0,
        iterations,
        nodeId,
      });

      await Promise.all([nodePromise, forPromise]);

      {
        const canonicalId = Ulid.construct(nodeId).toCanonical();
        context.sessionCreatedNodeIds.add(canonicalId);
        return { name: nodeName, nodeId: canonicalId };
      }
    }

    case 'createHttpNode': {
      const nodeId = Ulid.generate().bytes;
      const position = (args.position as { x: number; y: number }) ?? { x: 0, y: 0 };
      const nodeName = normalizeNodeName(args.name as string);

      let httpId: Uint8Array;
      let httpIdStr: string;
      const insertPromises: Promise<unknown>[] = [];

      if (args.httpId) {
        // Use existing HTTP request
        httpId = parseUlid(args.httpId as string);
        httpIdStr = args.httpId as string;
      } else {
        // Validate HTTP method
        const methodStr = ((args.method as string) ?? '').toUpperCase();
        if (!methodStr) {
          throw new Error(
            'method is required when creating a new HTTP node. ' +
              'Valid methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS',
          );
        }
        const method = HTTP_METHOD_MAP[methodStr];
        if (method === undefined) {
          throw new Error(
            `Invalid HTTP method: "${args.method}". ` + 'Valid methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS',
          );
        }

        const url = (args.url as string) ?? '';
        const methodsWithBody = new Set(['PATCH', 'POST', 'PUT']);
        const needsBody = methodsWithBody.has(methodStr);

        // Create new HTTP request with appropriate bodyKind
        httpId = Ulid.generate().bytes;
        httpIdStr = Ulid.construct(httpId).toCanonical();

        insertPromises.push(
          httpCollection.utils.insert({
            bodyKind: needsBody ? HttpBodyKind.RAW : HttpBodyKind.UNSPECIFIED,
            httpId,
            method,
            name: nodeName,
            url,
          }),
          getNextAgentFileOrder(fileCollection, workspaceId).then((order) =>
            fileCollection.utils.insert({
              fileId: httpId,
              kind: FileKind.HTTP,
              order,
              workspaceId,
            }),
          ),
        );

        // If a body is provided and the method supports it, insert the raw body
        const body = args.body as string | undefined;
        if (body && needsBody) {
          insertPromises.push(
            collections.httpBodyRawCollection.utils.insert({
              data: body,
              httpId,
            }),
          );
        } else if (body && !needsBody) {
          throw new Error(
            `Cannot set body for ${methodStr} requests. ` + 'Only POST, PUT, and PATCH methods support a request body.',
          );
        }
      }

      // Call all inserts before awaiting to ensure optimistic updates happen
      // synchronously before any sync responses can arrive from the server
      insertPromises.push(
        nodeCollection.utils.insert({
          flowId,
          kind: NodeKind.HTTP,
          name: nodeName,
          nodeId,
          position,
        }),
        nodeHttpCollection.utils.insert({
          httpId,
          nodeId,
        }),
      );

      await Promise.all(insertPromises);

      {
        const canonicalId = Ulid.construct(nodeId).toCanonical();
        context.sessionCreatedNodeIds.add(canonicalId);
        return { httpId: httpIdStr, name: nodeName, nodeId: canonicalId };
      }
    }

    case 'createJsNode': {
      const nodeId = Ulid.generate().bytes;
      const position = (args.position as { x: number; y: number }) ?? { x: 0, y: 0 };
      const code = normalizeJsCodeReferences(args.code as string);
      const nodeName = normalizeNodeName(args.name as string);

      // Call both inserts before awaiting to ensure optimistic updates happen
      // synchronously before any sync responses can arrive from the server
      const nodePromise = nodeCollection.utils.insert({
        flowId,
        kind: NodeKind.JS,
        name: nodeName,
        nodeId,
        position,
      });

      const jsPromise = jsCollection.utils.insert({
        code: `export default function(ctx) {\n  ${code}\n}`,
        nodeId,
      });

      await Promise.all([nodePromise, jsPromise]);

      {
        const canonicalId = Ulid.construct(nodeId).toCanonical();
        context.sessionCreatedNodeIds.add(canonicalId);
        return { name: nodeName, nodeId: canonicalId };
      }
    }

    case 'createVariable': {
      const flowVariableId = Ulid.generate().bytes;
      const key = args.key as string;
      const value = args.value as string;
      const enabled = args.enabled as boolean;
      const description = args.description as string;
      const order = args.order as number;

      // Await to ensure server persistence before returning
      await variableCollection.utils.insert({
        description,
        enabled,
        flowId,
        flowVariableId,
        key,
        order,
        value,
      });

      return { flowVariableId: Ulid.construct(flowVariableId).toCanonical() };
    }

    case 'deleteNode': {
      const nodeIdStr = args.nodeId as string;

      if (context.sessionCreatedNodeIds.has(nodeIdStr)) {
        return {
          blocked: true,
          message:
            'Cannot delete a node you just created. If the node has an error, explain the issue to the user and suggest what they can do to fix it (e.g., adding an AI Provider node). Do NOT delete and recreate with a different node type.',
        };
      }

      const nodeId = parseUlid(nodeIdStr);

      // Query live edges from collection to avoid stale flowContext during batched tool calls.
      const liveEdges = await queryCollection((_) =>
        _.from({ edge: edgeCollection }).where((_) => eq(_.edge.flowId, flowId)),
      );
      const connectedEdgeIds = liveEdges
        .filter((edge) => edge.edgeId != null && edge.sourceId != null && edge.targetId != null)
        .filter((edge) => areBytesEqual(edge.sourceId, nodeId) || areBytesEqual(edge.targetId, nodeId))
        .map((edge) => edge.edgeId);

      for (const edgeId of connectedEdgeIds) {
        edgeCollection.utils.delete({ edgeId });
      }

      nodeCollection.utils.delete({ nodeId });
      return { deletedEdges: connectedEdgeIds.length, success: true };
    }

    case 'disconnectNodes': {
      const edgeId = parseUlid(args.edgeId as string);
      edgeCollection.utils.delete({ edgeId });
      return { success: true };
    }

    case 'flowRunRequest': {
      await request({
        input: { flowId },
        method: FlowService.method.flowRun,
        transport,
      });

      await context.waitForFlowCompletion();

      return {
        message: 'Flow execution completed. Use getFlowExecutionSummary to inspect results.',
        success: true,
      };
    }

    case 'flowStopRequest': {
      await request({
        input: { flowId },
        method: FlowService.method.flowStop,
        transport,
      });
      return { message: 'Flow execution stopped', success: true };
    }

    case 'getFlowExecutionSummary': {
      // Query fresh nodes from the collection
      const freshNodes = await queryCollection((_) =>
        _.from({ node: collections.nodeCollection }).where((_) => eq(_.node.flowId, flowId)),
      );

      // Build a set of node IDs belonging to this flow
      const nodeIdSet = new Set(
        freshNodes.filter((n) => n.nodeId != null).map((n) => Ulid.construct(n.nodeId).toCanonical()),
      );

      // Query all executions and filter to this flow's nodes
      const allExecs = await queryCollection((_) => _.from({ exec: collections.executionCollection }));
      const flowExecs = allExecs.filter(
        (e) => e.nodeId != null && nodeIdSet.has(Ulid.construct(e.nodeId).toCanonical()),
      );
      const executedNodeIds = new Set(flowExecs.map((e) => Ulid.construct(e.nodeId).toCanonical()));

      // Build executed nodes list with state from execution records
      const executedNodes = freshNodes
        .filter((n) => n.nodeId != null && executedNodeIds.has(Ulid.construct(n.nodeId).toCanonical()))
        .map((n) => {
          const nodeExecs = flowExecs
            .filter((e) => Ulid.construct(e.nodeId).toCanonical() === Ulid.construct(n.nodeId).toCanonical())
            .sort((a, b) => {
              if (!a.completedAt && !b.completedAt) return 0;
              if (!a.completedAt) return 1;
              if (!b.completedAt) return -1;
              return Number(b.completedAt - a.completedAt);
            });
          const latestExec = nodeExecs[0];
          return {
            id: Ulid.construct(n.nodeId).toCanonical(),
            name: n.name,
            state: latestExec ? (FLOW_ITEM_STATE_NAMES[latestExec.state] ?? 'Unknown') : 'Unknown',
          };
        });

      // Never-reached: non-ManualStart nodes without any executions
      const neverReachedNodes = freshNodes
        .filter(
          (n) =>
            n.nodeId != null &&
            n.kind !== NodeKind.MANUAL_START &&
            !executedNodeIds.has(Ulid.construct(n.nodeId).toCanonical()),
        )
        .map((n) => ({
          id: Ulid.construct(n.nodeId).toCanonical(),
          kind: NODE_KIND_NAMES[n.kind] ?? 'Unknown',
          name: n.name,
        }));

      return {
        executedNodes,
        neverReachedNodes,
        warning:
          neverReachedNodes.length > 0
            ? `${neverReachedNodes.length} node(s) were never reached during execution. This may indicate an untaken branch or a wiring problem.`
            : undefined,
      };
    }

    case 'inspectNode': {
      const nodeIdStr = args.nodeId as string;
      const includeOutput = (args.includeOutput as boolean) ?? false;
      const node = flowContext.nodes.find((n) => n.id === nodeIdStr);
      if (!node) throw new Error(`Node not found: ${nodeIdStr}`);

      const nodeIdBytes = parseUlid(nodeIdStr);

      // Base info (always returned)
      const result: Record<string, unknown> = {
        error: node.info ?? undefined,
        id: node.id,
        kind: node.kind,
        name: node.name,
        state: node.state,
      };

      // Type-specific config
      switch (node.kind) {
        case 'Condition': {
          const [condData] = await queryCollection((_) =>
            _.from({ cond: conditionCollection })
              .where((_) => eq(_.cond.nodeId, nodeIdBytes))
              .findOne(),
          );
          result.condition = condData?.condition ?? '';
          break;
        }
        case 'For': {
          const [forData] = await queryCollection((_) =>
            _.from({ f: forCollection })
              .where((_) => eq(_.f.nodeId, nodeIdBytes))
              .findOne(),
          );
          result.iterations = forData?.iterations;
          result.condition = forData?.condition ?? '';
          result.errorHandling = forData?.errorHandling === 1 ? 'break' : 'continue';
          break;
        }
        case 'ForEach': {
          const [feData] = await queryCollection((_) =>
            _.from({ fe: forEachCollection })
              .where((_) => eq(_.fe.nodeId, nodeIdBytes))
              .findOne(),
          );
          result.path = feData?.path ?? '';
          result.condition = feData?.condition ?? '';
          result.errorHandling = feData?.errorHandling === 1 ? 'break' : 'continue';
          break;
        }
        case 'HTTP': {
          if (!node.httpId) break;
          const httpIdBytes = parseUlid(node.httpId);

          const [httpData] = await queryCollection((_) =>
            _.from({ http: httpCollection })
              .where((_) => eq(_.http.httpId, httpIdBytes))
              .findOne(),
          );

          const searchParams = await queryCollection((_) =>
            _.from({ sp: httpSearchParamCollection }).where((_) => eq(_.sp.httpId, httpIdBytes)),
          );

          const headers = await queryCollection((_) =>
            _.from({ h: httpHeaderCollection }).where((_) => eq(_.h.httpId, httpIdBytes)),
          );

          const bodyRaw = await queryCollection((_) =>
            _.from({ br: httpBodyRawCollection }).where((_) => eq(_.br.httpId, httpIdBytes)),
          );

          const asserts = await queryCollection((_) =>
            _.from({ a: httpAssertCollection }).where((_) => eq(_.a.httpId, httpIdBytes)),
          );

          const HTTP_METHOD_NAMES: Record<number, string> = {
            0: 'UNSPECIFIED',
            1: 'GET',
            2: 'POST',
            3: 'PUT',
            4: 'PATCH',
            5: 'DELETE',
            6: 'HEAD',
            7: 'OPTIONS',
            8: 'CONNECT',
          };

          result.httpId = node.httpId;
          result.url = httpData?.url ?? '';
          result.method = HTTP_METHOD_NAMES[httpData?.method ?? 0] ?? 'UNSPECIFIED';
          result.headers = headers.map((h) => ({
            enabled: h.enabled,
            id: h.httpHeaderId ? Ulid.construct(h.httpHeaderId).toCanonical() : undefined,
            key: h.key,
            value: h.value,
          }));
          result.searchParams = searchParams.map((sp) => ({
            enabled: sp.enabled,
            id: sp.httpSearchParamId ? Ulid.construct(sp.httpSearchParamId).toCanonical() : undefined,
            key: sp.key,
            value: sp.value,
          }));
          result.body = bodyRaw.length > 0 ? bodyRaw[0]?.data : undefined;
          result.assertions = asserts.map((a) => ({
            enabled: a.enabled,
            id: a.httpAssertId ? Ulid.construct(a.httpAssertId).toCanonical() : undefined,
            value: a.value,
          }));
          break;
        }
        case 'Ai': {
          const [aiData] = await queryCollection((_) =>
            _.from({ ai: aiCollection })
              .where((_) => eq(_.ai.nodeId, nodeIdBytes))
              .findOne(),
          );
          result.prompt = aiData?.prompt ?? '';
          result.maxIterations = aiData?.maxIterations ?? 5;
          break;
        }
        case 'JavaScript': {
          const [jsData] = await queryCollection((_) =>
            _.from({ js: jsCollection })
              .where((_) => eq(_.js.nodeId, nodeIdBytes))
              .findOne(),
          );
          result.code = jsData?.code ?? '';
          break;
        }
      }

      // Query execution data fresh from collection (not cached flowContext)
      const allExecs = await queryCollection((_) => _.from({ exec: executionCollection }));
      const nodeExecs = allExecs
        .filter((e) => e.nodeId != null && Ulid.construct(e.nodeId).toCanonical() === nodeIdStr)
        .sort((a, b) => {
          if (!a.completedAt && !b.completedAt) return 0;
          if (!a.completedAt) return 1;
          if (!b.completedAt) return -1;
          return Number(b.completedAt - a.completedAt);
        });

      if (nodeExecs.length > 0) {
        const latest = nodeExecs[0]!;
        result.execution = {
          completedAt:
            latest.completedAt instanceof Date
              ? latest.completedAt.toISOString()
              : latest.completedAt
                ? String(latest.completedAt)
                : undefined,
          error: latest.error ?? undefined,
          state: FLOW_ITEM_STATE_NAMES[latest.state] ?? 'Unknown',
        };

        if (includeOutput) {
          const MAX_OUTPUT_LENGTH = 10000;
          const truncateData = (data: unknown): unknown => {
            if (data == null) return data;
            const str = typeof data === 'string' ? data : JSON.stringify(data);
            if (str.length <= MAX_OUTPUT_LENGTH) return data;
            return {
              _originalLength: str.length,
              _truncated: true,
              preview: str.slice(0, MAX_OUTPUT_LENGTH) + '...',
            };
          };
          (result.execution as Record<string, unknown>).input = truncateData(latest.input);
          (result.execution as Record<string, unknown>).output = truncateData(latest.output);
        }
      }

      return result;
    }

    case 'updateNode': {
      const nodeIdStr = args.nodeId as string;
      const node = flowContext.nodes.find((n) => n.id === nodeIdStr);
      if (!node) throw new Error(`Node not found: ${nodeIdStr}`);

      const nodeIdBytes = parseUlid(nodeIdStr);
      const updatedFields: string[] = [];

      // --- Base fields (any node type) ---
      if (args.name !== undefined) {
        nodeCollection.utils.update({
          name: normalizeNodeName(args.name as string),
          nodeId: nodeIdBytes,
        });
        updatedFields.push('name');
      }

      // --- Type-specific fields ---
      switch (node.kind) {
        case 'Ai': {
          const aiUpdates: Record<string, unknown> = { nodeId: nodeIdBytes };
          let hasAiUpdates = false;

          if (args.prompt !== undefined) {
            aiUpdates.prompt = args.prompt;
            hasAiUpdates = true;
            updatedFields.push('prompt');
          }
          if (args.maxIterations !== undefined) {
            const maxIterations = args.maxIterations as number;
            if (!Number.isInteger(maxIterations) || maxIterations <= 0) {
              throw new Error(`maxIterations must be a positive integer, got: ${maxIterations}`);
            }
            aiUpdates.maxIterations = maxIterations;
            hasAiUpdates = true;
            updatedFields.push('maxIterations');
          }
          if (hasAiUpdates) aiCollection.utils.update(aiUpdates);
          break;
        }
        case 'Condition': {
          if (args.condition !== undefined) {
            conditionCollection.utils.update({
              condition: normalizeConditionSyntax(args.condition as string),
              nodeId: nodeIdBytes,
            });
            updatedFields.push('condition');
          }
          break;
        }
        case 'For': {
          const forUpdates: Record<string, unknown> = { nodeId: nodeIdBytes };
          let hasForUpdates = false;

          if (args.iterations !== undefined) {
            const iterations = args.iterations as number;
            if (!Number.isInteger(iterations) || iterations <= 0) {
              throw new Error(`iterations must be a positive integer, got: ${iterations}`);
            }
            forUpdates.iterations = iterations;
            hasForUpdates = true;
            updatedFields.push('iterations');
          }
          if (args.condition !== undefined) {
            forUpdates.condition = normalizeConditionSyntax(args.condition as string);
            hasForUpdates = true;
            updatedFields.push('condition');
          }
          if (args.errorHandling !== undefined) {
            forUpdates.errorHandling = args.errorHandling === 'break' ? 1 : 0;
            hasForUpdates = true;
            updatedFields.push('errorHandling');
          }
          if (hasForUpdates) forCollection.utils.update(forUpdates);
          break;
        }
        case 'ForEach': {
          const feUpdates: Record<string, unknown> = { nodeId: nodeIdBytes };
          let hasFeUpdates = false;

          if (args.path !== undefined) {
            feUpdates.path = normalizeConditionSyntax(args.path as string);
            hasFeUpdates = true;
            updatedFields.push('path');
          }
          if (args.condition !== undefined) {
            feUpdates.condition = normalizeConditionSyntax(args.condition as string);
            hasFeUpdates = true;
            updatedFields.push('condition');
          }
          if (args.errorHandling !== undefined) {
            feUpdates.errorHandling = args.errorHandling === 'break' ? 1 : 0;
            hasFeUpdates = true;
            updatedFields.push('errorHandling');
          }
          if (hasFeUpdates) forEachCollection.utils.update(feUpdates);
          break;
        }
        case 'JavaScript': {
          if (args.code !== undefined) {
            jsCollection.utils.update({
              code: `export default function(ctx) {\n  ${normalizeJsCodeReferences(args.code as string)}\n}`,
              nodeId: nodeIdBytes,
            });
            updatedFields.push('code');
          }
          break;
        }
        case 'HTTP': {
          if (!node.httpId) throw new Error(`HTTP node "${node.name}" has no associated HTTP request`);
          const httpIdBytes = parseUlid(node.httpId);
          const METHODS_WITH_BODY = new Set(['PATCH', 'POST', 'PUT']);
          const HTTP_METHOD_NAMES_LOCAL: Record<number, string> = {
            0: 'UNSPECIFIED',
            1: 'GET',
            2: 'POST',
            3: 'PUT',
            4: 'PATCH',
            5: 'DELETE',
            6: 'HEAD',
            7: 'OPTIONS',
            8: 'CONNECT',
          };

          const [httpData] = await queryCollection((_) =>
            _.from({ http: httpCollection })
              .where((_) => eq(_.http.httpId, httpIdBytes))
              .findOne(),
          );

          const clearHttpBody = async () => {
            httpCollection.utils.update({ bodyKind: HttpBodyKind.UNSPECIFIED, httpId: httpIdBytes });
            const existingBody = await queryCollection((_) =>
              _.from({ br: httpBodyRawCollection }).where((_) => eq(_.br.httpId, httpIdBytes)),
            );
            if (existingBody.length > 0) {
              httpBodyRawCollection.utils.update({ data: '', httpId: httpIdBytes });
            }
          };

          // Update method/url
          const httpUpdates: Record<string, unknown> = { httpId: httpIdBytes };
          let hasHttpUpdates = false;
          const currentMethod = HTTP_METHOD_NAMES_LOCAL[httpData?.method ?? 0] ?? 'UNSPECIFIED';
          let effectiveMethod = currentMethod;

          if (args.method !== undefined) {
            const methodStr = (args.method as string).toUpperCase();
            const method = HTTP_METHOD_MAP[methodStr];
            if (method === undefined) {
              throw new Error(
                `Invalid HTTP method: "${args.method}". Valid: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS`,
              );
            }
            effectiveMethod = methodStr;
            httpUpdates.method = method;
            hasHttpUpdates = true;
            updatedFields.push('method');
          }

          if (args.url !== undefined) {
            httpUpdates.url = args.url;
            hasHttpUpdates = true;
            updatedFields.push('url');
          }

          if (hasHttpUpdates) {
            httpCollection.utils.update(httpUpdates);
          }

          // Replace headers if provided
          if (args.headers !== undefined) {
            const existingHeaders = await queryCollection((_) =>
              _.from({ h: httpHeaderCollection }).where((_) => eq(_.h.httpId, httpIdBytes)),
            );
            for (const h of existingHeaders) {
              if (h.httpHeaderId) httpHeaderCollection.utils.delete({ httpHeaderId: h.httpHeaderId });
            }
            const newHeaders = args.headers as {
              description?: string;
              enabled?: boolean;
              key: string;
              value?: string;
            }[];
            for (let i = 0; i < newHeaders.length; i++) {
              const h = newHeaders[i]!;
              await httpHeaderCollection.utils.insert({
                description: h.description ?? '',
                enabled: h.enabled ?? true,
                httpHeaderId: Ulid.generate().bytes,
                httpId: httpIdBytes,
                key: h.key,
                order: i,
                value: h.value ?? '',
              });
            }
            updatedFields.push('headers');
          }

          // Replace search params if provided
          if (args.searchParams !== undefined) {
            const existingParams = await queryCollection((_) =>
              _.from({ sp: httpSearchParamCollection }).where((_) => eq(_.sp.httpId, httpIdBytes)),
            );
            for (const sp of existingParams) {
              if (sp.httpSearchParamId)
                httpSearchParamCollection.utils.delete({ httpSearchParamId: sp.httpSearchParamId });
            }
            const newParams = args.searchParams as {
              description?: string;
              enabled?: boolean;
              key: string;
              value?: string;
            }[];
            for (let i = 0; i < newParams.length; i++) {
              const sp = newParams[i]!;
              await httpSearchParamCollection.utils.insert({
                description: sp.description ?? '',
                enabled: sp.enabled ?? true,
                httpId: httpIdBytes,
                httpSearchParamId: Ulid.generate().bytes,
                key: sp.key,
                order: i,
                value: sp.value ?? '',
              });
            }
            updatedFields.push('searchParams');
          }

          // Method-body guard: validate body is only set for methods that support it
          if (args.body !== undefined && args.body !== null) {
            if (!METHODS_WITH_BODY.has(effectiveMethod)) {
              throw new Error(
                `Cannot set body for ${effectiveMethod} requests. ` +
                  'Only POST, PUT, and PATCH methods support a request body. ' +
                  'Either change the method first or remove the body.',
              );
            }
          }

          // If method is changed to a no-body method and body wasn't explicitly provided,
          // clear any existing body to keep method/body state consistent.
          if (args.method !== undefined && args.body === undefined && !METHODS_WITH_BODY.has(effectiveMethod)) {
            await clearHttpBody();
            updatedFields.push('body');
          }

          // Set or clear body
          if (args.body !== undefined) {
            const body = args.body as null | string;
            if (body === null) {
              await clearHttpBody();
            } else {
              httpCollection.utils.update({ bodyKind: HttpBodyKind.RAW, httpId: httpIdBytes });
              const existingBody = await queryCollection((_) =>
                _.from({ br: httpBodyRawCollection }).where((_) => eq(_.br.httpId, httpIdBytes)),
              );
              if (existingBody.length > 0) {
                httpBodyRawCollection.utils.update({ data: body, httpId: httpIdBytes });
              } else {
                await httpBodyRawCollection.utils.insert({ data: body, httpId: httpIdBytes });
              }
            }
            updatedFields.push('body');
          }

          // Replace assertions if provided
          if (args.assertions !== undefined) {
            const existingAsserts = await queryCollection((_) =>
              _.from({ a: httpAssertCollection }).where((_) => eq(_.a.httpId, httpIdBytes)),
            );
            for (const a of existingAsserts) {
              if (a.httpAssertId) httpAssertCollection.utils.delete({ httpAssertId: a.httpAssertId });
            }
            const newAsserts = args.assertions as { enabled?: boolean; value: string }[];
            for (let i = 0; i < newAsserts.length; i++) {
              const a = newAsserts[i]!;
              await httpAssertCollection.utils.insert({
                enabled: a.enabled ?? true,
                httpAssertId: Ulid.generate().bytes,
                httpId: httpIdBytes,
                order: i,
                value: a.value,
              });
            }
            updatedFields.push('assertions');
          }
          break;
        }
      }

      if (updatedFields.length === 0) {
        return { message: `No applicable fields provided for ${node.kind} node "${node.name}"`, success: false };
      }

      return { success: true, updatedFields };
    }

    case 'patchHttpNode': {
      const nodeIdStr = args.nodeId as string;
      const node = flowContext.nodes.find((n) => n.id === nodeIdStr);
      if (!node) throw new Error(`Node not found: ${nodeIdStr}`);
      if (node.kind !== 'HTTP') throw new Error(`patchHttpNode only works on HTTP nodes, got: ${node.kind}`);
      if (!node.httpId) throw new Error(`HTTP node "${node.name}" has no associated HTTP request`);

      const httpIdBytes = parseUlid(node.httpId);
      const patchedFields: string[] = [];
      const warnings: string[] = [];

      // --- Remove headers ---
      const removeHeaderIds = args.removeHeaderIds as string[] | undefined;
      const addHeaders = args.addHeaders as
        | { description?: string; enabled?: boolean; key: string; value?: string }[]
        | undefined;

      if (removeHeaderIds?.length) {
        const existingHeaders = await queryCollection((_) =>
          _.from({ h: httpHeaderCollection }).where((_) => eq(_.h.httpId, httpIdBytes)),
        );
        const existingHeaderIds = new Set(
          existingHeaders
            .filter((h) => h.httpHeaderId != null)
            .map((h) => Ulid.construct(h.httpHeaderId).toCanonical()),
        );
        let removedCount = 0;
        for (const id of removeHeaderIds) {
          if (!existingHeaderIds.has(id)) continue;
          httpHeaderCollection.utils.delete({ httpHeaderId: parseUlid(id) });
          removedCount++;
        }
        if (removedCount > 0) {
          patchedFields.push(`removedHeaders(${removedCount})`);
        }
        const skippedCount = removeHeaderIds.length - removedCount;
        if (skippedCount > 0) {
          warnings.push(`Skipped ${skippedCount} header ID(s) not belonging to this HTTP node.`);
        }
      }

      // --- Add headers ---
      if (addHeaders?.length) {
        const existingHeaders = await queryCollection((_) =>
          _.from({ h: httpHeaderCollection }).where((_) => eq(_.h.httpId, httpIdBytes)),
        );
        const maxOrder = existingHeaders.reduce((max, h) => Math.max(max, h.order ?? -1), -1);
        let nextOrder = maxOrder + 1;
        for (const h of addHeaders) {
          await httpHeaderCollection.utils.insert({
            description: h.description ?? '',
            enabled: h.enabled ?? true,
            httpHeaderId: Ulid.generate().bytes,
            httpId: httpIdBytes,
            key: h.key,
            order: nextOrder++,
            value: h.value ?? '',
          });
        }
        patchedFields.push(`addedHeaders(${addHeaders.length})`);
      }

      // --- Remove search params ---
      const removeSearchParamIds = args.removeSearchParamIds as string[] | undefined;
      const addSearchParams = args.addSearchParams as
        | { description?: string; enabled?: boolean; key: string; value?: string }[]
        | undefined;

      if (removeSearchParamIds?.length) {
        const existingSearchParams = await queryCollection((_) =>
          _.from({ sp: httpSearchParamCollection }).where((_) => eq(_.sp.httpId, httpIdBytes)),
        );
        const existingSearchParamIds = new Set(
          existingSearchParams
            .filter((sp) => sp.httpSearchParamId != null)
            .map((sp) => Ulid.construct(sp.httpSearchParamId).toCanonical()),
        );
        let removedCount = 0;
        for (const id of removeSearchParamIds) {
          if (!existingSearchParamIds.has(id)) continue;
          httpSearchParamCollection.utils.delete({ httpSearchParamId: parseUlid(id) });
          removedCount++;
        }
        if (removedCount > 0) {
          patchedFields.push(`removedSearchParams(${removedCount})`);
        }
        const skippedCount = removeSearchParamIds.length - removedCount;
        if (skippedCount > 0) {
          warnings.push(`Skipped ${skippedCount} query param ID(s) not belonging to this HTTP node.`);
        }
      }

      // --- Add search params ---
      if (addSearchParams?.length) {
        const existingSearchParams = await queryCollection((_) =>
          _.from({ sp: httpSearchParamCollection }).where((_) => eq(_.sp.httpId, httpIdBytes)),
        );
        const maxOrder = existingSearchParams.reduce((max, sp) => Math.max(max, sp.order ?? -1), -1);
        let nextOrder = maxOrder + 1;
        for (const sp of addSearchParams) {
          await httpSearchParamCollection.utils.insert({
            description: sp.description ?? '',
            enabled: sp.enabled ?? true,
            httpId: httpIdBytes,
            httpSearchParamId: Ulid.generate().bytes,
            key: sp.key,
            order: nextOrder++,
            value: sp.value ?? '',
          });
        }
        patchedFields.push(`addedSearchParams(${addSearchParams.length})`);
      }

      // --- Remove assertions ---
      const removeAssertionIds = args.removeAssertionIds as string[] | undefined;
      const addAssertions = args.addAssertions as { enabled?: boolean; value: string }[] | undefined;

      if (removeAssertionIds?.length) {
        const existingAssertions = await queryCollection((_) =>
          _.from({ a: httpAssertCollection }).where((_) => eq(_.a.httpId, httpIdBytes)),
        );
        const existingAssertionIds = new Set(
          existingAssertions
            .filter((a) => a.httpAssertId != null)
            .map((a) => Ulid.construct(a.httpAssertId).toCanonical()),
        );
        let removedCount = 0;
        for (const id of removeAssertionIds) {
          if (!existingAssertionIds.has(id)) continue;
          httpAssertCollection.utils.delete({ httpAssertId: parseUlid(id) });
          removedCount++;
        }
        if (removedCount > 0) {
          patchedFields.push(`removedAssertions(${removedCount})`);
        }
        const skippedCount = removeAssertionIds.length - removedCount;
        if (skippedCount > 0) {
          warnings.push(`Skipped ${skippedCount} assertion ID(s) not belonging to this HTTP node.`);
        }
      }

      // --- Add assertions ---
      if (addAssertions?.length) {
        const existingAssertions = await queryCollection((_) =>
          _.from({ a: httpAssertCollection }).where((_) => eq(_.a.httpId, httpIdBytes)),
        );
        const maxOrder = existingAssertions.reduce((max, a) => Math.max(max, a.order ?? -1), -1);
        let nextOrder = maxOrder + 1;
        for (const a of addAssertions) {
          await httpAssertCollection.utils.insert({
            enabled: a.enabled ?? true,
            httpAssertId: Ulid.generate().bytes,
            httpId: httpIdBytes,
            order: nextOrder++,
            value: a.value,
          });
        }
        patchedFields.push(`addedAssertions(${addAssertions.length})`);
      }

      if (patchedFields.length === 0) {
        return { message: 'No patch operations provided', success: false };
      }

      return { patchedFields, success: true, warnings: warnings.length > 0 ? warnings : undefined };
    }

    case 'updateVariable': {
      const flowVariableId = parseUlid(args.flowVariableId as string);
      const updates: Record<string, unknown> = { flowVariableId };

      if (args.key !== undefined) updates.key = args.key;
      if (args.value !== undefined) updates.value = args.value;
      if (args.enabled !== undefined) updates.enabled = args.enabled;
      if (args.description !== undefined) updates.description = args.description;
      if (args.order !== undefined) updates.order = args.order;

      variableCollection.utils.update(updates);
      return { success: true };
    }

    default:
      throw new Error(`Unknown tool: ${name}`);
  }
};

export type { Collections, ToolExecutorContext };
