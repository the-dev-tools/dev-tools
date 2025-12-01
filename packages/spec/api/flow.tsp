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
}

enum EdgeKind {
  NoOp,
}

@TanStackDB.collection
model Edge {
  @primaryKey edgeId: Id;
  @foreignKey flowId: Id;
  kind: EdgeKind;
  @foreignKey sourceId: Id;
  @foreignKey targetId: Id;
  sourceHandle: HandleKind;
  @visibility(Lifecycle.Read) state: FlowItemState;
}

enum NodeKind {
  NoOp,
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

enum NodeNoOpKind {
  Start,
  Create,
  Then,
  Else,
  Loop,
}

@TanStackDB.collection
model NodeNoOp {
  @primaryKey nodeId: Id;
  kind: NodeNoOpKind;
}

@TanStackDB.collection
model NodeHttp {
  @primaryKey nodeId: Id;
  @foreignKey httpId?: Id;
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
