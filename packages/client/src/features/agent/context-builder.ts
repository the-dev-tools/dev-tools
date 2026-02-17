/* eslint-disable @typescript-eslint/no-unnecessary-condition */
import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { FlowItemState, NodeKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { HttpMethod } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import {
  EdgeCollectionSchema,
  FlowVariableCollectionSchema,
  NodeCollectionSchema,
  NodeExecutionCollectionSchema,
  NodeHttpCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { HttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { useApiCollection } from '~/shared/api';
import { queryCollection } from '~/shared/lib';
import type { EdgeInfo, FlowContextData, NodeExecutionInfo, NodeInfo, VariableInfo } from './types';

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

const HTTP_METHOD_NAMES: Record<number, string> = {
  [HttpMethod.DELETE]: 'DELETE',
  [HttpMethod.GET]: 'GET',
  [HttpMethod.HEAD]: 'HEAD',
  [HttpMethod.OPTIONS]: 'OPTIONS',
  [HttpMethod.PATCH]: 'PATCH',
  [HttpMethod.POST]: 'POST',
  [HttpMethod.PUT]: 'PUT',
  [HttpMethod.UNSPECIFIED]: 'UNSPECIFIED',
};

const escapeXml = (s: string): string =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&apos;');

export const useFlowContext = (flowId: Uint8Array): FlowContextData => {
  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const edgeCollection = useApiCollection(EdgeCollectionSchema);
  const variableCollection = useApiCollection(FlowVariableCollectionSchema);
  const executionCollection = useApiCollection(NodeExecutionCollectionSchema);
  const nodeHttpCollection = useApiCollection(NodeHttpCollectionSchema);
  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { data: nodesData } = useLiveQuery(
    (_) => _.from({ node: nodeCollection }).where((_) => eq(_.node.flowId, flowId)),
    [nodeCollection, flowId],
  );

  const { data: edgesData } = useLiveQuery(
    (_) => _.from({ edge: edgeCollection }).where((_) => eq(_.edge.flowId, flowId)),
    [edgeCollection, flowId],
  );

  const { data: variablesData } = useLiveQuery(
    (_) => _.from({ variable: variableCollection }).where((_) => eq(_.variable.flowId, flowId)),
    [variableCollection, flowId],
  );

  // Get all node IDs from the current flow as a Set for efficient lookup
  const nodeIdSet = new Set(
    (nodesData ?? []).filter((n) => n.nodeId != null).map((n) => Ulid.construct(n.nodeId).toCanonical()),
  );

  // Get all executions - we'll filter in memory by node IDs
  const { data: allExecutionsData } = useLiveQuery((_) => _.from({ exec: executionCollection }), [executionCollection]);

  // Filter executions to only those belonging to nodes in this flow
  const executionsData = (allExecutionsData ?? []).filter(
    (e) => e.nodeId != null && nodeIdSet.has(Ulid.construct(e.nodeId).toCanonical()),
  );

  // Get all nodeHttp mappings for HTTP nodes
  const { data: nodeHttpData } = useLiveQuery((_) => _.from({ nodeHttp: nodeHttpCollection }), [nodeHttpCollection]);

  // Build a map of nodeId -> httpId for quick lookup
  const nodeHttpMap = new Map(
    (nodeHttpData ?? [])
      .filter((nh) => nh.nodeId != null && nh.httpId != null)
      .map((nh) => [Ulid.construct(nh.nodeId).toCanonical(), Ulid.construct(nh.httpId).toCanonical()]),
  );

  // Get all HTTP requests to fetch their methods
  const { data: httpData } = useLiveQuery((_) => _.from({ http: httpCollection }), [httpCollection]);

  // Build a map of httpId -> method for quick lookup
  const httpMethodMap = new Map(
    (httpData ?? [])
      .filter((h) => h.httpId != null)
      .map((h) => [Ulid.construct(h.httpId).toCanonical(), HTTP_METHOD_NAMES[h.method] ?? 'UNSPECIFIED']),
  );

  const nodes: NodeInfo[] = (nodesData ?? [])
    .filter((n) => n.nodeId != null)
    .map((n) => {
      const nodeIdStr = Ulid.construct(n.nodeId).toCanonical();
      const httpId = n.kind === NodeKind.HTTP ? nodeHttpMap.get(nodeIdStr) : undefined;
      const httpMethod = httpId ? httpMethodMap.get(httpId) : undefined;
      return {
        httpId,
        httpMethod,
        id: nodeIdStr,
        info: n.info ?? undefined,
        kind: NODE_KIND_NAMES[n.kind] ?? 'Unknown',
        name: n.name,
        position: { x: n.position?.x ?? 0, y: n.position?.y ?? 0 },
        state: FLOW_ITEM_STATE_NAMES[n.state] ?? 'Idle',
      };
    });

  const edges: EdgeInfo[] = (edgesData ?? [])
    .filter((e) => e.edgeId != null)
    .map((e) => ({
      id: Ulid.construct(e.edgeId).toCanonical(),
      sourceHandle: e.sourceHandle !== undefined ? String(e.sourceHandle) : undefined,
      sourceId: Ulid.construct(e.sourceId).toCanonical(),
      targetId: Ulid.construct(e.targetId).toCanonical(),
    }));

  const variables: VariableInfo[] = (variablesData ?? [])
    .filter((v) => v.flowVariableId != null)
    .map((v) => ({
      enabled: v.enabled,
      id: Ulid.construct(v.flowVariableId).toCanonical(),
      key: v.key,
      value: v.value,
    }));

  // Only keep the most recent execution per node to limit context size
  // Input/output are stored but will be truncated when accessed via getNodeOutput
  const executionsByNode = new Map<string, (typeof executionsData)[0]>();
  for (const e of executionsData ?? []) {
    if (e.nodeExecutionId == null) continue;
    const nodeIdStr = Ulid.construct(e.nodeId).toCanonical();
    const existing = executionsByNode.get(nodeIdStr);
    if (!existing || (e.completedAt && (!existing.completedAt || e.completedAt > existing.completedAt))) {
      executionsByNode.set(nodeIdStr, e);
    }
  }

  const executions: NodeExecutionInfo[] = Array.from(executionsByNode.values()).map((e) => ({
    completedAt: e.completedAt instanceof Date ? e.completedAt.toISOString() : e.completedAt,
    error: e.error ?? undefined,
    id: Ulid.construct(e.nodeExecutionId).toCanonical(),
    input: e.input ?? undefined,
    name: e.name,
    nodeId: Ulid.construct(e.nodeId).toCanonical(),
    output: e.output ?? undefined,
    state: FLOW_ITEM_STATE_NAMES[e.state] ?? 'Idle',
  }));

  return {
    edges,
    executions,
    flowId: Ulid.construct(flowId).toCanonical(),
    nodes,
    variables,
  };
};

interface FlowCollections {
  edgeCollection: ReturnType<typeof useApiCollection<typeof EdgeCollectionSchema>>;
  executionCollection: ReturnType<typeof useApiCollection<typeof NodeExecutionCollectionSchema>>;
  httpCollection: ReturnType<typeof useApiCollection<typeof HttpCollectionSchema>>;
  nodeCollection: ReturnType<typeof useApiCollection<typeof NodeCollectionSchema>>;
  nodeHttpCollection: ReturnType<typeof useApiCollection<typeof NodeHttpCollectionSchema>>;
  variableCollection: ReturnType<typeof useApiCollection<typeof FlowVariableCollectionSchema>>;
}

/**
 * Async version of useFlowContext that queries collections directly.
 * Use this outside React's render cycle (e.g. in the agent tool loop)
 * to get a fresh snapshot of flow data after mutations.
 */
export const refreshFlowContext = async (
  flowId: Uint8Array,
  collections: FlowCollections,
): Promise<FlowContextData> => {
  const {
    edgeCollection,
    executionCollection,
    httpCollection,
    nodeCollection,
    nodeHttpCollection,
    variableCollection,
  } = collections;

  const nodesData = await queryCollection((_) =>
    _.from({ node: nodeCollection }).where((_) => eq(_.node.flowId, flowId)),
  );

  const edgesData = await queryCollection((_) =>
    _.from({ edge: edgeCollection }).where((_) => eq(_.edge.flowId, flowId)),
  );

  const variablesData = await queryCollection((_) =>
    _.from({ variable: variableCollection }).where((_) => eq(_.variable.flowId, flowId)),
  );

  const nodeIdSet = new Set(
    nodesData.filter((n) => n.nodeId != null).map((n) => Ulid.construct(n.nodeId).toCanonical()),
  );

  const allExecutionsData = await queryCollection((_) => _.from({ exec: executionCollection }));
  const executionsData = allExecutionsData.filter(
    (e) => e.nodeId != null && nodeIdSet.has(Ulid.construct(e.nodeId).toCanonical()),
  );

  const nodeHttpData = await queryCollection((_) => _.from({ nodeHttp: nodeHttpCollection }));
  const nodeHttpMap = new Map(
    nodeHttpData
      .filter((nh) => nh.nodeId != null && nh.httpId != null)
      .map((nh) => [Ulid.construct(nh.nodeId).toCanonical(), Ulid.construct(nh.httpId).toCanonical()]),
  );

  const httpData = await queryCollection((_) => _.from({ http: httpCollection }));
  const httpMethodMap = new Map(
    httpData
      .filter((h) => h.httpId != null)
      .map((h) => [Ulid.construct(h.httpId).toCanonical(), HTTP_METHOD_NAMES[h.method] ?? 'UNSPECIFIED']),
  );

  const nodes: NodeInfo[] = nodesData
    .filter((n) => n.nodeId != null)
    .map((n) => {
      const nodeIdStr = Ulid.construct(n.nodeId).toCanonical();
      const httpId = n.kind === NodeKind.HTTP ? nodeHttpMap.get(nodeIdStr) : undefined;
      const httpMethod = httpId ? httpMethodMap.get(httpId) : undefined;
      return {
        httpId,
        httpMethod,
        id: nodeIdStr,
        info: n.info ?? undefined,
        kind: NODE_KIND_NAMES[n.kind] ?? 'Unknown',
        name: n.name,
        position: { x: n.position?.x ?? 0, y: n.position?.y ?? 0 },
        state: FLOW_ITEM_STATE_NAMES[n.state] ?? 'Idle',
      };
    });

  const edges: EdgeInfo[] = edgesData
    .filter((e) => e.edgeId != null)
    .map((e) => ({
      id: Ulid.construct(e.edgeId).toCanonical(),
      sourceHandle: e.sourceHandle !== undefined ? String(e.sourceHandle) : undefined,
      sourceId: Ulid.construct(e.sourceId).toCanonical(),
      targetId: Ulid.construct(e.targetId).toCanonical(),
    }));

  const variables: VariableInfo[] = variablesData
    .filter((v) => v.flowVariableId != null)
    .map((v) => ({
      enabled: v.enabled,
      id: Ulid.construct(v.flowVariableId).toCanonical(),
      key: v.key,
      value: v.value,
    }));

  const executionsByNode = new Map<string, (typeof executionsData)[0]>();
  for (const e of executionsData) {
    if (e.nodeExecutionId == null) continue;
    const nodeIdStr = Ulid.construct(e.nodeId).toCanonical();
    const existing = executionsByNode.get(nodeIdStr);
    if (!existing || (e.completedAt && (!existing.completedAt || e.completedAt > existing.completedAt))) {
      executionsByNode.set(nodeIdStr, e);
    }
  }

  const executions: NodeExecutionInfo[] = Array.from(executionsByNode.values()).map((e) => ({
    completedAt: e.completedAt instanceof Date ? e.completedAt.toISOString() : e.completedAt,
    error: e.error ?? undefined,
    id: Ulid.construct(e.nodeExecutionId).toCanonical(),
    input: e.input ?? undefined,
    name: e.name,
    nodeId: Ulid.construct(e.nodeId).toCanonical(),
    output: e.output ?? undefined,
    state: FLOW_ITEM_STATE_NAMES[e.state] ?? 'Idle',
  }));

  return {
    edges,
    executions,
    flowId: Ulid.construct(flowId).toCanonical(),
    nodes,
    variables,
  };
};

/**
 * Detect orphan nodes that are not reachable from ManualStart via BFS.
 * Reusable by both the system prompt builder and the post-execution validation loop.
 */
export const detectOrphanNodes = (
  nodes: Pick<NodeInfo, 'id' | 'kind' | 'name'>[],
  edges: Pick<EdgeInfo, 'sourceId' | 'targetId'>[],
): Pick<NodeInfo, 'id' | 'kind' | 'name'>[] => {
  const startNode = nodes.find((n) => n.kind === 'ManualStart');
  if (!startNode) return [];

  // Build outgoing edge map
  const outgoing = new Map<string, string[]>();
  for (const e of edges) {
    const list = outgoing.get(e.sourceId) ?? [];
    list.push(e.targetId);
    outgoing.set(e.sourceId, list);
  }

  // BFS to find reachable nodes
  const reachable = new Set<string>();
  const queue = [startNode.id];
  while (queue.length > 0) {
    const nodeId = queue.shift()!;
    if (reachable.has(nodeId)) continue;
    reachable.add(nodeId);
    queue.push(...(outgoing.get(nodeId) ?? []));
  }

  return nodes.filter((n) => n.kind !== 'ManualStart' && !reachable.has(n.id));
};

/**
 * Detect dead-end nodes: reachable from Start but have no outgoing edges.
 * Only flags as problematic when there are many dead-ends AND the flow has
 * deeper interior nodes — indicating the model forgot fan-in connections.
 */
export const detectDeadEndNodes = (
  nodes: Pick<NodeInfo, 'id' | 'kind' | 'name'>[],
  edges: Pick<EdgeInfo, 'sourceId' | 'targetId'>[],
): Pick<NodeInfo, 'id' | 'kind' | 'name'>[] => {
  const hasOutgoing = new Set(edges.map((e) => e.sourceId));
  const hasIncoming = new Set(edges.map((e) => e.targetId));

  // Dead-ends: non-start nodes with incoming edges but no outgoing edges
  const deadEnds = nodes.filter((n) => n.kind !== 'ManualStart' && hasIncoming.has(n.id) && !hasOutgoing.has(n.id));

  // Interior nodes: non-start nodes that DO have outgoing edges (flow has depth)
  const interiorNodes = nodes.filter((n) => n.kind !== 'ManualStart' && hasOutgoing.has(n.id));

  // Only flag when: many dead-ends AND flow has interior depth
  if (deadEnds.length > 3 && interiorNodes.length > 0) {
    return deadEnds;
  }

  return [];
};

const buildXmlFlowBlock = (context: FlowContextData): string => {
  // 1. Build outgoing edge map: sourceId -> EdgeInfo[]
  const outgoingEdges = new Map<string, EdgeInfo[]>();
  for (const e of context.edges) {
    const list = outgoingEdges.get(e.sourceId) ?? [];
    list.push(e);
    outgoingEdges.set(e.sourceId, list);
  }

  // 2. Build node-name lookup
  const nodeNameMap = new Map<string, string>();
  for (const n of context.nodes) {
    nodeNameMap.set(n.id, n.name);
  }

  // 3. Compute orphan set
  const orphanNodes = detectOrphanNodes(context.nodes, context.edges);
  const orphanSet = new Set(orphanNodes.map((n) => n.id));

  // 4. Compute endpoint set (sequential nodes with no outgoing edges)
  const endpointSet = new Set(
    context.nodes
      .filter((n) => ['HTTP', 'JavaScript', 'ManualStart'].includes(n.kind) && !outgoingEdges.has(n.id))
      .map((n) => n.id),
  );

  // 5. Compute selected set
  const selectedSet = new Set(context.selectedNodeIds ?? []);

  // 6. Build execution error map: nodeId -> error string
  const errorMap = new Map<string, string>();
  for (const exec of context.executions) {
    if (exec.state === 'Failure' && exec.error) {
      errorMap.set(exec.nodeId, exec.error);
    }
  }

  // 7. Build XML nodes
  const lines: string[] = ['<flow>'];

  for (const node of context.nodes) {
    const attrs: string[] = [
      `id="${escapeXml(node.id)}"`,
      `name="${escapeXml(node.name)}"`,
      `type="${escapeXml(node.kind)}"`,
    ];

    if (node.httpMethod) attrs.push(`method="${escapeXml(node.httpMethod)}"`);
    if (node.state !== 'Idle') attrs.push(`state="${escapeXml(node.state)}"`);

    // Prefer execution error over node.info
    const errorDetail = errorMap.get(node.id) ?? node.info;
    if (errorDetail) attrs.push(`error="${escapeXml(errorDetail)}"`);

    if (selectedSet.has(node.id)) attrs.push('selected="true"');
    if (orphanSet.has(node.id)) attrs.push('orphan="true"');
    if (endpointSet.has(node.id)) attrs.push('endpoint="true"');

    const edges = outgoingEdges.get(node.id);
    if (!edges || edges.length === 0) {
      lines.push(`  <node ${attrs.join(' ')}/>`);
    } else {
      lines.push(`  <node ${attrs.join(' ')}>`);
      for (const edge of edges) {
        const targetName = nodeNameMap.get(edge.targetId) ?? edge.targetId;
        const edgeAttrs = [`id="${escapeXml(edge.id)}"`, `target="${escapeXml(targetName)}"`];
        if (edge.sourceHandle) edgeAttrs.push(`handle="${escapeXml(edge.sourceHandle)}"`);
        lines.push(`    <edge ${edgeAttrs.join(' ')}/>`);
      }
      lines.push('  </node>');
    }
  }

  // 8. Variables block (only enabled, skip if empty)
  const enabledVars = context.variables.filter((v) => v.enabled);
  if (enabledVars.length > 0) {
    lines.push('  <variables>');
    for (const v of enabledVars) {
      lines.push(`    <var id="${escapeXml(v.id)}" key="${escapeXml(v.key)}" value="${escapeXml(v.value)}"/>`);
    }
    lines.push('  </variables>');
  }

  lines.push('</flow>');
  return lines.join('\n');
};

const buildXmlCompactSummary = (context: FlowContextData): string => {
  const orphans = detectOrphanNodes(context.nodes, context.edges);

  // Find endpoint nodes
  const outgoing = new Set(context.edges.map((e) => e.sourceId));
  const endpoints = context.nodes.filter(
    (n) => ['HTTP', 'JavaScript', 'ManualStart'].includes(n.kind) && !outgoing.has(n.id),
  );

  const lines: string[] = [`<flow-update nodes="${context.nodes.length}" edges="${context.edges.length}">`];

  for (const ep of endpoints) {
    lines.push(`  <endpoint id="${escapeXml(ep.id)}" name="${escapeXml(ep.name)}"/>`);
  }

  for (const o of orphans) {
    lines.push(`  <orphan id="${escapeXml(o.id)}" name="${escapeXml(o.name)}"/>`);
  }

  if (endpoints.length > 5) {
    lines.push(
      `  <!-- WARNING: ${endpoints.length} dead-end nodes — ensure all parallel branches fan-in to their downstream node using connectChain -->`,
    );
  }

  lines.push('</flow-update>');
  return lines.join('\n');
};

export const buildXmlValidationMessage = (
  orphans: Pick<NodeInfo, 'id' | 'kind' | 'name'>[],
  deadEnds: Pick<NodeInfo, 'id' | 'kind' | 'name'>[],
): string => {
  if (orphans.length > 0) {
    const orphanElements = orphans
      .map((n) => `  <orphan id="${escapeXml(n.id)}" name="${escapeXml(n.name)}" type="${escapeXml(n.kind)}"/>`)
      .join('\n');
    return `<validation status="failed">\n${orphanElements}\n</validation>\nConnect these nodes using connectChain before responding.`;
  }

  const deadEndElements = deadEnds
    .map((n) => `  <dead-end id="${escapeXml(n.id)}" name="${escapeXml(n.name)}" type="${escapeXml(n.kind)}"/>`)
    .join('\n');
  return `<validation status="warning">\n${deadEndElements}\n</validation>\nUse connectChain with nested arrays for fan-in: [["NodeA","NodeB"],"TargetNode"].`;
};

export const buildCompactStateSummary = (context: FlowContextData): string => {
  return buildXmlCompactSummary(context);
};

export const buildSystemPrompt = (context: FlowContextData): string => {
  return `You are a workflow automation assistant. You help users create and modify workflow nodes using natural language.

Current Workflow State (ID: ${context.flowId}):

${buildXmlFlowBlock(context)}

IMPORTANT RULES:
1. To find the start node, look for a node with type "ManualStart".
2. When connecting nodes, use the node IDs from the workflow XML.
3. Node outputs are stored by node name. In JS code use ctx["NodeName"]. HTTP nodes output { response: { status, body }, request }. ForEach nodes expose { item, key } during iteration. In HTTP fields use {{NodeName.response.body.field}} interpolation — see <variable-syntax>.
4. A node can connect to multiple targets for parallel execution (all branches run and complete before downstream nodes continue). To run steps sequentially, chain them: Start → A → B → C. Only create Condition nodes when "then" and "else" lead to DIFFERENT destinations — if both go to the same node, skip the Condition.
5. ALWAYS use connectChain for ALL connections — sequential, branching (auto-applies "then"), fan-out, and fan-in. Examples: ["A","B"] single, ["A","B","C"] chain, ["A",["B","C"],"D"] fan-out/fan-in, [["B","C"],"D"] fan-in only. Pass sourceHandle: "else" or "loop" for non-default branches. Use edge id attributes from \`<edge>\` elements when calling disconnectNodes.
6. Always confirm what you did after executing tools.
7. If a node has state="Failure", use inspectNode to get detailed error and config information.
8. Use inspectNode with includeOutput: true to see the input/output data of a node's most recent execution.
9. Use updateNode to modify any node's configuration — condition expressions, loop iterations/paths, JS code, HTTP settings, or node names. Provide only the fields to change. Arrays (headers, searchParams, assertions) replace the full existing set.
10. Nodes with selected="true" are currently selected on canvas — prefer operating on those nodes unless the user specifies otherwise.
11. Nodes with endpoint="true" are the last in their chain — new nodes connect there.
12. Nodes with orphan="true" are mistakes — they must be connected to the flow via connectChain.
13. Create ALL nodes first, then connect them all at once with connectChain. Do not alternate between creating and connecting.
14. For multi-phase flows, use SEPARATE connectChain calls per phase with a shared fan-in node. Example: ["Start",["GET1","GET2"],"ProcessData"] then ["ProcessData",["POST1","POST2"],"End"]. NEVER use consecutive nested arrays — split them across calls.
15. NEVER delete a node to work around an error. If a node fails or cannot be configured with available tools, explain the problem to the user and suggest what they need to do manually. Deleting user-requested nodes and replacing them with a different type is not allowed unless the user explicitly asks for it.
16. AI nodes require a connected AI Provider node that supplies the LLM model and credentials. The agent cannot create or configure AI Provider nodes — this must be done by the user on the canvas. If an AI node fails with a provider-related error, tell the user they need to add and connect an AI Provider node to it with the appropriate credentials.
17. Use patchHttpNode to add or remove individual headers, query params, or assertions without affecting the rest. Use updateNode only when you want to replace the entire set.

<variable-syntax>
All text fields in HTTP nodes (url, headers, body, query params) support {{}} interpolation.
The server resolves these at runtime — use variable references, not hardcoded values.

Syntax:
- Flow/node variable: {{BASE_URL}}, {{user_id}}
- Node output path: {{NodeName.response.body.field}}, {{NodeName.response.status}}
- Environment var: {{#env:HOME}}, {{#env:API_SECRET}}
- Functions: {{uuid()}}, {{uuid("v7")}}, {{ulid()}}, {{now()}}
- File content: {{#file:/path/to/file}}

Examples:
- URL: {{BASE_URL}}/api/users/{{Get_User.response.body.id}}
- Header: Bearer {{Auth.response.body.token}}
- Body: {"id": "{{uuid()}}", "name": "{{user_name}}"}

The <variables> block in the flow XML shows available flow variables — reference them via {{key}}.
When a value (base URL, API key) appears in multiple nodes, create a variable with createVariable and reference it.
Node names use underscores for spaces: "Get User" → Get_User in references.
</variable-syntax>`;
};
