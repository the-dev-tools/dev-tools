import type { ChatCompletionMessageParam, ChatCompletionTool } from 'openai/resources/chat/completions';

export type MessageRole = 'assistant' | 'system' | 'tool' | 'user';

export interface Message {
  content: string;
  id: string;
  role: MessageRole;
  timestamp: number;
  toolCallId?: string;
  toolCalls?: ToolCall[];
}

export interface ToolCall {
  arguments: Record<string, unknown>;
  id: string;
  name: string;
}

export interface ToolResult {
  error?: string;
  isMutation?: boolean;
  result: unknown;
  toolCallId: string;
}

export interface AgentChatState {
  error: null | string;
  isLoading: boolean;
  messages: Message[];
  streamingContent: string;
}

export interface FlowContextData {
  edges: EdgeInfo[];
  executions: NodeExecutionInfo[];
  flowId: string;
  nodes: NodeInfo[];
  selectedNodeIds?: string[];
  variables: VariableInfo[];
}

export interface NodeInfo {
  httpId?: string;
  httpMethod?: string;
  id: string;
  info?: string;
  kind: string;
  name: string;
  position: { x: number; y: number };
  state: string;
}

export interface NodeExecutionInfo {
  completedAt?: string;
  error?: string;
  id: string;
  input?: unknown;
  name: string;
  nodeId: string;
  output?: unknown;
  state: string;
}

export interface EdgeInfo {
  id: string;
  sourceHandle?: string;
  sourceId: string;
  targetId: string;
}

export interface VariableInfo {
  enabled: boolean;
  id: string;
  key: string;
  value: string;
}

export interface ToolSchema {
  description: string;
  name: string;
  parameters: {
    additionalProperties?: boolean;
    properties: Record<string, unknown>;
    required?: string[];
    type: 'object';
  };
}

export const formatToolAsOpenAI = (schema: ToolSchema): ChatCompletionTool => ({
  function: {
    description: schema.description,
    name: schema.name,
    parameters: schema.parameters,
  },
  type: 'function',
});

export type OpenAIMessage = ChatCompletionMessageParam;
