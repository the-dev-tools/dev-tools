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

@AITools.mutationTool(
  #{ operation: AITools.CrudOperation.Insert, title: "Create Condition Node", name: "CreateConditionNode", parent: "Node", exclude: #["kind"], description: "Create a condition node that routes flow based on a boolean expression. Has THEN and ELSE output handles." }
)
@TanStackDB.collection
model NodeCondition {
  @primaryKey nodeId: Id;
  condition: string;
}

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
