import type { ChatCompletionTool } from 'openai/resources/chat/completions';
import type { OpenAIMessage } from './types';

/**
 * "Stresseur (included)": Claude on the workspace's included AI credits,
 * through the desktop main process (`POST /api/cli/studio/ai/messages`). The
 * agent keeps its history in OpenAI chat format; this converts it to the
 * Anthropic Messages shape and reads the Anthropic stream events back.
 */

export const STRESSEUR_MODEL = 'claude-sonnet-5-5';
const MAX_TOKENS = 8192;

type AnthropicBlock =
  | { content: string; is_error?: boolean; tool_use_id: string; type: 'tool_result' }
  | { id: string; input: unknown; name: string; type: 'tool_use' }
  | { text: string; type: 'text' };

interface AnthropicMessage {
  content: AnthropicBlock[];
  role: 'assistant' | 'user';
}

export interface StresseurRequest {
  max_tokens: number;
  messages: AnthropicMessage[];
  model: string;
  system?: string;
  tool_choice: { type: 'auto' };
  tools: { description: string; input_schema: Record<string, unknown>; name: string }[];
}

/** Error from Stresseur Cloud, kept whole so the panel can offer "Use your own key" and "Upgrade". */
export class StresseurCloudRequestError extends Error {
  constructor(readonly error: StresseurCloudError) {
    super(
      error.kind === 'error'
        ? error.message
        : error.kind === 'offline'
          ? 'Stresseur Cloud is unreachable. Check your connection, or use your own key.'
          : 'Sign in to Stresseur again, or use your own key.',
    );
    this.name = 'StresseurCloudRequestError';
  }
}

const textOf = (content: OpenAIMessage['content']): string => {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content.map((part) => ('text' in part && typeof part.text === 'string' ? part.text : '')).join('');
};

const parseArguments = (raw: string): unknown => {
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    return {};
  }
};

/**
 * OpenAI chat messages to Anthropic Messages: leading system messages become
 * `system`; later ones (state summaries) become user text; tool results join
 * the next user turn. Consecutive same-role turns are merged.
 */
export const toStresseurRequest = (messages: OpenAIMessage[], tools: ChatCompletionTool[]): StresseurRequest => {
  const system: string[] = [];
  const out: AnthropicMessage[] = [];

  const push = (role: AnthropicMessage['role'], blocks: AnthropicBlock[]) => {
    if (blocks.length === 0) return;
    const last = out.at(-1);
    if (last?.role === role) last.content.push(...blocks);
    else out.push({ content: blocks, role });
  };

  for (const message of messages) {
    if (message.role === 'system' || message.role === 'developer') {
      const text = textOf(message.content);
      if (!text) continue;
      if (out.length === 0) system.push(text);
      else push('user', [{ text, type: 'text' }]);
      continue;
    }

    if (message.role === 'user') {
      const text = textOf(message.content);
      if (text) push('user', [{ text, type: 'text' }]);
      continue;
    }

    if (message.role === 'tool') {
      push('user', [{ content: textOf(message.content), tool_use_id: message.tool_call_id, type: 'tool_result' }]);
      continue;
    }

    if (message.role === 'assistant') {
      const blocks: AnthropicBlock[] = [];
      const text = textOf(message.content ?? null);
      if (text.trim()) blocks.push({ text, type: 'text' });
      for (const call of message.tool_calls ?? []) {
        if (call.type !== 'function') continue;
        blocks.push({
          id: call.id,
          input: parseArguments(call.function.arguments),
          name: call.function.name,
          type: 'tool_use',
        });
      }
      push('assistant', blocks);
    }
  }

  return {
    max_tokens: MAX_TOKENS,
    messages: out,
    model: STRESSEUR_MODEL,
    ...(system.length > 0 ? { system: system.join('\n\n') } : {}),
    tool_choice: { type: 'auto' },
    tools: tools.flatMap((tool) =>
      tool.type === 'function'
        ? [
            {
              description: tool.function.description ?? '',
              input_schema: (tool.function.parameters ?? { properties: {}, type: 'object' }) as Record<string, unknown>,
              name: tool.function.name,
            },
          ]
        : [],
    ),
  };
};

export interface StreamedTurn {
  message: {
    content: null | string;
    tool_calls?: undefined | { function: { arguments: string; name: string }; id: string; type: 'function' }[];
  };
  meta: { finishReason: null | string | undefined; usage: unknown };
}

interface StreamEvent {
  content_block?: { id?: string; name?: string; type?: string };
  delta?: { partial_json?: string; stop_reason?: null | string; text?: string; type?: string };
  error?: { message?: string };
  index?: number;
  message?: { usage?: unknown };
  type?: string;
  usage?: unknown;
}

const FINISH_REASONS: Record<string, string> = { end_turn: 'stop', max_tokens: 'length', tool_use: 'tool_calls' };

/** One streamed turn on included credits, shaped like the OpenAI stream the agent loop reads. */
export const streamStresseurTurn = async (
  request: StresseurRequest,
  signal: AbortSignal,
  onContent: (accumulated: string) => void,
): Promise<StreamedTurn> => {
  const cloud = window.electron.cloud;
  if (!cloud) throw new StresseurCloudRequestError({ kind: 'signed-out' });

  const id = crypto.randomUUID();
  let content = '';
  const tools = new Map<number, { arguments: string; id: string; name: string }>();
  let finishReason: null | string | undefined = null;
  let usage: unknown = undefined;
  const failure: { message: null | string } = { message: null };

  const onEvent = (raw: unknown) => {
    const event = raw as StreamEvent;
    switch (event.type) {
      case 'content_block_delta': {
        if (event.delta?.type === 'text_delta' && event.delta.text) {
          content += event.delta.text;
          onContent(content);
        }
        const tool = tools.get(event.index ?? -1);
        if (event.delta?.type === 'input_json_delta' && tool) tool.arguments += event.delta.partial_json ?? '';
        break;
      }
      case 'content_block_start':
        if (event.content_block?.type === 'tool_use')
          tools.set(event.index ?? tools.size, {
            arguments: '',
            id: event.content_block.id ?? '',
            name: event.content_block.name ?? '',
          });
        break;
      case 'error':
        failure.message = event.error?.message ?? 'Claude stopped with an error.';
        break;
      case 'message_delta':
        if (event.delta?.stop_reason) finishReason = FINISH_REASONS[event.delta.stop_reason] ?? event.delta.stop_reason;
        usage = event.usage;
        break;
      case 'message_start':
        usage = event.message?.usage;
        break;
    }
  };

  const abort = () => void cloud.aiAbort(id);
  signal.addEventListener('abort', abort, { once: true });
  let result: StresseurCloudResult<null>;
  try {
    result = await cloud.aiMessages(id, request, onEvent);
  } finally {
    signal.removeEventListener('abort', abort);
  }

  if (signal.aborted) throw new DOMException('Aborted', 'AbortError');
  if (!result.ok) throw new StresseurCloudRequestError(result.error);
  if (failure.message) throw new Error(failure.message);

  const toolCalls = [...tools.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, tool]) => ({
      function: { arguments: tool.arguments || '{}', name: tool.name },
      id: tool.id,
      type: 'function' as const,
    }));

  return {
    message: { content: content || null, ...(toolCalls.length > 0 ? { tool_calls: toolCalls } : {}) },
    meta: { finishReason, usage },
  };
};
