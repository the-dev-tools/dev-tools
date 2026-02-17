/* eslint-disable @typescript-eslint/no-unnecessary-condition */
import { eq } from '@tanstack/react-db';
import { Ulid } from 'id128';
import OpenAI from 'openai';
import { useCallback, useRef, useSyncExternalStore } from 'react';
import { FlowItemState, NodeKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import {
  EdgeCollectionSchema,
  FlowCollectionSchema,
  FlowVariableCollectionSchema,
  NodeAiCollectionSchema,
  NodeCollectionSchema,
  NodeConditionCollectionSchema,
  NodeExecutionCollectionSchema,
  NodeForCollectionSchema,
  NodeForEachCollectionSchema,
  NodeHttpCollectionSchema,
  NodeJsCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { FileCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import {
  HttpAssertCollectionSchema,
  HttpBodyRawCollectionSchema,
  HttpCollectionSchema,
  HttpHeaderCollectionSchema,
  HttpSearchParamCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { useApiCollection } from '~/shared/api';
import { queryCollection } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { AgentLogger } from './agent-logger';
import {
  buildCompactStateSummary,
  buildSystemPrompt,
  buildXmlValidationMessage,
  detectDeadEndNodes,
  detectOrphanNodes,
  refreshFlowContext,
  useFlowContext,
} from './context-builder';
import { defaultHorizontalConfig, layoutNodes } from './layout';
import { type Collections, executeToolCall, type ToolExecutorContext } from './tool-executor';
import { allToolSchemas } from './tool-schemas';
import {
  type AgentChatState,
  formatToolAsOpenAI,
  type FlowContextData,
  type Message,
  type OpenAIMessage,
  type ToolCall,
  type ToolResult,
  type ToolSchema,
} from './types';

const MODEL = 'minimax/minimax-m2.5';

const createOpenRouterClient = (apiKey: string) =>
  new OpenAI({
    apiKey,
    baseURL: 'https://openrouter.ai/api/v1',
    dangerouslyAllowBrowser: true,
  });

const generateId = () => crypto.randomUUID();

/** JSON stringify with BigInt support */
const safeStringify = (value: unknown): string =>
  JSON.stringify(value, (_key: string, v: unknown) => (typeof v === 'bigint' ? v.toString() : v));

// ---------------------------------------------------------------------------
// Streaming helpers
// ---------------------------------------------------------------------------

interface StreamedMessage {
  content: null | string;
  tool_calls?: {
    function: { arguments: string; name: string };
    id: string;
    type: 'function';
  }[];
}

interface StreamMeta {
  finishReason: null | string | undefined;
  usage: unknown;
}

/**
 * Consumes an OpenAI streaming response, accumulating content and tool calls.
 * Calls `onContent` with the accumulated text after every content delta so the
 * UI can render tokens in real-time.
 */
const consumeStream = async (
  stream: AsyncIterable<OpenAI.Chat.Completions.ChatCompletionChunk>,
  onContent: (accumulated: string) => void,
): Promise<{ message: StreamedMessage; meta: StreamMeta }> => {
  let content = '';
  let hasContent = false;
  const toolCallsMap = new Map<number, { arguments: string; id: string; name: string }>();
  let finishReason: null | string | undefined = null;
  let usage: unknown = undefined;

  for await (const chunk of stream) {
    const choice = chunk.choices[0];
    if (!choice) {
      // Final chunk may carry only usage data
      if (chunk.usage) usage = chunk.usage;
      continue;
    }

    if (choice.finish_reason) finishReason = choice.finish_reason;
    if (chunk.usage) usage = chunk.usage;

    const delta = choice.delta;
    if (delta?.content) {
      content += delta.content;
      hasContent = true;
      onContent(content);
    }

    if (delta?.tool_calls) {
      for (const tc of delta.tool_calls) {
        const existing = toolCallsMap.get(tc.index);
        if (existing) {
          if (tc.function?.name) existing.name += tc.function.name;
          if (tc.function?.arguments) existing.arguments += tc.function.arguments;
        } else {
          toolCallsMap.set(tc.index, {
            arguments: tc.function?.arguments ?? '',
            id: tc.id ?? '',
            name: tc.function?.name ?? '',
          });
        }
      }
    }
  }

  const toolCalls =
    toolCallsMap.size > 0
      ? Array.from(toolCallsMap.entries())
          .sort(([a], [b]) => a - b)
          .map(([, tc]) => ({
            function: { arguments: tc.arguments, name: tc.name },
            id: tc.id,
            type: 'function' as const,
          }))
      : undefined;

  return {
    message: {
      content: hasContent ? content : null,
      tool_calls: toolCalls,
    },
    meta: { finishReason, usage },
  };
};

type NodeCollection = ReturnType<typeof useApiCollection<typeof NodeCollectionSchema>>;
type EdgeCollection = ReturnType<typeof useApiCollection<typeof EdgeCollectionSchema>>;

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

/**
 * Query fresh nodes and edges directly from collections, then apply layout.
 * This avoids stale context issues when mutations haven't propagated to React state yet.
 */
const applyLayoutToFlow = async (
  flowId: Uint8Array,
  nodeCollection: NodeCollection,
  edgeCollection: EdgeCollection,
): Promise<void> => {
  // Query fresh nodes directly from the collection
  const freshNodes = await queryCollection((_) =>
    _.from({ node: nodeCollection }).where((_) => eq(_.node.flowId, flowId)),
  );

  // Query fresh edges directly from the collection
  const freshEdges = await queryCollection((_) =>
    _.from({ edge: edgeCollection }).where((_) => eq(_.edge.flowId, flowId)),
  );

  // Build node info for layout
  const nodes = freshNodes
    .filter((n) => n.nodeId != null)
    .map((n) => ({
      id: Ulid.construct(n.nodeId).toCanonical(),
      kind: NODE_KIND_NAMES[n.kind] ?? 'Unknown',
      name: n.name,
      position: { x: n.position?.x ?? 0, y: n.position?.y ?? 0 },
      state: 'Idle',
    }));

  // Build a set of valid node IDs for filtering
  const validNodeIds = new Set(nodes.map((n) => n.id));

  // Build edge info for layout - only include edges where both source and target exist
  const edges = freshEdges
    .filter((e) => e.edgeId != null && e.sourceId != null && e.targetId != null)
    .map((e) => ({
      id: Ulid.construct(e.edgeId).toCanonical(),
      sourceHandle: e.sourceHandle !== undefined ? String(e.sourceHandle) : undefined,
      sourceId: Ulid.construct(e.sourceId).toCanonical(),
      targetId: Ulid.construct(e.targetId).toCanonical(),
    }))
    .filter((e) => validNodeIds.has(e.sourceId) && validNodeIds.has(e.targetId));

  const result = layoutNodes(nodes, edges, defaultHorizontalConfig());
  if (!result) return;

  for (const [nodeId, position] of result.positions) {
    nodeCollection.utils.update({
      nodeId: Ulid.fromCanonical(nodeId).bytes,
      position: { x: position.x, y: position.y },
    });
  }
};

const clientToolSchemas: ToolSchema[] = [
  {
    description:
      "Inspect a node's full config and execution state. Returns type-specific config (HTTP: url/method/headers/params/body/assertions, JS: code, Condition: expression, For: iterations/condition, ForEach: path/condition) plus execution state/error. " +
      'Set includeOutput: true to also get execution input/output payloads (can be large).',
    name: 'inspectNode',
    parameters: {
      additionalProperties: false,
      properties: {
        includeOutput: {
          description:
            'Include execution input/output payloads (default: false). Only use when you need to see actual request/response data.',
          type: 'boolean',
        },
        nodeId: { description: 'The node ID to inspect', type: 'string' },
      },
      required: ['nodeId'],
      type: 'object',
    },
  },
  {
    description: 'Get a summary of the latest flow execution showing which nodes ran and which were never reached.',
    name: 'getFlowExecutionSummary',
    parameters: {
      additionalProperties: false,
      properties: {},
      required: [],
      type: 'object',
    },
  },
  {
    description:
      "Update any node's configuration in a single call. Provide nodeId and only the fields to change — unspecified fields stay unchanged. " +
      'Base fields (name) work on any node. Type-specific fields: ' +
      'Ai: prompt, maxIterations. Condition: condition. For: iterations, condition (break), errorHandling. ' +
      'ForEach: path, condition (break), errorHandling. JS: code. ' +
      'HTTP: method, url, headers, searchParams, body, assertions (arrays replace existing set).',
    name: 'updateNode',
    parameters: {
      additionalProperties: false,
      properties: {
        assertions: {
          description: 'Replaces all existing assertions (HTTP only)',
          items: {
            properties: {
              enabled: { type: 'boolean' },
              value: { type: 'string' },
            },
            required: ['value'],
            type: 'object',
          },
          type: 'array',
        },
        body: {
          description:
            'Raw body content (JSON string). Set to null to clear. (HTTP only) Supports {{variable}} interpolation.',
          type: ['string', 'null'],
        },
        code: {
          description: 'JavaScript code (JS nodes only)',
          type: 'string',
        },
        condition: {
          description:
            'For Condition nodes: branching expression. For For/ForEach: break condition (expr-lang syntax).',
          type: 'string',
        },
        errorHandling: {
          description: 'Error handling strategy (For/ForEach only)',
          enum: ['ignore', 'break'],
          type: 'string',
        },
        headers: {
          description: 'Replaces all existing headers (HTTP only)',
          items: {
            properties: {
              enabled: { type: 'boolean' },
              key: { type: 'string' },
              value: {
                description: 'Supports {{variable}} interpolation, e.g. Bearer {{Auth.response.body.token}}',
                type: 'string',
              },
            },
            required: ['key'],
            type: 'object',
          },
          type: 'array',
        },
        iterations: {
          description: 'Number of loop iterations, must be positive (For nodes only)',
          type: 'integer',
        },
        maxIterations: {
          description: 'Maximum number of agentic iterations, must be positive (Ai nodes only)',
          type: 'integer',
        },
        method: {
          description: 'HTTP method (HTTP nodes only)',
          enum: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'],
          type: 'string',
        },
        name: {
          description: 'New node name (any node type)',
          type: 'string',
        },
        nodeId: { description: 'The node ID to update', type: 'string' },
        path: {
          description: 'Collection expression to iterate (ForEach nodes only, expr-lang syntax)',
          type: 'string',
        },
        prompt: {
          description: 'The prompt or system instructions for the AI agent (Ai nodes only)',
          type: 'string',
        },
        searchParams: {
          description: 'Replaces all existing query parameters (HTTP only)',
          items: {
            properties: {
              enabled: { type: 'boolean' },
              key: { type: 'string' },
              value: { description: 'Supports {{variable}} interpolation.', type: 'string' },
            },
            required: ['key'],
            type: 'object',
          },
          type: 'array',
        },
        url: {
          description:
            'Request URL (HTTP nodes only). Supports {{variable}} interpolation, e.g. {{BASE_URL}}/api/users/{{id}}',
          type: 'string',
        },
      },
      required: ['nodeId'],
      type: 'object',
    },
  },
  {
    description:
      'Incrementally add or remove headers, query params, or assertions on an HTTP node without replacing the full set. ' +
      'Use this when modifying individual items. For full replacement, use updateNode instead.',
    name: 'patchHttpNode',
    parameters: {
      additionalProperties: false,
      properties: {
        nodeId: { description: 'The HTTP node ID to patch', type: 'string' },
        addHeaders: {
          description: 'Headers to append. Supports {{variable}} interpolation in values.',
          items: {
            properties: {
              description: { type: 'string' },
              enabled: { type: 'boolean' },
              key: { type: 'string' },
              value: { description: 'Supports {{variable}} interpolation', type: 'string' },
            },
            required: ['key'],
            type: 'object',
          },
          type: 'array',
        },
        removeHeaderIds: {
          description: 'IDs of headers to remove (get IDs from inspectNode)',
          items: { type: 'string' },
          type: 'array',
        },
        addSearchParams: {
          description: 'Query params to append. Supports {{variable}} interpolation in values.',
          items: {
            properties: {
              description: { type: 'string' },
              enabled: { type: 'boolean' },
              key: { type: 'string' },
              value: { description: 'Supports {{variable}} interpolation', type: 'string' },
            },
            required: ['key'],
            type: 'object',
          },
          type: 'array',
        },
        removeSearchParamIds: {
          description: 'IDs of query params to remove (get IDs from inspectNode)',
          items: { type: 'string' },
          type: 'array',
        },
        addAssertions: {
          description: 'Assertions to append',
          items: {
            properties: {
              enabled: { type: 'boolean' },
              value: { type: 'string' },
            },
            required: ['value'],
            type: 'object',
          },
          type: 'array',
        },
        removeAssertionIds: {
          description: 'IDs of assertions to remove (get IDs from inspectNode)',
          items: { type: 'string' },
          type: 'array',
        },
      },
      required: ['nodeId'],
      type: 'object',
    },
  },
  {
    description:
      'PREFERRED tool for ALL node connections. Connects nodes into a chain with optional parallel fan-out. ' +
      'Flat array: sequential chain. Nested array: parallel branches. ' +
      'Example: ["Start",["A","B"],"End"] creates Start→A, Start→B, A→End, B→End. ' +
      'Works for ALL node types. For branching nodes (Condition, For, ForEach, Ai), auto-applies "then" handle by default. ' +
      'Use sourceHandle "else" or "loop" to override for non-default branches. ' +
      'Use sourceHandle "ai_tools" to connect tool nodes to an Ai node.',
    name: 'connectChain',
    parameters: {
      additionalProperties: false,
      properties: {
        nodeIds: {
          description:
            'Ordered list of node IDs. Use nested arrays for fan-out/fan-in: ' +
            '["A","B","C"] chains A→B→C. ' +
            '["A",["B","C"],"D"] fans out A→B, A→C then fans in B→D, C→D. ' +
            'Minimum 2 elements. No consecutive nested arrays.',
          items: { oneOf: [{ type: 'string' }, { items: { type: 'string' }, type: 'array' }] },
          type: 'array',
        },
        sourceHandle: {
          description:
            'Handle for branching source nodes. Defaults to "then". ' +
            'Use "else" for Condition false-branch, "loop" for For/ForEach loop-body, ' +
            '"ai_tools" for connecting tool nodes to an Ai node.',
          enum: ['then', 'else', 'loop', 'ai_tools'],
          type: 'string',
        },
      },
      required: ['nodeIds'],
      type: 'object',
    },
  },
];

interface UseAgentChatOptions {
  apiKey: string;
  flowId: Uint8Array;
  selectedNodeIds?: string[];
}

const createInitialAgentChatState = (): AgentChatState => ({
  error: null,
  isLoading: false,
  messages: [],
  streamingContent: '',
});

// ---------------------------------------------------------------------------
// Module-level external store – survives React component remounts
// ---------------------------------------------------------------------------

interface ChatStoreEntry {
  abortController: AbortController | null;
  state: AgentChatState;
}

const chatStoreEntries = new Map<string, ChatStoreEntry>();
const chatStoreListeners = new Map<string, Set<() => void>>();

const chatStore = {
  getAbortController(key: string): AbortController | null {
    return chatStoreEntries.get(key)?.abortController ?? null;
  },

  getState(key: string): AgentChatState {
    let entry = chatStoreEntries.get(key);
    if (!entry) {
      entry = { abortController: null, state: createInitialAgentChatState() };
      chatStoreEntries.set(key, entry);
    }
    return entry.state;
  },

  notify(key: string) {
    chatStoreListeners.get(key)?.forEach((cb) => cb());
  },

  setAbortController(key: string, ac: AbortController | null) {
    let entry = chatStoreEntries.get(key);
    if (!entry) {
      entry = { abortController: null, state: createInitialAgentChatState() };
      chatStoreEntries.set(key, entry);
    }
    entry.abortController = ac;
  },

  setState(key: string, updater: ((prev: AgentChatState) => AgentChatState) | AgentChatState) {
    let entry = chatStoreEntries.get(key);
    if (!entry) {
      entry = { abortController: null, state: createInitialAgentChatState() };
      chatStoreEntries.set(key, entry);
    }
    entry.state = typeof updater === 'function' ? updater(entry.state) : updater;
    chatStore.notify(key);
  },

  subscribe(key: string, callback: () => void): () => void {
    let listeners = chatStoreListeners.get(key);
    if (!listeners) {
      listeners = new Set();
      chatStoreListeners.set(key, listeners);
    }
    listeners.add(callback);
    return () => {
      listeners.delete(callback);
      if (listeners.size === 0) chatStoreListeners.delete(key);
    };
  },
};

export const useAgentChat = ({ apiKey, flowId, selectedNodeIds }: UseAgentChatOptions) => {
  const flowIdKey = Ulid.construct(flowId).toCanonical();

  const state = useSyncExternalStore(
    useCallback((cb: () => void) => chatStore.subscribe(flowIdKey, cb), [flowIdKey]),
    useCallback(() => chatStore.getState(flowIdKey), [flowIdKey]),
  );

  const { transport } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const flowContext = useFlowContext(flowId);

  // Use refs to always access latest values in callbacks
  const flowContextRef = useRef(flowContext);
  flowContextRef.current = flowContext;

  const selectedNodeIdsRef = useRef(selectedNodeIds);
  selectedNodeIdsRef.current = selectedNodeIds;

  const messagesRef = useRef(state.messages);
  messagesRef.current = state.messages;

  const nodeCollection = useApiCollection(NodeCollectionSchema);
  const edgeCollection = useApiCollection(EdgeCollectionSchema);
  const variableCollection = useApiCollection(FlowVariableCollectionSchema);
  const aiCollection = useApiCollection(NodeAiCollectionSchema);
  const jsCollection = useApiCollection(NodeJsCollectionSchema);
  const conditionCollection = useApiCollection(NodeConditionCollectionSchema);
  const forCollection = useApiCollection(NodeForCollectionSchema);
  const forEachCollection = useApiCollection(NodeForEachCollectionSchema);
  const nodeHttpCollection = useApiCollection(NodeHttpCollectionSchema);
  const httpCollection = useApiCollection(HttpCollectionSchema);
  const httpSearchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);
  const httpHeaderCollection = useApiCollection(HttpHeaderCollectionSchema);
  const httpBodyRawCollection = useApiCollection(HttpBodyRawCollectionSchema);
  const httpAssertCollection = useApiCollection(HttpAssertCollectionSchema);
  const executionCollection = useApiCollection(NodeExecutionCollectionSchema);
  const fileCollection = useApiCollection(FileCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);

  const sendMessage = useCallback(
    async (content: string) => {
      // Cancel any existing request
      chatStore.getAbortController(flowIdKey)?.abort();
      const abortController = new AbortController();
      chatStore.setAbortController(flowIdKey, abortController);

      const openai = createOpenRouterClient(apiKey);

      // Use ref to get latest flowContext at execution time
      const currentFlowContext = {
        ...flowContextRef.current,
        selectedNodeIds: selectedNodeIdsRef.current,
      };

      // Build context fresh at execution time to avoid stale closures
      const collections: Collections = {
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
      };

      const waitForFlowCompletion = async (): Promise<void> => {
        const POLL_INTERVAL = 500;
        const MAX_WAIT = 30_000;
        const INITIAL_DELAY = 500;
        let elapsed = 0;

        await new Promise((r) => setTimeout(r, INITIAL_DELAY));
        elapsed += INITIAL_DELAY;

        while (elapsed < MAX_WAIT) {
          await new Promise((r) => setTimeout(r, POLL_INTERVAL));
          elapsed += POLL_INTERVAL;

          const [flow] = await queryCollection((_) =>
            _.from({ item: flowCollection })
              .where((_) => eq(_.item.flowId, flowId))
              .findOne(),
          );
          if (flow && !flow.running) break;
        }
      };

      const toolContext: ToolExecutorContext = {
        collections,
        flowContext: currentFlowContext,
        sessionCreatedNodeIds: new Set<string>(),
        transport,
        waitForFlowCompletion,
        workspaceId,
      };

      const userMessage: Message = {
        content,
        id: generateId(),
        role: 'user',
        timestamp: Date.now(),
      };

      const logger = new AgentLogger(currentFlowContext.flowId);
      logger.logSessionStart(currentFlowContext.flowId, content);

      chatStore.setState(flowIdKey, (prev) => ({
        ...prev,
        error: null,
        isLoading: true,
        messages: [...prev.messages, userMessage],
      }));

      try {
        const systemPrompt = buildSystemPrompt(currentFlowContext);
        const tools = [...allToolSchemas, ...clientToolSchemas].map(formatToolAsOpenAI);

        logger.logSystemPrompt(systemPrompt, {
          edges: currentFlowContext.edges.length,
          nodes: currentFlowContext.nodes.length,
          variables: currentFlowContext.variables.length,
        });
        logger.logUserMessage(content);

        const openAIMessages: OpenAIMessage[] = [
          { content: systemPrompt, role: 'system' },
          ...messagesRef.current.map(messageToOpenAI),
          { content, role: 'user' },
        ];

        logger.logApiRequest(MODEL, openAIMessages.length, true);
        let apiStart = performance.now();

        const updateStreamingContent = (content: string) => {
          chatStore.setState(flowIdKey, (prev) => ({ ...prev, streamingContent: content }));
        };

        let stream = await openai.chat.completions.create(
          {
            messages: openAIMessages,
            model: MODEL,
            stream: true,
            tool_choice: 'auto',
            tools,
          },
          { signal: abortController.signal },
        );

        let { message: streamedMsg, meta } = await consumeStream(stream, updateStreamingContent);
        chatStore.setState(flowIdKey, (prev) => ({ ...prev, streamingContent: '' }));

        logger.logApiResponse(performance.now() - apiStart, meta.finishReason, meta.usage);
        let assistantMessage = streamedMsg;

        let validationRetries = 0;
        const MAX_VALIDATION_RETRIES = 2;

        for (;;) {
          // === Existing tool call loop ===
          while (assistantMessage?.tool_calls && assistantMessage.tool_calls.length > 0) {
            const toolCalls: ToolCall[] = assistantMessage.tool_calls.map((tc) => ({
              arguments: JSON.parse(tc.function.arguments) as Record<string, unknown>,
              id: tc.id,
              name: tc.function.name,
            }));

            const toolMessage: Message = {
              content: assistantMessage.content ?? '',
              id: generateId(),
              role: 'assistant',
              timestamp: Date.now(),
              toolCalls,
            };

            chatStore.setState(flowIdKey, (prev) => ({
              ...prev,
              messages: [...prev.messages, toolMessage],
            }));

            for (const tc of toolCalls) {
              logger.logToolCallStart(tc.id, tc.name, tc.arguments);
            }

            const toolCallTimers: number[] = [];
            const toolResults: ToolResult[] = [];
            for (const tc of toolCalls) {
              toolCallTimers.push(performance.now());
              toolResults.push(await executeToolCall(tc, flowId, toolContext));
            }

            for (let i = 0; i < toolResults.length; i++) {
              const tr = toolResults[i]!;
              const tc = toolCalls[i]!;
              const elapsed = performance.now() - toolCallTimers[i]!;
              logger.logToolCallEnd(tc.id, tc.name, elapsed, tr.error ?? safeStringify(tr.result), tr.error);
            }

            // Apply layout and refresh context after mutations
            const hadMutations = toolResults.some((tr: ToolResult) => tr.isMutation && !tr.error);
            if (hadMutations) {
              // Query fresh data directly from collections to avoid stale React context
              await applyLayoutToFlow(flowId, nodeCollection, edgeCollection);

              // Refresh flow context so subsequent tool calls see newly created nodes
              toolContext.flowContext = {
                ...(await refreshFlowContext(flowId, {
                  edgeCollection,
                  executionCollection,
                  httpCollection,
                  nodeCollection,
                  nodeHttpCollection,
                  variableCollection,
                })),
                selectedNodeIds: selectedNodeIdsRef.current,
              };

              // Inject updated flow state so LLM sees current topology
              const stateSummary = buildCompactStateSummary(toolContext.flowContext);
              openAIMessages.push({ content: stateSummary, role: 'system' });
            }

            const toolResultMessages: Message[] = toolResults.map((tr) => ({
              content: tr.error ?? safeStringify(tr.result),
              id: generateId(),
              role: 'tool' as const,
              timestamp: Date.now(),
              toolCallId: tr.toolCallId,
            }));

            chatStore.setState(flowIdKey, (prev) => ({
              ...prev,
              messages: [...prev.messages, ...toolResultMessages],
            }));

            openAIMessages.push({
              content: assistantMessage.content,
              role: 'assistant',
              tool_calls: assistantMessage.tool_calls,
            });

            // Collapse identical error messages to reduce noise
            const errorGroups = new Map<string, { count: number; firstId: string }>();
            for (const tr of toolResults) {
              if (tr.error) {
                const existing = errorGroups.get(tr.error);
                if (existing) {
                  existing.count++;
                } else {
                  errorGroups.set(tr.error, { count: 1, firstId: tr.toolCallId });
                }
              }
            }

            for (const tr of toolResults) {
              const errorGroup = tr.error ? errorGroups.get(tr.error) : undefined;
              let content: string;
              if (tr.error && errorGroup && errorGroup.count > 1) {
                if (tr.toolCallId === errorGroup.firstId) {
                  content = `${tr.error} (this error occurred ${errorGroup.count} times in this batch)`;
                } else {
                  content = `Same error as ${errorGroup.firstId}`;
                }
              } else {
                content = tr.error ?? safeStringify(tr.result);
              }
              openAIMessages.push({
                content,
                role: 'tool',
                tool_call_id: tr.toolCallId,
              });
            }

            logger.logApiRequest(MODEL, openAIMessages.length, true);
            apiStart = performance.now();

            stream = await openai.chat.completions.create(
              {
                messages: openAIMessages,
                model: MODEL,
                stream: true,
                tool_choice: 'auto',
                tools,
              },
              { signal: abortController.signal },
            );

            ({ message: streamedMsg, meta } = await consumeStream(stream, updateStreamingContent));
            chatStore.setState(flowIdKey, (prev) => ({ ...prev, streamingContent: '' }));

            logger.logApiResponse(performance.now() - apiStart, meta.finishReason, meta.usage);
            assistantMessage = streamedMsg;
          }

          // === Post-execution validation: check for orphan nodes ===
          if (validationRetries >= MAX_VALIDATION_RETRIES) break;

          const freshNodes = await queryCollection((_) =>
            _.from({ node: nodeCollection }).where((_) => eq(_.node.flowId, flowId)),
          );
          const freshEdges = await queryCollection((_) =>
            _.from({ edge: edgeCollection }).where((_) => eq(_.edge.flowId, flowId)),
          );

          const nodeInfos = freshNodes
            .filter((n) => n.nodeId != null)
            .map((n) => ({
              id: Ulid.construct(n.nodeId).toCanonical(),
              kind: NODE_KIND_NAMES[n.kind] ?? 'Unknown',
              name: n.name,
            }));
          const edgeInfos = freshEdges
            .filter((e) => e.edgeId != null)
            .map((e) => ({
              sourceId: Ulid.construct(e.sourceId).toCanonical(),
              targetId: Ulid.construct(e.targetId).toCanonical(),
            }));

          const orphans = detectOrphanNodes(nodeInfos, edgeInfos);
          const deadEnds = orphans.length === 0 ? detectDeadEndNodes(nodeInfos, edgeInfos) : [];
          logger.logValidation(
            orphans.length,
            orphans.map((n) => n.name),
          );
          if (orphans.length === 0 && deadEnds.length === 0) break;

          validationRetries++;

          const validationContent = buildXmlValidationMessage(orphans, deadEnds);

          // Add the assistant's text response to messages before injecting validation
          if (assistantMessage?.content) {
            openAIMessages.push({
              content: assistantMessage.content,
              role: 'assistant',
            });
          }

          openAIMessages.push({
            content: validationContent,
            role: 'user',
          });

          logger.logApiRequest(MODEL, openAIMessages.length, true);
          apiStart = performance.now();

          stream = await openai.chat.completions.create(
            { messages: openAIMessages, model: MODEL, stream: true, tool_choice: 'auto', tools },
            { signal: abortController.signal },
          );

          ({ message: streamedMsg, meta } = await consumeStream(stream, updateStreamingContent));
          chatStore.setState(flowIdKey, (prev) => ({ ...prev, streamingContent: '' }));

          logger.logApiResponse(performance.now() - apiStart, meta.finishReason, meta.usage);
          assistantMessage = streamedMsg;
        }

        const finalMessage: Message = {
          content: assistantMessage?.content ?? '',
          id: generateId(),
          role: 'assistant',
          timestamp: Date.now(),
        };

        logger.logAssistantMessage(finalMessage.content);
        logger.logSessionEnd(true, false);

        chatStore.setState(flowIdKey, (prev) => ({
          ...prev,
          isLoading: false,
          messages: [...prev.messages, finalMessage],
        }));
      } catch (error) {
        // Ignore abort errors
        if (error instanceof Error && error.name === 'AbortError') {
          logger.logSessionEnd(false, true);
          chatStore.setState(flowIdKey, (prev) => ({ ...prev, isLoading: false, streamingContent: '' }));
          return;
        }
        logger.logError(error, 'sendMessage');
        logger.logSessionEnd(false, false);
        const errorMessage = error instanceof Error ? error.message : 'An error occurred';
        chatStore.setState(flowIdKey, (prev) => ({
          ...prev,
          error: errorMessage,
          isLoading: false,
          streamingContent: '',
        }));
      } finally {
        if (chatStore.getAbortController(flowIdKey) === abortController) {
          chatStore.setAbortController(flowIdKey, null);
        }
      }
    },
    [
      apiKey,
      flowId,
      transport,
      nodeCollection,
      edgeCollection,
      variableCollection,
      aiCollection,
      jsCollection,
      conditionCollection,
      forCollection,
      forEachCollection,
      nodeHttpCollection,
      httpCollection,
      httpSearchParamCollection,
      httpHeaderCollection,
      httpBodyRawCollection,
      httpAssertCollection,
      executionCollection,
      fileCollection,
      flowCollection,
      workspaceId,
    ],
  );

  const clearMessages = useCallback(() => {
    chatStore.getAbortController(flowIdKey)?.abort();
    chatStore.setAbortController(flowIdKey, null);
    chatStore.setState(flowIdKey, {
      messages: [],
      isLoading: false,
      error: null,
      streamingContent: '',
    });
  }, [flowIdKey]);

  const cancel = useCallback(() => {
    chatStore.getAbortController(flowIdKey)?.abort();
    chatStore.setAbortController(flowIdKey, null);
    chatStore.setState(flowIdKey, (prev) => ({ ...prev, isLoading: false, streamingContent: '' }));
  }, [flowIdKey]);

  return {
    cancel,
    clearMessages,
    error: state.error,
    isLoading: state.isLoading,
    messages: state.messages,
    sendMessage,
    streamingContent: state.streamingContent,
  };
};

const messageToOpenAI = (message: Message): OpenAIMessage => {
  if (message.role === 'tool' && message.toolCallId) {
    return {
      content: message.content,
      role: 'tool',
      tool_call_id: message.toolCallId,
    };
  }

  if (message.role === 'assistant' && message.toolCalls) {
    return {
      content: message.content,
      role: 'assistant',
      tool_calls: message.toolCalls.map((tc) => ({
        function: {
          arguments: JSON.stringify(tc.arguments),
          name: tc.name,
        },
        id: tc.id,
        type: 'function' as const,
      })),
    };
  }

  return {
    content: message.content,
    role: message.role as 'assistant' | 'system' | 'user',
  };
};
