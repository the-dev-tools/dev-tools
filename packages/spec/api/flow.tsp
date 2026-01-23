using DevTools;

namespace Api.Flow;

@AITools.explorationTool(#{ name: "ValidateWorkflow", title: "Validate Workflow", description: "Validate a workflow by checking its structure, connections, and node configurations for errors." })
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

@AITools.explorationTool(#{ name: "GetFlowVariable", title: "Get Flow Variable", description: "Get a flow variable by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Create Variable", name: "CreateVariable", description: "Create a new workflow variable that can be referenced in node expressions." },
  #{ operation: AITools.CrudOperation.Update, title: "Update Variable", name: "UpdateVariable", description: "Update an existing workflow variable." }
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
}

@AITools.explorationTool(#{ name: "GetEdge", title: "Get Edge", description: "Get a edge by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Connect Nodes", name: "ConnectNodes", description: "Create an edge connection between two nodes. IMPORTANT: For sequential flows (Manual Start, JS, HTTP nodes), do NOT specify sourceHandle - omit it entirely. Only use sourceHandle for Condition nodes (then/else) and Loop nodes (loop/then)." },
  #{ operation: AITools.CrudOperation.Delete, title: "Disconnect Nodes", name: "DisconnectNodes", description: "Remove an edge connection between nodes." }
)
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
}

enum FlowItemState {
  Running,
  Success,
  Failure,
  Canceled,
}

model Position {
  x: float32;
  y: float32;
}

@AITools.explorationTool(#{ name: "GetNode", title: "Get Node", description: "Get a node by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Update, title: "Update Node Config", name: "UpdateNodeConfig", description: "Update general node properties like name or position." },
  #{ operation: AITools.CrudOperation.Delete, title: "Delete Node", name: "DeleteNode", description: "Delete a node from the workflow. Also removes all connected edges." }
)
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

@AITools.explorationTool(#{ name: "GetNodeHttp", title: "Get Node Http", description: "Get a node http by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Create HTTP Node", name: "CreateHttpNode", parent: "Node", exclude: #["kind"], description: "Create a new HTTP request node that makes an API call." }
)
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

@AITools.explorationTool(#{ name: "GetNodeFor", title: "Get Node For", description: "Get a node for by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Create For Loop Node", name: "CreateForNode", parent: "Node", exclude: #["kind"], description: "Create a for-loop node that iterates a fixed number of times." }
)
@TanStackDB.collection
model NodeFor {
  @primaryKey nodeId: Id;
  @doc("Number of iterations to perform") iterations: int32;
  condition: string;
  errorHandling: ErrorHandling;
}

@AITools.explorationTool(#{ name: "GetNodeForEach", title: "Get Node For Each", description: "Get a node for each by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Create ForEach Loop Node", name: "CreateForEachNode", parent: "Node", exclude: #["kind"], description: "Create a forEach node that iterates over an array or object." }
)
@TanStackDB.collection
model NodeForEach {
  @primaryKey nodeId: Id;
  @doc("Path to the array/object to iterate (e.g., \"input.items\")") path: string;
  condition: string;
  errorHandling: ErrorHandling;
}

@AITools.explorationTool(#{ name: "GetNodeCondition", title: "Get Node Condition", description: "Get a node condition by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Create Condition Node", name: "CreateConditionNode", parent: "Node", exclude: #["kind"], description: "Create a condition node that routes flow based on a boolean expression. Has THEN and ELSE output handles." }
)
@TanStackDB.collection
model NodeCondition {
  @primaryKey nodeId: Id;
  condition: string;
}

@AITools.explorationTool(#{ name: "GetNodeJs", title: "Get Node Js", description: "Get a node js by its primary key." })
@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Create JavaScript Node", name: "CreateJsNode", parent: "Node", exclude: #["kind"], description: "Create a new JavaScript node in the workflow. JS nodes can transform data, make calculations, or perform custom logic." },
  #{ operation: AITools.CrudOperation.Update, title: "Update Node Code", name: "UpdateNodeCode", description: "Update the JavaScript code of a JS node." }
)
@TanStackDB.collection
model NodeJs {
  @primaryKey nodeId: Id;
  code: string;
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
