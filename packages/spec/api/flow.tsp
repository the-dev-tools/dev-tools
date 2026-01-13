using DevTools;

namespace Api.Flow;

@TanStackDB.collection
model Flow {
  @primaryKey flowId: Id;
  @foreignKey @visibility(Lifecycle.Create) workspaceId: Id;
  name: string;
  @visibility(Lifecycle.Read) duration?: int32;
  @visibility(Lifecycle.Read) running: boolean;
}

model FlowDuplicateRequest {
  flowId: Id;
}

op FlowDuplicate(...FlowDuplicateRequest): {};

model FlowRunRequest {
  flowId: Id;
}

op FlowRun(...FlowRunRequest): {};

model FlowStopRequest {
  flowId: Id;
}

op FlowStop(...FlowStopRequest): {};

@TanStackDB.collection(#{ isReadOnly: true })
model FlowVersion {
  @primaryKey flowVersionId: Id;
  @foreignKey flowId: Id;
}

@TanStackDB.collection
model FlowVariable {
  @primaryKey flowVariableId: Id;
  ...CommonTableFields<Flow>;
}

enum HandleKind {
  Then,
  Else,
  Loop,
  AiModel,
  AiMemory,
  AiTools,
}

@TanStackDB.collection
model Edge {
  @primaryKey edgeId: Id;
  @foreignKey flowId: Id;
  @foreignKey sourceId: Id;
  @foreignKey targetId: Id;
  sourceHandle: HandleKind;
  @visibility(Lifecycle.Read) state: FlowItemState;
}

enum NodeKind {
  ManualStart,
  Http,
  Condition,
  For,
  ForEach,
  Js,
  Ai,
}

enum AiModel {
  // OpenAI - GPT-5.2 family
  Gpt52Instant,
  Gpt52Thinking,
  Gpt52Pro,
  Gpt52Codex,

  // OpenAI - Reasoning models
  O3,
  O4Mini,

  // Anthropic - Claude 4.5 family
  ClaudeOpus45,
  ClaudeSonnet45,
  ClaudeHaiku45,

  // Google - Gemini 3 family
  Gemini3Pro,
  Gemini3Flash,

  Custom,
}

enum FlowItemState {
// ... (skip lines to match context if needed, but easier to do separate calls)

  Running,
  Success,
  Failure,
  Canceled,
}

model Position {
  x: float32;
  y: float32;
}

@TanStackDB.collection
model Node {
  @primaryKey nodeId: Id;
  @foreignKey flowId: Id;
  kind: NodeKind;
  name: string;
  position: Position;
  @visibility(Lifecycle.Read) state: FlowItemState;
  @visibility(Lifecycle.Read) info?: string;
}

@TanStackDB.collection
model NodeHttp {
  @primaryKey nodeId: Id;
  @foreignKey httpId: Id;
  @foreignKey deltaHttpId?: Id;
}

enum ErrorHandling {
  Ignore,
  Break,
}

@TanStackDB.collection
model NodeFor {
  @primaryKey nodeId: Id;
  iterations: int32;
  condition: string;
  errorHandling: ErrorHandling;
}

@TanStackDB.collection
model NodeForEach {
  @primaryKey nodeId: Id;
  path: string;
  condition: string;
  errorHandling: ErrorHandling;
}

@TanStackDB.collection
model NodeCondition {
  @primaryKey nodeId: Id;
  condition: string;
}

@TanStackDB.collection
model NodeJs {
  @primaryKey nodeId: Id;
  code: string;
}

@TanStackDB.collection
model NodeAi {
  @primaryKey nodeId: Id;
  `model`: AiModel;
  @foreignKey credentialId: Id;
  prompt: string;
  maxIterations: int32;
}

@TanStackDB.collection(#{ isReadOnly: true })
model NodeExecution {
  @primaryKey nodeExecutionId: Id;
  @foreignKey nodeId: Id;
  name: string;
  state: FlowItemState;
  error?: string;
  input?: Protobuf.WellKnown.Json;
  output?: Protobuf.WellKnown.Json;
  httpResponseId?: Id;
  completedAt?: Protobuf.WellKnown.Timestamp;
}
