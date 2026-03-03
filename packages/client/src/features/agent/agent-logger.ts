/** JSON stringify with BigInt support */
const safeStringify = (value: unknown): string =>
  JSON.stringify(value, (_key: string, v: unknown) => (typeof v === 'bigint' ? v.toString() : v));

/** Truncate a string to maxLen, appending '...[truncated]' if needed */
const truncate = (s: string, maxLen = 2048): string => (s.length <= maxLen ? s : s.slice(0, maxLen) + '...[truncated]');

interface AgentLogIpc {
  cleanup: () => void;
  write: (fileName: string, jsonLine: string) => void;
}

interface LogEntry {
  [key: string]: unknown;
  event: string;
  sessionId: string;
  ts: string;
}

/** Get the agentLog IPC bridge if running inside Electron, null otherwise */
const getAgentLogIpc = (): AgentLogIpc | null => {
  if (typeof window === 'undefined') return null;
  const electron = (window as unknown as { electron?: { agentLog?: AgentLogIpc } }).electron;
  return electron?.agentLog ?? null;
};

/**
 * JSONL logger for agent conversations.
 * Writes to local files via Electron IPC. Silent no-op when running outside Electron.
 */
export class AgentLogger {
  private buffer: string[] = [];
  private fileName: string;
  private flushTimer: null | ReturnType<typeof setTimeout> = null;
  private ipc: AgentLogIpc | null;
  private sessionId: string;
  private sessionStart: number;

  constructor(flowId: string) {
    this.sessionId = crypto.randomUUID();
    this.sessionStart = performance.now();
    this.ipc = getAgentLogIpc();
    const shortFlowId = flowId.slice(0, 8);
    const ts = new Date().toISOString().replace(/[:.]/g, '-');
    this.fileName = `agent-${shortFlowId}-${ts}-${this.sessionId.slice(0, 8)}.jsonl`;
  }

  private write(entry: LogEntry) {
    if (!this.ipc) return;
    this.buffer.push(safeStringify(entry));
    this.flushTimer ??= setTimeout(() => void this.flush(), 100);
  }

  private flush() {
    if (!this.ipc || this.buffer.length === 0) return;
    const batch = this.buffer.join('\n') + '\n';
    this.buffer = [];
    this.ipc.write(this.fileName, batch);
  }

  // --- Event methods ---

  logSessionStart(flowId: string, messageContent: string) {
    this.write({
      event: 'session_start',
      flowId,
      sessionId: this.sessionId,
      ts: new Date().toISOString(),
      userMessagePreview: truncate(messageContent, 500),
    });
  }

  logSessionEnd(success: boolean, aborted: boolean) {
    this.write({
      aborted,
      durationMs: Math.round(performance.now() - this.sessionStart),
      event: 'session_end',
      sessionId: this.sessionId,
      success,
      ts: new Date().toISOString(),
    });
    // Flush synchronously on close
    this.close();
  }

  logSystemPrompt(prompt: string, contextStats: { edges: number; nodes: number; variables: number }) {
    this.write({
      contextStats,
      event: 'system_prompt',
      promptLength: prompt.length,
      sessionId: this.sessionId,
      ts: new Date().toISOString(),
    });
  }

  logUserMessage(content: string) {
    this.write({
      content: truncate(content),
      event: 'user_message',
      sessionId: this.sessionId,
      ts: new Date().toISOString(),
    });
  }

  logAssistantMessage(content: string) {
    this.write({
      content: truncate(content),
      event: 'assistant_message',
      sessionId: this.sessionId,
      ts: new Date().toISOString(),
    });
  }

  logApiRequest(model: string, messageCount: number, hasTools: boolean) {
    this.write({
      event: 'api_request',
      hasTools,
      messageCount,
      model,
      sessionId: this.sessionId,
      ts: new Date().toISOString(),
    });
  }

  logApiResponse(
    latencyMs: number,
    finishReason: null | string | undefined,
    usage: null | undefined | { completion_tokens?: number; prompt_tokens?: number; total_tokens?: number },
  ) {
    this.write({
      event: 'api_response',
      finishReason: finishReason ?? 'unknown',
      latencyMs: Math.round(latencyMs),
      sessionId: this.sessionId,
      ts: new Date().toISOString(),
      usage: usage ?? null,
    });
  }

  logToolCallStart(toolCallId: string, toolName: string, args: Record<string, unknown>) {
    this.write({
      args: truncate(safeStringify(args)),
      event: 'tool_call_start',
      sessionId: this.sessionId,
      toolCallId,
      toolName,
      ts: new Date().toISOString(),
    });
  }

  logToolCallEnd(toolCallId: string, toolName: string, durationMs: number, result: string, error?: string) {
    this.write({
      durationMs: Math.round(durationMs),
      error: error ?? undefined,
      event: 'tool_call_end',
      result: truncate(result),
      sessionId: this.sessionId,
      toolCallId,
      toolName,
      ts: new Date().toISOString(),
    });
  }

  logValidation(orphanCount: number, orphanNames: string[]) {
    this.write({
      event: 'validation',
      orphanCount,
      orphanNames,
      sessionId: this.sessionId,
      ts: new Date().toISOString(),
    });
  }

  logError(error: unknown, phase: string) {
    const message = error instanceof Error ? error.message : String(error);
    const stack = error instanceof Error ? error.stack : undefined;
    this.write({
      event: 'error',
      message,
      phase,
      sessionId: this.sessionId,
      stack,
      ts: new Date().toISOString(),
    });
  }

  /** Flush remaining buffer immediately */
  close() {
    if (this.flushTimer) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }
    this.flush();
  }
}
