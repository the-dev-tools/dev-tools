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

@AITools.aiTool(#{ category: AITools.ToolCategory.Execution, title: "Run Flow" })
@doc("Execute a workflow from the start node.")
model FlowRunRequest {
  @doc("The ULID of the workflow to run") flowId: Id;
}

op FlowRun(...FlowRunRequest): {};

@AITools.aiTool(#{ category: AITools.ToolCategory.Execution, title: "Stop Flow" })
@doc("Stop a running workflow execution.")
model FlowStopRequest {
  @doc("The ULID of the workflow to stop") flowId: Id;
}

op FlowStop(...FlowStopRequest): {};

@TanStackDB.collection(#{ isReadOnly: true })
model FlowVersion {
  @primaryKey flowVersionId: Id;
  @foreignKey flowId: Id;
}

@AITools.mutationTool(
  #{
    operation: AITools.CrudOperation.Insert,
    title: "Create Variable",
    name: "CreateVariable",
    description: "Create a new workflow variable that can be referenced in node expressions.",
  },
  #{
    operation: AITools.CrudOperation.Update,
    title: "Update Variable",
    name: "UpdateVariable",
    description: "Update an existing workflow variable.",
  }
)
@TanStackDB.collection
model FlowVariable {
  @primaryKey flowVariableId: Id;
  ...CommonTableFields<Flow>;
}

enum HandleKind {
  Then,
  Else,
  Loop,
  AiProvider,
  AiMemory,
  AiTools,
}

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Delete,
  title: "Disconnect Nodes",
  name: "DisconnectNodes",
  description: "Remove an edge connection between nodes.",
})
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
  AiProvider,
  AiMemory,
}

enum AiMemoryType {
  WindowBuffer,
}

enum AiModel {
  // OpenAI - GPT-5.2 family
  Gpt52,

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

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Delete,
  description: "Delete a node from the workflow. Also removes all connected edges.",
})
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

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Insert,
  title: "Create HTTP Node",
  name: "CreateHttpNode",
  parent: "Node",
  exclude: #["kind", "httpId", "deltaHttpId"],
  include: #[#{ fromModel: "Http", fields: #["url", "method"] }],
  description: "Create a new HTTP request node that makes an API call.",
})
@TanStackDB.collection
model NodeHttp {
  @primaryKey nodeId: Id;
  @doc("The ULID of the HTTP request definition to use") @foreignKey httpId: Id;
  @foreignKey deltaHttpId?: Id;
}

enum ErrorHandling {
  Ignore,
  Break,
}

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Insert,
  title: "Create For Loop Node",
  name: "CreateForNode",
  parent: "Node",
  exclude: #["kind"],
  description: "Create a for-loop node. REQUIRED: iterations (positive integer) AND condition (break condition that exits the loop early when true). The loop runs up to 'iterations' times but stops early when the break condition evaluates to true.",
})
@TanStackDB.collection
model NodeFor {
  @primaryKey nodeId: Id;
  @doc("Number of iterations to perform (must be > 0)") iterations: int32;

  @doc("REQUIRED break condition - loop exits early when this expression is true. Use expr-lang syntax.")
  condition: string;

  errorHandling: ErrorHandling;
}

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Insert,
  title: "Create ForEach Loop Node",
  name: "CreateForEachNode",
  parent: "Node",
  exclude: #["kind"],
  description: "Create a forEach node that iterates over an array or object. REQUIRED: path (expression for the collection) AND condition (break condition that exits the loop early when true).",
})
@TanStackDB.collection
model NodeForEach {
  @primaryKey nodeId: Id;

  @doc("Expression for the array/object to iterate. Use expr-lang syntax: [\"JS Node\"].items or [\"HTTP\"].response.body")
  path: string;

  @doc("REQUIRED break condition - loop exits early when this expression is true. Use expr-lang syntax.")
  condition: string;

  errorHandling: ErrorHandling;
}

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Insert,
  title: "Create Condition Node",
  name: "CreateConditionNode",
  parent: "Node",
  exclude: #["kind"],
  description: "Create a condition node that routes flow based on a boolean expression. Has THEN and ELSE output handles.",
})
@TanStackDB.collection
model NodeCondition {
  @primaryKey nodeId: Id;
  condition: string;
}

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Insert,
  title: "Create JavaScript Node",
  name: "CreateJsNode",
  parent: "Node",
  exclude: #["kind"],
  description: "Create a new JavaScript node in the workflow. JS nodes can transform data, make calculations, or perform custom logic.",
})
@TanStackDB.collection
model NodeJs {
  @primaryKey nodeId: Id;
  code: string;
}

@AITools.mutationTool(#{
  operation: AITools.CrudOperation.Insert,
  title: "Create AI Node",
  name: "CreateAiNode",
  parent: "Node",
  exclude: #["kind"],
  description: "Create a new AI agent node that uses an LLM to process a prompt. The AI node orchestrates tool calling and can iterate multiple times.",
})
@TanStackDB.collection
model NodeAi {
  @primaryKey nodeId: Id;
  @doc("The prompt or system instructions for the AI agent") prompt: string;
  @doc("Maximum number of agentic iterations (LLM calls) before stopping, must be > 0") maxIterations: int32;
}

@TanStackDB.collection
model NodeAiProvider {
  @primaryKey nodeId: Id;
  @foreignKey credentialId: Id;
  `model`: AiModel;
  temperature?: float32;
  maxTokens?: int32;
}

@TanStackDB.collection
model NodeAiMemory {
  @primaryKey nodeId: Id;
  memoryType: AiMemoryType;
  windowSize: int32;
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
